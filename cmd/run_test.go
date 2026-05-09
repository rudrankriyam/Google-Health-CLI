package cmd

import (
	"bytes"
	"encoding/json"
	"testing"
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
		"--from", "2026-05-08T00:00:00Z",
		"--to", "2026-05-09T00:00:00Z",
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
