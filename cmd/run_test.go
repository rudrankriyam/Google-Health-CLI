package cmd

import (
	"bytes"
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
