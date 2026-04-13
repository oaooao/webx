package twitter

import (
	"fmt"
	"strings"
)

// RenderMarkdown converts a slice of parsed tweets into a readable Markdown
// document. The first tweet is treated as the focal tweet; subsequent tweets
// are replies or thread continuations.
func RenderMarkdown(tweets []Tweet) string {
	if len(tweets) == 0 {
		return ""
	}

	var sb strings.Builder

	for i, tweet := range tweets {
		if i > 0 {
			sb.WriteString("\n---\n\n")
		}

		// Header: focal tweet vs reply
		if i == 0 {
			fmt.Fprintf(&sb, "# Tweet by @%s\n\n", authorHandle(tweet))
		} else {
			fmt.Fprintf(&sb, "## Reply by @%s\n\n", authorHandle(tweet))
		}

		// Author line with display name if available
		if tweet.Author.Name != "" && tweet.Author.ScreenName != "" {
			fmt.Fprintf(&sb, "**%s** (@%s)", tweet.Author.Name, tweet.Author.ScreenName)
		} else if tweet.Author.ScreenName != "" {
			fmt.Fprintf(&sb, "@%s", tweet.Author.ScreenName)
		} else if tweet.Author.Name != "" {
			sb.WriteString(tweet.Author.Name)
		}
		if tweet.CreatedAt != "" {
			fmt.Fprintf(&sb, " · %s", tweet.CreatedAt)
		}
		sb.WriteString("\n\n")

		// Tweet body
		sb.WriteString(tweet.Text)
		sb.WriteString("\n")

		// Media attachments
		for _, m := range tweet.Media {
			sb.WriteString("\n")
			switch m.Type {
			case "photo":
				fmt.Fprintf(&sb, "![photo](%s)\n", m.URL)
			case "video":
				fmt.Fprintf(&sb, "[video](%s)\n", m.URL)
			case "animated_gif":
				fmt.Fprintf(&sb, "[gif](%s)\n", m.URL)
			default:
				fmt.Fprintf(&sb, "[media: %s](%s)\n", m.Type, m.URL)
			}
		}

		// Quoted tweet (inline block)
		if qt := tweet.QuotedTweet; qt != nil {
			sb.WriteString("\n> **Quoted:** @")
			sb.WriteString(authorHandle(*qt))
			sb.WriteString("\n>\n> ")
			// Indent each line of the quoted text
			quotedLines := strings.Split(qt.Text, "\n")
			sb.WriteString(strings.Join(quotedLines, "\n> "))
			sb.WriteString("\n")
		}

		// Engagement metrics (non-zero only)
		if len(tweet.Metrics) > 0 {
			parts := make([]string, 0, 5)
			metricOrder := []string{"reply_count", "retweet_count", "quote_count", "favorite_count", "bookmark_count"}
			metricLabel := map[string]string{
				"reply_count":    "replies",
				"retweet_count":  "retweets",
				"quote_count":    "quotes",
				"favorite_count": "likes",
				"bookmark_count": "bookmarks",
			}
			for _, k := range metricOrder {
				if v, ok := tweet.Metrics[k]; ok && v > 0 {
					parts = append(parts, fmt.Sprintf("%d %s", v, metricLabel[k]))
				}
			}
			if len(parts) > 0 {
				sb.WriteString("\n*")
				sb.WriteString(strings.Join(parts, " · "))
				sb.WriteString("*\n")
			}
		}
	}

	return sb.String()
}

// authorHandle returns the best-effort display handle for a tweet.
func authorHandle(t Tweet) string {
	if t.Author.ScreenName != "" {
		return t.Author.ScreenName
	}
	if t.Author.Name != "" {
		return t.Author.Name
	}
	return "unknown"
}
