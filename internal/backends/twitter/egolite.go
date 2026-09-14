package twitter

import (
	"encoding/json"
	"fmt"

	"github.com/oaooao/webx/internal/backends/egolite"
	"github.com/oaooao/webx/internal/types"
)

// egoliteOrigin is the same-origin page opened before calling the GraphQL API.
// robots.txt is tiny and needs no rendering, unlike a status page (~5 s).
const egoliteOrigin = "https://x.com/robots.txt"

// egoliteLoginCookies are the cookies a signed-in x.com session always has.
// auth_token is httpOnly; the transport reads it through CDP.
var egoliteLoginCookies = []string{"auth_token", "ct0"}

// egoliteHeaders are the headers x.com's web client sends to /i/api/graphql.
// The browser supplies cookies, user agent and sec-* headers itself; the CSRF
// header is filled from the page's ct0 cookie inside the browser.
func egoliteHeaders() map[string]string {
	return map[string]string{
		"authorization":             "Bearer " + BearerToken,
		"x-csrf-token":              "{{cookie:ct0}}",
		"x-twitter-active-user":     "yes",
		"x-twitter-auth-type":       "OAuth2Session",
		"x-twitter-client-language": "en",
	}
}

// FetchTweetDetailViaEgoLite fetches TweetDetail (focal tweet plus the first
// page of replies) through the user's logged-in ego lite browser.
func FetchTweetDetailViaEgoLite(tweetID string) (json.RawMessage, error) {
	return egoliteGraphQLGet(tweetDetailRequest(tweetID))
}

// FetchArticleByTweetIDViaEgoLite is FetchArticleByTweetID over ego lite.
func FetchArticleByTweetIDViaEgoLite(tweetID string) (json.RawMessage, error) {
	return egoliteGraphQLGet(articleRequest(tweetID))
}

// SearchTwitterViaEgoLite runs SearchTimeline through ego lite.
func SearchTwitterViaEgoLite(ctx types.SearchContext) (*types.NormalizedSearchResult, error) {
	limit, product := searchParams(ctx.Options)
	headers := egoliteHeaders()
	headers["content-type"] = "application/json"
	body, err := egoliteDo(egolite.Request{
		URL:     BuildSearchTimelineURL(ctx.Query, limit, product),
		Method:  "POST",
		Headers: headers,
		Body:    buildSearchTimelineBody(ctx.Query, limit, product),
	}, "SearchTimeline")
	if err != nil {
		return nil, err
	}
	return searchResultFromBody(body, ctx.Query, limit, egolite.BackendID)
}

func egoliteGraphQLGet(req graphQLRequest) (json.RawMessage, error) {
	apiURL, err := buildGraphQLURL(req)
	if err != nil {
		return nil, err
	}
	return egoliteDo(egolite.Request{URL: apiURL, Headers: egoliteHeaders()}, req.Operation)
}

func egoliteDo(req egolite.Request, operation string) (json.RawMessage, error) {
	res, err := egolite.Run(egolite.Job{
		Site:         "x.com",
		Origin:       egoliteOrigin,
		LoginCookies: egoliteLoginCookies,
		Requests:     []egolite.Request{req},
	})
	if err != nil {
		return nil, err
	}
	if len(res.Responses) != 1 {
		return nil, types.NewWebxError(types.ErrBackendFailed,
			fmt.Sprintf("egolite returned %d responses for 1 request", len(res.Responses)))
	}
	return graphQLBodyFromEgoLite(res.Responses[0], operation)
}

// graphQLBodyFromEgoLite maps a browser-side response to the raw body or a
// WebxError, mirroring the direct client's status handling.
func graphQLBodyFromEgoLite(resp egolite.Response, operation string) (json.RawMessage, error) {
	switch {
	case resp.Error != "":
		return nil, types.NewWebxError(types.ErrFetchFailed, "x.com "+operation+" fetch failed inside ego lite: "+resp.Error)
	case resp.Status == 200:
		return json.RawMessage(resp.Body), nil
	case resp.Status == 404 || resp.Status == 422:
		return nil, types.NewWebxError(types.ErrAntiBot, fmt.Sprintf(
			"Twitter %s returned HTTP %d via ego lite — queryId may be stale. Update from https://github.com/fa0311/twitter-openapi",
			operation, resp.Status))
	default:
		return nil, egolite.ErrorForStatus("x.com", resp.Status)
	}
}
