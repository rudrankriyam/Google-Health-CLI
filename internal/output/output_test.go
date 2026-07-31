package output

import (
	"bytes"
	"errors"
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

func TestPrintErrorNeutralizesTerminalControlCharacters(t *testing.T) {
	var buf bytes.Buffer
	PrintError(&buf, errors.New("provider\nforged\x1b[31m"), Options{Format: FormatTable}, "retry\rnever")
	got := buf.String()
	if strings.ContainsAny(got, "\r\x1b") || strings.Count(got, "\n") != 2 {
		t.Fatalf("PrintError preserved terminal control characters: %q", got)
	}
	if !strings.Contains(got, "provider forged [31m") || !strings.Contains(got, "retry never") {
		t.Fatalf("PrintError() = %q", got)
	}
}

func TestHumanOutputsNeutralizeControlCharactersInHeadersAndValues(t *testing.T) {
	value := map[string]any{"unsafe\nheader": "unsafe\rvalue\x1b[31m"}
	for _, format := range []string{FormatTable, FormatMarkdown} {
		t.Run(format, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Print(&buf, value, Options{Format: format}); err != nil {
				t.Fatal(err)
			}
			got := buf.String()
			if strings.ContainsAny(got, "\r\x1b") {
				t.Fatalf("Print(%s) preserved control characters: %q", format, got)
			}
			if !strings.Contains(got, "unsafe header") || !strings.Contains(got, "unsafe value [31m") {
				t.Fatalf("Print(%s) = %q", format, got)
			}
		})
	}
}
