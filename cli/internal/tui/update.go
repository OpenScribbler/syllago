package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/updater"
)

type updateStep int

const (
	stepUpdateMenu    updateStep = iota // "See what's new" / "Update now"
	stepUpdatePreview                   // release notes or git log + diff stat
	stepUpdatePull                      // running git pull
	stepUpdateDone                      // result
)

// Messages

type updateCheckMsg struct {
	localVersion  string
	remoteVersion string
	commitsBehind int
	updateAvail   bool
	releaseBody   string
	err           error
}

type updatePreviewMsg struct {
	releaseNotes string // glamour-rendered release notes (empty = use fallback)
	fallbackLog  string // git log output
	fallbackStat string // diff stat output
	versionRange string // e.g. "v0.2.0 → v0.3.0"
	err          error
}

type updatePullMsg struct {
	output string
	err    error
}

// updateModel handles the "Update syllago..." screen.
type updateModel struct {
	repoRoot       string
	localVersion   string
	remoteVersion  string
	updateAvail    bool
	commitsBehind  int
	isReleaseBuild bool
	releaseBody    string // raw release notes from API (release builds only)
	step           updateStep
	cursor         int
	releaseNotes   string // glamour-rendered release notes
	fallbackLog    string // git log (used when no release notes)
	fallbackStat   string // diff stat (used when no release notes)
	versionRange   string // e.g. "v0.2.0 → v0.3.0"
	previewErr     error
	pullOutput     string
	pullErr        error
	scrollOffset   int
	width, height  int
	spinner        spinner.Model
	loading        bool // true while an async operation is in progress
}

func newUpdateModel(repoRoot, localVersion, remoteVersion string, commitsBehind int, isReleaseBuild bool) updateModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(primaryColor)
	return updateModel{
		repoRoot:       repoRoot,
		localVersion:   localVersion,
		remoteVersion:  remoteVersion,
		updateAvail:    remoteVersion != "" && versionNewer(remoteVersion, localVersion),
		commitsBehind:  commitsBehind,
		isReleaseBuild: isReleaseBuild,
		step:           stepUpdateMenu,
		spinner:        sp,
	}
}

// versionNewer returns true if a is newer than b using simple major.minor.patch comparison.
func versionNewer(a, b string) bool {
	pa := parseVersion(a)
	pb := parseVersion(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

// versionInRange returns true if v is in the range (localV, remoteV] — strictly
// newer than localV and at most remoteV.
func versionInRange(v, localV, remoteV string) bool {
	return versionNewer(v, localV) && !versionNewer(v, remoteV)
}

func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	var out [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		out[i], _ = strconv.Atoi(parts[i])
	}
	return out
}

// validateStep checks entry-prerequisites for the current step.
func (m updateModel) validateStep() {
	switch m.step {
	case stepUpdateMenu:
		// Entry step — no prerequisites.
	case stepUpdatePreview:
		if m.releaseNotes == "" && m.fallbackLog == "" {
			panic("wizard invariant: stepUpdatePreview entered with no release notes or fallback log")
		}
	case stepUpdatePull:
		// Async operation — triggered by menu selection.
	case stepUpdateDone:
		// Terminal step — shows result.
	}
}

func (m updateModel) Update(msg tea.Msg) (updateModel, tea.Cmd) {
	m.validateStep()
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft && m.step == stepUpdateMenu && !m.loading {
			menuItems := m.menuItemCount()
			for i := 0; i < menuItems; i++ {
				if zone.Get(fmt.Sprintf("update-opt-%d", i)).InBounds(msg) {
					m.cursor = i
					return m.updateMenu(tea.KeyMsg{Type: tea.KeyEnter})
				}
			}
		}
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case updateCheckMsg:
		m.loading = false
		if msg.err == nil && msg.remoteVersion != "" {
			m.remoteVersion = msg.remoteVersion
			m.commitsBehind = msg.commitsBehind
			m.updateAvail = versionNewer(msg.remoteVersion, m.localVersion)
			if msg.releaseBody != "" {
				m.releaseBody = msg.releaseBody
			}
		}
		m.cursor = 0
		return m, nil

	case updatePreviewMsg:
		m.loading = false
		m.releaseNotes = msg.releaseNotes
		m.fallbackLog = msg.fallbackLog
		m.fallbackStat = msg.fallbackStat
		m.versionRange = msg.versionRange
		m.previewErr = msg.err
		m.step = stepUpdatePreview
		m.scrollOffset = 0
		return m, nil

	case updatePullMsg:
		m.loading = false
		m.pullOutput = msg.output
		m.pullErr = msg.err
		m.step = stepUpdateDone
		return m, nil

	case tea.KeyMsg:
		switch m.step {
		case stepUpdateMenu:
			return m.updateMenu(msg)
		case stepUpdatePreview:
			return m.updatePreview(msg)
		case stepUpdatePull:
			// No key handling while pull is running
			return m, nil
		case stepUpdateDone:
			return m.updateDone(msg)
		}
	}
	return m, nil
}

func (m updateModel) updateMenu(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	menuItems := m.menuItemCount()
	switch {
	case key.Matches(msg, keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, keys.Down):
		if m.cursor < menuItems-1 {
			m.cursor++
		}
	case key.Matches(msg, keys.Enter):
		if m.updateAvail && m.cursor == 0 {
			// "See what's new"
			m.loading = true
			return m, tea.Batch(m.fetchReleaseNotes(), m.spinner.Tick)
		}
		if !m.updateAvail && m.cursor == 0 {
			// "View release notes" (on latest version)
			m.loading = true
			return m, tea.Batch(m.fetchLocalReleaseNotes(), m.spinner.Tick)
		}
		// "Update now" (cursor 1 when update avail) or "Check for updates" (cursor 1 when on latest)
		if m.updateAvail {
			m.step = stepUpdatePull
			m.loading = true
			return m, tea.Batch(m.startPull(), m.spinner.Tick)
		}
		// "Check for updates" — re-run the check
		m.loading = true
		if m.isReleaseBuild {
			return m, tea.Batch(checkForUpdateRelease(m.localVersion), m.spinner.Tick)
		}
		return m, tea.Batch(checkForUpdate(m.repoRoot, m.localVersion), m.spinner.Tick)
	}
	return m, nil
}

func (m updateModel) updatePreview(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up) || msg.String() == "k":
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}
	case key.Matches(msg, keys.Down) || msg.String() == "j":
		m.scrollOffset++
	case key.Matches(msg, keys.PageUp):
		pageSize := m.height - 8
		if pageSize < 1 {
			pageSize = 10
		}
		m.scrollOffset -= pageSize
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
	case key.Matches(msg, keys.PageDown):
		pageSize := m.height - 8
		if pageSize < 1 {
			pageSize = 10
		}
		m.scrollOffset += pageSize
	case key.Matches(msg, keys.Enter):
		if !m.updateAvail {
			// Viewing local release notes — Enter is a no-op
			return m, nil
		}
		m.step = stepUpdatePull
		m.loading = true
		return m, tea.Batch(m.startPull(), m.spinner.Tick)
	case key.Matches(msg, keys.Back):
		m.step = stepUpdateMenu
		m.cursor = 0
	}
	return m, nil
}

func (m updateModel) updateDone(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	// Esc returns to category (handled by app.go)
	return m, nil
}

// menuItemCount returns how many menu items are shown.
func (m updateModel) menuItemCount() int {
	return 2 // always 2: release notes + update/check
}

func (m updateModel) helpText() string {
	switch m.step {
	case stepUpdateMenu:
		return "up/down navigate • enter select • esc back"
	case stepUpdatePreview:
		if m.updateAvail {
			return "up/down scroll • enter update now • esc back"
		}
		return "up/down scroll • esc back"
	case stepUpdateDone:
		return "esc back"
	default:
		return "esc back"
	}
}

func (m updateModel) View() string {
	s := renderBreadcrumb(
		BreadcrumbSegment{"Home", "crumb-home"},
		BreadcrumbSegment{"Update", ""},
	) + "\n"

	switch m.step {
	case stepUpdateMenu:
		if m.loading {
			s += "\n" + m.spinner.View() + " " + helpStyle.Render("Fetching update info...") + "\n"
		} else if m.updateAvail {
			s += helpStyle.Render(fmt.Sprintf("You're on v%s. Version v%s is available.", m.localVersion, m.remoteVersion)) + "\n\n"
			options := []string{"See what's new", "Update now"}
			for i, opt := range options {
				prefix := "  "
				style := itemStyle
				if i == m.cursor {
					prefix = "> "
					style = selectedItemStyle
				}
				row := fmt.Sprintf("  %s%s", prefix, style.Render(opt))
				s += zone.Mark(fmt.Sprintf("update-opt-%d", i), row) + "\n"
			}
		} else {
			s += helpStyle.Render(fmt.Sprintf("You're on v%s (latest)", m.localVersion)) + "\n\n"
			options := []string{"View release notes", "Check for updates"}
			for i, opt := range options {
				prefix := "  "
				style := itemStyle
				if i == m.cursor {
					prefix = "> "
					style = selectedItemStyle
				}
				row := fmt.Sprintf("  %s%s", prefix, style.Render(opt))
				s += zone.Mark(fmt.Sprintf("update-opt-%d", i), row) + "\n"
			}
		}

	case stepUpdatePreview:
		if m.previewErr != nil {
			s += errorMsgStyle.Render(fmt.Sprintf("Error: %s", m.previewErr)) + "\n"
			break
		}

		// Header with version range
		if m.versionRange != "" {
			s += helpStyle.Render(fmt.Sprintf("What's new (%s)", m.versionRange)) + "\n\n"
		} else {
			s += helpStyle.Render("What's new") + "\n\n"
		}

		// Build scrollable content
		var content strings.Builder
		if m.releaseNotes != "" {
			content.WriteString(m.releaseNotes)
		} else {
			// Fallback to git log + diff stat
			if m.fallbackLog != "" {
				content.WriteString(labelStyle.Render("Commits:") + "\n")
				content.WriteString(m.fallbackLog + "\n")
			}
			if m.fallbackStat != "" {
				content.WriteString(labelStyle.Render("Files changed:") + "\n")
				content.WriteString(m.fallbackStat)
			}
		}

		lines := strings.Split(content.String(), "\n")
		visibleRows := m.height - 8 // header + subtitle + blank + footer
		if visibleRows < 3 {
			visibleRows = 3
		}
		// Clamp scroll
		maxOffset := len(lines) - visibleRows
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.scrollOffset > maxOffset {
			m.scrollOffset = maxOffset
		}

		end := m.scrollOffset + visibleRows
		if end > len(lines) {
			end = len(lines)
		}
		for i := m.scrollOffset; i < end; i++ {
			s += "  " + lines[i] + "\n"
		}
		if end < len(lines) {
			s += helpStyle.Render("  (more below)") + "\n"
		}

	case stepUpdatePull:
		s += "\n" + m.spinner.View() + " " + helpStyle.Render("Updating syllago...") + "\n"

	case stepUpdateDone:
		if m.pullErr != nil {
			s += "\n" + errorMsgStyle.Render("Update failed") + "\n"
			s += helpStyle.Render(m.pullErr.Error()) + "\n"
		} else {
			s += "\n" + successMsgStyle.Render(fmt.Sprintf("Updated to v%s", m.remoteVersion)) + "\n"
			s += helpStyle.Render("Restart syllago to complete the update.") + "\n"
		}
	}

	return s
}

// fetchReleaseNotes reads release notes for all versions between local and remote.
// For release builds it renders the body already fetched from the GitHub API.
// For dev builds it reads release notes from git and falls back to git log + diff stat.
func (m updateModel) fetchReleaseNotes() tea.Cmd {
	if m.isReleaseBuild {
		localVersion := m.localVersion
		remoteVersion := m.remoteVersion
		releaseBody := m.releaseBody
		return func() tea.Msg {
			versionRange := fmt.Sprintf("v%s -> v%s", localVersion, remoteVersion)
			if releaseBody == "" {
				return updatePreviewMsg{versionRange: versionRange}
			}
			rendered, err := glamour.Render(releaseBody, "auto")
			if err != nil {
				return updatePreviewMsg{versionRange: versionRange}
			}
			return updatePreviewMsg{
				releaseNotes: strings.TrimSpace(rendered),
				versionRange: versionRange,
			}
		}
	}

	repoRoot := m.repoRoot
	localVersion := m.localVersion
	remoteVersion := m.remoteVersion
	return func() tea.Msg {
		versionRange := fmt.Sprintf("v%s -> v%s", localVersion, remoteVersion)

		// List release notes files on origin/main
		lsCmd := exec.Command("git", "-C", repoRoot, "ls-tree", "--name-only", "origin/main", "releases/")
		lsOut, err := lsCmd.Output()

		var versions []string
		if err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(lsOut)), "\n") {
				name := filepath.Base(line)
				if !strings.HasPrefix(name, "v") || !strings.HasSuffix(name, ".md") {
					continue
				}
				v := strings.TrimSuffix(name, ".md")
				if versionInRange(v, localVersion, remoteVersion) {
					versions = append(versions, v)
				}
			}
		}

		// Sort descending (newest first)
		sort.Slice(versions, func(i, j int) bool {
			return versionNewer(versions[i], versions[j])
		})

		// Read and concatenate release notes
		if len(versions) > 0 {
			var combined strings.Builder
			for i, v := range versions {
				if i > 0 {
					combined.WriteString("\n---\n\n")
				}
				showCmd := exec.Command("git", "-C", repoRoot, "show", fmt.Sprintf("origin/main:releases/%s.md", v))
				showOut, err := showCmd.Output()
				if err != nil {
					// If any file fails to read, fall through to fallback
					versions = nil
					break
				}
				combined.Write(showOut)
			}

			if len(versions) > 0 {
				rendered, err := glamour.Render(combined.String(), "auto")
				if err == nil {
					return updatePreviewMsg{
						releaseNotes: strings.TrimSpace(rendered),
						versionRange: versionRange,
					}
				}
			}
		}

		// Fallback: git log + diff stat
		logCmd := exec.Command("git", "-C", repoRoot, "log", "HEAD..origin/main", "--oneline", "--no-decorate")
		logOut, logErr := logCmd.Output()

		statCmd := exec.Command("git", "-C", repoRoot, "diff", "--stat", "HEAD..origin/main")
		statOut, _ := statCmd.Output()

		if logErr != nil {
			return updatePreviewMsg{err: fmt.Errorf("git log: %w", logErr)}
		}
		return updatePreviewMsg{
			fallbackLog:  strings.TrimSpace(string(logOut)),
			fallbackStat: strings.TrimSpace(string(statOut)),
			versionRange: versionRange,
		}
	}
}

// fetchLocalReleaseNotes reads release notes for the current local version from disk.
func (m updateModel) fetchLocalReleaseNotes() tea.Cmd {
	repoRoot := m.repoRoot
	localVersion := m.localVersion
	return func() tea.Msg {
		path := filepath.Join(repoRoot, "releases", fmt.Sprintf("v%s.md", localVersion))
		data, err := os.ReadFile(path)
		if err != nil {
			return updatePreviewMsg{err: fmt.Errorf("no release notes found for v%s", localVersion)}
		}

		rendered, err := glamour.Render(string(data), "auto")
		if err != nil {
			return updatePreviewMsg{err: fmt.Errorf("render error: %w", err)}
		}

		return updatePreviewMsg{
			releaseNotes: strings.TrimSpace(rendered),
			versionRange: fmt.Sprintf("v%s", localVersion),
		}
	}
}

// startPull runs the appropriate update mechanism based on build type.
func (m updateModel) startPull() tea.Cmd {
	if m.isReleaseBuild {
		return m.startPullRelease()
	}
	return m.startPullGit()
}

// startPullRelease downloads and installs the latest release binary via the updater package.
func (m updateModel) startPullRelease() tea.Cmd {
	localVersion := m.localVersion
	return func() tea.Msg {
		err := updater.Update(localVersion, func(string) {})
		return updatePullMsg{err: err}
	}
}

// startPullGit checks for local changes and runs git pull.
func (m updateModel) startPullGit() tea.Cmd {
	repoRoot := m.repoRoot
	return func() tea.Msg {
		// Check for local modifications
		statusCmd := exec.Command("git", "-C", repoRoot, "status", "--porcelain")
		statusOut, _ := statusCmd.Output()
		if len(strings.TrimSpace(string(statusOut))) > 0 {
			// Filter: ignore untracked files (lines starting with "??")
			dirty := false
			for _, line := range strings.Split(strings.TrimSpace(string(statusOut)), "\n") {
				if line != "" && !strings.HasPrefix(line, "??") {
					dirty = true
					break
				}
			}
			if dirty {
				return updatePullMsg{err: fmt.Errorf("you have local changes that may conflict with the update. Please commit or stash them first")}
			}
		}

		pullCmd := exec.Command("git", "-C", repoRoot, "pull", "origin", "main")
		out, err := pullCmd.CombinedOutput()
		if err != nil {
			return updatePullMsg{output: string(out), err: fmt.Errorf("git pull failed: %s", strings.TrimSpace(string(out)))}
		}
		return updatePullMsg{output: string(out)}
	}
}

// checkForUpdateRelease fetches the latest release from GitHub and compares it to
// the local version. Used for release builds (no git required).
func checkForUpdateRelease(localVersion string) tea.Cmd {
	return func() tea.Msg {
		info, err := updater.CheckLatest(localVersion)
		if err != nil {
			return updateCheckMsg{err: err}
		}
		return updateCheckMsg{
			localVersion:  localVersion,
			remoteVersion: info.Version,
			updateAvail:   info.UpdateAvail,
			releaseBody:   info.Body,
		}
	}
}

// checkForUpdate runs a background fetch and compares local vs remote versions.
// Uses GIT_TERMINAL_PROMPT=0 and a timeout to prevent blocking the TUI when
// credentials are unavailable (private repos, expired tokens, VHS recordings).
func checkForUpdate(repoRoot, localVersion string) tea.Cmd {
	return func() tea.Msg {
		// Fetch latest from origin (quiet, fail silently if offline or no credentials)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		fetchCmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "fetch", "origin", "main", "--quiet", "--tags")
		fetchCmd.Stdin = nil
		fetchCmd.Stdout = nil
		fetchCmd.Stderr = nil
		fetchCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		_ = fetchCmd.Run() // ignore errors (offline is fine)

		// Get latest tag on origin/main
		descCmd := exec.Command("git", "-C", repoRoot, "describe", "--tags", "--abbrev=0", "origin/main")
		descCmd.Stdin = nil
		descCmd.Stderr = nil
		descOut, err := descCmd.Output()

		remoteVersion := ""
		commitsBehind := 0

		if err == nil {
			tag := strings.TrimSpace(string(descOut))
			remoteVersion = strings.TrimPrefix(tag, "v")
		}

		// Count commits behind
		behindCmd := exec.Command("git", "-C", repoRoot, "rev-list", "--count", "HEAD..origin/main")
		behindCmd.Stdin = nil
		behindCmd.Stderr = nil
		behindOut, err := behindCmd.Output()
		if err == nil {
			fmt.Sscanf(strings.TrimSpace(string(behindOut)), "%d", &commitsBehind)
		}

		return updateCheckMsg{
			localVersion:  localVersion,
			remoteVersion: remoteVersion,
			commitsBehind: commitsBehind,
		}
	}
}
