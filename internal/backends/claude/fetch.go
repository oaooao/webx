package claude

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/oaooao/webx/internal/backends"
	"github.com/oaooao/webx/internal/types"
)

// shareIDRe extracts the UUID share ID from the URL path.
var shareIDRe = regexp.MustCompile(`/share/([0-9a-f-]+)`)

// ExtractShareID returns the share UUID from a URL path like /share/xxxx-xxxx.
func ExtractShareID(path string) string {
	m := shareIDRe.FindStringSubmatch(path)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// FetchConversation fetches a shared Claude conversation via the public snapshot API.
// Returns the raw JSON map and the backend name used.
func FetchConversation(shareID string) (map[string]any, string, error) {
	const backend = "claude_snapshot"

	// Public API endpoint for chat snapshots (no org context needed).
	apiURL := fmt.Sprintf("https://claude.ai/api/chat_snapshots/%s?rendering_mode=messages&render_all_tools=true", shareID)

	body, err := backends.FetchHTMLStd(apiURL)
	if err != nil {
		return nil, backend, err
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return nil, backend, types.NewWebxError(types.ErrBackendFailed,
			fmt.Sprintf("failed to parse Claude snapshot JSON: %s", err))
	}

	return data, backend, nil
}
