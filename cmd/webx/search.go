package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/oaooao/webx/internal/core"
	"github.com/oaooao/webx/internal/types"
	"github.com/spf13/cobra"
)

var (
	searchPlatform string
	searchLimit    int
	searchSort     string
	searchFormat   string
)

var searchCmd = &cobra.Command{
	Use:   "search <query> --platform <platform>",
	Short: "Search a platform for content",
	Long:  "Search a specific platform (twitter, reddit, hacker-news, youtube) for content matching the query.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if searchPlatform == "" {
			return fmt.Errorf("--platform is required for search")
		}

		opts := types.SearchOptions{
			Limit: searchLimit,
			Sort:  searchSort,
		}

		envelope := core.RunSearch(args[0], searchPlatform, opts)

		switch searchFormat {
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
	searchCmd.Flags().StringVar(&searchPlatform, "platform", "", "Platform to search (twitter, reddit, hacker-news, youtube)")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 20, "Maximum number of results")
	searchCmd.Flags().StringVar(&searchSort, "sort", "relevance", "Sort order: relevance, recent, top")
	searchCmd.Flags().StringVar(&searchFormat, "format", "json", "Output format: json, markdown")
	_ = searchCmd.MarkFlagRequired("platform")
}
