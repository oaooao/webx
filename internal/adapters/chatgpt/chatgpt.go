package chatgpt

import (
	"fmt"
	"strings"

	backends_chatgpt "github.com/oaooao/webx/internal/backends/chatgpt"
	"github.com/oaooao/webx/internal/types"
)

// ChatGPTExtractData is the structured data returned by Extract.
type ChatGPTExtractData struct {
	Conversation backends_chatgpt.Conversation `json:"conversation"`
}

type chatgptAdapter struct{}

// New returns a new ChatGPT share adapter.
func New() types.ExtractableAdapter {
	return &chatgptAdapter{}
}

func (a *chatgptAdapter) ID() string { return "chatgpt-share" }

// Priority 91: higher than Reddit (90), very specific URL pattern.
func (a *chatgptAdapter) Priority() int { return 91 }

func (a *chatgptAdapter) Kinds() []types.WebxKind {
	return []types.WebxKind{types.KindConversation, types.KindArticle, types.KindMetadata}
}

// Match returns true for chatgpt.com/share/* and chat.openai.com/share/* URLs.
func (a *chatgptAdapter) Match(ctx types.MatchContext) bool {
	host := ctx.URL.Hostname()
	if host != "chatgpt.com" && host != "chat.openai.com" {
		return false
	}
	return strings.HasPrefix(ctx.URL.Path, "/share/")
}

func (a *chatgptAdapter) Read(ctx types.RunContext) (*types.NormalizedReadResult, error) {
	conv, backend, err := a.fetchAndParse(ctx, "adapter.read")
	if err != nil {
		return nil, err
	}

	markdown := backends_chatgpt.RenderMarkdown(conv)

	return &types.NormalizedReadResult{
		Title:    types.StringPtr(conv.Title),
		Markdown: &markdown,
		HTML:     nil,
		Backend:  backend,
	}, nil
}

func (a *chatgptAdapter) Extract(ctx types.RunContext) (*types.NormalizedExtractResult, error) {
	conv, backend, err := a.fetchAndParse(ctx, "adapter.extract")
	if err != nil {
		return nil, err
	}

	markdown := backends_chatgpt.RenderMarkdown(conv)

	return &types.NormalizedExtractResult{
		Title:    types.StringPtr(conv.Title),
		Markdown: &markdown,
		HTML:     nil,
		Data:     ChatGPTExtractData{Conversation: *conv},
		Backend:  backend,
	}, nil
}

// fetchAndParse is the shared core for Read and Extract.
func (a *chatgptAdapter) fetchAndParse(ctx types.RunContext, step string) (*backends_chatgpt.Conversation, string, error) {
	shareID := backends_chatgpt.ExtractShareID(ctx.URL.Path)
	if shareID == "" {
		err := types.NewWebxError(types.ErrFetchFailed, "cannot extract share ID from URL path")
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceReasonFromError(err),
			Adapter: "chatgpt-share",
			Detail:  err.Error(),
		})
		return nil, "", err
	}

	data, backend, err := backends_chatgpt.FetchConversation(ctx.URL.String(), shareID)
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceReasonFromError(err),
			Adapter: "chatgpt-share",
			Backend: backend,
			Detail:  err.Error(),
		})
		return nil, backend, err
	}

	conv, parseErr := backends_chatgpt.ParseConversation(data, shareID)
	if parseErr != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceReasonFromError(parseErr),
			Adapter: "chatgpt-share",
			Backend: backend,
			Detail:  parseErr.Error(),
		})
		return nil, backend, parseErr
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    step,
		Reason:  types.TraceRouteMatch,
		Adapter: "chatgpt-share",
		Backend: backend,
		Detail:  fmt.Sprintf("parsed %d messages from %q", len(conv.Messages), conv.Title),
	})

	return conv, backend, nil
}
