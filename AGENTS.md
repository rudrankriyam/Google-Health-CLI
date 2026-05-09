# Agent Notes

This repository builds `ghealth`, a Go CLI for the Google Health API.

## Priorities

- Keep the binary name `ghealth`.
- Keep output deterministic for automation. Non-interactive output should stay JSON by default.
- Preserve `ghealth api METHOD PATH` as the compatibility escape hatch when Google changes the API.
- Require `--yes` for destructive commands.
- Do not copy implementation details from MCP projects. Use public Google Health docs as the source of truth.

## Checks

Run before committing:

```sh
gofmt -w .
go test ./...
go build ./...
```
