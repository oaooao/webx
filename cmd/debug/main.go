package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"

	fhttp "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

func main() {
	jar := tlsclient.NewCookieJar()
	client, _ := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(),
		tlsclient.WithClientProfile(profiles.Chrome_133),
		tlsclient.WithCookieJar(jar),
		tlsclient.WithTimeoutSeconds(30),
	)

	// Seed
	req, _ := fhttp.NewRequest("GET", "https://x.com", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	resp, _ := client.Do(req)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	fmt.Printf("Seed: HTTP %d\n", resp.StatusCode)

	u, _ := url.Parse("https://x.com")
	fmt.Printf("Cookies: %d\n", len(client.GetCookies(u)))

	// Add auth cookies
	at := os.Getenv("TWITTER_AUTH_TOKEN")
	ct0 := os.Getenv("TWITTER_CT0")
	client.SetCookies(u, []*fhttp.Cookie{
		{Name: "auth_token", Value: at},
		{Name: "ct0", Value: ct0},
	})
	fmt.Printf("Cookies after auth: %d\n", len(client.GetCookies(u)))

	// Build search URL (same as twitter-cli)
	qid := "MJpyQGqgklrVl_0X9gNy3A"
	variables, _ := json.Marshal(map[string]any{"count": 7, "rawQuery": "hello", "querySource": "typed_query", "product": "Top"})
	features, _ := json.Marshal(map[string]bool{
		"responsive_web_graphql_exclude_directive_enabled": true, "creator_subscriptions_tweet_preview_api_enabled": true,
		"responsive_web_graphql_timeline_navigation_enabled": true, "c9s_tweet_anatomy_moderator_badge_enabled": true,
		"tweetypie_unmention_optimization_enabled": true, "responsive_web_edit_tweet_api_enabled": true,
		"graphql_is_translatable_rweb_tweet_is_translatable_enabled": true, "view_counts_everywhere_api_enabled": true,
		"longform_notetweets_consumption_enabled": true, "responsive_web_twitter_article_tweet_consumption_enabled": true,
		"longform_notetweets_rich_text_read_enabled": true, "rweb_video_timestamps_enabled": true,
		"responsive_web_media_download_video_enabled": true, "freedom_of_speech_not_reach_fetch_enabled": true,
		"standardized_nudges_misinfo": true,
	})
	searchURL := fmt.Sprintf("https://x.com/i/api/graphql/%s/SearchTimeline?variables=%s&features=%s",
		qid, url.QueryEscape(string(variables)), url.QueryEscape(string(features)))

	req2, _ := fhttp.NewRequest("GET", searchURL, nil)
	req2.Header.Set("Authorization", "Bearer AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA")
	req2.Header.Set("X-Csrf-Token", ct0)
	req2.Header.Set("X-Twitter-Active-User", "yes")
	req2.Header.Set("X-Twitter-Auth-Type", "OAuth2Session")
	req2.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")

	resp2, err := client.Do(req2)
	if err != nil {
		fmt.Println("Search error:", err)
		return
	}
	body, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	fmt.Printf("Search: HTTP %d, body=%d bytes\n", resp2.StatusCode, len(body))
	if resp2.StatusCode == 200 && len(body) > 0 {
		fmt.Println(string(body)[:min(200, len(body))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
