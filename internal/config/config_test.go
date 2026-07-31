package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GHEALTH_CONFIG_DIR", dir)
	// Make sure environment overrides do not leak into assertions.
	for _, key := range []string{
		"GHEALTH_BASE_URL", "GHEALTH_USER", "GHEALTH_PROJECT",
		"GHEALTH_CLIENT_ID", "GHEALTH_CLIENT_SECRET", "GHEALTH_REDIRECT_URI", "GHEALTH_SCOPES",
	} {
		t.Setenv(key, "")
	}
	return dir
}

func TestSaveLoadRoundTrip(t *testing.T) {
	setConfigDir(t)

	saved := Config{
		BaseURL:      "https://example.com/api/",
		User:         "/users/someone/",
		Project:      "my-project",
		ClientID:     " client-id ",
		ClientSecret: "client-secret",
		RedirectURL:  "http://127.0.0.1:9999/callback",
		Scopes:       []string{"scope-a", "scope-b"},
	}
	if err := Save(saved); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.BaseURL != "https://example.com/api" {
		t.Fatalf("BaseURL = %q", loaded.BaseURL)
	}
	if loaded.User != "users/someone" {
		t.Fatalf("User = %q", loaded.User)
	}
	if loaded.Project != "my-project" {
		t.Fatalf("Project = %q", loaded.Project)
	}
	if loaded.ClientID != "client-id" {
		t.Fatalf("ClientID = %q", loaded.ClientID)
	}
	if loaded.ClientSecret != "client-secret" {
		t.Fatalf("ClientSecret = %q", loaded.ClientSecret)
	}
	if loaded.RedirectURL != "http://127.0.0.1:9999/callback" {
		t.Fatalf("RedirectURL = %q", loaded.RedirectURL)
	}
	if len(loaded.Scopes) != 2 || loaded.Scopes[0] != "scope-a" || loaded.Scopes[1] != "scope-b" {
		t.Fatalf("Scopes = %#v", loaded.Scopes)
	}
}

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	setConfigDir(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseURL != DefaultBaseURL {
		t.Fatalf("BaseURL = %q, want %q", cfg.BaseURL, DefaultBaseURL)
	}
	if cfg.User != DefaultUser {
		t.Fatalf("User = %q, want %q", cfg.User, DefaultUser)
	}
	if cfg.RedirectURL != DefaultRedirectURL {
		t.Fatalf("RedirectURL = %q, want %q", cfg.RedirectURL, DefaultRedirectURL)
	}
}

func TestLoadCorruptFileReturnsError(t *testing.T) {
	dir := setConfigDir(t)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("Load of corrupt config: expected error")
	}
}

func TestLoadStoredExcludesEnvironmentOverrides(t *testing.T) {
	setConfigDir(t)
	if err := Save(Config{
		BaseURL:      "https://stored.example",
		ClientID:     "stored-client",
		ClientSecret: "stored-secret",
		Scopes:       []string{"stored-scope"},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	t.Setenv("GHEALTH_BASE_URL", "https://effective.example")
	t.Setenv("GHEALTH_CLIENT_ID", "effective-client")
	t.Setenv("GHEALTH_CLIENT_SECRET", "effective-secret")
	t.Setenv("GHEALTH_SCOPES", "effective-a,effective-b")

	effective, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if effective.BaseURL != "https://effective.example" || effective.ClientID != "effective-client" || effective.ClientSecret != "effective-secret" {
		t.Fatalf("effective config did not apply environment: %#v", effective)
	}
	if got := strings.Join(effective.Scopes, ","); got != "effective-a,effective-b" {
		t.Fatalf("effective scopes = %q", got)
	}

	stored, err := LoadStored()
	if err != nil {
		t.Fatalf("LoadStored: %v", err)
	}
	if stored.BaseURL != "https://stored.example" || stored.ClientID != "stored-client" || stored.ClientSecret != "stored-secret" {
		t.Fatalf("stored config contains environment overrides: %#v", stored)
	}
	if got := strings.Join(stored.Scopes, ","); got != "stored-scope" {
		t.Fatalf("stored scopes = %q", got)
	}
}

func TestRedactedConfigNeverSerializesClientSecret(t *testing.T) {
	cfg := Config{
		BaseURL:      "https://health.googleapis.com",
		ClientID:     "client-id",
		ClientSecret: "super-secret-value",
		Scopes:       []string{"scope-a"},
	}
	public := cfg.Redacted()
	encoded, err := json.Marshal(public)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(encoded), cfg.ClientSecret) {
		t.Fatalf("redacted config exposed secret: %s", encoded)
	}
	if public.ClientSecret != RedactedSecret {
		t.Fatalf("ClientSecret = %q, want %q", public.ClientSecret, RedactedSecret)
	}

	if got := (Config{}).Redacted().ClientSecret; got != "" {
		t.Fatalf("empty secret redaction = %q, want empty", got)
	}
}

func TestSaveScopesUpdatesOnlyStoredScopes(t *testing.T) {
	setConfigDir(t)
	if err := Save(Config{
		BaseURL:      "https://stored.example",
		ClientID:     "stored-client",
		ClientSecret: "stored-secret",
		Scopes:       []string{"old-scope"},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	t.Setenv("GHEALTH_BASE_URL", "https://effective.example")
	t.Setenv("GHEALTH_CLIENT_SECRET", "effective-secret")

	if err := SaveScopes([]string{"new-a", "new-b"}); err != nil {
		t.Fatalf("SaveScopes: %v", err)
	}
	stored, err := LoadStored()
	if err != nil {
		t.Fatalf("LoadStored: %v", err)
	}
	if stored.BaseURL != "https://stored.example" || stored.ClientID != "stored-client" || stored.ClientSecret != "stored-secret" {
		t.Fatalf("SaveScopes changed stored config: %#v", stored)
	}
	if got := strings.Join(stored.Scopes, ","); got != "new-a,new-b" {
		t.Fatalf("stored scopes = %q", got)
	}
}

func TestSaveAtomicallyRepairsConfigPermissions(t *testing.T) {
	dir := setConfigDir(t)
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Save(Config{ClientID: "client-id"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %v, want 0600", got)
	}
	files, err := filepath.Glob(filepath.Join(dir, ".config.json.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("temporary files remain after Save: %v", files)
	}
}
