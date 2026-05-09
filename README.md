# ghealth

[![PR Checks](https://github.com/rudrankriyam/google-health-cli/actions/workflows/pr-checks.yml/badge.svg)](https://github.com/rudrankriyam/google-health-cli/actions/workflows/pr-checks.yml)
[![Release](https://github.com/rudrankriyam/google-health-cli/actions/workflows/release.yml/badge.svg)](https://github.com/rudrankriyam/google-health-cli/actions/workflows/release.yml)

`ghealth` is a Go workbench CLI for the Google Health API. It is built for fast terminal use, stable automation, and agent workflows that need predictable JSON.

The command name is intentionally short and Homebrew-friendly:

```sh
ghealth types list
ghealth data list steps --from 2026-05-08T00:00:00Z --to 2026-05-09T00:00:00Z --json
ghealth agent manifest
```

## Status

This is a 1.0-ready scaffold for the newly documented Google Health API surface:

- 31 data types from the official Google Health data types docs
- 18 v4 REST methods from the official REST reference
- OAuth token storage under the local user config directory
- Table output for people, JSON output for pipes and agents
- Raw `ghealth api METHOD PATH` escape hatch for new or changing endpoints

Google notes that breaking changes may occur until the end of May 2026, so the CLI keeps the raw API path available on purpose.

## Install

From source:

```sh
go install github.com/rudrankriyam/google-health-cli@latest
```

Local development:

```sh
go build -o ghealth .
./ghealth doctor
```

Homebrew is planned with the formula name `ghealth`.

Release automation is wired through GoReleaser. Publishing to `rudrankriyam/homebrew-tap` requires a repository secret named `TAP_GITHUB_TOKEN` with write access to the tap.

## Auth

Create an OAuth client for the Google Health API, then set the client ID:

```sh
ghealth config set client-id YOUR_CLIENT_ID
ghealth config set client-secret YOUR_CLIENT_SECRET
ghealth auth login
```

Environment variables work too:

```sh
export GHEALTH_CLIENT_ID=...
export GHEALTH_CLIENT_SECRET=...
ghealth auth login
```

Read-only scopes are requested by default. Add write scopes only when needed:

```sh
ghealth auth login --write
```

## Commands

```sh
ghealth doctor
ghealth auth status
ghealth types list
ghealth types describe heart-rate-variability
ghealth endpoints list
ghealth profile get
ghealth settings get
ghealth identity get
ghealth data list heart-rate --from 2026-05-08T00:00:00Z --to 2026-05-09T00:00:00Z
ghealth data reconcile sleep --from 2026-05-01T00:00:00Z --to 2026-05-09T00:00:00Z
ghealth rollup daily steps --from 2026-05-01 --to 2026-05-09
ghealth subscribers list --project YOUR_PROJECT_ID
ghealth api GET /v4/users/me/profile
```

## Output

`ghealth` defaults to tables in an interactive terminal and JSON when output is piped. You can force a format:

```sh
ghealth types list --format markdown
ghealth data list steps --json --pretty
ghealth data list steps --format ndjson
ghealth data list steps --format csv
```

## Agent Mode

Agents should start here:

```sh
ghealth agent manifest
ghealth agent capabilities
ghealth agent schema --type sleep
```

Destructive commands require `--yes`. API failures map to stable exit codes so agent loops can branch cleanly.

## Project Boundary

This project is an original CLI implementation based on the public Google Health API documentation. It is not affiliated with Google and does not copy or vendor code from Google Health MCP projects.
