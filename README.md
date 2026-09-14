# webx

URL in, content out. The read layer of Axiom, the author's personal agent workbench.

> **Status: internal tool, not a product.** webx is maintained for one user's agents and ingestion jobs. It is public as source only: no releases, no roadmap, no support promise. Feature surface is frozen; fixes go in when they keep the read path working.

```
$ webx read https://news.ycombinator.com/item?id=43234567
{
  "ok": true,
  "kind": "thread",
  "source": { "adapter": "hn", "backend": "algolia" },
  "content": { "title": "Show HN: ...", "markdown": "..." },
  ...
}
```

webx routes a URL to the right backend, extracts clean content, and returns a consistent JSON envelope in a few hundred milliseconds, with no LLM in the loop. Public pages (HN, arXiv, GitHub, blogs, YouTube transcripts) are fetched directly. Logged-in sites (Twitter/X, Reddit) fall back to [ego lite](#logged-in-sites-via-ego-lite) when direct requests are rejected.

## Install

**Go install**

```sh
go install github.com/oaooao/webx/cmd/webx@latest
```

**Build from source**

```sh
git clone https://github.com/oaooao/webx.git
cd webx
go build -o webx ./cmd/webx
```

## Quick Start

**Read any URL**

```sh
$ webx read https://arxiv.org/abs/2501.12345
```

```json
{
  "ok": true,
  "schema_version": "1",
  "kind": "article",
  "source": {
    "url": "https://arxiv.org/abs/2501.12345",
    "domain": "arxiv.org",
    "adapter": "arxiv",
    "backend": "arxiv-api"
  },
  "content": {
    "title": "Attention Is All You Need (Revisited)",
    "markdown": "## Abstract\n\nWe revisit..."
  },
  "meta": {
    "fetched_at": "2026-04-14T10:00:00Z",
    "fallback_depth": 0
  }
}
```

**Extract structured data**

```sh
$ webx extract https://claude.ai/share/abc123 --kind conversation
```

```json
{
  "ok": true,
  "kind": "conversation",
  "data": {
    "messages": [
      { "role": "human", "content": "What is the capital of France?" },
      { "role": "assistant", "content": "Paris." }
    ]
  }
}
```

**Diagnose routing**

```sh
$ webx doctor https://twitter.com/user/status/123
```

```json
{
  "ok": true,
  "kind": "thread",
  "source": { "adapter": "twitter", "backend": "pending" },
  "trace": [
    { "step": "route", "reason": "ROUTE_MATCH", "adapter": "twitter", "detail": "Matched adapter twitter" }
  ]
}
```

**Search across platforms**

```sh
$ webx search "async rust" --platform hn
$ webx search "webx" --platform reddit          # subreddit + time filter supported
$ webx search "go tutorial" --platform youtube
$ webx search "AI agents" --platform twitter     # uses ego lite if no stored auth
```

**Post, reply, react**

```sh
$ webx post twitter "Hello from webx!" --confirm
$ webx reply https://x.com/user/status/123 "Great thread!" --confirm
$ webx react https://reddit.com/r/golang/comments/abc --like --confirm
```

Write operations require `--confirm` (safety gate) and platform authentication.

**Manage credentials**

```sh
$ webx auth add twitter    # interactive setup
$ webx auth list           # show configured platforms
$ webx auth test twitter   # verify credentials work
$ webx auth remove twitter
```

**Output as Markdown**

```sh
$ webx read https://reddit.com/r/golang/comments/abc --format markdown
```

## Commands

| Command | Description |
|---------|-------------|
| `webx read <url>` | Fetch URL, return content as JSON (default) or Markdown |
| `webx extract <url> --kind <kind>` | Extract structured data of a specific kind |
| `webx search <query> --platform <p>` | Search a platform (hn, reddit, youtube, twitter) |
| `webx post <platform> "content"` | Post new content to a platform |
| `webx reply <url> "content"` | Reply to a specific URL |
| `webx react <url> --like` | React to content (like, upvote, retweet) |
| `webx auth add <platform>` | Configure credentials for a platform |
| `webx doctor <url>` | Diagnose routing, backend selection, and trace |

## Supported Platforms

| Platform | Adapter | What you get |
|----------|---------|--------------|
| Twitter / X | `twitter` | Tweet text, author, metrics, first page of replies ¹ |
| Reddit | `reddit` | Post + comment tree, subreddit, score ¹ |
| YouTube | `youtube` | Title, description, transcript (if available) |
| Hacker News | `hn` | Post + comment tree, scores |
| arXiv | `arxiv` | Title, authors, abstract |
| Claude Share | `claude` | Full conversation as structured messages |
| ChatGPT Share | `chatgpt` | Full conversation as structured messages |
| Any URL | `generic` | Clean article text via Defuddle extraction |

¹ Reads through ego lite when direct requests are rejected. See [Logged-in sites via ego lite](#logged-in-sites-via-ego-lite).

## Logged-in sites via ego lite

Twitter/X and Reddit reject direct requests from scripts (401/403). webx does not fight that with exported tokens any more. Instead, when the direct backend is missing credentials or gets a 401/403, the adapter sends the same API request through **egolite**: it runs `page.fetch` inside [ego lite](https://lite.ego.app/), a Chromium browser where you are already signed in, via its `ego-browser nodejs` CLI.

- Requirements: ego lite is installed and open, `ego-browser` is on `PATH` or at `~/.local/bin/ego-browser` (override with `WEBX_EGO_BROWSER`), and you are signed in to x.com / reddit.com inside ego lite.
- Cost: the ego lite hop takes about 1–2 s (≈0.7 s process start, then a same-origin page open and the API call); a rejected direct attempt before it adds its own round trip, so a Twitter read measured 5.5 s end to end and a Reddit thread 2.7 s on 2026-09-14. No tokens leave the browser.
- Envelope: `source.backend` is `egolite` and `meta.fallback_depth` is `1`. The skipped or rejected direct attempt is recorded in `trace`.
- Data: `webx read` on a tweet returns `data.tweet` and `data.replies[]` (author, created_at, text, likes) from TweetDetail's first page. On a Reddit thread it returns `data.post` and `data.comments[]` as a nested tree (author, created_at, body, score).
- Errors: if ego lite is not running, `ego-browser` is missing, or the site is not signed in, the result is `LOGIN_REQUIRED` with a message telling you to sign in inside ego lite.

webx opens one Page per run in an ego lite task space named `webx` and closes it when done.

## Twitter Setup

Optional. Without stored credentials Twitter reads go straight to ego lite. Stored cookies only matter for write operations (post, reply, react) and for skipping the browser hop.

**Option A: Use `webx auth`**

```sh
webx auth add twitter
```

This interactively prompts for your cookies and stores them in `~/.config/webx/auth.json`.

**Option B: Environment variables**

1. Log in to [x.com](https://x.com) in your browser
2. Open DevTools (F12) → Application → Cookies → `https://x.com`
3. Copy `auth_token` and `ct0`, then export:

```sh
export TWITTER_AUTH_TOKEN="your_auth_token_here"
export TWITTER_CT0="your_ct0_here"
```

webx checks the auth store first, then environment variables. Without either, reads use ego lite; writes return `LOGIN_REQUIRED`.

## Reddit Setup

Reddit read operations need no credentials: the public JSON endpoint is tried first, then ego lite on 403. Write operations (post, comment, vote) require an OAuth2 access token:

```sh
webx auth add reddit
```

Or set `REDDIT_ACCESS_TOKEN` in your environment.

## Output Schema

Every response follows the same envelope:

```json
{
  "ok": true,
  "schema_version": "1",
  "kind": "article | conversation | thread | video | comments | metadata",
  "source": {
    "url": "...",
    "domain": "...",
    "adapter": "...",
    "backend": "..."
  },
  "content": {
    "title": "...",
    "markdown": "...",
    "html": null
  },
  "data": {},
  "meta": {
    "fetched_at": "2026-04-14T10:00:00Z",
    "fallback_depth": 0
  },
  "trace": [],
  "error": null
}
```

`data` contains adapter-specific structured output (e.g., conversation messages, comment trees). `trace` records each routing and fetch step for debugging.

## Scope

webx does one job for its agents: give a URL, get deterministic Markdown plus JSON quickly, with an honest error code when that is not possible. Content-extraction APIs (Jina, Firecrawl, Tavily, Exa) and agent browsers (ego lite, Vercel agent-browser) cover the general market better; webx stays small and local. Browsing, clicking and other interaction belong in ego lite, not here.

Direct fetches use [uTLS](https://github.com/refraction-networking/utls) for a Chrome TLS fingerprint and [go-defuddle](https://github.com/vaayne/go-defuddle) for content extraction.

## Contributing

This is an internal tool; issues and pull requests may not get a response.

```sh
git clone https://github.com/oaooao/webx.git
cd webx
go test ./...
```

## License

MIT — see [LICENSE](LICENSE).
