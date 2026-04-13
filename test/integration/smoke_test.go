//go:build integration

package integration_test

import (
	"os"
	"strings"
	"testing"

	_ "github.com/oaooao/webx/internal/adapters"
	"github.com/oaooao/webx/internal/core"
)

type smokeCase struct {
	id              string
	url             string
	expectedAdapter string
	// acceptedBackends: any of these is fine (order doesn't matter)
	acceptedBackends []string
	skipReason       string // non-empty => t.Skip
	note             string
}

var smokeCases = []smokeCase{
	{
		id:               "T-01",
		url:              "https://x.com/browser_use/status/2032731571686629514",
		expectedAdapter:  "twitter",
		acceptedBackends: []string{"twitter_graphql"},
		note:             "requires TWITTER_AUTH_TOKEN + TWITTER_CT0",
	},
	{
		id:               "T-02",
		url:              "https://www.reddit.com/r/ClaudeCode/comments/1sal1yk/",
		expectedAdapter:  "reddit",
		acceptedBackends: []string{"reddit_json"},
	},
	{
		id:               "T-03",
		url:              "https://www.youtube.com/watch?v=Ah9p7v7nJWg",
		expectedAdapter:  "generic-article",
		acceptedBackends: []string{"jina", "defuddle"},
		note:             "YouTube adapter may vary",
	},
	{
		id:               "T-04",
		url:              "https://news.ycombinator.com/item?id=47336171",
		expectedAdapter:  "hacker-news",
		acceptedBackends: []string{"hn_algolia"},
	},
	{
		id:               "T-05",
		url:              "https://arxiv.org/abs/2401.00001",
		expectedAdapter:  "arxiv",
		acceptedBackends: []string{"defuddle", "jina"},
	},
	{
		id:               "T-07",
		url:              "https://simonwillison.net/2025/Apr/9/mcp-prompt-injection/",
		expectedAdapter:  "generic-article",
		acceptedBackends: []string{"defuddle", "jina"},
	},
	{
		id:               "T-08",
		url:              "https://petstore.swagger.io/",
		expectedAdapter:  "generic-article",
		acceptedBackends: []string{"jina", "defuddle"},
	},
	{
		id:               "T-09",
		url:              "https://example.com",
		expectedAdapter:  "generic-article",
		acceptedBackends: []string{"jina", "defuddle"},
	},
}

func TestSmokeAll(t *testing.T) {
	for _, tc := range smokeCases {
		tc := tc // capture range variable
		t.Run(tc.id+" "+adapterLabel(tc.expectedAdapter), func(t *testing.T) {
			t.Parallel()
			runSmokeCase(t, tc)
		})
	}
}

func runSmokeCase(t *testing.T, tc smokeCase) {
	t.Helper()

	// Skip Twitter tests when env vars are missing
	if tc.expectedAdapter == "twitter" {
		if os.Getenv("TWITTER_AUTH_TOKEN") == "" || os.Getenv("TWITTER_CT0") == "" {
			t.Skip("skipping: TWITTER_AUTH_TOKEN and/or TWITTER_CT0 not set")
		}
	}

	if tc.skipReason != "" {
		t.Skip(tc.skipReason)
	}

	env := core.RunRead(tc.url, nil)

	// --- OK ---
	if !env.OK {
		errMsg := "<nil>"
		if env.Error != nil {
			errMsg = env.Error.Code + ": " + env.Error.Message
		}
		t.Fatalf("expected ok=true, got ok=false; error=%s", errMsg)
	}

	// --- Error must be nil ---
	if env.Error != nil {
		t.Fatalf("expected no error, got %s: %s", env.Error.Code, env.Error.Message)
	}

	// --- Adapter ---
	if env.Source.Adapter != tc.expectedAdapter {
		t.Errorf("adapter: want %q, got %q", tc.expectedAdapter, env.Source.Adapter)
	}

	// --- Backend (loose match) ---
	if len(tc.acceptedBackends) > 0 {
		matched := false
		for _, b := range tc.acceptedBackends {
			if env.Source.Backend == b {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("backend: got %q, want one of %v", env.Source.Backend, tc.acceptedBackends)
		}
	}

	// --- Markdown content ---
	md := env.Content.Markdown
	if md == nil {
		t.Fatal("expected non-nil markdown")
	}
	if len(*md) < 100 {
		t.Errorf("markdown too short: got %d bytes, want >= 100", len(*md))
	}
}

// adapterLabel returns a human-friendly label for test names (e.g. "generic-article" -> "GenericArticle").
func adapterLabel(adapter string) string {
	parts := strings.Split(adapter, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}
