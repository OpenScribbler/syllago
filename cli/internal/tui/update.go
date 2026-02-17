package tui

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type updateStep int

const (
	stepUpdateMenu    updateStep = iota // "See what's new" / "Update now"
	stepUpdatePreview                   // git log + diff stat
	stepUpdatePull                      // running git pull
	stepUpdateDone                      // result
)

// Messages

type updateCheckMsg struct {
	localVersion  string
	remoteVersion string
	commitsBehind int
	err           error
}

type updatePreviewMsg struct {
	log  string
	stat string
	err  error
}

type updatePullMsg struct {
	output string
	err    error
}

// updateModel handles the "Update nesco..." screen.
type updateModel struct {
	repoRoot       string
	localVersion   string
	remoteVersion  string
	updateAvail    bool
	commitsBehind  int
	step           updateStep
	cursor         int
	previewLog     string
	previewStat    string
	previewErr     error
	pullOutput     string
	pullErr        error
	scrollOffset   int
	width, height  int
	spinner        spinner.Model
	loading        bool // true while an async operation is in progress
}

func newUpdateModel(repoRoot, localVersion, remoteVersion string, commitsBehind int) updateModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(primaryColor)
	return updateModel{
		repoRoot:      repoRoot,
		localVersion:  localVersion,
		remoteVersion: remoteVersion,
		updateAvail:   remoteVersion != "" && versionNewer(remoteVersion, localVersion),
		commitsBehind: commitsBehind,
		step:          stepUpdateMenu,
		spinner:       sp,
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

func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	var out [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		out[i], _ = strconv.Atoi(parts[i])
	}
	return out
}

func (m updateModel) Update(msg tea.Msg) (updateModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case updatePreviewMsg:
		m.loading = false
		m.previewLog = msg.log
		m.previewStat = msg.stat
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
			return m, tea.Batch(m.fetchPreview(), m.spinner.Tick)
		}
		// "Update now" (cursor 1 when update avail, cursor 0 when current)
		m.step = stepUpdatePull
		m.loading = true
		return m, tea.Batch(m.startPull(), m.spinner.Tick)
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
	case key.Matches(msg, keys.Enter):
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
	if m.updateAvail {
		return 2 // "See what's new" + "Update now"
	}
	return 1 // "Update now" (check for updates)
}

func (m updateModel) View() string {
	s := helpStyle.Render("nesco >") + " " + titleStyle.Render("Update nesco") + "\n"

	switch m.step {
	case stepUpdateMenu:
		if m.loading {
			s += "\n" + m.spinner.View() + " " + helpStyle.Render("Fetching update info...") + "\n"
		} else if m.updateAvail {
			s += helpStyle.Render(fmt.Sprintf("You're on v%s. Version v%s is available.", m.localVersion, m.remoteVersion)) + "\n\n"
			options := []string{"See what's new", "Update now"}
			for i, opt := range options {
				prefix := "   "
				style := itemStyle
				if i == m.cursor {
					prefix = " > "
					style = selectedItemStyle
				}
				s += prefix + style.Render(opt) + "\n"
			}
			s += "\n" + helpStyle.Render("up/down navigate • enter select • esc back")
		} else {
			s += helpStyle.Render(fmt.Sprintf("You're on v%s (latest)", m.localVersion)) + "\n\n"
			prefix := " > "
			style := selectedItemStyle
			s += prefix + style.Render("Check for updates") + "\n"
			s += "\n" + helpStyle.Render("up/down navigate • enter select • esc back")
		}

	case stepUpdatePreview:
		if m.previewErr != nil {
			s += errorMsgStyle.Render(fmt.Sprintf("Error: %s", m.previewErr)) + "\n"
			s += "\n" + helpStyle.Render("esc back")
			break
		}
		s += helpStyle.Render("What's new") + "\n\n"

		// Combine log and stat into scrollable content
		var content strings.Builder
		if m.previewLog != "" {
			content.WriteString(labelStyle.Render("Commits:") + "\n")
			content.WriteString(m.previewLog + "\n")
		}
		if m.previewStat != "" {
			content.WriteString(labelStyle.Render("Files changed:") + "\n")
			content.WriteString(m.previewStat)
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

		s += "\n" + helpStyle.Render("up/down/jk scroll • enter update now • esc back")

	case stepUpdatePull:
		s += "\n" + m.spinner.View() + " " + helpStyle.Render("Updating nesco...") + "\n"

	case stepUpdateDone:
		if m.pullErr != nil {
			s += "\n" + errorMsgStyle.Render("Update failed") + "\n"
			s += helpStyle.Render(m.pullErr.Error()) + "\n"
		} else {
			s += "\n" + successMsgStyle.Render(fmt.Sprintf("Updated to v%s", m.remoteVersion)) + "\n"
			s += helpStyle.Render("Restart nesco to complete the update.") + "\n"
		}
		s += "\n" + helpStyle.Render("esc to return")
	}

	return s
}

// fetchPreview runs git log and git diff --stat in the background.
func (m updateModel) fetchPreview() tea.Cmd {
	repoRoot := m.repoRoot
	return func() tea.Msg {
		logCmd := exec.Command("git", "-C", repoRoot, "log", "HEAD..origin/main", "--oneline", "--no-decorate")
		logOut, logErr := logCmd.Output()

		statCmd := exec.Command("git", "-C", repoRoot, "diff", "--stat", "HEAD..origin/main")
		statOut, _ := statCmd.Output()

		if logErr != nil {
			return updatePreviewMsg{err: fmt.Errorf("git log: %w", logErr)}
		}
		return updatePreviewMsg{
			log:  strings.TrimSpace(string(logOut)),
			stat: strings.TrimSpace(string(statOut)),
		}
	}
}

// startPull checks for local changes and runs git pull.
func (m updateModel) startPull() tea.Cmd {
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

// checkForUpdate runs a background fetch and compares local vs remote versions.
func checkForUpdate(repoRoot, localVersion string) tea.Cmd {
	return func() tea.Msg {
		// Fetch latest from origin (quiet, fail silently if offline)
		fetchCmd := exec.Command("git", "-C", repoRoot, "fetch", "origin", "main", "--quiet", "--tags")
		fetchCmd.Run() // ignore errors (offline is fine)

		// Get latest tag on origin/main
		descCmd := exec.Command("git", "-C", repoRoot, "describe", "--tags", "--abbrev=0", "origin/main")
		descOut, err := descCmd.Output()

		remoteVersion := ""
		commitsBehind := 0

		if err == nil {
			tag := strings.TrimSpace(string(descOut))
			remoteVersion = strings.TrimPrefix(tag, "v")
		}

		// Count commits behind
		behindCmd := exec.Command("git", "-C", repoRoot, "rev-list", "--count", "HEAD..origin/main")
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
