package claude

import (
	"strings"

	"github.com/oaooao/webx/internal/types"
)

// ParseConversation converts the raw snapshot JSON into a structured Conversation.
func ParseConversation(data map[string]any, shareID string) (*Conversation, error) {
	title, _ := data["name"].(string)
	if title == "" {
		title = "Claude Conversation"
	}

	createdAt, _ := data["created_at"].(string)
	model, _ := data["model"].(string)

	rawMessages, ok := data["chat_messages"].([]any)
	if !ok {
		return nil, types.NewWebxError(types.ErrContentEmpty, "chat_messages field is missing or invalid")
	}

	var messages []Message
	for _, raw := range rawMessages {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		msg := parseMessage(m)
		if msg != nil {
			messages = append(messages, *msg)
		}
	}

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

// parseMessage extracts a single message from the raw JSON object.
// Returns nil for messages that should be skipped (e.g. system messages).
func parseMessage(m map[string]any) *Message {
	sender, _ := m["sender"].(string)

	// Only keep human and assistant messages.
	if sender != "human" && sender != "assistant" {
		return nil
	}

	// Extract content: try "text" field first, then "content" array.
	content := extractContent(m)
	if strings.TrimSpace(content) == "" {
		return nil
	}

	uuid, _ := m["uuid"].(string)
	createdAt, _ := m["created_at"].(string)

	return &Message{
		UUID:      uuid,
		Role:      sender,
		Content:   content,
		CreatedAt: createdAt,
	}
}

// extractContent tries multiple strategies to get the text content from a message.
func extractContent(m map[string]any) string {
	// Strategy 1: direct "text" field.
	if text, ok := m["text"].(string); ok && text != "" {
		return text
	}

	// Strategy 2: "content" array with type/text objects.
	if contentArr, ok := m["content"].([]any); ok {
		var parts []string
		for _, item := range contentArr {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if t, _ := obj["type"].(string); t == "text" {
				if text, _ := obj["text"].(string); text != "" {
					parts = append(parts, text)
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	}

	return ""
}
