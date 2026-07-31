package healthapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

const (
	DefaultBaseURL        = "https://health.googleapis.com"
	defaultRequestTimeout = 60 * time.Second
	maximumResponseBytes  = 64 << 20
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	User       string
}

// TransportError preserves errors.Is/errors.As access to the underlying
// failure while deliberately omitting request URLs, query parameters, and URL
// user information from its printable form.
type TransportError struct {
	Err error
}

func (e *TransportError) Error() string {
	if e == nil {
		return ""
	}
	switch {
	case errors.Is(e.Err, context.Canceled):
		return "Google Health request canceled"
	case errors.Is(e.Err, context.DeadlineExceeded):
		return "Google Health request timed out"
	default:
		return "Google Health request failed"
	}
}

func (e *TransportError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type ListOptions struct {
	Filter    string
	PageSize  int
	PageToken string
}

type ReconcileOptions struct {
	Filter           string
	PageSize         int
	PageToken        string
	DataSourceFamily string
}

type UpdateOptions struct {
	UpdateMask string
}

type SubscriberListOptions struct {
	PageSize  int
	PageToken string
}

type CreateSubscriberOptions struct {
	SubscriberID string
}

type PatchSubscriberOptions struct {
	UpdateMask string
}

type DeleteSubscriberOptions struct {
	Force bool
}

type SubscriptionListOptions struct {
	Filter    string
	PageSize  int
	PageToken string
}

type CreateSubscriptionOptions struct {
	SubscriptionID string
}

type PatchSubscriptionOptions struct {
	UpdateMask string
}

type PairedDeviceListOptions struct {
	PageSize  int
	PageToken string
}

type ExportExerciseTCXOptions struct {
	PartialData bool
}

func New(baseURL, user string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultRequestTimeout}
	} else if httpClient.Timeout == 0 {
		clientWithTimeout := *httpClient
		clientWithTimeout.Timeout = defaultRequestTimeout
		httpClient = &clientWithTimeout
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	user = strings.Trim(strings.TrimSpace(user), "/")
	if user == "" {
		user = "users/me"
	}
	return &Client{BaseURL: baseURL, HTTPClient: httpClient, User: user}
}

func (c *Client) GetIdentity(ctx context.Context) (Identity, error) {
	requestPath, err := c.userSingletonPath("identity")
	if err != nil {
		return Identity{}, err
	}
	return getJSON[Identity](ctx, c, requestPath, nil)
}

func (c *Client) GetIRNProfile(ctx context.Context) (IRNProfile, error) {
	requestPath, err := c.userSingletonPath("irnProfile")
	if err != nil {
		return IRNProfile{}, err
	}
	return getJSON[IRNProfile](ctx, c, requestPath, nil)
}

func (c *Client) GetProfile(ctx context.Context) (Profile, error) {
	requestPath, err := c.userSingletonPath("profile")
	if err != nil {
		return Profile{}, err
	}
	return getJSON[Profile](ctx, c, requestPath, nil)
}

func (c *Client) UpdateProfile(ctx context.Context, body UpdateProfileRequest) (Profile, error) {
	return c.UpdateProfileWithOptions(ctx, body, UpdateOptions{})
}

func (c *Client) UpdateProfileWithOptions(ctx context.Context, body UpdateProfileRequest, opts UpdateOptions) (Profile, error) {
	requestPath, err := c.userSingletonPath("profile")
	if err != nil {
		return Profile{}, err
	}
	return sendJSON[UpdateProfileRequest, Profile](ctx, c, http.MethodPatch, requestPath, updateQuery(opts.UpdateMask), body)
}

func (c *Client) GetSettings(ctx context.Context) (Settings, error) {
	requestPath, err := c.userSingletonPath("settings")
	if err != nil {
		return Settings{}, err
	}
	return getJSON[Settings](ctx, c, requestPath, nil)
}

func (c *Client) UpdateSettings(ctx context.Context, body UpdateSettingsRequest) (Settings, error) {
	return c.UpdateSettingsWithOptions(ctx, body, UpdateOptions{})
}

func (c *Client) UpdateSettingsWithOptions(ctx context.Context, body UpdateSettingsRequest, opts UpdateOptions) (Settings, error) {
	requestPath, err := c.userSingletonPath("settings")
	if err != nil {
		return Settings{}, err
	}
	return sendJSON[UpdateSettingsRequest, Settings](ctx, c, http.MethodPatch, requestPath, updateQuery(opts.UpdateMask), body)
}

func (c *Client) ListDataPoints(ctx context.Context, dataType string, opts ListOptions) (ListDataPointsResponse, error) {
	requestPath, err := c.dataPointsPath(dataType)
	if err != nil {
		return ListDataPointsResponse{}, err
	}
	query := pageQuery(opts.PageSize, opts.PageToken)
	if opts.Filter != "" {
		query.Set("filter", opts.Filter)
	}
	return getJSON[ListDataPointsResponse](ctx, c, requestPath, query)
}

func (c *Client) ReconcileDataPoints(ctx context.Context, dataType string, opts ReconcileOptions) (ReconcileDataPointsResponse, error) {
	requestPath, err := c.dataPointsPath(dataType)
	if err != nil {
		return ReconcileDataPointsResponse{}, err
	}
	query := pageQuery(opts.PageSize, opts.PageToken)
	if opts.Filter != "" {
		query.Set("filter", opts.Filter)
	}
	if opts.DataSourceFamily != "" {
		query.Set("dataSourceFamily", opts.DataSourceFamily)
	}
	return getJSON[ReconcileDataPointsResponse](ctx, c, requestPath+":reconcile", query)
}

func (c *Client) GetDataPoint(ctx context.Context, dataType, id string) (DataPoint, error) {
	requestPath, err := c.dataPointName(dataType, id)
	if err != nil {
		return nil, err
	}
	return getJSON[DataPoint](ctx, c, requestPath, nil)
}

func (c *Client) CreateDataPoint(ctx context.Context, dataType string, body DataPoint) (Operation, error) {
	requestPath, err := c.dataPointsPath(dataType)
	if err != nil {
		return Operation{}, err
	}
	return sendOperation(ctx, c, http.MethodPost, requestPath, nil, body)
}

func (c *Client) PatchDataPoint(ctx context.Context, dataType, id string, body DataPoint) (Operation, error) {
	requestPath, err := c.dataPointName(dataType, id)
	if err != nil {
		return Operation{}, err
	}
	return sendOperation(ctx, c, http.MethodPatch, requestPath, nil, body)
}

func (c *Client) BatchDeleteDataPoints(ctx context.Context, dataType string, body BatchDeleteDataPointsRequest) (Operation, error) {
	requestPath, err := c.dataPointsPath(dataType)
	if err != nil {
		return Operation{}, err
	}
	validatedNames := make([]string, len(body.Names))
	for index, name := range body.Names {
		trimmedName := strings.Trim(name, "/")
		if !strings.HasPrefix(trimmedName, "users/") {
			return Operation{}, errors.New("batch-delete data point names must be full resource names")
		}
		if _, err := c.dataPointName(dataType, trimmedName); err != nil {
			return Operation{}, err
		}
		validatedNames[index] = trimmedName
	}
	body.Names = validatedNames
	return sendOperation(ctx, c, http.MethodPost, requestPath+":batchDelete", nil, body)
}

func (c *Client) DailyRollUp(ctx context.Context, dataType string, body DailyRollUpDataPointsRequest) (DailyRollUpDataPointsResponse, error) {
	requestPath, err := c.dataPointsPath(dataType)
	if err != nil {
		return DailyRollUpDataPointsResponse{}, err
	}
	return sendJSON[DailyRollUpDataPointsRequest, DailyRollUpDataPointsResponse](ctx, c, http.MethodPost, requestPath+":dailyRollUp", nil, body)
}

func (c *Client) RollUp(ctx context.Context, dataType string, body RollUpDataPointsRequest) (RollUpDataPointsResponse, error) {
	requestPath, err := c.dataPointsPath(dataType)
	if err != nil {
		return RollUpDataPointsResponse{}, err
	}
	return sendJSON[RollUpDataPointsRequest, RollUpDataPointsResponse](ctx, c, http.MethodPost, requestPath+":rollUp", nil, body)
}

func (c *Client) ExportExerciseTCX(ctx context.Context, id string) ([]byte, error) {
	return c.ExportExerciseTCXWithOptions(ctx, id, ExportExerciseTCXOptions{})
}

func (c *Client) ExportExerciseTCXWithOptions(ctx context.Context, id string, opts ExportExerciseTCXOptions) ([]byte, error) {
	requestPath, err := c.dataPointName("exercise", id)
	if err != nil {
		return nil, err
	}
	query := url.Values{"alt": []string{"media"}}
	if opts.PartialData {
		query.Set("partialData", "true")
	}
	return c.doBytes(ctx, http.MethodGet, requestPath+":exportExerciseTcx", query, nil, "application/xml")
}

func (c *Client) ListSubscribers(ctx context.Context, project string) (ListSubscribersResponse, error) {
	return c.ListSubscribersWithOptions(ctx, project, SubscriberListOptions{})
}

func (c *Client) ListSubscribersWithOptions(ctx context.Context, project string, opts SubscriberListOptions) (ListSubscribersResponse, error) {
	requestPath, err := subscriberCollectionPath(project)
	if err != nil {
		return ListSubscribersResponse{}, err
	}
	return getJSON[ListSubscribersResponse](ctx, c, requestPath, pageQuery(opts.PageSize, opts.PageToken))
}

func (c *Client) CreateSubscriber(ctx context.Context, project string, body CreateSubscriberRequest) (Operation, error) {
	return c.CreateSubscriberWithOptions(ctx, project, body, CreateSubscriberOptions{})
}

func (c *Client) CreateSubscriberWithOptions(ctx context.Context, project string, body CreateSubscriberRequest, opts CreateSubscriberOptions) (Operation, error) {
	requestPath, err := subscriberCollectionPath(project)
	if err != nil {
		return Operation{}, err
	}
	query := url.Values{}
	if opts.SubscriberID != "" {
		if _, err := safeSegment(opts.SubscriberID, "subscriber ID"); err != nil {
			return Operation{}, err
		}
		query.Set("subscriberId", opts.SubscriberID)
	}
	return sendOperation(ctx, c, http.MethodPost, requestPath, query, body)
}

func (c *Client) PatchSubscriber(ctx context.Context, name string, body UpdateSubscriberRequest) (Operation, error) {
	return c.PatchSubscriberWithOptions(ctx, name, body, PatchSubscriberOptions{})
}

func (c *Client) PatchSubscriberWithOptions(ctx context.Context, name string, body UpdateSubscriberRequest, opts PatchSubscriberOptions) (Operation, error) {
	requestPath, err := validateResourceName(name, "projects", "subscribers")
	if err != nil {
		return Operation{}, err
	}
	return sendOperation(ctx, c, http.MethodPatch, requestPath, updateQuery(opts.UpdateMask), body)
}

func (c *Client) DeleteSubscriber(ctx context.Context, name string) (Operation, error) {
	return c.DeleteSubscriberWithOptions(ctx, name, DeleteSubscriberOptions{})
}

func (c *Client) DeleteSubscriberWithOptions(ctx context.Context, name string, opts DeleteSubscriberOptions) (Operation, error) {
	requestPath, err := validateResourceName(name, "projects", "subscribers")
	if err != nil {
		return Operation{}, err
	}
	query := url.Values{}
	if opts.Force {
		query.Set("force", "true")
	}
	return deleteOperation(ctx, c, requestPath, query)
}

func (c *Client) ListSubscriptions(ctx context.Context, parent string) (ListSubscriptionsResponse, error) {
	return c.ListSubscriptionsWithOptions(ctx, parent, SubscriptionListOptions{})
}

func (c *Client) ListSubscriptionsWithOptions(ctx context.Context, parent string, opts SubscriptionListOptions) (ListSubscriptionsResponse, error) {
	requestPath, err := subscriptionCollectionPath(parent)
	if err != nil {
		return ListSubscriptionsResponse{}, err
	}
	query := pageQuery(opts.PageSize, opts.PageToken)
	if opts.Filter != "" {
		query.Set("filter", opts.Filter)
	}
	return getJSON[ListSubscriptionsResponse](ctx, c, requestPath, query)
}

func (c *Client) CreateSubscription(ctx context.Context, parent string, body CreateSubscriptionRequest) (Subscription, error) {
	return c.CreateSubscriptionWithOptions(ctx, parent, body, CreateSubscriptionOptions{})
}

func (c *Client) CreateSubscriptionWithOptions(ctx context.Context, parent string, body CreateSubscriptionRequest, opts CreateSubscriptionOptions) (Subscription, error) {
	requestPath, err := subscriptionCollectionPath(parent)
	if err != nil {
		return Subscription{}, err
	}
	query := url.Values{}
	if opts.SubscriptionID != "" {
		if _, err := safeSegment(opts.SubscriptionID, "subscription ID"); err != nil {
			return Subscription{}, err
		}
		query.Set("subscriptionId", opts.SubscriptionID)
	}
	return sendJSON[CreateSubscriptionRequest, Subscription](ctx, c, http.MethodPost, requestPath, query, body)
}

func (c *Client) PatchSubscription(ctx context.Context, name string, body UpdateSubscriptionRequest) (Subscription, error) {
	return c.PatchSubscriptionWithOptions(ctx, name, body, PatchSubscriptionOptions{})
}

func (c *Client) PatchSubscriptionWithOptions(ctx context.Context, name string, body UpdateSubscriptionRequest, opts PatchSubscriptionOptions) (Subscription, error) {
	requestPath, err := validateResourceName(name, "projects", "subscribers", "subscriptions")
	if err != nil {
		return Subscription{}, err
	}
	return sendJSON[UpdateSubscriptionRequest, Subscription](ctx, c, http.MethodPatch, requestPath, updateQuery(opts.UpdateMask), body)
}

func (c *Client) DeleteSubscription(ctx context.Context, name string) (Empty, error) {
	requestPath, err := validateResourceName(name, "projects", "subscribers", "subscriptions")
	if err != nil {
		return Empty{}, err
	}
	return deleteJSON[Empty](ctx, c, requestPath, nil)
}

func (c *Client) ListPairedDevices(ctx context.Context) (ListPairedDevicesResponse, error) {
	return c.ListPairedDevicesWithOptions(ctx, PairedDeviceListOptions{})
}

func (c *Client) ListPairedDevicesWithOptions(ctx context.Context, opts PairedDeviceListOptions) (ListPairedDevicesResponse, error) {
	userPath, err := c.userPath()
	if err != nil {
		return ListPairedDevicesResponse{}, err
	}
	return getJSON[ListPairedDevicesResponse](ctx, c, userPath+"/pairedDevices", pageQuery(opts.PageSize, opts.PageToken))
}

func (c *Client) GetPairedDevice(ctx context.Context, nameOrID string) (PairedDevice, error) {
	var requestPath string
	var err error
	if strings.HasPrefix(strings.Trim(nameOrID, "/"), "users/") {
		requestPath, err = c.userBoundResourceName(nameOrID, "pairedDevices")
	} else {
		userPath, userErr := c.userPath()
		if userErr != nil {
			return PairedDevice{}, userErr
		}
		segment, segmentErr := safeSegment(nameOrID, "paired device ID")
		if segmentErr != nil {
			return PairedDevice{}, segmentErr
		}
		requestPath = userPath + "/pairedDevices/" + segment
	}
	if err != nil {
		return PairedDevice{}, err
	}
	return getJSON[PairedDevice](ctx, c, requestPath, nil)
}

// Raw is the explicit escape hatch for provider methods not yet modeled by the
// client. Normal commands should use the typed methods above.
func (c *Client) Raw(ctx context.Context, method, requestPath string, query url.Values, body json.RawMessage) (json.RawMessage, error) {
	response, err := c.doBytes(ctx, strings.ToUpper(strings.TrimSpace(method)), requestPath, query, body, "application/json")
	if err != nil {
		return nil, err
	}
	if len(response) == 0 {
		return json.RawMessage(`{"status":"ok"}`), nil
	}
	if !json.Valid(response) {
		return nil, fmt.Errorf("decode response: invalid JSON")
	}
	return json.RawMessage(response), nil
}

// RawBytes is the byte-preserving escape hatch for endpoints backed by
// google.api.HttpBody or other non-JSON media. Callers are responsible for
// selecting authenticated or unauthenticated transport explicitly.
func (c *Client) RawBytes(ctx context.Context, method, requestPath string, query url.Values, body json.RawMessage) ([]byte, error) {
	return c.doBytes(ctx, strings.ToUpper(strings.TrimSpace(method)), requestPath, query, body, "*/*")
}

func getJSON[Response any](ctx context.Context, client *Client, requestPath string, query url.Values) (Response, error) {
	return doJSON[Response](ctx, client, http.MethodGet, requestPath, query, nil)
}

func deleteJSON[Response any](ctx context.Context, client *Client, requestPath string, query url.Values) (Response, error) {
	return doJSON[Response](ctx, client, http.MethodDelete, requestPath, query, nil)
}

func sendJSON[Request, Response any](ctx context.Context, client *Client, method, requestPath string, query url.Values, request Request) (Response, error) {
	body, err := json.Marshal(request)
	if err != nil {
		var zero Response
		return zero, fmt.Errorf("encode request: %w", err)
	}
	return doJSON[Response](ctx, client, method, requestPath, query, body)
}

func doJSON[Response any](ctx context.Context, client *Client, method, requestPath string, query url.Values, body json.RawMessage) (Response, error) {
	var value Response
	response, err := client.doBytes(ctx, method, requestPath, query, body, "application/json")
	if err != nil {
		return value, err
	}
	if len(response) == 0 {
		if _, ok := any(value).(Empty); ok {
			return value, nil
		}
		return value, fmt.Errorf("decode response: empty body")
	}
	if err := json.Unmarshal(response, &value); err != nil {
		return value, fmt.Errorf("decode response: %w", err)
	}
	return value, nil
}

func sendOperation[Request any](ctx context.Context, client *Client, method, requestPath string, query url.Values, request Request) (Operation, error) {
	operation, err := sendJSON[Request, Operation](ctx, client, method, requestPath, query, request)
	if err != nil {
		return Operation{}, err
	}
	return operation, operation.Err()
}

func deleteOperation(ctx context.Context, client *Client, requestPath string, query url.Values) (Operation, error) {
	operation, err := doJSON[Operation](ctx, client, http.MethodDelete, requestPath, query, nil)
	if err != nil {
		return Operation{}, err
	}
	return operation, operation.Err()
}

func (c *Client) doBytes(ctx context.Context, method, requestPath string, query url.Values, body json.RawMessage, accept string) ([]byte, error) {
	baseURL, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, errors.New("invalid Google Health base URL")
	}
	if err := validateBaseURL(baseURL); err != nil {
		return nil, err
	}
	requestPath, err = normalizeRequestPath(requestPath)
	if err != nil {
		return nil, err
	}
	baseURL.Path = strings.TrimRight(baseURL.Path, "/") + requestPath
	baseURL.RawPath = ""
	baseURL.RawQuery = query.Encode()
	baseURL.Fragment = ""

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, baseURL.String(), reader)
	if err != nil {
		return nil, &TransportError{Err: err}
	}
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Accept", accept)

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultRequestTimeout}
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, &TransportError{Err: err}
	}
	defer response.Body.Close()
	responseBytes, readErr := readBounded(response.Body, maximumResponseBytes)
	if readErr != nil {
		return nil, &TransportError{Err: readErr}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, parseAPIError(response, responseBytes)
	}
	return responseBytes, nil
}

func validateBaseURL(baseURL *url.URL) error {
	if baseURL == nil || baseURL.Host == "" {
		return errors.New("invalid Google Health base URL")
	}
	if baseURL.User != nil {
		return errors.New("Google Health base URL must not include credentials")
	}
	if baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return errors.New("Google Health base URL must not include a query or fragment")
	}
	scheme := strings.ToLower(baseURL.Scheme)
	hostIP := net.ParseIP(baseURL.Hostname())
	if hostIP != nil && hostIP.IsLoopback() && (scheme == "http" || scheme == "https") {
		return nil
	}
	productionPath := baseURL.EscapedPath()
	if scheme == "https" && strings.EqualFold(baseURL.Hostname(), "health.googleapis.com") && baseURL.Port() == "" && (productionPath == "" || productionPath == "/") {
		return nil
	}
	return errors.New("Google Health base URL must be https://health.googleapis.com; literal loopback HTTP or HTTPS is allowed for tests")
}

func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	bytes, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(bytes)) > limit {
		return nil, fmt.Errorf("Google Health API response exceeds %d bytes", limit)
	}
	return bytes, nil
}

func parseAPIError(response *http.Response, responseBody []byte) *APIError {
	apiError := &APIError{
		StatusCode: response.StatusCode,
		Status:     response.Status,
		Code:       response.StatusCode,
		Message:    http.StatusText(response.StatusCode),
	}
	var envelope struct {
		Error GoogleStatus `json:"error"`
	}
	if json.Unmarshal(responseBody, &envelope) != nil {
		return apiError
	}
	if envelope.Error.Code != 0 {
		apiError.Code = envelope.Error.Code
	}
	apiError.APIStatus = envelope.Error.Status
	if envelope.Error.Message != "" {
		apiError.Message = sanitizeMessage(envelope.Error.Message)
	}
	apiError.Details = envelope.Error.Details
	for _, detail := range envelope.Error.Details {
		var info struct {
			Reason string `json:"reason"`
			Domain string `json:"domain"`
		}
		if json.Unmarshal(detail, &info) == nil && info.Reason != "" {
			apiError.Reason = info.Reason
			apiError.Domain = info.Domain
			break
		}
	}
	return apiError
}

func (c *Client) userPath() (string, error) {
	return validateResourceName(c.User, "users")
}

func (c *Client) userSingletonPath(resource string) (string, error) {
	userPath, err := c.userPath()
	if err != nil {
		return "", err
	}
	return userPath + "/" + resource, nil
}

func (c *Client) dataPointsPath(dataType string) (string, error) {
	userPath, err := c.userPath()
	if err != nil {
		return "", err
	}
	dataType, err = safeSegment(dataType, "data type")
	if err != nil {
		return "", err
	}
	return userPath + "/dataTypes/" + dataType + "/dataPoints", nil
}

func (c *Client) dataPointName(dataType, id string) (string, error) {
	id = strings.Trim(id, "/")
	if strings.HasPrefix(id, "users/") {
		identifiers, requestPath, err := parseResourceName(id, "users", "dataTypes", "dataPoints")
		if err != nil {
			return "", err
		}
		configuredUser, err := c.configuredUserID()
		if err != nil {
			return "", err
		}
		dataType, err = safeSegment(dataType, "data type")
		if err != nil {
			return "", err
		}
		if identifiers[0] != configuredUser {
			return "", fmt.Errorf("data point user %q does not match configured user", identifiers[0])
		}
		if identifiers[1] != dataType {
			return "", fmt.Errorf("data point type %q does not match requested type %q", identifiers[1], dataType)
		}
		return requestPath, nil
	}
	collection, err := c.dataPointsPath(dataType)
	if err != nil {
		return "", err
	}
	id, err = safeSegment(id, "data point ID")
	if err != nil {
		return "", err
	}
	return collection + "/" + id, nil
}

func subscriberCollectionPath(project string) (string, error) {
	project = strings.Trim(project, "/")
	if !strings.HasPrefix(project, "projects/") {
		project = "projects/" + project
	}
	projectPath, err := validateResourceName(project, "projects")
	if err != nil {
		return "", err
	}
	return projectPath + "/subscribers", nil
}

func subscriptionCollectionPath(parent string) (string, error) {
	parentPath, err := validateResourceName(parent, "projects", "subscribers")
	if err != nil {
		return "", err
	}
	return parentPath + "/subscriptions", nil
}

func validateResourceName(name string, literals ...string) (string, error) {
	_, requestPath, err := parseResourceName(name, literals...)
	return requestPath, err
}

func parseResourceName(name string, literals ...string) ([]string, string, error) {
	name = strings.Trim(strings.TrimSpace(name), "/")
	parts := strings.Split(name, "/")
	if len(parts) != len(literals)*2 {
		return nil, "", fmt.Errorf("invalid resource name %q", name)
	}
	identifiers := make([]string, len(literals))
	for index, literal := range literals {
		if parts[index*2] != literal {
			return nil, "", fmt.Errorf("invalid resource name %q", name)
		}
		segment, err := safeSegment(parts[index*2+1], literal+" ID")
		if err != nil {
			return nil, "", err
		}
		parts[index*2+1] = segment
		identifiers[index] = segment
	}
	return identifiers, "/v4/" + strings.Join(parts, "/"), nil
}

func (c *Client) configuredUserID() (string, error) {
	identifiers, _, err := parseResourceName(c.User, "users")
	if err != nil {
		return "", err
	}
	return identifiers[0], nil
}

func (c *Client) userBoundResourceName(name string, literals ...string) (string, error) {
	allLiterals := append([]string{"users"}, literals...)
	identifiers, requestPath, err := parseResourceName(name, allLiterals...)
	if err != nil {
		return "", err
	}
	configuredUser, err := c.configuredUserID()
	if err != nil {
		return "", err
	}
	if identifiers[0] != configuredUser {
		return "", fmt.Errorf("resource user %q does not match configured user", identifiers[0])
	}
	return requestPath, nil
}

func safeSegment(value, label string) (string, error) {
	if value == "" || value != strings.TrimSpace(value) || value == "." || value == ".." {
		return "", fmt.Errorf("invalid %s %q", label, value)
	}
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '-' || character == '_' || character == '.' || character == '~' {
			continue
		}
		return "", fmt.Errorf("invalid %s %q", label, value)
	}
	return value, nil
}

func normalizeRequestPath(requestPath string) (string, error) {
	requestPath = strings.TrimSpace(requestPath)
	if requestPath == "" || strings.Contains(requestPath, "\\") || strings.Contains(requestPath, "://") || strings.HasPrefix(requestPath, "//") {
		return "", errors.New("invalid request path")
	}
	if !strings.HasPrefix(requestPath, "/") {
		requestPath = "/" + requestPath
	}
	parsed, err := url.ParseRequestURI(requestPath)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("invalid request path")
	}
	for _, segment := range strings.Split(parsed.Path, "/") {
		if segment == "." || segment == ".." {
			return "", errors.New("invalid request path")
		}
	}
	return parsed.Path, nil
}

func pageQuery(pageSize int, pageToken string) url.Values {
	query := url.Values{}
	if pageSize > 0 {
		query.Set("pageSize", fmt.Sprint(pageSize))
	}
	if pageToken != "" {
		query.Set("pageToken", pageToken)
	}
	return query
}

func updateQuery(updateMask string) url.Values {
	query := url.Values{}
	if updateMask != "" {
		query.Set("updateMask", updateMask)
	}
	return query
}

func IsNotLoggedIn(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not logged in")
}

func IsAPIError(err error, status int) bool {
	var apiError *APIError
	return errors.As(err, &apiError) && apiError.StatusCode == status
}
