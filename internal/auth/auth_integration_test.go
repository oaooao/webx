package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// --- End-to-end lifecycle test ---

func TestAuthStore_E2E_SetGetDeleteList(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(filepath.Join(dir, "auth.json"))

	// Step 1: Initially empty.
	all, err := store.List()
	if err != nil {
		t.Fatalf("List (initial): %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected empty store, got %d entries", len(all))
	}

	// Step 2: Set two platforms.
	err = store.Set("twitter", PlatformAuth{
		Type: "cookie",
		Credentials: map[string]string{
			"auth_token": "tw-token-123",
			"ct0":        "tw-ct0-456",
		},
	})
	if err != nil {
		t.Fatalf("Set twitter: %v", err)
	}

	err = store.Set("reddit", PlatformAuth{
		Type: "oauth2",
		Credentials: map[string]string{
			"access_token": "reddit-token-789",
		},
	})
	if err != nil {
		t.Fatalf("Set reddit: %v", err)
	}

	// Step 3: Get each and verify.
	tw, err := store.Get("twitter")
	if err != nil {
		t.Fatalf("Get twitter: %v", err)
	}
	if tw == nil {
		t.Fatal("expected non-nil twitter auth")
	}
	if tw.Credentials["auth_token"] != "tw-token-123" {
		t.Errorf("twitter auth_token: got %q, want %q", tw.Credentials["auth_token"], "tw-token-123")
	}
	if tw.Credentials["ct0"] != "tw-ct0-456" {
		t.Errorf("twitter ct0: got %q, want %q", tw.Credentials["ct0"], "tw-ct0-456")
	}
	if tw.AddedAt == "" {
		t.Error("twitter AddedAt should be populated")
	}

	rd, err := store.Get("reddit")
	if err != nil {
		t.Fatalf("Get reddit: %v", err)
	}
	if rd == nil {
		t.Fatal("expected non-nil reddit auth")
	}
	if rd.Credentials["access_token"] != "reddit-token-789" {
		t.Errorf("reddit access_token: got %q", rd.Credentials["access_token"])
	}

	// Step 4: List shows both.
	all, err = store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 platforms, got %d", len(all))
	}

	// Step 5: Delete twitter.
	if err := store.Delete("twitter"); err != nil {
		t.Fatalf("Delete twitter: %v", err)
	}

	// Step 6: twitter is gone, reddit remains.
	tw, err = store.Get("twitter")
	if err != nil {
		t.Fatalf("Get twitter after delete: %v", err)
	}
	if tw != nil {
		t.Error("twitter should have been deleted")
	}

	all, err = store.List()
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 platform after delete, got %d", len(all))
	}
	if _, ok := all["reddit"]; !ok {
		t.Error("reddit should remain after twitter delete")
	}

	// Step 7: Delete non-existent platform — should not error.
	if err := store.Delete("nonexistent"); err != nil {
		t.Errorf("Delete non-existent should not error, got: %v", err)
	}
}

// --- File permission test ---

func TestAuthStore_FilePermissions_AreEnforced0600(t *testing.T) {
	store := tempStore(t)

	// Set triggers a file write.
	if err := store.Set("twitter", PlatformAuth{
		Type:        "cookie",
		Credentials: map[string]string{"auth_token": "secret"},
	}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	info, err := os.Stat(store.Path())
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions: got 0%o, want 0600", perm)
	}

	// Overwrite with a second Set — permissions must remain 0600.
	if err := store.Set("reddit", PlatformAuth{
		Type:        "oauth2",
		Credentials: map[string]string{"access_token": "secret2"},
	}); err != nil {
		t.Fatalf("second Set: %v", err)
	}

	info2, err := os.Stat(store.Path())
	if err != nil {
		t.Fatalf("stat after second write: %v", err)
	}
	if info2.Mode().Perm() != 0600 {
		t.Errorf("permissions after second write: got 0%o, want 0600", info2.Mode().Perm())
	}
}

// --- Concurrent read/write safety ---

func TestAuthStore_ConcurrentSetGet_ShouldNotRace(t *testing.T) {
	store := tempStore(t)

	const goroutines = 10
	var wg sync.WaitGroup
	errs := make(chan error, goroutines*2)

	// Concurrent writers.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			platform := fmt.Sprintf("platform-%d", i)
			err := store.Set(platform, PlatformAuth{
				Type:        "api_key",
				Credentials: map[string]string{"key": fmt.Sprintf("token-%d", i)},
			})
			if err != nil {
				errs <- fmt.Errorf("Set %s: %w", platform, err)
			}
		}()
	}

	// Concurrent readers (may see partial state — should not panic or corrupt).
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.List()
			if err != nil {
				errs <- fmt.Errorf("List: %w", err)
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent operation error: %v", err)
	}
}

func TestAuthStore_ConcurrentDeletes_ShouldNotCorrupt(t *testing.T) {
	store := tempStore(t)

	// Pre-populate.
	for i := 0; i < 5; i++ {
		_ = store.Set(fmt.Sprintf("p%d", i), PlatformAuth{
			Type:        "cookie",
			Credentials: map[string]string{"key": "val"},
		})
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			_ = store.Delete(fmt.Sprintf("p%d", i))
		}()
	}
	wg.Wait()

	// After concurrent deletes, List should work without error.
	all, err := store.List()
	if err != nil {
		t.Errorf("List after concurrent deletes: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected empty store after all deletes, got %d", len(all))
	}
}

// --- Update (overwrite) preserves other platforms ---

func TestAuthStore_Set_ShouldPreserveOtherPlatforms(t *testing.T) {
	store := tempStore(t)

	_ = store.Set("twitter", PlatformAuth{
		Type:        "cookie",
		Credentials: map[string]string{"auth_token": "t1"},
	})
	_ = store.Set("reddit", PlatformAuth{
		Type:        "oauth2",
		Credentials: map[string]string{"access_token": "r1"},
	})

	// Overwrite twitter only.
	_ = store.Set("twitter", PlatformAuth{
		Type:        "cookie",
		Credentials: map[string]string{"auth_token": "t2"},
	})

	tw, _ := store.Get("twitter")
	if tw.Credentials["auth_token"] != "t2" {
		t.Errorf("twitter token should be updated to t2, got %q", tw.Credentials["auth_token"])
	}

	rd, _ := store.Get("reddit")
	if rd == nil || rd.Credentials["access_token"] != "r1" {
		t.Error("reddit should remain unaffected by twitter overwrite")
	}
}
