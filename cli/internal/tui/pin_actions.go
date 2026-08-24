package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/rollback"
)

const confirmPurposeRollback = "rollback"

// Message types
type pinToggleMsg struct {
	name      string
	typ       string
	pinned    bool
	sourceSHA string
	err       error
}

type rollbackPlanMsg struct {
	plan *rollback.Plan
	err  error
}

type rollbackDoneMsg struct {
	name      string
	typ       string
	fromCopy  bool
	sha       string
	reapplied int
	warnings  []string
	err       error
}

// handlePin toggle action
func (a App) handlePin() (tea.Model, tea.Cmd) {
	item := a.selectedItem()
	if item == nil {
		return a, nil
	}
	if item.Meta == nil || item.Meta.SourceRegistry == "" || !item.Library {
		cmd := a.toast.Push("Only installed registry items can be pinned", toastWarning)
		return a, cmd
	}

	coord := installstore.Coord{
		Registry: item.Meta.SourceRegistry,
		Type:     string(item.Type),
		Name:     item.Name,
	}

	cmd := func() tea.Msg {
		storePath, err := installstore.DefaultPath()
		if err != nil {
			return pinToggleMsg{err: fmt.Errorf("pin needs an installed item — install it first")}
		}
		store, err := installstore.Load(storePath)
		if err != nil {
			return pinToggleMsg{err: fmt.Errorf("pin needs an installed item — install it first")}
		}
		rec := store.Find(coord)
		if rec == nil {
			return pinToggleMsg{err: fmt.Errorf("pin needs an installed item — install it first")}
		}

		nextPinned := !rec.Pinned
		err = installstore.SetPinned(storePath, coord, nextPinned, time.Now())
		if err != nil {
			return pinToggleMsg{err: err}
		}

		return pinToggleMsg{
			name:      coord.Name,
			typ:       coord.Type,
			pinned:    nextPinned,
			sourceSHA: rec.SourceSHA,
		}
	}

	return a, cmd
}

// handleRollback action
func (a App) handleRollback() (tea.Model, tea.Cmd) {
	item := a.selectedItem()
	if item == nil {
		return a, nil
	}
	cmd := func() tea.Msg {
		plan, err := rollback.PlanFor(*item)
		return rollbackPlanMsg{plan: plan, err: err}
	}
	return a, cmd
}

// doRollbackCmd
func (a App) doRollbackCmd(plan *rollback.Plan) tea.Cmd {
	return func() tea.Msg {
		err := rollback.Restore(plan, a.version)
		if err != nil {
			return rollbackDoneMsg{err: err}
		}

		// Re-apply placements
		opts := rollback.Options{ProjectRoot: a.projectRoot}
		reapplied, warnings := rollback.ReapplyPlacements(plan.Item, plan.Placements, opts)

		return rollbackDoneMsg{
			name:      plan.Item.Name,
			typ:       string(plan.Item.Type),
			fromCopy:  plan.FromCopy,
			sha:       plan.Prev.SourceSHA,
			reapplied: len(reapplied),
			warnings:  warnings,
		}
	}
}
