# Endpoint Coverage

The canonical coverage table is `internal/healthapi/client_test.go`.

## Covered v4 Methods

| Method | Request Coverage | Response Coverage |
| --- | --- | --- |
| `projects.subscribers.create` | `POST /v4/projects/{project}/subscribers`, `subscriberId`, `CreateSubscriberPayload` | `Operation` |
| `projects.subscribers.delete` | `DELETE /v4/projects/{project}/subscribers/{subscriber}`, `force` | `Operation` |
| `projects.subscribers.list` | `GET /v4/projects/{project}/subscribers`, `pageSize`, `pageToken` | `ListSubscribersResponse` |
| `projects.subscribers.patch` | `PATCH /v4/projects/{project}/subscribers/{subscriber}`, `updateMask`, `Subscriber` | `Operation` |
| `users.getIdentity` | `GET /v4/users/me/identity` | `Identity` |
| `users.getProfile` | `GET /v4/users/me/profile` | `Profile` |
| `users.getSettings` | `GET /v4/users/me/settings` | `Settings` |
| `users.updateProfile` | `PATCH /v4/users/me/profile`, `updateMask`, `Profile` | `Profile` |
| `users.updateSettings` | `PATCH /v4/users/me/settings`, `updateMask`, `Settings` | `Settings` |
| `users.dataTypes.dataPoints.batchDelete` | `POST /v4/users/me/dataTypes/{type}/dataPoints:batchDelete`, `BatchDeleteDataPointsRequest` | `Operation` |
| `users.dataTypes.dataPoints.create` | `POST /v4/users/me/dataTypes/{type}/dataPoints`, `DataPoint` | `Operation` |
| `users.dataTypes.dataPoints.dailyRollUp` | `POST /v4/users/me/dataTypes/{type}/dataPoints:dailyRollUp`, `DailyRollUpDataPointsRequest` | `DailyRollUpDataPointsResponse` |
| `users.dataTypes.dataPoints.exportExerciseTcx` | `GET /v4/users/me/dataTypes/exercise/dataPoints/{id}:exportExerciseTcx`, `partialData` | `ExportExerciseTcxResponse` bytes |
| `users.dataTypes.dataPoints.get` | `GET /v4/users/me/dataTypes/{type}/dataPoints/{id}` | `DataPoint` |
| `users.dataTypes.dataPoints.list` | `GET /v4/users/me/dataTypes/{type}/dataPoints`, `filter`, `pageSize`, `pageToken`, `view` | `ListDataPointsResponse` |
| `users.dataTypes.dataPoints.patch` | `PATCH /v4/users/me/dataTypes/{type}/dataPoints/{id}`, `DataPoint` | `Operation` |
| `users.dataTypes.dataPoints.reconcile` | `GET /v4/users/me/dataTypes/{type}/dataPoints:reconcile`, `filter`, `pageSize`, `pageToken`, `dataSourceFamily` | `ReconcileDataPointsResponse` |
| `users.dataTypes.dataPoints.rollUp` | `POST /v4/users/me/dataTypes/{type}/dataPoints:rollUp`, `RollUpDataPointsRequest` | `RollUpDataPointsResponse` |

## Non-Live Cases

- HTTP 4xx/5xx responses become `APIError` with status code and body.
- Malformed JSON responses fail with a decode error.
- The test suite fails if the number of endpoint cases differs from the 18 methods in the internal registry.
- The registry suite fails if the documented surface drifts away from 31 data types or 18 REST methods.
