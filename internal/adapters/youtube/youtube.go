package youtube

import (
	"fmt"

	ytbe "github.com/oaooao/webx/internal/backends/youtube"
	"github.com/oaooao/webx/internal/types"
)

type youtubeAdapter struct{}

func New() types.ExtractableAdapter {
	return &youtubeAdapter{}
}

func (a *youtubeAdapter) ID() string   { return "youtube" }
func (a *youtubeAdapter) Priority() int { return 89 }

func (a *youtubeAdapter) Kinds() []types.WebxKind {
	return []types.WebxKind{types.KindVideo, types.KindArticle, types.KindMetadata}
}

func (a *youtubeAdapter) Match(ctx types.MatchContext) bool {
	host := ctx.URL.Hostname()
	return host == "youtube.com" || host == "www.youtube.com" ||
		host == "m.youtube.com" || host == "youtu.be"
}

func (a *youtubeAdapter) Read(ctx types.RunContext) (*types.NormalizedReadResult, error) {
	result, err := a.fetchVideo(ctx, "adapter.read")
	if err != nil {
		return nil, err
	}

	markdown := ytbe.RenderMarkdown(result)
	title := result.Video.Title

	return &types.NormalizedReadResult{
		Title:    &title,
		Markdown: &markdown,
		Backend:  "youtube_native",
	}, nil
}

// YoutubeExtractData is the structured data returned by Extract.
type YoutubeExtractData struct {
	Video      ytbe.VideoMeta          `json:"video"`
	Transcript []ytbe.TranscriptSegment `json:"transcript,omitempty"`
}

func (a *youtubeAdapter) Extract(ctx types.RunContext) (*types.NormalizedExtractResult, error) {
	result, err := a.fetchVideo(ctx, "adapter.extract")
	if err != nil {
		return nil, err
	}

	markdown := ytbe.RenderMarkdown(result)
	title := result.Video.Title

	data := YoutubeExtractData{
		Video:      result.Video,
		Transcript: result.Transcript,
	}

	return &types.NormalizedExtractResult{
		Title:    &title,
		Markdown: &markdown,
		Data:     data,
		Backend:  "youtube_native",
	}, nil
}

func (a *youtubeAdapter) fetchVideo(ctx types.RunContext, step string) (*ytbe.FetchResult, error) {
	result, err := ytbe.FetchVideo(ctx.URL.String())
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceReasonFromError(err),
			Adapter: "youtube",
			Backend: "youtube_native",
			Detail:  err.Error(),
		})
		return nil, err
	}

	transcriptNote := "no captions available"
	if len(result.Transcript) > 0 {
		transcriptNote = fmt.Sprintf("%d transcript segments", len(result.Transcript))
	}

	ctx.Trace.Push(types.TraceEvent{
		Step:    step,
		Reason:  types.TraceRouteMatch,
		Adapter: "youtube",
		Backend: "youtube_native",
		Detail:  fmt.Sprintf("fetched video %q by %s (%s)", result.Video.Title, result.Video.Channel, transcriptNote),
	})

	return result, nil
}
