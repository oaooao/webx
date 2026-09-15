package twitter

import (
	"fmt"
	"strings"
)

// RenderMarkdown converts a slice of parsed tweets into a readable Markdown
// document. The first tweet is treated as the focal tweet; subsequent tweets
// are rendered as replies.
func RenderMarkdown(tweets []Tweet) string {
	var sb strings.Builder
	for i, tweet := range tweets {
		writeTweet(&sb, i, tweet, nil)
	}
	return sb.String()
}

// RenderConversationMarkdown renders a split conversation: ancestors, then
// the focal tweet with the author's thread continuations appended to its body
// as plain paragraphs, then the replies.
func RenderConversationMarkdown(c Conversation) string {
	if c.Focal.ID == "" {
		return ""
	}
	var sb strings.Builder
	i := 0
	for _, t := range c.Ancestors {
		writeTweet(&sb, i, t, nil)
		i++
	}
	writeTweet(&sb, i, c.Focal, c.Thread)
	i++
	for _, t := range c.Replies {
		writeTweet(&sb, i, t, nil)
		i++
	}
	return sb.String()
}

// writeTweet renders one tweet block. index 0 gets the document heading;
// later blocks are replies. thread tweets are continuations of this tweet's
// body and are rendered inline without their own heading or metrics.
func writeTweet(sb *strings.Builder, index int, tweet Tweet, thread []Tweet) {
	if index > 0 {
		sb.WriteString("\n---\n\n")
	}

	if index == 0 {
		fmt.Fprintf(sb, "# Tweet by @%s\n\n", authorHandle(tweet))
	} else {
		fmt.Fprintf(sb, "## Reply by @%s\n\n", authorHandle(tweet))
	}

	// Author line with display name if available
	if tweet.Author.Name != "" && tweet.Author.ScreenName != "" {
		fmt.Fprintf(sb, "**%s** (@%s)", tweet.Author.Name, tweet.Author.ScreenName)
	} else if tweet.Author.ScreenName != "" {
		fmt.Fprintf(sb, "@%s", tweet.Author.ScreenName)
	} else if tweet.Author.Name != "" {
		sb.WriteString(tweet.Author.Name)
	}
	if tweet.CreatedAt != "" {
		fmt.Fprintf(sb, " · %s", tweet.CreatedAt)
	}
	sb.WriteString("\n\n")

	writeTweetBody(sb, tweet)
	for _, t := range thread {
		sb.WriteString("\n")
		writeTweetBody(sb, t)
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

// writeTweetBody renders a tweet's text, media, article and quoted tweet.
func writeTweetBody(sb *strings.Builder, tweet Tweet) {
	sb.WriteString(tweet.Text)
	sb.WriteString("\n")

	// Media attachments
	for _, m := range tweet.Media {
		sb.WriteString("\n")
		switch m.Type {
		case "photo":
			fmt.Fprintf(sb, "![photo](%s)\n", m.URL)
		case "video":
			fmt.Fprintf(sb, "[video](%s)\n", m.URL)
		case "animated_gif":
			fmt.Fprintf(sb, "[gif](%s)\n", m.URL)
		default:
			fmt.Fprintf(sb, "[media: %s](%s)\n", m.Type, m.URL)
		}
	}

	// X Article body (long-form content attached to the tweet).
	if tweet.ArticleTitle != "" || tweet.ArticleText != "" {
		sb.WriteString("\n")
		if tweet.ArticleTitle != "" {
			fmt.Fprintf(sb, "### Article: %s\n\n", tweet.ArticleTitle)
		} else {
			sb.WriteString("### Article\n\n")
		}
		if tweet.ArticleText != "" {
			sb.WriteString(tweet.ArticleText)
			sb.WriteString("\n")
		}
	}

	// Quoted tweet (inline block)
	if qt := tweet.QuotedTweet; qt != nil {
		sb.WriteString("\n> **Quoted:** @")
		sb.WriteString(authorHandle(*qt))
		sb.WriteString("\n>\n> ")
		quotedLines := strings.Split(qt.Text, "\n")
		sb.WriteString(strings.Join(quotedLines, "\n> "))
		sb.WriteString("\n")
	}
}

// RenderArticleMarkdown renders a standalone X Article read from its
// /i/article/<id> link: title, byline, then the article body.
func RenderArticleMarkdown(tweet Tweet) string {
	var sb strings.Builder
	title := tweet.ArticleTitle
	if title == "" {
		title = "X Article by @" + authorHandle(tweet)
	}
	fmt.Fprintf(&sb, "# %s\n\n", title)
	if tweet.Author.Name != "" && tweet.Author.ScreenName != "" {
		fmt.Fprintf(&sb, "**%s** (@%s)", tweet.Author.Name, tweet.Author.ScreenName)
	} else {
		fmt.Fprintf(&sb, "@%s", authorHandle(tweet))
	}
	if tweet.CreatedAt != "" {
		fmt.Fprintf(&sb, " · %s", tweet.CreatedAt)
	}
	sb.WriteString("\n\n")
	sb.WriteString(tweet.ArticleText)
	sb.WriteString("\n")
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
