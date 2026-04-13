package reddit

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/oaooao/webx/internal/backends"
	"github.com/oaooao/webx/internal/types"
)

var redditPostIDRe = regexp.MustCompile(`/comments/([a-zA-Z0-9]+)`)

type redditAdapter struct{}

// New returns a new Reddit adapter.
func New() types.Adapter {
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

func (a *redditAdapter) Read(ctx types.RunContext) (*types.NormalizedReadResult, error) {
	listings, err := backends.FetchRedditJSON(ctx.URL.String())
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.read",
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
			Step:    "adapter.read",
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

	markdown := backends.RenderRedditMarkdown(result)

	ctx.Trace.Push(types.TraceEvent{
		Step:    "adapter.read",
		Reason:  types.TraceRouteMatch,
		Adapter: "reddit",
		Backend: "reddit_json",
		Detail:  fmt.Sprintf("parsed %d top-level comments", len(result.Comments)),
	})

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
