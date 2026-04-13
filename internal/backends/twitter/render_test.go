package twitter

import (
	"strings"
	"testing"
)

func TestRenderMarkdown_SingleTweet(t *testing.T) {
	tweets := []Tweet{
		{
			ID:   "1",
			Text: "Hello, world!",
			Author: Author{
				Name:       "Test User",
				ScreenName: "testuser",
			},
			CreatedAt: "Sun Apr 13 10:00:00 +0000 2025",
		},
	}

	md := RenderMarkdown(tweets)

	checks := []string{
		"# Tweet by @testuser",
		"**Test User** (@testuser)",
		"Sun Apr 13 10:00:00 +0000 2025",
		"Hello, world!",
	}
	for _, want := range checks {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
}

func TestRenderMarkdown_WithReply(t *testing.T) {
	tweets := []Tweet{
		{
			ID:     "1",
			Text:   "Original tweet.",
			Author: Author{Name: "Alice", ScreenName: "alice"},
		},
		{
			ID:     "2",
			Text:   "This is a reply.",
			Author: Author{Name: "Bob", ScreenName: "bob"},
		},
	}

	md := RenderMarkdown(tweets)

	if !strings.Contains(md, "# Tweet by @alice") {
		t.Error("missing focal tweet header")
	}
	if !strings.Contains(md, "## Reply by @bob") {
		t.Error("missing reply header")
	}
	if !strings.Contains(md, "---") {
		t.Error("missing separator between tweets")
	}
	if !strings.Contains(md, "This is a reply.") {
		t.Error("missing reply text")
	}
}

func TestRenderMarkdown_WithMedia(t *testing.T) {
	tweets := []Tweet{
		{
			ID:     "1",
			Text:   "Look at this!",
			Author: Author{ScreenName: "photog"},
			Media: []Media{
				{Type: "photo", URL: "https://pbs.twimg.com/media/photo1.jpg"},
				{Type: "video", URL: "https://video.twimg.com/vid1.mp4"},
				{Type: "animated_gif", URL: "https://video.twimg.com/gif1.mp4"},
			},
		},
	}

	md := RenderMarkdown(tweets)

	if !strings.Contains(md, "![photo](https://pbs.twimg.com/media/photo1.jpg)") {
		t.Error("missing photo markdown")
	}
	if !strings.Contains(md, "[video](https://video.twimg.com/vid1.mp4)") {
		t.Error("missing video markdown")
	}
	if !strings.Contains(md, "[gif](https://video.twimg.com/gif1.mp4)") {
		t.Error("missing gif markdown")
	}
}

func TestRenderMarkdown_WithQuotedTweet(t *testing.T) {
	tweets := []Tweet{
		{
			ID:     "1",
			Text:   "Quoting this:",
			Author: Author{ScreenName: "quoter"},
			QuotedTweet: &Tweet{
				ID:     "2",
				Text:   "The original thought.\nWith a second line.",
				Author: Author{ScreenName: "original"},
			},
		},
	}

	md := RenderMarkdown(tweets)

	if !strings.Contains(md, "> **Quoted:** @original") {
		t.Error("missing quoted tweet header")
	}
	if !strings.Contains(md, "> The original thought.") {
		t.Error("missing quoted tweet text")
	}
	// Second line should also be blockquoted.
	if !strings.Contains(md, "> With a second line.") {
		t.Error("missing second line of quoted tweet")
	}
}

func TestRenderMarkdown_WithMetrics(t *testing.T) {
	tweets := []Tweet{
		{
			ID:     "1",
			Text:   "Popular tweet.",
			Author: Author{ScreenName: "popular"},
			Metrics: map[string]int{
				"favorite_count": 1000,
				"retweet_count":  200,
				"reply_count":    50,
				"quote_count":    10,
				"bookmark_count": 5,
			},
		},
	}

	md := RenderMarkdown(tweets)

	// Metrics should appear in order: replies, retweets, quotes, likes, bookmarks.
	checks := []string{
		"50 replies",
		"200 retweets",
		"10 quotes",
		"1000 likes",
		"5 bookmarks",
	}
	for _, want := range checks {
		if !strings.Contains(md, want) {
			t.Errorf("missing metric %q in:\n%s", want, md)
		}
	}

	// Should be italic (wrapped in *)
	if !strings.Contains(md, "*50 replies") {
		t.Error("metrics line should start with italic marker")
	}
}

func TestRenderMarkdown_ZeroMetricsOmitted(t *testing.T) {
	tweets := []Tweet{
		{
			ID:     "1",
			Text:   "Quiet tweet.",
			Author: Author{ScreenName: "quiet"},
			Metrics: map[string]int{
				"favorite_count": 1,
			},
		},
	}

	md := RenderMarkdown(tweets)
	if strings.Contains(md, "retweets") {
		t.Error("zero-value metrics should not appear")
	}
	if !strings.Contains(md, "1 likes") {
		t.Error("non-zero metric missing")
	}
}

func TestRenderMarkdown_Empty(t *testing.T) {
	md := RenderMarkdown(nil)
	if md != "" {
		t.Errorf("expected empty string for nil tweets, got %q", md)
	}
	md = RenderMarkdown([]Tweet{})
	if md != "" {
		t.Errorf("expected empty string for empty tweets, got %q", md)
	}
}

func TestRenderMarkdown_AuthorFallbacks(t *testing.T) {
	// Only Name, no ScreenName.
	tweets := []Tweet{
		{
			ID:     "1",
			Text:   "Name only.",
			Author: Author{Name: "Display Name"},
		},
	}
	md := RenderMarkdown(tweets)
	if !strings.Contains(md, "Display Name") {
		t.Error("display name should appear when screen_name is empty")
	}

	// No name, no screen name.
	tweets2 := []Tweet{
		{
			ID:   "2",
			Text: "Anonymous.",
		},
	}
	md2 := RenderMarkdown(tweets2)
	if !strings.Contains(md2, "unknown") {
		t.Error("should fall back to 'unknown' when no author info")
	}
}
