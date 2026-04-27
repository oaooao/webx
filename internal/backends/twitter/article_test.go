package twitter

import (
	"encoding/json"
	"strings"
	"testing"
)

// articlePreviewFixture mirrors what TweetDetail returns for an X Article:
// only title + preview_text, no content_state. The adapter must trigger a
// follow-up TweetResultByRestId fetch to enrich.
const articlePreviewFixture = `{
  "rest_id": "999",
  "core": {"user_results": {"result": {"legacy": {"name": "Author", "screen_name": "author"}}}},
  "legacy": {"full_text": "https://t.co/abc", "created_at": "Sun Apr 13 10:00:00 +0000 2026", "conversation_id_str": "999"},
  "article": {
    "article_results": {
      "result": {
        "rest_id": "888",
        "title": "Preview Only Article",
        "preview_text": "First line of the preview..."
      }
    }
  }
}`

// articleFullFixture mirrors TweetResultByRestId output: title + Draft.js
// content_state with multiple block types and an inline LINK entity.
const articleFullFixture = `{
  "data": {
    "tweetResult": {
      "result": {
        "__typename": "Tweet",
        "rest_id": "999",
        "core": {"user_results": {"result": {"legacy": {"name": "Author", "screen_name": "author"}}}},
        "legacy": {"full_text": "https://t.co/abc", "created_at": "Sun Apr 13 10:00:00 +0000 2026", "conversation_id_str": "999"},
        "article": {
          "article_results": {
            "result": {
              "rest_id": "888",
              "title": "Full Article",
              "content_state": {
                "blocks": [
                  {"type": "header-one", "text": "Intro", "entityRanges": []},
                  {"type": "unstyled", "text": "Visit example for more.", "entityRanges": [{"key": 0, "offset": 6, "length": 7}]},
                  {"type": "unordered-list-item", "text": "first", "entityRanges": []},
                  {"type": "unordered-list-item", "text": "second", "entityRanges": []},
                  {"type": "ordered-list-item", "text": "step one", "entityRanges": []},
                  {"type": "ordered-list-item", "text": "step two", "entityRanges": []},
                  {"type": "blockquote", "text": "wisdom", "entityRanges": []},
                  {"type": "code-block", "text": "let x = 1", "entityRanges": []}
                ],
                "entityMap": {
                  "0": {"type": "LINK", "data": {"url": "https://example.com/"}}
                }
              }
            }
          }
        }
      }
    }
  }
}`

func TestExtractArticle_PreviewOnlyReturnsTitle(t *testing.T) {
	var result map[string]json.RawMessage
	if err := json.Unmarshal([]byte(articlePreviewFixture), &result); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	title, text := extractArticle(result)
	if title != "Preview Only Article" {
		t.Errorf("expected title %q, got %q", "Preview Only Article", title)
	}
	if text != "" {
		t.Errorf("expected empty text for preview-only article, got %q", text)
	}
}

func TestParseTweetResultByRestIdResponse_FullArticle(t *testing.T) {
	tweet, err := ParseTweetResultByRestIdResponse(json.RawMessage(articleFullFixture))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if tweet == nil {
		t.Fatal("expected tweet, got nil")
	}
	if tweet.ArticleTitle != "Full Article" {
		t.Errorf("ArticleTitle = %q, want %q", tweet.ArticleTitle, "Full Article")
	}

	body := tweet.ArticleText
	expectations := []string{
		"# Intro",
		"Visit [example](https://example.com/) for more.",
		"- first",
		"- second",
		"1. step one",
		"2. step two",
		"> wisdom",
		"```\nlet x = 1\n```",
	}
	for _, want := range expectations {
		if !strings.Contains(body, want) {
			t.Errorf("ArticleText missing %q\n--- got ---\n%s", want, body)
		}
	}
}

func TestParseTweetResultByRestIdResponse_Empty(t *testing.T) {
	tweet, err := ParseTweetResultByRestIdResponse(json.RawMessage(`{"data": {}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tweet != nil {
		t.Errorf("expected nil tweet for empty response, got %+v", tweet)
	}
}

func TestRenderMarkdown_ArticleSection(t *testing.T) {
	tweet := Tweet{
		ID:           "1",
		Text:         "https://t.co/abc",
		Author:       Author{Name: "Author", ScreenName: "author"},
		ArticleTitle: "Full Article",
		ArticleText:  "# Intro\n\nBody.",
	}
	md := RenderMarkdown([]Tweet{tweet})
	if !strings.Contains(md, "### Article: Full Article") {
		t.Errorf("missing article header in output:\n%s", md)
	}
	if !strings.Contains(md, "# Intro") || !strings.Contains(md, "Body.") {
		t.Errorf("missing article body in output:\n%s", md)
	}
}

func TestRenderMarkdown_NoArticleSectionWhenAbsent(t *testing.T) {
	tweet := Tweet{
		ID:     "1",
		Text:   "plain tweet",
		Author: Author{Name: "Author", ScreenName: "author"},
	}
	md := RenderMarkdown([]Tweet{tweet})
	if strings.Contains(md, "### Article") {
		t.Errorf("unexpected article section in plain tweet output:\n%s", md)
	}
}
