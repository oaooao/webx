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
		article: func(id string) (json.RawMessage, error) { return twitterbe.FetchArticleByTweetID(id, auth) },
	}
}

func egoliteTransport() transport {
	return transport{
		backend:       egolite.BackendID,
		fallbackDepth: 1,
		article:       twitterbe.FetchArticleByTweetIDViaEgoLite,
	}
}

// Seams for tests.
var (
	loadAuth           = twitterbe.LoadAuth
	directTweetDetail  = twitterbe.FetchTweetDetail
	egoliteTweetDetail = twitterbe.FetchTweetDetailViaEgoLite
	directSearch       = func(ctx types.SearchContext) (*types.NormalizedSearchResult, error) {
		return twitterbe.SearchTwitter(ctx, "", "")
	}
	egoliteSearch = twitterbe.SearchTwitterViaEgoLite
)

// fetchTweetDetail runs the transport chain for TweetDetail: direct first,
// egolite when direct has no credentials or is rejected with 401/403.
func fetchTweetDetail(ctx types.RunContext, step, tweetID string) (transport, json.RawMessage, error) {
	auth, authErr := loadAuth()
	if authErr == nil {
		raw, err := directTweetDetail(tweetID, auth)
		if err == nil {
			return directTransport(auth), raw, nil
		}
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  traceReason(err),
			Adapter: "twitter",
			Backend: directBackend,
			Detail:  err.Error(),
		})
		if !isLoginRequired(err) {
			return transport{}, nil, err
		}
	} else {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step,
			Reason:  types.TraceLoginRequired,
			Adapter: "twitter",
			Backend: directBackend,
			Detail:  "no stored Twitter credentials; skipping direct backend",
		})
	}

	tr := egoliteTransport()
	raw, err := egoliteTweetDetail(tweetID)
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    step + ".fallback",
			Reason:  traceReason(err),
			Adapter: "twitter",
			Backend: tr.backend,
			Detail:  err.Error(),
		})
		return transport{}, nil, err
	}
	return tr, raw, nil
}

// searchWithFallback is the same chain for SearchTimeline.
func searchWithFallback(ctx types.SearchContext) (*types.NormalizedSearchResult, error) {
	result, err := directSearch(ctx)
	if err == nil {
		return result, nil
	}
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
	result, err = egoliteSearch(ctx)
	if err != nil {
		ctx.Trace.Push(types.TraceEvent{
			Step:    "adapter.search.fallback",
			Reason:  traceReason(err),
			Adapter: "twitter",
			Backend: egolite.BackendID,
			Detail:  err.Error(),
		})
	}
	return result, err
}

func isLoginRequired(err error) bool {
	var wxErr *types.WebxError
	return errors.As(err, &wxErr) && wxErr.Code == types.ErrLoginRequired
}
