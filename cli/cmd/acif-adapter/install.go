package main

import (
	"encoding/json"
	"errors"

	"github.com/OpenScribbler/syllago/cli/internal/acif"
)

type resolveInstallTargetsInput struct {
	Provider    string             `json:"provider"`
	ContentType string             `json:"content_type"`
	ContentName string             `json:"content_name"`
	HomeDir     string             `json:"home_dir"`
	ProjectRoot string             `json:"project_root"`
	Scope       string             `json:"scope"`
	Entry       *acif.InstallEntry `json:"entry"`
}

// handleResolveInstallTargets implements PROTOCOL op 4.14 over
// [ACIF-INSTALL] §6–§11.
func handleResolveInstallTargets(raw json.RawMessage) any {
	var input resolveInstallTargetsInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return errorResponse("adapter: " + err.Error())
	}

	targets, diags, err := acif.ResolveInstallTargets(acif.InstallResolveInput{
		Provider:    input.Provider,
		ContentType: input.ContentType,
		ContentName: input.ContentName,
		HomeDir:     input.HomeDir,
		ProjectRoot: input.ProjectRoot,
		Scope:       input.Scope,
		Entry:       input.Entry,
	})
	if err != nil {
		var reject *acif.RejectError
		if errors.As(err, &reject) {
			return rejectResponse(reject)
		}
		return errorResponse("adapter: " + err.Error())
	}

	result := map[string]any{"targets": targets}
	if len(diags) > 0 {
		result["diagnostics"] = diags
	}
	return okResponse(result)
}
