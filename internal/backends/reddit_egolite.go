package backends

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/oaooao/webx/internal/backends/egolite"
	"github.com/oaooao/webx/internal/types"
)

// redditEgoLiteOrigin is the same-origin page opened before fetching Reddit
// JSON. Every request is rewritten to www.reddit.com so CORS stays same-origin
// for old./new./bare reddit.com URLs.
const redditEgoLiteOrigin = "https://www.reddit.com/robots.txt"

// FetchRedditJSONViaEgoLite fetches a thread's .json (post plus comment tree)
// through the user's ego lite browser. Reddit serves public threads without
// login, so no login cookie is required up front; a 401/403 inside the
// browser still maps to LOGIN_REQUIRED.
func FetchRedditJSONViaEgoLite(redditURL string, commentSort ...string) ([]RedditListing, error) {
	jsonURL, err := redditJSONURL(redditURL, commentSort...)
	if err != nil {
		return nil, err
	}
	body, err := redditEgoLiteGet(jsonURL)
	if err != nil {
		return nil, err
	}
	return decodeRedditListings(body)
}

// SearchRedditPostsViaEgoLite is SearchRedditPosts over ego lite.
func SearchRedditPostsViaEgoLite(apiURL string) (*types.NormalizedSearchResult, error) {
	body, err := redditEgoLiteGet(apiURL)
	if err != nil {
		return nil, err
	}
	return redditSearchResultFromBody(body, egolite.BackendID)
}

func redditEgoLiteGet(rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, err.Error())
	}
	if u.Hostname() == "reddit.com" || strings.HasSuffix(u.Hostname(), ".reddit.com") {
		u.Scheme, u.Host = "https", "www.reddit.com"
	}

	res, err := egolite.Run(egolite.Job{
		Site:     "reddit.com",
		Origin:   redditEgoLiteOrigin,
		Requests: []egolite.Request{{URL: u.String(), Headers: map[string]string{"accept": "application/json"}}},
	})
	if err != nil {
		return nil, err
	}
	if len(res.Responses) != 1 {
		return nil, types.NewWebxError(types.ErrBackendFailed,
			fmt.Sprintf("egolite returned %d responses for 1 request", len(res.Responses)))
	}
	resp := res.Responses[0]
	switch {
	case resp.Error != "":
		return nil, types.NewWebxError(types.ErrFetchFailed, "Reddit fetch failed inside ego lite: "+resp.Error)
	case resp.Status == 200:
		return []byte(resp.Body), nil
	case resp.Status == 404:
		return nil, types.NewWebxError(types.ErrContentEmpty, "Reddit returned 404 via ego lite: the post does not exist or was removed ("+u.String()+")")
	default:
		return nil, egolite.ErrorForStatus("reddit.com", resp.Status)
	}
}
