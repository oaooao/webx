package backends

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/oaooao/webx/internal/types"
)

const (
	fetchHTMLTimeout = 30 * time.Second
	maxBodyBytes     = 10 * 1024 * 1024 // 10 MB
)

// cloudflareMarkers are substrings present in Cloudflare challenge pages.
var cloudflareMarkers = []string{
	"Just a moment...",
	"cf-browser-verification",
	"cf_clearance",
	"Checking your browser before accessing",
	"DDoS protection by Cloudflare",
	"Ray ID:",
	"challenge-platform",
}

// FetchHTML downloads the raw HTML of a URL using the shared uTLS client
// (Chrome fingerprint) and returns the body as a string.
//
// Returns:
//   - (html, nil)                     on success
//   - (nil, *WebxError{ErrAntiBot})   when Cloudflare challenge detected
//   - (nil, *WebxError{ErrFetchFailed}) on network/HTTP error
//   - (nil, *WebxError{ErrFetchTimeout}) on timeout
func FetchHTML(rawURL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchHTMLTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", types.NewWebxError(types.ErrFetchFailed, fmt.Sprintf("build request: %s", err))
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := sharedUTLSClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", types.NewWebxError(types.ErrFetchTimeout, fmt.Sprintf("GET %s timed out", rawURL))
		}
		return "", types.NewWebxError(types.ErrFetchFailed, fmt.Sprintf("GET %s: %s", rawURL, err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusServiceUnavailable {
		// Pre-emptively flag likely bot protection before reading body.
		// We still read the body below to check for CF markers.
	}

	// Decompress if server returned gzip (auto-decompression is disabled when
	// using a custom DialTLSContext transport like our utls client).
	var reader io.Reader = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gzr, gzErr := gzip.NewReader(resp.Body)
		if gzErr != nil {
			return "", types.NewWebxError(types.ErrFetchFailed, fmt.Sprintf("gzip decode: %s", gzErr))
		}
		defer gzr.Close()
		reader = gzr
	}

	limited := io.LimitReader(reader, maxBodyBytes)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", types.NewWebxError(types.ErrFetchFailed, fmt.Sprintf("read body: %s", err))
	}

	html := string(body)

	if isCloudflareChallenge(html, resp.StatusCode) {
		return "", types.NewWebxError(types.ErrAntiBot, fmt.Sprintf("Cloudflare challenge detected at %s", rawURL))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", types.NewWebxError(types.ErrFetchFailed, fmt.Sprintf("HTTP %d for %s", resp.StatusCode, rawURL))
	}

	return html, nil
}

// isCloudflareChallenge returns true when the response looks like a CF challenge page.
func isCloudflareChallenge(html string, statusCode int) bool {
	if statusCode != http.StatusForbidden && statusCode != http.StatusServiceUnavailable {
		return false
	}
	for _, marker := range cloudflareMarkers {
		if strings.Contains(html, marker) {
			return true
		}
	}
	return false
}
