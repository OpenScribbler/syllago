//! The filesystem seam. Every read and change the engine makes goes through
//! [`Fs`], so a test can fail any write or delete and check what rollback
//! leaves behind; [`RealFs`] is the one implementation over `std::fs`.

use std::fs;
use std::io::{self, Write};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicUsize, Ordering};

/// What is at a path.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Node {
    Missing,
    File {
        bytes: Vec<u8>,
        /// Any executable bit is set. Always `false` on Windows.
        executable: bool,
    },
    /// A directory, or a symlink to one.
    Dir,
    /// A symlink to anything but a directory, with the path it finally
    /// resolves to, or `None` when it resolves to nothing.
    Link {
        target: Option<PathBuf>,
    },
    /// Anything else, such as a device or a socket.
    Other,
}

/// The filesystem operations an apply or remove makes.
pub trait Fs {
    /// What is at `path`, without following a symlink at `path` itself
    /// except to tell whether it names a directory.
    fn read(&self, path: &Path) -> io::Result<Node>;
    /// Replaces whatever file is at `path` with `bytes`, whole: a failure
    /// leaves the previous file in place. On Linux and macOS the file is
    /// executable exactly when `executable` is set.
    fn write(&self, path: &Path, bytes: &[u8], executable: bool) -> io::Result<()>;
    fn remove_file(&self, path: &Path) -> io::Result<()>;
    /// Creates one directory, whose parent exists.
    fn create_dir(&self, path: &Path) -> io::Result<()>;
    /// Removes an empty directory, failing with
    /// [`io::ErrorKind::DirectoryNotEmpty`] when it holds anything.
    fn remove_dir(&self, path: &Path) -> io::Result<()>;
    /// Deletes what a [`Fs::write`] to `path` left beside it when the
    /// process died before the write finished.
    fn remove_temps(&self, path: &Path) -> io::Result<()>;
    /// `path` with every symlink in it resolved, for telling whether two
    /// paths name one file. A path that does not exist resolves through
    /// its nearest existing ancestor.
    fn canonical(&self, path: &Path) -> io::Result<PathBuf>;
}

/// [`Fs`] over `std::fs`.
///
/// A write goes to a temporary file beside the target and is renamed over
/// it, so a reader never sees a partial file; on Linux and macOS the
/// directory is then synced, so the rename survives a power loss. The
/// temporary file is readable by its owner alone until its bytes are
/// written. The file ends with the permissions of the file it replaces,
/// or the default for a new file, except that its execute bits follow
/// `executable`: set wherever read is, or cleared. Bytes are written as
/// given, so LF line endings stay LF on Windows. On Windows a rename or
/// delete that meets a sharing violation, because another process holds
/// the file open, is retried briefly.
#[derive(Debug, Clone, Copy, Default)]
pub struct RealFs;

impl Fs for RealFs {
    fn read(&self, path: &Path) -> io::Result<Node> {
        let metadata = match fs::symlink_metadata(path) {
            Ok(metadata) => metadata,
            Err(error)
                if matches!(
                    error.kind(),
                    io::ErrorKind::NotFound | io::ErrorKind::NotADirectory
                ) =>
            {
                return Ok(Node::Missing);
            }
            Err(error) => return Err(error),
        };
        if metadata.is_file() {
            return Ok(Node::File {
                bytes: fs::read(path)?,
                executable: is_executable(&metadata),
            });
        }
        if metadata.is_dir() || (metadata.is_symlink() && path.is_dir()) {
            return Ok(Node::Dir);
        }
        if metadata.is_symlink() {
            return Ok(Node::Link {
                target: fs::canonicalize(path).ok(),
            });
        }
        Ok(Node::Other)
    }

    fn write(&self, path: &Path, bytes: &[u8], executable: bool) -> io::Result<()> {
        let (temp, mut file, default_mode) = create_temp(path)?;
        let result = (|| {
            file.write_all(bytes)?;
            file.sync_all()?;
            drop(file);
            set_mode(&temp, path, executable, default_mode)?;
            retrying(|| fs::rename(&temp, path))?;
            sync_parent(path);
            Ok(())
        })();
        if result.is_err() {
            let _ = fs::remove_file(&temp);
        }
        result
    }

    fn remove_file(&self, path: &Path) -> io::Result<()> {
        retrying(|| fs::remove_file(path))
    }

    fn create_dir(&self, path: &Path) -> io::Result<()> {
        fs::create_dir(path)
    }

    fn remove_dir(&self, path: &Path) -> io::Result<()> {
        retrying(|| fs::remove_dir(path))
    }

    fn remove_temps(&self, path: &Path) -> io::Result<()> {
        let (Some(dir), Some(name)) = (path.parent(), path.file_name()) else {
            return Ok(());
        };
        let entries = match fs::read_dir(dir) {
            Ok(entries) => entries,
            Err(error) if error.kind() == io::ErrorKind::NotFound => return Ok(()),
            Err(error) => return Err(error),
        };
        for entry in entries {
            let entry = entry?;
            if is_temp_name(&entry.file_name(), name) {
                retrying(|| fs::remove_file(entry.path()))?;
            }
        }
        Ok(())
    }

    fn canonical(&self, path: &Path) -> io::Result<PathBuf> {
        match fs::canonicalize(path) {
            Err(error) if error.kind() == io::ErrorKind::NotFound => {
                match (path.parent(), path.file_name()) {
                    (Some(parent), Some(name)) => Ok(self.canonical(parent)?.join(name)),
                    _ => Ok(path.to_path_buf()),
                }
            }
            result => result,
        }
    }
}

/// Whether `candidate` is a temporary name [`create_temp`] gives a file
/// named `name`: `.<name>.syllago-<pid>-<n>.tmp`.
fn is_temp_name(candidate: &std::ffi::OsStr, name: &std::ffi::OsStr) -> bool {
    let (Some(candidate), Some(name)) = (candidate.to_str(), name.to_str()) else {
        return false;
    };
    let Some(rest) = candidate
        .strip_prefix('.')
        .and_then(|c| c.strip_prefix(name))
        .and_then(|c| c.strip_prefix(".syllago-"))
        .and_then(|c| c.strip_suffix(".tmp"))
    else {
        return false;
    };
    let digits = |s: &str| !s.is_empty() && s.bytes().all(|b| b.is_ascii_digit());
    rest.split_once('-')
        .is_some_and(|(pid, n)| digits(pid) && digits(n))
}

/// Syncs the directory holding `path`, so a rename into it is durable.
/// Best effort: the rename itself already happened.
fn sync_parent(path: &Path) {
    #[cfg(unix)]
    if let Some(parent) = path.parent()
        && let Ok(dir) = fs::File::open(parent)
    {
        let _ = dir.sync_all();
    }
    #[cfg(not(unix))]
    let _ = path;
}

/// Numbers temporary names within this process.
static TEMP_COUNTER: AtomicUsize = AtomicUsize::new(0);

/// Creates a temporary file beside `path`, in the same directory so the
/// rename never crosses a filesystem. Its name carries the process id and
/// a counter, and it is created only if nothing is there, so a file that
/// happens to have the name is never truncated or renamed away. On Linux
/// and macOS it is made readable by its owner alone before anything is
/// written to it; the mode it was created with, the default for a new
/// file, comes back with it.
fn create_temp(path: &Path) -> io::Result<(PathBuf, fs::File, u32)> {
    let name = path.file_name().ok_or_else(|| {
        io::Error::new(
            io::ErrorKind::InvalidInput,
            format!("{} names no file", path.display()),
        )
    })?;
    let mut collisions = 0;
    loop {
        let n = TEMP_COUNTER.fetch_add(1, Ordering::Relaxed);
        let temp = path.with_file_name(temp_name(name, n));
        match fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .open(&temp)
        {
            Ok(file) => {
                let default_mode = match private(&file) {
                    Ok(mode) => mode,
                    Err(error) => {
                        let _ = fs::remove_file(&temp);
                        return Err(error);
                    }
                };
                return Ok((temp, file, default_mode));
            }
            Err(error) if error.kind() == io::ErrorKind::AlreadyExists && collisions < 100 => {
                collisions += 1;
            }
            Err(error) => return Err(error),
        }
    }
}

fn temp_name(name: &std::ffi::OsStr, n: usize) -> std::ffi::OsString {
    let mut temp = std::ffi::OsString::from(".");
    temp.push(name);
    temp.push(format!(".syllago-{}-{n}.tmp", std::process::id()));
    temp
}

#[cfg(unix)]
fn is_executable(metadata: &fs::Metadata) -> bool {
    use std::os::unix::fs::PermissionsExt;
    metadata.permissions().mode() & 0o111 != 0
}

#[cfg(not(unix))]
fn is_executable(_: &fs::Metadata) -> bool {
    false
}

/// Makes an empty new file readable and writable by its owner alone,
/// returning the mode it had.
#[cfg(unix)]
fn private(file: &fs::File) -> io::Result<u32> {
    use std::os::unix::fs::PermissionsExt;
    let mode = file.metadata()?.permissions().mode() & 0o7777;
    file.set_permissions(fs::Permissions::from_mode(0o600))?;
    Ok(mode)
}

#[cfg(not(unix))]
fn private(_: &fs::File) -> io::Result<u32> {
    Ok(0)
}

/// Gives `temp` the permissions of the file at `target`, or
/// `default_mode` when there is none, with execute set wherever read is,
/// or cleared.
#[cfg(unix)]
fn set_mode(temp: &Path, target: &Path, executable: bool, default_mode: u32) -> io::Result<()> {
    use std::os::unix::fs::PermissionsExt;
    let base = match fs::symlink_metadata(target) {
        Ok(metadata) if metadata.is_file() => metadata.permissions().mode(),
        _ => default_mode,
    } & 0o7777;
    let mode = if executable {
        base | (base & 0o444) >> 2
    } else {
        base & !0o111
    };
    fs::set_permissions(temp, fs::Permissions::from_mode(mode))
}

#[cfg(not(unix))]
fn set_mode(_: &Path, _: &Path, _: bool, _: u32) -> io::Result<()> {
    Ok(())
}

/// Runs `op`, retrying on Windows while another process holds the file:
/// an editor or the provider itself reading it.
#[cfg(windows)]
fn retrying<T>(mut op: impl FnMut() -> io::Result<T>) -> io::Result<T> {
    // ERROR_ACCESS_DENIED, ERROR_SHARING_VIOLATION, ERROR_LOCK_VIOLATION.
    const HELD: [i32; 3] = [5, 32, 33];
    for attempt in 0..8 {
        match op() {
            Err(error)
                if error
                    .raw_os_error()
                    .is_some_and(|code| HELD.contains(&code)) =>
            {
                std::thread::sleep(std::time::Duration::from_millis(10 << attempt));
            }
            result => return result,
        }
    }
    op()
}

#[cfg(not(windows))]
fn retrying<T>(op: impl FnOnce() -> io::Result<T>) -> io::Result<T> {
    op()
}

#[cfg(test)]
mod tests {
    use super::*;

    // The names the next writes would try are taken; the write skips them
    // and leaves each untouched.
    #[test]
    fn a_taken_temporary_name_is_skipped() {
        let dir = std::env::temp_dir().join(format!("apply-fs-{}", std::process::id()));
        fs::create_dir_all(&dir).unwrap();
        let target = dir.join("AGENTS.md");
        let next = TEMP_COUNTER.load(Ordering::Relaxed);
        for n in next..next + 3 {
            fs::write(dir.join(temp_name("AGENTS.md".as_ref(), n)), b"mine\n").unwrap();
        }
        RealFs.write(&target, b"new\n", false).unwrap();
        assert_eq!(fs::read(&target).unwrap(), b"new\n");
        for n in next..next + 3 {
            let taken = dir.join(temp_name("AGENTS.md".as_ref(), n));
            assert_eq!(fs::read(taken).unwrap(), b"mine\n");
        }
        fs::remove_dir_all(&dir).unwrap();
    }

    // A replacement for a private file is never readable by others, even
    // before its bytes are written.
    #[cfg(unix)]
    #[test]
    fn a_temporary_file_is_private_before_it_is_written() {
        use std::os::unix::fs::PermissionsExt;
        let dir = std::env::temp_dir().join(format!("apply-fs-mode-{}", std::process::id()));
        fs::create_dir_all(&dir).unwrap();
        let target = dir.join("AGENTS.md");
        fs::write(&target, b"secret\n").unwrap();
        fs::set_permissions(&target, fs::Permissions::from_mode(0o600)).unwrap();
        let (temp, _file, _) = create_temp(&target).unwrap();
        assert_eq!(fs::metadata(&temp).unwrap().permissions().mode() & 0o077, 0);
        fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn only_temporary_names_for_the_file_are_removed() {
        let dir = std::env::temp_dir().join(format!("apply-fs-temps-{}", std::process::id()));
        fs::create_dir_all(&dir).unwrap();
        for name in [
            ".SKILL.md.syllago-12-3.tmp",
            ".SKILL.md.syllago-tmp",
            ".SKILL.md.syllago-x-3.tmp",
            ".OTHER.md.syllago-12-3.tmp",
            "SKILL.md",
        ] {
            fs::write(dir.join(name), b"x").unwrap();
        }
        RealFs.remove_temps(&dir.join("SKILL.md")).unwrap();
        let mut left: Vec<_> = fs::read_dir(&dir)
            .unwrap()
            .map(|e| e.unwrap().file_name().into_string().unwrap())
            .collect();
        left.sort();
        assert_eq!(
            left,
            [
                ".OTHER.md.syllago-12-3.tmp",
                ".SKILL.md.syllago-tmp",
                ".SKILL.md.syllago-x-3.tmp",
                "SKILL.md"
            ]
        );
        fs::remove_dir_all(&dir).unwrap();
    }
}
