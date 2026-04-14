package youtube

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"

	"github.com/oaooao/webx/internal/backends"
	"github.com/oaooao/webx/internal/types"
)

var ytInitialDataRe = regexp.MustCompile(`var\s+ytInitialData\s*=\s*`)

// SearchVideos fetches the YouTube search results page and parses ytInitialData.
// apiURL should be the full URL (e.g. "https://www.youtube.com/results?search_query=golang").
// In tests, a test server URL is passed directly; in production, use BuildYouTubeSearchURL.
func SearchVideos(apiURL string, limit int) (*types.NormalizedSearchResult, error) {
	// FetchHTMLStd handles 5xx errors by returning an error.
	// For search pages, std client is sufficient (no TLS fingerprinting needed).
	pageHTML, err := backends.FetchHTMLStd(apiURL)
	if err != nil {
		return nil, err
	}

	initialData, err := extractYTInitialData(pageHTML)
	if err != nil {
		return nil, err
	}

	items, err := parseYTSearchResults(initialData, limit)
	if err != nil {
		return nil, err
	}

	return &types.NormalizedSearchResult{
		Items:   items,
		Query:   "",
		Backend: "youtube_scrape",
		HasMore: limit > 0 && len(items) >= limit,
	}, nil
}

// BuildYouTubeSearchURL constructs the YouTube search results URL.
func BuildYouTubeSearchURL(query string) string {
	return "https://www.youtube.com/results?search_query=" + url.QueryEscape(query)
}

func extractYTInitialData(pageHTML string) ([]byte, error) {
	loc := ytInitialDataRe.FindStringIndex(pageHTML)
	if loc == nil {
		return nil, types.NewWebxError(types.ErrBackendFailed, "ytInitialData not found in YouTube search page")
	}

	start := loc[1]
	depth := 0
	inString := false
	escaped := false
	end := -1
	for i := start; i < len(pageHTML); i++ {
		ch := pageHTML[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
			}
		}
		if end > 0 {
			break
		}
	}

	if end <= start {
		return nil, types.NewWebxError(types.ErrBackendFailed, "failed to extract ytInitialData JSON")
	}

	return []byte(pageHTML[start:end]), nil
}

// ytSearchResult is a minimal representation of the ytInitialData search structure.
type ytSearchResult struct {
	Contents struct {
		TwoColumnSearchResultsRenderer struct {
			PrimaryContents struct {
				SectionListRenderer struct {
					Contents []struct {
						ItemSectionRenderer struct {
							Contents []ytSearchItem `json:"contents"`
						} `json:"itemSectionRenderer"`
					} `json:"contents"`
				} `json:"sectionListRenderer"`
			} `json:"primaryContents"`
		} `json:"twoColumnSearchResultsRenderer"`
	} `json:"contents"`
}

type ytSearchItem struct {
	VideoRenderer *ytVideoRenderer `json:"videoRenderer"`
}

type ytVideoRenderer struct {
	VideoID string `json:"videoId"`
	Title   struct {
		Runs []ytRun `json:"runs"`
	} `json:"title"`
	OwnerText struct {
		Runs []ytRun `json:"runs"`
	} `json:"ownerText"`
	ViewCountText struct {
		SimpleText string `json:"simpleText"`
	} `json:"viewCountText"`
	DescriptionSnippet struct {
		Runs []ytRun `json:"runs"`
	} `json:"descriptionSnippet"`
	PublishedTimeText struct {
		SimpleText string `json:"simpleText"`
	} `json:"publishedTimeText"`
}

type ytRun struct {
	Text string `json:"text"`
}

func parseYTSearchResults(data []byte, limit int) ([]types.SearchResultItem, error) {
	var result ytSearchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, types.NewWebxError(types.ErrBackendFailed, "failed to parse ytInitialData: "+err.Error())
	}

	var items []types.SearchResultItem
	sections := result.Contents.TwoColumnSearchResultsRenderer.PrimaryContents.SectionListRenderer.Contents
	for _, section := range sections {
		for _, entry := range section.ItemSectionRenderer.Contents {
			if entry.VideoRenderer == nil {
				continue
			}
			vr := entry.VideoRenderer
			if vr.VideoID == "" {
				continue
			}

			title := joinRuns(vr.Title.Runs)
			channel := joinRuns(vr.OwnerText.Runs)
			snippet := joinRuns(vr.DescriptionSnippet.Runs)
			videoURL := "https://www.youtube.com/watch?v=" + vr.VideoID
			viewCount := vr.ViewCountText.SimpleText
			publishedAt := vr.PublishedTimeText.SimpleText

			items = append(items, types.SearchResultItem{
				Title:   title,
				URL:     videoURL,
				Snippet: snippet,
				Author:  channel,
				Date:    publishedAt,
				Kind:    types.KindVideo,
				Meta: map[string]any{
					"view_count": viewCount,
					"video_id":   vr.VideoID,
				},
			})

			if limit > 0 && len(items) >= limit {
				return items, nil
			}
		}
	}

	return items, nil
}

func joinRuns(runs []ytRun) string {
	var parts []string
	for _, r := range runs {
		parts = append(parts, r.Text)
	}
	return strings.Join(parts, "")
}
