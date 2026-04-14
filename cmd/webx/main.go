package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	// adapter registration side-effects
	_ "github.com/oaooao/webx/internal/adapters"
)

// Set by GoReleaser ldflags at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:     "webx",
	Short:   "One command to read any URL",
	Long:    "webx is a unified web runtime for AI agents. URL in, content out.",
	Version: version,
}

func main() {
	rootCmd.AddCommand(readCmd)
	rootCmd.AddCommand(extractCmd)
	rootCmd.AddCommand(doctorCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
