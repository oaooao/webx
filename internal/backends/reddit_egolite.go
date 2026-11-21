package backends

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/oaooao/webx/internal/backends/egolite"
	"github.com/oaooao/webx/internal/types"
)

// redditEgoLiteOrigin returns the HTML page to open before fetching the JSON
// at u (already on www.reddit.com): the thread or listing page the JSON
// belongs to. Reddit rejects a .json fetch issued from a non-HTML page such
// as robots.txt with 403 even in a signed-in browser, and answers the same
// fetch from the HTML page with 200 (measured 2026-11-21). Search pages load
// their results by script, so search opens the parent page instead.
func redditEgoLiteOrigin(u *url.URL) string {
	path := strings.TrimSuffix(strings.TrimSuffix(u.Path, "/"), ".json")
	if path == "/search" || strings.HasSuffix(path, "/search") {
		path = strings.TrimSuffix(path, "search")
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return (&url.URL{Scheme: "https", Host: "www.reddit.com", Path: path}).String()
}

// errRedditSearchUnavailable is returned when search.json is refused inside
// ego lite. Reddit answers it with 403 from any page, signed in or not
// (measured 2026-11-21), so signing in again does not help.
func errRedditSearchUnavailable(status int) error {
	return types.NewWebxError(types.ErrBackendUnavailable, fmt.Sprintf(
		"Reddit search is unavailable through ego lite (HTTP %d for search.json even when signed in). "+
			"Read a subreddit listing instead and filter it yourself, e.g. "+
			"webx read https://www.reddit.com/r/<subreddit>/top/?t=week (or /new/).", status))
}

// redditEgoLiteRun is egolite.Run; tests replace it.
var redditEgoLiteRun = egolite.Run

// FetchRedditJSONViaEgoLite fetches a thread's .json (post plus comment tree)
// through the user's ego lite browser. Reddit serves public threads without
// login, so no login cookie is required up front; a 401/403 inside the
// browser still maps to LOGIN_REQUIRED.
func FetchRedditJSONViaEgoLite(redditURL string, commentSort ...string) ([]RedditListing, error) {
	jsonURL, err := redditJSONURL(redditURL, commentSort...)
	if err != nil {
		return nil, err
	}
	body, err := redditEgoLiteGet(jsonURL, nil)
	if err != nil {
		return nil, err
	}
	return decodeRedditListings(body)
}

// SearchRedditPostsViaEgoLite is SearchRedditPosts over ego lite.
func SearchRedditPostsViaEgoLite(apiURL string) (*types.NormalizedSearchResult, error) {
	body, err := redditEgoLiteGet(apiURL, errRedditSearchUnavailable)
	if err != nil {
		return nil, err
	}
	return redditSearchResultFromBody(body, egolite.BackendID)
}

// redditEgoLiteGet fetches rawURL inside ego lite. onDenied, when set, builds
// the error for a 401/403 instead of the default "sign in again" error.
func redditEgoLiteGet(rawURL string, onDenied func(status int) error) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, types.NewWebxError(types.ErrFetchFailed, err.Error())
	}
	if u.Hostname() == "reddit.com" || strings.HasSuffix(u.Hostname(), ".reddit.com") {
		u.Scheme, u.Host = "https", "www.reddit.com"
	}

	res, err := redditEgoLiteRun(egolite.Job{
		Site:     "reddit.com",
		Origin:   redditEgoLiteOrigin(u),
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
	case (resp.Status == 401 || resp.Status == 403) && onDenied != nil:
		return nil, onDenied(resp.Status)
	default:
		return nil, egolite.ErrorForStatus("reddit.com", resp.Status)
	}
}
