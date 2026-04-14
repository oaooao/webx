package chatgpt

import "testing"

func TestExtractShareID_WhenValidPath_ShouldExtractUUID(t *testing.T) {
	cases := []struct {
		path   string
		wantID string
	}{
		{"/share/69ddbda6-a51c-83ea-af56-fa0be87039e6", "69ddbda6-a51c-83ea-af56-fa0be87039e6"},
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
		"/c/some-chat-id",
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

func TestFindClosingBrace_WhenSimpleObject_ShouldFindEnd(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{`{"key": "value"}`, 15},
		{`{}`, 1},
		{`{"nested": {"inner": 1}}`, 23},
		{`{"str": "contains } brace"}`, 26},
	}

	for _, tc := range cases {
		got := findClosingBrace(tc.input)
		if got != tc.want {
			t.Errorf("findClosingBrace(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestFindClosingBrace_WhenUnclosed_ShouldReturnNegative(t *testing.T) {
	got := findClosingBrace(`{"unclosed": true`)
	if got != -1 {
		t.Errorf("expected -1 for unclosed brace, got %d", got)
	}
}

func TestFindClosingBrace_WhenEscapedQuotes_ShouldHandleCorrectly(t *testing.T) {
	input := `{"key": "value with \" escaped"}`
	got := findClosingBrace(input)
	if got < 0 {
		t.Errorf("expected valid index, got %d", got)
	}
}

func TestParseConversation_WhenValidMapping_ShouldParseMessages(t *testing.T) {
	data := map[string]any{
		"title":      "Test Conversation",
		"create_time": float64(1700000000),
		"mapping": map[string]any{
			"node-1": map[string]any{
				"id":       "node-1",
				"parent":   nil,
				"children": []any{"node-2"},
				"message": map[string]any{
					"id": "msg-1",
					"author": map[string]any{
						"role": "user",
					},
					"content": map[string]any{
						"content_type": "text",
						"parts":        []any{"Hello, ChatGPT!"},
					},
					"create_time": float64(1700000001),
				},
			},
			"node-2": map[string]any{
				"id":       "node-2",
				"parent":   "node-1",
				"children": []any{},
				"message": map[string]any{
					"id": "msg-2",
					"author": map[string]any{
						"role": "assistant",
					},
					"content": map[string]any{
						"content_type": "text",
						"parts":        []any{"Hello! How can I help you?"},
					},
					"create_time": float64(1700000002),
				},
			},
		},
	}

	conv, err := ParseConversation(data, "test-share-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conv.Title != "Test Conversation" {
		t.Errorf("Title: got %q", conv.Title)
	}
	if len(conv.Messages) < 1 {
		t.Fatalf("expected at least 1 message, got %d", len(conv.Messages))
	}
}

func TestParseConversation_WhenNoMapping_ShouldReturnError(t *testing.T) {
	data := map[string]any{
		"title": "Empty",
	}
	_, err := ParseConversation(data, "test-id")
	if err == nil {
		t.Error("expected error for missing mapping")
	}
}
