package twitter

import "encoding/json"

// Tweet is the parsed domain model for a single tweet.
type Tweet struct {
	ID             string         `json:"id"`
	Text           string         `json:"text"`
	Author         Author         `json:"author"`
	CreatedAt      string         `json:"created_at"`
	Metrics        map[string]int `json:"metrics,omitempty"`
	QuotedTweet    *Tweet         `json:"quoted_tweet,omitempty"`
	Media          []Media        `json:"media,omitempty"`
	IsNoteTweet      bool           `json:"is_note_tweet"`
	ConversationID   string         `json:"conversation_id"`
	InReplyToID      string         `json:"in_reply_to_id,omitempty"`
	InReplyToUser    string         `json:"in_reply_to_user,omitempty"`
}

// Author holds the tweet author's display name and handle.
type Author struct {
	Name       string `json:"name"`
	ScreenName string `json:"screen_name"`
}

// Media represents a photo, video, or animated GIF attached to a tweet.
type Media struct {
	Type string `json:"type"` // "photo", "video", "animated_gif"
	URL  string `json:"url"`
}

// ParseTweetDetailResponse extracts tweets from the deeply nested TweetDetail
// GraphQL response. The JSON path is:
//
//	data → tweetDetailV2 (or tweet_detail)
//	  → threaded_conversation_with_injections_v2
//	    → instructions[] → entries[]
//	      → content → itemContent → tweet_results → result
//
// Returns nil, nil when the structure is valid but contains no tweet entries.
func ParseTweetDetailResponse(raw json.RawMessage) ([]Tweet, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, err
	}

	dataRaw, ok := top["data"]
	if !ok {
		return nil, nil
	}

	var data map[string]json.RawMessage
	if err := json.Unmarshal(dataRaw, &data); err != nil {
		return nil, err
	}

	// Twitter API has used multiple response shapes across versions:
	// 1. data.threaded_conversation_with_injections_v2 (current, direct)
	// 2. data.tweetDetailV2.threaded_conversation_with_injections_v2 (older)
	// 3. data.tweet_detail.threaded_conversation_with_injections_v2 (older)
	var threadedRaw json.RawMessage
	if tc, exists := data["threaded_conversation_with_injections_v2"]; exists {
		threadedRaw = tc
	} else {
		for _, key := range []string{"tweetDetailV2", "tweet_detail"} {
			v, exists := data[key]
			if !exists {
				continue
			}
			var inner map[string]json.RawMessage
			if json.Unmarshal(v, &inner) != nil {
				continue
			}
			if tc, exists := inner["threaded_conversation_with_injections_v2"]; exists {
				threadedRaw = tc
				break
			}
		}
	}
	if threadedRaw == nil {
		return nil, nil
	}

	var threaded map[string]json.RawMessage
	if err := json.Unmarshal(threadedRaw, &threaded); err != nil {
		return nil, err
	}

	instructionsRaw, ok := threaded["instructions"]
	if !ok {
		return nil, nil
	}

	var instructions []json.RawMessage
	if err := json.Unmarshal(instructionsRaw, &instructions); err != nil {
		return nil, err
	}

	var tweets []Tweet
	for _, instrRaw := range instructions {
		var instr map[string]json.RawMessage
		if json.Unmarshal(instrRaw, &instr) != nil {
			continue
		}
		entriesRaw, ok := instr["entries"]
		if !ok {
			continue
		}
		var entries []json.RawMessage
		if json.Unmarshal(entriesRaw, &entries) != nil {
			continue
		}
		for _, entryRaw := range entries {
			tweets = append(tweets, extractTweetsFromEntry(entryRaw)...)
		}
	}

	return tweets, nil
}

func extractTweetsFromEntry(entryRaw json.RawMessage) []Tweet {
	var entry map[string]json.RawMessage
	if json.Unmarshal(entryRaw, &entry) != nil {
		return nil
	}

	contentRaw, ok := entry["content"]
	if !ok {
		return nil
	}

	var content map[string]json.RawMessage
	if json.Unmarshal(contentRaw, &content) != nil {
		return nil
	}

	// TimelineTimelineItem — a single tweet entry.
	if itemContentRaw, ok := content["itemContent"]; ok {
		if t := extractTweetFromItemContent(itemContentRaw); t != nil {
			return []Tweet{*t}
		}
	}

	// TimelineTimelineModule — a conversation thread module with multiple items.
	if itemsRaw, ok := content["items"]; ok {
		var items []json.RawMessage
		if json.Unmarshal(itemsRaw, &items) != nil {
			return nil
		}
		var tweets []Tweet
		for _, itemRaw := range items {
			var item map[string]json.RawMessage
			if json.Unmarshal(itemRaw, &item) != nil {
				continue
			}
			innerRaw, ok := item["item"]
			if !ok {
				continue
			}
			var inner map[string]json.RawMessage
			if json.Unmarshal(innerRaw, &inner) != nil {
				continue
			}
			ic, ok := inner["itemContent"]
			if !ok {
				continue
			}
			if t := extractTweetFromItemContent(ic); t != nil {
				tweets = append(tweets, *t)
			}
		}
		return tweets
	}

	return nil
}

func extractTweetFromItemContent(itemContentRaw json.RawMessage) *Tweet {
	var itemContent map[string]json.RawMessage
	if json.Unmarshal(itemContentRaw, &itemContent) != nil {
		return nil
	}

	tweetResultsRaw, ok := itemContent["tweet_results"]
	if !ok {
		return nil
	}

	var tweetResults map[string]json.RawMessage
	if json.Unmarshal(tweetResultsRaw, &tweetResults) != nil {
		return nil
	}

	resultRaw, ok := tweetResults["result"]
	if !ok {
		return nil
	}

	return parseTweetResult(resultRaw)
}

// parseTweetResult navigates the result node which may be wrapped in
// TweetWithVisibilityResults before reaching the actual tweet payload.
func parseTweetResult(resultRaw json.RawMessage) *Tweet {
	var result map[string]json.RawMessage
	if json.Unmarshal(resultRaw, &result) != nil {
		return nil
	}

	// Unwrap TweetWithVisibilityResults envelope.
	var typeName string
	if tn, ok := result["__typename"]; ok {
		_ = json.Unmarshal(tn, &typeName)
	}
	if typeName == "TweetWithVisibilityResults" {
		inner, ok := result["tweet"]
		if !ok {
			return nil
		}
		return parseTweetResult(inner)
	}

	tweet := &Tweet{}

	// rest_id — the numeric tweet ID as a string.
	if v, ok := result["rest_id"]; ok {
		_ = json.Unmarshal(v, &tweet.ID)
	}

	// Author: core → user_results → result → legacy.
	if coreRaw, ok := result["core"]; ok {
		var core map[string]json.RawMessage
		if json.Unmarshal(coreRaw, &core) == nil {
			if urRaw, ok := core["user_results"]; ok {
				var ur map[string]json.RawMessage
				if json.Unmarshal(urRaw, &ur) == nil {
					if urResultRaw, ok := ur["result"]; ok {
						var urResult map[string]json.RawMessage
						if json.Unmarshal(urResultRaw, &urResult) == nil {
							// Try both new "core" sub-object and legacy flat layout.
							name, screenName := extractUserNameFields(urResult)
							tweet.Author = Author{Name: name, ScreenName: screenName}
						}
					}
				}
			}
		}
	}

	// Tweet content from legacy.
	if legacyRaw, ok := result["legacy"]; ok {
		var legacy map[string]json.RawMessage
		if json.Unmarshal(legacyRaw, &legacy) == nil {
			parseLegacyIntoTweet(legacy, tweet)
		}
	}

	// Note tweets (long-form) override the legacy full_text.
	if noteRaw, ok := result["note_tweet"]; ok {
		var note map[string]json.RawMessage
		if json.Unmarshal(noteRaw, &note) == nil {
			if nrRaw, ok := note["note_tweet_results"]; ok {
				var nr map[string]json.RawMessage
				if json.Unmarshal(nrRaw, &nr) == nil {
					if nrResultRaw, ok := nr["result"]; ok {
						var nrResult map[string]json.RawMessage
						if json.Unmarshal(nrResultRaw, &nrResult) == nil {
							if textRaw, ok := nrResult["text"]; ok {
								var noteText string
								if json.Unmarshal(textRaw, &noteText) == nil && noteText != "" {
									tweet.Text = noteText
									tweet.IsNoteTweet = true
								}
							}
						}
					}
				}
			}
		}
	}

	// Quoted tweet — recursively parsed.
	if qsrRaw, ok := result["quoted_status_result"]; ok {
		var qsr map[string]json.RawMessage
		if json.Unmarshal(qsrRaw, &qsr) == nil {
			if qResultRaw, ok := qsr["result"]; ok {
				tweet.QuotedTweet = parseTweetResult(qResultRaw)
			}
		}
	}

	if tweet.ID == "" {
		return nil
	}
	return tweet
}

// extractUserNameFields returns (name, screen_name) from a user result node,
// handling both the old flat `legacy` layout and the newer `core` sub-object.
func extractUserNameFields(userResult map[string]json.RawMessage) (string, string) {
	// New layout: userResult.core.{name,screen_name}
	if coreRaw, ok := userResult["core"]; ok {
		var core map[string]json.RawMessage
		if json.Unmarshal(coreRaw, &core) == nil {
			name := jsonString(core["name"])
			screenName := jsonString(core["screen_name"])
			if name != "" || screenName != "" {
				return name, screenName
			}
		}
	}
	// Old layout: userResult.legacy.{name,screen_name}
	if legacyRaw, ok := userResult["legacy"]; ok {
		var legacy map[string]json.RawMessage
		if json.Unmarshal(legacyRaw, &legacy) == nil {
			return jsonString(legacy["name"]), jsonString(legacy["screen_name"])
		}
	}
	return "", ""
}

func parseLegacyIntoTweet(legacy map[string]json.RawMessage, tweet *Tweet) {
	tweet.Text = jsonString(legacy["full_text"])
	tweet.CreatedAt = jsonString(legacy["created_at"])
	tweet.ConversationID = jsonString(legacy["conversation_id_str"])
	tweet.InReplyToID = jsonString(legacy["in_reply_to_status_id_str"])
	tweet.InReplyToUser = jsonString(legacy["in_reply_to_screen_name"])

	tweet.Metrics = make(map[string]int)
	for _, key := range []string{"favorite_count", "retweet_count", "reply_count", "quote_count", "bookmark_count"} {
		if v, ok := legacy[key]; ok {
			var n int
			if json.Unmarshal(v, &n) == nil && n > 0 {
				tweet.Metrics[key] = n
			}
		}
	}

	// Media from extended_entities (higher fidelity than entities).
	if eeRaw, ok := legacy["extended_entities"]; ok {
		var ee map[string]json.RawMessage
		if json.Unmarshal(eeRaw, &ee) == nil {
			if mediaRaw, ok := ee["media"]; ok {
				var mediaList []map[string]json.RawMessage
				if json.Unmarshal(mediaRaw, &mediaList) == nil {
					for _, m := range mediaList {
						media := Media{
							Type: jsonString(m["type"]),
							URL:  jsonString(m["media_url_https"]),
						}
						if media.URL != "" {
							tweet.Media = append(tweet.Media, media)
						}
					}
				}
			}
		}
	}
}

// jsonString safely unmarshals a json.RawMessage into a Go string.
func jsonString(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var s string
	_ = json.Unmarshal(raw, &s)
	return s
}
