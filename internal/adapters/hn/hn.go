package hn

import (
	"fmt"

	"github.com/oaooao/webx/internal/backends"
	"github.com/oaooao/webx/internal/types"
)

type hnAdapter struct{}

// New returns a new Hacker News adapter.
func New() types.ExtractableAdapter {
	return &hnAdapter{}
}

// Search implements types.SearchableAdapter for Hacker News via Algolia search API.
func (a *hnAdapter) Search(ctx types.SearchContext) (*types.NormalizedSearchResult, error) {
	limit := ctx.Options.Limit
	if limit <= 0 {
		limit = 20
	}

	resp, err := backends.SearchHNStories(ctx.Query, limit, ctx.Options.Sort)
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.search",
			Reason:  types.TraceReasonFromError(err),
			Adapter: "hacker-news",
			Backend: "hn_algolia",
			Detail:  err.Error(),
		})
		return nil, err
	}

	items := make([]types.SearchResultItem, 0, len(resp.Hits))
	for _, hit := range resp.Hits {
		itemURL := fmt.Sprintf("https://news.ycombinator.com/item?id=%s", hit.ObjectID)
		snippet := hit.StoryText
		if len(snippet) > 300 {
			snippet = snippet[:300] + "..."
		}
		items = append(items, types.SearchResultItem{
			Title:   hit.Title,
			URL:     itemURL,
			Snippet: snippet,
			Author:  hit.Author,
			Date:    hit.CreatedAt,
			Score:   float64(hit.Points),
			Kind:    types.KindThread,
			Meta: map[string]any{
				"num_comments":  hit.NumComments,
				"external_url":  hit.URL,
			},
		})
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    "adapter.search",
		Reason:  types.TraceRouteMatch,
		Adapter: "hacker-news",
		Backend: "hn_algolia",
		Detail:  fmt.Sprintf("search returned %d results for %q", len(items), ctx.Query),
	})

	hasMore := resp.Page+1 < resp.NbPages

	return &types.NormalizedSearchResult{
		Items:         items,
		Query:         ctx.Query,
		TotalEstimate: resp.NbHits,
		Backend:       "hn_algolia",
		HasMore:       hasMore,
	}, nil
}

func (a *hnAdapter) ID() string { return "hacker-news" }

// Priority 88: above generic adapters, below reddit (90) to keep relative order clear.
func (a *hnAdapter) Priority() int { return 88 }

func (a *hnAdapter) Kinds() []types.WebxKind {
	return []types.WebxKind{types.KindThread, types.KindArticle, types.KindComments, types.KindMetadata}
}

// Match returns true only for HN item URLs: https://news.ycombinator.com/item?id=<digits>
// This deliberately excludes /newest, /show, /ask, /jobs, etc.
func (a *hnAdapter) Match(ctx types.MatchContext) bool {
	return ctx.URL.Hostname() == "news.ycombinator.com" &&
		ctx.URL.Path == "/item" &&
		ctx.URL.Query().Get("id") != ""
}

func (a *hnAdapter) Read(ctx types.RunContext) (*types.NormalizedReadResult, error) {
	item, err := a.fetchItem(ctx)
	if err != nil {
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

	title := itemTitle(item)

	return &types.NormalizedReadResult{
		Title:    types.StringPtr(title),
		Markdown: &markdown,
		HTML:     nil,
		Backend:  "hn_algolia",
	}, nil
}

// HNExtractData is the structured data returned by Extract.
type HNExtractData struct {
	Story    HNStoryMeta   `json:"story"`
	Comments []HNCommentNode `json:"comments"`
}

// HNStoryMeta holds the story-level metadata.
type HNStoryMeta struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	URL       string `json:"url,omitempty"`
	Author    string `json:"author"`
	Points    *int   `json:"points,omitempty"`
	CreatedAt string `json:"created_at"`
}

// HNCommentNode is a recursive comment tree node.
type HNCommentNode struct {
	ID        int             `json:"id"`
	Author    string          `json:"author"`
	Text      string          `json:"text,omitempty"`
	CreatedAt string          `json:"created_at"`
	Children  []HNCommentNode `json:"children,omitempty"`
}

func (a *hnAdapter) Extract(ctx types.RunContext) (*types.NormalizedExtractResult, error) {
	item, err := a.fetchItem(ctx)
	if err != nil {
		return nil, err
	}

	markdown := backends.RenderHNItemMarkdown(item)
	title := itemTitle(item)

	data := HNExtractData{
		Story: HNStoryMeta{
			ID:        item.ID,
			Title:     item.Title,
			URL:       item.URL,
			Author:    item.Author,
			Points:    item.Points,
			CreatedAt: item.CreatedAt,
		},
		Comments: convertComments(item.Children),
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    "adapter.extract",
		Reason:  types.TraceRouteMatch,
		Adapter: "hacker-news",
		Backend: "hn_algolia",
		Detail:  fmt.Sprintf("extracted story + %d top-level comments", len(item.Children)),
	})

	return &types.NormalizedExtractResult{
		Title:    types.StringPtr(title),
		Markdown: &markdown,
		Data:     data,
		Backend:  "hn_algolia",
	}, nil
}

// fetchItem retrieves the HN item, handling validation and tracing.
func (a *hnAdapter) fetchItem(ctx types.RunContext) (*backends.HNItem, error) {
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
		return nil, err
	}
	return item, nil
}

func itemTitle(item *backends.HNItem) string {
	if item.Title != "" {
		return item.Title
	}
	return fmt.Sprintf("HN item %d", item.ID)
}

func convertComments(items []backends.HNItem) []HNCommentNode {
	if len(items) == 0 {
		return nil
	}
	nodes := make([]HNCommentNode, len(items))
	for i, c := range items {
		text := ""
		if c.Text != nil {
			text = backends.StripHTMLTags(*c.Text)
		}
		nodes[i] = HNCommentNode{
			ID:        c.ID,
			Author:    c.Author,
			Text:      text,
			CreatedAt: c.CreatedAt,
			Children:  convertComments(c.Children),
		}
	}
	return nodes
}
