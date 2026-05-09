package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintJSON(t *testing.T) {
	var buf bytes.Buffer
	err := Print(&buf, map[string]any{"ok": true}, Options{Format: FormatJSON, Pretty: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"ok": true`) {
		t.Fatalf("JSON output = %q", buf.String())
	}
}

func TestPrintNDJSONUsesRows(t *testing.T) {
	var buf bytes.Buffer
	err := Print(&buf, []map[string]any{{"name": "steps"}, {"name": "sleep"}}, Options{Format: FormatNDJSON})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("NDJSON lines = %d, want 2: %q", len(lines), buf.String())
	}
}
