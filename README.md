# Google-Health-CLI

[![PR Checks](https://github.com/rudrankriyam/Google-Health-CLI/actions/workflows/pr-checks.yml/badge.svg)](https://github.com/rudrankriyam/Google-Health-CLI/actions/workflows/pr-checks.yml)
[![Release](https://github.com/rudrankriyam/Google-Health-CLI/actions/workflows/release.yml/badge.svg)](https://github.com/rudrankriyam/Google-Health-CLI/actions/workflows/release.yml)

Unofficial Google-Health-CLI for the Google Health API, written in Go.

The CLI ships as `ghealth` and gives you OAuth setup, data queries, rollups, profile and settings access, webhook subscriber management, and predictable JSON for scripts and agents.

## Install

```sh
brew install rudrankriyam/tap/ghealth
```

Or install with Go:

```sh
go install github.com/rudrankriyam/Google-Health-CLI@latest
```

Check your setup:

```sh
ghealth doctor
```

## Quick Start

```sh
ghealth types list
ghealth types describe heart-rate-variability
ghealth endpoints list
ghealth agent manifest
```

To call Google Health data, configure OAuth first:

```sh
ghealth config set client-id YOUR_CLIENT_ID
ghealth config set client-secret YOUR_CLIENT_SECRET
ghealth auth login
```

Environment variables are supported too:

```sh
export GHEALTH_CLIENT_ID=...
export GHEALTH_CLIENT_SECRET=...
ghealth auth login
```

Read-only scopes are requested by default. Use write scopes only when you need to create, update, or delete data:

```sh
ghealth auth login --write
```

## Examples

List heart-rate data:

```sh
ghealth data list heart-rate \
  --from 2026-05-08T00:00:00Z \
  --to 2026-05-09T00:00:00Z \
  --json
```

Reconcile sleep data:

```sh
ghealth data reconcile sleep \
  --from 2026-05-01T00:00:00Z \
  --to 2026-05-09T00:00:00Z \
  --family users/me/dataSourceFamilies/all-sources
```

Roll up daily steps:

```sh
ghealth rollup daily steps \
  --from 2026-05-01 \
  --to 2026-05-09 \
  --window-days 1
```

Read profile, settings, and identity:

```sh
ghealth profile get
ghealth settings get
ghealth identity get
```

Manage webhook subscribers:

```sh
ghealth subscribers list --project YOUR_PROJECT_ID
ghealth subscribers create --project YOUR_PROJECT_ID --subscriber-id my-sub --file subscriber.json
ghealth subscribers patch --name projects/YOUR_PROJECT_ID/subscribers/my-sub --update-mask endpoint_uri --file subscriber.json
ghealth subscribers delete --name projects/YOUR_PROJECT_ID/subscribers/my-sub --yes
```

Use the raw API escape hatch:

```sh
ghealth api GET /v4/users/me/profile --json
```

## Output

`ghealth` uses table output in an interactive terminal and JSON when output is piped. You can force an output format:

```sh
ghealth types list --format table
ghealth types list --format markdown
ghealth data list steps --format ndjson
ghealth data list steps --format csv
ghealth data list steps --json --pretty
```

## Agent Commands

These commands are designed for tools that need stable machine-readable context:

```sh
ghealth agent manifest
ghealth agent capabilities
ghealth agent schema --type sleep
ghealth agent context today
```

Destructive commands require `--yes`. API failures use stable exit codes so automation can branch cleanly.

## API Coverage

`ghealth` tracks the documented Google Health v4 surface:

- 31 data types
- 18 REST methods
- profile, settings, identity, data points, rollups, TCX export, and webhook subscribers

See [Endpoint Coverage](docs/endpoint-coverage.md).

## Development

```sh
go test ./...
go build ./...
```

Releases are tag-driven through GoReleaser:

```sh
git tag 1.0.0
git push origin 1.0.0
```
