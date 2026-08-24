package main

import (
	"fmt"
	"os"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

type pinResult struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Registry  string `json:"registry"`
	Pinned    bool   `json:"pinned"`
	SourceSHA string `json:"source_sha,omitempty"`
}

var pinCmd = &cobra.Command{
	Use:   "pin <name>",
	Short: "Pin an installed registry item to hold its current content",
	Args:  cobra.ExactArgs(1),
	RunE:  runPin,
}

var unpinCmd = &cobra.Command{
	Use:   "unpin <name>",
	Short: "Unpin a pinned registry item to allow updates",
	Args:  cobra.ExactArgs(1),
	RunE:  runUnpin,
}

func init() {
	pinCmd.Flags().String("type", "", "Disambiguate when name exists in multiple types")
	unpinCmd.Flags().String("type", "", "Disambiguate when name exists in multiple types")
	rootCmd.AddCommand(pinCmd)
	rootCmd.AddCommand(unpinCmd)
}

func runPin(cmd *cobra.Command, args []string) error {
	return handlePinState(cmd, args[0], true)
}

func runUnpin(cmd *cobra.Command, args []string) error {
	return handlePinState(cmd, args[0], false)
}

func handlePinState(cmd *cobra.Command, name string, pinned bool) error {
	typeFilter, _ := cmd.Flags().GetString("type")

	// Use an empty temp dir as contentRoot to avoid scan shadowing.
	emptyRoot, err := os.MkdirTemp("", "syllago-pin-*")
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "creating temp dir failed", "Check filesystem permissions and disk space", err.Error())
	}
	defer func() { _ = os.RemoveAll(emptyRoot) }()

	cat, err := catalog.ScanWithGlobalAndRegistries(emptyRoot, emptyRoot, nil)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "scanning library failed", "Check that ~/.syllago/content/ exists and is readable", err.Error())
	}

	item, err := findLibraryItem(cat, name, typeFilter)
	if err != nil {
		return err
	}
	telemetry.Enrich("content_type", string(item.Type))

	if item.Meta == nil || item.Meta.SourceRegistry == "" {
		return output.NewStructuredError(
			output.ErrItemNotFound,
			"only registry items can be pinned",
			fmt.Sprintf("Run 'syllago info %s' to check its metadata", name),
		)
	}

	coord := installstore.Coord{
		Registry: item.Meta.SourceRegistry,
		Type:     string(item.Type),
		Name:     item.Name,
	}

	storePath, err := installstore.DefaultPath()
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "could not determine install store path", "Check filesystem permissions", err.Error())
	}

	if err := installstore.SetPinned(storePath, coord, pinned, time.Now()); err != nil {
		return output.NewStructuredErrorDetail(
			output.ErrInstallNotInstalled,
			"pinning needs an installed item",
			fmt.Sprintf("Run 'syllago install %s --to <provider>' first", name),
			err.Error(),
		)
	}

	var sourceSHA string
	store, err := installstore.Load(storePath)
	if err == nil {
		if rec := store.Find(coord); rec != nil {
			sourceSHA = rec.SourceSHA
		}
	}

	if output.JSON {
		output.Print(pinResult{
			Name:      item.Name,
			Type:      string(item.Type),
			Registry:  item.Meta.SourceRegistry,
			Pinned:    pinned,
			SourceSHA: sourceSHA,
		})
		return nil
	}

	if pinned {
		if sourceSHA != "" {
			sha12 := sourceSHA
			if len(sha12) > 12 {
				sha12 = sha12[:12]
			}
			fmt.Fprintf(output.Writer, "Pinned %s/%s — holding at %s\n", string(item.Type), item.Name, sha12)
		} else {
			fmt.Fprintf(output.Writer, "Pinned %s/%s\n", string(item.Type), item.Name)
		}
	} else {
		fmt.Fprintf(output.Writer, "Unpinned %s/%s\n", string(item.Type), item.Name)
	}

	return nil
}
