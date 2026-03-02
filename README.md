# Michelangelo

A self-hosted Tumblr media client written in Go. Shows photos and videos from your dashboard in a true masonry grid. No ads, no tracking, no npm.

## Features

- True CSS masonry grid (variable-height, packed tight)
- Infinite scroll via IntersectionObserver
- Browse individual blogs in grid view (`/blog/<name>`)
- Tag search (`/search?q=<tag>`)
- Like and reblog from the lightbox
- Dark/light mode via `@media (prefers-color-scheme)`
- Zero external runtime dependencies (Go stdlib only)

## Setup

### 1. Create a Tumblr app

Go to https://www.tumblr.com/oauth/apps and create an app.  
Set the callback URL to `http://localhost:8080/auth/callback` (or your domain).

### 2. Configure environment

```sh
export CONSUMER_KEY="your-consumer-key"
export CONSUMER_SECRET="your-consumer-secret"
export PORT=8080  # optional, defaults to 8080
```

Or use a `.env` file with a tool like `direnv`.

### 3. Build and run

```sh
go build -o michelangelo .
./michelangelo
```

Then open http://localhost:8080 — you'll be redirected to Tumblr to authorize.

## Structure

```
main.go          — HTTP router, OAuth flow, API proxy handlers
tumblr.go        — Tumblr API client (OAuth 1.0a, no external deps)
templates/
  layout.html    — Single HTML shell
static/
  app.css        — Masonry grid, dark/light modes
  app.js         — Infinite scroll, lightbox, like/reblog
```

## Notes

- OAuth tokens are stored in cookies. Clear them or hit `/logout` to re-authenticate.
- The like/reblog endpoints use the Tumblr v1 API; reblogging posts to your own blog requires your blog name to be detected correctly on first auth.
- Tag search uses the `/tagged` endpoint which returns one batch (no pagination).
