package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/Google-Health-CLI/internal/healthapi"
	"github.com/rudrankriyam/Google-Health-CLI/internal/registry"
)

func TestRunTypesList(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithWriters([]string{"types", "list"}, "test", &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("heart-rate-variability")) {
		t.Fatalf("types output missing HRV: %s", stdout.String())
	}
}

func TestRunAgentManifestIsJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithWriters([]string{"agent", "manifest"}, "test", &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"name": "ghealth"`)) {
		t.Fatalf("manifest output = %s", stdout.String())
	}
}

func TestUnknownCommandIsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWithWriters([]string{"wat"}, "test", &stdout, &stderr)
	if code != ExitUsage {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
}

func TestAPICallWithoutTokenReturnsJSONAuthError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer
	code := RunWithWriters([]string{
		"data", "list", "steps",
		"--from", "2026-05-08",
		"--to", "2026-05-09",
	}, "test", &stdout, &stderr)
	if code != ExitAuth {
		t.Fatalf("exit = %d, stdout = %s, stderr = %s", code, stdout.String(), stderr.String())
	}

	var payload map[string]string
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("stderr is not JSON: %v\n%s", err, stderr.String())
	}
	if payload["status"] != "error" {
		t.Fatalf("status = %q, payload = %#v", payload["status"], payload)
	}
	if payload["message"] == "" || payload["message"] == "unknown error" {
		t.Fatalf("message = %q, payload = %#v", payload["message"], payload)
	}
}

func TestCivilDateTimeValid(t *testing.T) {
	value, err := civilDateTime("2026-05-08")
	if err != nil {
		t.Fatalf("civilDateTime: %v", err)
	}
	date, ok := value["date"].(map[string]any)
	if !ok {
		t.Fatalf("date missing: %#v", value)
	}
	if date["year"] != 2026 || date["month"] != 5 || date["day"] != 8 {
		t.Fatalf("date = %#v", date)
	}
	if _, hasTime := value["time"]; hasTime {
		t.Fatalf("unexpected time component: %#v", value)
	}

	value, err = civilDateTime("2026-12-31T23:59:07Z")
	if err != nil {
		t.Fatalf("civilDateTime with time: %v", err)
	}
	clock, ok := value["time"].(map[string]any)
	if !ok {
		t.Fatalf("time missing: %#v", value)
	}
	if clock["hours"] != 23 || clock["minutes"] != 59 || clock["seconds"] != 7 {
		t.Fatalf("time = %#v", clock)
	}
}

func TestCivilDateTimeInvalid(t *testing.T) {
	cases := []struct {
		input   string
		wantErr string
	}{
		{"2026-13-01", "invalid month"},
		{"2026-00-01", "invalid month"},
		{"2026-01-32", "invalid day"},
		{"2026-01-00", "invalid day"},
		{"2026-02-29", "invalid calendar date"},
		{"2026-01-02T24:00:00", "invalid hour"},
		{"2026-01-02T23:60:00", "invalid minute"},
		{"2026-01-02T23:59:60", "invalid second"},
		{"2026-01", "expected YYYY-MM-DD"},
		{"2026-01-02T25", "expected time"},
		{"2026-0a-01", "invalid number"},
	}
	for _, tc := range cases {
		_, err := civilDateTime(tc.input)
		if err == nil {
			t.Fatalf("civilDateTime(%q): expected error", tc.input)
		}
		if !strings.Contains(err.Error(), tc.wantErr) {
			t.Fatalf("civilDateTime(%q) error = %q, want substring %q", tc.input, err, tc.wantErr)
		}
	}
}

func TestDataSafetyValidationRunsBeforeAuthentication(t *testing.T) {
	t.Setenv("GHEALTH_CONFIG_DIR", t.TempDir())
	t.Setenv("GHEALTH_TOKEN_FILE", filepath.Join(t.TempDir(), "missing-token.json"))

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "delete confirmation", args: []string{"data", "delete", "weight"}, want: "without --yes"},
		{name: "unsupported operation", args: []string{"data", "get", "steps", "point"}, want: "does not support get"},
		{name: "unsupported filter", args: []string{"data", "list", "food", "--from", "2026-07-31"}, want: "does not support time-range filters"},
		{name: "raw mutation confirmation", args: []string{"api", "POST", "/v4/example", "--no-auth"}, want: "without --yes"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := RunWithWriters(test.args, "test", &stdout, &stderr)
			if code != ExitUsage {
				t.Fatalf("exit = %d, stdout = %s, stderr = %s", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("stderr = %s, want %q", stderr.String(), test.want)
			}
		})
	}
}

func TestHelpAndVersionWorkWithCorruptConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GHEALTH_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := RunWithWriters([]string{"help"}, "test", &stdout, &stderr); code != ExitSuccess {
		t.Fatalf("help exit = %d, stderr = %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Usage:")) {
		t.Fatalf("help output = %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := RunWithWriters([]string{"--version"}, "1.2.3-test", &stdout, &stderr); code != ExitSuccess {
		t.Fatalf("version exit = %d, stderr = %s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "1.2.3-test" {
		t.Fatalf("version output = %q", stdout.String())
	}

	// Other commands still surface the config error.
	stdout.Reset()
	stderr.Reset()
	if code := RunWithWriters([]string{"types", "list"}, "test", &stdout, &stderr); code == ExitSuccess {
		t.Fatalf("types list with corrupt config: expected failure, stdout = %s", stdout.String())
	}
}

func TestEveryCommandGroupHelpIsAuthFree(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GHEALTH_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := [][]string{
		{"doctor", "--help"},
		{"auth", "login", "--help"},
		{"config", "set", "--help"},
		{"types", "describe", "--help"},
		{"endpoints", "list", "--help"},
		{"agent", "schema", "--help"},
		{"profile", "update", "--help"},
		{"settings", "update", "--help"},
		{"identity", "get", "--help"},
		{"irn-profile", "get", "--help"},
		{"paired-devices", "list", "--help"},
		{"data", "list", "--help"},
		{"rollup", "daily", "--help"},
		{"subscribers", "create", "--help"},
		{"subscribers", "subscriptions", "create", "--help"},
		{"api", "--help"},
	}

	for _, args := range tests {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := RunWithWriters(args, "test", &stdout, &stderr); code != ExitSuccess {
				t.Fatalf("exit = %d, stdout = %s, stderr = %s", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stdout.String(), "Usage:") {
				t.Fatalf("help output = %s", stdout.String())
			}
		})
	}
}

func TestSubcommandHelpDocumentsActualFlags(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{args: []string{"auth", "login", "--help"}, want: []string{"--client-id", "--redirect-uri", "--scope", "GHEALTH_CLIENT_SECRET"}},
		{args: []string{"data", "list", "--help"}, want: []string{"--filter", "--limit", "--page-token", "[--from TIME]"}},
		{args: []string{"data", "reconcile", "--help"}, want: []string{"--family", "--filter", "--page-token"}},
		{args: []string{"data", "export-tcx", "--help"}, want: []string{"--partial-data"}},
		{args: []string{"rollup", "daily", "--help"}, want: []string{"--file", "--family", "--page-size", "--window-days"}},
		{args: []string{"rollup", "physical", "--help"}, want: []string{"--file", "--family", "--page-token", "--window-size"}},
		{args: []string{"api", "GET", "--help"}, want: []string{"--raw", "--no-auth", "--yes"}},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.args[:len(test.args)-1], "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := RunWithWriters(test.args, "test", &stdout, &stderr); code != ExitSuccess {
				t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
			}
			for _, flag := range test.want {
				if !strings.Contains(stdout.String(), flag) {
					t.Fatalf("help output missing %q:\n%s", flag, stdout.String())
				}
			}
		})
	}
}

func TestConfigIndependentRequestsAreExact(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GHEALTH_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := RunWithWriters([]string{"types", "list", "version"}, "test", &stdout, &stderr); code == ExitSuccess {
		t.Fatalf("positional version bypassed corrupt config: stdout = %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := RunWithWriters([]string{"types", "list", "help"}, "test", &stdout, &stderr); code == ExitSuccess {
		t.Fatalf("positional help bypassed corrupt config: stdout = %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := RunWithWriters([]string{"--format", "json", "version"}, "1.2.3-test", &stdout, &stderr); code != ExitSuccess {
		t.Fatalf("global version exit = %d, stderr = %s", code, stderr.String())
	}
}

func TestAuthRevokeWorksWithCorruptConfigAndReportsNoToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GHEALTH_CONFIG_DIR", dir)
	t.Setenv("GHEALTH_TOKEN_FILE", filepath.Join(dir, "missing-token.json"))
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := RunWithWriters([]string{"auth", "revoke", "--yes"}, "test", &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	var receipt map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
		t.Fatalf("decode receipt: %v\n%s", err, stdout.String())
	}
	if receipt["localTokenDeleted"] != false || receipt["remoteCredentialRevoked"] != false || receipt["revocationScope"] != "local" {
		t.Fatalf("receipt = %#v", receipt)
	}

	stdout.Reset()
	stderr.Reset()
	if code := RunWithWriters([]string{"auth", "revoke", "--yes", "extra"}, "test", &stdout, &stderr); code == ExitSuccess {
		t.Fatalf("non-exact revoke bypassed corrupt config: stdout = %s", stdout.String())
	}
}

func TestClientSecretHasNoArgumentOrConfigSetSurface(t *testing.T) {
	t.Setenv("GHEALTH_CONFIG_DIR", t.TempDir())

	var stdout, stderr bytes.Buffer
	for _, args := range [][]string{{"auth", "login", "--help"}, {"config", "set", "--help"}} {
		stdout.Reset()
		stderr.Reset()
		if code := RunWithWriters(args, "test", &stdout, &stderr); code != ExitSuccess {
			t.Fatalf("%v exit = %d, stderr = %s", args, code, stderr.String())
		}
		if strings.Contains(stdout.String(), "--client-secret") || strings.Contains(stdout.String(), "client-secret|") {
			t.Fatalf("%v help exposed a client-secret input:\n%s", args, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	if code := RunWithWriters([]string{"auth", "login", "--client-secret", "secret"}, "test", &stdout, &stderr); code == ExitSuccess {
		t.Fatalf("auth login accepted --client-secret: stdout = %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := RunWithWriters([]string{"config", "set", "client-secret", "secret"}, "test", &stdout, &stderr); code != ExitUsage {
		t.Fatalf("config set client-secret exit = %d, stderr = %s", code, stderr.String())
	}
}

func TestPageSizeMaximaFailBeforeCredentials(t *testing.T) {
	t.Setenv("GHEALTH_CONFIG_DIR", t.TempDir())
	t.Setenv("GHEALTH_TOKEN_FILE", filepath.Join(t.TempDir(), "missing-token.json"))
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "sleep", args: []string{"data", "list", "sleep", "--limit", "26"}, want: "must not exceed 25"},
		{name: "general data", args: []string{"data", "list", "steps", "--limit", "10001"}, want: "must not exceed 10000"},
		{name: "paired devices", args: []string{"paired-devices", "list", "--limit", "101"}, want: "must not exceed 100"},
		{name: "subscribers", args: []string{"subscribers", "list", "--project", "123", "--limit", "1001"}, want: "must not exceed 1000"},
		{name: "subscriptions", args: []string{"subscribers", "subscriptions", "list", "--parent", "projects/123/subscribers/example", "--limit", "1001"}, want: "must not exceed 1000"},
		{name: "rollup", args: []string{"rollup", "daily", "steps", "--from", "2026-01-01", "--to", "2026-01-02", "--page-size", "10001"}, want: "between 0 and 10000"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := RunWithWriters(test.args, "test", &stdout, &stderr)
			if code != ExitUsage || !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("exit = %d, stderr = %s, want %q", code, stderr.String(), test.want)
			}
		})
	}
}

func TestRollUpValidationMatchesProviderLimits(t *testing.T) {
	heartRate, err := registry.MustLookup("heart-rate")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := registry.MustLookup("steps")
	if err != nil {
		t.Fatal(err)
	}
	civil := func(year, month, day int32) *healthapi.CivilDateTime {
		return &healthapi.CivilDateTime{Date: &healthapi.Date{Year: year, Month: month, Day: day}}
	}

	dailyTests := []struct {
		name    string
		request healthapi.DailyRollUpDataPointsRequest
		set     bool
		want    string
	}{
		{name: "missing end", request: healthapi.DailyRollUpDataPointsRequest{Range: &healthapi.CivilTimeInterval{Start: civil(2026, 1, 1)}}, want: "both start and end"},
		{name: "reverse", request: healthapi.DailyRollUpDataPointsRequest{Range: &healthapi.CivilTimeInterval{Start: civil(2026, 1, 2), End: civil(2026, 1, 1)}}, want: "end after"},
		{name: "fourteen day maximum", request: healthapi.DailyRollUpDataPointsRequest{Range: &healthapi.CivilTimeInterval{Start: civil(2026, 1, 1), End: civil(2026, 1, 16)}}, want: "14 days"},
		{name: "window exceeds range", request: healthapi.DailyRollUpDataPointsRequest{Range: &healthapi.CivilTimeInterval{Start: civil(2026, 1, 1), End: civil(2026, 1, 3)}, WindowSizeDays: 3}, set: true, want: "must not exceed"},
		{name: "huge window does not overflow", request: healthapi.DailyRollUpDataPointsRequest{Range: &healthapi.CivilTimeInterval{Start: civil(2026, 1, 1), End: civil(2026, 1, 3)}, WindowSizeDays: 2_147_483_647}, set: true, want: "must not exceed"},
	}
	for _, test := range dailyTests {
		t.Run("daily "+test.name, func(t *testing.T) {
			err := validateDailyRollUpRequest(heartRate, test.request, test.set)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}

	physicalTests := []struct {
		name    string
		request healthapi.RollUpDataPointsRequest
		want    string
	}{
		{name: "missing end", request: healthapi.RollUpDataPointsRequest{Range: &healthapi.Interval{StartTime: "2026-01-01T00:00:00Z"}, WindowSize: "1s"}, want: "both startTime and endTime"},
		{name: "reverse", request: healthapi.RollUpDataPointsRequest{Range: &healthapi.Interval{StartTime: "2026-01-02T00:00:00Z", EndTime: "2026-01-01T00:00:00Z"}, WindowSize: "1s"}, want: "end after"},
		{name: "subsecond", request: healthapi.RollUpDataPointsRequest{Range: &healthapi.Interval{StartTime: "2026-01-01T00:00:00Z", EndTime: "2026-01-02T00:00:00Z"}, WindowSize: "0.5s"}, want: "at least 1 second"},
		{name: "non-Google duration", request: healthapi.RollUpDataPointsRequest{Range: &healthapi.Interval{StartTime: "2026-01-01T00:00:00Z", EndTime: "2026-01-02T00:00:00Z"}, WindowSize: "1m0s"}, want: "invalid"},
		{name: "window exceeds range", request: healthapi.RollUpDataPointsRequest{Range: &healthapi.Interval{StartTime: "2026-01-01T00:00:00Z", EndTime: "2026-01-02T00:00:00Z"}, WindowSize: "86401s"}, want: "must not exceed"},
		{name: "ninety day maximum", request: healthapi.RollUpDataPointsRequest{Range: &healthapi.Interval{StartTime: "2026-01-01T00:00:00Z", EndTime: "2026-04-02T00:00:00Z"}, WindowSize: "86400s"}, want: "90 days"},
	}
	for _, test := range physicalTests {
		t.Run("physical "+test.name, func(t *testing.T) {
			err := validatePhysicalRollUpRequest(steps, test.request)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRollUpFileValidatesTypedRangeBeforeCredentials(t *testing.T) {
	t.Setenv("GHEALTH_CONFIG_DIR", t.TempDir())
	t.Setenv("GHEALTH_TOKEN_FILE", filepath.Join(t.TempDir(), "missing-token.json"))
	file := filepath.Join(t.TempDir(), "rollup.json")
	if err := os.WriteFile(file, []byte(`{"range":{"startTime":"2026-01-01T00:00:00Z"},"windowSize":"3600s"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := RunWithWriters([]string{"rollup", "physical", "steps", "--file", file}, "test", &stdout, &stderr)
	if code != ExitUsage || !strings.Contains(stderr.String(), "both startTime and endTime") {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
}

func TestReadInputEnforcesSixteenMiBLimit(t *testing.T) {
	file := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(file, bytes.Repeat([]byte("x"), maxInputBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	contents, err := readInput(file)
	if err != nil || len(contents) != maxInputBytes {
		t.Fatalf("exact-limit read: len = %d, err = %v", len(contents), err)
	}
	if err := os.WriteFile(file, bytes.Repeat([]byte("x"), maxInputBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readInput(file); err == nil || !strings.Contains(err.Error(), "16 MiB") {
		t.Fatalf("over-limit error = %v", err)
	}
}

func TestAPINoAuthRawWritesResponseBytes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/octet-stream")
		_, _ = writer.Write([]byte{0x00, 0x01, 0xfe, 0xff})
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	code := RunWithWriters([]string{"--base-url", server.URL, "api", "GET", "/v4/shl/r/example/token", "--no-auth", "--raw"}, "test", &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	want := []byte{0x00, 0x01, 0xfe, 0xff}
	if !bytes.Equal(stdout.Bytes(), want) {
		t.Fatalf("stdout = %v, want %v", stdout.Bytes(), want)
	}
}

func TestResolveScopesAliases(t *testing.T) {
	tests := []struct {
		name  string
		value string
		write bool
		want  []string
	}{
		{name: "read", value: "read", want: registry.ReadOnlyScopes()},
		{name: "write", value: "write", want: registry.WriteScopes()},
		{name: "all", value: "all", want: registry.AllScopes()},
		{name: "management", value: "management", want: registry.ManagementScopes()},
		{name: "compatibility write flag", value: "read", write: true, want: registry.AllScopes()},
		{name: "custom", value: "scope.one, scope.two", want: []string{"scope.one", "scope.two"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveScopes(test.value, test.write); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("resolveScopes(%q, %t) = %#v, want %#v", test.value, test.write, got, test.want)
			}
		})
	}
}

func TestConfigGetRedactsClientSecret(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GHEALTH_CONFIG_DIR", dir)
	t.Setenv("GHEALTH_CLIENT_SECRET", "")
	contents := `{"clientID":"client-id","clientSecret":"do-not-print-this"}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := RunWithWriters([]string{"config", "get"}, "test", &stdout, &stderr); code != ExitSuccess {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "do-not-print-this") {
		t.Fatalf("config output exposed client secret: %s", stdout.String())
	}
	var value map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &value); err != nil {
		t.Fatalf("decode config output: %v", err)
	}
	if got := value["clientSecret"]; got != "[REDACTED]" {
		t.Fatalf("clientSecret = %#v, want redacted marker", got)
	}
}

func TestConfigSetDoesNotPersistEnvironmentSecret(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GHEALTH_CONFIG_DIR", dir)
	t.Setenv("GHEALTH_CLIENT_SECRET", "environment-only-secret")
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"clientSecret":"legacy-stored-secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := RunWithWriters([]string{"config", "set", "user", "users/test-user"}, "test", &stdout, &stderr); code != ExitSuccess {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	contents, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "environment-only-secret") || strings.Contains(string(contents), "legacy-stored-secret") || strings.Contains(string(contents), "clientSecret") {
		t.Fatalf("stored config persisted environment secret: %s", contents)
	}
	var value map[string]any
	if err := json.Unmarshal(contents, &value); err != nil {
		t.Fatalf("decode stored config: %v", err)
	}
	if got := value["user"]; got != "users/test-user" {
		t.Fatalf("user = %#v, want users/test-user", got)
	}
}

func TestSubscriptionDeleteRequiresConfirmationBeforeCredentials(t *testing.T) {
	t.Setenv("GHEALTH_CONFIG_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := RunWithWriters([]string{
		"subscribers", "subscriptions", "delete",
		"--name", "projects/123456789/subscribers/example/subscriptions/user-one",
	}, "test", &stdout, &stderr)
	if code != ExitUsage {
		t.Fatalf("exit = %d, stdout = %s, stderr = %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "without --yes") {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestTypedSubscriberJSONRejectsObsoleteSingularDataType(t *testing.T) {
	file := filepath.Join(t.TempDir(), "subscriber.json")
	contents := `{
  "endpointUri": "https://example.com/webhooks/health",
  "endpointAuthorization": {"secret": "Bearer fixture"},
  "subscriberConfigs": [{"dataType": "steps", "subscriptionCreatePolicy": "AUTOMATIC"}]
}`
	if err := os.WriteFile(file, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	var request healthapi.CreateSubscriberRequest
	err := readJSONInto(file, &request)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("readJSONInto error = %v, want unknown-field error", err)
	}
}

func TestTypedUpdateRequestsAcceptCurrentWritableFields(t *testing.T) {
	var profile healthapi.UpdateProfileRequest
	if err := decodeJSONBytes([]byte(`{"age":35,"userConfiguredWalkingStrideLengthMm":740}`), &profile); err != nil {
		t.Fatalf("profile payload: %v", err)
	}
	if profile.Age == nil || *profile.Age != 35 {
		t.Fatalf("profile age = %#v", profile.Age)
	}

	var settings healthapi.UpdateSettingsRequest
	payload := []byte(`{
  "distanceUnit":"DISTANCE_UNIT_KILOMETERS",
  "strideLengthWalkingType":"STRIDE_LENGTH_TYPE_MANUAL",
  "strideLengthRunningType":"STRIDE_LENGTH_TYPE_AUTO",
  "timeZone":"Asia/Kolkata",
  "utcOffset":"+05:30",
  "autoStrideEnabled":false
}`)
	if err := decodeJSONBytes(payload, &settings); err != nil {
		t.Fatalf("settings payload: %v", err)
	}
	if settings.AutoStrideEnabled == nil || *settings.AutoStrideEnabled {
		t.Fatalf("autoStrideEnabled = %#v", settings.AutoStrideEnabled)
	}
}
