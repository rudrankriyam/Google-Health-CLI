# Contributing

Thanks for helping improve `ghealth`.

## Development

```sh
go test ./...
go build ./...
```

Run formatting before opening a pull request:

```sh
gofmt -w .
```

## Design Rules

- Keep `ghealth` stable for scripts and agents.
- Prefer additive commands over breaking existing output contracts.
- Keep JSON field names stable once released.
- Keep destructive commands behind `--yes`.
- Use official Google Health documentation as the API source of truth.

## Releases

Releases are tag-driven:

```sh
git tag 1.0.0
git push origin 1.0.0
```

The release workflow uses GoReleaser to publish GitHub release artifacts and update the Homebrew tap.
