package hn

import (
	"fmt"

	"github.com/oaooao/webx/internal/backends"
	"github.com/oaooao/webx/internal/types"
)

type hnAdapter struct{}

// New returns a new Hacker News adapter.
func New() types.Adapter {
	return &hnAdapter{}
}

func (a *hnAdapter) ID() string { return "hacker-news" }

// Priority 88: above generic adapters, below reddit (90) to keep relative order clear.
func (a *hnAdapter) Priority() int { return 88 }

func (a *hnAdapter) Kinds() []types.WebxKind {
	return []types.WebxKind{types.KindArticle, types.KindComments, types.KindMetadata}
}

// Match returns true only for HN item URLs: https://news.ycombinator.com/item?id=<digits>
// This deliberately excludes /newest, /show, /ask, /jobs, etc.
func (a *hnAdapter) Match(ctx types.MatchContext) bool {
	return ctx.URL.Hostname() == "news.ycombinator.com" &&
		ctx.URL.Path == "/item" &&
		ctx.URL.Query().Get("id") != ""
}

func (a *hnAdapter) Read(ctx types.RunContext) (*types.NormalizedReadResult, error) {
	itemID := ctx.URL.Query().Get("id")
	if itemID == "" {
		return nil, types.NewWebxError(types.ErrNoMatch, "no HN item id in URL")
	}

	item, err := backends.FetchHNItem(itemID)
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.read",
			Reason:  types.TraceReasonFromError(err),
			Adapter: "hacker-news",
			Backend: "hn_algolia",
			Detail:  err.Error(),
		})
		// TODO(v1): fallback to go-defuddle on the original HN URL once defuddle
		// backend is available from Phase 2. For now, surface the error directly.
		return nil, err
	}

	markdown := backends.RenderHNItemMarkdown(item)
	if markdown == "" {
		err := types.NewWebxError(types.ErrContentEmpty, "HN Algolia returned empty content")
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.read",
			Reason:  types.TraceEmptyContent,
			Adapter: "hacker-news",
			Backend: "hn_algolia",
			Detail:  err.Error(),
		})
		return nil, err
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    "adapter.read",
		Reason:  types.TraceRouteMatch,
		Adapter: "hacker-news",
		Backend: "hn_algolia",
		Detail:  fmt.Sprintf("HN Algolia returned %d top-level comments", len(item.Children)),
	})

	title := item.Title
	if title == "" {
		title = fmt.Sprintf("HN item %d", item.ID)
	}

	return &types.NormalizedReadResult{
		Title:    types.StringPtr(title),
		Markdown: &markdown,
		HTML:     nil,
		Backend:  "hn_algolia",
	}, nil
}
