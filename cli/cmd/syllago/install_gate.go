package main

import (
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/moatinstall"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// gateLibraryItems runs the trust gate over Library items before any of
// them installs, as a registry install gates its items before fetching.
// A copy from a MOAT registry is checked against the registry's cached
// manifest and the project lockfile; no registry is synced.
//
// It returns the items that may install, the items the gate or the user
// refused, and the first gate refusal, which fails the command once the
// other items have installed. stop is true when a headless run met a
// prompt and the process has been sent its exit code. A lockfile that
// cannot be read refuses every item when any of them came from a
// registry, since its archived revocations cannot be checked.
func gateLibraryItems(errW io.Writer, items []catalog.ContentItem, cfg *config.Config, projectRoot string) (allowed []catalog.ContentItem, refused []skippedItem, stop bool, refusal error) {
	cacheDir, _ := config.GlobalDirPath()
	in := moat.BuildGateInputs(cfg, cacheDir)
	lf, err := moat.LoadLockfile(moat.LockfilePath(projectRoot))
	if err != nil && slices.ContainsFunc(items, installer.HasRegistryLineage) {
		return nil, nil, false, err
	}
	session := moat.NewSession()

	for _, item := range items {
		stop, err := gateLibraryItem(errW, item, in, lf, session)
		switch {
		case stop:
			return nil, nil, true, nil
		case err == nil:
			allowed = append(allowed, item)
		case errors.Is(err, moatinstall.ErrDeclined):
			fmt.Fprintln(errW, "install refused by operator")
			refused = append(refused, skippedItem{Name: item.Name, Reason: err.Error()})
		default:
			refused = append(refused, skippedItem{Name: item.Name, Reason: err.Error()})
			if !output.JSON {
				fmt.Fprintf(errW, "  skip %s: %s\n", item.Name, err)
			}
			if refusal == nil {
				refusal = err
			}
		}
	}
	return allowed, refused, false, refusal
}

// gateLibraryItem asks the user each question the gate raises for item and
// returns nil once it may install, ErrDeclined when the user says no, or
// the gate's refusal. stop is true when a headless run has exited.
func gateLibraryItem(errW io.Writer, item catalog.ContentItem, in *moat.GateInputs, lf *moat.Lockfile, session *moat.Session) (stop bool, err error) {
	for {
		check, ok := installer.CheckItem(item, in, lf, session, moatInstallMinTier)
		if !ok {
			return false, nil
		}
		switch check.Decision {
		case installer.MOATGateProceed:
			return false, nil
		case installer.MOATGatePublisherWarn:
			reason := moat.SanitizeForDisplay(check.Revocation.Reason)
			fmt.Fprintf(errW, "\nPublisher-source revocation for %q: %s\n", item.Name, reason)
			if !moatInstallInteractiveFn() {
				fmt.Fprintf(errW, "syllago: %s\n", moat.FailurePublisherRevocation.Message())
				moatSyncExit(moat.ExitMoatPublisherRevocation)
				return true, nil
			}
			if !moatInstallPromptFn(errW, "Proceed anyway? [Y/n]: ") {
				return false, moatinstall.ErrDeclined
			}
			installer.MarkPublisherConfirmed(session, check.Revocation.IssuingRegistryURL, check.Entry.ContentHash)
		case installer.MOATGatePrivatePrompt:
			fmt.Fprintf(errW, "\n%q is declared as coming from a private repository.\n", item.Name)
			if !moatInstallInteractiveFn() {
				fmt.Fprintf(errW, "syllago: %s (private content requires operator acknowledgement)\n", moat.FailureTOFUAcceptance.Message())
				moatSyncExit(moat.ExitMoatTOFUAcceptance)
				return true, nil
			}
			if !moatInstallPromptFn(errW, "Install from private source? [Y/n]: ") {
				return false, moatinstall.ErrDeclined
			}
			installer.MarkPrivateConfirmed(session, check.RegistryURL, check.Entry.ContentHash)
		default:
			return false, moatinstall.GateError(check.Entry, check.GateBlock)
		}
	}
}
