# Endpoint Coverage

The canonical executable coverage table is `internal/healthapi/client_test.go`. The registry follows the public Google Health REST reference; the live discovery document is checked separately for drift.

## Covered v4 Methods

| Method | Request Coverage | Response Coverage |
| --- | --- | --- |
| `projects.subscribers.create` | `POST /v4/projects/{project}/subscribers`, `subscriberId`, `CreateSubscriberPayload` | `Operation` |
| `projects.subscribers.delete` | `DELETE /v4/projects/{project}/subscribers/{subscriber}`, `force` | `Operation` |
| `projects.subscribers.list` | `GET /v4/projects/{project}/subscribers`, `pageSize`, `pageToken` | `ListSubscribersResponse` |
| `projects.subscribers.patch` | `PATCH /v4/projects/{project}/subscribers/{subscriber}`, `updateMask`, `Subscriber` | `Operation` |
| `projects.subscribers.subscriptions.create` | `POST /v4/projects/{project}/subscribers/{subscriber}/subscriptions`, `subscriptionId`, `CreateSubscriptionPayload` | `Subscription` |
| `projects.subscribers.subscriptions.delete` | `DELETE /v4/projects/{project}/subscribers/{subscriber}/subscriptions/{subscription}` | `Empty` |
| `projects.subscribers.subscriptions.list` | `GET /v4/projects/{project}/subscribers/{subscriber}/subscriptions`, `filter`, `pageSize`, `pageToken` | `ListSubscriptionsResponse` |
| `projects.subscribers.subscriptions.patch` | `PATCH /v4/projects/{project}/subscribers/{subscriber}/subscriptions/{subscription}`, `updateMask`, `Subscription` | `Subscription` |
| `users.getIdentity` | `GET /v4/users/me/identity` | `Identity` |
| `users.getIrnProfile` | `GET /v4/users/me/irnProfile` | `IrnProfile` |
| `users.getProfile` | `GET /v4/users/me/profile` | `Profile` |
| `users.getSettings` | `GET /v4/users/me/settings` | `Settings` |
| `users.pairedDevices.get` | `GET /v4/users/me/pairedDevices/{device}` | `PairedDevice` |
| `users.pairedDevices.list` | `GET /v4/users/me/pairedDevices`, `pageSize`, `pageToken` | `ListPairedDevicesResponse` |
| `users.updateProfile` | `PATCH /v4/users/me/profile`, `updateMask`, `Profile` | `Profile` |
| `users.updateSettings` | `PATCH /v4/users/me/settings`, `updateMask`, `Settings` | `Settings` |
| `users.dataTypes.dataPoints.batchDelete` | `POST /v4/users/me/dataTypes/{type}/dataPoints:batchDelete`, `BatchDeleteDataPointsRequest` | `Operation` |
| `users.dataTypes.dataPoints.create` | `POST /v4/users/me/dataTypes/{type}/dataPoints`, `DataPoint` | `Operation` |
| `users.dataTypes.dataPoints.dailyRollUp` | `POST /v4/users/me/dataTypes/{type}/dataPoints:dailyRollUp`, `DailyRollUpDataPointsRequest` | `DailyRollUpDataPointsResponse` |
| `users.dataTypes.dataPoints.exportExerciseTcx` | `GET /v4/users/me/dataTypes/exercise/dataPoints/{id}:exportExerciseTcx`, `alt=media`, `partialData`; activity-and-fitness and location read scopes | Raw TCX bytes |
| `users.dataTypes.dataPoints.get` | `GET /v4/users/me/dataTypes/{type}/dataPoints/{id}` | `DataPoint` |
| `users.dataTypes.dataPoints.list` | `GET /v4/users/me/dataTypes/{type}/dataPoints`, `filter`, `pageSize`, `pageToken` | `ListDataPointsResponse` |
| `users.dataTypes.dataPoints.patch` | `PATCH /v4/users/me/dataTypes/{type}/dataPoints/{id}`, `DataPoint` | `Operation` |
| `users.dataTypes.dataPoints.reconcile` | `GET /v4/users/me/dataTypes/{type}/dataPoints:reconcile`, `filter`, `pageSize`, `pageToken`, `dataSourceFamily` | `ReconcileDataPointsResponse` |
| `users.dataTypes.dataPoints.rollUp` | `POST /v4/users/me/dataTypes/{type}/dataPoints:rollUp`, `RollUpDataPointsRequest` | `RollUpDataPointsResponse` |

## Contract Fixtures

- Request tests assert HTTP method, escaped resource path, supported query parameters, request JSON, and required confirmation boundaries.
- Response tests cover typed resources, list envelopes, pagination tokens, long-running operations, empty responses, and raw TCX media.
- HTTP 4xx/5xx responses become `APIError` values with status and provider details.
- Malformed JSON responses fail with a decode error.
- The registry suite is aligned with 39 publicly documented data types and 25 publicly documented REST methods.
- OAuth fixtures cover the current `.readonly` and `.writeonly` scope names; TCX coverage includes the additional location scope.

## Discovery-Only Observations

The v4 discovery document revision `20260730` also contains two SHL methods:

- `POST /v4/shl/m/{externalShlId}`
- `GET /v4/shl/r/{externalShlId}/{resourceToken}`

It also contains `basalEnergyBurned` / `basal-energy-burned` with list and
reconcile support. These additions are not yet listed in the public REST or
data-type navigation. `ghealth api --no-auth --raw` supports public HttpBody
responses without treating them as JSON; all three observations remain
excluded from documented typed coverage until Google publishes stable public
contracts.

Sources: [REST reference](https://developers.google.com/health/reference/rest), [data types](https://developers.google.com/health/data-types), and the [v4 discovery document](https://health.googleapis.com/$discovery/rest?version=v4).
