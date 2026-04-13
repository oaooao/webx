// Package twitter provides a direct GraphQL backend for Twitter/X.com,
// using a Chrome TLS fingerprint to avoid bot detection.
package twitter

import (
	"os"

	"github.com/oaooao/webx/internal/types"
)

// BearerToken is the Twitter public web client bearer token.
// This is NOT a secret API key — it is embedded in Twitter's own web app
// JavaScript and shared by every browser client. All third-party Twitter
// clients use this exact same token.
const BearerToken = "AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA"

// Auth holds the user-supplied Twitter session cookies.
type Auth struct {
	AuthToken string // auth_token cookie
	CT0       string // ct0 cookie (CSRF token, also used as X-Csrf-Token header)
}

// LoadAuth reads Twitter authentication from environment variables.
// Returns LOGIN_REQUIRED error when variables are absent or empty.
func LoadAuth() (*Auth, error) {
	authToken := os.Getenv("TWITTER_AUTH_TOKEN")
	ct0 := os.Getenv("TWITTER_CT0")

	if authToken == "" || ct0 == "" {
		return nil, types.NewWebxError(
			types.ErrLoginRequired,
			"TWITTER_AUTH_TOKEN and TWITTER_CT0 environment variables are required. "+
				"Obtain them from your browser cookies on x.com after logging in.",
		)
	}

	return &Auth{
		AuthToken: authToken,
		CT0:       ct0,
	}, nil
}

// CookieHeader returns the Cookie header value for authenticated requests.
func (a *Auth) CookieHeader() string {
	return "auth_token=" + a.AuthToken + "; ct0=" + a.CT0
}
