package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/oaooao/webx/internal/core"
	"github.com/spf13/cobra"
)

var (
	replyConfirm bool
	replyDryRun  bool
	replyFormat  string
)

var replyCmd = &cobra.Command{
	Use:   "reply <url> <content>",
	Short: "Reply to a post or tweet",
	Long: `Post a reply to an existing post identified by its URL.

Supported platforms:
  twitter  — reply to a tweet (x.com/user/status/<id>)
  reddit   — comment on a post or comment (reddit.com/r/.../comments/...)

Examples:
  webx reply https://x.com/user/status/123 "Great point!" --confirm
  webx reply https://reddit.com/r/golang/comments/abc123/title/ "Nice post!" --confirm`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetURL := args[0]
		content := args[1]

		fmt.Fprintf(os.Stderr, "About to reply on %s:\n  Target: %s\n  Content: %s\n\n",
			inferPlatform(targetURL), targetURL, truncate(content, 120))

		if replyDryRun {
			fmt.Fprintln(os.Stderr, "[dry-run] No action taken.")
			return nil
		}

		if !replyConfirm {
			if !promptConfirm() {
				fmt.Fprintln(os.Stderr, "Aborted.")
				return nil
			}
		}

		envelope := core.RunReply(targetURL, content)
		return printWriteResult(envelope, replyFormat)
	},
}

func init() {
	replyCmd.Flags().BoolVar(&replyConfirm, "confirm", false, "Skip interactive confirmation prompt")
	replyCmd.Flags().BoolVar(&replyDryRun, "dry-run", false, "Preview the operation without executing it")
	replyCmd.Flags().StringVar(&replyFormat, "format", "json", "Output format: json, markdown")
}

// inferPlatform returns a human-readable platform name from a URL.
func inferPlatform(rawURL string) string {
	switch {
	case strings.Contains(rawURL, "x.com") || strings.Contains(rawURL, "twitter.com"):
		return "twitter"
	case strings.Contains(rawURL, "reddit.com"):
		return "reddit"
	default:
		return "unknown"
	}
}
