package healthapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/Google-Health-CLI/internal/registry"
)

type endpointCase struct {
	name          string
	operationName string
	method        string
	path          string
	query         url.Values
	request       map[string]any
	response      string
	call          func(context.Context, *Client) (any, error)
	assert        func(*testing.T, any)
}

func TestClientCoversAllDocumentedRESTEndpoints(t *testing.T) {
	cases := endpointCases()
	if got, want := len(cases), len(registry.RESTOperations()); got != want {
		t.Fatalf("endpoint cases = %d, documented operations = %d", got, want)
	}

	covered := map[string]bool{}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			covered[tc.operationName] = true
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.method {
					t.Fatalf("method = %s, want %s", r.Method, tc.method)
				}
				if r.URL.Path != tc.path {
					t.Fatalf("path = %s, want %s", r.URL.Path, tc.path)
				}
				if !reflect.DeepEqual(r.URL.Query(), tc.query) {
					t.Fatalf("query = %#v, want %#v", r.URL.Query(), tc.query)
				}
				assertRequestBody(t, r, tc.request)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.response))
			}))
			defer server.Close()

			client := New(server.URL, "users/me", server.Client())
			value, err := tc.call(context.Background(), client)
			if err != nil {
				t.Fatalf("call failed: %v", err)
			}
			if tc.assert != nil {
				tc.assert(t, value)
			}
		})
	}

	for _, op := range registry.RESTOperations() {
		if !covered[op.Name] {
			t.Fatalf("documented operation %q has no client test case", op.Name)
		}
	}
}

func TestClientAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":{"code":403,"message":"missing scope"}}`, http.StatusForbidden)
	}))
	defer server.Close()

	client := New(server.URL, "users/me", server.Client())
	_, err := client.GetProfile(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", apiErr.StatusCode, http.StatusForbidden)
	}
	if !strings.Contains(apiErr.Body, "missing scope") {
		t.Fatalf("body = %q", apiErr.Body)
	}
}

func TestClientRejectsInvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{nope"))
	}))
	defer server.Close()

	client := New(server.URL, "users/me", server.Client())
	_, err := client.GetProfile(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("error = %v, want decode response error", err)
	}
}

func endpointCases() []endpointCase {
	profile := map[string]any{"name": "users/me/profile", "userConfiguredWalkingStrideLengthMm": float64(740)}
	settings := map[string]any{"name": "users/me/settings", "temperatureUnit": "TEMPERATURE_UNIT_CELSIUS"}
	subscriberCreate := map[string]any{
		"endpointUri":           "https://example.com/google-health",
		"endpointAuthorization": map[string]any{"secret": "Bearer test-secret"},
		"subscriberConfigs":     []any{map[string]any{"dataType": "steps"}},
	}
	subscriber := map[string]any{
		"name":              "projects/project-123/subscribers/my-sub-123",
		"endpointUri":       "https://example.com/google-health-v2",
		"subscriberConfigs": []any{map[string]any{"dataType": "sleep"}},
	}
	weightPoint := map[string]any{
		"name": "users/me/dataTypes/weight/dataPoints/weight-123",
		"weight": map[string]any{
			"sampleTime":  map[string]any{"physicalTime": "2026-05-09T06:00:00Z"},
			"weightGrams": float64(78000),
		},
	}
	batchDelete := map[string]any{"names": []any{"users/me/dataTypes/weight/dataPoints/weight-123"}}
	dailyRollup := map[string]any{
		"range": map[string]any{
			"start": map[string]any{"date": map[string]any{"year": float64(2026), "month": float64(5), "day": float64(8)}},
			"end":   map[string]any{"date": map[string]any{"year": float64(2026), "month": float64(5), "day": float64(9)}},
		},
		"windowSizeDays":   float64(1),
		"pageSize":         float64(100),
		"pageToken":        "daily-next",
		"dataSourceFamily": "users/me/dataSourceFamilies/all-sources",
	}
	physicalRollup := map[string]any{
		"range":            map[string]any{"startTime": "2026-05-08T00:00:00Z", "endTime": "2026-05-09T00:00:00Z"},
		"windowSize":       "3600s",
		"pageSize":         float64(100),
		"pageToken":        "physical-next",
		"dataSourceFamily": "users/me/dataSourceFamilies/google-wearables",
	}

	return []endpointCase{
		{
			name:          "projects.subscribers.create",
			operationName: "projects.subscribers.create",
			method:        http.MethodPost,
			path:          "/v4/projects/project-123/subscribers",
			query:         values("subscriberId", "my-sub-123"),
			request:       subscriberCreate,
			response:      operationResponse("subscriber-create"),
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.CreateSubscriberWithOptions(ctx, "project-123", subscriberCreate, CreateSubscriberOptions{SubscriberID: "my-sub-123"})
			},
			assert: assertOperation("operations/subscriber-create"),
		},
		{
			name:          "projects.subscribers.delete",
			operationName: "projects.subscribers.delete",
			method:        http.MethodDelete,
			path:          "/v4/projects/project-123/subscribers/my-sub-123",
			query:         values("force", "true"),
			response:      operationResponse("subscriber-delete"),
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.DeleteSubscriberWithOptions(ctx, "projects/project-123/subscribers/my-sub-123", DeleteSubscriberOptions{Force: true})
			},
			assert: assertOperation("operations/subscriber-delete"),
		},
		{
			name:          "projects.subscribers.list",
			operationName: "projects.subscribers.list",
			method:        http.MethodGet,
			path:          "/v4/projects/project-123/subscribers",
			query:         values("pageSize", "50", "pageToken", "sub-next"),
			response:      `{"subscribers":[{"name":"projects/project-123/subscribers/my-sub-123","state":"ACTIVE"}],"nextPageToken":"sub-next-2","totalSize":1}`,
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.ListSubscribersWithOptions(ctx, "projects/project-123", SubscriberListOptions{PageSize: 50, PageToken: "sub-next"})
			},
			assert: assertField("nextPageToken", "sub-next-2"),
		},
		{
			name:          "projects.subscribers.patch",
			operationName: "projects.subscribers.patch",
			method:        http.MethodPatch,
			path:          "/v4/projects/project-123/subscribers/my-sub-123",
			query:         values("updateMask", "endpoint_uri,subscriber_configs"),
			request:       subscriber,
			response:      operationResponse("subscriber-patch"),
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.PatchSubscriberWithOptions(ctx, "projects/project-123/subscribers/my-sub-123", subscriber, PatchSubscriberOptions{UpdateMask: "endpoint_uri,subscriber_configs"})
			},
			assert: assertOperation("operations/subscriber-patch"),
		},
		{
			name:          "users.getIdentity",
			operationName: "users.getIdentity",
			method:        http.MethodGet,
			path:          "/v4/users/me/identity",
			query:         url.Values{},
			response:      `{"name":"users/me/identity","healthUserId":"health-user-123","legacyUserId":"fitbit-user-123"}`,
			call:          func(ctx context.Context, c *Client) (any, error) { return c.GetIdentity(ctx) },
			assert:        assertField("healthUserId", "health-user-123"),
		},
		{
			name:          "users.getProfile",
			operationName: "users.getProfile",
			method:        http.MethodGet,
			path:          "/v4/users/me/profile",
			query:         url.Values{},
			response:      `{"name":"users/me/profile","age":35}`,
			call:          func(ctx context.Context, c *Client) (any, error) { return c.GetProfile(ctx) },
			assert:        assertField("age", float64(35)),
		},
		{
			name:          "users.getSettings",
			operationName: "users.getSettings",
			method:        http.MethodGet,
			path:          "/v4/users/me/settings",
			query:         url.Values{},
			response:      `{"name":"users/me/settings","temperatureUnit":"TEMPERATURE_UNIT_CELSIUS"}`,
			call:          func(ctx context.Context, c *Client) (any, error) { return c.GetSettings(ctx) },
			assert:        assertField("temperatureUnit", "TEMPERATURE_UNIT_CELSIUS"),
		},
		{
			name:          "users.updateProfile",
			operationName: "users.updateProfile",
			method:        http.MethodPatch,
			path:          "/v4/users/me/profile",
			query:         values("updateMask", "user_configured_walking_stride_length_mm"),
			request:       profile,
			response:      `{"name":"users/me/profile","userConfiguredWalkingStrideLengthMm":740}`,
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.UpdateProfileWithOptions(ctx, profile, UpdateOptions{UpdateMask: "user_configured_walking_stride_length_mm"})
			},
			assert: assertField("userConfiguredWalkingStrideLengthMm", float64(740)),
		},
		{
			name:          "users.updateSettings",
			operationName: "users.updateSettings",
			method:        http.MethodPatch,
			path:          "/v4/users/me/settings",
			query:         values("updateMask", "temperature_unit"),
			request:       settings,
			response:      `{"name":"users/me/settings","temperatureUnit":"TEMPERATURE_UNIT_CELSIUS"}`,
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.UpdateSettingsWithOptions(ctx, settings, UpdateOptions{UpdateMask: "temperature_unit"})
			},
			assert: assertField("temperatureUnit", "TEMPERATURE_UNIT_CELSIUS"),
		},
		{
			name:          "users.dataTypes.dataPoints.batchDelete",
			operationName: "users.dataTypes.dataPoints.batchDelete",
			method:        http.MethodPost,
			path:          "/v4/users/me/dataTypes/weight/dataPoints:batchDelete",
			query:         url.Values{},
			request:       batchDelete,
			response:      operationResponse("batch-delete"),
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.BatchDeleteDataPoints(ctx, "weight", batchDelete)
			},
			assert: assertOperation("operations/batch-delete"),
		},
		{
			name:          "users.dataTypes.dataPoints.create",
			operationName: "users.dataTypes.dataPoints.create",
			method:        http.MethodPost,
			path:          "/v4/users/me/dataTypes/weight/dataPoints",
			query:         url.Values{},
			request:       weightPoint,
			response:      operationResponse("data-point-create"),
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.CreateDataPoint(ctx, "weight", weightPoint)
			},
			assert: assertOperation("operations/data-point-create"),
		},
		{
			name:          "users.dataTypes.dataPoints.dailyRollUp",
			operationName: "users.dataTypes.dataPoints.dailyRollUp",
			method:        http.MethodPost,
			path:          "/v4/users/me/dataTypes/steps/dataPoints:dailyRollUp",
			query:         url.Values{},
			request:       dailyRollup,
			response:      `{"rollupDataPoints":[{"date":{"year":2026,"month":5,"day":8},"steps":{"count":12345}}]}`,
			call:          func(ctx context.Context, c *Client) (any, error) { return c.DailyRollUp(ctx, "steps", dailyRollup) },
			assert:        assertArray("rollupDataPoints", 1),
		},
		{
			name:          "users.dataTypes.dataPoints.exportExerciseTcx",
			operationName: "users.dataTypes.dataPoints.exportExerciseTcx",
			method:        http.MethodGet,
			path:          "/v4/users/me/dataTypes/exercise/dataPoints/exercise-123:exportExerciseTcx",
			query:         values("partialData", "true"),
			response:      `{"tcxData":"<TrainingCenterDatabase />"}`,
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.ExportExerciseTCXWithOptions(ctx, "exercise-123", ExportExerciseTCXOptions{PartialData: true})
			},
			assert: func(t *testing.T, value any) {
				bytes, ok := value.([]byte)
				if !ok {
					t.Fatalf("value = %T, want []byte", value)
				}
				if !strings.Contains(string(bytes), "TrainingCenterDatabase") {
					t.Fatalf("TCX response = %q", string(bytes))
				}
			},
		},
		{
			name:          "users.dataTypes.dataPoints.get",
			operationName: "users.dataTypes.dataPoints.get",
			method:        http.MethodGet,
			path:          "/v4/users/me/dataTypes/sleep/dataPoints/sleep-123",
			query:         url.Values{},
			response:      `{"name":"users/me/dataTypes/sleep/dataPoints/sleep-123","sleep":{"interval":{"startTime":"2026-05-08T22:00:00Z","endTime":"2026-05-09T06:00:00Z"}}}`,
			call:          func(ctx context.Context, c *Client) (any, error) { return c.GetDataPoint(ctx, "sleep", "sleep-123") },
			assert:        assertField("name", "users/me/dataTypes/sleep/dataPoints/sleep-123"),
		},
		{
			name:          "users.dataTypes.dataPoints.list",
			operationName: "users.dataTypes.dataPoints.list",
			method:        http.MethodGet,
			path:          "/v4/users/me/dataTypes/steps/dataPoints",
			query:         values("filter", `steps.interval.civil_start_time >= "2026-05-08" AND steps.interval.civil_start_time < "2026-05-09"`, "pageSize", "25", "pageToken", "list-next", "view", "FULL"),
			response:      `{"dataPoints":[{"steps":{"interval":{"civilStartTime":"2026-05-08"},"count":12345}}],"nextPageToken":"list-next-2"}`,
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.ListDataPoints(ctx, "steps", ListOptions{Filter: `steps.interval.civil_start_time >= "2026-05-08" AND steps.interval.civil_start_time < "2026-05-09"`, PageSize: 25, PageToken: "list-next", View: "FULL"})
			},
			assert: assertField("nextPageToken", "list-next-2"),
		},
		{
			name:          "users.dataTypes.dataPoints.patch",
			operationName: "users.dataTypes.dataPoints.patch",
			method:        http.MethodPatch,
			path:          "/v4/users/me/dataTypes/weight/dataPoints/weight-123",
			query:         url.Values{},
			request:       weightPoint,
			response:      operationResponse("data-point-patch"),
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.PatchDataPoint(ctx, "weight", "weight-123", weightPoint)
			},
			assert: assertOperation("operations/data-point-patch"),
		},
		{
			name:          "users.dataTypes.dataPoints.reconcile",
			operationName: "users.dataTypes.dataPoints.reconcile",
			method:        http.MethodGet,
			path:          "/v4/users/me/dataTypes/heart-rate/dataPoints:reconcile",
			query:         values("filter", `heart_rate.sample_time.physical_time >= "2026-05-08T00:00:00Z"`, "pageSize", "10", "pageToken", "reconcile-next", "dataSourceFamily", "users/me/dataSourceFamilies/google-wearables"),
			response:      `{"dataPoints":[{"heartRate":{"sampleTime":{"physicalTime":"2026-05-08T00:00:00Z"},"beatsPerMinute":61}}],"nextPageToken":"reconcile-next-2"}`,
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.ReconcileDataPoints(ctx, "heart-rate", ReconcileOptions{Filter: `heart_rate.sample_time.physical_time >= "2026-05-08T00:00:00Z"`, PageSize: 10, PageToken: "reconcile-next", DataSourceFamily: "users/me/dataSourceFamilies/google-wearables"})
			},
			assert: assertField("nextPageToken", "reconcile-next-2"),
		},
		{
			name:          "users.dataTypes.dataPoints.rollUp",
			operationName: "users.dataTypes.dataPoints.rollUp",
			method:        http.MethodPost,
			path:          "/v4/users/me/dataTypes/steps/dataPoints:rollUp",
			query:         url.Values{},
			request:       physicalRollup,
			response:      `{"rollupDataPoints":[{"range":{"startTime":"2026-05-08T00:00:00Z","endTime":"2026-05-08T01:00:00Z"},"steps":{"count":501}}],"nextPageToken":"rollup-next"}`,
			call:          func(ctx context.Context, c *Client) (any, error) { return c.RollUp(ctx, "steps", physicalRollup) },
			assert:        assertField("nextPageToken", "rollup-next"),
		},
	}
}

func assertRequestBody(t *testing.T, r *http.Request, want map[string]any) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	if want == nil {
		if len(strings.TrimSpace(string(body))) != 0 {
			t.Fatalf("body = %s, want empty", string(body))
		}
		return
	}
	if got := r.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode request body %q: %v", string(body), err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("body = %#v, want %#v", got, want)
	}
}

func values(items ...string) url.Values {
	values := url.Values{}
	for i := 0; i < len(items); i += 2 {
		values.Set(items[i], items[i+1])
	}
	return values
}

func operationResponse(id string) string {
	return `{"name":"operations/` + id + `","done":true,"response":{}}`
}

func assertOperation(name string) func(*testing.T, any) {
	return assertField("name", name)
}

func assertField(field string, want any) func(*testing.T, any) {
	return func(t *testing.T, value any) {
		t.Helper()
		gotMap, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("value = %T, want map[string]any", value)
		}
		if got := gotMap[field]; !reflect.DeepEqual(got, want) {
			t.Fatalf("%s = %#v, want %#v", field, got, want)
		}
	}
}

func assertArray(field string, wantLen int) func(*testing.T, any) {
	return func(t *testing.T, value any) {
		t.Helper()
		gotMap, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("value = %T, want map[string]any", value)
		}
		items, ok := gotMap[field].([]any)
		if !ok {
			t.Fatalf("%s = %T, want []any", field, gotMap[field])
		}
		if len(items) != wantLen {
			t.Fatalf("%s length = %d, want %d", field, len(items), wantLen)
		}
	}
}
