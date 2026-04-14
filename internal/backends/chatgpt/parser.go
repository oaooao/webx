package chatgpt

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/oaooao/webx/internal/types"
)

// ParseConversation converts the raw JSON map into a structured Conversation.
func ParseConversation(data map[string]any, shareID string) (*Conversation, error) {
	title, _ := data["title"].(string)
	if title == "" {
		title = "ChatGPT Conversation"
	}

	var createdAt string
	if ct, ok := data["create_time"].(float64); ok && ct > 0 {
		createdAt = time.Unix(int64(ct), int64((ct-float64(int64(ct)))*1e9)).UTC().Format(time.RFC3339)
	}

	mappingRaw, ok := data["mapping"].(map[string]any)
	if !ok {
		return nil, types.NewWebxError(types.ErrContentEmpty, "conversation mapping is missing or invalid")
	}

	// Parse all nodes.
	nodeMap := make(map[string]*node, len(mappingRaw))
	for id, v := range mappingRaw {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		n := &node{ID: id}

		if p, ok := m["parent"].(string); ok {
			n.Parent = p
		}
		if children, ok := m["children"].([]any); ok {
			for _, c := range children {
				if cs, ok := c.(string); ok {
					n.Children = append(n.Children, cs)
				}
			}
		}

		if msg, ok := m["message"].(map[string]any); ok {
			n.Message = parseMessage(msg)
		}

		nodeMap[id] = n
	}

	// Determine the path from root to current_node.
	currentNode, _ := data["current_node"].(string)
	messages := linearize(nodeMap, currentNode)

	// Try to extract model from messages metadata or top-level.
	model := extractModel(data, mappingRaw)

	conv := &Conversation{
		ID:        shareID,
		Title:     title,
		Model:     model,
		CreatedAt: createdAt,
		Messages:  messages,
	}

	if len(messages) == 0 {
		return conv, types.NewWebxError(types.ErrContentEmpty, "conversation has no messages")
	}

	return conv, nil
}

type node struct {
	ID       string
	Parent   string
	Children []string
	Message  *Message
}

func parseMessage(msg map[string]any) *Message {
	author, _ := msg["author"].(map[string]any)
	role, _ := author["role"].(string)

	// Skip system and tool messages.
	if role != "user" && role != "assistant" {
		return nil
	}

	content, _ := msg["content"].(map[string]any)
	parts, _ := content["parts"].([]any)

	var textParts []string
	for _, p := range parts {
		switch v := p.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				textParts = append(textParts, v)
			}
		case map[string]any:
			// Some parts are objects (e.g. images). Skip them or extract text.
			if text, ok := v["text"].(string); ok && strings.TrimSpace(text) != "" {
				textParts = append(textParts, text)
			}
		}
	}

	if len(textParts) == 0 {
		return nil
	}

	msgID, _ := msg["id"].(string)

	var createdAt string
	if ct, ok := msg["create_time"].(float64); ok && ct > 0 {
		createdAt = time.Unix(int64(ct), int64((ct-float64(int64(ct)))*1e9)).UTC().Format(time.RFC3339)
	}

	return &Message{
		ID:        msgID,
		Role:      role,
		Content:   strings.Join(textParts, "\n"),
		CreatedAt: createdAt,
	}
}

// linearize walks the tree from root to current_node and collects messages in order.
func linearize(nodes map[string]*node, currentNode string) []Message {
	if currentNode == "" {
		// No current_node: fallback to collecting all messages sorted by position.
		return collectAllMessages(nodes)
	}

	// Build the path from current_node back to root.
	var path []string
	cur := currentNode
	visited := make(map[string]bool)
	for cur != "" && !visited[cur] {
		visited[cur] = true
		path = append(path, cur)
		if n, ok := nodes[cur]; ok {
			cur = n.Parent
		} else {
			break
		}
	}

	// Reverse to get root-to-leaf order.
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	var messages []Message
	for _, id := range path {
		n, ok := nodes[id]
		if !ok || n.Message == nil {
			continue
		}
		messages = append(messages, *n.Message)
	}

	return messages
}

// collectAllMessages is the fallback when current_node is not available.
// It walks from each root through children, collecting messages.
func collectAllMessages(nodes map[string]*node) []Message {
	// Find roots (nodes with no parent or parent not in map).
	var roots []string
	for id, n := range nodes {
		if n.Parent == "" {
			roots = append(roots, id)
			continue
		}
		if _, exists := nodes[n.Parent]; !exists {
			roots = append(roots, id)
		}
	}

	sort.Strings(roots)

	var messages []Message
	visited := make(map[string]bool)

	var walk func(id string)
	walk = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true

		n, ok := nodes[id]
		if !ok {
			return
		}
		if n.Message != nil {
			messages = append(messages, *n.Message)
		}
		for _, child := range n.Children {
			walk(child)
		}
	}

	for _, root := range roots {
		walk(root)
	}

	return messages
}

// extractModel tries to find the model slug from the conversation data.
func extractModel(data map[string]any, mapping map[string]any) string {
	// Check top-level model field.
	if model, ok := data["model"].(string); ok && model != "" {
		return model
	}
	if modelSlug, ok := data["model_slug"].(string); ok && modelSlug != "" {
		return modelSlug
	}

	// Check first assistant message metadata.
	for _, v := range mapping {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		msg, ok := m["message"].(map[string]any)
		if !ok {
			continue
		}
		author, _ := msg["author"].(map[string]any)
		role, _ := author["role"].(string)
		if role != "assistant" {
			continue
		}
		metadata, ok := msg["metadata"].(map[string]any)
		if !ok {
			continue
		}
		if slug, ok := metadata["model_slug"].(string); ok && slug != "" {
			return slug
		}
		if mid, ok := metadata["model_id"].(string); ok && mid != "" {
			return mid
		}
	}

	return ""
}

// FormatModel returns a display-friendly model name.
func FormatModel(slug string) string {
	known := map[string]string{
		"gpt-4":          "GPT-4",
		"gpt-4o":         "GPT-4o",
		"gpt-4o-mini":    "GPT-4o Mini",
		"gpt-4-turbo":    "GPT-4 Turbo",
		"gpt-3.5-turbo":  "GPT-3.5 Turbo",
		"o1-preview":     "o1-preview",
		"o1-mini":        "o1-mini",
		"o3-mini":        "o3-mini",
		"text-davinci":   "GPT-3",
	}
	if name, ok := known[slug]; ok {
		return name
	}
	// Check prefix matches.
	for prefix, name := range known {
		if strings.HasPrefix(slug, prefix) {
			return fmt.Sprintf("%s (%s)", name, slug)
		}
	}
	return slug
}
