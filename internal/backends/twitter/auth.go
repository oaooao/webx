// Package twitter provides a direct GraphQL backend for Twitter/X.com,
// using a Chrome TLS fingerprint to avoid bot detection.
package twitter

import (
	"os"

	"github.com/oaooao/webx/internal/auth"
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

// LoadAuth reads Twitter authentication.
// Priority: 1) ~/.config/webx/auth.json, 2) TWITTER_AUTH_TOKEN + TWITTER_CT0 env vars.
func LoadAuth() (*Auth, error) {
	// 1. Try auth store.
	store := auth.DefaultStore()
	if pa, err := store.Get("twitter"); err == nil && pa != nil {
		token := pa.Credentials["auth_token"]
		ct0 := pa.Credentials["ct0"]
		if token != "" && ct0 != "" {
			return &Auth{AuthToken: token, CT0: ct0}, nil
		}
	}

	// 2. Fallback to environment variables.
	authToken := os.Getenv("TWITTER_AUTH_TOKEN")
	ct0 := os.Getenv("TWITTER_CT0")

	if authToken != "" && ct0 != "" {
		return &Auth{AuthToken: authToken, CT0: ct0}, nil
	}

	return nil, types.NewWebxError(
		types.ErrLoginRequired,
		"Twitter credentials not found. Run: webx auth add twitter\n"+
			"Or set TWITTER_AUTH_TOKEN and TWITTER_CT0 environment variables.\n"+
			"See https://github.com/oaooao/webx#twitter-setup",
	)
}

// CookieHeader returns the Cookie header value for authenticated requests.
func (a *Auth) CookieHeader() string {
	return "auth_token=" + a.AuthToken + "; ct0=" + a.CT0
}
