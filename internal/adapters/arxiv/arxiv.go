// Package arxiv provides an adapter for arxiv.org paper pages.
// It rewrites /abs/ URLs to /html/ to get the HTML-rendered paper,
// then uses defuddle or jina to extract clean content.
package arxiv

import (
	"fmt"
	"strings"

	"github.com/oaooao/webx/internal/backends"
	"github.com/oaooao/webx/internal/types"
)

const adapterID = "arxiv"

type arxivAdapter struct{}

// New returns a new arXiv adapter.
func New() types.ExtractableAdapter {
	return &arxivAdapter{}
}

func (a *arxivAdapter) ID() string { return adapterID }

// Priority 80: higher than generic (10) so arXiv URLs are handled specifically.
func (a *arxivAdapter) Priority() int { return 80 }

func (a *arxivAdapter) Kinds() []types.WebxKind {
	return []types.WebxKind{types.KindArticle}
}

// Match returns true for arxiv.org /abs/ and /pdf/ URLs.
func (a *arxivAdapter) Match(ctx types.MatchContext) bool {
	host := ctx.URL.Hostname()
	if host != "arxiv.org" && !strings.HasSuffix(host, ".arxiv.org") {
		return false
	}
	path := ctx.URL.Path
	return strings.HasPrefix(path, "/abs/") ||
		strings.HasPrefix(path, "/pdf/") ||
		strings.HasPrefix(path, "/html/")
}

// Read extracts the arXiv paper content.
// Strategy:
//  1. Rewrite /abs/<id> → /html/<id> for structured HTML rendering.
//  2. Try defuddle on the HTML URL.
//  3. Fall back to Jina on the original /abs/ URL if defuddle fails.
func (a *arxivAdapter) Read(ctx types.RunContext) (*types.NormalizedReadResult, error) {
	originalURL := ctx.URL.String()
	htmlURL := rewriteToHTML(ctx.URL.Path, originalURL)

	ctx.Trace.Push(types.TraceEvent{
		Step:    "adapter.rewrite",
		Reason:  types.TraceRouteMatch,
		Adapter: adapterID,
		Detail:  fmt.Sprintf("rewritten URL: %s", htmlURL),
	})

	// --- attempt 1: defuddle on HTML URL ---
	defResult, err := backends.RunDefuddle(htmlURL)
	if err == nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "backend.defuddle",
			Reason:  types.TraceRouteMatch,
			Adapter: adapterID,
			Backend: "defuddle",
			Detail:  "defuddle succeeded on /html/ URL",
		})
		return &types.NormalizedReadResult{
			Title:         ptrOf(defResult.Title),
			Markdown:      ptrOf(defResult.Markdown),
			HTML:          ptrOf(defResult.HTML),
			Backend:       "defuddle",
			FallbackDepth: 0,
		}, nil
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    "backend.defuddle",
		Reason:  types.TraceReasonFromError(err),
		Adapter: adapterID,
		Backend: "defuddle",
		Detail:  fmt.Sprintf("defuddle failed on %s: %s", htmlURL, err),
	})

	// --- attempt 2: jina on original /abs/ URL ---
	jinaResult, jinaErr := backends.FetchViaJina(originalURL)
	if jinaErr != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "backend.jina",
			Reason:  types.TraceReasonFromError(jinaErr),
			Adapter: adapterID,
			Backend: "jina",
			Detail:  jinaErr.Error(),
		})
		return nil, jinaErr
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    "backend.jina",
		Reason:  types.TraceRouteMatch,
		Adapter: adapterID,
		Backend: "jina",
		Detail:  "jina fallback succeeded",
	})

	return &types.NormalizedReadResult{
		Title:         ptrOf(jinaResult.Title),
		Markdown:      ptrOf(jinaResult.Markdown),
		Backend:       "jina",
		FallbackDepth: 1,
	}, nil
}

// rewriteToHTML converts /abs/<id> or /pdf/<id> paths to /html/<id>.
// If the path is already /html/ or unrecognised, it returns the original URL.
func rewriteToHTML(path, originalURL string) string {
	// Strip version suffix for /html/ which requires the bare arXiv ID.
	// e.g. /abs/2503.23350v2 → /html/2503.23350
	switch {
	case strings.HasPrefix(path, "/abs/"):
		id := strings.TrimPrefix(path, "/abs/")
		id = stripVersion(id)
		return "https://arxiv.org/html/" + id
	case strings.HasPrefix(path, "/pdf/"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/pdf/"), ".pdf")
		id = stripVersion(id)
		return "https://arxiv.org/html/" + id
	default:
		return originalURL
	}
}

// stripVersion removes a trailing version like "v2" from an arXiv paper ID.
func stripVersion(id string) string {
	if idx := strings.LastIndex(id, "v"); idx > 0 {
		suffix := id[idx+1:]
		allDigits := len(suffix) > 0
		for _, c := range suffix {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return id[:idx]
		}
	}
	return id
}

// ArxivExtractData is the structured data returned by Extract.
type ArxivExtractData struct {
	ArxivID string `json:"arxiv_id"`
	Title   string `json:"title"`
}

func (a *arxivAdapter) Extract(ctx types.RunContext) (*types.NormalizedExtractResult, error) {
	originalURL := ctx.URL.String()
	htmlURL := rewriteToHTML(ctx.URL.Path, originalURL)

	ctx.Trace.Push(types.TraceEvent{
		Step:    "adapter.rewrite",
		Reason:  types.TraceRouteMatch,
		Adapter: adapterID,
		Detail:  fmt.Sprintf("rewritten URL: %s", htmlURL),
	})

	// --- attempt 1: defuddle on HTML URL ---
	defResult, err := backends.RunDefuddle(htmlURL)
	if err == nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "backend.defuddle",
			Reason:  types.TraceRouteMatch,
			Adapter: adapterID,
			Backend: "defuddle",
			Detail:  "defuddle succeeded on /html/ URL",
		})
		return &types.NormalizedExtractResult{
			Title:    ptrOf(defResult.Title),
			Markdown: ptrOf(defResult.Markdown),
			HTML:     ptrOf(defResult.HTML),
			Data: ArxivExtractData{
				ArxivID: extractArxivID(ctx.URL.Path),
				Title:   defResult.Title,
			},
			Backend:       "defuddle",
			FallbackDepth: 0,
		}, nil
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    "backend.defuddle",
		Reason:  types.TraceReasonFromError(err),
		Adapter: adapterID,
		Backend: "defuddle",
		Detail:  fmt.Sprintf("defuddle failed on %s: %s", htmlURL, err),
	})

	// --- attempt 2: jina on original /abs/ URL ---
	jinaResult, jinaErr := backends.FetchViaJina(originalURL)
	if jinaErr != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "backend.jina",
			Reason:  types.TraceReasonFromError(jinaErr),
			Adapter: adapterID,
			Backend: "jina",
			Detail:  jinaErr.Error(),
		})
		return nil, jinaErr
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    "backend.jina",
		Reason:  types.TraceRouteMatch,
		Adapter: adapterID,
		Backend: "jina",
		Detail:  "jina fallback succeeded",
	})

	return &types.NormalizedExtractResult{
		Title:    ptrOf(jinaResult.Title),
		Markdown: ptrOf(jinaResult.Markdown),
		Data: ArxivExtractData{
			ArxivID: extractArxivID(ctx.URL.Path),
			Title:   jinaResult.Title,
		},
		Backend:       "jina",
		FallbackDepth: 1,
	}, nil
}

// extractArxivID pulls the paper ID from a URL path like /abs/2503.23350v2.
func extractArxivID(path string) string {
	for _, prefix := range []string{"/abs/", "/pdf/", "/html/"} {
		if strings.HasPrefix(path, prefix) {
			id := strings.TrimPrefix(path, prefix)
			id = strings.TrimSuffix(id, ".pdf")
			return id
		}
	}
	return ""
}

func ptrOf(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
