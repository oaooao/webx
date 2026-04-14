// Package twitter — auth loading tests.
//
// Tests for LoadAuth priority: AuthStore → env vars → LOGIN_REQUIRED.
//
// TestLoadAuth_FromStore and TestLoadAuth_FallbackToEnv test the
// updated LoadAuth() that queries AuthStore before falling back to env vars.
// These are activated when worker updates LoadAuth() to integrate with AuthStore.
//
// Expected updated LoadAuth() behavior (per architect-plan-auth.md D4):
//  1. Query auth.DefaultStore().Get("twitter")
//  2. If found and credentials non-empty → use store credentials
//  3. Else query TWITTER_AUTH_TOKEN + TWITTER_CT0 env vars
//  4. If env vars set → use env credentials
//  5. Else → return ErrLoginRequired

package twitter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oaooao/webx/internal/auth"
	"github.com/oaooao/webx/internal/types"
)


// withTempStore sets the process's auth store to a temporary FileStore for the
// duration of the test. It relies on LoadAuth() accepting a custom store path
// via an env var override (WEBX_AUTH_FILE) or a package-level var.
// If neither mechanism exists yet, these tests stay in the /* */ block.
//
// The preferred mechanism: LoadAuth uses auth.DefaultStore() which reads
// DefaultStorePath(). Tests override WEBX_AUTH_FILE env var to redirect to
// a temp path. This is safer than monkey-patching.

// tempAuthStore creates a temp FileStore and returns its path.
func tempAuthStore(t *testing.T) *auth.FileStore {
	t.Helper()
	dir := t.TempDir()
	return auth.NewFileStore(filepath.Join(dir, "auth.json"))
}

// setEnvVars sets env vars for the duration of the test and restores them on cleanup.
func setEnvVars(t *testing.T, vars map[string]string) {
	t.Helper()
	for k, v := range vars {
		k, old := k, os.Getenv(k)
		os.Setenv(k, v)
		t.Cleanup(func() { os.Setenv(k, old) })
	}
}

// clearEnvVar unsets an env var for the duration of the test.
func clearEnvVar(t *testing.T, key string) {
	t.Helper()
	old := os.Getenv(key)
	os.Unsetenv(key)
	t.Cleanup(func() { os.Setenv(key, old) })
}

// --- Tests below depend on LoadAuth() integrating with AuthStore ---
// Uncomment when worker updates internal/backends/twitter/auth.go to
// query auth.DefaultStore() / WEBX_AUTH_FILE before env vars.

// TestLoadAuth_FromStore verifies that when the AuthStore has Twitter credentials,
// LoadAuth uses them (even if env vars are also set).
func TestLoadAuth_FromStore_ShouldPreferStoreOverEnv(t *testing.T) {
	store := tempAuthStore(t)
	_ = store.Set("twitter", auth.PlatformAuth{
		Type: "cookie",
		Credentials: map[string]string{
			"auth_token": "store-token",
			"ct0":        "store-ct0",
		},
	})

	// Point LoadAuth at the temp store.
	setEnvVars(t, map[string]string{
		"WEBX_AUTH_FILE":     store.Path(),
		"TWITTER_AUTH_TOKEN": "env-token",
		"TWITTER_CT0":        "env-ct0",
	})

	a, err := LoadAuth()
	if err != nil {
		t.Fatalf("LoadAuth: %v", err)
	}
	if a.AuthToken != "store-token" {
		t.Errorf("AuthToken: got %q, want store-token", a.AuthToken)
	}
	if a.CT0 != "store-ct0" {
		t.Errorf("CT0: got %q, want store-ct0", a.CT0)
	}
}

// TestLoadAuth_FallbackToEnv verifies that when the AuthStore has no Twitter entry,
// LoadAuth falls back to env vars.
func TestLoadAuth_FallbackToEnv_WhenStoreEmpty(t *testing.T) {
	store := tempAuthStore(t) // empty store

	setEnvVars(t, map[string]string{
		"WEBX_AUTH_FILE":     store.Path(),
		"TWITTER_AUTH_TOKEN": "env-only-token",
		"TWITTER_CT0":        "env-only-ct0",
	})

	a, err := LoadAuth()
	if err != nil {
		t.Fatalf("LoadAuth: %v", err)
	}
	if a.AuthToken != "env-only-token" {
		t.Errorf("AuthToken: got %q, want env-only-token", a.AuthToken)
	}
	if a.CT0 != "env-only-ct0" {
		t.Errorf("CT0: got %q, want env-only-ct0", a.CT0)
	}
}

// TestLoadAuth_BothEmpty verifies that when neither store nor env vars have credentials,
// LoadAuth returns ErrLoginRequired.
func TestLoadAuth_BothEmpty_ShouldReturnLoginRequired(t *testing.T) {
	store := tempAuthStore(t) // empty store

	setEnvVars(t, map[string]string{"WEBX_AUTH_FILE": store.Path()})
	clearEnvVar(t, "TWITTER_AUTH_TOKEN")
	clearEnvVar(t, "TWITTER_CT0")

	_, err := LoadAuth()
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

// TestLoadAuth_StorePartialCredentials verifies that store entries with missing fields
// do not panic and fall back to env vars.
func TestLoadAuth_StorePartialCredentials_ShouldFallbackToEnv(t *testing.T) {
	store := tempAuthStore(t)
	// Set only auth_token, missing ct0.
	_ = store.Set("twitter", auth.PlatformAuth{
		Type:        "cookie",
		Credentials: map[string]string{"auth_token": "partial-token"},
	})

	setEnvVars(t, map[string]string{
		"WEBX_AUTH_FILE":     store.Path(),
		"TWITTER_AUTH_TOKEN": "env-full-token",
		"TWITTER_CT0":        "env-full-ct0",
	})

	a, err := LoadAuth()
	if err != nil {
		t.Fatalf("LoadAuth: %v", err)
	}
	// Partial store credentials should not be used — fallback to env.
	if a.AuthToken != "env-full-token" {
		t.Errorf("AuthToken: got %q, want env-full-token", a.AuthToken)
	}
}
