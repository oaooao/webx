package claude

import (
	"fmt"
	"strings"

	backends_claude "github.com/oaooao/webx/internal/backends/claude"
	"github.com/oaooao/webx/internal/types"
)

// ClaudeExtractData is the structured data returned by Extract.
type ClaudeExtractData struct {
	Conversation backends_claude.Conversation `json:"conversation"`
}

type claudeAdapter struct{}

// New returns a new Claude share adapter.
func New() types.ExtractableAdapter {
	return &claudeAdapter{}
}

func (a *claudeAdapter) ID() string { return "claude-share" }

// Priority 91: higher than generic, very specific URL pattern.
func (a *claudeAdapter) Priority() int { return 91 }

func (a *claudeAdapter) Kinds() []types.WebxKind {
	return []types.WebxKind{types.KindConversation, types.KindArticle, types.KindMetadata}
}

// Match returns true for claude.ai/share/* URLs.
func (a *claudeAdapter) Match(ctx types.MatchContext) bool {
	host := ctx.URL.Hostname()
	if host != "claude.ai" {
		return false
	}
	return strings.HasPrefix(ctx.URL.Path, "/share/")
}

func (a *claudeAdapter) Read(ctx types.RunContext) (*types.NormalizedReadResult, error) {
	conv, backend, err := a.fetchAndParse(ctx, "adapter.read")
	if err != nil {
		return nil, err
	}

	markdown := backends_claude.RenderMarkdown(conv)

	return &types.NormalizedReadResult{
		Title:    types.StringPtr(conv.Title),
		Markdown: &markdown,
		HTML:     nil,
		Backend:  backend,
	}, nil
}

func (a *claudeAdapter) Extract(ctx types.RunContext) (*types.NormalizedExtractResult, error) {
	conv, backend, err := a.fetchAndParse(ctx, "adapter.extract")
	if err != nil {
		return nil, err
	}

	markdown := backends_claude.RenderMarkdown(conv)

	return &types.NormalizedExtractResult{
		Title:    types.StringPtr(conv.Title),
		Markdown: &markdown,
		HTML:     nil,
		Data:     ClaudeExtractData{Conversation: *conv},
		Backend:  backend,
	}, nil
}

// fetchAndParse is the shared core for Read and Extract.
func (a *claudeAdapter) fetchAndParse(ctx types.RunContext, step string) (*backends_claude.Conversation, string, error) {
	shareID := backends_claude.ExtractShareID(ctx.URL.Path)
	if shareID == "" {
		err := types.NewWebxError(types.ErrFetchFailed, "cannot extract share ID from URL path")
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceReasonFromError(err),
			Adapter: "claude-share",
			Detail:  err.Error(),
		})
		return nil, "", err
	}

	data, backend, err := backends_claude.FetchConversation(shareID)
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceReasonFromError(err),
			Adapter: "claude-share",
			Backend: backend,
			Detail:  err.Error(),
		})
		return nil, backend, err
	}

	conv, parseErr := backends_claude.ParseConversation(data, shareID)
	if parseErr != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceReasonFromError(parseErr),
			Adapter: "claude-share",
			Backend: backend,
			Detail:  parseErr.Error(),
		})
		return nil, backend, parseErr
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    step,
		Reason:  types.TraceRouteMatch,
		Adapter: "claude-share",
		Backend: backend,
		Detail:  fmt.Sprintf("parsed %d messages from %q", len(conv.Messages), conv.Title),
	})

	return conv, backend, nil
}
