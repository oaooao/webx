package twitter

import (
	"encoding/json"
	"testing"
)

// --- Fixtures ---

// currentFormatFixture mirrors the current Twitter GraphQL response shape:
// data.threaded_conversation_with_injections_v2 (no intermediate tweetDetailV2 layer).
const currentFormatFixture = `{
  "data": {
    "threaded_conversation_with_injections_v2": {
      "instructions": [
        {
          "type": "TimelineAddEntries",
          "entries": [
            {
              "entryId": "tweet-1234567890",
              "sortIndex": "1234567890",
              "content": {
                "entryType": "TimelineTimelineItem",
                "itemContent": {
                  "itemType": "TimelineTweet",
                  "tweet_results": {
                    "result": {
                      "__typename": "Tweet",
                      "rest_id": "1234567890",
                      "core": {
                        "user_results": {
                          "result": {
                            "legacy": {
                              "name": "Test User",
                              "screen_name": "testuser"
                            }
                          }
                        }
                      },
                      "legacy": {
                        "full_text": "Hello, world! This is a test tweet.",
                        "created_at": "Sun Apr 13 10:00:00 +0000 2025",
                        "conversation_id_str": "1234567890",
                        "favorite_count": 42,
                        "retweet_count": 7,
                        "reply_count": 3,
                        "quote_count": 1,
                        "bookmark_count": 2,
                        "extended_entities": {
                          "media": [
                            {
                              "type": "photo",
                              "media_url_https": "https://pbs.twimg.com/media/test123.jpg"
                            }
                          ]
                        }
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
}`

// oldFormatFixture wraps the conversation inside data.tweetDetailV2.
const oldFormatFixture = `{
  "data": {
    "tweetDetailV2": {
      "threaded_conversation_with_injections_v2": {
        "instructions": [
          {
            "type": "TimelineAddEntries",
            "entries": [
              {
                "entryId": "tweet-9999",
                "content": {
                  "entryType": "TimelineTimelineItem",
                  "itemContent": {
                    "tweet_results": {
                      "result": {
                        "__typename": "Tweet",
                        "rest_id": "9999",
                        "core": {
                          "user_results": {
                            "result": {
                              "legacy": {
                                "name": "Old Format User",
                                "screen_name": "oldformat"
                              }
                            }
                          }
                        },
                        "legacy": {
                          "full_text": "Tweet from old format.",
                          "created_at": "Sat Apr 12 09:00:00 +0000 2025",
                          "conversation_id_str": "9999",
                          "favorite_count": 10,
                          "retweet_count": 0,
                          "reply_count": 0,
                          "quote_count": 0,
                          "bookmark_count": 0
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
}`

// visibilityResultsFixture wraps tweet in TweetWithVisibilityResults.
const visibilityResultsFixture = `{
  "data": {
    "threaded_conversation_with_injections_v2": {
      "instructions": [
        {
          "entries": [
            {
              "entryId": "tweet-5555",
              "content": {
                "itemContent": {
                  "tweet_results": {
                    "result": {
                      "__typename": "TweetWithVisibilityResults",
                      "tweet": {
                        "__typename": "Tweet",
                        "rest_id": "5555",
                        "core": {
                          "user_results": {
                            "result": {
                              "legacy": {
                                "name": "Vis User",
                                "screen_name": "visuser"
                              }
                            }
                          }
                        },
                        "legacy": {
                          "full_text": "I'm wrapped in visibility results.",
                          "created_at": "Fri Apr 11 08:00:00 +0000 2025",
                          "conversation_id_str": "5555",
                          "favorite_count": 1,
                          "retweet_count": 0,
                          "reply_count": 0,
                          "quote_count": 0,
                          "bookmark_count": 0
                        }
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
}`

// noteTweetFixture contains a long-form note tweet that overrides full_text.
const noteTweetFixture = `{
  "data": {
    "threaded_conversation_with_injections_v2": {
      "instructions": [
        {
          "entries": [
            {
              "entryId": "tweet-7777",
              "content": {
                "itemContent": {
                  "tweet_results": {
                    "result": {
                      "__typename": "Tweet",
                      "rest_id": "7777",
                      "core": {
                        "user_results": {
                          "result": {
                            "legacy": {
                              "name": "Note Author",
                              "screen_name": "noteauthor"
                            }
                          }
                        }
                      },
                      "legacy": {
                        "full_text": "This is the short truncated text...",
                        "created_at": "Thu Apr 10 12:00:00 +0000 2025",
                        "conversation_id_str": "7777",
                        "favorite_count": 100,
                        "retweet_count": 0,
                        "reply_count": 0,
                        "quote_count": 0,
                        "bookmark_count": 0
                      },
                      "note_tweet": {
                        "note_tweet_results": {
                          "result": {
                            "text": "This is the full long-form note tweet content that exceeds the normal character limit and provides much more detail about the topic at hand."
                          }
                        }
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
}`

// quotedTweetFixture contains a tweet quoting another tweet.
const quotedTweetFixture = `{
  "data": {
    "threaded_conversation_with_injections_v2": {
      "instructions": [
        {
          "entries": [
            {
              "entryId": "tweet-8888",
              "content": {
                "itemContent": {
                  "tweet_results": {
                    "result": {
                      "__typename": "Tweet",
                      "rest_id": "8888",
                      "core": {
                        "user_results": {
                          "result": {
                            "legacy": {
                              "name": "Quoter",
                              "screen_name": "quoter"
                            }
                          }
                        }
                      },
                      "legacy": {
                        "full_text": "Check this out!",
                        "created_at": "Wed Apr 09 15:00:00 +0000 2025",
                        "conversation_id_str": "8888",
                        "favorite_count": 5,
                        "retweet_count": 1,
                        "reply_count": 0,
                        "quote_count": 0,
                        "bookmark_count": 0
                      },
                      "quoted_status_result": {
                        "result": {
                          "__typename": "Tweet",
                          "rest_id": "6666",
                          "core": {
                            "user_results": {
                              "result": {
                                "legacy": {
                                  "name": "Original Author",
                                  "screen_name": "original"
                                }
                              }
                            }
                          },
                          "legacy": {
                            "full_text": "The original tweet being quoted.",
                            "created_at": "Tue Apr 08 12:00:00 +0000 2025",
                            "conversation_id_str": "6666",
                            "favorite_count": 50,
                            "retweet_count": 10,
                            "reply_count": 2,
                            "quote_count": 3,
                            "bookmark_count": 0
                          }
                        }
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
}`

// --- Tests ---

func TestParseTweetDetailResponse_CurrentFormat(t *testing.T) {
	tweets, err := ParseTweetDetailResponse(json.RawMessage(currentFormatFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tweets) != 1 {
		t.Fatalf("expected 1 tweet, got %d", len(tweets))
	}

	tw := tweets[0]
	if tw.ID != "1234567890" {
		t.Errorf("ID: got %q, want %q", tw.ID, "1234567890")
	}
	if tw.Author.Name != "Test User" {
		t.Errorf("Author.Name: got %q, want %q", tw.Author.Name, "Test User")
	}
	if tw.Author.ScreenName != "testuser" {
		t.Errorf("Author.ScreenName: got %q, want %q", tw.Author.ScreenName, "testuser")
	}
	if tw.Text != "Hello, world! This is a test tweet." {
		t.Errorf("Text: got %q", tw.Text)
	}
	if tw.CreatedAt != "Sun Apr 13 10:00:00 +0000 2025" {
		t.Errorf("CreatedAt: got %q", tw.CreatedAt)
	}
	if tw.ConversationID != "1234567890" {
		t.Errorf("ConversationID: got %q", tw.ConversationID)
	}
}

func TestParseTweetDetailResponse_CurrentFormat_Metrics(t *testing.T) {
	tweets, err := ParseTweetDetailResponse(json.RawMessage(currentFormatFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tw := tweets[0]

	expected := map[string]int{
		"favorite_count": 42,
		"retweet_count":  7,
		"reply_count":    3,
		"quote_count":    1,
		"bookmark_count": 2,
	}
	for k, want := range expected {
		got, ok := tw.Metrics[k]
		if !ok {
			t.Errorf("Metrics missing key %q", k)
			continue
		}
		if got != want {
			t.Errorf("Metrics[%q]: got %d, want %d", k, got, want)
		}
	}
}

func TestParseTweetDetailResponse_CurrentFormat_Media(t *testing.T) {
	tweets, err := ParseTweetDetailResponse(json.RawMessage(currentFormatFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tw := tweets[0]

	if len(tw.Media) != 1 {
		t.Fatalf("expected 1 media, got %d", len(tw.Media))
	}
	if tw.Media[0].Type != "photo" {
		t.Errorf("Media[0].Type: got %q, want %q", tw.Media[0].Type, "photo")
	}
	if tw.Media[0].URL != "https://pbs.twimg.com/media/test123.jpg" {
		t.Errorf("Media[0].URL: got %q", tw.Media[0].URL)
	}
}

func TestParseTweetDetailResponse_OldFormat(t *testing.T) {
	tweets, err := ParseTweetDetailResponse(json.RawMessage(oldFormatFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tweets) != 1 {
		t.Fatalf("expected 1 tweet, got %d", len(tweets))
	}

	tw := tweets[0]
	if tw.ID != "9999" {
		t.Errorf("ID: got %q, want %q", tw.ID, "9999")
	}
	if tw.Author.ScreenName != "oldformat" {
		t.Errorf("Author.ScreenName: got %q, want %q", tw.Author.ScreenName, "oldformat")
	}
	if tw.Text != "Tweet from old format." {
		t.Errorf("Text: got %q", tw.Text)
	}
}

func TestParseTweetDetailResponse_VisibilityResults(t *testing.T) {
	tweets, err := ParseTweetDetailResponse(json.RawMessage(visibilityResultsFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tweets) != 1 {
		t.Fatalf("expected 1 tweet, got %d", len(tweets))
	}

	tw := tweets[0]
	if tw.ID != "5555" {
		t.Errorf("ID: got %q, want %q", tw.ID, "5555")
	}
	if tw.Author.ScreenName != "visuser" {
		t.Errorf("Author.ScreenName: got %q, want %q", tw.Author.ScreenName, "visuser")
	}
	if tw.Text != "I'm wrapped in visibility results." {
		t.Errorf("Text: got %q", tw.Text)
	}
}

func TestParseTweetDetailResponse_NoteTweet(t *testing.T) {
	tweets, err := ParseTweetDetailResponse(json.RawMessage(noteTweetFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tweets) != 1 {
		t.Fatalf("expected 1 tweet, got %d", len(tweets))
	}

	tw := tweets[0]
	if tw.ID != "7777" {
		t.Errorf("ID: got %q, want %q", tw.ID, "7777")
	}
	if !tw.IsNoteTweet {
		t.Error("expected IsNoteTweet=true")
	}
	wantPrefix := "This is the full long-form note tweet content"
	if len(tw.Text) < len(wantPrefix) || tw.Text[:len(wantPrefix)] != wantPrefix {
		t.Errorf("note tweet text not applied, got: %q", tw.Text)
	}
	// Confirm it overrode the truncated legacy text.
	if tw.Text == "This is the short truncated text..." {
		t.Error("note tweet should override legacy full_text")
	}
}

func TestParseTweetDetailResponse_QuotedTweet(t *testing.T) {
	tweets, err := ParseTweetDetailResponse(json.RawMessage(quotedTweetFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tweets) != 1 {
		t.Fatalf("expected 1 tweet, got %d", len(tweets))
	}

	tw := tweets[0]
	if tw.ID != "8888" {
		t.Errorf("ID: got %q, want %q", tw.ID, "8888")
	}
	if tw.QuotedTweet == nil {
		t.Fatal("expected non-nil QuotedTweet")
	}

	qt := tw.QuotedTweet
	if qt.ID != "6666" {
		t.Errorf("QuotedTweet.ID: got %q, want %q", qt.ID, "6666")
	}
	if qt.Author.ScreenName != "original" {
		t.Errorf("QuotedTweet.Author.ScreenName: got %q, want %q", qt.Author.ScreenName, "original")
	}
	if qt.Text != "The original tweet being quoted." {
		t.Errorf("QuotedTweet.Text: got %q", qt.Text)
	}
}

func TestParseTweetDetailResponse_EmptyResponse(t *testing.T) {
	tweets, err := ParseTweetDetailResponse(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error for empty object: %v", err)
	}
	if tweets != nil {
		t.Errorf("expected nil tweets for empty response, got %d", len(tweets))
	}
}

func TestParseTweetDetailResponse_NoDataKey(t *testing.T) {
	tweets, err := ParseTweetDetailResponse(json.RawMessage(`{"errors": [{"message": "not found"}]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tweets != nil {
		t.Errorf("expected nil tweets when no data key, got %d", len(tweets))
	}
}

func TestParseTweetDetailResponse_NoEntries(t *testing.T) {
	raw := `{
		"data": {
			"threaded_conversation_with_injections_v2": {
				"instructions": [
					{
						"type": "TimelineAddEntries",
						"entries": []
					}
				]
			}
		}
	}`
	tweets, err := ParseTweetDetailResponse(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tweets) != 0 {
		t.Errorf("expected 0 tweets for empty entries, got %d", len(tweets))
	}
}

func TestParseTweetDetailResponse_InvalidJSON(t *testing.T) {
	_, err := ParseTweetDetailResponse(json.RawMessage(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// Test the conversation thread module format (multiple tweets in items[]).
func TestParseTweetDetailResponse_ThreadModule(t *testing.T) {
	raw := `{
		"data": {
			"threaded_conversation_with_injections_v2": {
				"instructions": [
					{
						"entries": [
							{
								"entryId": "conversationthread-111",
								"content": {
									"entryType": "TimelineTimelineModule",
									"items": [
										{
											"item": {
												"itemContent": {
													"tweet_results": {
														"result": {
															"__typename": "Tweet",
															"rest_id": "111",
															"core": {"user_results": {"result": {"legacy": {"name": "A", "screen_name": "a"}}}},
															"legacy": {"full_text": "First in thread", "favorite_count": 0, "retweet_count": 0, "reply_count": 0, "quote_count": 0, "bookmark_count": 0}
														}
													}
												}
											}
										},
										{
											"item": {
												"itemContent": {
													"tweet_results": {
														"result": {
															"__typename": "Tweet",
															"rest_id": "222",
															"core": {"user_results": {"result": {"legacy": {"name": "B", "screen_name": "b"}}}},
															"legacy": {"full_text": "Second in thread", "favorite_count": 0, "retweet_count": 0, "reply_count": 0, "quote_count": 0, "bookmark_count": 0}
														}
													}
												}
											}
										}
									]
								}
							}
						]
					}
				]
			}
		}
	}`
	tweets, err := ParseTweetDetailResponse(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tweets) != 2 {
		t.Fatalf("expected 2 tweets from thread module, got %d", len(tweets))
	}
	if tweets[0].ID != "111" {
		t.Errorf("first tweet ID: got %q, want %q", tweets[0].ID, "111")
	}
	if tweets[1].ID != "222" {
		t.Errorf("second tweet ID: got %q, want %q", tweets[1].ID, "222")
	}
}
