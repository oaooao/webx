package backends

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/oaooao/webx/internal/types"
)

const (
	jinaBaseURL = "https://r.jina.ai/"
	jinaTimeout = 45 * time.Second
)

// JinaResult holds the output of a successful Jina reader API call.
type JinaResult struct {
	Title    string
	Markdown string
}

// FetchViaJina retrieves a URL's content using the r.jina.ai reader API.
// Jina strips ads/navigation and returns clean Markdown, making it a good
// fallback when go-defuddle fails (e.g. heavy JS pages).
//
// Uses http.DefaultClient (not uTLS) since r.jina.ai is a public API that
// doesn't apply fingerprint-based blocking.
func FetchViaJina(rawURL string) (*JinaResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), jinaTimeout)
	defer cancel()

	jinaURL := jinaBaseURL + rawURL

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jinaURL, nil)
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed,
			fmt.Sprintf("jina: build request: %s", err))
	}

	req.Header.Set("Accept", "text/plain,text/markdown,*/*")
	req.Header.Set("User-Agent", "webx/0.1 (+https://github.com/oaooao/webx)")
	req.Header.Set("X-Return-Format", "markdown")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, types.NewWebxError(types.ErrFetchTimeout,
				fmt.Sprintf("jina: GET %s timed out", rawURL))
		}
		return nil, types.NewWebxError(types.ErrFetchFailed,
			fmt.Sprintf("jina: GET %s: %s", rawURL, err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, types.NewWebxError(types.ErrRateLimited,
			fmt.Sprintf("jina: rate limited (429) for %s", rawURL))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, types.NewWebxError(types.ErrBackendFailed,
			fmt.Sprintf("jina: HTTP %d for %s", resp.StatusCode, rawURL))
	}

	limited := io.LimitReader(resp.Body, maxBodyBytes)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed,
			fmt.Sprintf("jina: read body: %s", err))
	}

	markdown := strings.TrimSpace(string(body))
	if markdown == "" {
		return nil, types.NewWebxError(types.ErrContentEmpty,
			fmt.Sprintf("jina: empty response for %s", rawURL))
	}

	// Jina prepends a "Title: ..." line; extract it if present.
	title, markdown := extractJinaTitle(markdown)

	return &JinaResult{
		Title:    title,
		Markdown: markdown,
	}, nil
}

// extractJinaTitle parses the optional "Title: <text>\n" prefix that Jina
// sometimes prepends. Returns (title, remainingMarkdown).
func extractJinaTitle(md string) (string, string) {
	const prefix = "Title: "
	if !strings.HasPrefix(md, prefix) {
		return "", md
	}
	newline := strings.IndexByte(md, '\n')
	if newline == -1 {
		return strings.TrimPrefix(md, prefix), ""
	}
	title := strings.TrimSpace(md[len(prefix):newline])
	rest := strings.TrimSpace(md[newline+1:])
	return title, rest
}
