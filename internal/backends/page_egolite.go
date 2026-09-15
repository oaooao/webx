package backends

import (
	"net/url"

	"github.com/oaooao/webx/internal/backends/egolite"
)

// ReadPageViaEgoLite opens rawURL in the user's ego lite browser, waits for
// the rendered text to settle and extracts the main content from the
// rendered DOM with defuddle. It sees what a signed-in person sees, so it
// gets past pages that only render in a browser or need the user's cookies.
func ReadPageViaEgoLite(rawURL string) (*DefuddleResult, error) {
	site := rawURL
	if u, err := url.Parse(rawURL); err == nil && u.Hostname() != "" {
		site = u.Hostname()
	}
	page, err := egolite.RenderPage(egolite.PageJob{Site: site, URL: rawURL, WithHTML: true})
	if err != nil {
		return nil, err
	}
	pageURL := page.URL
	if pageURL == "" {
		pageURL = rawURL
	}
	res, err := ParseHTML(page.HTML, pageURL)
	if err != nil {
		return nil, err
	}
	if res.Title == "" {
		res.Title = page.Title
	}
	return res, nil
}
