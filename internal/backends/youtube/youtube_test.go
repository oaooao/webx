package youtube

import "testing"

func TestExtractVideoID_WhenStandardURL_ShouldExtractID(t *testing.T) {
	cases := []struct {
		url    string
		wantID string
	}{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://m.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=120", "dQw4w9WgXcQ"},
		{"https://www.youtube.com/watch?list=PL123&v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
	}

	for _, tc := range cases {
		got := ExtractVideoID(tc.url)
		if got != tc.wantID {
			t.Errorf("ExtractVideoID(%q) = %q, want %q", tc.url, got, tc.wantID)
		}
	}
}

func TestExtractVideoID_WhenShortURL_ShouldExtractID(t *testing.T) {
	cases := []struct {
		url    string
		wantID string
	}{
		{"https://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://youtu.be/dQw4w9WgXcQ?t=120", "dQw4w9WgXcQ"},
	}

	for _, tc := range cases {
		got := ExtractVideoID(tc.url)
		if got != tc.wantID {
			t.Errorf("ExtractVideoID(%q) = %q, want %q", tc.url, got, tc.wantID)
		}
	}
}

func TestExtractVideoID_WhenShortsURL_ShouldExtractID(t *testing.T) {
	got := ExtractVideoID("https://www.youtube.com/shorts/abc123def45")
	if got != "abc123def45" {
		t.Errorf("ExtractVideoID(shorts) = %q, want %q", got, "abc123def45")
	}
}

func TestExtractVideoID_WhenNoID_ShouldReturnEmpty(t *testing.T) {
	noIDCases := []string{
		"https://www.youtube.com/",
		"https://www.youtube.com/playlist?list=PL123",
		"https://www.youtube.com/channel/UC123",
		"https://example.com/watch?v=abc",
		"",
	}

	for _, raw := range noIDCases {
		got := ExtractVideoID(raw)
		if got != "" {
			t.Errorf("ExtractVideoID(%q) = %q, want empty", raw, got)
		}
	}
}

func TestFormatDuration_ShouldFormatCorrectly(t *testing.T) {
	cases := []struct {
		seconds int64
		want    string
	}{
		{0, ""},
		{-1, ""},
		{65, "01:05"},
		{3600, "01:00:00"},
		{3661, "01:01:01"},
		{120, "02:00"},
		{59, "00:59"},
	}

	for _, tc := range cases {
		got := formatDuration(tc.seconds)
		if got != tc.want {
			t.Errorf("formatDuration(%d) = %q, want %q", tc.seconds, got, tc.want)
		}
	}
}

func TestRenderMarkdown_WhenBasicResult_ShouldContainTitle(t *testing.T) {
	result := &FetchResult{
		Video: VideoMeta{
			ID:      "abc123",
			Title:   "Test Video",
			Channel: "Test Channel",
		},
	}
	md := RenderMarkdown(result)
	if md == "" {
		t.Fatal("expected non-empty markdown")
	}
	if len(md) < 10 {
		t.Errorf("markdown too short: %q", md)
	}
}

func TestRenderMarkdown_WhenHasTranscript_ShouldContainTranscriptSection(t *testing.T) {
	result := &FetchResult{
		Video: VideoMeta{
			ID:    "abc123",
			Title: "Test Video",
		},
		Transcript: []TranscriptSegment{
			{Start: 0, Duration: 5, Text: "Hello"},
			{Start: 5, Duration: 5, Text: "World"},
		},
	}
	md := RenderMarkdown(result)
	if md == "" {
		t.Fatal("expected non-empty markdown")
	}
}
