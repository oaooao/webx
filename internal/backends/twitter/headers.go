package twitter

import "net/http"

// SetChromeHeaders applies the full set of headers that Chrome sends when
// accessing Twitter's GraphQL API. These headers are required — missing or
// wrong values trigger Twitter's bot-detection and return 403/401.
//
// Note: x-client-transaction-id is intentionally omitted in v0.
// If Twitter starts rejecting requests because of its absence, the trace
// will contain detail about the HTTP status and the error code will be ANTI_BOT.
func SetChromeHeaders(req *http.Request, auth *Auth) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Authorization", "Bearer "+BearerToken)
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Cookie", auth.CookieHeader())
	req.Header.Set("DNT", "1")
	req.Header.Set("Origin", "https://x.com")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", "https://x.com/")
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="136", "Google Chrome";v="136", "Not.A/Brand";v="99"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36")
	req.Header.Set("X-Csrf-Token", auth.CT0)
	req.Header.Set("X-Twitter-Active-User", "yes")
	req.Header.Set("X-Twitter-Auth-Type", "OAuth2Session")
	req.Header.Set("X-Twitter-Client-Language", "en")
}
