// Package backends — Reddit auth loading tests.
//
// Tests for LoadRedditAccessToken priority: AuthStore → env vars → LOGIN_REQUIRED.
//
// Expected LoadRedditAccessToken() behavior (per architect-plan-auth.md D4):
//  1. Query auth.DefaultStore().Get("reddit") (DefaultStore respects WEBX_AUTH_FILE)
//  2. If found and access_token non-empty → return token
//  3. Else query REDDIT_ACCESS_TOKEN env var
//  4. If env var set → return token
//  5. Else → return ErrLoginRequired

package backends

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oaooao/webx/internal/auth"
	"github.com/oaooao/webx/internal/types"
)

// tempRedditAuthStore creates a temp FileStore for Reddit auth tests.
func tempRedditAuthStore(t *testing.T) *auth.FileStore {
	t.Helper()
	dir := t.TempDir()
	return auth.NewFileStore(filepath.Join(dir, "auth.json"))
}

// setRedditEnvVars sets env vars for the duration of the test and restores them on cleanup.
func setRedditEnvVars(t *testing.T, vars map[string]string) {
	t.Helper()
	for k, v := range vars {
		key, old := k, os.Getenv(k)
		os.Setenv(key, v)
		t.Cleanup(func() { os.Setenv(key, old) })
	}
}

// clearRedditEnvVar unsets an env var for the duration of the test.
func clearRedditEnvVar(t *testing.T, key string) {
	t.Helper()
	old := os.Getenv(key)
	os.Unsetenv(key)
	t.Cleanup(func() { os.Setenv(key, old) })
}

// TestLoadRedditAccessToken_FromStore verifies that when the AuthStore has Reddit credentials,
// LoadRedditAccessToken uses them (even if env var is also set).
func TestLoadRedditAccessToken_FromStore_ShouldPreferStoreOverEnv(t *testing.T) {
	store := tempRedditAuthStore(t)
	_ = store.Set("reddit", auth.PlatformAuth{
		Type:        "oauth2",
		Credentials: map[string]string{"access_token": "store-reddit-token"},
	})

	setRedditEnvVars(t, map[string]string{
		"WEBX_AUTH_FILE":      store.Path(),
		"REDDIT_ACCESS_TOKEN": "env-reddit-token",
	})

	token, err := LoadRedditAccessToken()
	if err != nil {
		t.Fatalf("LoadRedditAccessToken: %v", err)
	}
	if token != "store-reddit-token" {
		t.Errorf("token: got %q, want store-reddit-token", token)
	}
}

// TestLoadRedditAccessToken_FallbackToEnv verifies that when the AuthStore has no Reddit entry,
// LoadRedditAccessToken falls back to REDDIT_ACCESS_TOKEN env var.
func TestLoadRedditAccessToken_FallbackToEnv_WhenStoreEmpty(t *testing.T) {
	store := tempRedditAuthStore(t) // empty store

	setRedditEnvVars(t, map[string]string{
		"WEBX_AUTH_FILE":      store.Path(),
		"REDDIT_ACCESS_TOKEN": "env-only-reddit-token",
	})

	token, err := LoadRedditAccessToken()
	if err != nil {
		t.Fatalf("LoadRedditAccessToken: %v", err)
	}
	if token != "env-only-reddit-token" {
		t.Errorf("token: got %q, want env-only-reddit-token", token)
	}
}

// TestLoadRedditAccessToken_BothEmpty verifies that when neither store nor env var have credentials,
// LoadRedditAccessToken returns ErrLoginRequired.
func TestLoadRedditAccessToken_BothEmpty_ShouldReturnLoginRequired(t *testing.T) {
	store := tempRedditAuthStore(t) // empty store

	setRedditEnvVars(t, map[string]string{"WEBX_AUTH_FILE": store.Path()})
	clearRedditEnvVar(t, "REDDIT_ACCESS_TOKEN")

	_, err := LoadRedditAccessToken()
	if err == nil {
		t.Fatal("expected ErrLoginRequired when both store and env are empty")
	}
	wxErr, ok := err.(*types.WebxError)
	if !ok {
		t.Fatalf("expected *types.WebxError, got %T", err)
	}
	if wxErr.Code != types.ErrLoginRequired {
		t.Errorf("error code: got %s, want ErrLoginRequired", wxErr.Code)
	}
}

// TestLoadRedditAccessToken_StorePartialCredentials verifies that store entries without
// access_token fall back to env var.
func TestLoadRedditAccessToken_StorePartialCredentials_ShouldFallbackToEnv(t *testing.T) {
	store := tempRedditAuthStore(t)
	// Set reddit entry but with a different credential key (no access_token).
	_ = store.Set("reddit", auth.PlatformAuth{
		Type:        "oauth2",
		Credentials: map[string]string{"refresh_token": "only-refresh"},
	})

	setRedditEnvVars(t, map[string]string{
		"WEBX_AUTH_FILE":      store.Path(),
		"REDDIT_ACCESS_TOKEN": "env-full-token",
	})

	token, err := LoadRedditAccessToken()
	if err != nil {
		t.Fatalf("LoadRedditAccessToken: %v", err)
	}
	// Partial store credentials (no access_token) should fall back to env.
	if token != "env-full-token" {
		t.Errorf("token: got %q, want env-full-token", token)
	}
}
