package output

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

const (
	FormatAuto     = "auto"
	FormatJSON     = "json"
	FormatNDJSON   = "ndjson"
	FormatTable    = "table"
	FormatMarkdown = "markdown"
	FormatCSV      = "csv"
)

type Options struct {
	Format string
	Pretty bool
}

type ErrorPayload struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

func DefaultFormat() string {
	if env := strings.TrimSpace(os.Getenv("GHEALTH_OUTPUT")); env != "" {
		return NormalizeFormat(env)
	}
	if isTerminal(os.Stdout) {
		return FormatTable
	}
	return FormatJSON
}

func NormalizeFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", FormatAuto:
		return DefaultFormat()
	case "md":
		return FormatMarkdown
	default:
		return strings.ToLower(strings.TrimSpace(format))
	}
}

func Validate(opts Options, allowed ...string) (Options, error) {
	opts.Format = NormalizeFormat(opts.Format)
	if len(allowed) == 0 {
		allowed = []string{FormatJSON, FormatNDJSON, FormatTable, FormatMarkdown, FormatCSV}
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, item := range allowed {
		allowedSet[NormalizeFormat(item)] = struct{}{}
	}
	if _, ok := allowedSet[opts.Format]; !ok {
		return opts, fmt.Errorf("unsupported output format %q", opts.Format)
	}
	if opts.Pretty && opts.Format != FormatJSON {
		return opts, errors.New("--pretty is only valid with JSON output")
	}
	return opts, nil
}

func Print(w io.Writer, value any, opts Options) error {
	opts, err := Validate(opts)
	if err != nil {
		return err
	}
	switch opts.Format {
	case FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		if opts.Pretty {
			encoder.SetIndent("", "  ")
		}
		return encoder.Encode(value)
	case FormatNDJSON:
		return printNDJSON(w, value)
	case FormatTable:
		return PrintTable(w, value)
	case FormatMarkdown:
		return PrintMarkdown(w, value)
	case FormatCSV:
		return PrintCSV(w, value)
	default:
		return fmt.Errorf("unsupported output format %q", opts.Format)
	}
}

func PrintError(w io.Writer, err error, opts Options, hint string) {
	if NormalizeFormat(opts.Format) == FormatJSON {
		_ = Print(w, ErrorPayload{Status: "error", Message: err.Error(), Hint: hint}, Options{Format: FormatJSON, Pretty: true})
		return
	}
	if hint == "" {
		fmt.Fprintf(w, "Error: %s\n", err)
		return
	}
	fmt.Fprintf(w, "Error: %s\nHint: %s\n", err, hint)
}

func PrintTable(w io.Writer, value any) error {
	rows := rowsFromValue(value)
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "No data")
		return err
	}
	headers := sortedKeys(rows[0])
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, row := range rows {
		fields := make([]string, 0, len(headers))
		for _, header := range headers {
			fields = append(fields, stringify(row[header]))
		}
		fmt.Fprintln(tw, strings.Join(fields, "\t"))
	}
	return tw.Flush()
}

func PrintMarkdown(w io.Writer, value any) error {
	rows := rowsFromValue(value)
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "No data")
		return err
	}
	headers := sortedKeys(rows[0])
	fmt.Fprintf(w, "| %s |\n", strings.Join(headers, " | "))
	separators := make([]string, len(headers))
	for i := range separators {
		separators[i] = "---"
	}
	fmt.Fprintf(w, "| %s |\n", strings.Join(separators, " | "))
	for _, row := range rows {
		fields := make([]string, 0, len(headers))
		for _, header := range headers {
			fields = append(fields, strings.ReplaceAll(stringify(row[header]), "|", "\\|"))
		}
		fmt.Fprintf(w, "| %s |\n", strings.Join(fields, " | "))
	}
	return nil
}

func PrintCSV(w io.Writer, value any) error {
	rows := rowsFromValue(value)
	cw := csv.NewWriter(w)
	if len(rows) == 0 {
		cw.Flush()
		return cw.Error()
	}
	headers := sortedKeys(rows[0])
	if err := cw.Write(headers); err != nil {
		return err
	}
	for _, row := range rows {
		record := make([]string, 0, len(headers))
		for _, header := range headers {
			record = append(record, stringify(row[header]))
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func printNDJSON(w io.Writer, value any) error {
	rows := rowsFromValue(value)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	if len(rows) == 0 {
		return encoder.Encode(value)
	}
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			return err
		}
	}
	return nil
}

func rowsFromValue(value any) []map[string]any {
	bytes, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var rows []map[string]any
	if json.Unmarshal(bytes, &rows) == nil {
		return rows
	}
	var row map[string]any
	if json.Unmarshal(bytes, &row) == nil {
		if nested := firstArrayOfObjects(row); len(nested) > 0 {
			return nested
		}
		return []map[string]any{row}
	}
	return nil
}

func firstArrayOfObjects(row map[string]any) []map[string]any {
	keys := sortedKeys(row)
	for _, key := range keys {
		values, ok := row[key].([]any)
		if !ok || len(values) == 0 {
			continue
		}
		result := make([]map[string]any, 0, len(values))
		for _, value := range values {
			item, ok := value.(map[string]any)
			if !ok {
				return nil
			}
			result = append(result, item)
		}
		return result
	}
	return nil
}

func sortedKeys(row map[string]any) []string {
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func stringify(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return sanitize(v)
	case float64, bool:
		return fmt.Sprint(v)
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return sanitize(string(bytes))
	}
}

func sanitize(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, value)
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
