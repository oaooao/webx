# webx

URL in, content out. A unified web runtime for AI agents.

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

webx routes any URL to the right backend, extracts clean content, and returns a consistent JSON envelope — no configuration, no browser, no API keys.

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

**Output as Markdown**

```sh
$ webx read https://reddit.com/r/golang/comments/abc --format markdown
```

## Commands

| Command | Description |
|---------|-------------|
| `webx read <url>` | Fetch URL, return content as JSON (default) or Markdown |
| `webx extract <url> --kind <kind>` | Extract structured data of a specific kind |
| `webx doctor <url>` | Diagnose routing, backend selection, and trace |

**Flags**

```
webx read --format json|markdown   # output format (default: json)
webx read --kind <kind>            # hint the expected content kind
webx extract --kind <kind>         # required: conversation, thread, article, video, comments, metadata
```

## Supported Platforms

| Platform | Adapter | What you get |
|----------|---------|--------------|
| Twitter / X | `twitter` | Tweet text, thread, author, metrics |
| Reddit | `reddit` | Post + top comments, subreddit, score |
| YouTube | `youtube` | Title, description, transcript (if available) |
| Hacker News | `hn` | Post + comment tree, scores |
| arXiv | `arxiv` | Title, authors, abstract |
| Claude Share | `claude` | Full conversation as structured messages |
| ChatGPT Share | `chatgpt` | Full conversation as structured messages |
| Any URL | `generic` | Clean article text via Defuddle extraction |

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

## Why webx

Most tools solve one layer of the problem. webx solves all five:

|  | Route | Extract | Anti-bot | Unified output | Zero config |
|--|-------|---------|----------|----------------|-------------|
| **webx** | ✅ | ✅ | ✅ | ✅ | ✅ |
| Defuddle | — | ✅ | — | — | — |
| Jina Reader | — | ✅ | cloud | — | — |
| wget / curl | — | — | — | — | ✅ |
| Playwright | ✅ | — | ✅ | — | — |

webx uses [uTLS](https://github.com/refraction-networking/utls) for TLS fingerprint spoofing and [go-defuddle](https://github.com/vaayne/go-defuddle) for content extraction — no headless browser required.

## Contributing

Bug reports and pull requests are welcome. For major changes, open an issue first.

```sh
git clone https://github.com/oaooao/webx.git
cd webx
go test ./...
```

## License

MIT — see [LICENSE](LICENSE).
