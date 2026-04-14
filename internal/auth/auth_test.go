package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func tempStore(t *testing.T) *FileStore {
	t.Helper()
	dir := t.TempDir()
	return NewFileStore(filepath.Join(dir, "auth.json"))
}

func TestAuthStore_Set_ShouldPersistToFile(t *testing.T) {
	store := tempStore(t)

	err := store.Set("twitter", PlatformAuth{
		Type: "cookie",
		Credentials: map[string]string{
			"auth_token": "abc",
			"ct0":        "xyz",
		},
	})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Verify file exists.
	if _, err := os.Stat(store.Path()); err != nil {
		t.Fatalf("auth file not created: %v", err)
	}
}

func TestAuthStore_Get_WhenExists_ShouldReturnAuth(t *testing.T) {
	store := tempStore(t)

	_ = store.Set("twitter", PlatformAuth{
		Type:        "cookie",
		Credentials: map[string]string{"auth_token": "abc", "ct0": "xyz"},
	})

	auth, err := store.Get("twitter")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if auth == nil {
		t.Fatal("expected non-nil auth")
	}
	if auth.Type != "cookie" {
		t.Errorf("Type: got %q, want %q", auth.Type, "cookie")
	}
	if auth.Credentials["auth_token"] != "abc" {
		t.Errorf("auth_token: got %q, want %q", auth.Credentials["auth_token"], "abc")
	}
	if auth.AddedAt == "" {
		t.Error("AddedAt should be auto-populated")
	}
}

func TestAuthStore_Get_WhenNotExists_ShouldReturnNil(t *testing.T) {
	store := tempStore(t)

	auth, err := store.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if auth != nil {
		t.Errorf("expected nil for non-existent platform, got %v", auth)
	}
}

func TestAuthStore_Get_WhenFileNotExists_ShouldReturnNil(t *testing.T) {
	store := NewFileStore("/nonexistent/path/auth.json")

	auth, err := store.Get("twitter")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if auth != nil {
		t.Errorf("expected nil when file doesn't exist, got %v", auth)
	}
}

func TestAuthStore_Delete_ShouldRemovePlatform(t *testing.T) {
	store := tempStore(t)

	_ = store.Set("twitter", PlatformAuth{
		Type:        "cookie",
		Credentials: map[string]string{"auth_token": "abc"},
	})
	_ = store.Set("reddit", PlatformAuth{
		Type:        "oauth2",
		Credentials: map[string]string{"access_token": "xyz"},
	})

	err := store.Delete("twitter")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Twitter should be gone.
	auth, _ := store.Get("twitter")
	if auth != nil {
		t.Error("twitter should have been deleted")
	}

	// Reddit should remain.
	auth, _ = store.Get("reddit")
	if auth == nil {
		t.Error("reddit should still exist")
	}
}

func TestAuthStore_List_ShouldReturnAll(t *testing.T) {
	store := tempStore(t)

	_ = store.Set("twitter", PlatformAuth{
		Type:        "cookie",
		Credentials: map[string]string{"auth_token": "abc"},
	})
	_ = store.Set("reddit", PlatformAuth{
		Type:        "oauth2",
		Credentials: map[string]string{"access_token": "xyz"},
	})

	all, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 platforms, got %d", len(all))
	}
	if _, ok := all["twitter"]; !ok {
		t.Error("missing twitter in list")
	}
	if _, ok := all["reddit"]; !ok {
		t.Error("missing reddit in list")
	}
}

func TestAuthStore_List_WhenEmpty_ShouldReturnEmptyMap(t *testing.T) {
	store := tempStore(t)

	all, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 platforms, got %d", len(all))
	}
}

func TestAuthStore_FilePermissions_ShouldBe0600(t *testing.T) {
	store := tempStore(t)

	_ = store.Set("twitter", PlatformAuth{
		Type:        "cookie",
		Credentials: map[string]string{"auth_token": "secret"},
	})

	info, err := os.Stat(store.Path())
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions: got %o, want 0600", perm)
	}
}

func TestAuthStore_Set_ShouldOverwriteExisting(t *testing.T) {
	store := tempStore(t)

	_ = store.Set("twitter", PlatformAuth{
		Type:        "cookie",
		Credentials: map[string]string{"auth_token": "old"},
	})
	_ = store.Set("twitter", PlatformAuth{
		Type:        "cookie",
		Credentials: map[string]string{"auth_token": "new"},
	})

	auth, _ := store.Get("twitter")
	if auth.Credentials["auth_token"] != "new" {
		t.Errorf("expected updated auth_token, got %q", auth.Credentials["auth_token"])
	}
}

func TestAuthStore_List_ShouldReturnCopy(t *testing.T) {
	store := tempStore(t)

	_ = store.Set("twitter", PlatformAuth{
		Type:        "cookie",
		Credentials: map[string]string{"auth_token": "abc"},
	})

	all, _ := store.List()
	delete(all, "twitter")

	// Original should not be affected.
	all2, _ := store.List()
	if _, ok := all2["twitter"]; !ok {
		t.Error("List should return a copy, not a reference to internal state")
	}
}
