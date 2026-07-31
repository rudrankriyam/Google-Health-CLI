package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultBaseURL     = "https://health.googleapis.com"
	DefaultRedirectURL = "http://127.0.0.1:3000/callback"
	DefaultUser        = "users/me"
)

type Config struct {
	BaseURL      string   `json:"baseURL,omitempty"`
	User         string   `json:"user,omitempty"`
	Project      string   `json:"project,omitempty"`
	ClientID     string   `json:"clientID,omitempty"`
	ClientSecret string   `json:"clientSecret,omitempty"`
	RedirectURL  string   `json:"redirectURL,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
}

// PublicConfig is the safe-to-display representation of Config. It preserves
// whether a client secret is configured without exposing the secret itself.
type PublicConfig struct {
	BaseURL      string   `json:"baseURL,omitempty"`
	User         string   `json:"user,omitempty"`
	Project      string   `json:"project,omitempty"`
	ClientID     string   `json:"clientID,omitempty"`
	ClientSecret string   `json:"clientSecret,omitempty"`
	RedirectURL  string   `json:"redirectURL,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
}

const RedactedSecret = "[REDACTED]"

func Load() (Config, error) {
	cfg, err := LoadStored()
	if err != nil {
		return cfg, err
	}
	applyEnv(&cfg)
	cfg.normalize()
	return cfg, nil
}

// LoadStored returns only values read from disk plus defaults. Environment
// overrides are deliberately excluded so callers can update durable state
// without accidentally persisting process-local credentials or settings.
func LoadStored() (Config, error) {
	cfg := Config{
		BaseURL:     DefaultBaseURL,
		User:        DefaultUser,
		RedirectURL: DefaultRedirectURL,
	}

	path, err := ConfigPath()
	if err != nil {
		return cfg, err
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return cfg, err
		}
	} else if len(bytes) > 0 {
		if err := json.Unmarshal(bytes, &cfg); err != nil {
			return cfg, err
		}
	}

	cfg.normalize()
	return cfg, nil
}

func Save(cfg Config) error {
	cfg.normalize()
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	bytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	bytes = append(bytes, '\n')
	return WritePrivateFile(path, bytes)
}

// SaveScopes updates the durable scope receipt without persisting any
// environment or command-line overrides present in an effective Config.
func SaveScopes(scopes []string) error {
	cfg, err := LoadStored()
	if err != nil {
		return err
	}
	cfg.Scopes = append([]string(nil), scopes...)
	return Save(cfg)
}

// Redacted returns a representation suitable for CLI and diagnostic output.
func (cfg Config) Redacted() PublicConfig {
	secret := ""
	if strings.TrimSpace(cfg.ClientSecret) != "" {
		secret = RedactedSecret
	}
	return PublicConfig{
		BaseURL:      cfg.BaseURL,
		User:         cfg.User,
		Project:      cfg.Project,
		ClientID:     cfg.ClientID,
		ClientSecret: secret,
		RedirectURL:  cfg.RedirectURL,
		Scopes:       append([]string(nil), cfg.Scopes...),
	}
}

// WritePrivateFile atomically replaces path with a mode-0600 file. Writing a
// new inode before rename also repairs permissions when an older destination
// was too permissive and avoids exposing partially written credentials.
func WritePrivateFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	temporary, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()

	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace private file: %w", err)
	}
	return nil
}

func ConfigDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("GHEALTH_CONFIG_DIR")); dir != "" {
		return dir, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ghealth"), nil
}

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func TokenPath() (string, error) {
	if path := strings.TrimSpace(os.Getenv("GHEALTH_TOKEN_FILE")); path != "" {
		return path, nil
	}
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "token.json"), nil
}

func applyEnv(cfg *Config) {
	if value := strings.TrimSpace(os.Getenv("GHEALTH_BASE_URL")); value != "" {
		cfg.BaseURL = value
	}
	if value := strings.TrimSpace(os.Getenv("GHEALTH_USER")); value != "" {
		cfg.User = value
	}
	if value := strings.TrimSpace(os.Getenv("GHEALTH_PROJECT")); value != "" {
		cfg.Project = value
	}
	if value := strings.TrimSpace(os.Getenv("GHEALTH_CLIENT_ID")); value != "" {
		cfg.ClientID = value
	}
	if value := strings.TrimSpace(os.Getenv("GHEALTH_CLIENT_SECRET")); value != "" {
		cfg.ClientSecret = value
	}
	if value := strings.TrimSpace(os.Getenv("GHEALTH_REDIRECT_URI")); value != "" {
		cfg.RedirectURL = value
	}
	if value := strings.TrimSpace(os.Getenv("GHEALTH_SCOPES")); value != "" {
		cfg.Scopes = splitCSV(value)
	}
}

func (cfg *Config) normalize() {
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	cfg.User = strings.Trim(strings.TrimSpace(cfg.User), "/")
	if cfg.User == "" {
		cfg.User = DefaultUser
	}
	cfg.Project = strings.Trim(strings.TrimSpace(cfg.Project), "/")
	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	cfg.ClientSecret = strings.TrimSpace(cfg.ClientSecret)
	cfg.RedirectURL = strings.TrimSpace(cfg.RedirectURL)
	if cfg.RedirectURL == "" {
		cfg.RedirectURL = DefaultRedirectURL
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
