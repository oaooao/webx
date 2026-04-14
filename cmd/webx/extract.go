package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/oaooao/webx/internal/core"
	"github.com/oaooao/webx/internal/types"
	"github.com/spf13/cobra"
)

var extractKind string

var extractCmd = &cobra.Command{
	Use:   "extract <url>",
	Short: "Extract structured data from a URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if extractKind == "" {
			return fmt.Errorf("--kind is required for extract")
		}
		k := types.WebxKind(extractKind)
		if !k.IsValid() {
			return fmt.Errorf("invalid kind: %s", extractKind)
		}

		envelope := core.RunExtract(args[0], &k)

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(envelope)
	},
}

func init() {
	extractCmd.Flags().StringVar(&extractKind, "kind", "", "Required: content kind to extract (conversation, thread, etc.)")
	_ = extractCmd.MarkFlagRequired("kind")
}
