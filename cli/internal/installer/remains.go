package installer

import (
	"bytes"
	"errors"
	"io/fs"
	"os"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
	"github.com/tidwall/gjson"
)

// InstallPath returns the path an install of item for prov occupies under
// home, which is where UninstallFrom removes it: the legacy location when
// only that one holds it. It returns "" when prov merges item into a
// settings file or does not install it to a directory under home.
func InstallPath(item catalog.ContentItem, prov provider.Provider, home string) (string, error) {
	if prov.InstallDir == nil || IsJSONMerge(prov, item.Type) {
		return "", nil
	}
	target, err := targetIn(prov.InstallDir(home, item.Type), item, prov)
	if err != nil {
		return "", nil
	}
	lp, _, ok, err := legacyTargetChecked(item, prov, home, target)
	if err != nil {
		return "", err
	}
	if ok {
		return lp, nil
	}
	return target, nil
}

// MCPKeyIn reports whether the MCP config at cfgPath holds key, a server's
// JSON path as an MCP install records it.
func MCPKeyIn(cfgPath, key string) (bool, error) {
	data, err := readJSONFile(cfgPath)
	if err != nil {
		return false, err
	}
	return gjson.GetBytes(converter.StripJSONCComments(data), key).Exists(), nil
}

// RuleAppendedIn reports whether targetFile holds any version of rule as an
// appended block, by the search UninstallRuleAppend makes.
func RuleAppendedIn(targetFile string, rule *rulestore.Loaded) (bool, error) {
	raw, err := os.ReadFile(targetFile)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	normalized := canonical.Normalize(raw)
	for _, v := range rule.Meta.Versions {
		pattern := append([]byte{'\n'}, canonical.Normalize(rule.History[v.Hash])...)
		if bytes.Contains(normalized, pattern) {
			return true, nil
		}
	}
	return false, nil
}
