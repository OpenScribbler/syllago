package installer

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// InstallMethod controls how content is placed in the target directory.
type InstallMethod string

const (
	MethodSymlink InstallMethod = "symlink"
	MethodCopy    InstallMethod = "copy"
	// MethodAppend routes rule installs to InstallRuleAppend, which appends
	// the rule's canonical body to a monolithic file (e.g. CLAUDE.md) rather
	// than placing a file in a file-rules directory (D5). Only valid for
	// catalog.Rules on providers whose slug has a MonolithicFilenames entry.
	MethodAppend InstallMethod = "append"
)

const (
	MechanismSymlink   = "symlink"
	MechanismCopy      = "copy"
	MechanismHookMerge = "hook_merge"
	MechanismMCPMerge  = "mcp_merge"
	// MechanismRuleAppend is a rule appended to a monolithic rule file.
	// InstallRuleAppend returns no Placement; callers that record it build
	// one with this mechanism and the target file as Path.
	MechanismRuleAppend = "rule_append"
)

// Placement describes where an install (or uninstall) acted: the mechanism
// used, the filesystem destination, and — for JSON merges — the keys touched.
// String() renders the same human-readable description these functions used
// to return, so display call sites keep their exact output.
type Placement struct {
	Mechanism string   // "symlink", "copy", "hook_merge", or "mcp_merge"
	Path      string   // destination path (symlink/copy) or settings file (merges)
	Keys      []string // full JSON keys for merges (e.g. "hooks.PreToolUse", "mcpServers.github"); nil otherwise
	Notices   []Notice // what the install has to tell the user; set even when it fails
	desc      string   // exact legacy display string
}

func (p Placement) String() string { return p.desc }

// Status represents the install status of an item for a provider.
type Status int

const (
	StatusNotAvailable Status = iota // provider doesn't support this content type
	StatusNotInstalled               // available but not installed
	StatusInstalled                  // installed (symlink points to our repo)
)

func (s Status) String() string {
	switch s {
	case StatusNotAvailable:
		return "[-]"
	case StatusNotInstalled:
		return "[--]"
	case StatusInstalled:
		return "[ok]"
	}
	return "[?]"
}

// IsJSONMerge returns true if the provider uses JSON merge for the given content type.
// Hooks always do: every hook install encodes through the provider's
// HookAdapter, including Pi's extension file, and a provider with no adapter
// refuses the install rather than receiving the Library folder unconverted.
func IsJSONMerge(prov provider.Provider, itemType catalog.ContentType) bool {
	if itemType == catalog.Hooks {
		return true
	}
	var resolver *config.PathResolver
	return resolver.InstallDir(prov, itemType, "") == provider.JSONMergeSentinel
}

// resolveTargetWithBase computes the target path using a specific base directory.
// Returns an error if the provider doesn't support the content type or uses JSON merge.
func resolveTargetWithBase(item catalog.ContentItem, prov provider.Provider, baseDir string) (string, error) {
	return targetIn(prov.InstallDir(baseDir, item.Type), item, prov)
}

// targetIn returns the item's path inside installDir, or an error when
// installDir says prov does not install the item's type on the filesystem.
func targetIn(installDir string, item catalog.ContentItem, prov provider.Provider) (string, error) {
	if installDir == "" {
		return "", fmt.Errorf("%s does not support %s", prov.Name, item.Type.Label())
	}
	if installDir == provider.JSONMergeSentinel {
		return "", fmt.Errorf("%s uses JSON merge for %s (not filesystem install)", prov.Name, item.Type.Label())
	}
	if installDir == provider.ProjectScopeSentinel {
		return "", fmt.Errorf("%s %s is project-scoped (use export with --to from within a project directory)", prov.Name, item.Type.Label())
	}
	if item.Type == catalog.Agents {
		return agentTargetPath(installDir, item, prov), nil
	}
	if item.Type.IsUniversal() {
		return filepath.Join(installDir, item.Name), nil
	}
	return filepath.Join(installDir, filepath.Base(item.Path)), nil
}

// resolveTarget computes the target path for an item in a provider's install directory.
// Uses the user's home directory as the base.
func resolveTarget(item catalog.ContentItem, prov provider.Provider) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return resolveTargetWithBase(item, prov, home)
}

// CheckStatus checks whether an item is installed for a given provider.
// registryPaths contains additional valid symlink source roots (registry cache directories).
func CheckStatus(item catalog.ContentItem, prov provider.Provider, repoRoot string, registryPaths ...string) Status {
	status, _ := StatusOf(item, prov, repoRoot, registryPaths...)
	return status
}

// StatusOf is CheckStatus that also reports when it could not tell. A
// non-nil error means a file the check needed could not be read or parsed,
// so the returned Status is a guess; a caller about to delete the item
// treats that provider as unknown rather than as not installed.
func StatusOf(item catalog.ContentItem, prov provider.Provider, repoRoot string, registryPaths ...string) (Status, error) {
	// Dispatch to JSON merge handlers for types that need it
	if IsJSONMerge(prov, item.Type) {
		switch item.Type {
		case catalog.MCP:
			return mcpStatus(item, prov, repoRoot)
		case catalog.Hooks:
			return hookStatus(item, prov, repoRoot)
		}
		return StatusNotAvailable, nil
	}

	targetPath, err := resolveTarget(item, prov)
	if err != nil {
		return StatusNotAvailable, nil
	}
	if home, homeErr := os.UserHomeDir(); homeErr == nil {
		lp, _, ok, err := legacyTargetChecked(item, prov, home, targetPath)
		if err != nil {
			return StatusNotInstalled, err
		}
		if ok {
			targetPath = lp
		}
	}

	allRoots := append([]string{repoRoot}, registryPaths...)
	if IsSymlinkedToAny(targetPath, allRoots) {
		return StatusInstalled, nil
	}

	// Also check if target exists as a regular file (e.g., installed via copy)
	if _, err := os.Lstat(targetPath); err == nil {
		return StatusInstalled, nil
	} else if lstatUnreadable(err) {
		return StatusNotInstalled, err
	}

	return StatusNotInstalled, nil
}

// lstatUnreadable reports whether err from os.Lstat leaves it unknown
// whether the path exists. A missing path, or a file where a parent
// directory should be, means nothing is there.
func lstatUnreadable(err error) bool {
	return err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR)
}

// CheckStatusWithResolver checks whether an item is installed, using the resolver
// for path resolution instead of the default home directory.
func CheckStatusWithResolver(item catalog.ContentItem, prov provider.Provider, repoRoot string, resolver *config.PathResolver, registryPaths ...string) Status {
	// Dispatch to JSON merge handlers for types that need it
	if IsJSONMerge(prov, item.Type) {
		switch item.Type {
		case catalog.MCP:
			return checkMCPStatus(item, prov, repoRoot)
		case catalog.Hooks:
			return checkHookStatus(item, prov, repoRoot)
		}
		return StatusNotAvailable
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return StatusNotAvailable
	}

	installDir := resolver.InstallDir(prov, item.Type, home)
	if installDir == "" || installDir == provider.JSONMergeSentinel || installDir == provider.ProjectScopeSentinel {
		return StatusNotAvailable
	}

	// Compute target path directly from the resolved install dir.
	var targetPath string
	if item.Type == catalog.Agents {
		targetPath = agentTargetPath(installDir, item, prov)
	} else if item.Type.IsUniversal() {
		targetPath = filepath.Join(installDir, item.Name)
	} else {
		targetPath = filepath.Join(installDir, filepath.Base(item.Path))
	}
	if lp, _, ok := legacyTarget(item, prov, home, targetPath); ok {
		targetPath = lp
	}

	allRoots := append([]string{repoRoot}, registryPaths...)
	if IsSymlinkedToAny(targetPath, allRoots) {
		return StatusInstalled
	}

	if _, err := os.Lstat(targetPath); err == nil {
		return StatusInstalled
	}

	return StatusNotInstalled
}

// Install places the given item under the provider's install directory.
// For JSON merge types (MCP, hooks), it merges into the provider's config file.
// For filesystem types, it creates a symlink or copy depending on the method.
// baseDir overrides the home directory as the install root. If empty, uses home dir.
// scan configures the hook scanner chain; other content types ignore it.
// Returns placement metadata; Placement.String renders the legacy description.
func Install(item catalog.ContentItem, prov provider.Provider, repoRoot string, method InstallMethod, baseDir string, scan ScanOptions) (Placement, error) {
	// Dispatch to JSON merge handlers for types that need it
	if IsJSONMerge(prov, item.Type) {
		switch item.Type {
		case catalog.MCP:
			return installMCP(item, prov, repoRoot)
		case catalog.Hooks:
			return installHook(item, prov, repoRoot, scan)
		}
		return Placement{}, fmt.Errorf("%s does not support %s via JSON merge", prov.Name, item.Type.Label())
	}

	// Resolve target path using baseDir or home dir
	resolveTarget := func() (string, error) {
		if baseDir != "" {
			return resolveTargetWithBase(item, prov, baseDir)
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home directory: %w", err)
		}
		return resolveTargetWithBase(item, prov, home)
	}

	if item.Type == catalog.Agents {
		targetPath, err := resolveTarget()
		if err != nil {
			return Placement{}, err
		}
		return installAgent(item, prov, targetPath)
	}

	// Check for cross-provider rendering via converter
	if conv := converter.For(item.Type); conv != nil {
		// Source provider differs from target → render from canonical
		if item.Provider != "" && item.Provider != prov.Slug {
			targetPath, err := resolveTarget()
			if err != nil {
				return Placement{}, err
			}
			installedPath, notices, err := installWithRenderTo(item, prov, conv, filepath.Dir(targetPath))
			return Placement{Mechanism: MechanismCopy, Path: installedPath, Notices: notices, desc: installedPath}, err
		}
		// Same provider + has .source/ → use original for lossless install
		if converter.HasSourceFile(item) && item.Provider == prov.Slug {
			targetPath, err := resolveTarget()
			if err != nil {
				return Placement{}, err
			}
			installedPath, err := installFromSourceTo(item, prov, filepath.Dir(targetPath))
			return Placement{Mechanism: MechanismCopy, Path: installedPath, desc: installedPath}, err
		}
	}

	targetPath, err := resolveTarget()
	if err != nil {
		return Placement{}, err
	}

	sourcePath := SourcePathFor(item)

	notices := placedNotices(item, prov)
	switch method {
	case MethodCopy:
		return Placement{Mechanism: MechanismCopy, Path: targetPath, Notices: notices, desc: targetPath}, CopyContent(sourcePath, targetPath)
	default:
		if IsWindowsMount(targetPath) {
			note := Notice{Kind: NoticeNote, Message: targetPath + " is on a Windows mount, using copy instead of symlink"}
			return Placement{Mechanism: MechanismCopy, Path: targetPath, Notices: append(notices, note), desc: targetPath}, CopyContent(sourcePath, targetPath)
		}
		return Placement{Mechanism: MechanismSymlink, Path: targetPath, Notices: notices, desc: targetPath}, CreateSymlink(sourcePath, targetPath)
	}
}

// InstallWithResolver places the given item under the provider's install directory,
// using the resolver for path resolution instead of a flat baseDir string.
// This respects the full priority chain: per-type path > CLI --base-dir > config baseDir > default.
func InstallWithResolver(item catalog.ContentItem, prov provider.Provider, repoRoot string, method InstallMethod, resolver *config.PathResolver, scan ScanOptions) (Placement, error) {
	// Dispatch to JSON merge handlers for types that need it
	if IsJSONMerge(prov, item.Type) {
		switch item.Type {
		case catalog.MCP:
			return installMCP(item, prov, repoRoot)
		case catalog.Hooks:
			return installHook(item, prov, repoRoot, scan)
		}
		return Placement{}, fmt.Errorf("%s does not support %s via JSON merge", prov.Name, item.Type.Label())
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return Placement{}, fmt.Errorf("getting home directory: %w", err)
	}

	installDir := resolver.InstallDir(prov, item.Type, home)
	if installDir == "" {
		return Placement{}, fmt.Errorf("%s does not support %s", prov.Name, item.Type.Label())
	}
	if installDir == provider.JSONMergeSentinel {
		return Placement{}, fmt.Errorf("%s uses JSON merge for %s (not filesystem install)", prov.Name, item.Type.Label())
	}
	if installDir == provider.ProjectScopeSentinel {
		return Placement{}, fmt.Errorf("%s %s is project-scoped (use export with --to from within a project directory)", prov.Name, item.Type.Label())
	}

	// Compute target path from resolved install dir (same logic as CheckStatusWithResolver).
	var targetPath string
	if item.Type == catalog.Agents {
		targetPath = agentTargetPath(installDir, item, prov)
	} else if item.Type.IsUniversal() {
		targetPath = filepath.Join(installDir, item.Name)
	} else {
		targetPath = filepath.Join(installDir, filepath.Base(item.Path))
	}

	if item.Type == catalog.Agents {
		return installAgent(item, prov, targetPath)
	}

	// Check for cross-provider rendering via converter
	if conv := converter.For(item.Type); conv != nil {
		if item.Provider != "" && item.Provider != prov.Slug {
			installedPath, notices, err := installWithRenderTo(item, prov, conv, filepath.Dir(targetPath))
			return Placement{Mechanism: MechanismCopy, Path: installedPath, Notices: notices, desc: installedPath}, err
		}
		if converter.HasSourceFile(item) && item.Provider == prov.Slug {
			installedPath, err := installFromSourceTo(item, prov, filepath.Dir(targetPath))
			return Placement{Mechanism: MechanismCopy, Path: installedPath, desc: installedPath}, err
		}
	}

	sourcePath := SourcePathFor(item)

	notices := placedNotices(item, prov)
	switch method {
	case MethodCopy:
		return Placement{Mechanism: MechanismCopy, Path: targetPath, Notices: notices, desc: targetPath}, CopyContent(sourcePath, targetPath)
	default:
		if IsWindowsMount(targetPath) {
			note := Notice{Kind: NoticeNote, Message: targetPath + " is on a Windows mount, using copy instead of symlink"}
			return Placement{Mechanism: MechanismCopy, Path: targetPath, Notices: append(notices, note), desc: targetPath}, CopyContent(sourcePath, targetPath)
		}
		return Placement{Mechanism: MechanismSymlink, Path: targetPath, Notices: notices, desc: targetPath}, CreateSymlink(sourcePath, targetPath)
	}
}

// Uninstall removes the given item from the provider's default install
// directory under the home directory. See UninstallFrom.
func Uninstall(item catalog.ContentItem, prov provider.Provider, repoRoot string) (Placement, error) {
	return UninstallFrom(item, prov, repoRoot, "", nil)
}

// UninstallFrom removes the given item from where Install or
// InstallWithResolver placed it with the same baseDir or resolver. A
// non-nil resolver overrides baseDir, and an empty baseDir means the home
// directory.
// For JSON merge types, it removes the entry from the provider's config file
// and ignores baseDir and resolver, as Install does.
// For filesystem types, it removes the symlink or copy.
// Returns placement metadata; Placement.String renders the legacy description.
func UninstallFrom(item catalog.ContentItem, prov provider.Provider, repoRoot, baseDir string, resolver *config.PathResolver) (Placement, error) {
	// Dispatch to JSON merge handlers for types that need it
	if IsJSONMerge(prov, item.Type) {
		switch item.Type {
		case catalog.MCP:
			return uninstallMCP(item, prov, repoRoot)
		case catalog.Hooks:
			return uninstallHook(item, prov, repoRoot)
		}
		return Placement{}, fmt.Errorf("%s does not support %s via JSON merge", prov.Name, item.Type.Label())
	}

	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return Placement{}, fmt.Errorf("getting home directory: %w", homeErr)
	}
	var installDir string
	switch {
	case resolver != nil:
		installDir = resolver.InstallDir(prov, item.Type, home)
	case baseDir != "":
		installDir = prov.InstallDir(baseDir, item.Type)
	default:
		installDir = prov.InstallDir(home, item.Type)
	}
	targetPath, err := targetIn(installDir, item, prov)
	if err != nil {
		return Placement{}, err
	}
	if lp, ld, ok := legacyTarget(item, prov, home, targetPath); ok {
		targetPath, installDir = lp, ld
	}

	info, err := os.Lstat(targetPath)
	if err != nil {
		return Placement{}, fmt.Errorf("not installed: %s", targetPath)
	}

	// Remove symlinks only when they still point at this library item.
	if info.Mode()&os.ModeSymlink != 0 {
		actualTarget, err := resolveSymlinkTarget(targetPath)
		if err != nil {
			return Placement{}, fmt.Errorf("reading symlink %s: %w", targetPath, err)
		}
		if !symlinkTargetBelongsToItem(actualTarget, item) {
			return Placement{}, fmt.Errorf("refusing to remove %s: symlink points to %s, not to %s", targetPath, actualTarget, item.Path)
		}
		return Placement{Mechanism: MechanismSymlink, Path: targetPath, desc: targetPath}, os.Remove(targetPath)
	}

	// Remove regular files (copies)
	if info.Mode().IsRegular() {
		return Placement{Mechanism: MechanismCopy, Path: targetPath, desc: targetPath}, os.Remove(targetPath)
	}

	// Remove directories (copy-installed content)
	if info.IsDir() {
		// Verify targetPath is within the expected install directory to prevent
		// path traversal attacks from removing arbitrary directories.
		rel, relErr := filepath.Rel(installDir, targetPath)
		if relErr != nil || rel == "." || strings.HasPrefix(rel, "..") {
			return Placement{}, fmt.Errorf("refusing to remove %s: outside install directory %s", targetPath, installDir)
		}
		return Placement{Mechanism: MechanismCopy, Path: targetPath, desc: targetPath}, os.RemoveAll(targetPath)
	}

	return Placement{}, fmt.Errorf("unexpected file type at %s, remove manually", targetPath)
}

// legacyTarget returns the item's path under prov's legacy install directory
// for its type, and that directory, when nothing exists at targetPath and
// something exists at the legacy path.
func legacyTarget(item catalog.ContentItem, prov provider.Provider, home, targetPath string) (string, string, bool) {
	lp, dir, ok, _ := legacyTargetChecked(item, prov, home, targetPath)
	return lp, dir, ok
}

// legacyTargetChecked is legacyTarget that also returns the error when the
// legacy path could not be checked, so an install there may be unseen.
func legacyTargetChecked(item catalog.ContentItem, prov provider.Provider, home, targetPath string) (string, string, bool, error) {
	if prov.LegacyInstallDir == nil {
		return "", "", false, nil
	}
	if _, err := os.Lstat(targetPath); err == nil {
		return "", "", false, nil
	}
	dir := prov.LegacyInstallDir(home, item.Type)
	if dir == "" {
		return "", "", false, nil
	}
	lp := filepath.Join(dir, filepath.Base(targetPath))
	if _, err := os.Lstat(lp); err != nil {
		if lstatUnreadable(err) {
			return "", "", false, err
		}
		return "", "", false, nil
	}
	return lp, dir, true, nil
}

func symlinkTargetBelongsToItem(target string, item catalog.ContentItem) bool {
	target = filepath.Clean(target)
	sourcePath := filepath.Clean(SourcePathFor(item))
	itemPath := filepath.Clean(item.Path)
	return target == sourcePath || target == itemPath || pathWithinRoot(target, itemPath)
}

// agentTargetPath returns where an agent installs for prov: one file named
// after the item, with the extension of prov's agent format.
func agentTargetPath(installDir string, item catalog.ContentItem, prov provider.Provider) string {
	return filepath.Join(installDir, item.Name+converter.AgentFileExt(prov.Slug))
}

// installAgent writes item to targetPath in prov's agent format. The library
// file is not in any target's native format, so an agent install is always a
// copy, never a symlink to it: the preserved original when prov is the
// provider the agent came from, otherwise a render for prov.
func installAgent(item catalog.ContentItem, prov provider.Provider, targetPath string) (Placement, error) {
	placement := Placement{Mechanism: MechanismCopy, Path: targetPath, desc: targetPath}

	// Releases before agents were rendered left symlinks to the canonical
	// file here. Replace one that points into this item; never write through
	// any other symlink.
	if info, err := os.Lstat(targetPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
		actual, err := resolveSymlinkTarget(targetPath)
		if err != nil || !symlinkTargetBelongsToItem(actual, item) {
			return Placement{}, fmt.Errorf("destination is a symlink: %s (refusing to follow for security)", targetPath)
		}
		if err := os.Remove(targetPath); err != nil {
			return Placement{}, err
		}
	}

	// Copy the original only when it is in the format prov reads from
	// targetPath: a Kiro CLI JSON agent does not belong in a Kiro .md file.
	src := converter.SourceFilePath(item)
	if src != "" && itemSourceProvider(item) == prov.Slug && strings.HasSuffix(src, converter.AgentFileExt(prov.Slug)) {
		return placement, CopyContent(src, targetPath)
	}

	contentFile := converter.ResolveContentFile(item)
	if contentFile == "" {
		return Placement{}, fmt.Errorf("no content file found in %s", item.Path)
	}
	content, err := os.ReadFile(contentFile)
	if err != nil {
		return Placement{}, fmt.Errorf("reading content file: %w", err)
	}

	// The library file is in the source provider's format when it was added
	// without a type (add --all skips canonicalization), and canonical
	// otherwise. Canonicalize from the source provider the way convert does;
	// a file that fails to parse in that format is already canonical.
	conv := converter.For(catalog.Agents)
	if canonical, cErr := conv.Canonicalize(content, itemSourceProvider(item)); cErr == nil && canonical != nil && canonical.Content != nil {
		content = canonical.Content
	}

	result, err := conv.Render(content, prov)
	if err != nil {
		return Placement{}, fmt.Errorf("rendering for %s: %w", prov.Name, err)
	}
	placement.Notices = conversionNotices(item, result.Warnings)
	if result.Content == nil {
		return Placement{Notices: placement.Notices}, fmt.Errorf("skipped %s: not compatible with %s", item.Name, prov.Name)
	}
	return placement, writeFileAtomic(targetPath, bytes.NewReader(result.Content))
}

// itemSourceProvider returns the provider slug item was imported from: the
// catalog's provider directory for provider-specific types, the metadata's
// source_provider for universal types such as agents, which the catalog
// scans without a provider directory.
func itemSourceProvider(item catalog.ContentItem) string {
	if item.Provider != "" {
		return item.Provider
	}
	if item.Meta != nil {
		return item.Meta.SourceProvider
	}
	return ""
}

// installWithRenderTo reads canonical content, renders it for the target provider,
// and writes to the specified target directory. The render's warnings come
// back as notices, also when the render skips the item.
func installWithRenderTo(item catalog.ContentItem, prov provider.Provider, conv converter.Converter, targetDir string) (string, []Notice, error) {
	contentFile := converter.ResolveContentFile(item)
	if contentFile == "" {
		return "", nil, fmt.Errorf("no content file found in %s", item.Path)
	}

	content, err := os.ReadFile(contentFile)
	if err != nil {
		return "", nil, fmt.Errorf("reading content file: %w", err)
	}

	result, err := conv.Render(content, prov)
	if err != nil {
		return "", nil, fmt.Errorf("rendering for %s: %w", prov.Name, err)
	}
	notices := conversionNotices(item, result.Warnings)

	// nil Content means this rule should be skipped (e.g. non-alwaysApply for single-file providers)
	if result.Content == nil {
		return "", notices, fmt.Errorf("skipped %s: not compatible with %s", item.Name, prov.Name)
	}

	targetPath := filepath.Join(targetDir, result.Filename)

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return "", notices, err
	}
	return targetPath, notices, os.WriteFile(targetPath, result.Content, 0644)
}

// placedNotices is what prov loses from an item placed as it is, without
// conversion: the warnings converting it for prov gives, such as a skill's
// hooks that prov runs only from its own settings. An item placed for the
// provider it came from loses nothing. The warnings are advisory, so a
// conversion that fails reports none.
func placedNotices(item catalog.ContentItem, prov provider.Provider) []Notice {
	if item.Type == catalog.Hooks || item.Provider == prov.Slug {
		return nil
	}
	c, err := converter.ConvertItem(item, prov, "")
	if err != nil {
		return nil
	}
	return conversionNotices(item, c.Warnings)
}

// conversionNotices turns a render's warnings into notices about item.
func conversionNotices(item catalog.ContentItem, warnings []string) []Notice {
	var notices []Notice
	for _, w := range warnings {
		notices = append(notices, Notice{Kind: NoticeConversionWarning, Message: item.Name + ": " + w})
	}
	return notices
}

// installFromSourceTo copies the .source/ original directly to the specified target directory.
// Used for lossless roundtrip when target matches source provider.
func installFromSourceTo(item catalog.ContentItem, _ provider.Provider, targetDir string) (string, error) {
	sourcePath := converter.SourceFilePath(item)
	if sourcePath == "" {
		return "", fmt.Errorf("no source file found in %s/.source/", item.Path)
	}

	targetPath := filepath.Join(targetDir, filepath.Base(sourcePath))

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return "", err
	}
	return targetPath, CopyContent(sourcePath, targetPath)
}
