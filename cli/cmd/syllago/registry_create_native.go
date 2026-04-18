package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"gopkg.in/yaml.v3"
)

// registryCreateNativeStdin is the input source for the interactive
// `registry create --from-native` wizard. Tests replace it with a
// strings.NewReader to simulate multi-step stdin.
var registryCreateNativeStdin io.Reader = os.Stdin

// registryCreateFromNative scans native provider content and generates
// a registry.yaml with indexed items. Called by `registry create --from-native`.
func registryCreateFromNative(desc string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "getting working directory failed", "Check filesystem permissions", err.Error())
	}

	// Guard: don't overwrite existing registry.yaml
	if _, err := os.Stat(filepath.Join(cwd, "registry.yaml")); err == nil {
		return output.NewStructuredError(output.ErrRegistryInvalid, "registry.yaml already exists in this directory", "Remove the existing registry.yaml first, or use a different directory")
	}

	// Scan for native content
	result := catalog.ScanNativeContent(cwd)
	if result.HasSyllagoStructure {
		return output.NewStructuredError(output.ErrRegistryInvalid, "this directory already has syllago structure", "Use 'registry create --new' instead for syllago-native registries")
	}
	if len(result.Providers) == 0 {
		return output.NewStructuredError(output.ErrRegistryInvalid, "no AI coding tool content found in this directory", "Ensure this directory contains provider-specific content (e.g., .cursor/rules/, .claude/)")
	}

	// Display discovered content
	fmt.Fprintf(output.Writer, "\nScanning for AI coding tool content...\n\n")
	totalItems := 0
	for _, prov := range result.Providers {
		fmt.Fprintf(output.Writer, "  %s\n", prov.ProviderName)
		for typeLabel, items := range prov.Items {
			fmt.Fprintf(output.Writer, "    %3d %-10s\n", len(items), typeLabel)
			totalItems += len(items)
		}
		fmt.Fprintln(output.Writer)
	}

	if totalItems == 0 {
		return output.NewStructuredError(output.ErrRegistryInvalid, "no indexable items found", "Check that the content directories contain valid files")
	}

	scanner := bufio.NewScanner(registryCreateNativeStdin)

	// Selection mode
	fmt.Fprintf(output.Writer, "How would you like to index this content?\n\n")
	fmt.Fprintln(output.Writer, "  1) All content from all providers")
	fmt.Fprintln(output.Writer, "  2) Select by provider")
	fmt.Fprintln(output.Writer, "  3) Select individual items")
	fmt.Fprintf(output.Writer, "\nChoice [1]: ")

	var selectedItems []registry.ManifestItem
	choice := "1"
	if scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text != "" {
			choice = text
		}
	}

	switch choice {
	case "1":
		selectedItems = allItemsFromScan(result)
	case "2":
		selectedItems, err = selectByProvider(result, scanner)
		if err != nil {
			return err
		}
	case "3":
		selectedItems, err = selectIndividualItems(result, scanner)
		if err != nil {
			return err
		}
	default:
		return output.NewStructuredError(output.ErrInputInvalid, fmt.Sprintf("invalid choice: %s", choice), "Enter 1, 2, or 3")
	}

	if len(selectedItems) == 0 {
		return output.NewStructuredError(output.ErrInputMissing, "no items selected", "Select at least one item to include in the registry")
	}

	// Offer to scan user-scoped settings for hooks
	hookItems, hookErr := promptUserScopedHooks(scanner, cwd)
	if hookErr != nil {
		fmt.Fprintf(output.ErrWriter, "warning: %v\n", hookErr)
	}
	if len(hookItems) > 0 {
		selectedItems = append(selectedItems, hookItems...)
	}

	// Registry metadata
	repoName := filepath.Base(cwd)
	fmt.Fprintf(output.Writer, "\nRegistry name [%s]: ", repoName)
	if scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text != "" {
			repoName = text
		}
	}

	if desc == "" {
		fmt.Fprintf(output.Writer, "Description (optional): ")
		if scanner.Scan() {
			desc = strings.TrimSpace(scanner.Text())
		}
	}

	// Generate registry.yaml
	manifest := registry.Manifest{
		Name:        repoName,
		Description: desc,
		Version:     "0.1.0",
		Items:       selectedItems,
	}

	data, err := yaml.Marshal(&manifest)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "marshaling registry.yaml failed", "Check that the manifest data is valid", err.Error())
	}

	if err := os.WriteFile(filepath.Join(cwd, "registry.yaml"), data, 0644); err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "writing registry.yaml failed", "Check filesystem permissions", err.Error())
	}

	// Summary
	fmt.Fprintf(output.Writer, "\nGenerated registry.yaml\n\n")
	fmt.Fprintf(output.Writer, "  name: %s\n", repoName)
	fmt.Fprintf(output.Writer, "  %d items indexed\n\n", len(selectedItems))
	fmt.Fprintf(output.Writer, "This repo can now be added as a registry:\n")
	fmt.Fprintf(output.Writer, "  syllago registry add <url-to-this-repo>\n\n")

	return nil
}

// allItemsFromScan converts all scanned native content to ManifestItems.
func allItemsFromScan(result catalog.NativeScanResult) []registry.ManifestItem {
	var items []registry.ManifestItem
	for _, prov := range result.Providers {
		for typeLabel, nativeItems := range prov.Items {
			for _, ni := range nativeItems {
				mi := registry.ManifestItem{
					Name:     ni.Name,
					Type:     typeLabel,
					Provider: prov.ProviderSlug,
					Path:     ni.Path,
				}
				if ni.HookEvent != "" {
					mi.HookEvent = ni.HookEvent
					mi.HookIndex = ni.HookIndex
				}
				items = append(items, mi)
			}
		}
	}
	return items
}

// selectByProvider presents provider selection and returns items from chosen providers.
func selectByProvider(result catalog.NativeScanResult, scanner *bufio.Scanner) ([]registry.ManifestItem, error) {
	fmt.Fprintf(output.Writer, "\nSelect providers (comma-separated numbers):\n\n")
	for i, prov := range result.Providers {
		count := 0
		for _, items := range prov.Items {
			count += len(items)
		}
		fmt.Fprintf(output.Writer, "  %d) %s (%d items)\n", i+1, prov.ProviderName, count)
	}
	fmt.Fprintf(output.Writer, "\nProviders: ")

	if !scanner.Scan() {
		return nil, output.NewStructuredError(output.ErrInputTerminal, "no selection made", "Enter comma-separated numbers to select providers")
	}

	var selected []registry.ManifestItem
	for _, part := range strings.Split(scanner.Text(), ",") {
		idx, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || idx < 1 || idx > len(result.Providers) {
			continue
		}
		prov := result.Providers[idx-1]
		for typeLabel, nativeItems := range prov.Items {
			for _, ni := range nativeItems {
				mi := registry.ManifestItem{
					Name:     ni.Name,
					Type:     typeLabel,
					Provider: prov.ProviderSlug,
					Path:     ni.Path,
				}
				if ni.HookEvent != "" {
					mi.HookEvent = ni.HookEvent
					mi.HookIndex = ni.HookIndex
				}
				selected = append(selected, mi)
			}
		}
	}
	return selected, nil
}

// selectIndividualItems presents all items for individual selection.
func selectIndividualItems(result catalog.NativeScanResult, scanner *bufio.Scanner) ([]registry.ManifestItem, error) {
	type numberedItem struct {
		provSlug  string
		typeLabel string
		item      catalog.NativeItem
	}

	var all []numberedItem
	for _, prov := range result.Providers {
		for typeLabel, items := range prov.Items {
			for _, item := range items {
				all = append(all, numberedItem{prov.ProviderSlug, typeLabel, item})
			}
		}
	}

	fmt.Fprintf(output.Writer, "\nAvailable items:\n\n")
	for i, ni := range all {
		label := ni.item.Name
		if ni.item.DisplayName != "" {
			label = ni.item.DisplayName
		}
		fmt.Fprintf(output.Writer, "  %3d) [%s/%s] %s\n", i+1, ni.provSlug, ni.typeLabel, label)
	}

	fmt.Fprintf(output.Writer, "\nSelect items (comma-separated numbers, or 'all'): ")
	if !scanner.Scan() {
		return nil, output.NewStructuredError(output.ErrInputTerminal, "no selection made", "Enter comma-separated numbers or 'all' to select items")
	}

	text := strings.TrimSpace(scanner.Text())
	if text == "all" {
		return allItemsFromScan(result), nil
	}

	var selected []registry.ManifestItem
	for _, part := range strings.Split(text, ",") {
		idx, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || idx < 1 || idx > len(all) {
			continue
		}
		ni := all[idx-1]
		mi := registry.ManifestItem{
			Name:     ni.item.Name,
			Type:     ni.typeLabel,
			Provider: ni.provSlug,
			Path:     ni.item.Path,
		}
		if ni.item.HookEvent != "" {
			mi.HookEvent = ni.item.HookEvent
			mi.HookIndex = ni.item.HookIndex
		}
		selected = append(selected, mi)
	}
	return selected, nil
}

// promptUserScopedHooks asks whether to scan user settings for hooks.
// If the user agrees, extracts hooks to .syllago/hooks/ and returns ManifestItems.
func promptUserScopedHooks(scanner *bufio.Scanner, repoRoot string) ([]registry.ManifestItem, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil
	}

	type settingsSource struct {
		provider string
		slug     string
		path     string
	}

	sources := []settingsSource{
		{"Claude Code", "claude-code", filepath.Join(home, ".claude", "settings.json")},
		{"Gemini CLI", "gemini-cli", filepath.Join(home, ".gemini", "settings.json")},
	}

	// Check which have hooks
	var available []settingsSource
	for _, s := range sources {
		if _, err := os.Stat(s.path); err == nil {
			hooks, scanErr := registry.ScanUserHooks(s.path, repoRoot)
			if scanErr == nil && len(hooks) > 0 {
				available = append(available, s)
			}
		}
	}

	if len(available) == 0 {
		return nil, nil
	}

	fmt.Fprintln(output.Writer, "\nWould you also like to include hooks from your user settings?")
	for i, s := range available {
		fmt.Fprintf(output.Writer, "  %d) Scan %s (%s)\n", i+1, s.provider, s.path)
	}
	fmt.Fprintf(output.Writer, "  0) No, project content only\n")
	fmt.Fprintf(output.Writer, "\nChoice [0]: ")

	if !scanner.Scan() {
		return nil, nil
	}
	choice := strings.TrimSpace(scanner.Text())
	if choice == "" || choice == "0" {
		return nil, nil
	}

	idx, parseErr := strconv.Atoi(choice)
	if parseErr != nil || idx < 1 || idx > len(available) {
		return nil, nil
	}

	src := available[idx-1]

	// Security warning
	fmt.Fprintln(output.Writer, "\n  SECURITY WARNING")
	fmt.Fprintln(output.Writer, "  Hooks contain executable scripts that run on the consumer's machine.")
	fmt.Fprintln(output.Writer, "  Only include hooks you trust and intend to share publicly.")
	fmt.Fprintln(output.Writer, "  Scripts will be copied to .syllago/hooks/ in this repo.")
	fmt.Fprintf(output.Writer, "\n  Continue? [y/N]: ")

	if !scanner.Scan() {
		return nil, nil
	}
	confirm := strings.ToLower(strings.TrimSpace(scanner.Text()))
	if confirm != "y" && confirm != "yes" {
		return nil, nil
	}

	hooks, err := registry.ScanUserHooks(src.path, repoRoot)
	if err != nil {
		return nil, fmt.Errorf("scanning %s: %w", src.path, err)
	}

	// Let user select individual hooks
	fmt.Fprintf(output.Writer, "\nFound %d hooks:\n\n", len(hooks))
	for i, h := range hooks {
		warning := ""
		if h.ScriptPath != "" && !h.ScriptInRepo {
			warning = " (script outside repo)"
		}
		display := h.Command
		if display == "" {
			display = h.Event
		}
		fmt.Fprintf(output.Writer, "  %d) %s: %s%s\n", i+1, h.Event, display, warning)
	}
	fmt.Fprintf(output.Writer, "\nSelect hooks (comma-separated numbers, or 'all'): ")

	if !scanner.Scan() {
		return nil, nil
	}
	text := strings.TrimSpace(scanner.Text())

	var selectedHooks []registry.UserScopedHook
	if text == "all" {
		selectedHooks = hooks
	} else {
		for _, part := range strings.Split(text, ",") {
			i, pErr := strconv.Atoi(strings.TrimSpace(part))
			if pErr == nil && i >= 1 && i <= len(hooks) {
				selectedHooks = append(selectedHooks, hooks[i-1])
			}
		}
	}

	if len(selectedHooks) == 0 {
		return nil, nil
	}

	// Extract to .syllago/hooks/
	hooksDir := filepath.Join(repoRoot, ".syllago", "hooks")
	if err := registry.ExtractHooksToDir(selectedHooks, hooksDir); err != nil {
		return nil, fmt.Errorf("extracting hooks: %w", err)
	}

	fmt.Fprintf(output.Writer, "\n  Extracted %d hooks to .syllago/hooks/\n", len(selectedHooks))

	// Build ManifestItems
	var items []registry.ManifestItem
	for _, h := range selectedHooks {
		mi := registry.ManifestItem{
			Name:      h.Name,
			Type:      "hooks",
			Provider:  src.slug,
			Path:      filepath.Join(".syllago", "hooks", h.Name),
			HookEvent: h.Event,
			HookIndex: h.Index,
		}
		if h.ScriptPath != "" && h.ScriptInRepo {
			mi.Scripts = []string{filepath.Base(h.ScriptPath)}
		}
		items = append(items, mi)
	}

	return items, nil
}
