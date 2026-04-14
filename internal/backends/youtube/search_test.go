package youtube

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// buildYouTubeSearchPageFixture returns a minimal HTML page containing ytInitialData
// with a videoRenderer entry, similar to what YouTube's /results?search_query= returns.
func buildYouTubeSearchPageFixture() string {
	// This is a stripped-down version of the real ytInitialData structure.
	// Only the fields needed by the search parser are included.
	ytInitialData := `{
		"contents": {
			"twoColumnSearchResultsRenderer": {
				"primaryContents": {
					"sectionListRenderer": {
						"contents": [
							{
								"itemSectionRenderer": {
									"contents": [
										{
											"videoRenderer": {
												"videoId": "dQw4w9WgXcQ",
												"title": {
													"runs": [{"text": "Go Tutorial for Beginners"}]
												},
												"ownerText": {
													"runs": [{"text": "Tech Channel"}]
												},
												"viewCountText": {
													"simpleText": "1,234,567 views"
												},
												"descriptionSnippet": {
													"runs": [{"text": "Learn Go programming from scratch in this tutorial."}]
												},
												"publishedTimeText": {
													"simpleText": "2 years ago"
												}
											}
										},
										{
											"videoRenderer": {
												"videoId": "abc123XYZww",
												"title": {
													"runs": [{"text": "Advanced Go Concurrency"}]
												},
												"ownerText": {
													"runs": [{"text": "Golang Masters"}]
												},
												"viewCountText": {
													"simpleText": "567,890 views"
												},
												"descriptionSnippet": {
													"runs": [{"text": "Deep dive into goroutines and channels."}]
												},
												"publishedTimeText": {
													"simpleText": "1 year ago"
												}
											}
										}
									]
								}
							}
						]
					}
				}
			}
		}
	}`

	return `<!DOCTYPE html><html><head></head><body>
<script>var ytInitialData = ` + ytInitialData + `;</script>
</body></html>`
}

func TestParseYouTubeSearchResponse_WhenValid_ShouldParseVideos(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(buildYouTubeSearchPageFixture()))
	}))
	defer ts.Close()

	result, err := SearchVideos(ts.URL+"?search_query=golang", 20)
	if err != nil {
		t.Fatalf("SearchVideos: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}

	// First video
	item := result.Items[0]
	if item.Title != "Go Tutorial for Beginners" {
		t.Errorf("title: got %q, want %q", item.Title, "Go Tutorial for Beginners")
	}
	if item.Author != "Tech Channel" {
		t.Errorf("channel: got %q, want %q", item.Author, "Tech Channel")
	}
	if !strings.Contains(item.URL, "dQw4w9WgXcQ") {
		t.Errorf("URL should contain video ID dQw4w9WgXcQ, got %q", item.URL)
	}
	if item.Snippet == "" {
		t.Error("snippet should not be empty")
	}
	// Meta should contain view_count
	if item.Meta == nil {
		t.Error("Meta should not be nil")
	}
}

func TestParseYouTubeSearchResponse_WhenEmpty_ShouldReturnEmptyItems(t *testing.T) {
	emptyPage := `<!DOCTYPE html><html><head></head><body>
<script>var ytInitialData = {"contents": {"twoColumnSearchResultsRenderer": {"primaryContents": {"sectionListRenderer": {"contents": [{"itemSectionRenderer": {"contents": []}}]}}}}};</script>
</body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(emptyPage))
	}))
	defer ts.Close()

	result, err := SearchVideos(ts.URL+"?search_query=xyznotexist", 20)
	if err != nil {
		t.Fatalf("SearchVideos: %v", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(result.Items))
	}
}

func TestParseYouTubeSearchResponse_WhenNoInitialData_ShouldReturnError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body>No ytInitialData here</body></html>`))
	}))
	defer ts.Close()

	_, err := SearchVideos(ts.URL+"?search_query=golang", 20)
	if err == nil {
		t.Error("expected error when ytInitialData is missing")
	}
}

func TestParseYouTubeSearchResponse_WhenHTTPError_ShouldReturnError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer ts.Close()

	_, err := SearchVideos(ts.URL+"?search_query=golang", 20)
	if err == nil {
		t.Error("expected error for HTTP 500")
	}
}
