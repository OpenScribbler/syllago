package tui

import (
	"context"
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/lifecycle"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/moatinstall"
)

// registryInstallOp builds the operation a registry install runs. Tests
// replace it to stub the sync and the clone.
var registryInstallOp = func(minTier moat.TrustTier) *moatinstall.Operation {
	return &moatinstall.Operation{MinTier: minTier}
}

// registryInstall is one install of a MOAT registry item, kept whole so an
// answer to a decision can run it again.
type registryInstall struct {
	req          moatinstall.Request
	item         catalog.ContentItem // the registry item, for the modals
	providerName string              // the single target's name; empty for install-all
	all          bool
}

// pendingRegistryDecision is a registry install waiting on the user's
// answer in the confirm or TOFU modal.
type pendingRegistryDecision struct {
	install  registryInstall
	decision *lifecycle.DecisionRequired
}

// registryInstallDoneMsg carries what one run of the operation returned.
type registryInstallDoneMsg struct {
	install registryInstall
	res     moatinstall.Result
	err     error
}

// registryInstallFor builds the operation's request for a wizard install.
func (a App) registryInstallFor(msg installResultMsg) registryInstall {
	baseDir := ""
	switch msg.location {
	case "global":
	case "project":
		baseDir = msg.projectRoot
	default:
		baseDir = msg.location
	}
	return registryInstall{
		req: moatinstall.Request{
			Registry:    msg.item.Registry,
			Items:       []string{msg.item.Name},
			ProjectRoot: msg.projectRoot,
			Targets:     []lifecycle.Target{{Provider: msg.provider, BaseDir: baseDir}},
			Method:      msg.method,
			Session:     a.moatSession,
		},
		item:         msg.item,
		providerName: msg.provider.Name,
	}
}

// registryInstallAllFor builds the operation's request for install-all: one
// item to every provider in the list.
func (a App) registryInstallAllFor(msg installAllResultMsg) registryInstall {
	targets := make([]lifecycle.Target, 0, len(msg.providers))
	for _, prov := range msg.providers {
		targets = append(targets, lifecycle.Target{Provider: prov})
	}
	return registryInstall{
		req: moatinstall.Request{
			Registry:    msg.item.Registry,
			Items:       []string{msg.item.Name},
			ProjectRoot: msg.projectRoot,
			Targets:     targets,
			Method:      installer.MethodSymlink,
			Session:     a.moatSession,
		},
		item: msg.item,
		all:  true,
	}
}

// displayName names the item in toasts.
func (p registryInstall) displayName() string {
	if p.item.DisplayName != "" {
		return p.item.DisplayName
	}
	return p.item.Name
}

// startRegistryInstall runs the operation in the background. Esc cancels
// it until registryInstallDoneMsg arrives. One install runs at a time, so
// Esc always names the install in flight.
func (a App) startRegistryInstall(p registryInstall) (tea.Model, tea.Cmd) {
	if a.registryInstallCancel != nil {
		return a, a.toast.Push("A registry install is already running", toastWarning)
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.registryInstallCancel = cancel
	op := registryInstallOp(a.moatMinTier)
	toast := a.toast.Push(fmt.Sprintf("Installing %q from %s... (Esc cancels)", p.displayName(), p.req.Registry), toastSuccess)
	run := func() tea.Msg {
		defer cancel()
		res, err := op.Install(ctx, p.req)
		return registryInstallDoneMsg{install: p, res: res, err: err}
	}
	return a, tea.Batch(toast, run)
}

// handleRegistryInstallDone reports the install, or opens the modal for the
// decision it needs.
func (a App) handleRegistryInstallDone(msg registryInstallDoneMsg) (tea.Model, tea.Cmd) {
	a.registryInstallCancel = nil
	p := msg.install
	var decision *lifecycle.DecisionRequired
	if errors.As(msg.err, &decision) {
		return a.askRegistryDecision(p, decision)
	}
	// Staging can put the item in the Library before a cancel or a failed
	// placement stops the install, so a failure reads the catalog again.
	failed := func(m tea.Model, cmd tea.Cmd) (tea.Model, tea.Cmd) {
		if len(msg.res.Stage.Completed) == 0 {
			return m, cmd
		}
		app := m.(App)
		return app, tea.Batch(cmd, app.rescanCatalog())
	}
	if errors.Is(msg.err, context.Canceled) {
		cmd := a.toast.Push("Install cancelled", toastWarning)
		return failed(a, cmd)
	}
	if msg.err != nil {
		cmd := a.toast.Push("Install failed: "+formatToastErr(msg.err), toastError)
		return failed(a, cmd)
	}

	ir := msg.res.Items[0]
	warnings := append(msg.res.Stage.Warnings(), ir.Install.Warnings()...)
	for _, t := range ir.Unsupported {
		warnings = append(warnings, fmt.Sprintf("skipped %s: it does not support %s", t.Provider.Name, ir.Library.Type.Label()))
	}
	if p.all && len(ir.Install.Completed) > 0 {
		// The toast names only the first placement failure.
		firstErr := ir.Err
		for _, f := range ir.Install.Failed {
			if f.Stage == lifecycle.StagePlace {
				firstErr = f.Err
				break
			}
		}
		return a.handleInstallAllDone(installAllDoneMsg{
			itemName: p.displayName(),
			count:    len(ir.Install.Completed),
			notices:  ir.Install.Notices,
			warnings: warnings,
			firstErr: firstErr,
		})
	}
	done := installDoneMsg{
		itemName:     p.displayName(),
		providerName: p.providerName,
		notices:      ir.Install.Notices,
		warnings:     warnings,
		err:          ir.Err,
	}
	if len(ir.Install.Completed) > 0 {
		done.targetPath = ir.Install.Completed[0].Placement.String()
	}
	if ir.Err != nil {
		return failed(a.handleInstallDone(done))
	}
	return a.handleInstallDone(done)
}

// askRegistryDecision opens the modal that answers decision. A pinned
// Library copy has no modal: the user unpins it first, as in the CLI.
func (a App) askRegistryDecision(p registryInstall, decision *lifecycle.DecisionRequired) (tea.Model, tea.Cmd) {
	// The decision arrives in the background, so a prompt the user already
	// has open keeps the screen: replacing it would strand that prompt's
	// pending action, or let this answer land on it.
	if a.anyOverlayActive() {
		return a, a.toast.Push(fmt.Sprintf("Installing %q needs your answer, but another prompt is open; install it again after closing that one", p.displayName()), toastWarning)
	}
	pending := &pendingRegistryDecision{install: p, decision: decision}
	switch decision.Kind {
	case lifecycle.TrustOnFirstUse:
		profile, _ := decision.Context.(config.SigningProfile)
		a.pendingRegistryDecision = pending
		a.tofu.Open(p.req.Registry, a.registryManifestURI(p.req.Registry), profile)
		return a, nil

	case lifecycle.PublisherWarn, lifecycle.PrivateSource:
		prompts, _ := decision.Context.([]moatinstall.GatePrompt)
		if len(prompts) == 0 {
			break
		}
		item := p.item
		a.pendingRegistryDecision = pending
		if decision.Kind == lifecycle.PublisherWarn {
			a.confirm.OpenForItem(publisherWarnTitle(item), publisherWarnBody(item, prompts[0].Gate.Revocation), "Install anyway", true, nil, item)
		} else {
			a.confirm.OpenForItem(privatePromptTitle(item), privatePromptBody(item), "Install", false, nil, item)
		}
		return a, nil

	case lifecycle.OverwritePinned:
		pinned, _ := decision.Context.([]lifecycle.PinnedDestination)
		if len(pinned) > 0 {
			name := pinned[0].Name
			return a, a.toast.Push(fmt.Sprintf("%s/%s is pinned; unpin it to update: syllago unpin %s", p.req.Registry, name, name), toastError)
		}
	}
	return a, a.toast.Push(fmt.Sprintf("Install failed: unhandled decision %s", decision.Kind), toastError)
}

// answerRegistryDecision records the user's answer from the confirm modal
// and runs the install again. A decline stops it: nothing is recorded, so
// nothing can carry the decline past a newer manifest.
func (a App) answerRegistryDecision(pending pendingRegistryDecision, confirmed bool) (tea.Model, tea.Cmd) {
	kind := gateKindPublisherWarn
	if pending.decision.Kind == lifecycle.PrivateSource {
		kind = gateKindPrivatePrompt
	}
	if !confirmed {
		return a, a.toast.Push(gateCancelledToastText(kind), toastWarning)
	}
	p := pending.install
	prompts, _ := pending.decision.Context.([]moatinstall.GatePrompt)
	answers := &p.req.Decisions.PublisherWarn
	if kind == gateKindPrivatePrompt {
		answers = &p.req.Decisions.PrivateSource
	}
	if *answers == nil {
		*answers = map[string]bool{}
	}
	for _, prompt := range prompts {
		(*answers)[prompt.Entry.ContentHash] = true
	}
	return a.startRegistryInstall(p)
}

// answerRegistryTOFU runs the install again once the user trusts the
// registry's signing profile.
func (a App) answerRegistryTOFU(pending pendingRegistryDecision, accepted bool) (tea.Model, tea.Cmd) {
	p := pending.install
	if !accepted {
		return a, a.toast.Push("Rejected signing identity for "+p.req.Registry, toastWarning)
	}
	p.req.Decisions.AcceptTOFU = true
	return a.startRegistryInstall(p)
}

// registryManifestURI is the manifest URL the TOFU modal shows.
func (a App) registryManifestURI(name string) string {
	if a.cfg == nil {
		return ""
	}
	for _, r := range a.cfg.Registries {
		if r.Name == name {
			return r.ManifestURI
		}
	}
	return ""
}

// anyOverlayActive reports whether a modal is open to take a key.
func (a App) anyOverlayActive() bool {
	for _, ov := range a.overlays() {
		if ov.Active() {
			return true
		}
	}
	return false
}
