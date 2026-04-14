// Package auth provides unified credential management for webx platform adapters.
// Credentials are stored in a JSON file at ~/.config/webx/auth.json with 0600 permissions.
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PlatformAuth holds the credentials for a single platform.
type PlatformAuth struct {
	Type        string            `json:"type"`        // "cookie", "oauth2", "api_key"
	Credentials map[string]string `json:"credentials"` // platform-specific key-value pairs
	AddedAt     string            `json:"added_at"`
}

// authFile is the on-disk JSON format.
type authFile struct {
	Version   int                      `json:"version"`
	Platforms map[string]PlatformAuth  `json:"platforms"`
}

// AuthStore provides CRUD operations for platform credentials.
type AuthStore interface {
	Get(platformID string) (*PlatformAuth, error)
	Set(platformID string, auth PlatformAuth) error
	Delete(platformID string) error
	List() (map[string]PlatformAuth, error)
}

// FileStore is an AuthStore backed by a JSON file.
type FileStore struct {
	path string
	mu   sync.Mutex
}

// NewFileStore creates a FileStore at the given path.
func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

// DefaultStorePath returns the default auth file path: ~/.config/webx/auth.json
func DefaultStorePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "webx", "auth.json")
}

// DefaultStore returns a FileStore at the default path.
func DefaultStore() *FileStore {
	return NewFileStore(DefaultStorePath())
}

// Get retrieves the credentials for a platform. Returns nil if not found.
func (s *FileStore) Get(platformID string) (*PlatformAuth, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	af, err := s.read()
	if err != nil {
		return nil, nil // file doesn't exist yet → no credentials
	}

	auth, ok := af.Platforms[platformID]
	if !ok {
		return nil, nil
	}
	return &auth, nil
}

// Set stores credentials for a platform.
func (s *FileStore) Set(platformID string, auth PlatformAuth) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	af, err := s.read()
	if err != nil {
		// File doesn't exist; create new.
		af = &authFile{
			Version:   1,
			Platforms: make(map[string]PlatformAuth),
		}
	}

	if auth.AddedAt == "" {
		auth.AddedAt = time.Now().UTC().Format(time.RFC3339)
	}
	af.Platforms[platformID] = auth

	return s.write(af)
}

// Delete removes credentials for a platform.
func (s *FileStore) Delete(platformID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	af, err := s.read()
	if err != nil {
		return nil // nothing to delete
	}

	delete(af.Platforms, platformID)
	return s.write(af)
}

// List returns all stored platform credentials.
func (s *FileStore) List() (map[string]PlatformAuth, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	af, err := s.read()
	if err != nil {
		return map[string]PlatformAuth{}, nil
	}

	// Return a copy.
	result := make(map[string]PlatformAuth, len(af.Platforms))
	for k, v := range af.Platforms {
		result[k] = v
	}
	return result, nil
}

// Path returns the file path of this store.
func (s *FileStore) Path() string {
	return s.path
}

// read loads the auth file from disk.
func (s *FileStore) read() (*authFile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}

	var af authFile
	if err := json.Unmarshal(data, &af); err != nil {
		return nil, fmt.Errorf("invalid auth file %s: %w", s.path, err)
	}

	if af.Platforms == nil {
		af.Platforms = make(map[string]PlatformAuth)
	}

	return &af, nil
}

// write persists the auth file to disk with 0600 permissions.
func (s *FileStore) write(af *authFile) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create auth directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(af, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal auth data: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("failed to write auth file %s: %w", s.path, err)
	}

	return nil
}
