// Package generic provides a catch-all adapter for arbitrary web articles.
// It attempts extraction via go-defuddle first; on failure, falls back to
// the r.jina.ai reader API.
package generic

import (
	"github.com/oaooao/webx/internal/backends"
	"github.com/oaooao/webx/internal/types"
)

const adapterID = "generic-article"

type genericAdapter struct{}

// New returns a new generic article adapter.
func New() types.Adapter {
	return &genericAdapter{}
}

func (a *genericAdapter) ID() string { return adapterID }

// Priority 10: lowest priority so specific adapters take precedence.
func (a *genericAdapter) Priority() int { return 10 }

func (a *genericAdapter) Kinds() []types.WebxKind {
	return []types.WebxKind{types.KindArticle}
}

// Match accepts any URL with an http/https scheme. Since Priority is 10,
// specific adapters will match first.
func (a *genericAdapter) Match(ctx types.MatchContext) bool {
	scheme := ctx.URL.Scheme
	return scheme == "http" || scheme == "https"
}

func (a *genericAdapter) Read(ctx types.RunContext) (*types.NormalizedReadResult, error) {
	rawURL := ctx.URL.String()

	// --- attempt 1: defuddle ---
	defResult, err := backends.RunDefuddle(rawURL)
	if err == nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "backend.defuddle",
			Reason:  types.TraceRouteMatch,
			Adapter: adapterID,
			Backend: "defuddle",
			Detail:  "defuddle succeeded",
		})
		return &types.NormalizedReadResult{
			Title:         ptrOf(defResult.Title),
			Markdown:      ptrOf(defResult.Markdown),
			HTML:          ptrOf(defResult.HTML),
			Backend:       "defuddle",
			FallbackDepth: 0,
		}, nil
	}

	// log defuddle failure and try jina
	ctx.Trace.Push(types.TraceEvent{
		Step:    "backend.defuddle",
		Reason:  types.TraceReasonFromError(err),
		Adapter: adapterID,
		Backend: "defuddle",
		Detail:  err.Error(),
	})

	// --- attempt 2: jina fallback ---
	jinaResult, jinaErr := backends.FetchViaJina(rawURL)
	if jinaErr != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "backend.jina",
			Reason:  types.TraceReasonFromError(jinaErr),
			Adapter: adapterID,
			Backend: "jina",
			Detail:  jinaErr.Error(),
		})
		// Return the more informative of the two errors; prefer the jina error
		// unless defuddle hit anti-bot (which jina will also struggle with).
		return nil, jinaErr
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    "backend.jina",
		Reason:  types.TraceRouteMatch,
		Adapter: adapterID,
		Backend: "jina",
		Detail:  "jina fallback succeeded",
	})

	title := jinaResult.Title
	return &types.NormalizedReadResult{
		Title:         ptrOf(title),
		Markdown:      ptrOf(jinaResult.Markdown),
		Backend:       "jina",
		FallbackDepth: 1,
	}, nil
}

func ptrOf(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
