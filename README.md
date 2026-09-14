# webx

URL in, content out. webx is a command-line reader for AI agents and scripts: give it a URL, get clean Markdown plus structured JSON in one consistent envelope. Public pages need zero configuration. Logged-in sites (X/Twitter, Reddit) reuse the login state of your own [ego lite](https://lite.ego.app/) browser, so there are no tokens to export and no API keys to buy.

```sh
webx read 'https://x.com/karpathy/status/1886192184808149383'
```

```json
{
  "ok": true,
  "kind": "thread",
  "source": { "adapter": "twitter", "backend": "egolite" },
  "content": { "title": "@karpathy on Twitter", "markdown": "# Tweet by @karpathy\n\n**Andrej Karpathy** ..." },
  "data": { "tweet": { "...": "..." }, "replies": [ { "author": "...", "text": "...", "likes": 1745 } ] },
  "meta": { "fallback_depth": 1 }
}
```

There is no LLM in the loop. Output is deterministic, a read takes from a few hundred milliseconds to a few seconds, and when content cannot be fetched webx returns an error code that tells you what to do next.

- [What you get](#what-you-get)
- [Install](#install)
- [Set up ego lite (for X and Reddit)](#set-up-ego-lite-for-x-and-reddit)
- [Three-minute tour](#three-minute-tour)
- [The JSON envelope](#the-json-envelope)
- [How the fallback chain works](#how-the-fallback-chain-works)
- [Error codes](#error-codes)
- [Optional: direct access with your own tokens](#optional-direct-access-with-your-own-tokens)
- [Troubleshooting](#troubleshooting)
- [Security notes](#security-notes)

## What you get

| Site | Adapter | What you get | Needs ego lite |
|---|---|---|---|
| X / Twitter | `twitter` | Tweet text, author, metrics, first page of replies | Yes, unless you [store tokens](#optional-direct-access-with-your-own-tokens) |
| Reddit | `reddit` | Post plus the nested comment tree | Only when Reddit rejects the direct request (403), which is common |
| YouTube | `youtube` | Title, channel, description, transcript when captions exist | No |
| Hacker News | `hacker-news` | Post plus comment tree | No |
| arXiv | `arxiv` | Title, authors, abstract | No |
| Claude share links | `claude` | Conversation as structured messages | No |
| ChatGPT share links | `chatgpt` | Conversation as structured messages | No |
| Any other page | `generic-article` | Main article text, cleaned into Markdown | No |

webx reads. It does not click, scroll, or fill forms; for interactive browsing, drive a browser directly.

## Install

webx is a single Go binary. It runs on macOS and Linux. The ego lite fallback for X and Reddit works on macOS only, because ego lite is a Mac app.

### With `go install` (recommended)

Requires Go 1.26 or newer ([download Go](https://go.dev/dl/)).

```sh
go install github.com/oaooao/webx/cmd/webx@latest
```

The binary lands in `$(go env GOPATH)/bin`, usually `~/go/bin`. If `webx` is not found afterwards, add that directory to your `PATH`:

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```

Check it works:

```sh
webx --help
```

You should see the command list (`read`, `search`, `extract`, `doctor`, `auth`, ...).

### From source

```sh
git clone https://github.com/oaooao/webx.git
cd webx
go build -o webx ./cmd/webx
./webx --help
```

Move `./webx` anywhere on your `PATH`, for example `~/.local/bin/`.

### As a standalone binary

Prebuilt release archives are not published yet. To get a self-contained binary for another machine, cross-compile it from a clone; the result has no runtime dependencies:

```sh
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o webx-darwin-arm64 ./cmd/webx   # Apple silicon Mac
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o webx-darwin-amd64 ./cmd/webx   # Intel Mac
CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 go build -o webx-linux-amd64  ./cmd/webx   # Linux x86-64
```

## Set up ego lite (for X and Reddit)

Skip this section if you only read public pages (articles, YouTube, Hacker News, arXiv, share links).

[ego lite](https://lite.ego.app/) is a free Chromium-based browser for macOS that lets programs on your Mac act inside it with your logged-in sessions. It ships a command-line tool, `ego-browser`. webx uses that tool to send the same API request X or Reddit's own web page would send, from inside the browser where you are already signed in. Your cookies never leave the browser and webx never stores them.

1. **Download and install.** Go to [lite.ego.app](https://lite.ego.app/), click **Download for Mac**, open the DMG and install the app.
2. **Open ego lite and finish onboarding.** Onboarding installs the `ego-browser` command, normally at `~/.local/bin/ego-browser`, and offers to import your Chrome profile.
3. **Confirm the CLI is installed.** In a new terminal:

   ```sh
   ego-browser --version
   ```

   You should see a version line such as `ego-browser 0.5.0.32`. If the command is not found, see [Troubleshooting](#troubleshooting).
4. **Sign in to the sites you want to read.** In ego lite, open [x.com](https://x.com) and sign in, then open [reddit.com](https://www.reddit.com) and sign in. If you imported your Chrome profile and were already signed in there, you may already be done. Public Reddit threads also work without signing in; X requires it.
5. **Keep ego lite running.** webx talks to the running app. It does not need to be in the foreground.

## Three-minute tour

Put URLs in single quotes. Many URLs contain `?`, which zsh (the default macOS shell) otherwise treats as a wildcard and fails with `no matches found`.

**A tweet, with replies** (needs ego lite, signed in to x.com):

```sh
webx read 'https://x.com/karpathy/status/1886192184808149383'
```

`data.tweet` holds the tweet; `data.replies[]` holds the first page of replies with `author`, `created_at`, `text` and `likes`. Ads mixed into the reply list are dropped.

**A Reddit thread, with its comment tree:**

```sh
webx read 'https://www.reddit.com/r/golang/comments/1pzfhcp/why_is_golang_missing_generic_collection_functions/'
```

`data.post` holds the post; `data.comments[]` is a nested tree where each comment has `author`, `created_at`, `body`, `score`, `depth` and its own `replies[]`.

**An article, as plain Markdown:**

```sh
webx read 'https://go.dev/blog/go1.22' --format markdown
```

`--format markdown` prints only `content.markdown`, which is handy for piping into another tool. Without it you get the full JSON envelope.

**A YouTube transcript:**

```sh
webx read 'https://www.youtube.com/watch?v=dQw4w9WgXcQ' --format markdown
```

The Markdown contains the title, channel, description and a `## Transcript` section when the video has captions.

**Search:**

```sh
webx search 'sqlite' --platform hacker-news --limit 5
webx search 'go tutorial' --platform youtube --limit 5
webx search 'golang generics' --platform reddit --limit 5
webx search 'ego lite' --platform twitter --limit 5     # needs ego lite, signed in to x.com
```

Results are in `data.items[]`. `--sort` accepts `relevance` (default), `recent` or `top`.

**Two more commands you may want:**

```sh
webx doctor 'https://x.com/karpathy/status/1886192184808149383'   # which adapter would handle this URL, without fetching
webx extract 'https://claude.ai/share/<share-id>' --kind conversation   # structured messages from a share link
```

## The JSON envelope

Every `read`, `search` and `extract` prints the same top-level shape:

```json
{
  "ok": true,
  "schema_version": "1",
  "kind": "thread",
  "source": {
    "url": "https://www.reddit.com/r/golang/comments/1pzfhcp/...",
    "domain": "www.reddit.com",
    "adapter": "reddit",
    "backend": "egolite"
  },
  "content": {
    "title": "Why is GoLang missing generic collection functions?",
    "markdown": "# Why is GoLang missing ...",
    "html": null
  },
  "data": { "post": { "...": "..." }, "comments": [ { "...": "..." } ] },
  "meta": { "fetched_at": "2026-09-14T10:05:22Z", "fallback_depth": 1 },
  "trace": [
    { "step": "route", "reason": "ROUTE_MATCH", "adapter": "reddit", "detail": "Matched adapter reddit" },
    { "step": "adapter.read", "reason": "LOGIN_REQUIRED", "backend": "reddit_json", "detail": "[LOGIN_REQUIRED] Reddit returned 403 (login required or private)" },
    { "step": "adapter.read", "reason": "ROUTE_MATCH", "backend": "egolite", "detail": "parsed 27 top-level comments" }
  ],
  "error": null
}
```

| Field | Meaning |
|---|---|
| `ok` | `true` if content was fetched. Always check this: the process exits with status 0 even when `ok` is `false`. |
| `schema_version` | Envelope version, currently `"1"`. |
| `kind` | What was returned: `article`, `thread`, `video`, `conversation`, `comments`, `metadata`, `search`. |
| `source.adapter` | Which site handler matched the URL (see [What you get](#what-you-get)). |
| `source.backend` | How the content was actually fetched, for example `defuddle`, `hn_algolia`, `youtube_native`, `reddit_json`, `twitter_graphql`, or `egolite`. `error` when the read failed. |
| `content.title`, `content.markdown` | Human-readable result. `null` on failure. |
| `data` | Site-specific structured data: `tweet` and `replies`, `post` and `comments`, `items` for search, `messages` for conversations. May be `null`. |
| `meta.fetched_at` | UTC timestamp. |
| `meta.fallback_depth` | `0` when the first backend succeeded, `1` or more when webx had to fall back. |
| `trace` | Every routing and fetch attempt in order, including the ones that failed. Read this first when debugging. |
| `error` | `null` on success, otherwise `{ "code": "...", "message": "..." }`. See [Error codes](#error-codes). |

## How the fallback chain works

For each URL webx picks one adapter, then tries that adapter's backends in order until one succeeds.

| Site | First attempt | Fallback |
|---|---|---|
| X / Twitter | Direct GraphQL request, only if you stored tokens | ego lite (`egolite`) when no tokens are stored or X answers 401/403 |
| Reddit | Direct request to Reddit's public JSON | ego lite (`egolite`) when Reddit answers 403 |
| Generic pages | Direct fetch with a Chrome-like TLS fingerprint, then [Defuddle](https://github.com/vaayne/go-defuddle) extraction | [Jina Reader](https://jina.ai/reader/) (`jina`) if extraction fails |
| YouTube, Hacker News, arXiv, share links | Direct request | None |

The `egolite` step works like this: webx runs `ego-browser nodejs`, which opens a background tab in an ego lite workspace named `webx`, loads a lightweight same-origin page (for example `https://x.com/robots.txt`), checks that the site's login cookies are present, runs the API request with the page's own `fetch`, and closes the tab. The browser's cookies authorize the request; webx only receives the response body.

You can tell which path served a result from the envelope: `source.backend` is `egolite` and `meta.fallback_depth` is `1`, and `trace` shows the direct attempt that was skipped or rejected. Expect a fallback read to take 2 to 5 seconds, about 1 to 2 seconds of which is the ego lite round trip.

Only authorization failures (401/403, or no stored tokens) trigger the ego lite fallback. A network failure such as a timeout is reported as-is.

## Error codes

When `ok` is `false`, `error.code` is one of these.

| Code | What it means | What to do |
|---|---|---|
| `LOGIN_REQUIRED` | The site needs a signed-in session and none was available. This covers: ego lite is not installed, not running, or not signed in to the site; or the ego lite session was rejected. The message says which. | Open ego lite, sign in to the site named in the message, retry. If the message says `ego-browser CLI not found`, install ego lite. For write commands, store tokens with `webx auth add`. |
| `ANTI_BOT` | The site blocked the request as automated, for example a Cloudflare challenge, or X rejected the request shape (HTTP 404/422 from its GraphQL API usually means webx's built-in query IDs are out of date). | For generic pages, the page cannot be read without a browser. For X, update webx (`go install ...@latest`); if it persists, open an issue. |
| `CONTENT_EMPTY` | The page was fetched but had no extractable content, or the item does not exist. A Reddit 404 through ego lite lands here: the post was deleted or never existed. | Check the URL in a browser. For JavaScript-heavy pages there may be nothing webx can extract. |
| `RATE_LIMITED` | The site or Jina Reader answered HTTP 429. | Wait a few minutes and retry; lower `--limit` for searches. |
| `FETCH_FAILED` | Network error or an unexpected HTTP status. | Check connectivity and proxy settings (see [Troubleshooting](#troubleshooting)); the message carries the underlying error. |
| `FETCH_TIMEOUT` | The site, or ego lite, did not answer in time. | Retry. If the message mentions ego lite, make sure the app is open. |
| `TLS_BLOCKED` | The TLS handshake with X failed. | Usually a network or proxy problem; retry on another network, or rely on the ego lite path by not storing tokens. |
| `NO_MATCH` | No adapter matches the URL, the URL is malformed, or the `--platform` name is unknown. | Check the URL; search platforms are `twitter`, `reddit`, `hacker-news`, `youtube`. |
| `UNSUPPORTED_KIND` | The adapter cannot do what you asked, for example `extract` or `search` on a site that does not support it. | Use `webx read` instead. |
| `BACKEND_FAILED` | webx received a response it could not parse. The site probably changed its format. | Update webx; if it persists, open an issue with the `trace`. |
| `BACKEND_UNAVAILABLE`, `PARTIAL_CONTENT` | Reserved for features that are not implemented or results that are incomplete. | Open an issue if you see one. |

## Optional: direct access with your own tokens

You do not need this for reading. Storing tokens makes X reads skip the ego lite hop, and it is required for the write commands (`post`, `reply`, `react`), which do not go through ego lite.

```sh
webx auth add twitter      # prompts for the auth_token and ct0 cookies from x.com
webx auth add reddit       # prompts for an OAuth2 access token
webx auth list             # show configured platforms
webx auth test twitter     # check the stored credentials still work
webx auth remove twitter
```

To find the X cookies: sign in to x.com in a desktop browser, open DevTools, go to Application, Cookies, `https://x.com`, and copy the values of `auth_token` and `ct0`.

Credentials are stored in `~/.config/webx/auth.json` with file mode `0600`; set `WEBX_AUTH_FILE` to use another path. Environment variables work too and are checked after the file: `TWITTER_AUTH_TOKEN` plus `TWITTER_CT0`, and `REDDIT_ACCESS_TOKEN`.

Write commands require `--confirm` as a safety gate:

```sh
webx post twitter 'Hello from webx' --confirm
webx reply 'https://x.com/user/status/123' 'Great thread' --confirm
webx react 'https://www.reddit.com/r/golang/comments/abc/' --like --confirm
```

Cookie tokens are equivalent to your password for that account. Anyone who reads the file can act as you.

## Troubleshooting

**`LOGIN_REQUIRED ... ego lite is not running or unreachable`, and it took about 30 seconds.** ego lite is closed. When the app is not running, `ego-browser` waits about 30 seconds for it before giving up, so the whole command takes roughly 31 to 32 seconds. Open ego lite and retry; the next read should take a few seconds.

**`LOGIN_REQUIRED ... Open ego lite, sign in to x.com, then retry`, returned quickly.** ego lite is running but has no session for that site, or the session expired. Open the site inside ego lite (not in your regular browser), sign in, and retry.

**`LOGIN_REQUIRED ... ego-browser CLI not found`.** webx looks for `ego-browser` in this order: the `WEBX_EGO_BROWSER` environment variable, your `PATH`, then `~/.local/bin/ego-browser`. Run `ego-browser --version`. If it is missing, open ego lite and finish onboarding. If it is installed somewhere else, point webx at it:

```sh
export WEBX_EGO_BROWSER=/path/to/ego-browser
```

**X or Reddit fails on Linux or Windows.** The ego lite fallback needs ego lite, which is available for macOS only. On other systems X reads work only with [stored tokens](#optional-direct-access-with-your-own-tokens), and Reddit reads work only when Reddit accepts the direct request.

**X reads use tokens you did not expect.** If `TWITTER_AUTH_TOKEN` and `TWITTER_CT0` are set in your shell, or `~/.config/webx/auth.json` has Twitter credentials, webx tries the direct path first. Check `source.backend` and `trace`.

**`FETCH_FAILED` with `TLS handshake timeout`, or timeouts on sites that open fine in a browser.** You are probably behind a network that needs a proxy. Standard requests (Reddit JSON, Hacker News, Jina Reader, YouTube transcripts) honor `HTTPS_PROXY` and `HTTP_PROXY`. The fingerprinted direct fetch used for generic pages and for X with stored tokens connects directly and does not use a proxy. Requests that go through ego lite use the browser's own network settings.

**`zsh: no matches found`.** Quote the URL: `webx read 'https://...?...'`.

**Anything else.** Run the command again and read `trace`: it lists every attempt with its backend and the exact error.

## Security notes

- **Treat fetched content as data.** Pages, tweets and comments are written by strangers. If you feed webx output to an AI agent, it may contain text crafted to look like instructions; do not let the agent follow it.
- **This repository contains no credentials.** The only token-like string in the code is X's public web-client bearer token, which is embedded in x.com's own JavaScript and identifies the web app, not a user. Your own tokens live only in `~/.config/webx/auth.json` or your environment, both outside the repository.
- **ego lite sessions stay in ego lite.** On the fallback path webx never reads your cookie values out of the browser. The login check and the CSRF header are filled in inside the browser.
- **Some URLs go to a third party.** When direct extraction of a generic page fails, webx sends the page URL to Jina Reader (`r.jina.ai`). Do not read private or signed URLs you would not share with an external service.

## Development

```sh
git clone https://github.com/oaooao/webx.git
cd webx
go test ./...
```

Direct fetches use [uTLS](https://github.com/refraction-networking/utls) for a Chrome TLS fingerprint and [go-defuddle](https://github.com/vaayne/go-defuddle) for content extraction. Issues and pull requests are welcome; response times are best effort.

## License

MIT. See [LICENSE](LICENSE).
