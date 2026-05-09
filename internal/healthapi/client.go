package healthapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

const DefaultBaseURL = "https://health.googleapis.com"

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	User       string
}

type APIError struct {
	StatusCode int    `json:"statusCode"`
	Status     string `json:"status"`
	Body       string `json:"body,omitempty"`
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return fmt.Sprintf("Google Health API returned %s", e.Status)
	}
	return fmt.Sprintf("Google Health API returned %s: %s", e.Status, body)
}

type ListOptions struct {
	Filter    string
	PageSize  int
	PageToken string
	View      string
}

type ReconcileOptions struct {
	Filter    string
	PageSize  int
	PageToken string
}

func New(baseURL, user string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
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

func (c *Client) GetIdentity(ctx context.Context) (map[string]any, error) {
	return c.get(ctx, "/v4/"+c.User+"/identity", nil)
}

func (c *Client) GetProfile(ctx context.Context) (map[string]any, error) {
	return c.get(ctx, "/v4/"+c.User+"/profile", nil)
}

func (c *Client) UpdateProfile(ctx context.Context, body any) (map[string]any, error) {
	return c.patch(ctx, "/v4/"+c.User+"/profile", nil, body)
}

func (c *Client) GetSettings(ctx context.Context) (map[string]any, error) {
	return c.get(ctx, "/v4/"+c.User+"/settings", nil)
}

func (c *Client) UpdateSettings(ctx context.Context, body any) (map[string]any, error) {
	return c.patch(ctx, "/v4/"+c.User+"/settings", nil, body)
}

func (c *Client) ListDataPoints(ctx context.Context, dataType string, opts ListOptions) (map[string]any, error) {
	query := url.Values{}
	if opts.Filter != "" {
		query.Set("filter", opts.Filter)
	}
	if opts.PageSize > 0 {
		query.Set("pageSize", fmt.Sprint(opts.PageSize))
	}
	if opts.PageToken != "" {
		query.Set("pageToken", opts.PageToken)
	}
	if opts.View != "" {
		query.Set("view", opts.View)
	}
	return c.get(ctx, c.dataPointsPath(dataType), query)
}

func (c *Client) ReconcileDataPoints(ctx context.Context, dataType string, opts ReconcileOptions) (map[string]any, error) {
	query := url.Values{}
	if opts.Filter != "" {
		query.Set("filter", opts.Filter)
	}
	if opts.PageSize > 0 {
		query.Set("pageSize", fmt.Sprint(opts.PageSize))
	}
	if opts.PageToken != "" {
		query.Set("pageToken", opts.PageToken)
	}
	return c.get(ctx, c.dataPointsPath(dataType)+":reconcile", query)
}

func (c *Client) GetDataPoint(ctx context.Context, dataType, id string) (map[string]any, error) {
	return c.get(ctx, c.dataPointName(dataType, id), nil)
}

func (c *Client) CreateDataPoint(ctx context.Context, dataType string, body any) (map[string]any, error) {
	return c.post(ctx, c.dataPointsPath(dataType), nil, body)
}

func (c *Client) PatchDataPoint(ctx context.Context, dataType, id string, body any) (map[string]any, error) {
	return c.patch(ctx, c.dataPointName(dataType, id), nil, body)
}

func (c *Client) BatchDeleteDataPoints(ctx context.Context, dataType string, body any) (map[string]any, error) {
	return c.post(ctx, c.dataPointsPath(dataType)+":batchDelete", nil, body)
}

func (c *Client) DailyRollUp(ctx context.Context, dataType string, body any) (map[string]any, error) {
	return c.post(ctx, c.dataPointsPath(dataType)+":dailyRollUp", nil, body)
}

func (c *Client) RollUp(ctx context.Context, dataType string, body any) (map[string]any, error) {
	return c.post(ctx, c.dataPointsPath(dataType)+":rollUp", nil, body)
}

func (c *Client) ExportExerciseTCX(ctx context.Context, id string) ([]byte, error) {
	return c.doBytes(ctx, http.MethodGet, c.dataPointName("exercise", id)+":exportExerciseTcx", nil, nil)
}

func (c *Client) ListSubscribers(ctx context.Context, project string) (map[string]any, error) {
	return c.get(ctx, "/v4/"+cleanProject(project)+"/subscribers", nil)
}

func (c *Client) CreateSubscriber(ctx context.Context, project string, body any) (map[string]any, error) {
	return c.post(ctx, "/v4/"+cleanProject(project)+"/subscribers", nil, body)
}

func (c *Client) PatchSubscriber(ctx context.Context, name string, body any) (map[string]any, error) {
	return c.patch(ctx, "/v4/"+strings.Trim(name, "/"), nil, body)
}

func (c *Client) DeleteSubscriber(ctx context.Context, name string) (map[string]any, error) {
	return c.delete(ctx, "/v4/"+strings.Trim(name, "/"), nil, nil)
}

func (c *Client) Raw(ctx context.Context, method, requestPath string, query url.Values, body any) (map[string]any, error) {
	var out map[string]any
	if err := c.doJSON(ctx, strings.ToUpper(method), requestPath, query, body, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{"status": "ok"}
	}
	return out, nil
}

func (c *Client) get(ctx context.Context, requestPath string, query url.Values) (map[string]any, error) {
	return c.Raw(ctx, http.MethodGet, requestPath, query, nil)
}

func (c *Client) post(ctx context.Context, requestPath string, query url.Values, body any) (map[string]any, error) {
	return c.Raw(ctx, http.MethodPost, requestPath, query, body)
}

func (c *Client) patch(ctx context.Context, requestPath string, query url.Values, body any) (map[string]any, error) {
	return c.Raw(ctx, http.MethodPatch, requestPath, query, body)
}

func (c *Client) delete(ctx context.Context, requestPath string, query url.Values, body any) (map[string]any, error) {
	return c.Raw(ctx, http.MethodDelete, requestPath, query, body)
}

func (c *Client) doJSON(ctx context.Context, method, requestPath string, query url.Values, body any, out any) error {
	bytes, err := c.doBytes(ctx, method, requestPath, query, body)
	if err != nil {
		return err
	}
	if len(bytes) == 0 || out == nil {
		return nil
	}
	if err := json.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) doBytes(ctx context.Context, method, requestPath string, query url.Values, body any) ([]byte, error) {
	if c.HTTPClient == nil {
		c.HTTPClient = http.DefaultClient
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	requestPath = "/" + strings.TrimLeft(requestPath, "/")
	u.Path = path.Clean(requestPath)
	if strings.HasSuffix(requestPath, ":reconcile") || strings.HasSuffix(requestPath, ":dailyRollUp") || strings.HasSuffix(requestPath, ":rollUp") || strings.HasSuffix(requestPath, ":batchDelete") || strings.HasSuffix(requestPath, ":exportExerciseTcx") {
		u.Path = requestPath
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{StatusCode: resp.StatusCode, Status: resp.Status, Body: string(respBytes)}
	}
	return respBytes, nil
}

func (c *Client) dataPointsPath(dataType string) string {
	return "/v4/" + c.User + "/dataTypes/" + url.PathEscape(dataType) + "/dataPoints"
}

func (c *Client) dataPointName(dataType, id string) string {
	id = strings.Trim(id, "/")
	if strings.HasPrefix(id, "users/") {
		return "/v4/" + id
	}
	return c.dataPointsPath(dataType) + "/" + url.PathEscape(id)
}

func cleanProject(project string) string {
	project = strings.Trim(project, "/")
	if project == "" {
		return "projects/-"
	}
	if strings.HasPrefix(project, "projects/") {
		return project
	}
	return "projects/" + project
}

func IsNotLoggedIn(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not logged in")
}

func IsAPIError(err error, status int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == status
}
