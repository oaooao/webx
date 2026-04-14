package chatgpt

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/oaooao/webx/internal/backends"
	"github.com/oaooao/webx/internal/types"
)

// shareIDRe extracts the UUID share ID from the URL path.
var shareIDRe = regexp.MustCompile(`/share/([0-9a-f-]+)`)

// nextDataRe extracts the __NEXT_DATA__ JSON blob embedded in the HTML page.
var nextDataRe = regexp.MustCompile(`<script\s+id="__NEXT_DATA__"\s+type="application/json"[^>]*>(.*?)</script>`)

// serverPropsRe is a broader fallback: looks for the conversation JSON in any script tag.
var serverPropsRe = regexp.MustCompile(`"serverResponse"\s*:\s*(\{.*?"mapping"\s*:\s*\{.*?\}.*?\})`)

// ExtractShareID returns the share ID from a URL path like /share/xxxx-xxxx.
func ExtractShareID(path string) string {
	m := shareIDRe.FindStringSubmatch(path)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// FetchConversation attempts to fetch and parse a shared ChatGPT conversation.
// It tries the backend API first, then falls back to HTML scraping.
// Returns the raw JSON map and the backend name used.
func FetchConversation(shareURL string, shareID string) (map[string]any, string, error) {
	// Strategy 1: Backend API endpoint (JSON directly).
	apiURL := fmt.Sprintf("https://chatgpt.com/backend-api/share/conversation/%s", shareID)
	apiBody, apiErr := backends.FetchHTMLStd(apiURL)
	if apiErr == nil {
		var data map[string]any
		if err := json.Unmarshal([]byte(apiBody), &data); err == nil {
			if _, hasMapping := data["mapping"]; hasMapping {
				return data, "chatgpt_api", nil
			}
		}
	}

	// Strategy 2: Fetch the HTML share page and extract embedded JSON.
	html, err := backends.FetchHTMLStd(shareURL)
	if err != nil {
		return nil, "chatgpt_html", err
	}

	data, parseErr := extractEmbeddedJSON(html)
	if parseErr != nil {
		return nil, "chatgpt_html", parseErr
	}

	return data, "chatgpt_html", nil
}

// extractEmbeddedJSON tries to extract the conversation JSON from the HTML page.
func extractEmbeddedJSON(html string) (map[string]any, error) {
	// Try __NEXT_DATA__ first.
	if m := nextDataRe.FindStringSubmatch(html); len(m) >= 2 {
		data, err := drillNextData(m[1])
		if err == nil {
			return data, nil
		}
	}

	// Try serverResponse pattern.
	if m := serverPropsRe.FindStringSubmatch(html); len(m) >= 2 {
		var data map[string]any
		if err := json.Unmarshal([]byte(m[1]), &data); err == nil {
			if _, hasMapping := data["mapping"]; hasMapping {
				return data, nil
			}
		}
	}

	// Last resort: scan for any JSON object containing "mapping" key.
	idx := strings.Index(html, `"mapping"`)
	if idx < 0 {
		return nil, types.NewWebxError(types.ErrContentEmpty, "no conversation data found in HTML")
	}

	// Walk backward to find the opening brace.
	start := strings.LastIndex(html[:idx], "{")
	if start < 0 {
		return nil, types.NewWebxError(types.ErrContentEmpty, "cannot locate conversation JSON boundary")
	}

	// Try progressively larger slices to find valid JSON.
	for end := idx + 200; end <= len(html) && end <= idx+5*1024*1024; end += 1000 {
		bracket := findClosingBrace(html[start:end])
		if bracket > 0 {
			var data map[string]any
			if err := json.Unmarshal([]byte(html[start:start+bracket+1]), &data); err == nil {
				if _, hasMapping := data["mapping"]; hasMapping {
					return data, nil
				}
			}
		}
	}

	return nil, types.NewWebxError(types.ErrContentEmpty, "failed to parse embedded conversation JSON")
}

// drillNextData parses the __NEXT_DATA__ blob and drills into
// props.pageProps.serverResponse.data to find the conversation data.
func drillNextData(raw string) (map[string]any, error) {
	var root map[string]any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return nil, err
	}

	// Navigate: props -> pageProps -> serverResponse -> data
	paths := [][]string{
		{"props", "pageProps", "serverResponse", "data"},
		{"props", "pageProps", "serverResponse"},
		{"props", "pageProps"},
	}

	for _, path := range paths {
		cur := root
		ok := true
		for _, key := range path {
			next, exists := cur[key]
			if !exists {
				ok = false
				break
			}
			m, isMap := next.(map[string]any)
			if !isMap {
				ok = false
				break
			}
			cur = m
		}
		if ok {
			if _, hasMapping := cur["mapping"]; hasMapping {
				return cur, nil
			}
		}
	}

	// Maybe the root itself has mapping (unlikely but defensive).
	if _, hasMapping := root["mapping"]; hasMapping {
		return root, nil
	}

	return nil, fmt.Errorf("__NEXT_DATA__ does not contain conversation mapping")
}

// findClosingBrace returns the index of the matching closing '}' for an opening '{' at position 0,
// handling nesting and string escapes. Returns -1 if not found.
func findClosingBrace(s string) int {
	depth := 0
	inString := false
	escaped := false

	for i, ch := range s {
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
