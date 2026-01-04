# Mastodon CLI (Go)

![Mastodon CLI](image.png)

Minimal Go CLI for Mastodon. Supports OAuth login and reading your home timeline.

## Features

- OAuth authorization code flow (OOB redirect)
- Home timeline fetch with configurable limit
- Local config storage with secure permissions

## Installation

```bash
go build -o mastodon ./cmd/mastodon
```

## Usage

Log in and authorize the app:

```bash
./mastodon login --instance mastodon.social
```

Fetch the latest posts from your home timeline:

```bash
./mastodon timeline --limit 10
```

Fetch your own posts:

```bash
./mastodon posts --limit 10
```

Show help:

```bash
./mastodon help
```

Launch the TUI:

```bash
./mastodon ui
```

## Configuration

Config is stored at `~/.config/mastodon-cli/config.json` and includes:

- `instance` (e.g. `mastodon.social`)
- `client_id` and `client_secret`
- `access_token`
- `redirect_uri` (defaults to `urn:ietf:wg:oauth:2.0:oob`)

File permissions are set to `0600`.

## Commands

- `login --instance <domain> [--force]`
  - Registers the OAuth app if needed, then prompts for the authorization code.
  - `--force` re-registers the app even if one is already stored.
- `timeline --limit <n>`
  - Reads the home timeline. `n` must be 1-40.
- `posts --limit <n> [--boosts] [--replies]`
  - Reads your own posts. By default boosts and replies are excluded. Supports pagination up to 800 posts and shows progress for larger requests.
- `ui`
  - Launches a TUI to browse your home timeline. Press `r` to fetch newer statuses.

## API Notes

This CLI follows the Mastodon API docs:

- App registration: `POST /api/v1/apps`
- OAuth authorization: `GET /oauth/authorize`
- Token exchange: `POST /oauth/token`
- Home timeline: `GET /api/v1/timelines/home`

Scopes: the CLI requests `read`, which is sufficient for home timeline access.
