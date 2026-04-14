package chatgpt

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/oaooao/webx/internal/types"
)

// turboStreamRe matches the React Router streamController.enqueue("...") call
// that contains the serialized conversation data.
var turboStreamRe = regexp.MustCompile(`streamController\.enqueue\("`)

// extractTurboStream parses ChatGPT's React Router Turbo Stream format and
// returns a standard map[string]any with keys like "title", "mapping",
// "current_node", etc. — the same structure ParseConversation expects.
//
// The format is a flat JSON array used as a heap: objects use "_NNN" keys
// where NNN is the array index holding the real key name, and the value is
// an index reference into the same array (or a literal for non-int types).
func extractTurboStream(html string) (map[string]any, error) {
	loc := turboStreamRe.FindStringIndex(html)
	if loc == nil {
		return nil, types.NewWebxError(types.ErrContentEmpty, "no React Router stream data found")
	}

	// Extract the JS string payload: everything between the opening " and closing ")
	start := loc[1] // right after the opening quote
	payload, err := extractJSString(html, start)
	if err != nil {
		return nil, err
	}

	// Parse the unescaped payload as a JSON array.
	var arr []any
	if err := json.Unmarshal([]byte(payload), &arr); err != nil {
		return nil, types.NewWebxError(types.ErrBackendFailed,
			fmt.Sprintf("failed to parse turbo stream array: %s", err))
	}

	if len(arr) < 40 {
		return nil, types.NewWebxError(types.ErrContentEmpty, "turbo stream array too short")
	}

	// Find the conversation data by looking for key strings in the array.
	// The structure is pairs: [... "key_name", value, "key_name", value, ...]
	// We need to find "mapping" and reconstruct the conversation object.
	return buildConversationFromArray(arr)
}

// extractJSString extracts and unescapes a JavaScript string starting at pos
// (which points to the first char after the opening quote).
func extractJSString(html string, pos int) (string, error) {
	var sb strings.Builder
	sb.Grow(len(html) / 4) // rough estimate

	i := pos
	for i < len(html) {
		ch := html[i]
		if ch == '"' {
			// End of string — verify it's followed by ')'
			if i+1 < len(html) && html[i+1] == ')' {
				return sb.String(), nil
			}
			// Unescaped quote not followed by ) — shouldn't happen, but be safe
			sb.WriteByte(ch)
			i++
			continue
		}
		if ch == '\\' && i+1 < len(html) {
			next := html[i+1]
			switch next {
			case '"':
				sb.WriteByte('"')
				i += 2
			case '\\':
				sb.WriteByte('\\')
				i += 2
			case 'n':
				sb.WriteByte('\n')
				i += 2
			case 't':
				sb.WriteByte('\t')
				i += 2
			case 'r':
				sb.WriteByte('\r')
				i += 2
			case '/':
				sb.WriteByte('/')
				i += 2
			case 'u':
				// Unicode escape: \uXXXX
				if i+5 < len(html) {
					hex := html[i+2 : i+6]
					code, err := strconv.ParseUint(hex, 16, 32)
					if err == nil {
						sb.WriteRune(rune(code))
						i += 6
						continue
					}
				}
				sb.WriteByte('\\')
				i++
			default:
				sb.WriteByte('\\')
				sb.WriteByte(next)
				i += 2
			}
			continue
		}
		sb.WriteByte(ch)
		i++
	}

	return "", types.NewWebxError(types.ErrContentEmpty, "unterminated JS string in turbo stream")
}

// buildConversationFromArray resolves the root structure and navigates to the
// conversation data inside the React Router loader data.
//
// Structure: root -> loaderData -> share route -> serverResponse -> data
func buildConversationFromArray(arr []any) (map[string]any, error) {
	// Resolve the root object at arr[0].
	rootRaw := resolveValue(arr, arr[0], 0)
	root, ok := rootRaw.(map[string]any)
	if !ok {
		return nil, types.NewWebxError(types.ErrContentEmpty, "turbo stream root is not an object")
	}

	// Navigate: loaderData -> share route -> serverResponse -> data
	loaderData, ok := root["loaderData"].(map[string]any)
	if !ok {
		return nil, types.NewWebxError(types.ErrContentEmpty, "no loaderData in turbo stream root")
	}

	// Find the share route — key contains "share"
	var routeData map[string]any
	for key, val := range loaderData {
		if !strings.Contains(key, "share") {
			continue
		}
		if rd, ok := val.(map[string]any); ok {
			routeData = rd
			break
		}
	}
	if routeData == nil {
		return nil, types.NewWebxError(types.ErrContentEmpty, "no share route in loaderData")
	}

	// Extract serverResponse.data (or serverResponse itself if it has mapping)
	sr, ok := routeData["serverResponse"].(map[string]any)
	if !ok {
		if _, hasMapping := routeData["mapping"]; hasMapping {
			return routeData, nil
		}
		return nil, types.NewWebxError(types.ErrContentEmpty, "no serverResponse in share route")
	}

	if data, ok := sr["data"].(map[string]any); ok {
		if _, hasMapping := data["mapping"]; hasMapping {
			return data, nil
		}
	}
	if _, hasMapping := sr["mapping"]; hasMapping {
		return sr, nil
	}

	return nil, types.NewWebxError(types.ErrContentEmpty, "no conversation mapping in serverResponse")
}

const maxResolveDepth = 30

// resolveValue recursively resolves Turbo Stream references.
//
// Rules:
//   - Objects with "_NNN" keys: NNN is the array index of the real key name,
//     the value is an array index reference.
//   - Negative integers: null.
//   - Arrays: each element is resolved recursively.
//   - Other types (string, bool, float64): literal values.
func resolveValue(arr []any, val any, depth int) any {
	if depth > maxResolveDepth {
		return val
	}

	switch v := val.(type) {
	case map[string]any:
		return resolveObject(arr, v, depth)
	case []any:
		return resolveArray(arr, v, depth)
	case float64:
		// In JSON, all numbers are float64. Integer indices are whole numbers.
		idx := int(v)
		if v == float64(idx) && idx >= 0 && idx < len(arr) {
			// This could be an index reference. But we only resolve it when
			// we KNOW we're in a _NNN context (handled by resolveObject).
			// At the top level or in arrays, we need context to decide.
			// For safety, return as-is here; resolveObject handles dereference.
			return v
		}
		return v
	default:
		return val
	}
}

// resolveObject resolves a Turbo Stream object with "_NNN" indexed keys.
func resolveObject(arr []any, obj map[string]any, depth int) any {
	// Check if this is a _NNN-keyed object.
	hasIndexedKeys := false
	for k := range obj {
		if strings.HasPrefix(k, "_") {
			if _, err := strconv.Atoi(k[1:]); err == nil {
				hasIndexedKeys = true
				break
			}
		}
	}

	if !hasIndexedKeys {
		// Regular object — just resolve values recursively.
		result := make(map[string]any, len(obj))
		for k, v := range obj {
			result[k] = resolveValue(arr, v, depth+1)
		}
		return result
	}

	// _NNN-keyed object: resolve both keys and values from the array.
	result := make(map[string]any, len(obj))
	for k, v := range obj {
		if !strings.HasPrefix(k, "_") {
			result[k] = resolveValue(arr, v, depth+1)
			continue
		}

		keyIdx, err := strconv.Atoi(k[1:])
		if err != nil || keyIdx < 0 || keyIdx >= len(arr) {
			result[k] = resolveValue(arr, v, depth+1)
			continue
		}

		// The real key name is at arr[keyIdx].
		realKey, ok := arr[keyIdx].(string)
		if !ok {
			result[k] = resolveValue(arr, v, depth+1)
			continue
		}

		// The value: dereference if it's an integer index.
		var realVal any
		switch vv := v.(type) {
		case float64:
			idx := int(vv)
			if vv == float64(idx) {
				if idx < 0 {
					// Negative index = null
					realVal = nil
				} else if idx < len(arr) {
					realVal = resolveValue(arr, arr[idx], depth+1)
				} else {
					realVal = v
				}
			} else {
				realVal = v
			}
		default:
			realVal = resolveValue(arr, v, depth+1)
		}

		result[realKey] = realVal
	}

	return result
}

// resolveArray resolves each element of an array from the Turbo Stream format.
// Array elements that are integer indices are dereferenced.
func resolveArray(arr []any, list []any, depth int) any {
	result := make([]any, len(list))
	for i, item := range list {
		switch v := item.(type) {
		case float64:
			idx := int(v)
			if v == float64(idx) && idx >= 0 && idx < len(arr) {
				result[i] = resolveValue(arr, arr[idx], depth+1)
			} else {
				result[i] = v
			}
		default:
			result[i] = resolveValue(arr, item, depth+1)
		}
	}
	return result
}
