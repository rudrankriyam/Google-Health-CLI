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
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestNewAppliesDefaultRequestTimeout(t *testing.T) {
	client := New("", "", &http.Client{})
	if client.HTTPClient.Timeout != 60*time.Second {
		t.Fatalf("timeout = %s, want 60s", client.HTTPClient.Timeout)
	}

	configured := &http.Client{Timeout: 5 * time.Second}
	client = New("", "", configured)
	if client.HTTPClient != configured || client.HTTPClient.Timeout != 5*time.Second {
		t.Fatalf("configured client was not preserved: %#v", client.HTTPClient)
	}
}

func TestReadBoundedRejectsOversizedResponse(t *testing.T) {
	_, err := readBounded(strings.NewReader("123456"), 5)
	if err == nil || !strings.Contains(err.Error(), "exceeds 5 bytes") {
		t.Fatalf("error = %v, want response-size error", err)
	}

	value, err := readBounded(strings.NewReader("12345"), 5)
	if err != nil || string(value) != "12345" {
		t.Fatalf("value = %q, error = %v", string(value), err)
	}
}

func TestRawBytesPreservesNonJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Accept"); got != "*/*" {
			t.Errorf("Accept = %q, want */*", got)
		}
		writer.Header().Set("Content-Type", "application/octet-stream")
		_, _ = writer.Write([]byte{0x00, 0x01, 0xfe, 0xff})
	}))
	defer server.Close()

	client := New(server.URL, "users/me", server.Client())
	value, err := client.RawBytes(context.Background(), http.MethodGet, "/v4/shl/r/example/token", nil, nil)
	if err != nil {
		t.Fatalf("RawBytes: %v", err)
	}
	want := []byte{0x00, 0x01, 0xfe, 0xff}
	if !reflect.DeepEqual(value, want) {
		t.Fatalf("value = %v, want %v", value, want)
	}
}

type endpointCase struct {
	name          string
	operationName string
	method        string
	path          string
	query         url.Values
	request       json.RawMessage
	response      string
	contentType   string
	accept        string
	call          func(context.Context, *Client) (any, error)
	assert        func(*testing.T, any)
}

func TestClientCoversAllDocumentedRESTEndpoints(t *testing.T) {
	cases := endpointCases()
	if got, want := len(cases), 25; got != want {
		t.Fatalf("endpoint cases = %d, current public operations = %d", got, want)
	}

	covered := make(map[string]bool, len(cases))
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			if covered[testCase.operationName] {
				t.Fatalf("operation %q is covered more than once", testCase.operationName)
			}
			covered[testCase.operationName] = true
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != testCase.method {
					t.Fatalf("method = %s, want %s", request.Method, testCase.method)
				}
				if request.URL.Path != testCase.path {
					t.Fatalf("path = %s, want %s", request.URL.Path, testCase.path)
				}
				if !reflect.DeepEqual(request.URL.Query(), testCase.query) {
					t.Fatalf("query = %#v, want %#v", request.URL.Query(), testCase.query)
				}
				wantAccept := testCase.accept
				if wantAccept == "" {
					wantAccept = "application/json"
				}
				if got := request.Header.Get("Accept"); got != wantAccept {
					t.Fatalf("Accept = %q, want %q", got, wantAccept)
				}
				assertRequestBody(t, request, testCase.request)
				contentType := testCase.contentType
				if contentType == "" {
					contentType = "application/json"
				}
				writer.Header().Set("Content-Type", contentType)
				_, _ = writer.Write([]byte(testCase.response))
			}))
			defer server.Close()

			client := New(server.URL, "users/me", server.Client())
			value, err := testCase.call(context.Background(), client)
			if err != nil {
				t.Fatalf("call failed: %v", err)
			}
			if testCase.assert != nil {
				testCase.assert(t, value)
			}
		})
	}
}

func TestClientParsesStructuredAPIErrorWithoutExposingRawBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusForbidden)
		_, _ = writer.Write([]byte(`{
			"error": {
				"code": 403,
				"message": "missing scope",
				"status": "PERMISSION_DENIED",
				"details": [{
					"@type": "type.googleapis.com/google.rpc.ErrorInfo",
					"reason": "ACCESS_TOKEN_SCOPE_INSUFFICIENT",
					"domain": "googleapis.com",
					"sensitive": "do-not-print"
				}]
			}
		}`))
	}))
	defer server.Close()

	client := New(server.URL, "users/me", server.Client())
	_, err := client.GetProfile(context.Background())
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("error = %T, want *APIError", err)
	}
	if apiError.StatusCode != http.StatusForbidden || apiError.Code != http.StatusForbidden {
		t.Fatalf("status/code = %d/%d", apiError.StatusCode, apiError.Code)
	}
	if apiError.APIStatus != "PERMISSION_DENIED" || apiError.Message != "missing scope" {
		t.Fatalf("status/message = %q/%q", apiError.APIStatus, apiError.Message)
	}
	if apiError.Reason != "ACCESS_TOKEN_SCOPE_INSUFFICIENT" || apiError.Domain != "googleapis.com" {
		t.Fatalf("reason/domain = %q/%q", apiError.Reason, apiError.Domain)
	}
	if strings.Contains(err.Error(), "do-not-print") || strings.Contains(err.Error(), "details") {
		t.Fatalf("error exposes raw response: %q", err)
	}
	if apiError.Retryable() {
		t.Fatal("403 error unexpectedly marked retryable")
	}
}

func TestClientSanitizesNonJSONAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "provider secret: do-not-print", http.StatusBadGateway)
	}))
	defer server.Close()

	client := New(server.URL, "users/me", server.Client())
	_, err := client.GetProfile(context.Background())
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("error = %T, want *APIError", err)
	}
	if strings.Contains(err.Error(), "do-not-print") {
		t.Fatalf("error exposes raw response: %q", err)
	}
	if apiError.Message != http.StatusText(http.StatusBadGateway) || !apiError.Retryable() {
		t.Fatalf("message/retryable = %q/%t", apiError.Message, apiError.Retryable())
	}
}

func TestClientReturnsEmbeddedOperationFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"name":"operations/data-point-create",
			"done":true,
			"error":{"code":9,"message":"write rejected","status":"FAILED_PRECONDITION"}
		}`))
	}))
	defer server.Close()

	client := New(server.URL, "users/me", server.Client())
	operation, err := client.CreateDataPoint(context.Background(), "weight", json.RawMessage(`{"weight":{}}`))
	if operation.Name != "operations/data-point-create" || !operation.IsDone() {
		t.Fatalf("operation = %#v", operation)
	}
	var operationError *OperationError
	if !errors.As(err, &operationError) {
		t.Fatalf("error = %T, want *OperationError", err)
	}
	if operationError.Status.Status != "FAILED_PRECONDITION" || !strings.Contains(err.Error(), "write rejected") {
		t.Fatalf("operation error = %#v", operationError)
	}
}

func TestDecodeOperationResponse(t *testing.T) {
	done := true
	operation := Operation{
		Name:     "operations/subscriber-create",
		Done:     &done,
		Response: json.RawMessage(`{"name":"projects/example/subscribers/example","state":"ACTIVE"}`),
	}
	value, err := DecodeOperationResponse[Subscriber](operation)
	if err != nil {
		t.Fatal(err)
	}
	if value.Name != "projects/example/subscribers/example" || value.State != SubscriberStateActive {
		t.Fatalf("decoded response = %#v", value)
	}
}

func TestClientRejectsInvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("{nope"))
	}))
	defer server.Close()

	client := New(server.URL, "users/me", server.Client())
	_, err := client.GetProfile(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("error = %v, want decode response error", err)
	}
}

func TestClientRejectsUnsafeResourcePathsBeforeTransport(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()

	tests := []struct {
		name string
		call func(*Client) error
	}{
		{
			name: "invalid configured user",
			call: func(client *Client) error {
				client.User = "users/../profile"
				_, err := client.GetProfile(context.Background())
				return err
			},
		},
		{
			name: "data point traversal",
			call: func(client *Client) error {
				_, err := client.GetDataPoint(context.Background(), "weight", "../settings")
				return err
			},
		},
		{
			name: "subscriber traversal",
			call: func(client *Client) error {
				_, err := client.DeleteSubscriber(context.Background(), "projects/example/subscribers/../users")
				return err
			},
		},
		{
			name: "raw absolute URL",
			call: func(client *Client) error {
				_, err := client.Raw(context.Background(), http.MethodGet, "https://example.com/v4/users/me/profile", nil, nil)
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := New(server.URL, "users/me", server.Client())
			if err := test.call(client); err == nil {
				t.Fatal("unsafe path accepted")
			}
		})
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("transport requests = %d, want 0", got)
	}
}

func TestClientRestrictsBaseURLToGoogleHealthOrLiteralLoopback(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		allowed bool
	}{
		{name: "HTTPS", baseURL: "https://health.googleapis.com", allowed: true},
		{name: "IPv4 loopback", baseURL: "http://127.0.0.1:8080", allowed: true},
		{name: "IPv6 loopback", baseURL: "http://[::1]:8080", allowed: true},
		{name: "HTTPS loopback", baseURL: "https://127.0.0.1:8443", allowed: true},
		{name: "localhost hostname", baseURL: "http://localhost:8080", allowed: false},
		{name: "non-loopback HTTP", baseURL: "http://example.com", allowed: false},
		{name: "arbitrary HTTPS", baseURL: "https://example.com", allowed: false},
		{name: "Google Health alternate port", baseURL: "https://health.googleapis.com:444", allowed: false},
		{name: "Google Health base path", baseURL: "https://health.googleapis.com/v4", allowed: false},
		{name: "unsupported scheme", baseURL: "ftp://health.googleapis.com", allowed: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var requests atomic.Int32
			transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
				requests.Add(1)
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"name":"users/me/profile"}`)),
					Request:    request,
				}, nil
			})
			client := New(test.baseURL, "users/me", &http.Client{Transport: transport})
			_, err := client.GetProfile(context.Background())
			if test.allowed && err != nil {
				t.Fatalf("allowed base URL rejected: %v", err)
			}
			if !test.allowed && err == nil {
				t.Fatal("insecure base URL accepted")
			}
			wantRequests := int32(0)
			if test.allowed {
				wantRequests = 1
			}
			if got := requests.Load(); got != wantRequests {
				t.Fatalf("transport requests = %d, want %d", got, wantRequests)
			}
		})
	}
}

func TestRejectedHTTPSHostReceivesNoOAuthTokenOrRequest(t *testing.T) {
	var requests atomic.Int32
	var authorizationHeaders atomic.Int32
	baseTransport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests.Add(1)
		if request.Header.Get("Authorization") != "" {
			authorizationHeaders.Add(1)
		}
		return nil, errors.New("transport must not run")
	})
	oauthTransport := &oauth2.Transport{
		Source: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "never-send-this-token"}),
		Base:   baseTransport,
	}
	client := New("https://attacker.example", "users/me", &http.Client{Transport: oauthTransport})
	_, err := client.GetProfile(context.Background())
	if err == nil {
		t.Fatal("arbitrary authenticated HTTPS host accepted")
	}
	if strings.Contains(err.Error(), "never-send-this-token") {
		t.Fatalf("error leaks access token: %q", err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("transport requests = %d, want 0", got)
	}
	if got := authorizationHeaders.Load(); got != 0 {
		t.Fatalf("authorization headers attached = %d, want 0", got)
	}
}

func TestClientRejectsBaseURLSecretsWithoutLeakingThem(t *testing.T) {
	credentialURL := (&url.URL{
		Scheme: "https",
		Host:   "health.googleapis.com",
		User:   url.User("api-user"),
	}).String()
	tests := []struct {
		name       string
		baseURL    string
		secretText []string
	}{
		{
			name:       "userinfo",
			baseURL:    credentialURL,
			secretText: []string{"api-user"},
		},
		{
			name:       "query",
			baseURL:    "https://health.googleapis.com?access_token=query-secret",
			secretText: []string{"access_token", "query-secret"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var requests atomic.Int32
			client := New(test.baseURL, "users/me", &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				requests.Add(1)
				return nil, errors.New("transport must not run")
			})})
			_, err := client.GetProfile(context.Background())
			if err == nil {
				t.Fatal("unsafe base URL accepted")
			}
			for _, secret := range test.secretText {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("error leaks %q: %q", secret, err)
				}
			}
			if got := requests.Load(); got != 0 {
				t.Fatalf("transport requests = %d, want 0", got)
			}
		})
	}
}

func TestTransportErrorRedactsRequestSecretsAndPreservesCause(t *testing.T) {
	requestURL := (&url.URL{
		Scheme:   "https",
		Host:     "health.googleapis.com",
		User:     url.User("api-user"),
		Path:     "/v4/users/me/profile",
		RawQuery: "access_token=query-secret",
	}).String()
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, &url.Error{
			Op:  http.MethodGet,
			URL: requestURL,
			Err: context.DeadlineExceeded,
		}
	})
	client := New("https://health.googleapis.com", "users/me", &http.Client{Transport: transport})
	_, err := client.Raw(
		context.Background(),
		http.MethodGet,
		"/v4/users/me/profile",
		url.Values{"access_token": []string{"query-secret"}},
		nil,
	)
	var transportError *TransportError
	if !errors.As(err, &transportError) {
		t.Fatalf("error = %T, want *TransportError", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error does not preserve deadline cause: %v", err)
	}
	if got := err.Error(); got != "Google Health request timed out" {
		t.Fatalf("error = %q", got)
	}
	for _, secret := range []string{"api-user", "access_token", "query-secret", "health.googleapis.com"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("transport error leaks %q: %q", secret, err)
		}
	}
}

func TestFullDataPointNameMustMatchConfiguredUserAndRequestedType(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.URL.Path != "/v4/users/me/dataTypes/weight/dataPoints/weight-123" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		_, _ = writer.Write([]byte(`{"name":"users/me/dataTypes/weight/dataPoints/weight-123"}`))
	}))
	defer server.Close()

	client := New(server.URL, "users/me", server.Client())
	if _, err := client.GetDataPoint(context.Background(), "weight", "users/me/dataTypes/weight/dataPoints/weight-123"); err != nil {
		t.Fatalf("matching full name rejected: %v", err)
	}
	for _, test := range []struct {
		name     string
		dataType string
		resource string
	}{
		{
			name:     "different user",
			dataType: "weight",
			resource: "users/other/dataTypes/weight/dataPoints/weight-123",
		},
		{
			name:     "different data type",
			dataType: "weight",
			resource: "users/me/dataTypes/sleep/dataPoints/sleep-123",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := client.GetDataPoint(context.Background(), test.dataType, test.resource); err == nil {
				t.Fatal("mismatched full resource name accepted")
			}
		})
	}
	for _, names := range [][]string{
		{"users/other/dataTypes/weight/dataPoints/weight-123"},
		{"users/me/dataTypes/sleep/dataPoints/sleep-123"},
		{"weight-123"},
	} {
		if _, err := client.BatchDeleteDataPoints(context.Background(), "weight", BatchDeleteDataPointsRequest{Names: names}); err == nil {
			t.Fatalf("unsafe batch-delete names accepted: %q", names)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("transport requests = %d, want 1", got)
	}
}

func TestFullPairedDeviceNameMustMatchConfiguredUser(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.URL.Path != "/v4/users/me/pairedDevices/device-123" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		_, _ = writer.Write([]byte(`{"name":"users/me/pairedDevices/device-123"}`))
	}))
	defer server.Close()

	client := New(server.URL, "users/me", server.Client())
	if _, err := client.GetPairedDevice(context.Background(), "users/me/pairedDevices/device-123"); err != nil {
		t.Fatalf("matching full name rejected: %v", err)
	}
	if _, err := client.GetPairedDevice(context.Background(), "users/other/pairedDevices/device-123"); err == nil {
		t.Fatal("cross-user paired device accepted")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("transport requests = %d, want 1", got)
	}
}

func TestProviderAndOperationNamesAreSanitizedToOneLine(t *testing.T) {
	errorsToCheck := []error{
		&APIError{
			StatusCode: http.StatusInternalServerError,
			Status:     "500 Provider\r\nInjected",
			Message:    "request failed\nretry later",
		},
		&OperationError{
			Name: "operations/create\nInjected",
			Status: GoogleStatus{
				Message: "write failed\r\nretry later",
			},
		},
	}
	_, decodeError := DecodeOperationResponse[Subscriber](Operation{Name: "operations/decode\nInjected"})
	errorsToCheck = append(errorsToCheck, decodeError)

	for _, err := range errorsToCheck {
		if err == nil {
			t.Fatal("expected error")
		}
		if strings.ContainsAny(err.Error(), "\r\n\t") {
			t.Fatalf("error is not one line: %q", err)
		}
	}
}

func TestRawPreservesJSONAndEmptySuccessReceipt(t *testing.T) {
	responses := []string{`{"newProviderField":{"largeID":9007199254740993}}`, ""}
	var index atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(responses[index.Add(1)-1]))
	}))
	defer server.Close()
	client := New(server.URL, "users/me", server.Client())

	value, err := client.Raw(context.Background(), http.MethodGet, "/v4/example", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != responses[0] {
		t.Fatalf("raw response = %s", value)
	}
	empty, err := client.Raw(context.Background(), http.MethodDelete, "/v4/example", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(empty) != `{"status":"ok"}` {
		t.Fatalf("empty receipt = %s", empty)
	}
}

func endpointCases() []endpointCase {
	walkingStride := int32(740)
	profileRequest := UpdateProfileRequest{
		Name:                                "users/me/profile",
		UserConfiguredWalkingStrideLengthMM: &walkingStride,
	}
	settingsRequest := UpdateSettingsRequest{
		Name:            "users/me/settings",
		TemperatureUnit: TemperatureUnitCelsius,
	}
	subscriberCreate := CreateSubscriberRequest{
		EndpointURI:           "https://example.com/google-health",
		EndpointAuthorization: EndpointAuthorizationInput{Secret: "Bearer test-secret"},
		SubscriberConfigs: []SubscriberConfig{{
			DataTypes:                []string{"steps", "sleep"},
			SubscriptionCreatePolicy: SubscriptionCreatePolicyManual,
		}},
	}
	subscriberUpdate := UpdateSubscriberRequest{
		Name:        "projects/project-123/subscribers/my-sub-123",
		EndpointURI: "https://example.com/google-health-v2",
		SubscriberConfigs: []SubscriberConfig{{
			DataTypes:                []string{"sleep"},
			SubscriptionCreatePolicy: SubscriptionCreatePolicyAutomatic,
		}},
	}
	subscriptionCreate := CreateSubscriptionRequest{
		User:      "users/health-user-123",
		DataTypes: []string{"sleep", "weight"},
	}
	subscriptionUpdate := UpdateSubscriptionRequest{
		Name:      "projects/project-123/subscribers/my-sub-123/subscriptions/user-sub-123",
		DataTypes: []string{"sleep"},
	}
	weightPoint := json.RawMessage(`{
		"name":"users/me/dataTypes/weight/dataPoints/weight-123",
		"weight":{"sampleTime":{"physicalTime":"2026-05-09T06:00:00Z"},"weightGrams":78000}
	}`)
	dailyRollup := DailyRollUpDataPointsRequest{
		Range: &CivilTimeInterval{
			Start: &CivilDateTime{Date: &Date{Year: 2026, Month: 5, Day: 8}},
			End:   &CivilDateTime{Date: &Date{Year: 2026, Month: 5, Day: 9}},
		},
		WindowSizeDays:   1,
		PageSize:         100,
		PageToken:        "daily-next",
		DataSourceFamily: "users/me/dataSourceFamilies/all-sources",
	}
	physicalRollup := RollUpDataPointsRequest{
		Range:            &Interval{StartTime: "2026-05-08T00:00:00Z", EndTime: "2026-05-09T00:00:00Z"},
		WindowSize:       "3600s",
		PageSize:         100,
		PageToken:        "physical-next",
		DataSourceFamily: "users/me/dataSourceFamilies/google-wearables",
	}

	return []endpointCase{
		{
			name:          "projects.subscribers.create",
			operationName: "projects.subscribers.create",
			method:        http.MethodPost,
			path:          "/v4/projects/project-123/subscribers",
			query:         values("subscriberId", "my-sub-123"),
			request:       mustJSON(subscriberCreate),
			response:      operationResponse("subscriber-create"),
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.CreateSubscriberWithOptions(ctx, "project-123", subscriberCreate, CreateSubscriberOptions{SubscriberID: "my-sub-123"})
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
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.DeleteSubscriberWithOptions(ctx, "projects/project-123/subscribers/my-sub-123", DeleteSubscriberOptions{Force: true})
			},
			assert: assertOperation("operations/subscriber-delete"),
		},
		{
			name:          "projects.subscribers.list",
			operationName: "projects.subscribers.list",
			method:        http.MethodGet,
			path:          "/v4/projects/project-123/subscribers",
			query:         values("pageSize", "50", "pageToken", "sub-next"),
			response:      `{"subscribers":[{"name":"projects/project-123/subscribers/my-sub-123","state":"ACTIVE","endpointAuthorization":{"secretSet":true}}],"nextPageToken":"sub-next-2","totalSize":1}`,
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.ListSubscribersWithOptions(ctx, "projects/project-123", SubscriberListOptions{PageSize: 50, PageToken: "sub-next"})
			},
			assert: func(t *testing.T, value any) {
				response := value.(ListSubscribersResponse)
				if response.NextPageToken != "sub-next-2" || len(response.Subscribers) != 1 || response.Subscribers[0].State != SubscriberStateActive {
					t.Fatalf("response = %#v", response)
				}
				if response.Subscribers[0].EndpointAuthorization == nil || response.Subscribers[0].EndpointAuthorization.SecretSet == nil || !*response.Subscribers[0].EndpointAuthorization.SecretSet {
					t.Fatalf("authorization status = %#v", response.Subscribers[0].EndpointAuthorization)
				}
			},
		},
		{
			name:          "projects.subscribers.patch",
			operationName: "projects.subscribers.patch",
			method:        http.MethodPatch,
			path:          "/v4/projects/project-123/subscribers/my-sub-123",
			query:         values("updateMask", "endpointUri,subscriberConfigs"),
			request:       mustJSON(subscriberUpdate),
			response:      operationResponse("subscriber-patch"),
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.PatchSubscriberWithOptions(ctx, "projects/project-123/subscribers/my-sub-123", subscriberUpdate, PatchSubscriberOptions{UpdateMask: "endpointUri,subscriberConfigs"})
			},
			assert: assertOperation("operations/subscriber-patch"),
		},
		{
			name:          "projects.subscribers.subscriptions.create",
			operationName: "projects.subscribers.subscriptions.create",
			method:        http.MethodPost,
			path:          "/v4/projects/project-123/subscribers/my-sub-123/subscriptions",
			query:         values("subscriptionId", "user-sub-123"),
			request:       mustJSON(subscriptionCreate),
			response:      `{"name":"projects/project-123/subscribers/my-sub-123/subscriptions/user-sub-123","user":"users/health-user-123","dataTypes":["sleep","weight"]}`,
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.CreateSubscriptionWithOptions(ctx, "projects/project-123/subscribers/my-sub-123", subscriptionCreate, CreateSubscriptionOptions{SubscriptionID: "user-sub-123"})
			},
			assert: assertSubscription("projects/project-123/subscribers/my-sub-123/subscriptions/user-sub-123"),
		},
		{
			name:          "projects.subscribers.subscriptions.delete",
			operationName: "projects.subscribers.subscriptions.delete",
			method:        http.MethodDelete,
			path:          "/v4/projects/project-123/subscribers/my-sub-123/subscriptions/user-sub-123",
			query:         url.Values{},
			response:      `{}`,
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.DeleteSubscription(ctx, "projects/project-123/subscribers/my-sub-123/subscriptions/user-sub-123")
			},
			assert: func(t *testing.T, value any) {
				if _, ok := value.(Empty); !ok {
					t.Fatalf("value = %T, want Empty", value)
				}
			},
		},
		{
			name:          "projects.subscribers.subscriptions.list",
			operationName: "projects.subscribers.subscriptions.list",
			method:        http.MethodGet,
			path:          "/v4/projects/project-123/subscribers/my-sub-123/subscriptions",
			query:         values("filter", `user = "users/health-user-123"`, "pageSize", "50", "pageToken", "subscription-next"),
			response:      `{"subscriptions":[{"name":"projects/project-123/subscribers/my-sub-123/subscriptions/user-sub-123","user":"users/health-user-123"}],"nextPageToken":"subscription-next-2"}`,
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.ListSubscriptionsWithOptions(ctx, "projects/project-123/subscribers/my-sub-123", SubscriptionListOptions{Filter: `user = "users/health-user-123"`, PageSize: 50, PageToken: "subscription-next"})
			},
			assert: func(t *testing.T, value any) {
				response := value.(ListSubscriptionsResponse)
				if len(response.Subscriptions) != 1 || response.NextPageToken != "subscription-next-2" {
					t.Fatalf("response = %#v", response)
				}
			},
		},
		{
			name:          "projects.subscribers.subscriptions.patch",
			operationName: "projects.subscribers.subscriptions.patch",
			method:        http.MethodPatch,
			path:          "/v4/projects/project-123/subscribers/my-sub-123/subscriptions/user-sub-123",
			query:         values("updateMask", "data_types"),
			request:       mustJSON(subscriptionUpdate),
			response:      `{"name":"projects/project-123/subscribers/my-sub-123/subscriptions/user-sub-123","dataTypes":["sleep"]}`,
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.PatchSubscriptionWithOptions(ctx, "projects/project-123/subscribers/my-sub-123/subscriptions/user-sub-123", subscriptionUpdate, PatchSubscriptionOptions{UpdateMask: "data_types"})
			},
			assert: assertSubscription("projects/project-123/subscribers/my-sub-123/subscriptions/user-sub-123"),
		},
		{
			name:          "users.getIdentity",
			operationName: "users.getIdentity",
			method:        http.MethodGet,
			path:          "/v4/users/me/identity",
			query:         url.Values{},
			response:      `{"name":"users/me/identity","healthUserId":"health-user-123","legacyUserId":"fitbit-user-123"}`,
			call:          func(ctx context.Context, client *Client) (any, error) { return client.GetIdentity(ctx) },
			assert: func(t *testing.T, value any) {
				if identity := value.(Identity); identity.HealthUserID != "health-user-123" {
					t.Fatalf("identity = %#v", identity)
				}
			},
		},
		{
			name:          "users.getIrnProfile",
			operationName: "users.getIrnProfile",
			method:        http.MethodGet,
			path:          "/v4/users/me/irnProfile",
			query:         url.Values{},
			response:      `{"name":"users/me/irnProfile","onboardingStatus":true,"enrollmentStatus":false,"updateTime":"2026-07-30T12:00:00Z"}`,
			call:          func(ctx context.Context, client *Client) (any, error) { return client.GetIRNProfile(ctx) },
			assert: func(t *testing.T, value any) {
				profile := value.(IRNProfile)
				if !profile.OnboardingStatus || profile.EnrollmentStatus {
					t.Fatalf("IRN profile = %#v", profile)
				}
			},
		},
		{
			name:          "users.getProfile",
			operationName: "users.getProfile",
			method:        http.MethodGet,
			path:          "/v4/users/me/profile",
			query:         url.Values{},
			response:      `{"name":"users/me/profile","age":35}`,
			call:          func(ctx context.Context, client *Client) (any, error) { return client.GetProfile(ctx) },
			assert: func(t *testing.T, value any) {
				profile := value.(Profile)
				if profile.Age == nil || *profile.Age != 35 {
					t.Fatalf("profile = %#v", profile)
				}
			},
		},
		{
			name:          "users.getSettings",
			operationName: "users.getSettings",
			method:        http.MethodGet,
			path:          "/v4/users/me/settings",
			query:         url.Values{},
			response:      `{"name":"users/me/settings","temperatureUnit":"TEMPERATURE_UNIT_CELSIUS"}`,
			call:          func(ctx context.Context, client *Client) (any, error) { return client.GetSettings(ctx) },
			assert: func(t *testing.T, value any) {
				if settings := value.(Settings); settings.TemperatureUnit != TemperatureUnitCelsius {
					t.Fatalf("settings = %#v", settings)
				}
			},
		},
		{
			name:          "users.updateProfile",
			operationName: "users.updateProfile",
			method:        http.MethodPatch,
			path:          "/v4/users/me/profile",
			query:         values("updateMask", "user_configured_walking_stride_length_mm"),
			request:       mustJSON(profileRequest),
			response:      `{"name":"users/me/profile","userConfiguredWalkingStrideLengthMm":740}`,
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.UpdateProfileWithOptions(ctx, profileRequest, UpdateOptions{UpdateMask: "user_configured_walking_stride_length_mm"})
			},
			assert: func(t *testing.T, value any) {
				profile := value.(Profile)
				if profile.UserConfiguredWalkingStrideLengthMM == nil || *profile.UserConfiguredWalkingStrideLengthMM != 740 {
					t.Fatalf("profile = %#v", profile)
				}
			},
		},
		{
			name:          "users.updateSettings",
			operationName: "users.updateSettings",
			method:        http.MethodPatch,
			path:          "/v4/users/me/settings",
			query:         values("updateMask", "temperature_unit"),
			request:       mustJSON(settingsRequest),
			response:      `{"name":"users/me/settings","temperatureUnit":"TEMPERATURE_UNIT_CELSIUS"}`,
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.UpdateSettingsWithOptions(ctx, settingsRequest, UpdateOptions{UpdateMask: "temperature_unit"})
			},
			assert: func(t *testing.T, value any) {
				if settings := value.(Settings); settings.TemperatureUnit != TemperatureUnitCelsius {
					t.Fatalf("settings = %#v", settings)
				}
			},
		},
		{
			name:          "users.dataTypes.dataPoints.batchDelete",
			operationName: "users.dataTypes.dataPoints.batchDelete",
			method:        http.MethodPost,
			path:          "/v4/users/me/dataTypes/weight/dataPoints:batchDelete",
			query:         url.Values{},
			request:       mustJSON(BatchDeleteDataPointsRequest{Names: []string{"users/me/dataTypes/weight/dataPoints/weight-123"}}),
			response:      operationResponse("batch-delete"),
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.BatchDeleteDataPoints(ctx, "weight", BatchDeleteDataPointsRequest{Names: []string{"users/me/dataTypes/weight/dataPoints/weight-123"}})
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
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.CreateDataPoint(ctx, "weight", weightPoint)
			},
			assert: assertOperation("operations/data-point-create"),
		},
		{
			name:          "users.dataTypes.dataPoints.dailyRollUp",
			operationName: "users.dataTypes.dataPoints.dailyRollUp",
			method:        http.MethodPost,
			path:          "/v4/users/me/dataTypes/steps/dataPoints:dailyRollUp",
			query:         url.Values{},
			request:       mustJSON(dailyRollup),
			response:      `{"rollupDataPoints":[{"civilStartTime":{"date":{"year":2026,"month":5,"day":8}},"steps":{"count":12345}}]}`,
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.DailyRollUp(ctx, "steps", dailyRollup)
			},
			assert: func(t *testing.T, value any) {
				if got := len(value.(DailyRollUpDataPointsResponse).RollupDataPoints); got != 1 {
					t.Fatalf("rollup count = %d", got)
				}
			},
		},
		{
			name:          "users.dataTypes.dataPoints.exportExerciseTcx",
			operationName: "users.dataTypes.dataPoints.exportExerciseTcx",
			method:        http.MethodGet,
			path:          "/v4/users/me/dataTypes/exercise/dataPoints/exercise-123:exportExerciseTcx",
			query:         values("alt", "media", "partialData", "true"),
			response:      `<TrainingCenterDatabase><Activities /></TrainingCenterDatabase>`,
			contentType:   "application/xml",
			accept:        "application/xml",
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.ExportExerciseTCXWithOptions(ctx, "exercise-123", ExportExerciseTCXOptions{PartialData: true})
			},
			assert: func(t *testing.T, value any) {
				if got := string(value.([]byte)); got != `<TrainingCenterDatabase><Activities /></TrainingCenterDatabase>` {
					t.Fatalf("TCX response = %q", got)
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
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.GetDataPoint(ctx, "sleep", "sleep-123")
			},
			assert: func(t *testing.T, value any) {
				assertJSONEqual(t, value.(json.RawMessage), json.RawMessage(`{"name":"users/me/dataTypes/sleep/dataPoints/sleep-123","sleep":{"interval":{"startTime":"2026-05-08T22:00:00Z","endTime":"2026-05-09T06:00:00Z"}}}`))
			},
		},
		{
			name:          "users.dataTypes.dataPoints.list",
			operationName: "users.dataTypes.dataPoints.list",
			method:        http.MethodGet,
			path:          "/v4/users/me/dataTypes/steps/dataPoints",
			query:         values("filter", `steps.interval.civil_start_time >= "2026-05-08" AND steps.interval.civil_start_time < "2026-05-09"`, "pageSize", "25", "pageToken", "list-next"),
			response:      `{"dataPoints":[{"steps":{"interval":{"civilStartTime":"2026-05-08"},"count":12345},"futureField":"preserved"}],"nextPageToken":"list-next-2"}`,
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.ListDataPoints(ctx, "steps", ListOptions{Filter: `steps.interval.civil_start_time >= "2026-05-08" AND steps.interval.civil_start_time < "2026-05-09"`, PageSize: 25, PageToken: "list-next"})
			},
			assert: func(t *testing.T, value any) {
				response := value.(ListDataPointsResponse)
				if response.NextPageToken != "list-next-2" || len(response.DataPoints) != 1 || !strings.Contains(string(response.DataPoints[0]), "futureField") {
					t.Fatalf("response = %#v", response)
				}
			},
		},
		{
			name:          "users.dataTypes.dataPoints.patch",
			operationName: "users.dataTypes.dataPoints.patch",
			method:        http.MethodPatch,
			path:          "/v4/users/me/dataTypes/weight/dataPoints/weight-123",
			query:         url.Values{},
			request:       weightPoint,
			response:      operationResponse("data-point-patch"),
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.PatchDataPoint(ctx, "weight", "weight-123", weightPoint)
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
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.ReconcileDataPoints(ctx, "heart-rate", ReconcileOptions{Filter: `heart_rate.sample_time.physical_time >= "2026-05-08T00:00:00Z"`, PageSize: 10, PageToken: "reconcile-next", DataSourceFamily: "users/me/dataSourceFamilies/google-wearables"})
			},
			assert: func(t *testing.T, value any) {
				response := value.(ReconcileDataPointsResponse)
				if response.NextPageToken != "reconcile-next-2" || len(response.DataPoints) != 1 {
					t.Fatalf("response = %#v", response)
				}
			},
		},
		{
			name:          "users.dataTypes.dataPoints.rollUp",
			operationName: "users.dataTypes.dataPoints.rollUp",
			method:        http.MethodPost,
			path:          "/v4/users/me/dataTypes/steps/dataPoints:rollUp",
			query:         url.Values{},
			request:       mustJSON(physicalRollup),
			response:      `{"rollupDataPoints":[{"startTime":"2026-05-08T00:00:00Z","endTime":"2026-05-08T01:00:00Z","steps":{"count":501}}],"nextPageToken":"rollup-next"}`,
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.RollUp(ctx, "steps", physicalRollup)
			},
			assert: func(t *testing.T, value any) {
				response := value.(RollUpDataPointsResponse)
				if response.NextPageToken != "rollup-next" || len(response.RollupDataPoints) != 1 {
					t.Fatalf("response = %#v", response)
				}
			},
		},
		{
			name:          "users.pairedDevices.get",
			operationName: "users.pairedDevices.get",
			method:        http.MethodGet,
			path:          "/v4/users/me/pairedDevices/device-123",
			query:         url.Values{},
			response:      `{"name":"users/me/pairedDevices/device-123","deviceType":"TRACKER","batteryLevel":85,"features":["STEPS","HEART_RATE"]}`,
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.GetPairedDevice(ctx, "device-123")
			},
			assert: func(t *testing.T, value any) {
				device := value.(PairedDevice)
				if device.DeviceType != DeviceTypeTracker || device.BatteryLevel == nil || *device.BatteryLevel != 85 {
					t.Fatalf("device = %#v", device)
				}
			},
		},
		{
			name:          "users.pairedDevices.list",
			operationName: "users.pairedDevices.list",
			method:        http.MethodGet,
			path:          "/v4/users/me/pairedDevices",
			query:         values("pageSize", "5", "pageToken", "device-next"),
			response:      `{"pairedDevices":[{"name":"users/me/pairedDevices/device-123","deviceType":"TRACKER"}],"nextPageToken":"device-next-2"}`,
			call: func(ctx context.Context, client *Client) (any, error) {
				return client.ListPairedDevicesWithOptions(ctx, PairedDeviceListOptions{PageSize: 5, PageToken: "device-next"})
			},
			assert: func(t *testing.T, value any) {
				response := value.(ListPairedDevicesResponse)
				if response.NextPageToken != "device-next-2" || len(response.PairedDevices) != 1 {
					t.Fatalf("response = %#v", response)
				}
			},
		},
	}
}

func assertRequestBody(t *testing.T, request *http.Request, want json.RawMessage) {
	t.Helper()
	body, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(want) == 0 {
		if len(strings.TrimSpace(string(body))) != 0 {
			t.Fatalf("body = %s, want empty", string(body))
		}
		if contentType := request.Header.Get("Content-Type"); contentType != "" {
			t.Fatalf("Content-Type = %q for empty body", contentType)
		}
		return
	}
	if got := request.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	assertJSONEqual(t, body, want)
}

func assertJSONEqual(t *testing.T, got, want []byte) {
	t.Helper()
	var gotValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("decode got JSON %q: %v", string(got), err)
	}
	var wantValue any
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("decode want JSON %q: %v", string(want), err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("JSON = %#v, want %#v", gotValue, wantValue)
	}
}

func mustJSON(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

func values(items ...string) url.Values {
	values := url.Values{}
	for index := 0; index < len(items); index += 2 {
		values.Set(items[index], items[index+1])
	}
	return values
}

func operationResponse(id string) string {
	return `{"name":"operations/` + id + `","done":true,"response":{}}`
}

func assertOperation(name string) func(*testing.T, any) {
	return func(t *testing.T, value any) {
		t.Helper()
		operation, ok := value.(Operation)
		if !ok {
			t.Fatalf("value = %T, want Operation", value)
		}
		if operation.Name != name || !operation.IsDone() {
			t.Fatalf("operation = %#v, want name %q and done", operation, name)
		}
	}
}

func assertSubscription(name string) func(*testing.T, any) {
	return func(t *testing.T, value any) {
		t.Helper()
		subscription, ok := value.(Subscription)
		if !ok {
			t.Fatalf("value = %T, want Subscription", value)
		}
		if subscription.Name != name {
			t.Fatalf("subscription = %#v, want name %q", subscription, name)
		}
	}
}
