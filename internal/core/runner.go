package core

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/oaooao/webx/internal/types"
)

func makeMatchContext(rawURL string, requestedKind *types.WebxKind) (types.MatchContext, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return types.MatchContext{}, err
	}
	return types.MatchContext{
		URL:           parsed,
		RequestedKind: requestedKind,
	}, nil
}

func defaultKind(requestedKind *types.WebxKind, adapter types.Adapter) types.WebxKind {
	if requestedKind != nil {
		return *requestedKind
	}
	kinds := adapter.Kinds()
	if len(kinds) > 0 {
		return kinds[0]
	}
	return types.KindMetadata
}

func RunDoctor(rawURL string, requestedKind *types.WebxKind) types.WebxEnvelope {
	trace := types.NewTraceBuffer()
	ctx, err := makeMatchContext(rawURL, requestedKind)
	if err != nil {
		trace.Push(types.TraceEvent{Step: "parse", Reason: types.TraceBackendFailed, Detail: err.Error()})
		kind := types.KindMetadata
		if requestedKind != nil {
			kind = *requestedKind
		}
		return types.MakeEnvelope(types.EnvelopeInput{
			OK: false, Kind: kind, URL: rawURL,
			Adapter: "none", Backend: "none",
			Trace: trace.All(),
			Error: &types.EnvelopeError{Code: string(types.ErrFetchFailed), Message: err.Error()},
		})
	}

	adapter := Route(ctx)
	if adapter == nil {
		trace.Push(types.TraceEvent{
			Step: "route", Reason: types.TraceNoMatch,
			Detail: "No adapter matched this URL/kind combination",
		})
		kind := types.KindMetadata
		if requestedKind != nil {
			kind = *requestedKind
		}
		return types.MakeEnvelope(types.EnvelopeInput{
			OK: false, Kind: kind, URL: rawURL,
			Adapter: "none", Backend: "none",
			Trace: trace.All(),
			Error: &types.EnvelopeError{Code: string(types.ErrNoMatch), Message: "No adapter matched URL: " + rawURL},
		})
	}

	trace.Push(types.TraceEvent{
		Step: "route", Reason: types.TraceRouteMatch,
		Adapter: adapter.ID(),
		Detail:  "Matched adapter " + adapter.ID(),
	})

	return types.MakeEnvelope(types.EnvelopeInput{
		OK: true, Kind: defaultKind(requestedKind, adapter), URL: rawURL,
		Adapter: adapter.ID(), Backend: "pending",
		Data:  map[string]any{"matched": adapter.ID()},
		Trace: trace.All(),
	})
}

func RunRead(rawURL string, requestedKind *types.WebxKind) types.WebxEnvelope {
	trace := types.NewTraceBuffer()
	ctx, err := makeMatchContext(rawURL, requestedKind)
	if err != nil {
		trace.Push(types.TraceEvent{Step: "parse", Reason: types.TraceBackendFailed, Detail: err.Error()})
		kind := types.KindMetadata
		if requestedKind != nil {
			kind = *requestedKind
		}
		return types.MakeEnvelope(types.EnvelopeInput{
			OK: false, Kind: kind, URL: rawURL,
			Adapter: "none", Backend: "none",
			Trace: trace.All(),
			Error: &types.EnvelopeError{Code: string(types.ErrFetchFailed), Message: err.Error()},
		})
	}

	adapter := Route(ctx)
	if adapter == nil {
		trace.Push(types.TraceEvent{
			Step: "route", Reason: types.TraceNoMatch,
			Detail: "No adapter matched this URL/kind combination",
		})
		kind := types.KindMetadata
		if requestedKind != nil {
			kind = *requestedKind
		}
		return types.MakeEnvelope(types.EnvelopeInput{
			OK: false, Kind: kind, URL: rawURL,
			Adapter: "none", Backend: "none",
			Trace: trace.All(),
			Error: &types.EnvelopeError{Code: string(types.ErrNoMatch), Message: "No adapter matched URL: " + rawURL},
		})
	}

	trace.Push(types.TraceEvent{
		Step: "route", Reason: types.TraceRouteMatch,
		Adapter: adapter.ID(),
		Detail:  "Matched adapter " + adapter.ID(),
	})

	runCtx := types.RunContext{MatchContext: ctx, Trace: trace}
	result, readErr := adapter.Read(runCtx)
	if readErr != nil {
		var wxErr *types.WebxError
		code := string(types.ErrBackendFailed)
		if errors.As(readErr, &wxErr) {
			code = string(wxErr.Code)
		}
		return types.MakeEnvelope(types.EnvelopeInput{
			OK: false, Kind: defaultKind(requestedKind, adapter), URL: rawURL,
			Adapter: adapter.ID(), Backend: "error",
			Trace: trace.All(),
			Error: &types.EnvelopeError{Code: code, Message: readErr.Error()},
		})
	}

	return types.MakeEnvelope(types.EnvelopeInput{
		OK: true, Kind: defaultKind(requestedKind, adapter), URL: rawURL,
		Adapter: adapter.ID(), Backend: result.Backend,
		Title: result.Title, Markdown: result.Markdown, HTML: result.HTML,
		Trace:         trace.All(),
		FallbackDepth: result.FallbackDepth,
	})
}

func RunExtract(rawURL string, requestedKind *types.WebxKind) types.WebxEnvelope {
	trace := types.NewTraceBuffer()
	ctx, err := makeMatchContext(rawURL, requestedKind)
	if err != nil {
		trace.Push(types.TraceEvent{Step: "parse", Reason: types.TraceBackendFailed, Detail: err.Error()})
		kind := types.KindMetadata
		if requestedKind != nil {
			kind = *requestedKind
		}
		return types.MakeEnvelope(types.EnvelopeInput{
			OK: false, Kind: kind, URL: rawURL,
			Adapter: "none", Backend: "none",
			Trace: trace.All(),
			Error: &types.EnvelopeError{Code: string(types.ErrFetchFailed), Message: err.Error()},
		})
	}

	adapter := Route(ctx)
	if adapter == nil {
		trace.Push(types.TraceEvent{
			Step: "route", Reason: types.TraceNoMatch,
			Detail: "No adapter matched this URL/kind combination",
		})
		kind := types.KindMetadata
		if requestedKind != nil {
			kind = *requestedKind
		}
		return types.MakeEnvelope(types.EnvelopeInput{
			OK: false, Kind: kind, URL: rawURL,
			Adapter: "none", Backend: "none",
			Trace: trace.All(),
			Error: &types.EnvelopeError{Code: string(types.ErrNoMatch), Message: "No adapter matched URL: " + rawURL},
		})
	}

	trace.Push(types.TraceEvent{
		Step: "route", Reason: types.TraceRouteMatch,
		Adapter: adapter.ID(),
		Detail:  "Matched adapter " + adapter.ID(),
	})

	extractable, ok := adapter.(types.ExtractableAdapter)
	if !ok {
		trace.Push(types.TraceEvent{
			Step: "extract", Reason: types.TraceUnsupportedKind,
			Adapter: adapter.ID(),
			Detail:  "Adapter does not implement Extract()",
		})
		return types.MakeEnvelope(types.EnvelopeInput{
			OK: false, Kind: defaultKind(requestedKind, adapter), URL: rawURL,
			Adapter: adapter.ID(), Backend: "none",
			Trace: trace.All(),
			Error: &types.EnvelopeError{Code: string(types.ErrUnsupportedKind), Message: adapter.ID() + " does not support extract()"},
		})
	}

	runCtx := types.RunContext{MatchContext: ctx, Trace: trace}
	result, extractErr := extractable.Extract(runCtx)
	if extractErr != nil {
		var wxErr *types.WebxError
		code := string(types.ErrBackendFailed)
		if errors.As(extractErr, &wxErr) {
			code = string(wxErr.Code)
		}
		return types.MakeEnvelope(types.EnvelopeInput{
			OK: false, Kind: defaultKind(requestedKind, adapter), URL: rawURL,
			Adapter: adapter.ID(), Backend: "error",
			Trace: trace.All(),
			Error: &types.EnvelopeError{Code: code, Message: extractErr.Error()},
		})
	}

	return types.MakeEnvelope(types.EnvelopeInput{
		OK: true, Kind: defaultKind(requestedKind, adapter), URL: rawURL,
		Adapter: adapter.ID(), Backend: result.Backend,
		Title: result.Title, Markdown: result.Markdown, HTML: result.HTML,
		Data:          result.Data,
		Trace:         trace.All(),
		FallbackDepth: result.FallbackDepth,
	})
}

// RunSearch executes a search on the specified platform and returns a WebxEnvelope.
func RunSearch(query string, platformID string, opts types.SearchOptions) types.WebxEnvelope {
	trace := types.NewTraceBuffer()

	adapter := FindAdapter(platformID)
	if adapter == nil {
		trace.Push(types.TraceEvent{
			Step:   "route",
			Reason: types.TraceNoMatch,
			Detail: "No adapter found with ID: " + platformID,
		})
		return types.MakeEnvelope(types.EnvelopeInput{
			OK: false, Kind: types.KindSearch, URL: "",
			Adapter: platformID, Backend: "none",
			Trace: trace.All(),
			Error: &types.EnvelopeError{Code: string(types.ErrNoMatch), Message: "Unknown platform: " + platformID},
		})
	}

	searchable, ok := adapter.(types.SearchableAdapter)
	if !ok {
		trace.Push(types.TraceEvent{
			Step:    "route",
			Reason:  types.TraceNotImplemented,
			Adapter: adapter.ID(),
			Detail:  "Adapter does not implement Search()",
		})
		return types.MakeEnvelope(types.EnvelopeInput{
			OK: false, Kind: types.KindSearch, URL: "",
			Adapter: adapter.ID(), Backend: "none",
			Trace: trace.All(),
			Error: &types.EnvelopeError{Code: string(types.ErrUnsupportedKind), Message: adapter.ID() + " does not support search"},
		})
	}

	trace.Push(types.TraceEvent{
		Step:    "route",
		Reason:  types.TraceRouteMatch,
		Adapter: adapter.ID(),
		Detail:  "Matched searchable adapter " + adapter.ID(),
	})

	ctx := types.SearchContext{
		Query:    query,
		Platform: platformID,
		Options:  opts,
		Trace:    trace,
	}

	result, err := searchable.Search(ctx)
	if err != nil {
		var wxErr *types.WebxError
		code := string(types.ErrBackendFailed)
		if errors.As(err, &wxErr) {
			code = string(wxErr.Code)
		}
		return types.MakeEnvelope(types.EnvelopeInput{
			OK: false, Kind: types.KindSearch, URL: "",
			Adapter: adapter.ID(), Backend: "error",
			Trace: trace.All(),
			Error: &types.EnvelopeError{Code: code, Message: err.Error()},
		})
	}

	// Render search results as markdown.
	markdown := RenderSearchMarkdown(result)

	title := "Search: " + query
	return types.MakeEnvelope(types.EnvelopeInput{
		OK: true, Kind: types.KindSearch, URL: "",
		Adapter:  adapter.ID(),
		Backend:  result.Backend,
		Title:    &title,
		Markdown: &markdown,
		Data:     result,
		Trace:    trace.All(),
	})
}

// RenderSearchMarkdown renders search results as readable markdown.
func RenderSearchMarkdown(result *types.NormalizedSearchResult) string {
	if result == nil || len(result.Items) == 0 {
		return "No results found.\n"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Search Results for %q\n\n", result.Query)
	fmt.Fprintf(&b, "%d results", len(result.Items))
	if result.TotalEstimate > 0 {
		fmt.Fprintf(&b, " (of ~%d total)", result.TotalEstimate)
	}
	b.WriteString("\n\n")

	for i, item := range result.Items {
		fmt.Fprintf(&b, "## %d. %s\n\n", i+1, item.Title)
		if item.URL != "" {
			fmt.Fprintf(&b, "URL: %s\n", item.URL)
		}
		if item.Author != "" {
			fmt.Fprintf(&b, "Author: %s\n", item.Author)
		}
		if item.Date != "" {
			fmt.Fprintf(&b, "Date: %s\n", item.Date)
		}
		if item.Score > 0 {
			fmt.Fprintf(&b, "Score: %.0f\n", item.Score)
		}
		if item.Snippet != "" {
			fmt.Fprintf(&b, "\n%s\n", item.Snippet)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// runWrite is the shared implementation for RunPost, RunReply, and RunReact.
func runWrite(ctx types.WriteContext) types.WebxEnvelope {
	trace := ctx.Trace
	if trace == nil {
		trace = types.NewTraceBuffer()
		ctx.Trace = trace
	}

	var adapter types.Adapter

	if ctx.Action == types.ActionPost {
		// Post routes by platform ID.
		adapter = FindAdapter(ctx.Platform)
		if adapter == nil {
			trace.Push(types.TraceEvent{
				Step: "route", Reason: types.TraceNoMatch,
				Detail: "No adapter found with ID: " + ctx.Platform,
			})
			return types.MakeEnvelope(types.EnvelopeInput{
				OK: false, Kind: types.KindWrite, URL: "",
				Adapter: ctx.Platform, Backend: "none",
				Trace: trace.All(),
				Error: &types.EnvelopeError{Code: string(types.ErrNoMatch), Message: "Unknown platform: " + ctx.Platform},
			})
		}
	} else {
		// Reply/React routes by target URL.
		matchCtx, err := makeMatchContext(ctx.TargetURL, nil)
		if err != nil {
			trace.Push(types.TraceEvent{Step: "parse", Reason: types.TraceBackendFailed, Detail: err.Error()})
			return types.MakeEnvelope(types.EnvelopeInput{
				OK: false, Kind: types.KindWrite, URL: ctx.TargetURL,
				Adapter: "none", Backend: "none",
				Trace: trace.All(),
				Error: &types.EnvelopeError{Code: string(types.ErrFetchFailed), Message: err.Error()},
			})
		}
		adapter = Route(matchCtx)
		if adapter == nil {
			trace.Push(types.TraceEvent{
				Step: "route", Reason: types.TraceNoMatch,
				Detail: "No adapter matched URL: " + ctx.TargetURL,
			})
			return types.MakeEnvelope(types.EnvelopeInput{
				OK: false, Kind: types.KindWrite, URL: ctx.TargetURL,
				Adapter: "none", Backend: "none",
				Trace: trace.All(),
				Error: &types.EnvelopeError{Code: string(types.ErrNoMatch), Message: "No adapter matched URL: " + ctx.TargetURL},
			})
		}
	}

	writable, ok := adapter.(types.WritableAdapter)
	if !ok {
		trace.Push(types.TraceEvent{
			Step:    "route",
			Reason:  types.TraceNotImplemented,
			Adapter: adapter.ID(),
			Detail:  "Adapter does not implement write operations",
		})
		return types.MakeEnvelope(types.EnvelopeInput{
			OK: false, Kind: types.KindWrite, URL: ctx.TargetURL,
			Adapter: adapter.ID(), Backend: "none",
			Trace: trace.All(),
			Error: &types.EnvelopeError{Code: string(types.ErrUnsupportedKind), Message: adapter.ID() + " does not support write operations"},
		})
	}

	trace.Push(types.TraceEvent{
		Step:    "route",
		Reason:  types.TraceRouteMatch,
		Adapter: adapter.ID(),
		Detail:  fmt.Sprintf("Matched writable adapter %s for %s", adapter.ID(), ctx.Action),
	})

	var result *types.NormalizedWriteResult
	var writeErr error

	switch ctx.Action {
	case types.ActionPost:
		result, writeErr = writable.Post(ctx)
	case types.ActionReply:
		result, writeErr = writable.Reply(ctx)
	case types.ActionReact:
		result, writeErr = writable.React(ctx)
	default:
		return types.MakeEnvelope(types.EnvelopeInput{
			OK: false, Kind: types.KindWrite, URL: ctx.TargetURL,
			Adapter: adapter.ID(), Backend: "none",
			Trace: trace.All(),
			Error: &types.EnvelopeError{Code: string(types.ErrBackendFailed), Message: "Unknown write action: " + string(ctx.Action)},
		})
	}

	if writeErr != nil {
		var wxErr *types.WebxError
		code := string(types.ErrBackendFailed)
		if errors.As(writeErr, &wxErr) {
			code = string(wxErr.Code)
		}
		return types.MakeEnvelope(types.EnvelopeInput{
			OK: false, Kind: types.KindWrite, URL: ctx.TargetURL,
			Adapter: adapter.ID(), Backend: "error",
			Trace: trace.All(),
			Error: &types.EnvelopeError{Code: code, Message: writeErr.Error()},
		})
	}

	markdown := fmt.Sprintf("**%s** completed on %s.\n", result.Action, adapter.ID())
	if result.ResourceURL != "" {
		markdown += fmt.Sprintf("\nURL: %s\n", result.ResourceURL)
	}
	if result.Message != "" {
		markdown += fmt.Sprintf("\n%s\n", result.Message)
	}

	return types.MakeEnvelope(types.EnvelopeInput{
		OK: true, Kind: types.KindWrite, URL: ctx.TargetURL,
		Adapter:  adapter.ID(),
		Backend:  result.Backend,
		Markdown: &markdown,
		Data:     result,
		Trace:    trace.All(),
	})
}

// RunPost executes a post operation on the specified platform.
func RunPost(platform string, content string) types.WebxEnvelope {
	return runWrite(types.WriteContext{
		Action:   types.ActionPost,
		Platform: platform,
		Content:  content,
		Trace:    types.NewTraceBuffer(),
	})
}

// RunReply executes a reply operation targeting the specified URL.
func RunReply(targetURL string, content string) types.WebxEnvelope {
	return runWrite(types.WriteContext{
		Action:    types.ActionReply,
		TargetURL: targetURL,
		Content:   content,
		Trace:     types.NewTraceBuffer(),
	})
}

// RunReact executes a reaction on the specified URL.
func RunReact(targetURL string, reaction string) types.WebxEnvelope {
	return runWrite(types.WriteContext{
		Action:    types.ActionReact,
		TargetURL: targetURL,
		Reaction:  reaction,
		Trace:     types.NewTraceBuffer(),
	})
}
