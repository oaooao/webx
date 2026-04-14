package claude

import "testing"

func TestExtractShareID_WhenValidPath_ShouldExtractUUID(t *testing.T) {
	cases := []struct {
		path   string
		wantID string
	}{
		{"/share/48088842-673f-4ef9-a867-a8add9e71549", "48088842-673f-4ef9-a867-a8add9e71549"},
		{"/share/abc-def", "abc-def"},
	}

	for _, tc := range cases {
		got := ExtractShareID(tc.path)
		if got != tc.wantID {
			t.Errorf("ExtractShareID(%q) = %q, want %q", tc.path, got, tc.wantID)
		}
	}
}

func TestExtractShareID_WhenInvalidPath_ShouldReturnEmpty(t *testing.T) {
	cases := []string{
		"/chat/some-id",
		"/",
		"",
		"/share/",
	}

	for _, path := range cases {
		got := ExtractShareID(path)
		if got != "" {
			t.Errorf("ExtractShareID(%q) = %q, want empty", path, got)
		}
	}
}

func TestParseConversation_WhenValidData_ShouldParseMessages(t *testing.T) {
	data := map[string]any{
		"name":       "Test Conversation",
		"created_at": "2024-01-01T00:00:00Z",
		"model":      "claude-3-opus-20240229",
		"chat_messages": []any{
			map[string]any{
				"uuid":       "msg-1",
				"sender":     "human",
				"text":       "Hello, Claude!",
				"created_at": "2024-01-01T00:00:01Z",
			},
			map[string]any{
				"uuid":   "msg-2",
				"sender": "assistant",
				"text":   "Hello! How can I help you?",
			},
		},
	}

	conv, err := ParseConversation(data, "test-share-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conv.Title != "Test Conversation" {
		t.Errorf("Title: got %q, want %q", conv.Title, "Test Conversation")
	}
	if conv.Model != "claude-3-opus-20240229" {
		t.Errorf("Model: got %q", conv.Model)
	}
	if len(conv.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(conv.Messages))
	}
	if conv.Messages[0].Role != "human" {
		t.Errorf("first message role: got %q", conv.Messages[0].Role)
	}
	if conv.Messages[1].Content != "Hello! How can I help you?" {
		t.Errorf("second message content: got %q", conv.Messages[1].Content)
	}
}

func TestParseConversation_WhenNoChatMessages_ShouldReturnError(t *testing.T) {
	data := map[string]any{
		"name": "Empty Conversation",
	}
	_, err := ParseConversation(data, "test-id")
	if err == nil {
		t.Error("expected error for missing chat_messages")
	}
}

func TestParseConversation_WhenContentArray_ShouldExtractText(t *testing.T) {
	data := map[string]any{
		"name": "Content Array Test",
		"chat_messages": []any{
			map[string]any{
				"uuid":   "msg-1",
				"sender": "assistant",
				"content": []any{
					map[string]any{
						"type": "text",
						"text": "Part one.",
					},
					map[string]any{
						"type": "text",
						"text": "Part two.",
					},
				},
			},
		},
	}

	conv, err := ParseConversation(data, "test-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conv.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(conv.Messages))
	}
	if conv.Messages[0].Content != "Part one.\nPart two." {
		t.Errorf("content: got %q", conv.Messages[0].Content)
	}
}

func TestParseConversation_WhenSystemMessage_ShouldSkip(t *testing.T) {
	data := map[string]any{
		"name": "System Skip Test",
		"chat_messages": []any{
			map[string]any{
				"uuid":   "sys-1",
				"sender": "system",
				"text":   "System prompt",
			},
			map[string]any{
				"uuid":   "msg-1",
				"sender": "human",
				"text":   "Hello",
			},
		},
	}

	conv, err := ParseConversation(data, "test-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conv.Messages) != 1 {
		t.Fatalf("expected 1 message (system skipped), got %d", len(conv.Messages))
	}
	if conv.Messages[0].Role != "human" {
		t.Errorf("expected human message, got %q", conv.Messages[0].Role)
	}
}

func TestFormatModel_WhenKnownModel_ShouldReturnFriendlyName(t *testing.T) {
	cases := []struct {
		slug string
		want string
	}{
		{"claude-3-opus-20240229", "Claude 3 Opus"},
		{"claude-sonnet-4-20250514", "Claude Sonnet 4"},
	}

	for _, tc := range cases {
		got := FormatModel(tc.slug)
		if got != tc.want {
			t.Errorf("FormatModel(%q) = %q, want %q", tc.slug, got, tc.want)
		}
	}
}

func TestFormatModel_WhenUnknownModel_ShouldReturnSlug(t *testing.T) {
	slug := "some-unknown-model-2025"
	got := FormatModel(slug)
	if got != slug {
		t.Errorf("FormatModel(%q) = %q, want %q", slug, got, slug)
	}
}

func TestRenderMarkdown_WhenBasicConversation_ShouldContainTitleAndMessages(t *testing.T) {
	conv := &Conversation{
		Title: "Test Chat",
		Model: "claude-3-opus-20240229",
		Messages: []Message{
			{Role: "human", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		},
	}
	md := RenderMarkdown(conv)
	if md == "" {
		t.Fatal("expected non-empty markdown")
	}
	if len(md) < 20 {
		t.Errorf("markdown too short: %q", md)
	}
}
