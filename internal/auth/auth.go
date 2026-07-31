package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	pathpkg "path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/rudrankriyam/Google-Health-CLI/internal/config"
	"github.com/rudrankriyam/Google-Health-CLI/internal/registry"
)

type LoginOptions struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	OpenBrowser  bool
	Timeout      time.Duration
	OnAuthURL    func(string)
}

type Status struct {
	Authenticated bool      `json:"authenticated"`
	TokenPath     string    `json:"tokenPath"`
	Expiry        time.Time `json:"expiry,omitempty"`
	Valid         bool      `json:"valid"`
	Scopes        []string  `json:"scopes,omitempty"`
}

func OAuthConfig(cfg config.Config, scopes []string) (*oauth2.Config, error) {
	clientID := strings.TrimSpace(cfg.ClientID)
	if clientID == "" {
		return nil, errors.New("missing OAuth client ID; set GHEALTH_CLIENT_ID or run `ghealth config set client-id <value>`")
	}
	redirectURL := strings.TrimSpace(cfg.RedirectURL)
	if redirectURL == "" {
		redirectURL = config.DefaultRedirectURL
	}
	if len(scopes) == 0 {
		scopes = cfg.Scopes
	}
	if len(scopes) == 0 {
		scopes = registry.ReadOnlyScopes()
	}
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: strings.TrimSpace(cfg.ClientSecret),
		RedirectURL:  redirectURL,
		Scopes:       scopes,
		Endpoint:     google.Endpoint,
	}, nil
}

func TokenSource(ctx context.Context, cfg config.Config) (oauth2.TokenSource, error) {
	token, err := LoadToken()
	if err != nil {
		return nil, err
	}
	oauthCfg, err := OAuthConfig(cfg, nil)
	if err != nil {
		return nil, err
	}
	return &persistingTokenSource{
		source: oauthCfg.TokenSource(ctx, token),
		last:   cloneToken(token),
	}, nil
}

type persistingTokenSource struct {
	source oauth2.TokenSource

	mu   sync.Mutex
	last *oauth2.Token
}

func (s *persistingTokenSource) Token() (*oauth2.Token, error) {
	token, err := s.source.Token()
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if tokenEqual(token, s.last) {
		return token, nil
	}
	if err := SaveToken(token); err != nil {
		return nil, err
	}
	s.last = cloneToken(token)
	return token, nil
}

func cloneToken(token *oauth2.Token) *oauth2.Token {
	if token == nil {
		return nil
	}
	clone := *token
	return &clone
}

func tokenEqual(a, b *oauth2.Token) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.AccessToken == b.AccessToken &&
		a.TokenType == b.TokenType &&
		a.RefreshToken == b.RefreshToken &&
		a.Expiry.Equal(b.Expiry) &&
		a.ExpiresIn == b.ExpiresIn
}

func Login(ctx context.Context, cfg config.Config, opts LoginOptions) (*oauth2.Token, error) {
	if opts.ClientID != "" {
		cfg.ClientID = opts.ClientID
	}
	if opts.ClientSecret != "" {
		cfg.ClientSecret = opts.ClientSecret
	}
	if opts.RedirectURL != "" {
		cfg.RedirectURL = opts.RedirectURL
	}
	if len(opts.Scopes) > 0 {
		cfg.Scopes = opts.Scopes
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Minute
	}

	oauthCfg, err := OAuthConfig(cfg, cfg.Scopes)
	if err != nil {
		return nil, err
	}
	redirect, err := validateLoopbackRedirect(oauthCfg.RedirectURL)
	if err != nil {
		return nil, err
	}

	codeChal, verifier, err := pkce()
	if err != nil {
		return nil, err
	}
	state, err := randomString(32)
	if err != nil {
		return nil, err
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	// Sends are non-blocking so duplicate callback hits (e.g. a browser
	// re-requesting the redirect URL) can never block the HTTP handler;
	// only the first result is consumed.
	sendCode := func(code string) {
		select {
		case codeCh <- code:
		default:
		}
	}
	sendErr := func(err error) {
		select {
		case errCh <- err:
		default:
		}
	}
	mux := http.NewServeMux()
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	listener, err := net.Listen("tcp", redirect.Host)
	if err != nil {
		return nil, err
	}
	mux.HandleFunc(redirect.Path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if got := r.URL.Query().Get("state"); got != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			// An unsolicited or stale callback must not be able to cancel the
			// real browser flow. Keep waiting for the callback with our state.
			return
		}
		if value := r.URL.Query().Get("error"); value != "" {
			message := oauthCallbackErrorMessage(value, r.URL.Query().Get("error_description"))
			http.Error(w, message, http.StatusBadRequest)
			sendErr(fmt.Errorf("OAuth error: %s", message))
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			sendErr(errors.New("OAuth callback did not include a code"))
			return
		}
		fmt.Fprintln(w, "ghealth login complete. You can close this tab.")
		sendCode(code)
	})
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			sendErr(err)
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	authURL := oauthCfg.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.SetAuthURLParam("code_challenge", codeChal),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	if opts.OnAuthURL != nil {
		opts.OnAuthURL(authURL)
	}
	if opts.OpenBrowser {
		_ = openBrowser(authURL)
	}

	timer := time.NewTimer(opts.Timeout)
	defer timer.Stop()
	select {
	case code := <-codeCh:
		token, err := oauthCfg.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))
		if err != nil {
			return nil, err
		}
		if err := SaveToken(token); err != nil {
			return nil, err
		}
		if err := config.SaveScopes(oauthCfg.Scopes); err != nil {
			return nil, fmt.Errorf("login succeeded but saving config failed: %w", err)
		}
		return token, nil
	case err := <-errCh:
		return nil, err
	case <-timer.C:
		return nil, errors.New("timed out waiting for OAuth callback")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func LoadToken() (*oauth2.Token, error) {
	store, err := activeStore()
	if err != nil {
		return nil, err
	}
	bytes, err := store.Read()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("not logged in; run `ghealth auth login`")
		}
		return nil, err
	}
	var token oauth2.Token
	if err := json.Unmarshal(bytes, &token); err != nil {
		return nil, err
	}
	if !token.Valid() && token.RefreshToken == "" {
		return nil, errors.New("stored token is expired and has no refresh token; run `ghealth auth login` again")
	}
	return &token, nil
}

func SaveToken(token *oauth2.Token) error {
	store, err := activeStore()
	if err != nil {
		return err
	}
	bytes, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	bytes = append(bytes, '\n')
	return store.Write(bytes)
}

// RevokeLocal deletes the token from the active local store. The boolean is
// true only when a token existed and was removed. It does not revoke the token
// at Google's OAuth endpoint.
func RevokeLocal() (bool, error) {
	store, err := activeStore()
	if err != nil {
		return false, err
	}
	return store.Delete()
}

// CurrentStatus returns a truthful local authentication receipt. A missing
// token is a normal unauthenticated state; backend, token-decoding, and stored
// configuration failures are returned instead of being collapsed into it.
func CurrentStatus() (Status, error) {
	var status Status
	store, err := activeStore()
	if err != nil {
		return status, err
	}
	status.TokenPath = store.Location()

	storedConfig, configErr := config.LoadStored()
	tokenBytes, tokenErr := store.Read()
	if tokenErr != nil {
		if errors.Is(tokenErr, os.ErrNotExist) {
			if configErr != nil {
				return status, fmt.Errorf("load stored authentication config: %w", configErr)
			}
			return status, nil
		}
		if configErr != nil {
			return status, errors.Join(
				fmt.Errorf("read stored OAuth token: %w", tokenErr),
				fmt.Errorf("load stored authentication config: %w", configErr),
			)
		}
		return status, fmt.Errorf("read stored OAuth token: %w", tokenErr)
	}

	var token oauth2.Token
	if err := json.Unmarshal(tokenBytes, &token); err != nil {
		tokenErr = fmt.Errorf("decode stored OAuth token: %w", err)
	}
	if tokenErr == nil {
		status.Authenticated = true
		status.Expiry = token.Expiry
		status.Valid = token.Valid() || token.RefreshToken != ""
	}
	if tokenErr != nil || configErr != nil {
		if tokenErr != nil && configErr != nil {
			return status, errors.Join(tokenErr, fmt.Errorf("load stored authentication config: %w", configErr))
		}
		if tokenErr != nil {
			return status, tokenErr
		}
		return status, fmt.Errorf("load stored authentication config: %w", configErr)
	}

	status.Scopes = append([]string(nil), storedConfig.Scopes...)
	return status, nil
}

func oauthCallbackErrorMessage(code, description string) string {
	code = sanitizeOAuthText(code)
	description = sanitizeOAuthText(description)
	if code == "" {
		code = "unknown_error"
	}
	if description == "" {
		return code
	}
	return code + ": " + description
}

func sanitizeOAuthText(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) {
			return ' '
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	const maximumRunes = 512
	runes := []rune(value)
	if len(runes) > maximumRunes {
		value = string(runes[:maximumRunes]) + "..."
	}
	return value
}

func validateLoopbackRedirect(rawURL string) (*url.URL, error) {
	redirect, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("invalid OAuth redirect URI: %w", err)
	}
	if redirect.Scheme != "http" || redirect.Opaque != "" {
		return nil, errors.New("OAuth redirect URI must use http on a literal loopback address")
	}
	if redirect.User != nil || redirect.RawQuery != "" || redirect.ForceQuery || redirect.Fragment != "" {
		return nil, errors.New("OAuth redirect URI must not contain credentials, a query, or a fragment")
	}
	ip := net.ParseIP(redirect.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return nil, errors.New("OAuth redirect URI host must be a literal loopback address")
	}
	portText := redirect.Port()
	if portText == "" {
		return nil, errors.New("OAuth redirect URI must include an explicit port")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return nil, errors.New("OAuth redirect URI has an invalid port")
	}
	if redirect.RawPath != "" || redirect.Path == "" || redirect.Path == "/" || pathpkg.Clean(redirect.Path) != redirect.Path {
		return nil, errors.New("OAuth redirect URI must include a clean, non-root callback path")
	}
	return redirect, nil
}

func pkce() (challenge, verifier string, err error) {
	verifier, err = randomString(64)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return challenge, verifier, nil
}

func randomString(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}
