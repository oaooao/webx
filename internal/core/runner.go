package core

import (
	"errors"
	"net/url"

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
