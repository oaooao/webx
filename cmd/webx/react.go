package main

import (
	"fmt"
	"os"

	"github.com/oaooao/webx/internal/core"
	"github.com/spf13/cobra"
)

var (
	reactReaction string
	reactConfirm  bool
	reactDryRun   bool
	reactFormat   string
)

var reactCmd = &cobra.Command{
	Use:   "react <url>",
	Short: "React to a post or tweet",
	Long: `Express a reaction (like, retweet, upvote, etc.) on a post identified by its URL.

Supported reactions:
  Twitter: like, retweet
  Reddit:  upvote, downvote, unvote

Examples:
  webx react https://x.com/user/status/123 --reaction like --confirm
  webx react https://reddit.com/r/golang/comments/abc123/ --reaction upvote --confirm`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetURL := args[0]

		if reactReaction == "" {
			return fmt.Errorf("--reaction is required (e.g. like, retweet, upvote, downvote)")
		}

		fmt.Fprintf(os.Stderr, "About to react %q on %s\n\n", reactReaction, targetURL)

		if reactDryRun {
			fmt.Fprintln(os.Stderr, "[dry-run] No action taken.")
			return nil
		}

		if !reactConfirm {
			if !promptConfirm() {
				fmt.Fprintln(os.Stderr, "Aborted.")
				return nil
			}
		}

		envelope := core.RunReact(targetURL, reactReaction)
		return printWriteResult(envelope, reactFormat)
	},
}

func init() {
	reactCmd.Flags().StringVar(&reactReaction, "reaction", "", "Reaction type: like, retweet (twitter) or upvote, downvote, unvote (reddit)")
	reactCmd.Flags().BoolVar(&reactConfirm, "confirm", false, "Skip interactive confirmation prompt")
	reactCmd.Flags().BoolVar(&reactDryRun, "dry-run", false, "Preview the operation without executing it")
	reactCmd.Flags().StringVar(&reactFormat, "format", "json", "Output format: json, markdown")
	_ = reactCmd.MarkFlagRequired("reaction")
}
