# AGENTS.md

## Project Summary
MastodonCLI is a Go-based Mastodon client with both a CLI and a TUI. It supports OAuth login, reading the home timeline, and fetching your own posts (with pagination). The TUI is built with Charm (Bubble Tea/Bubbles/Lip Gloss) and uses the same internal API client as the CLI.

## Key Commands
- Build binary: `go build -o mastodon ./cmd/mastodon`
- Run CLI: `./mastodon <command>`
- Run TUI: `./mastodon ui`

## CLI Commands
- `mastodon login --instance <domain> [--force]`
- `mastodon timeline --limit <n>` (1-40)
- `mastodon posts --limit <n> [--boosts] [--replies]` (1-800, paginates)
- `mastodon ui` (TUI timeline)

## OAuth / Config
- Config file: `~/.config/mastodon-cli/config.json`
- Stored values: `instance`, `client_id`, `client_secret`, `access_token`, `redirect_uri`
- OAuth flow: app registration (`POST /api/v1/apps`) + authorization code (OOB) + token exchange (`POST /oauth/token`)

## Project Structure
- `cmd/mastodon/main.go`: entrypoint
- `internal/cli`: CLI command handling + prompt
- `internal/config`: config load/save
- `internal/mastodon`: Mastodon API client + models
- `internal/output`: CLI formatting + HTML stripping
- `internal/ui`: TUI implementation

## TUI Notes
- Timeline list on left with filtering/shortcuts and highlight
- Detail pane on right with full post content
- Press `r` to fetch newer statuses using `since_id`
- Spinner in title while loading; list has a loading placeholder item

## API Client Notes
- `HomeTimelinePage(limit, sinceID, maxID)` supports pagination
- `AccountStatuses` supports `exclude_reblogs`, `exclude_replies`, and `max_id`

## Development Tips
- Use `rg` for searching
- Keep ASCII in files unless existing file uses Unicode
- Prefer updating shared client logic in `internal/mastodon` so CLI and TUI stay in sync
- Maintain CLI parity: every user-facing feature exposed in the TUI should also have a CLI command

## Mastodon API Docs (Quick Navigation)
Primary docs: https://docs.joinmastodon.org/api/

Common endpoints used here:
- OAuth + app registration:
  - Apps: https://docs.joinmastodon.org/methods/apps/
  - OAuth: https://docs.joinmastodon.org/methods/oauth/
  - Tokens/scopes: https://docs.joinmastodon.org/api/oauth-tokens/ and https://docs.joinmastodon.org/api/oauth-scopes/
- Timelines:
  - Home/public timelines: https://docs.joinmastodon.org/methods/timelines/
  - Local vs federated: use `/api/v1/timelines/public?local=true` (local) and `?local=false` (federated)
- Trends (Trending tab):
  - https://docs.joinmastodon.org/methods/trends/ (statuses: `/api/v1/trends/statuses`)

Navigation tips:
- Sidebar → “API Methods” → desired section (apps/oauth/timelines/trends).
- Use the “Request” section for required params and the “Response” examples for payload shape.
