//! Entry-point rows and their resolution (install-targets §6–§11).
//!
//! A row's enum fields are kept as the strings the publisher wrote, because a
//! consumer reads an unrecognized value as "unknown/other" rather than
//! rejecting the document: a row whose `scope` is unrecognized matches no
//! requested scope, an unrecognized `os` member matches no target OS, and a
//! write row whose `layout` or `status` is unrecognized is refused. InstallResolution
//! yields anchored POSIX paths; [`Target::native`] maps one to the
//! filesystem, so the same rows give byte-identical results on every OS.

use std::path::{Path, PathBuf};

use serde_json::Value;

use crate::{Diagnostic, Error, malformed};

/// An install scope (§7).
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum Scope {
    User,
    Project,
    Managed,
}

impl Scope {
    pub fn as_str(self) -> &'static str {
        match self {
            Scope::User => "user",
            Scope::Project => "project",
            Scope::Managed => "managed",
        }
    }
}

/// A target OS, one member of the closed OS enum.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum Os {
    Darwin,
    Linux,
    Windows,
}

impl Os {
    pub const ALL: [Os; 3] = [Os::Darwin, Os::Linux, Os::Windows];

    pub fn as_str(self) -> &'static str {
        match self {
            Os::Darwin => "darwin",
            Os::Linux => "linux",
            Os::Windows => "windows",
        }
    }
}

/// A layout (§9).
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum Layout {
    /// One item's file.
    SingleFile,
    /// One item's directory root.
    DirectoryOfFiles,
    /// A provider-owned file that items contribute to (§10).
    MergedIntoSharedFile,
}

impl Layout {
    pub fn parse(value: &str) -> Option<Layout> {
        Some(match value {
            "single_file" => Layout::SingleFile,
            "directory_of_files" => Layout::DirectoryOfFiles,
            "merged_into_shared_file" => Layout::MergedIntoSharedFile,
            _ => return None,
        })
    }

    pub fn as_str(self) -> &'static str {
        match self {
            Layout::SingleFile => "single_file",
            Layout::DirectoryOfFiles => "directory_of_files",
            Layout::MergedIntoSharedFile => "merged_into_shared_file",
        }
    }
}

/// One entry-point row, as published (§6).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct EntryPoint {
    pub scope: String,
    /// `None` when the row applies on every OS.
    pub os: Option<Vec<String>>,
    pub path_template: String,
    pub layout: String,
    pub dialect: Option<String>,
    pub status: String,
    pub as_of: Option<String>,
}

impl EntryPoint {
    /// Reads a row from its JSON form. Keys outside the row fields are
    /// ignored, as a consumer ignores an additive field; a required field
    /// that is missing or not a string, and an `os` that is null, empty or
    /// not a list of strings, are malformed, because either would read as
    /// "every OS".
    pub fn from_value(row: &Value) -> Result<EntryPoint, Error> {
        let row = row
            .as_object()
            .ok_or_else(|| malformed("an entry-point row is not an object"))?;
        let required = |name: &str| {
            row.get(name)
                .and_then(Value::as_str)
                .map(str::to_string)
                .ok_or_else(|| malformed(format!("entry-point row field {name} is missing")))
        };
        let optional = |name: &str| match row.get(name) {
            None => Ok(None),
            Some(Value::String(value)) => Ok(Some(value.clone())),
            Some(_) => Err(malformed(format!(
                "entry-point row field {name} is not a string"
            ))),
        };
        let os = match row.get("os") {
            None => None,
            Some(Value::Array(members)) if !members.is_empty() => Some(
                members
                    .iter()
                    .map(|m| m.as_str().map(str::to_string))
                    .collect::<Option<Vec<_>>>()
                    .ok_or_else(|| malformed("entry-point row os holds a non-string"))?,
            ),
            Some(_) => return Err(malformed("entry-point row os is not a non-empty list")),
        };
        Ok(EntryPoint {
            scope: required("scope")?,
            os,
            path_template: required("path_template")?,
            layout: required("layout")?,
            dialect: optional("dialect")?,
            status: required("status")?,
            as_of: optional("as_of")?,
        })
    }

    fn applies_on(&self, os: Os) -> bool {
        self.os
            .as_ref()
            .is_none_or(|members| members.iter().any(|m| m == os.as_str()))
    }
}

/// What a resolved path is relative to (§8.3).
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum Anchor {
    /// The user's home directory (`~/`).
    Home,
    /// The user's roaming application-data directory (`<appdata>/`).
    AppData,
    /// The project root.
    Project,
    /// An absolute managed location.
    Absolute,
}

/// A resolved entry point.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Target {
    pub scope: Scope,
    pub anchor: Anchor,
    /// The template with `<content-name>` substituted, `/`-separated and
    /// still carrying its anchor (`~/`, `<appdata>/`), so it is the same
    /// string on every OS.
    pub path: String,
    pub layout: Layout,
    /// The row is `status: superseded`.
    pub superseded: bool,
}

/// The directories an anchored path resolves against.
#[derive(Debug, Clone, Copy)]
pub struct Roots<'a> {
    pub home: &'a Path,
    pub appdata: Option<&'a Path>,
    pub project: Option<&'a Path>,
}

impl Target {
    /// The path with its anchor and without a directory's trailing `/`, for
    /// comparing two targets.
    pub fn location(&self) -> &str {
        self.path.strip_suffix('/').unwrap_or(&self.path)
    }

    /// The native path: the anchor's root with each segment joined, so the
    /// native separator is used. A managed path is used as written, with
    /// its `/` separators made native.
    pub fn native(&self, roots: &Roots<'_>) -> Result<PathBuf, Error> {
        let (root, rest) = match self.anchor {
            Anchor::Home => (roots.home, &self.path[2..]),
            Anchor::AppData => (
                roots
                    .appdata
                    .ok_or_else(|| malformed("no application-data directory was supplied"))?,
                &self.path["<appdata>/".len()..],
            ),
            Anchor::Project => (
                roots
                    .project
                    .ok_or_else(|| malformed("no project root was supplied"))?,
                self.path.as_str(),
            ),
            Anchor::Absolute => {
                return Ok(PathBuf::from(
                    self.path.replace('/', std::path::MAIN_SEPARATOR_STR),
                ));
            }
        };
        Ok(rest
            .split('/')
            .filter(|segment| !segment.is_empty())
            .fold(root.to_path_buf(), |path, segment| path.join(segment)))
    }
}

/// A resolution for one scope: the write target and every location the
/// provider reads content of the type from at that scope.
#[derive(Debug, Clone, PartialEq)]
pub struct InstallResolution {
    pub write: Target,
    /// Every row for the scope that survives OS filtering and resolves, in
    /// published order, the write target included. A row whose template
    /// carries an unrecognized token serves no location.
    pub reads: Vec<Target>,
    /// `acif.install.entry_point_superseded` when the write target is a
    /// superseded row.
    pub diagnostics: Vec<Diagnostic>,
}

/// Resolves `rows` for `scope` on `os` with `content_name` (§6, §8, §11).
///
/// The content name is checked before anything else. Rows are filtered by
/// OS; with none left the result is `acif.install.no_entry_point`, and with
/// none for `scope`, `acif.install.scope_unavailable` naming the scopes that
/// have rows. The write target is the first current row for the scope, or
/// the first superseded one with `acif.install.entry_point_superseded` when
/// none is current; a write row carrying a token outside the grammar is
/// `acif.install.placeholder_unrecognized`, and no later row is tried.
pub fn resolve(
    rows: &[EntryPoint],
    scope: Scope,
    os: Os,
    content_name: &str,
) -> Result<InstallResolution, Error> {
    check_content_name(content_name)?;
    let rows: Vec<&EntryPoint> = rows.iter().filter(|row| row.applies_on(os)).collect();
    if rows.is_empty() {
        return Err(refuse("acif.install.no_entry_point", &[]));
    }
    let in_scope: Vec<&EntryPoint> = rows
        .iter()
        .copied()
        .filter(|row| row.scope == scope.as_str())
        .collect();
    if in_scope.is_empty() {
        let mut available: Vec<&str> = Vec::new();
        for row in &rows {
            if !available.contains(&row.scope.as_str()) {
                available.push(&row.scope);
            }
        }
        return Err(refuse(
            "acif.install.scope_unavailable",
            &[("available_scopes", Value::from(available))],
        ));
    }
    let write_row = in_scope
        .iter()
        .find(|row| row.status == "current")
        .or_else(|| in_scope.iter().find(|row| row.status == "superseded"))
        .ok_or_else(|| {
            Error::Unsupported(format!("no {} row has a recognized status", scope.as_str()))
        })?;
    let write = resolve_row(write_row, scope, os, content_name)?;
    let mut diagnostics = Vec::new();
    if write.superseded {
        diagnostics.push(Diagnostic::new(
            "acif.install.entry_point_superseded",
            &[(
                "path_template",
                Value::from(write_row.path_template.as_str()),
            )],
        ));
    }
    let mut reads = Vec::new();
    for row in in_scope {
        match resolve_row(row, scope, os, content_name) {
            Ok(target) => reads.push(target),
            Err(Error::Refuse(_) | Error::Unsupported(_)) => {}
            Err(error) => return Err(error),
        }
    }
    Ok(InstallResolution {
        write,
        reads,
        diagnostics,
    })
}

/// The §8.2 validity predicate.
fn check_content_name(name: &str) -> Result<(), Error> {
    if name.is_empty() || name.contains(['/', '\\', '\0']) || name == "." || name == ".." {
        return Err(Error::Reject("acif.install.content_name_invalid"));
    }
    Ok(())
}

fn resolve_row(row: &EntryPoint, scope: Scope, os: Os, name: &str) -> Result<Target, Error> {
    let template = row.path_template.as_str();
    let (anchor, rest) = if let Some(rest) = template.strip_prefix("~/") {
        (Anchor::Home, rest)
    } else if let Some(rest) = template.strip_prefix("<appdata>/") {
        if row.os.as_deref() != Some(&["windows".to_string()][..]) || os != Os::Windows {
            return Err(malformed(format!(
                "{template}: <appdata> is allowed only on a windows-only row"
            )));
        }
        (Anchor::AppData, rest)
    } else if scope == Scope::Managed {
        if !is_absolute(template) {
            return Err(malformed(format!(
                "{template}: a managed template must be absolute"
            )));
        }
        (Anchor::Absolute, template)
    } else {
        if is_absolute(template) {
            return Err(malformed(format!(
                "{template}: only a managed template may be absolute"
            )));
        }
        (Anchor::Project, template)
    };
    let mut path = String::with_capacity(template.len() + name.len());
    path.push_str(&template[..template.len() - rest.len()]);
    let segments: Vec<&str> = rest.split('/').collect();
    for (i, segment) in segments.iter().enumerate() {
        let last = i == segments.len() - 1;
        let substituted = substitute(segment, name)?;
        // An empty segment is allowed only as a directory's trailing `/`
        // and before the root `/` of an absolute path.
        let root = i == 0 && anchor == Anchor::Absolute;
        if (substituted.is_empty() && !(last && i > 0) && !root)
            || substituted == "."
            || substituted == ".."
            || substituted == "~"
            || substituted.contains('\\')
        {
            return Err(malformed(format!(
                "{template}: the segment {segment:?} is not a path segment"
            )));
        }
        path.push_str(&substituted);
        if !last {
            path.push('/');
        }
    }
    let layout = Layout::parse(&row.layout)
        .ok_or_else(|| Error::Unsupported(format!("{template}: layout {:?}", row.layout)))?;
    let has_name = template.contains("<content-name>");
    if has_name == (layout == Layout::MergedIntoSharedFile) {
        return Err(malformed(format!(
            "{template}: layout {} disagrees with <content-name>",
            layout.as_str()
        )));
    }
    Ok(Target {
        scope,
        anchor,
        path,
        layout,
        superseded: row.status == "superseded",
    })
}

/// Substitutes `<content-name>` in one segment. Any other `<`…`>` token, or
/// an unmatched `<` or `>`, is outside the closed grammar.
fn substitute(segment: &str, name: &str) -> Result<String, Error> {
    let mut out = String::new();
    let mut rest = segment;
    while let Some(open) = rest.find(['<', '>']) {
        out.push_str(&rest[..open]);
        let token_end = rest[open..].find('>').map(|close| open + close + 1);
        match token_end {
            Some(end) if rest.as_bytes()[open] == b'<' && &rest[open..end] == "<content-name>" => {
                out.push_str(name);
                rest = &rest[end..];
            }
            _ => {
                return Err(refuse(
                    "acif.install.placeholder_unrecognized",
                    &[("segment", Value::from(segment))],
                ));
            }
        }
    }
    out.push_str(rest);
    Ok(out)
}

fn is_absolute(template: &str) -> bool {
    let bytes = template.as_bytes();
    template.starts_with('/')
        || (bytes.len() >= 3 && bytes[0].is_ascii_alphabetic() && &bytes[1..3] == b":/")
}

fn refuse(id: &'static str, params: &[(&str, Value)]) -> Error {
    Error::Refuse(Diagnostic::new(id, params))
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn rows(value: Value) -> Vec<EntryPoint> {
        value
            .as_array()
            .unwrap()
            .iter()
            .map(|row| EntryPoint::from_value(row).unwrap())
            .collect()
    }

    fn refusal(result: Result<InstallResolution, Error>) -> (&'static str, Value) {
        match result.unwrap_err() {
            Error::Refuse(d) => (d.id, Value::Object(d.params)),
            Error::Reject(id) => (id, json!({})),
            other => panic!("{other}"),
        }
    }

    fn row(scope: &str, template: &str, layout: &str, status: &str) -> Value {
        json!({"scope": scope, "path_template": template, "layout": layout, "status": status})
    }

    // TV-INSTALL-a: multiplicity, order and the first current row per scope.
    #[test]
    fn first_current_row_is_the_write_target_and_every_row_is_read() {
        let rows = rows(json!([
            row(
                "user",
                "~/.github/agents/<content-name>.md",
                "single_file",
                "current"
            ),
            row(
                "project",
                ".copilot/agents/<content-name>.md",
                "single_file",
                "current"
            ),
            row(
                "project",
                ".github/agents/<content-name>.md",
                "single_file",
                "current"
            ),
        ]));
        let user = resolve(&rows, Scope::User, Os::Linux, "reviewer").unwrap();
        assert_eq!(user.write.path, "~/.github/agents/reviewer.md");
        assert_eq!(user.write.anchor, Anchor::Home);
        let project = resolve(&rows, Scope::Project, Os::Linux, "reviewer").unwrap();
        assert_eq!(project.write.path, ".copilot/agents/reviewer.md");
        let reads: Vec<_> = project.reads.iter().map(|t| t.path.as_str()).collect();
        assert_eq!(
            reads,
            [".copilot/agents/reviewer.md", ".github/agents/reviewer.md"]
        );
        assert!(project.diagnostics.is_empty());
    }

    // TV-INSTALL-c.
    #[test]
    fn invalid_content_names_reject_before_resolution() {
        let rows = rows(json!([row(
            "user",
            "~/.claude/commands/<content-name>.md",
            "single_file",
            "current"
        )]));
        for name in ["a/b", "a\\b", "..", ".", "", "a\0b"] {
            let result = resolve(&rows, Scope::User, Os::Linux, name);
            assert_eq!(refusal(result).0, "acif.install.content_name_invalid");
        }
        // Validity is checked even when no row exists.
        assert_eq!(
            refusal(resolve(&[], Scope::User, Os::Linux, "a/b")).0,
            "acif.install.content_name_invalid"
        );
    }

    // TV-INSTALL-d.
    #[test]
    fn unrecognized_token_refuses_the_write_row_only() {
        let unknown = rows(json!([row(
            "user",
            "~/.claude/commands/<unknown-variant>/<content-name>.md",
            "single_file",
            "current"
        )]));
        assert_eq!(
            refusal(resolve(&unknown, Scope::User, Os::Linux, "deploy")).0,
            "acif.install.placeholder_unrecognized"
        );
        for template in ["~/a/<content-name", "~/a/content-name>.md", "~/<appdata>/x"] {
            let rows = rows(json!([row(
                "user",
                template,
                "merged_into_shared_file",
                "current"
            )]));
            assert_eq!(
                refusal(resolve(&rows, Scope::User, Os::Linux, "x")).0,
                "acif.install.placeholder_unrecognized",
                "{template}"
            );
        }
        // A later row with an unknown token serves no read location.
        let rows = rows(json!([
            row("user", "~/a/<content-name>.md", "single_file", "current"),
            row(
                "user",
                "~/b/<other>/<content-name>.md",
                "single_file",
                "current"
            ),
        ]));
        let resolution = resolve(&rows, Scope::User, Os::Linux, "x").unwrap();
        assert_eq!(resolution.reads.len(), 1);
    }

    // TV-INSTALL-e.
    #[test]
    fn disposition_lanes() {
        assert_eq!(
            refusal(resolve(&[], Scope::User, Os::Linux, "guard")).0,
            "acif.install.no_entry_point"
        );
        let project_only = rows(json!([row(
            "project",
            ".kiro/steering/<content-name>.md",
            "single_file",
            "current"
        )]));
        assert_eq!(
            refusal(resolve(&project_only, Scope::User, Os::Linux, "style")),
            (
                "acif.install.scope_unavailable",
                json!({"available_scopes": ["project"]})
            )
        );
        let superseded = rows(json!([row(
            "user",
            "~/.claude/commands/<content-name>.md",
            "single_file",
            "superseded"
        )]));
        let resolution = resolve(&superseded, Scope::User, Os::Linux, "deploy").unwrap();
        assert_eq!(resolution.write.path, "~/.claude/commands/deploy.md");
        assert!(resolution.write.superseded);
        assert_eq!(
            resolution.diagnostics[0].id,
            "acif.install.entry_point_superseded"
        );
        // A current row wins over an earlier superseded one.
        let both = rows(json!([
            row(
                "user",
                "~/old/<content-name>.md",
                "single_file",
                "superseded"
            ),
            row("user", "~/new/<content-name>.md", "single_file", "current"),
        ]));
        let resolution = resolve(&both, Scope::User, Os::Linux, "x").unwrap();
        assert_eq!(resolution.write.path, "~/new/x.md");
        assert!(resolution.diagnostics.is_empty());
    }

    // TV-INSTALL-f and TV-INSTALL-g.
    #[test]
    fn anchors_and_os_filtering() {
        let rows = rows(json!([
            {"scope": "user", "os": ["darwin", "linux"], "path_template": "~/.config/devin/rules/<content-name>.md", "layout": "single_file", "status": "current"},
            {"scope": "user", "os": ["windows"], "path_template": "<appdata>/devin/rules/<content-name>.md", "layout": "single_file", "status": "current"},
            {"scope": "managed", "os": ["linux"], "path_template": "/etc/devin/<content-name>.md", "layout": "single_file", "status": "current"},
            {"scope": "managed", "os": ["windows"], "path_template": "C:/ProgramData/Devin/<content-name>.md", "layout": "single_file", "status": "current"},
            row("project", ".devin/rules/<content-name>.md", "single_file", "current"),
        ]));
        let home = Path::new("home").join("dev");
        let appdata = Path::new("appdata");
        let project = Path::new("work").join("repo");
        let roots = Roots {
            home: &home,
            appdata: Some(appdata),
            project: Some(&project),
        };
        let linux = resolve(&rows, Scope::User, Os::Linux, "style")
            .unwrap()
            .write;
        assert_eq!(linux.path, "~/.config/devin/rules/style.md");
        assert_eq!(
            linux.native(&roots).unwrap(),
            home.join(".config")
                .join("devin")
                .join("rules")
                .join("style.md")
        );
        let windows = resolve(&rows, Scope::User, Os::Windows, "style")
            .unwrap()
            .write;
        assert_eq!(windows.anchor, Anchor::AppData);
        assert_eq!(
            windows.native(&roots).unwrap(),
            appdata.join("devin").join("rules").join("style.md")
        );
        let managed = resolve(&rows, Scope::Managed, Os::Linux, "style")
            .unwrap()
            .write;
        assert_eq!(
            (managed.anchor, managed.path.as_str()),
            (Anchor::Absolute, "/etc/devin/style.md")
        );
        let managed = resolve(&rows, Scope::Managed, Os::Windows, "style")
            .unwrap()
            .write;
        assert_eq!(managed.path, "C:/ProgramData/Devin/style.md");
        assert_eq!(
            refusal(resolve(&rows, Scope::Managed, Os::Darwin, "style")),
            (
                "acif.install.scope_unavailable",
                json!({"available_scopes": ["user", "project"]})
            )
        );
        let project_target = resolve(&rows, Scope::Project, Os::Darwin, "style")
            .unwrap()
            .write;
        assert_eq!(
            project_target.native(&roots).unwrap(),
            project.join(".devin").join("rules").join("style.md")
        );
    }

    #[test]
    fn directory_targets_keep_their_slash_and_compare_without_it() {
        let rows = rows(json!([row(
            "user",
            "~/.agents/skills/<content-name>/",
            "directory_of_files",
            "current"
        )]));
        let target = resolve(&rows, Scope::User, Os::Windows, "review")
            .unwrap()
            .write;
        assert_eq!(target.path, "~/.agents/skills/review/");
        assert_eq!(target.location(), "~/.agents/skills/review");
        assert_eq!(target.layout, Layout::DirectoryOfFiles);
    }

    #[test]
    fn templates_outside_the_grammar_are_malformed() {
        for (scope, template, layout) in [
            ("user", "~/a/../<content-name>.md", "single_file"),
            ("user", "~/a//<content-name>.md", "single_file"),
            ("user", "~/a\\<content-name>.md", "single_file"),
            ("user", "~/a/~/<content-name>.md", "single_file"),
            ("project", "/abs/<content-name>.md", "single_file"),
            ("project", "C:/abs/<content-name>.md", "single_file"),
            ("managed", "relative/<content-name>.md", "single_file"),
            ("user", "<appdata>/x/<content-name>.md", "single_file"),
            ("user", "~/AGENTS.md", "single_file"),
            ("user", "~/<content-name>.md", "merged_into_shared_file"),
        ] {
            let rows = rows(json!([row(scope, template, layout, "current")]));
            let scope = match scope {
                "user" => Scope::User,
                "project" => Scope::Project,
                _ => Scope::Managed,
            };
            let err = resolve(&rows, scope, Os::Windows, "x").unwrap_err();
            assert!(matches!(err, Error::Malformed(_)), "{template}: {err}");
        }
    }

    #[test]
    fn unrecognized_values_are_unknown_not_errors() {
        let rows = rows(json!([
            {"scope": "workspace", "path_template": "w/<content-name>.md", "layout": "single_file", "status": "current"},
            {"scope": "user", "os": ["plan9"], "path_template": "~/p/<content-name>.md", "layout": "single_file", "status": "current"},
            {"scope": "user", "path_template": "~/q/<content-name>.md", "layout": "single_file", "status": "draft", "extra": 1},
            row("user", "~/r/<content-name>.md", "nested", "current"),
        ]));
        assert_eq!(
            refusal(resolve(&rows[..1], Scope::User, Os::Linux, "x")),
            (
                "acif.install.scope_unavailable",
                json!({"available_scopes": ["workspace"]})
            )
        );
        assert_eq!(
            refusal(resolve(&rows[1..2], Scope::User, Os::Linux, "x")).0,
            "acif.install.no_entry_point"
        );
        assert!(matches!(
            resolve(&rows[2..3], Scope::User, Os::Linux, "x"),
            Err(Error::Unsupported(_))
        ));
        assert!(matches!(
            resolve(&rows[3..], Scope::User, Os::Linux, "x"),
            Err(Error::Unsupported(_))
        ));
    }

    #[test]
    fn malformed_rows_are_refused() {
        for bad in [
            json!("row"),
            json!({"scope": "user", "layout": "single_file", "status": "current"}),
            json!({"scope": "user", "path_template": 5, "layout": "single_file", "status": "current"}),
            json!({"scope": "user", "os": [], "path_template": "~/x", "layout": "single_file", "status": "current"}),
            json!({"scope": "user", "os": null, "path_template": "~/x", "layout": "single_file", "status": "current"}),
            json!({"scope": "user", "os": [1], "path_template": "~/x", "layout": "single_file", "status": "current"}),
            json!({"scope": "user", "path_template": "~/x", "layout": "single_file", "status": "current", "as_of": 3}),
        ] {
            assert!(EntryPoint::from_value(&bad).is_err(), "{bad}");
        }
    }
}
