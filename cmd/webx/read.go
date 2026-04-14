package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/oaooao/webx/internal/core"
	"github.com/oaooao/webx/internal/types"
	"github.com/spf13/cobra"
)

var readKind string
var readFormat string

var readCmd = &cobra.Command{
	Use:   "read <url>",
	Short: "Read a URL and return its content",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var kind *types.WebxKind
		if readKind != "" {
			k := types.WebxKind(readKind)
			if !k.IsValid() {
				return fmt.Errorf("invalid kind: %s", readKind)
			}
			kind = &k
		}

		envelope := core.RunRead(args[0], kind)

		switch readFormat {
		case "json":
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(envelope)
		case "markdown", "md":
			if envelope.Content.Markdown != nil {
				fmt.Print(*envelope.Content.Markdown)
				return nil
			}
			// fallback to JSON if no markdown
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(envelope)
		default: // default is JSON
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(envelope)
		}
	},
}

func init() {
	readCmd.Flags().StringVar(&readKind, "kind", "", "Requested content kind (article, thread, comments, etc.)")
	readCmd.Flags().StringVar(&readFormat, "format", "json", "Output format: json, markdown")
}
