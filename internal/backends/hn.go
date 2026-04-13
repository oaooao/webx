package backends

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/oaooao/webx/internal/types"
)

const hnTimeout = 15 * time.Second

// HNItem represents a Hacker News item as returned by the Algolia API.
// Both stories and comments use this same struct; stories have Title/URL,
// comments have Text. Children holds nested replies recursively.
type HNItem struct {
	ID        int      `json:"id"`
	Title     string   `json:"title"`
	URL       string   `json:"url"`
	Text      *string  `json:"text"`
	Author    string   `json:"author"`
	Points    *int     `json:"points"`
	CreatedAt string   `json:"created_at"`
	Children  []HNItem `json:"children"`
}

// FetchHNItem fetches a Hacker News item from the Algolia Items API.
// The Algolia API returns nested children in a single request, avoiding
// multiple round-trips that HN's official Firebase API would require.
func FetchHNItem(itemID string) (*HNItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hnTimeout)
	defer cancel()

	apiURL := fmt.Sprintf("https://hn.algolia.com/api/v1/items/%s", itemID)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, err.Error())
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "webx/0.1 (+https://github.com/oaooao/webx)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, types.NewWebxError(types.ErrFetchTimeout, "HN Algolia API timed out")
		}
		return nil, types.NewWebxError(types.ErrFetchFailed, err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, types.NewWebxError(types.ErrFetchFailed, "HN Algolia API HTTP "+resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, err.Error())
	}

	var item HNItem
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, types.NewWebxError(types.ErrBackendFailed, "HN Algolia returned invalid JSON")
	}

	return &item, nil
}

// HTML tag stripping patterns for HN comment HTML.
// HN comment HTML is simple: <p>, <a href>, <code>, <pre><code>, <i>/<em>.
// We use regex rather than a full HTML parser to keep dependencies minimal;
// for more complex HTML, golang.org/x/net/html tokenizer would be preferable.
var (
	htmlTagRe    = regexp.MustCompile(`<[^>]+>`)
	htmlEntityRe = regexp.MustCompile(`&(amp|lt|gt|quot|#39|#x27|apos);`)
	aTagRe       = regexp.MustCompile(`(?i)<a\s+href="([^"]*)"[^>]*>(.*?)</a>`)
	italicOpenRe = regexp.MustCompile(`(?i)<(i|em)>`)
	italicCloseRe = regexp.MustCompile(`(?i)</(i|em)>`)
)

var htmlEntityMap = map[string]string{
	"amp":  "&",
	"lt":   "<",
	"gt":   ">",
	"quot": `"`,
	"#39":  "'",
	"#x27": "'",
	"apos": "'",
}

// StripHTMLTags converts HN comment HTML to plain Markdown text.
// Conversion rules:
//   - <p>        → double newline
//   - <a href>   → [text](url)
//   - <pre><code>/<code> → fenced/inline code
//   - <i>/<em>   → *italic*
//   - everything else: strip tags
//   - HTML entities decoded
func StripHTMLTags(html string) string {
	s := html

	// <p> → paragraph break
	s = strings.ReplaceAll(s, "<p>", "\n\n")
	s = strings.ReplaceAll(s, "</p>", "")

	// <pre><code> → fenced code block
	s = strings.ReplaceAll(s, "<pre><code>", "\n```\n")
	s = strings.ReplaceAll(s, "</code></pre>", "\n```\n")

	// inline <code>
	s = strings.ReplaceAll(s, "<code>", "`")
	s = strings.ReplaceAll(s, "</code>", "`")

	// <a href="url">text</a> → [text](url)
	s = aTagRe.ReplaceAllString(s, "[$2]($1)")

	// <i>/<em> → *text*
	s = italicOpenRe.ReplaceAllString(s, "*")
	s = italicCloseRe.ReplaceAllString(s, "*")

	// strip remaining tags
	s = htmlTagRe.ReplaceAllString(s, "")

	// decode HTML entities
	s = htmlEntityRe.ReplaceAllStringFunc(s, func(match string) string {
		entity := match[1 : len(match)-1]
		if r, ok := htmlEntityMap[entity]; ok {
			return r
		}
		return match
	})

	return strings.TrimSpace(s)
}

// RenderHNItemMarkdown converts an HNItem (story + nested comments) to Markdown.
func RenderHNItemMarkdown(item *HNItem) string {
	var sb strings.Builder

	title := item.Title
	if title == "" {
		title = fmt.Sprintf("HN item %d", item.ID)
	}
	fmt.Fprintf(&sb, "# %s\n\n", title)

	if item.URL != "" {
		fmt.Fprintf(&sb, "URL: %s\n", item.URL)
	}
	if item.Author != "" {
		fmt.Fprintf(&sb, "Author: %s\n", item.Author)
	}
	if item.Points != nil {
		fmt.Fprintf(&sb, "Points: %d\n", *item.Points)
	}
	if item.CreatedAt != "" {
		fmt.Fprintf(&sb, "Created: %s\n", item.CreatedAt)
	}

	if item.Text != nil && *item.Text != "" {
		fmt.Fprintf(&sb, "\n## Post Text\n\n%s\n", StripHTMLTags(*item.Text))
	}

	if len(item.Children) > 0 {
		sb.WriteString("\n## Comments\n\n")
		for _, child := range item.Children {
			renderHNComment(&sb, child, 0)
		}
	}

	return sb.String()
}

func renderHNComment(sb *strings.Builder, item HNItem, depth int) {
	indent := strings.Repeat("  ", depth)
	author := item.Author
	if author == "" {
		author = "[deleted]"
	}

	fmt.Fprintf(sb, "%s- **%s**", indent, author)
	if item.Points != nil {
		fmt.Fprintf(sb, " (%d points)", *item.Points)
	}
	sb.WriteString(":\n")

	if item.Text != nil && *item.Text != "" {
		text := StripHTMLTags(*item.Text)
		for _, line := range strings.Split(text, "\n") {
			fmt.Fprintf(sb, "%s  %s\n", indent, line)
		}
	}
	sb.WriteString("\n")

	for _, child := range item.Children {
		renderHNComment(sb, child, depth+1)
	}
}
