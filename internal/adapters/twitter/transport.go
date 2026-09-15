package twitter

import (
	"encoding/json"
	"errors"

	"github.com/oaooao/webx/internal/backends/egolite"
	twitterbe "github.com/oaooao/webx/internal/backends/twitter"
	"github.com/oaooao/webx/internal/types"
)

// transport is one way of reaching x.com's GraphQL API.
type transport struct {
	backend       string
	fallbackDepth int
	article       func(tweetID string) (json.RawMessage, error)
}

func directTransport(auth *twitterbe.Auth) transport {
	return transport{
		backend: directBackend,
		article: func(id string) (json.RawMessage, error) { return directArticle(id, auth) },
	}
}

func egoliteTransport() transport {
	return transport{
		backend:       egolite.BackendID,
		fallbackDepth: 1,
		article:       func(id string) (json.RawMessage, error) { return egoliteArticle(id) },
	}
}

// Seams for tests.
var (
	loadAuth           = twitterbe.LoadAuth
	directTweetDetail  = twitterbe.FetchTweetDetail
	egoliteTweetDetail = twitterbe.FetchTweetDetailViaEgoLite
	directArticle      = twitterbe.FetchArticleByTweetID
	egoliteArticle     = twitterbe.FetchArticleByTweetIDViaEgoLite
	directSearch       = func(ctx types.SearchContext) (*types.NormalizedSearchResult, error) {
		return twitterbe.SearchTwitter(ctx, "", "")
	}
	egoliteSearch = twitterbe.SearchTwitterViaEgoLite
	renderPage    = egolite.RenderPage
)

// fetchWithFallback runs one read operation over the transport chain: direct
// first, egolite when direct has no credentials, is rejected with 401/403, or
// answers but yields nothing (stale TWITTER_* credentials can get HTTP 200
// with an empty timeline). parse turns a raw body into T; empty reports
// whether T is unusable. what names the result for messages ("tweets").
func fetchWithFallback[T any](
	trace *types.TraceBuffer, step, what string,
	direct func(*twitterbe.Auth) (json.RawMessage, error),
	viaEgoLite func() (json.RawMessage, error),
	parse func(json.RawMessage) (T, error),
	empty func(T) bool,
) (transport, T, error) {
	var zero T
	directEmpty := false

	auth, authErr := loadAuth()
	if authErr == nil {
		raw, err := direct(auth)
		if err == nil {
			v, perr := parse(raw)
			if perr != nil {
				return transport{}, zero, perr
			}
			if !empty(v) {
				return directTransport(auth), v, nil
			}
			directEmpty = true
			trace.Push(types.TraceEvent{
				Step:    step,
				Reason:  types.TraceEmptyContent,
				Adapter: "twitter",
				Backend: directBackend,
				Detail:  "direct backend answered with no " + what + " (stored credentials may be stale); trying egolite",
			})
		} else {
			trace.Push(types.TraceEvent{
				Step:    step,
				Reason:  traceReason(err),
				Adapter: "twitter",
				Backend: directBackend,
				Detail:  err.Error(),
			})
			if !isLoginRequired(err) {
				return transport{}, zero, err
			}
		}
	} else {
		trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceLoginRequired,
			Adapter: "twitter",
			Backend: directBackend,
			Detail:  "no stored Twitter credentials; skipping direct backend",
		})
	}

	tr := egoliteTransport()
	raw, err := viaEgoLite()
	if err != nil {
		trace.Push(types.TraceEvent{
			Step:    step + ".fallback",
			Reason:  traceReason(err),
			Adapter: "twitter",
			Backend: tr.backend,
			Detail:  err.Error(),
		})
		if directEmpty {
			err = afterEmptyDirect(err, what)
		}
		return transport{}, zero, err
	}
	v, err := parse(raw)
	if err != nil {
		return transport{}, zero, err
	}
	return tr, v, nil
}

// afterEmptyDirect explains a failed fallback that followed an empty direct
// answer, so the caller does not mistake it for a plain ego lite problem.
func afterEmptyDirect(err error, what string) error {
	var wxErr *types.WebxError
	if !errors.As(err, &wxErr) {
		return err
	}
	return types.NewWebxError(wxErr.Code,
		"direct Twitter backend returned no "+what+" (TWITTER_AUTH_TOKEN / TWITTER_CT0 may be stale) and the egolite fallback failed: "+wxErr.Message)
}

// searchWithFallback is the same chain for SearchTimeline. Direct search
// loads credentials itself and reports LOGIN_REQUIRED without them.
func searchWithFallback(ctx types.SearchContext) (*types.NormalizedSearchResult, error) {
	result, err := directSearch(ctx)
	directEmpty := false
	switch {
	case err == nil && result != nil && len(result.Items) > 0:
		return result, nil
	case err == nil:
		directEmpty = true
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.search",
			Reason:  types.TraceEmptyContent,
			Adapter: "twitter",
			Backend: directBackend,
			Detail:  "direct search returned 0 results (stored credentials may be stale); trying egolite",
		})
	default:
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.search",
			Reason:  traceReason(err),
			Adapter: "twitter",
			Backend: directBackend,
			Detail:  err.Error(),
		})
		if !isLoginRequired(err) {
			return nil, err
		}
	}

	result, err = egoliteSearch(ctx)
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.search.fallback",
			Reason:  traceReason(err),
			Adapter: "twitter",
			Backend: egolite.BackendID,
			Detail:  err.Error(),
		})
		if directEmpty {
			err = afterEmptyDirect(err, "search results")
		}
	}
	return result, err
}

func isLoginRequired(err error) bool {
	var wxErr *types.WebxError
	return errors.As(err, &wxErr) && wxErr.Code == types.ErrLoginRequired
}
