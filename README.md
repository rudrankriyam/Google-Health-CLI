# Google-Health-CLI

[![PR Checks](https://github.com/rudrankriyam/Google-Health-CLI/actions/workflows/pr-checks.yml/badge.svg)](https://github.com/rudrankriyam/Google-Health-CLI/actions/workflows/pr-checks.yml)
[![Release](https://github.com/rudrankriyam/Google-Health-CLI/actions/workflows/release.yml/badge.svg)](https://github.com/rudrankriyam/Google-Health-CLI/actions/workflows/release.yml)

Unofficial Google-Health-CLI for the Google Health API, written in Go.

The CLI ships as `ghealth` and provides OAuth setup, health-data queries, rollups, profile and settings access, paired-device and IRN-profile reads, webhook management, and machine-readable output for scripts and agents.

## Install

```sh
brew install rudrankriyam/tap/ghealth
```

Or install with Go:

```sh
go install github.com/rudrankriyam/Google-Health-CLI/cmd/ghealth@latest
```

Check the installation and local configuration:

```sh
ghealth doctor
```

## Quick Start

The registry and agent-discovery commands do not require authentication:

```sh
ghealth types list
ghealth types describe heart-rate-variability
ghealth endpoints list
ghealth agent manifest
```

Run `ghealth --help` or append `--help` to a command to inspect its current flags.

## Authentication

Configure an OAuth client before calling user-data endpoints. Prefer supplying the client secret through the environment instead of persisting it:

```sh
ghealth config set client-id YOUR_CLIENT_ID
export GHEALTH_CLIENT_SECRET=YOUR_CLIENT_SECRET
ghealth auth login
```

The CLI does not accept client secrets as command-line arguments, where they could be retained in shell history or process listings.

The default login requests the documented read-only Google Health scopes. Request write access only when the CLI must create, update, or delete data:

```sh
ghealth auth login --write
```

Google Health write scopes are write-only: they do not grant read access. The `--write` convenience flag therefore requests both the read scopes and their matching `.writeonly` scopes. For least-privilege access, pass the exact comma-separated scope URLs with `--scope`.

Subscriber and manual-subscription commands do not consume the end-user token created by `ghealth auth login`. They use [Application Default Credentials](https://cloud.google.com/docs/authentication/application-default-credentials) with the Google Cloud `cloud-platform` scope.

For local development, create ADC with the Google Cloud CLI:

```sh
gcloud auth application-default login \
  --scopes=https://www.googleapis.com/auth/cloud-platform
```

In CI or production, use workload identity or another ADC-supported credential source instead of exporting a long-lived service-account key. The `ghealth auth login --scope management` alias resolves the same scope for explicit OAuth flows, but subscriber commands intentionally ignore that saved user token and load ADC instead.

### Upgrading from 1.0.x

Google replaced the previous write scope names with `.writeonly` scope names. A token authorized for writes by `ghealth` 1.0.x does not automatically gain the corrected permissions. Run `ghealth auth login --write` again and complete the consent flow before using write commands.

The release also adds ECG, IRN, and exercise-location read scopes. Reauthorize when you need those newly supported resources. `ghealth auth status` shows the scopes recorded with the current token.

## Health Data

List heart-rate data:

```sh
ghealth data list heart-rate \
  --from 2026-05-08T00:00:00Z \
  --to 2026-05-09T00:00:00Z \
  --json
```

Endpoint identifiers use kebab case, such as `heart-rate`; filter fields use snake case, such as `heart_rate.sample_time.physical_time`.

Reconcile sleep data from a specific source family:

```sh
ghealth data reconcile sleep \
  --from 2026-05-01T00:00:00Z \
  --to 2026-05-09T00:00:00Z \
  --family users/me/dataSourceFamilies/google-wearables
```

Roll up daily steps:

```sh
ghealth rollup daily steps \
  --from 2026-05-01 \
  --to 2026-05-09 \
  --window-days 1
```

List newly supported data types:

```sh
ghealth data list active-energy-burned --from 2026-05-01 --to 2026-05-08
ghealth data list blood-glucose --from 2026-05-01T00:00:00Z --to 2026-05-08T00:00:00Z
ghealth data list electrocardiogram --from 2026-05-01T00:00:00Z
ghealth data list irregular-rhythm-notification
```

Google limits a single query to 14 days for active minutes, heart rate, calories in heart-rate zones, and total calories. The maximum for other data types is 90 days. Paginate larger result sets with `--page-token`.

Export an exercise as raw TCX. This requires both activity-and-fitness read access and location read access:

```sh
ghealth data export-tcx EXERCISE_ID > exercise.tcx
```

Use `--partial-data` to include TCX points when GPS data is unavailable.

## User Resources

```sh
ghealth profile get
ghealth settings get
ghealth identity get
ghealth irn-profile get
ghealth paired-devices list --limit 5
ghealth paired-devices get DEVICE_ID
```

`identity get` returns the stable Google Health `healthUserId` used when creating a manual webhook subscription. Paired-device responses can include device identifiers and hardware details; handle exported JSON as user data.

## Webhook Subscribers

Webhook administration requires a Google Cloud project **number**, Application Default Credentials authorized for the `cloud-platform` scope, and the corresponding IAM permissions.

Create `subscriber.json` without committing its authorization secret:

```json
{
  "endpointUri": "https://example.com/webhooks/google-health",
  "subscriberConfigs": [
    {
      "dataTypes": ["steps", "weight"],
      "subscriptionCreatePolicy": "AUTOMATIC"
    },
    {
      "dataTypes": ["sleep"],
      "subscriptionCreatePolicy": "MANUAL"
    }
  ],
  "endpointAuthorization": {
    "secret": "Bearer REPLACE_WITH_A_RANDOM_SECRET"
  }
}
```

Manage the subscriber:

```sh
ghealth subscribers list --project YOUR_PROJECT_NUMBER
ghealth subscribers create --project YOUR_PROJECT_NUMBER --subscriber-id my-sub --file subscriber.json
ghealth subscribers patch \
  --name projects/YOUR_PROJECT_NUMBER/subscribers/my-sub \
  --update-mask endpointUri,subscriberConfigs \
  --file subscriber.json
ghealth subscribers delete \
  --name projects/YOUR_PROJECT_NUMBER/subscribers/my-sub \
  --yes
```

`AUTOMATIC` subscriptions follow user consent and cannot be managed individually. A `MANUAL` subscriber configuration requires an explicit subscription for each user and data type:

```json
{
  "user": "users/HEALTH_USER_ID",
  "dataTypes": [
    "users/HEALTH_USER_ID/dataTypes/sleep"
  ]
}
```

```sh
ghealth subscribers subscriptions create \
  --parent projects/YOUR_PROJECT_NUMBER/subscribers/my-sub \
  --subscription-id my-user \
  --file subscription.json

ghealth subscribers subscriptions list \
  --parent projects/YOUR_PROJECT_NUMBER/subscribers/my-sub \
  --filter 'user = "users/HEALTH_USER_ID" AND data_type = "sleep"'

ghealth subscribers subscriptions patch \
  --name projects/YOUR_PROJECT_NUMBER/subscribers/my-sub/subscriptions/my-user \
  --update-mask dataTypes \
  --file subscription.json

ghealth subscribers subscriptions delete \
  --name projects/YOUR_PROJECT_NUMBER/subscribers/my-sub/subscriptions/my-user \
  --yes
```

Google verifies a new or changed endpoint with one authorized request that must return `200` or `201` and one unauthorized request that must return `401` or `403`. For notifications, verify the `GOOGLE-HEALTH-API-SIGNATURE` against the raw request body, acknowledge immediately with `204 No Content`, and process asynchronously. Delivery is retried with exponential backoff for up to seven days, so handlers must be idempotent.

See Google's [webhook guide](https://developers.google.com/health/webhooks) for endpoint verification, signature-key rotation, and notification payloads.

## Raw API

Use the raw escape hatch for newly added or discovery-only methods that do not yet have a dedicated command:

```sh
ghealth api GET /v4/users/me/profile --json
```

Raw POST, PUT, PATCH, and DELETE requests require `--yes`. Use `--raw` to
preserve non-JSON response bytes and `--no-auth` only for genuinely public
endpoints. For example, a discovery-only public SHL resource can be downloaded
without forcing it through the JSON renderer:

```sh
ghealth api GET /v4/shl/r/EXTERNAL_SHL_ID/RESOURCE_TOKEN \
  --no-auth \
  --raw > resource.bin
```

The live discovery document currently contains two SHL methods and a
`basal-energy-burned` data type that are not yet listed in the public REST or
data-type navigation. They remain behind the raw escape hatch; this release
does not claim stable typed contracts for them.

## Output and Automation

`ghealth` uses table output in an interactive terminal and JSON when output is piped. Force a format when a script requires one:

```sh
ghealth types list --format table
ghealth types list --format markdown
ghealth data list steps --format ndjson
ghealth data list steps --format csv
ghealth data list steps --json --pretty
```

CLI-owned JSON fields in manifests, capabilities, registry records, and errors are stable and evolve additively. Health API response bodies mirror Google's upstream JSON contract. Scripts should branch on operation names rather than hard-code registry counts.

Destructive commands require `--yes`. API failures use stable exit codes so automation can branch cleanly.

## Agent Commands

```sh
ghealth agent manifest
ghealth agent capabilities
ghealth agent schema --type sleep
ghealth agent context today
```

`agent schema` returns stable data-type metadata; it is not a JSON Schema for Google request or response bodies. Use `agent capabilities` to discover commands, operation names, current scopes, and supported data types.

## API Coverage

`ghealth` tracks the publicly documented Google Health v4 surface:

- 39 data types
- 25 documented REST methods
- Profile, settings, identity, IRN profile, paired devices, data points, rollups, TCX export, subscribers, and manual subscriptions

See [Endpoint Coverage](docs/endpoint-coverage.md), the official [data-type registry](https://developers.google.com/health/data-types), and the official [REST reference](https://developers.google.com/health/reference/rest).

## Development

```sh
go test ./...
go build ./...
go build -o ghealth ./cmd/ghealth
```

Releases are tag-driven through GoReleaser. Tags do not use a `v` prefix:

```sh
git tag 1.1.0
git push origin 1.1.0
```
