package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/rudrankriyam/Google-Health-CLI/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestTokenSourcePersistsRefreshedToken(t *testing.T) {
	tokenPath := configTokenPath(t)
	expired := &oauth2.Token{
		AccessToken:  "old-access-token",
		TokenType:    "Bearer",
		RefreshToken: "old-refresh-token",
		Expiry:       time.Now().Add(-time.Hour),
	}
	if err := SaveToken(expired); err != nil {
		t.Fatalf("SaveToken() error = %v", err)
	}

	var refreshCalls int
	httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		refreshCalls++
		if req.Method != http.MethodPost {
			t.Fatalf("refresh method = %s, want POST", req.Method)
		}
		if got := req.URL.String(); got != "https://oauth2.googleapis.com/token" {
			t.Fatalf("refresh URL = %s", got)
		}
		if err := req.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		wantForm := url.Values{
			"client_id":     {"client-id"},
			"client_secret": {"client-secret"},
			"grant_type":    {"refresh_token"},
			"refresh_token": {"old-refresh-token"},
		}
		for key, want := range wantForm {
			if got := req.Form[key]; len(got) != len(want) || got[0] != want[0] {
				t.Fatalf("form[%s] = %v, want %v", key, got, want)
			}
		}
		return jsonResponse(t, map[string]any{
			"access_token":  "new-access-token",
			"token_type":    "Bearer",
			"refresh_token": "new-refresh-token",
			"expires_in":    3600,
		}), nil
	})}

	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)
	source, err := TokenSource(ctx, config.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  config.DefaultRedirectURL,
	})
	if err != nil {
		t.Fatalf("TokenSource() error = %v", err)
	}

	token, err := source.Token()
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if token.AccessToken != "new-access-token" {
		t.Fatalf("AccessToken = %q", token.AccessToken)
	}
	if token.RefreshToken != "new-refresh-token" {
		t.Fatalf("RefreshToken = %q", token.RefreshToken)
	}
	if refreshCalls != 1 {
		t.Fatalf("refreshCalls = %d, want 1", refreshCalls)
	}

	stored, err := LoadToken()
	if err != nil {
		t.Fatalf("LoadToken() error = %v", err)
	}
	if stored.AccessToken != "new-access-token" {
		t.Fatalf("stored AccessToken = %q", stored.AccessToken)
	}
	if stored.RefreshToken != "new-refresh-token" {
		t.Fatalf("stored RefreshToken = %q", stored.RefreshToken)
	}
	if !stored.Valid() {
		t.Fatalf("stored token is not valid: %#v", stored)
	}
	info, err := os.Stat(tokenPath)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", tokenPath, err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("token mode = %v, want 0600", got)
	}
}

func TestValidateLoopbackRedirect(t *testing.T) {
	for _, rawURL := range []string{
		"http://127.0.0.1:3000/callback",
		"http://127.1.2.3:49152/oauth/callback",
		"http://[::1]:3000/callback",
	} {
		t.Run("valid_"+strings.NewReplacer(":", "_", "/", "_").Replace(rawURL), func(t *testing.T) {
			if _, err := validateLoopbackRedirect(rawURL); err != nil {
				t.Fatalf("validateLoopbackRedirect(%q): %v", rawURL, err)
			}
		})
	}

	for _, rawURL := range []string{
		"https://127.0.0.1:3000/callback",
		"http://localhost:3000/callback",
		"http://localhost.example:3000/callback",
		"http://0.0.0.0:3000/callback",
		"http://[::]:3000/callback",
		"http://127.0.0.1/callback",
		"http://127.0.0.1:0/callback",
		"http://127.0.0.1:70000/callback",
		"http://127.0.0.1:3000",
		"http://127.0.0.1:3000/",
		"http://127.0.0.1:3000/callback/../other",
		"http://127.0.0.1:3000/%63allback",
		"http://127.0.0.1:3000/callback?next=/other",
		"http://127.0.0.1:3000/callback#fragment",
		"http://user:password@127.0.0.1:3000/callback",
	} {
		t.Run("invalid_"+strings.NewReplacer(":", "_", "/", "_").Replace(rawURL), func(t *testing.T) {
			if _, err := validateLoopbackRedirect(rawURL); err == nil {
				t.Fatalf("validateLoopbackRedirect(%q): expected error", rawURL)
			}
		})
	}
}

func TestLoginIgnoresBadStateAndPersistsOnlyScopeReceipt(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("GHEALTH_CONFIG_DIR", configDir)
	t.Setenv("GHEALTH_TOKEN_STORAGE", "file")
	t.Setenv("GHEALTH_CLIENT_ID", "environment-client-id")
	t.Setenv("GHEALTH_CLIENT_SECRET", "environment-client-secret")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	redirectURL := availableRedirectURL(t)
	authURLCh := make(chan string, 1)
	requestFormCh := make(chan url.Values, 1)
	httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if err := req.ParseForm(); err != nil {
			return nil, err
		}
		requestFormCh <- req.Form
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(strings.NewReader(
				`{"access_token":"new-access-token","token_type":"Bearer","expires_in":3600,"scope":"scope-a scope-b"}`,
			)),
		}, nil
	})}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)
	type loginResult struct {
		token *oauth2.Token
		err   error
	}
	resultCh := make(chan loginResult, 1)
	go func() {
		token, err := Login(ctx, cfg, LoginOptions{
			ClientSecret: "flag-client-secret",
			RedirectURL:  redirectURL,
			Scopes:       []string{"scope-a", "scope-b"},
			Timeout:      3 * time.Second,
			OnAuthURL: func(rawURL string) {
				authURLCh <- rawURL
			},
		})
		resultCh <- loginResult{token: token, err: err}
	}()

	authURL := <-authURLCh
	if strings.Contains(authURL, "environment-client-secret") || strings.Contains(authURL, "flag-client-secret") {
		t.Fatalf("authorization URL exposed a client secret: %s", authURL)
	}
	parsedAuthURL, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	state := parsedAuthURL.Query().Get("state")
	if state == "" {
		t.Fatalf("authorization URL has no state: %s", authURL)
	}

	badResponse, err := http.Get(redirectURL + "?state=attacker-state&code=attacker-code")
	if err != nil {
		t.Fatalf("bad-state callback: %v", err)
	}
	_ = badResponse.Body.Close()
	if badResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad-state status = %d, want %d", badResponse.StatusCode, http.StatusBadRequest)
	}
	select {
	case result := <-resultCh:
		t.Fatalf("bad-state callback terminated login: token=%v err=%v", result.token, result.err)
	case <-time.After(100 * time.Millisecond):
	}

	callbackURL := redirectURL + "?state=" + url.QueryEscape(state) + "&code=real-code"
	response, err := http.Get(callbackURL)
	if err != nil {
		t.Fatalf("real callback: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("real callback status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	result := <-resultCh
	if result.err != nil {
		t.Fatalf("Login: %v", result.err)
	}
	if result.token == nil || result.token.AccessToken != "new-access-token" {
		t.Fatalf("token = %#v", result.token)
	}

	form := <-requestFormCh
	if form.Get("client_secret") != "flag-client-secret" {
		t.Fatalf("token exchange client_secret = %q", form.Get("client_secret"))
	}
	if form.Get("code") != "real-code" {
		t.Fatalf("token exchange code = %q", form.Get("code"))
	}
	if form.Get("code_verifier") == "" {
		t.Fatal("token exchange omitted PKCE verifier")
	}

	stored, err := config.LoadStored()
	if err != nil {
		t.Fatalf("LoadStored: %v", err)
	}
	if stored.ClientID != "" || stored.ClientSecret != "" {
		t.Fatalf("login persisted process credentials: %#v", stored)
	}
	if stored.RedirectURL != config.DefaultRedirectURL {
		t.Fatalf("login persisted flag redirect URL: %q", stored.RedirectURL)
	}
	if got := strings.Join(stored.Scopes, ","); got != "scope-a,scope-b" {
		t.Fatalf("stored scopes = %q", got)
	}

	status, err := CurrentStatus()
	if err != nil {
		t.Fatalf("CurrentStatus: %v", err)
	}
	if !status.Authenticated || !status.Valid {
		t.Fatalf("status = %#v", status)
	}
	if got := strings.Join(status.Scopes, ","); got != "scope-a,scope-b" {
		t.Fatalf("status scopes = %q", got)
	}
}

func TestCurrentStatusReportsStoredNotEnvironmentScopes(t *testing.T) {
	t.Setenv("GHEALTH_CONFIG_DIR", t.TempDir())
	t.Setenv("GHEALTH_TOKEN_STORAGE", "file")
	if err := config.Save(config.Config{Scopes: []string{"stored-a", "stored-b"}}); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	if err := SaveToken(&oauth2.Token{
		AccessToken: "access-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
	t.Setenv("GHEALTH_SCOPES", "environment-scope")

	status, err := CurrentStatus()
	if err != nil {
		t.Fatalf("CurrentStatus: %v", err)
	}
	if got := strings.Join(status.Scopes, ","); got != "stored-a,stored-b" {
		t.Fatalf("status scopes = %q", got)
	}
}

func TestCurrentStatusReportsMissingExpiredAndMalformedTokensTruthfully(t *testing.T) {
	t.Run("missing is unauthenticated", func(t *testing.T) {
		path := configTokenPath(t)
		status, err := CurrentStatus()
		if err != nil {
			t.Fatalf("CurrentStatus: %v", err)
		}
		if status.Authenticated || status.Valid {
			t.Fatalf("status = %#v", status)
		}
		if status.TokenPath != path {
			t.Fatalf("TokenPath = %q, want %q", status.TokenPath, path)
		}
	})

	t.Run("expired token remains an authenticated receipt", func(t *testing.T) {
		configTokenPath(t)
		if err := SaveToken(&oauth2.Token{
			AccessToken: "expired-access-token",
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(-time.Hour),
		}); err != nil {
			t.Fatalf("SaveToken: %v", err)
		}
		status, err := CurrentStatus()
		if err != nil {
			t.Fatalf("CurrentStatus: %v", err)
		}
		if !status.Authenticated || status.Valid {
			t.Fatalf("status = %#v", status)
		}
	})

	t.Run("malformed token is an error", func(t *testing.T) {
		path := configTokenPath(t)
		if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
			t.Fatal(err)
		}
		status, err := CurrentStatus()
		if err == nil || !strings.Contains(err.Error(), "decode stored OAuth token") {
			t.Fatalf("CurrentStatus error = %v", err)
		}
		if status.Authenticated || status.Valid {
			t.Fatalf("status = %#v", status)
		}
	})

	t.Run("store selection failure is an error", func(t *testing.T) {
		t.Setenv("GHEALTH_TOKEN_STORAGE", "unknown")
		if _, err := CurrentStatus(); err == nil || !strings.Contains(err.Error(), "unknown GHEALTH_TOKEN_STORAGE") {
			t.Fatalf("CurrentStatus error = %v", err)
		}
	})

	t.Run("store read failure is an error", func(t *testing.T) {
		t.Setenv("GHEALTH_TOKEN_STORAGE", "file")
		t.Setenv("GHEALTH_TOKEN_FILE", t.TempDir())
		if _, err := CurrentStatus(); err == nil || !strings.Contains(err.Error(), "read stored OAuth token") {
			t.Fatalf("CurrentStatus error = %v", err)
		}
	})
}

func TestCurrentStatusSurfacesStoredConfigFailure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GHEALTH_CONFIG_DIR", dir)
	t.Setenv("GHEALTH_TOKEN_FILE", dir+"/token.json")
	if err := SaveToken(&oauth2.Token{
		AccessToken: "access-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
	configPath, err := config.ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}

	status, err := CurrentStatus()
	if err == nil || !strings.Contains(err.Error(), "load stored authentication config") {
		t.Fatalf("CurrentStatus error = %v", err)
	}
	if !status.Authenticated || !status.Valid {
		t.Fatalf("partial status must preserve the valid token receipt: %#v", status)
	}
}

func TestRevokeLocalReportsWhetherTokenExisted(t *testing.T) {
	configTokenPath(t)
	deleted, err := RevokeLocal()
	if err != nil {
		t.Fatalf("RevokeLocal before SaveToken: %v", err)
	}
	if deleted {
		t.Fatal("RevokeLocal before SaveToken reported deletion")
	}
	if err := SaveToken(&oauth2.Token{AccessToken: "access-token"}); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
	deleted, err = RevokeLocal()
	if err != nil {
		t.Fatalf("RevokeLocal after SaveToken: %v", err)
	}
	if !deleted {
		t.Fatal("RevokeLocal after SaveToken did not report deletion")
	}
	deleted, err = RevokeLocal()
	if err != nil {
		t.Fatalf("second RevokeLocal: %v", err)
	}
	if deleted {
		t.Fatal("second RevokeLocal reported deletion")
	}

	t.Setenv("GHEALTH_TOKEN_STORAGE", "unknown")
	deleted, err = RevokeLocal()
	if err == nil || !strings.Contains(err.Error(), "unknown GHEALTH_TOKEN_STORAGE") {
		t.Fatalf("RevokeLocal with invalid store error = %v", err)
	}
	if deleted {
		t.Fatal("RevokeLocal with invalid store reported deletion")
	}
}

func TestOAuthCallbackErrorMessageRemovesControlCharacters(t *testing.T) {
	message := oauthCallbackErrorMessage(
		"access_denied\r\n\x1b[31m\u202e",
		"provider\tmessage\u2028with\u200bformatting",
	)
	if !strings.Contains(message, "access_denied") || !strings.Contains(message, "provider message") {
		t.Fatalf("message = %q", message)
	}
	for _, r := range message {
		if r < 0x20 || r == 0x7f || r == '\u2028' || r == '\u202e' || r == '\u200b' {
			t.Fatalf("message contains unsafe rune %U: %q", r, message)
		}
	}

	long := strings.Repeat("a", 600)
	if got := []rune(sanitizeOAuthText(long)); len(got) != 515 {
		t.Fatalf("sanitized long message rune count = %d, want 515", len(got))
	}
}

func TestLoginSanitizesProviderCallbackError(t *testing.T) {
	redirectURL := availableRedirectURL(t)
	authURLCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		_, err := Login(context.Background(), config.Config{
			ClientID:    "client-id",
			RedirectURL: redirectURL,
		}, LoginOptions{
			Timeout: 3 * time.Second,
			OnAuthURL: func(rawURL string) {
				authURLCh <- rawURL
			},
		})
		errCh <- err
	}()

	authURL := <-authURLCh
	parsedAuthURL, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	query := url.Values{
		"state":             {parsedAuthURL.Query().Get("state")},
		"error":             {"access_denied\r\n\x1b[31m"},
		"error_description": {"provider\tmessage\u2028with\u200bformatting"},
	}
	response, err := http.Get(redirectURL + "?" + query.Encode())
	if err != nil {
		t.Fatalf("provider callback: %v", err)
	}
	responseBody, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("callback status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}

	loginErr := <-errCh
	if loginErr == nil {
		t.Fatal("Login returned no provider callback error")
	}
	for label, value := range map[string]string{
		"response": string(responseBody),
		"error":    loginErr.Error(),
	} {
		for _, r := range value {
			if r == '\x1b' || r == '\u2028' || r == '\u202e' || r == '\u200b' {
				t.Fatalf("%s contains unsafe rune %U: %q", label, r, value)
			}
		}
	}
	if !strings.Contains(loginErr.Error(), "access_denied") || !strings.Contains(loginErr.Error(), "provider message") {
		t.Fatalf("Login error = %q", loginErr)
	}
}

func availableRedirectURL(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("http://127.0.0.1:%d/callback", port)
}

func configTokenPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/token.json"
	t.Setenv("GHEALTH_CONFIG_DIR", dir)
	t.Setenv("GHEALTH_TOKEN_FILE", path)
	return path
}

func jsonResponse(t *testing.T, payload any) *http.Response {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}
