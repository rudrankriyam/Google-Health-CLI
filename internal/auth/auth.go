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
	"path/filepath"
	"runtime"
	"strings"
	"time"

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
	return oauthCfg.TokenSource(ctx, token), nil
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
	redirect, err := url.Parse(oauthCfg.RedirectURL)
	if err != nil {
		return nil, err
	}
	if redirect.Scheme != "http" {
		return nil, errors.New("login currently requires an http localhost redirect URI")
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
	mux := http.NewServeMux()
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	listener, err := net.Listen("tcp", redirect.Host)
	if err != nil {
		return nil, err
	}
	mux.HandleFunc(redirect.Path, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("state"); got != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			errCh <- errors.New("OAuth state mismatch")
			return
		}
		if value := r.URL.Query().Get("error"); value != "" {
			http.Error(w, value, http.StatusBadRequest)
			errCh <- fmt.Errorf("OAuth error: %s", value)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- errors.New("OAuth callback did not include a code")
			return
		}
		fmt.Fprintln(w, "ghealth login complete. You can close this tab.")
		codeCh <- code
	})
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	defer server.Shutdown(context.Background())

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
		cfg.Scopes = oauthCfg.Scopes
		_ = config.Save(cfg)
		return token, nil
	case err := <-errCh:
		return nil, err
	case <-timer.C:
		return nil, errors.New("timed out waiting for OAuth callback")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func AuthURL(cfg config.Config, scopes []string) (string, error) {
	oauthCfg, err := OAuthConfig(cfg, scopes)
	if err != nil {
		return "", err
	}
	codeChal, _, err := pkce()
	if err != nil {
		return "", err
	}
	state, err := randomString(32)
	if err != nil {
		return "", err
	}
	return oauthCfg.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.SetAuthURLParam("code_challenge", codeChal),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	), nil
}

func LoadToken() (*oauth2.Token, error) {
	path, err := config.TokenPath()
	if err != nil {
		return nil, err
	}
	bytes, err := os.ReadFile(path)
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
	path, err := config.TokenPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	bytes, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	bytes = append(bytes, '\n')
	return os.WriteFile(path, bytes, 0o600)
}

func RevokeLocal() error {
	path, err := config.TokenPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func CurrentStatus() Status {
	path, err := config.TokenPath()
	status := Status{TokenPath: path}
	if err != nil {
		return status
	}
	token, err := LoadToken()
	if err != nil {
		return status
	}
	status.Authenticated = true
	status.Expiry = token.Expiry
	status.Valid = token.Valid() || token.RefreshToken != ""
	return status
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
