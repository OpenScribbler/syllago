package main

import (
	"strings"
	"testing"
)

func TestAddCmd_ExcludeFlagVisible(t *testing.T) {
	f := addCmd.Flags().Lookup("exclude")
	if f == nil {
		t.Fatal("--exclude flag not found")
	}
	if f.Hidden {
		t.Error("--exclude flag should be visible (not hidden)")
	}
}

func TestAddCmd_ScopeFlagVisible(t *testing.T) {
	f := addCmd.Flags().Lookup("scope")
	if f == nil {
		t.Fatal("--scope flag not found")
	}
	if f.Hidden {
		t.Error("--scope flag should be visible (not hidden)")
	}
}

func TestAddCmd_ScopeFlagDescription(t *testing.T) {
	f := addCmd.Flags().Lookup("scope")
	if f == nil {
		t.Fatal("--scope flag not found")
	}
	if strings.Contains(f.Usage, "hooks/mcp only") {
		t.Errorf("scope description should say 'hooks and MCP', not 'hooks/mcp only': %s", f.Usage)
	}
	if !strings.Contains(f.Usage, "hooks and MCP") {
		t.Errorf("scope description should contain 'hooks and MCP', got: %s", f.Usage)
	}
}

func TestAddCmd_LongNoRedundantInstallSentence(t *testing.T) {
	if strings.Contains(addCmd.Long, "After adding, use") {
		t.Error("addCmd.Long should not contain redundant 'After adding, use syllago install' sentence")
	}
}

func TestAddCmd_LongNoHooksSpecificMention(t *testing.T) {
	if strings.Contains(addCmd.Long, "Hooks-specific flags") {
		t.Error("addCmd.Long should not reference hidden flag behavior since flags are now visible")
	}
}

// syllago-7cul5: loadout apply Long

func TestLoadoutApplyCmd_LongNoClaudeCodeSpecificity(t *testing.T) {
	if strings.Contains(loadoutApplyCmd.Long, "Claude Code") {
		t.Error("loadoutApplyCmd.Long should say 'one or more providers', not 'Claude Code'")
	}
}

func TestLoadoutApplyCmd_LongMentionsToFlag(t *testing.T) {
	if !strings.Contains(loadoutApplyCmd.Long, "--to") {
		t.Error("loadoutApplyCmd.Long should document the --to flag")
	}
}

// syllago-b6hbs: install Long — hooks/MCP merge note

func TestInstallCmd_LongMentionsHooksMCPMerge(t *testing.T) {
	if !strings.Contains(installCmd.Long, "merged") {
		t.Error("installCmd.Long should explain that hooks and MCP configs are merged, not linked")
	}
}

// syllago-m5ie5: root command Long — add workflow description

func TestRootCmd_LongNoPathOrGitURL(t *testing.T) {
	if strings.Contains(rootCmd.Long, "from a path or git URL") {
		t.Error("rootCmd.Long should say 'from a provider or registry', not 'from a path or git URL'")
	}
}

func TestRootCmd_LongUsesProviderOrRegistry(t *testing.T) {
	if !strings.Contains(rootCmd.Long, "from a provider or registry") {
		t.Error("rootCmd.Long should describe add as importing 'from a provider or registry'")
	}
}

// syllago-qmpdz: remove [Coming Soon] labels

func TestSignCmd_ShortNoComingSoon(t *testing.T) {
	if strings.Contains(signCmd.Short, "[Coming Soon]") {
		t.Errorf("signCmd.Short should not contain '[Coming Soon]': %s", signCmd.Short)
	}
}

func TestVerifyCmd_ShortNoComingSoon(t *testing.T) {
	if strings.Contains(verifyCmd.Short, "[Coming Soon]") {
		t.Errorf("verifyCmd.Short should not contain '[Coming Soon]': %s", verifyCmd.Short)
	}
}

func TestExportCmd_ShortNoComingSoon(t *testing.T) {
	if strings.Contains(exportCmd.Short, "[Coming Soon]") {
		t.Errorf("exportCmd.Short should not contain '[Coming Soon]': %s", exportCmd.Short)
	}
}
