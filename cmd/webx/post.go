package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/oaooao/webx/internal/core"
	"github.com/oaooao/webx/internal/types"
	"github.com/spf13/cobra"
)

var (
	postConfirm bool
	postDryRun  bool
	postFormat  string
)

var postCmd = &cobra.Command{
	Use:   "post <platform> <content>",
	Short: "Post content to a platform",
	Long: `Post new content to a supported platform.

Platform format:
  twitter          — post a tweet
  reddit/<subreddit> — post a self-post to a subreddit (first line = title)

Examples:
  webx post twitter "Hello world!" --confirm
  webx post reddit/golang "My Post Title\n\nPost body here." --confirm`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		platform := args[0]
		content := args[1]

		fmt.Fprintf(os.Stderr, "About to post to %s:\n  Content: %s\n\n", platform, truncate(content, 120))

		if postDryRun {
			fmt.Fprintln(os.Stderr, "[dry-run] No action taken.")
			return nil
		}

		if !postConfirm {
			if !promptConfirm() {
				fmt.Fprintln(os.Stderr, "Aborted.")
				return nil
			}
		}

		envelope := core.RunPost(platform, content)
		return printWriteResult(envelope, postFormat)
	},
}

func init() {
	postCmd.Flags().BoolVar(&postConfirm, "confirm", false, "Skip interactive confirmation prompt")
	postCmd.Flags().BoolVar(&postDryRun, "dry-run", false, "Preview the operation without executing it")
	postCmd.Flags().StringVar(&postFormat, "format", "json", "Output format: json, markdown")
}

// promptConfirm reads a y/N confirmation from stdin.
func promptConfirm() bool {
	fmt.Fprint(os.Stderr, "Confirm? [y/N]: ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.ToLower(strings.TrimSpace(scanner.Text())) == "y"
	}
	return false
}

// truncate shortens s to at most n runes, appending "…" if truncated.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// printWriteResult outputs the write envelope to stdout.
func printWriteResult(envelope types.WebxEnvelope, format string) error {
	switch format {
	case "markdown", "md":
		if envelope.Content.Markdown != nil {
			fmt.Print(*envelope.Content.Markdown)
			return nil
		}
		// fallback to JSON if no markdown
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(envelope)
	default:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(envelope)
	}
}
