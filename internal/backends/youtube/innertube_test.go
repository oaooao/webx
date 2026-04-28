package youtube

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// innerTubeBodyWithCaptions builds an InnerTube /player response that
// carries one English caption track.
func innerTubeBodyWithCaptions(t *testing.T, baseURL string) string {
	t.Helper()
	body := map[string]any{
		"captions": map[string]any{
			"playerCaptionsTracklistRenderer": map[string]any{
				"captionTracks": []map[string]any{
					{"baseUrl": baseURL, "languageCode": "en", "kind": "asr"},
				},
			},
		},
	}
	out, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(out)
}

func TestTryInnerTubeClient_ReturnsBaseURLWhenServerCarriesCaptions(t *testing.T) {
	wantBaseURL := "https://www.youtube.com/api/timedtext?v=abc&kind=asr&lang=en"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"videoId":"abc"`) {
			t.Errorf("request body missing videoId: %s", body)
		}
		if !strings.Contains(string(body), `"clientName":"IOS"`) {
			t.Errorf("request body missing IOS client: %s", body)
		}
		_, _ = w.Write([]byte(innerTubeBodyWithCaptions(t, wantBaseURL)))
	}))
	defer srv.Close()

	original := innerTubePlayerURL
	innerTubePlayerURL = srv.URL
	defer func() { innerTubePlayerURL = original }()

	got := tryInnerTubeClient("abc", innerTubeClients[0])
	if got != wantBaseURL {
		t.Errorf("tryInnerTubeClient = %q, want %q", got, wantBaseURL)
	}
}

func TestTryInnerTubeClient_AndroidSendsUserAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if !strings.HasPrefix(ua, "com.google.android.youtube/") {
			t.Errorf("User-Agent = %q, want Android youtube prefix", ua)
		}
		_, _ = w.Write([]byte(innerTubeBodyWithCaptions(t, "https://example.com/cap")))
	}))
	defer srv.Close()

	original := innerTubePlayerURL
	innerTubePlayerURL = srv.URL
	defer func() { innerTubePlayerURL = original }()

	var android innerTubeClient
	for _, c := range innerTubeClients {
		if c.name == "ANDROID" {
			android = c
			break
		}
	}
	if android.name == "" {
		t.Fatal("ANDROID client not found in innerTubeClients")
	}

	if got := tryInnerTubeClient("abc", android); got == "" {
		t.Error("expected non-empty caption URL from Android client")
	}
}

func TestFetchInnerTubeCaptionURL_FallsThroughEmptyClients(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		switch n {
		case 1:
			// iOS: return malformed JSON to simulate failure
			_, _ = w.Write([]byte(`not json`))
		case 2:
			// Android: return valid JSON but empty captionTracks
			if !strings.Contains(string(body), `"clientName":"ANDROID"`) {
				t.Errorf("call 2 not ANDROID: %s", body)
			}
			_, _ = w.Write([]byte(`{"captions":{"playerCaptionsTracklistRenderer":{"captionTracks":[]}}}`))
		case 3:
			// WEB: succeed
			if !strings.Contains(string(body), `"clientName":"WEB"`) {
				t.Errorf("call 3 not WEB: %s", body)
			}
			_, _ = w.Write([]byte(innerTubeBodyWithCaptions(t, "https://example.com/web-cap")))
		default:
			t.Errorf("unexpected 4th call")
		}
	}))
	defer srv.Close()

	original := innerTubePlayerURL
	innerTubePlayerURL = srv.URL
	defer func() { innerTubePlayerURL = original }()

	got := fetchInnerTubeCaptionURL("abc")
	if got != "https://example.com/web-cap" {
		t.Errorf("got %q, want web-cap URL", got)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls (iOS+Android+WEB), got %d", calls.Load())
	}
}

func TestFetchInnerTubeCaptionURL_AllClientsFailReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	original := innerTubePlayerURL
	innerTubePlayerURL = srv.URL
	defer func() { innerTubePlayerURL = original }()

	if got := fetchInnerTubeCaptionURL("abc"); got != "" {
		t.Errorf("expected empty on all-fail, got %q", got)
	}
}
