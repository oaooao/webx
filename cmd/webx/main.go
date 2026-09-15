package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

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

func init() {
	// `go install ...@vX.Y.Z` builds without ldflags; fall back to the module version.
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = strings.TrimPrefix(info.Main.Version, "v")
		}
	}
	rootCmd.Version = version
}

func main() {
	rootCmd.AddCommand(readCmd)
	rootCmd.AddCommand(extractCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(postCmd)
	rootCmd.AddCommand(replyCmd)
	rootCmd.AddCommand(reactCmd)
	rootCmd.AddCommand(authCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
