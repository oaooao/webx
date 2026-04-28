package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oaooao/webx/internal/backends"
	"github.com/oaooao/webx/internal/types"
)

var videoIDPatterns = []*regexp.Regexp{
	regexp.MustCompile(`[?&]v=([a-zA-Z0-9_-]{11})`),
	regexp.MustCompile(`youtu\.be/([a-zA-Z0-9_-]{11})`),
	regexp.MustCompile(`/shorts/([a-zA-Z0-9_-]{11})`),
}

var playerResponseRe = regexp.MustCompile(`var\s+ytInitialPlayerResponse\s*=\s*`)

type VideoMeta struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Channel     string `json:"channel"`
	Description string `json:"description,omitempty"`
	ViewCount   int64  `json:"view_count,omitempty"`
	PublishDate string `json:"publish_date,omitempty"`
	Duration    string `json:"duration,omitempty"`
}

type TranscriptSegment struct {
	Start    float64 `json:"start"`
	Duration float64 `json:"duration"`
	Text     string  `json:"text"`
}

type FetchResult struct {
	Video      VideoMeta
	Transcript []TranscriptSegment
}

// ExtractVideoID extracts the 11-char video ID from a YouTube URL string.
func ExtractVideoID(rawURL string) string {
	for _, re := range videoIDPatterns {
		m := re.FindStringSubmatch(rawURL)
		if len(m) >= 2 {
			return m[1]
		}
	}
	return ""
}

// FetchVideo fetches video metadata and transcript from a YouTube URL.
//
// Metadata comes from the watch page's ytInitialPlayerResponse (always
// available without auth). Captions are fetched via the InnerTube /player
// API with a 3-client fallback (iOS → Android → Web): YouTube no longer
// returns content from the web caption track URLs, but InnerTube tracks
// carry the right tokens. See fetchInnerTubeCaptionURL for details.
//
// Reference: kepano/defuddle src/extractors/youtube.ts (MIT).
func FetchVideo(rawURL string) (*FetchResult, error) {
	videoID := ExtractVideoID(rawURL)
	if videoID == "" {
		return nil, types.NewWebxError(types.ErrNoMatch, "could not extract video ID from URL")
	}

	watchURL := "https://www.youtube.com/watch?v=" + videoID
	pageHTML, err := backends.FetchHTMLStd(watchURL)
	if err != nil {
		return nil, err
	}

	playerJSON, err := extractPlayerResponse(pageHTML)
	if err != nil {
		return nil, err
	}

	video, _, err := parsePlayerResponse(playerJSON, videoID)
	if err != nil {
		return nil, err
	}

	result := &FetchResult{Video: *video}

	// Web-page captionTracks[].baseUrl returns empty body since YouTube
	// gated /api/timedtext to authed clients. Use InnerTube instead.
	if captionURL := fetchInnerTubeCaptionURL(videoID); captionURL != "" {
		if segments, err := fetchTranscript(captionURL); err == nil {
			result.Transcript = segments
		}
		// non-fatal: no transcript is acceptable
	}

	return result, nil
}

// InnerTube client identifiers. YouTube updates these periodically; values
// sourced from kepano/defuddle. iOS is tried first because it doesn't
// require a special User-Agent (works in any context). If a client returns
// no caption tracks, we fall through to the next.
const (
	innerTubeAndroidUA    = "com.google.android.youtube/20.10.38 (Linux; U; Android 14)"
	innerTubeFetchTimeout = 8 * time.Second
)

// innerTubePlayerURL is a var (not const) so tests can point it at httptest.
var innerTubePlayerURL = "https://www.youtube.com/youtubei/v1/player?prettyPrint=false"

type innerTubeClient struct {
	name      string
	body      string
	userAgent string // optional; only Android requires it
}

var innerTubeClients = []innerTubeClient{
	{
		name: "IOS",
		body: `{"context":{"client":{"clientName":"IOS","clientVersion":"20.10.3"}},"videoId":"%s"}`,
	},
	{
		name:      "ANDROID",
		body:      `{"context":{"client":{"clientName":"ANDROID","clientVersion":"20.10.38"}},"videoId":"%s"}`,
		userAgent: innerTubeAndroidUA,
	},
	{
		name: "WEB",
		body: `{"context":{"client":{"clientName":"WEB","clientVersion":"2.20240101.00.00"}},"videoId":"%s"}`,
	},
}

// fetchInnerTubeCaptionURL queries the InnerTube /player API across iOS,
// Android, and Web clients in order. Returns the first non-empty caption
// track baseUrl found. Returns "" if no client yields captions or all
// requests fail (caller treats absence as non-fatal).
func fetchInnerTubeCaptionURL(videoID string) string {
	for _, c := range innerTubeClients {
		captionURL := tryInnerTubeClient(videoID, c)
		if captionURL != "" {
			return captionURL
		}
	}
	return ""
}

func tryInnerTubeClient(videoID string, client innerTubeClient) string {
	ctx, cancel := context.WithTimeout(context.Background(), innerTubeFetchTimeout)
	defer cancel()

	body := fmt.Sprintf(client.body, videoID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, innerTubePlayerURL, bytes.NewBufferString(body))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	if client.userAgent != "" {
		req.Header.Set("User-Agent", client.userAgent)
	}

	resp, err := backends.StdClient().Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return ""
	}

	var pr playerResponse
	if err := json.Unmarshal(data, &pr); err != nil {
		return ""
	}
	return pickCaptionURL(pr.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks)
}

func extractPlayerResponse(pageHTML string) ([]byte, error) {
	loc := playerResponseRe.FindStringIndex(pageHTML)
	if loc == nil {
		return nil, types.NewWebxError(types.ErrBackendFailed, "ytInitialPlayerResponse not found in page HTML")
	}

	start := loc[1] // right after "var ytInitialPlayerResponse = "
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
				break
			}
		}
		if end > 0 {
			break
		}
	}

	if end <= start {
		return nil, types.NewWebxError(types.ErrBackendFailed, "failed to extract ytInitialPlayerResponse JSON")
	}

	return []byte(pageHTML[start:end]), nil
}

// playerResponse is a minimal representation of ytInitialPlayerResponse.
type playerResponse struct {
	VideoDetails struct {
		VideoID          string `json:"videoId"`
		Title            string `json:"title"`
		Author           string `json:"author"`
		ShortDescription string `json:"shortDescription"`
		ViewCount        string `json:"viewCount"`
		LengthSeconds    string `json:"lengthSeconds"`
	} `json:"videoDetails"`
	Microformat struct {
		PlayerMicroformatRenderer struct {
			PublishDate string `json:"publishDate"`
		} `json:"playerMicroformatRenderer"`
	} `json:"microformat"`
	Captions struct {
		PlayerCaptionsTracklistRenderer struct {
			CaptionTracks []struct {
				BaseURL      string `json:"baseUrl"`
				LanguageCode string `json:"languageCode"`
				Kind         string `json:"kind"`
			} `json:"captionTracks"`
		} `json:"playerCaptionsTracklistRenderer"`
	} `json:"captions"`
}

func parsePlayerResponse(data []byte, videoID string) (*VideoMeta, string, error) {
	var pr playerResponse
	if err := json.Unmarshal(data, &pr); err != nil {
		return nil, "", types.NewWebxError(types.ErrBackendFailed, "failed to parse ytInitialPlayerResponse: "+err.Error())
	}

	vd := pr.VideoDetails
	viewCount, _ := strconv.ParseInt(vd.ViewCount, 10, 64)
	lengthSec, _ := strconv.ParseInt(vd.LengthSeconds, 10, 64)

	video := &VideoMeta{
		ID:          videoID,
		Title:       vd.Title,
		Channel:     vd.Author,
		Description: vd.ShortDescription,
		ViewCount:   viewCount,
		PublishDate: pr.Microformat.PlayerMicroformatRenderer.PublishDate,
		Duration:    formatDuration(lengthSec),
	}

	captionURL := pickCaptionURL(pr.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks)

	return video, captionURL, nil
}

type captionTrack struct {
	BaseURL      string
	LanguageCode string
	Kind         string
}

func pickCaptionURL(tracks []struct {
	BaseURL      string `json:"baseUrl"`
	LanguageCode string `json:"languageCode"`
	Kind         string `json:"kind"`
}) string {
	if len(tracks) == 0 {
		return ""
	}

	// Prefer English, then first available
	for _, t := range tracks {
		if t.LanguageCode == "en" {
			return t.BaseURL
		}
	}
	// Try en- variants (en-US, en-GB, etc.)
	for _, t := range tracks {
		if strings.HasPrefix(t.LanguageCode, "en") {
			return t.BaseURL
		}
	}
	return tracks[0].BaseURL
}

// transcriptXML represents the YouTube transcript XML format.
type transcriptXML struct {
	XMLName xml.Name      `xml:"transcript"`
	Texts   []xmlTextNode `xml:"text"`
}

type xmlTextNode struct {
	Start string `xml:"start,attr"`
	Dur   string `xml:"dur,attr"`
	Text  string `xml:",chardata"`
}

func fetchTranscript(captionURL string) ([]TranscriptSegment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, captionURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("transcript HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, err
	}

	var transcript transcriptXML
	if err := xml.Unmarshal(body, &transcript); err != nil {
		return nil, err
	}

	segments := make([]TranscriptSegment, 0, len(transcript.Texts))
	for _, t := range transcript.Texts {
		start, _ := strconv.ParseFloat(t.Start, 64)
		dur, _ := strconv.ParseFloat(t.Dur, 64)
		text := html.UnescapeString(t.Text)
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		segments = append(segments, TranscriptSegment{
			Start:    start,
			Duration: dur,
			Text:     text,
		})
	}

	return segments, nil
}

func formatDuration(totalSeconds int64) string {
	if totalSeconds <= 0 {
		return ""
	}
	h := totalSeconds / 3600
	m := (totalSeconds % 3600) / 60
	s := totalSeconds % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
