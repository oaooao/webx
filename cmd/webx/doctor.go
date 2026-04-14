package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/oaooao/webx/internal/core"
	"github.com/oaooao/webx/internal/types"
	"github.com/spf13/cobra"
)

var doctorKind string

var doctorCmd = &cobra.Command{
	Use:   "doctor <url>",
	Short: "Diagnose routing and backend status for a URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var kind *types.WebxKind
		if doctorKind != "" {
			k := types.WebxKind(doctorKind)
			if !k.IsValid() {
				return fmt.Errorf("invalid kind: %s", doctorKind)
			}
			kind = &k
		}

		envelope := core.RunDoctor(args[0], kind)

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(envelope)
	},
}

func init() {
	doctorCmd.Flags().StringVar(&doctorKind, "kind", "", "Requested content kind (optional)")
}
