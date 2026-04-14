//go:build integration

package integration_test

import (
	"os"
	"strings"
	"testing"

	_ "github.com/oaooao/webx/internal/adapters"
	"github.com/oaooao/webx/internal/auth"
	"github.com/oaooao/webx/internal/core"
	"github.com/oaooao/webx/internal/types"
)

// envOr returns the env var value or the fallback default.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// skipIfNoEnv returns a skip reason when the env var is not set; empty string means "don't skip".
func skipIfNoEnv(key, reason string) string {
	if os.Getenv(key) != "" {
		return ""
	}
	return reason
}

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
		expectedAdapter:  "youtube",
		acceptedBackends: []string{"youtube_native"},
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
		id:               "T-11",
		url:              envOr("WEBX_TEST_CLAUDE_SHARE", "https://claude.ai/share/48088842-673f-4ef9-a867-a8add9e71549"),
		expectedAdapter:  "claude-share",
		acceptedBackends: []string{"claude_snapshot"},
	},
	{
		id:               "T-12",
		url:              envOr("WEBX_TEST_CHATGPT_SHARE", "https://chatgpt.com/share/69ddbda6-a51c-83ea-af56-fa0be87039e6"),
		expectedAdapter:  "chatgpt-share",
		acceptedBackends: []string{"chatgpt_api", "chatgpt_html"},
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
	{
		id:               "T-10",
		url:              "https://youtu.be/dQw4w9WgXcQ",
		expectedAdapter:  "youtube",
		acceptedBackends: []string{"youtube_native"},
		note:             "youtu.be short URL variant",
	},
}

// searchSmokeCase holds parameters for a search smoke test.
type searchSmokeCase struct {
	id         string
	platform   string
	query      string
	skipReason string
	note       string
}

var searchSmokeCases = []searchSmokeCase{
	{
		id:       "S-01",
		platform: "hacker-news",
		query:    "golang",
		note:     "HN search via Algolia API",
	},
	{
		id:       "S-02",
		platform: "reddit",
		query:    "webx",
		note:     "Reddit /search.json",
	},
	{
		id:       "S-03",
		platform: "youtube",
		query:    "go tutorial",
		note:     "YouTube search results page parse",
	},
	{
		id:         "S-04",
		platform:   "twitter",
		query:      "webx",
		skipReason: skipIfNoEnv("TWITTER_AUTH_TOKEN", "skipping: TWITTER_AUTH_TOKEN not set"),
		note:       "Twitter GraphQL SearchTimeline (requires auth)",
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

func TestSearchSmokeAll(t *testing.T) {
	for _, tc := range searchSmokeCases {
		tc := tc
		t.Run(tc.id+" Search/"+tc.platform, func(t *testing.T) {
			t.Parallel()
			runSearchSmokeCase(t, tc)
		})
	}
}

func runSearchSmokeCase(t *testing.T, tc searchSmokeCase) {
	t.Helper()

	if tc.skipReason != "" {
		t.Skip(tc.skipReason)
	}

	env := core.RunSearch(tc.query, tc.platform, types.DefaultSearchOptions())

	if !env.OK {
		errMsg := "<nil>"
		if env.Error != nil {
			errMsg = env.Error.Code + ": " + env.Error.Message
		}
		t.Fatalf("expected ok=true, got ok=false; error=%s", errMsg)
	}

	if env.Error != nil {
		t.Fatalf("expected no error, got %s: %s", env.Error.Code, env.Error.Message)
	}

	if env.Kind != types.KindSearch {
		t.Errorf("kind: want %q, got %q", types.KindSearch, env.Kind)
	}

	if env.Source.Adapter != tc.platform {
		t.Errorf("adapter: want %q, got %q", tc.platform, env.Source.Adapter)
	}

	if env.Data == nil {
		t.Fatal("expected non-nil data")
	}

	if env.Content.Markdown == nil {
		t.Fatal("expected non-nil markdown")
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

// --- Write operation smoke tests ---
//
// Write operations are NOT run against real platforms (to avoid creating spam).
// Instead, we verify:
//   1. When auth is missing → LOGIN_REQUIRED error (auth check works)
//   2. The router correctly routes write requests to the right adapter
//
// For test cases that DO have auth (TWITTER_AUTH_TOKEN set), we skip real posting
// by default. A future "canary" test account could enable W-01/W-02 with confirmation.

type writeSmokeCase struct {
	id         string
	action     string // "post", "reply", "react"
	platform   string // for Post
	targetURL  string // for Reply/React
	content    string
	reaction   string
	skipReason string
	// wantErrCode: if non-empty, expect this error code (used for no-auth checks)
	wantErrCode string
	note        string
}

var writeSmokeCases = []writeSmokeCase{
	{
		id:          "W-01",
		action:      "post",
		platform:    "twitter",
		content:     "webx integration test [ignore]",
		wantErrCode: string(types.ErrLoginRequired),
		skipReason:  skipIfNoEnv("", ""), // always run: verifies auth check
		note:        "Twitter Post — no auth → LOGIN_REQUIRED (safe, no side effects)",
	},
	{
		id:          "W-02",
		action:      "reply",
		targetURL:   "https://x.com/browser_use/status/2032731571686629514",
		content:     "webx integration test [ignore]",
		wantErrCode: string(types.ErrLoginRequired),
		note:        "Twitter Reply — no auth → LOGIN_REQUIRED",
	},
	{
		id:          "W-03",
		action:      "react",
		targetURL:   "https://x.com/browser_use/status/2032731571686629514",
		reaction:    "like",
		wantErrCode: string(types.ErrLoginRequired),
		note:        "Twitter React (like) — no auth → LOGIN_REQUIRED",
	},
	{
		id:          "W-04",
		action:      "post",
		platform:    "reddit",
		content:     "webx integration test [ignore]",
		wantErrCode: string(types.ErrLoginRequired),
		note:        "Reddit Post — no auth → LOGIN_REQUIRED",
	},
	{
		id:          "W-05",
		action:      "reply",
		targetURL:   "https://www.reddit.com/r/ClaudeCode/comments/1sal1yk/",
		content:     "webx integration test [ignore]",
		wantErrCode: string(types.ErrLoginRequired),
		note:        "Reddit Reply — no auth → LOGIN_REQUIRED",
	},
}

func TestWriteSmokeAll(t *testing.T) {
	for _, tc := range writeSmokeCases {
		tc := tc
		t.Run(tc.id+" Write/"+tc.action+"/"+tc.platform+tc.targetURL, func(t *testing.T) {
			t.Parallel()
			runWriteSmokeCase(t, tc)
		})
	}
}

// --- Auth smoke tests ---
//
// A-01: Verifies that auth.DefaultStore().List() returns no error even when the
// store file does not exist (empty state). This is the baseline check for the
// auth subsystem — if this fails, all auth-dependent operations will break.

func TestAuthSmokeAll(t *testing.T) {
	t.Run("A-01 AuthList/EmptyStore", func(t *testing.T) {
		runAuthSmokeA01(t)
	})
}

func runAuthSmokeA01(t *testing.T) {
	t.Helper()

	// Point DefaultStore at a temp dir so we don't touch ~/.config/webx/auth.json.
	dir := t.TempDir()
	t.Setenv("WEBX_AUTH_FILE", dir+"/auth.json")

	store := auth.DefaultStore()
	all, err := store.List()
	if err != nil {
		t.Fatalf("A-01: auth list returned error on empty store: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("A-01: expected empty map, got %d entries", len(all))
	}
}

func runWriteSmokeCase(t *testing.T, tc writeSmokeCase) {
	t.Helper()

	if tc.skipReason != "" {
		t.Skip(tc.skipReason)
	}

	var env types.WebxEnvelope
	switch tc.action {
	case "post":
		env = core.RunPost(tc.platform, tc.content)
	case "reply":
		env = core.RunReply(tc.targetURL, tc.content)
	case "react":
		env = core.RunReact(tc.targetURL, tc.reaction)
	default:
		t.Fatalf("unknown write action: %s", tc.action)
	}

	if tc.wantErrCode != "" {
		// We expect a specific error (e.g. LOGIN_REQUIRED for no-auth checks).
		if env.OK {
			t.Fatalf("expected ok=false with error %s, got ok=true", tc.wantErrCode)
		}
		if env.Error == nil {
			t.Fatalf("expected error with code %s, got nil error", tc.wantErrCode)
		}
		if env.Error.Code != tc.wantErrCode {
			t.Errorf("error code: want %q, got %q (message: %s)", tc.wantErrCode, env.Error.Code, env.Error.Message)
		}
		// Verify kind is always "write" even on error
		if env.Kind != types.KindWrite {
			t.Errorf("kind: want %q, got %q", types.KindWrite, env.Kind)
		}
		return
	}

	// If no wantErrCode, we expect success.
	if !env.OK {
		errMsg := "<nil>"
		if env.Error != nil {
			errMsg = env.Error.Code + ": " + env.Error.Message
		}
		t.Fatalf("expected ok=true, got ok=false; error=%s", errMsg)
	}
	if env.Kind != types.KindWrite {
		t.Errorf("kind: want %q, got %q", types.KindWrite, env.Kind)
	}
	if env.Data == nil {
		t.Fatal("expected non-nil data")
	}
}
