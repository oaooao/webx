package twitter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oaooao/webx/internal/types"
)

// buildSearchTimelineFixture returns a minimal Twitter GraphQL SearchTimeline response.
func buildSearchTimelineFixture() json.RawMessage {
	return json.RawMessage(`{
		"data": {
			"search_by_raw_query": {
				"search_timeline": {
					"timeline": {
						"instructions": [
							{
								"type": "TimelineAddEntries",
								"entries": [
									{
										"entryId": "tweet-1001",
										"content": {
											"entryType": "TimelineTimelineItem",
											"itemContent": {
												"itemType": "TimelineTweet",
												"tweet_results": {
													"result": {
														"__typename": "Tweet",
														"rest_id": "1001",
														"core": {
															"user_results": {
																"result": {
																	"legacy": {
																		"name": "Go Developer",
																		"screen_name": "godev"
																	}
																}
															}
														},
														"legacy": {
															"full_text": "Just released a new #golang package for AI agents! #go #ai",
															"created_at": "Mon Apr 14 09:00:00 +0000 2025",
															"conversation_id_str": "1001",
															"favorite_count": 150,
															"retweet_count": 30,
															"reply_count": 12,
															"quote_count": 5,
															"bookmark_count": 20
														}
													}
												}
											}
										}
									},
									{
										"entryId": "tweet-1002",
										"content": {
											"entryType": "TimelineTimelineItem",
											"itemContent": {
												"itemType": "TimelineTweet",
												"tweet_results": {
													"result": {
														"__typename": "Tweet",
														"rest_id": "1002",
														"core": {
															"user_results": {
																"result": {
																	"legacy": {
																		"name": "AI Researcher",
																		"screen_name": "airesearch"
																	}
																}
															}
														},
														"legacy": {
															"full_text": "webx is a great tool for building AI agents with web access.",
															"created_at": "Sun Apr 13 15:00:00 +0000 2025",
															"conversation_id_str": "1002",
															"favorite_count": 75,
															"retweet_count": 10,
															"reply_count": 3,
															"quote_count": 2,
															"bookmark_count": 8
														}
													}
												}
											}
										}
									}
								]
							}
						]
					}
				}
			}
		}
	}`)
}

func buildSearchTimelineFixtureEmpty() json.RawMessage {
	return json.RawMessage(`{
		"data": {
			"search_by_raw_query": {
				"search_timeline": {
					"timeline": {
						"instructions": [
							{
								"type": "TimelineAddEntries",
								"entries": []
							}
						]
					}
				}
			}
		}
	}`)
}

func TestParseTwitterSearchResponse_WhenValid_ShouldParseTweets(t *testing.T) {
	result, err := ParseSearchTimelineResponse(buildSearchTimelineFixture())
	if err != nil {
		t.Fatalf("ParseSearchTimelineResponse: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 tweets, got %d", len(result))
	}
	tw := result[0]
	if tw.ID != "1001" {
		t.Errorf("ID: got %q, want %q", tw.ID, "1001")
	}
	if tw.Author.ScreenName != "godev" {
		t.Errorf("Author.ScreenName: got %q, want %q", tw.Author.ScreenName, "godev")
	}
	if tw.Text == "" {
		t.Error("Text should not be empty")
	}
	if tw.Metrics["favorite_count"] != 150 {
		t.Errorf("favorite_count: got %d, want 150", tw.Metrics["favorite_count"])
	}
	if tw.Metrics["retweet_count"] != 30 {
		t.Errorf("retweet_count: got %d, want 30", tw.Metrics["retweet_count"])
	}
}

func TestParseTwitterSearchResponse_WhenEmpty_ShouldReturnEmptySlice(t *testing.T) {
	result, err := ParseSearchTimelineResponse(buildSearchTimelineFixtureEmpty())
	if err != nil {
		t.Fatalf("ParseSearchTimelineResponse: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 tweets for empty response, got %d", len(result))
	}
}

func TestParseTwitterSearchResponse_WhenInvalidJSON_ShouldReturnError(t *testing.T) {
	_, err := ParseSearchTimelineResponse(json.RawMessage(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseTwitterSearchResponse_WhenNoDataKey_ShouldReturnNil(t *testing.T) {
	result, err := ParseSearchTimelineResponse(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for response with no data key, got %d tweets", len(result))
	}
}

func TestSearchTwitter_WhenNoAuth_ShouldReturnError(t *testing.T) {
	ctx := types.SearchContext{
		Query:    "webx",
		Platform: "twitter",
		Options:  types.DefaultSearchOptions(),
	}
	_, err := SearchTwitter(ctx, "", "")
	if err == nil {
		t.Error("expected error when no auth token is provided")
	}
}

func TestSearchTwitterHTTP_WhenAPIReturnsResults(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildSearchTimelineFixture())
	}))
	defer ts.Close()

	result, err := SearchTwitterWithURL(ts.URL, "webx", "fake-token", "fake-ct0", 20, "Top")
	if err != nil {
		t.Fatalf("SearchTwitterWithURL: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	if result.Items[0].Author != "godev" {
		t.Errorf("first author: got %q, want %q", result.Items[0].Author, "godev")
	}
}

func TestSearchTwitterHTTP_WhenAuthError_ShouldReturnError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"errors": [{"message": "Bad guest token."}]}`))
	}))
	defer ts.Close()

	_, err := SearchTwitterWithURL(ts.URL, "webx", "bad-token", "bad-ct0", 20, "Top")
	if err == nil {
		t.Error("expected error for HTTP 401")
	}
}
