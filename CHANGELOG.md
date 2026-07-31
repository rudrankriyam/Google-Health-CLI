# Changelog

## 1.1.0

- Updated the registry to all 39 publicly documented Google Health data types and corrected the operation metadata for existing types.
- Added typed coverage for all 25 publicly documented v4 REST methods, including manual subscription CRUD, paired-device reads, and IRN-profile access.
- Updated all six write permissions to their current `.writeonly` names and added ECG, IRN, and exercise-location read scopes.
- Separated user-health OAuth from Google Cloud subscriber administration: management commands now use Application Default Credentials, with a `management` alias retained for explicit OAuth scope resolution.
- Fixed exercise TCX export to request raw media with `alt=media` and documented its activity-and-fitness plus location scope requirements.
- Updated subscriber payload fixtures for plural `dataTypes` and explicit `AUTOMATIC` or `MANUAL` subscription policies.
- Added nested `subscribers subscriptions` commands with pagination, filters, update masks, and explicit deletion confirmation.
- Added `paired-devices list|get` and `irn-profile get` commands.
- Removed the retired data-point `view` query parameter from the typed list surface while preserving `ghealth api` as the raw escape hatch.
- Added explicit raw-byte and unauthenticated escape-hatch modes for public `HttpBody` endpoints, while requiring `--yes` for potentially mutating raw methods.
- Added deterministic request and response fixtures for the expanded API surface, provider errors, malformed responses, pagination, and raw TCX output.
- Redacted OAuth client secrets from configuration output.
- Removed client-secret command-line and configuration-write inputs, scrubbed legacy persisted secrets on later config writes, and stopped passing Keychain token values through process arguments.
- Added atomic private-file replacement, strict loopback OAuth callback validation, bounded 16 MiB request and 64 MiB response bodies, and a 60-second default request timeout.
- Restricted requests to the canonical Google Health origin or literal loopback test servers so OAuth and ADC bearer tokens cannot be redirected to an arbitrary host.
- Made authentication status and local revoke receipts truthful when credentials are missing, malformed, expired, unreadable, or not actually deleted.
- Enforced data-type operation support and destructive confirmation before authentication or network access, with stricter time, date, and pagination validation.
- Fixed command and subcommand `--help` so discovery never requires authentication.
- Moved the executable entrypoint to `cmd/ghealth`, making the documented Go installation produce a binary named `ghealth`.
- Documented project-number requirements, webhook endpoint verification, notification signature verification, immediate `204` acknowledgement, idempotency, and seven-day retries.
- Added a migration note for 1.0.x write tokens: complete OAuth authorization again before using write commands.
- Recorded discovery-only SHL methods and basal-energy-burned separately without claiming stable typed contracts before they appear in the public references.

## 1.0.1

- Added fake-server tests for all 18 documented Google Health v4 REST methods.
- Added request payload, query parameter, response, API error, and malformed-response coverage.
- Added endpoint coverage documentation.
- Added missing client options for `updateMask`, subscriber pagination and force deletion, `dataSourceFamily`, and partial TCX export.
- Fixed CLI rollup request bodies to send documented `range` payloads instead of filter strings.
- Added nutrition data types to the registry.
- Persisted refreshed OAuth tokens so long-running sessions stay logged in.
- Added an opt-in macOS Keychain token storage backend (`GHEALTH_TOKEN_STORAGE=keychain`); the plaintext file remains the default.
- Fixed the OAuth callback server so duplicate callback requests can never block the handler, and gave server shutdown a 5-second timeout.
- Fixed `ghealth auth login` to surface config save failures instead of silently ignoring them.
- Fixed `ghealth help` and `ghealth version` to keep working when the config file is corrupt.
- Fixed civil date parsing to reject out-of-range months and days.
- Added tests for `internal/config` (round-trip, defaults, corrupt file).
- Removed unused `auth.AuthURL`, `healthapi.IsNotLoggedIn`, and `healthapi.IsAPIError` helpers and a dead nil-check on `oauth2.NewClient`.

## 1.0.0

- Initial `ghealth` CLI workbench.
- Added OAuth login, local token handling, and setup checks.
- Added registry for all 31 documented Google Health data types.
- Added registry for all 18 documented v4 REST methods.
- Added data point, rollup, profile, settings, identity, subscriber, and raw API commands.
- Added agent manifest/capabilities/schema commands.
- Added GoReleaser and GitHub Actions CI/CD.
