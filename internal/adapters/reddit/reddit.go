package reddit

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/oaooao/webx/internal/backends"
	"github.com/oaooao/webx/internal/types"
)

var redditPostIDRe = regexp.MustCompile(`/comments/([a-zA-Z0-9]+)`)

// RedditExtractData is the structured data returned by Extract.
type RedditExtractData struct {
	Post     backends.RedditPost          `json:"post"`
	Comments []backends.RedditCommentNode `json:"comments"`
}

type redditAdapter struct{}

// New returns a new Reddit adapter.
func New() types.ExtractableAdapter {
	return &redditAdapter{}
}

func (a *redditAdapter) ID() string { return "reddit" }

// Priority 90: higher than generic adapters, matched before fallback chains.
func (a *redditAdapter) Priority() int { return 90 }

func (a *redditAdapter) Kinds() []types.WebxKind {
	return []types.WebxKind{types.KindThread, types.KindComments, types.KindMetadata}
}

// Match returns true for all reddit.com subdomains (www, old, new, etc.).
func (a *redditAdapter) Match(ctx types.MatchContext) bool {
	host := ctx.URL.Hostname()
	return host == "reddit.com" || strings.HasSuffix(host, ".reddit.com")
}

func extractPostID(path string) string {
	m := redditPostIDRe.FindStringSubmatch(path)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// fetchReddit is the shared core for Read and Extract. It fetches, parses,
// and expands the Reddit thread.
func (a *redditAdapter) fetchReddit(ctx types.RunContext, step string) (*backends.RedditResult, error) {
	listings, err := backends.FetchRedditJSON(ctx.URL.String())
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceReasonFromError(err),
			Adapter: "reddit",
			Backend: "reddit_json",
			Detail:  err.Error(),
		})
		return nil, err
	}

	result, parseErr := backends.ParseRedditListings(listings)
	if parseErr != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceReasonFromError(parseErr),
			Adapter: "reddit",
			Backend: "reddit_json",
			Detail:  parseErr.Error(),
		})
		return nil, parseErr
	}

	// Attempt to expand "more" comments (no-op in v0, best-effort, never fails).
	pid := extractPostID(ctx.URL.Path)
	if pid != "" {
		backends.ExpandMoreComments(result, "t3_"+pid)
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    step,
		Reason:  types.TraceRouteMatch,
		Adapter: "reddit",
		Backend: "reddit_json",
		Detail:  fmt.Sprintf("parsed %d top-level comments", len(result.Comments)),
	})

	return result, nil
}

func (a *redditAdapter) Read(ctx types.RunContext) (*types.NormalizedReadResult, error) {
	result, err := a.fetchReddit(ctx, "adapter.read")
	if err != nil {
		return nil, err
	}

	markdown := backends.RenderRedditMarkdown(result)

	title := result.Post.Title
	if title == "" {
		title = "Reddit Thread"
	}

	return &types.NormalizedReadResult{
		Title:    types.StringPtr(title),
		Markdown: &markdown,
		HTML:     nil,
		Backend:  "reddit_json",
	}, nil
}

// Search implements types.SearchableAdapter for Reddit via the public search.json endpoint.
func (a *redditAdapter) Search(ctx types.SearchContext) (*types.NormalizedSearchResult, error) {
	limit := ctx.Options.Limit
	if limit <= 0 {
		limit = 20
	}

	apiURL := backends.BuildRedditSearchURL(ctx.Query, limit, ctx.Options.Sort)
	result, err := backends.SearchRedditPosts(apiURL, limit, ctx.Options.Sort)
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.search",
			Reason:  types.TraceReasonFromError(err),
			Adapter: "reddit",
			Backend: "reddit_json",
			Detail:  err.Error(),
		})
		return nil, err
	}

	result.Query = ctx.Query

	ctx.Trace.Push(types.TraceEvent{
		Step:    "adapter.search",
		Reason:  types.TraceRouteMatch,
		Adapter: "reddit",
		Backend: "reddit_json",
		Detail:  fmt.Sprintf("search returned %d results for %q", len(result.Items), ctx.Query),
	})

	return result, nil
}

func (a *redditAdapter) Extract(ctx types.RunContext) (*types.NormalizedExtractResult, error) {
	result, err := a.fetchReddit(ctx, "adapter.extract")
	if err != nil {
		return nil, err
	}

	markdown := backends.RenderRedditMarkdown(result)

	title := result.Post.Title
	if title == "" {
		title = "Reddit Thread"
	}

	return &types.NormalizedExtractResult{
		Title:    types.StringPtr(title),
		Markdown: &markdown,
		HTML:     nil,
		Data:     RedditExtractData{Post: result.Post, Comments: result.Comments},
		Backend:  "reddit_json",
	}, nil
}

