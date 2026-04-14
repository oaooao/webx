package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/oaooao/webx/internal/auth"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth <subcommand>",
	Short: "Manage platform credentials",
	Long:  "Add, list, remove, and test credentials for supported platforms.",
}

// --- auth add ---

var authAddCmd = &cobra.Command{
	Use:   "add <platform>",
	Short: "Interactively set up credentials for a platform",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		platform := strings.ToLower(args[0])
		store := auth.DefaultStore()

		switch platform {
		case "twitter":
			return authAddTwitter(store)
		case "reddit":
			return authAddReddit(store)
		default:
			return fmt.Errorf("unknown platform %q; supported: twitter, reddit", platform)
		}
	},
}

func authAddTwitter(store *auth.FileStore) error {
	fmt.Println("Setting up Twitter authentication.")
	fmt.Println()
	fmt.Println("You'll need auth_token and ct0 cookies from your browser.")
	fmt.Println("Open x.com → DevTools → Application → Cookies → copy the values.")
	fmt.Println()

	authToken := promptSecret("auth_token: ")
	ct0 := promptSecret("ct0: ")

	if authToken == "" || ct0 == "" {
		return fmt.Errorf("auth_token and ct0 are both required")
	}

	fmt.Print("Testing... ")
	username, err := verifyTwitter(authToken, ct0)
	if err != nil {
		fmt.Println("failed")
		return fmt.Errorf("credential test failed: %w", err)
	}
	fmt.Printf("authenticated as @%s\n", username)

	if err := store.Set("twitter", auth.PlatformAuth{
		Type: "cookie",
		Credentials: map[string]string{
			"auth_token": authToken,
			"ct0":        ct0,
		},
	}); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Printf("\nSaved to %s\n", store.Path())
	return nil
}

func authAddReddit(store *auth.FileStore) error {
	fmt.Println("Setting up Reddit authentication.")
	fmt.Println()
	fmt.Println("You'll need a Reddit OAuth access token.")
	fmt.Println("See https://github.com/oaooao/webx#reddit-setup for instructions.")
	fmt.Println()

	accessToken := promptSecret("access_token: ")

	if accessToken == "" {
		return fmt.Errorf("access_token is required")
	}

	fmt.Print("Testing... ")
	username, err := verifyReddit(accessToken)
	if err != nil {
		fmt.Println("failed")
		return fmt.Errorf("credential test failed: %w", err)
	}
	fmt.Printf("authenticated as u/%s\n", username)

	if err := store.Set("reddit", auth.PlatformAuth{
		Type: "oauth2",
		Credentials: map[string]string{
			"access_token": accessToken,
		},
	}); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Printf("\nSaved to %s\n", store.Path())
	return nil
}

// --- auth list ---

var authListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured platform credentials",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store := auth.DefaultStore()
		all, err := store.List()
		if err != nil {
			return fmt.Errorf("failed to list credentials: %w", err)
		}

		if len(all) == 0 {
			fmt.Println("No credentials configured. Run: webx auth add <platform>")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "Platform\tType\tAdded")
		for platform, pa := range all {
			added := pa.AddedAt
			if len(added) > 10 {
				added = added[:10] // trim to date
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", platform, pa.Type, added)
		}
		return w.Flush()
	},
}

// --- auth remove ---

var authRemoveCmd = &cobra.Command{
	Use:   "remove <platform>",
	Short: "Remove credentials for a platform",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		platform := strings.ToLower(args[0])
		store := auth.DefaultStore()

		existing, err := store.Get(platform)
		if err != nil || existing == nil {
			fmt.Fprintf(os.Stderr, "No credentials found for %q\n", platform)
			return nil
		}

		if err := store.Delete(platform); err != nil {
			return fmt.Errorf("failed to remove credentials: %w", err)
		}

		fmt.Printf("Removed credentials for %s\n", platform)
		return nil
	},
}

// --- auth test ---

var authTestCmd = &cobra.Command{
	Use:   "test <platform>",
	Short: "Test whether stored credentials are valid",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		platform := strings.ToLower(args[0])
		store := auth.DefaultStore()

		pa, err := store.Get(platform)
		if err != nil || pa == nil {
			// Also check env vars for fallback.
			switch platform {
			case "twitter":
				at := os.Getenv("TWITTER_AUTH_TOKEN")
				ct0 := os.Getenv("TWITTER_CT0")
				if at == "" || ct0 == "" {
					return fmt.Errorf("no credentials found for twitter. Run: webx auth add twitter")
				}
				pa = &auth.PlatformAuth{
					Credentials: map[string]string{"auth_token": at, "ct0": ct0},
				}
			case "reddit":
				token := os.Getenv("REDDIT_ACCESS_TOKEN")
				if token == "" {
					return fmt.Errorf("no credentials found for reddit. Run: webx auth add reddit")
				}
				pa = &auth.PlatformAuth{
					Credentials: map[string]string{"access_token": token},
				}
			default:
				return fmt.Errorf("no credentials found for %q. Run: webx auth add %s", platform, platform)
			}
		}

		fmt.Printf("Testing %s credentials... ", platform)

		switch platform {
		case "twitter":
			username, err := verifyTwitter(pa.Credentials["auth_token"], pa.Credentials["ct0"])
			if err != nil {
				fmt.Println("FAILED")
				return fmt.Errorf("invalid: %w", err)
			}
			fmt.Printf("OK (authenticated as @%s)\n", username)
		case "reddit":
			username, err := verifyReddit(pa.Credentials["access_token"])
			if err != nil {
				fmt.Println("FAILED")
				return fmt.Errorf("invalid: %w", err)
			}
			fmt.Printf("OK (authenticated as u/%s)\n", username)
		default:
			return fmt.Errorf("test not implemented for platform %q", platform)
		}

		return nil
	},
}

// --- Verification helpers ---

// verifyTwitter calls GET /1.1/account/verify_credentials.json and returns the screen_name.
func verifyTwitter(authToken, ct0 string) (string, error) {
	const verifyURL = "https://api.twitter.com/1.1/account/verify_credentials.json"
	const bearerToken = "AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA"

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, verifyURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Cookie", "auth_token="+authToken+"; ct0="+ct0)
	req.Header.Set("X-Csrf-Token", ct0)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("HTTP %d: credentials rejected", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: unexpected status", resp.StatusCode)
	}

	var result struct {
		ScreenName string `json:"screen_name"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	if result.ScreenName == "" {
		return "", fmt.Errorf("no screen_name in response")
	}

	return result.ScreenName, nil
}

// verifyReddit calls GET https://oauth.reddit.com/api/v1/me and returns the username.
func verifyReddit(accessToken string) (string, error) {
	const meURL = "https://oauth.reddit.com/api/v1/me"

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, meURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", "webx/0.4 by github.com/oaooao/webx")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("HTTP %d: credentials rejected", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: unexpected status", resp.StatusCode)
	}

	var result struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	if result.Name == "" {
		return "", fmt.Errorf("no name in response")
	}

	return result.Name, nil
}

// promptSecret reads a line from stdin, masking is not implemented (terminal masking
// requires platform-specific syscalls; for simplicity we just read plaintext).
func promptSecret(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

func init() {
	authCmd.AddCommand(authAddCmd)
	authCmd.AddCommand(authListCmd)
	authCmd.AddCommand(authRemoveCmd)
	authCmd.AddCommand(authTestCmd)
}
