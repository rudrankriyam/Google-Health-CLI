package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/rudrankriyam/Google-Health-CLI/internal/auth"
	"github.com/rudrankriyam/Google-Health-CLI/internal/config"
	"github.com/rudrankriyam/Google-Health-CLI/internal/healthapi"
	"github.com/rudrankriyam/Google-Health-CLI/internal/output"
	"github.com/rudrankriyam/Google-Health-CLI/internal/registry"
)

var (
	errUsage = errors.New("usage error")
	errAuth  = errors.New("auth error")
)

const (
	maxInputBytes           = 16 << 20
	maxDataPageSize         = 10_000
	maxSessionDataPageSize  = 25
	maxRollupPageSize       = 10_000
	maxPairedDevicePageSize = 100
	maxSubscriberPageSize   = 1_000
	maxSubscriptionPageSize = 1_000
)

type app struct {
	args    []string
	out     io.Writer
	errOut  io.Writer
	version string
	opts    output.Options
	cfg     config.Config
}

func Run(args []string, version string) int {
	a := &app{
		args:    args,
		out:     os.Stdout,
		errOut:  os.Stderr,
		version: version,
		opts:    output.Options{Format: output.FormatAuto},
	}
	err := a.loadConfigAndRun()
	if err != nil {
		output.PrintError(a.errOut, unwrapUsage(err), a.opts, hintFor(err))
	}
	return exitCodeFromError(err)
}

func RunWithWriters(args []string, version string, stdout, stderr io.Writer) int {
	a := &app{
		args:    args,
		out:     stdout,
		errOut:  stderr,
		version: version,
		opts:    output.Options{Format: output.FormatJSON},
	}
	err := a.loadConfigAndRun()
	if err != nil {
		output.PrintError(a.errOut, unwrapUsage(err), a.opts, hintFor(err))
	}
	return exitCodeFromError(err)
}

// loadConfigAndRun loads the config and executes the requested command.
// Help, version, and exact local token deletion must keep working even when
// the config file is unreadable, so a config load error is fatal elsewhere.
func (a *app) loadConfigAndRun() error {
	cfg, cfgErr := config.Load()
	a.cfg = cfg
	if cfgErr != nil && !configIndependentRequest(a.args) {
		return cfgErr
	}
	return a.run()
}

func configIndependentRequest(args []string) bool {
	commandArgs, ok := commandArgsWithoutGlobalFlags(args)
	if !ok || len(commandArgs) == 0 {
		return ok
	}
	if commandArgs[0] == "help" || commandArgs[0] == "-h" || commandArgs[0] == "--help" || commandArgs[0] == "version" || commandArgs[0] == "--version" {
		return true
	}
	if helpFlagRequested(commandArgs) {
		return true
	}
	return len(commandArgs) == 3 && commandArgs[0] == "auth" && commandArgs[1] == "revoke" &&
		(commandArgs[2] == "--yes" || commandArgs[2] == "--yes=true")
}

// commandArgsWithoutGlobalFlags mirrors parseGlobal without loading or
// mutating configuration. The boolean is false when a value-taking global
// flag is incomplete, because run will report that usage error before help.
func commandArgsWithoutGlobalFlags(args []string) ([]string, bool) {
	result := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		key, _, hasInlineValue := strings.Cut(arg, "=")
		switch key {
		case "--json", "--pretty":
			continue
		case "--format", "--output", "--base-url", "--user", "--project":
			if !hasInlineValue {
				if i+1 >= len(args) {
					return nil, false
				}
				i++
			}
		default:
			result = append(result, arg)
		}
	}
	return result, true
}

func (a *app) run() error {
	args, err := a.parseGlobal(a.args)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return a.help()
	}
	if helpFlagRequested(args) {
		return a.helpFor(helpPath(args))
	}
	switch args[0] {
	case "-h", "--help", "help":
		return a.helpFor(args[1:])
	case "--version", "version":
		fmt.Fprintln(a.out, a.version)
		return nil
	case "doctor":
		return a.doctor(args[1:])
	case "auth":
		return a.auth(args[1:])
	case "config":
		return a.config(args[1:])
	case "types":
		return a.types(args[1:])
	case "endpoints":
		return a.endpoints(args[1:])
	case "agent":
		return a.agent(args[1:])
	case "profile":
		return a.profile(args[1:])
	case "settings":
		return a.settings(args[1:])
	case "identity":
		return a.identity(args[1:])
	case "irn-profile":
		return a.irnProfile(args[1:])
	case "paired-devices":
		return a.pairedDevices(args[1:])
	case "data":
		return a.data(args[1:])
	case "rollup":
		return a.rollup(args[1:])
	case "subscribers":
		return a.subscribers(args[1:])
	case "api":
		return a.api(args[1:])
	default:
		return usagef("unknown command %q", args[0])
	}
}

func helpFlagRequested(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

func helpPath(args []string) []string {
	path := make([]string, 0, 3)
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			break
		}
		path = append(path, arg)
		if len(path) == 2 && path[0] != "subscribers" {
			break
		}
		if len(path) == 3 {
			break
		}
	}
	return path
}

func (a *app) parseGlobal(args []string) ([]string, error) {
	result := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		key, inlineValue, hasInlineValue := strings.Cut(arg, "=")
		switch key {
		case "--json":
			a.opts.Format = output.FormatJSON
		case "--pretty":
			a.opts.Pretty = true
		case "--format", "--output":
			value := inlineValue
			if !hasInlineValue {
				if i+1 >= len(args) {
					return nil, usagef("%s requires a value", arg)
				}
				i++
				value = args[i]
			}
			a.opts.Format = value
		case "--base-url":
			value := inlineValue
			if !hasInlineValue {
				if i+1 >= len(args) {
					return nil, usagef("--base-url requires a value")
				}
				i++
				value = args[i]
			}
			a.cfg.BaseURL = strings.TrimRight(value, "/")
		case "--user":
			value := inlineValue
			if !hasInlineValue {
				if i+1 >= len(args) {
					return nil, usagef("--user requires a value")
				}
				i++
				value = args[i]
			}
			a.cfg.User = strings.Trim(value, "/")
		case "--project":
			value := inlineValue
			if !hasInlineValue {
				if i+1 >= len(args) {
					return nil, usagef("--project requires a value")
				}
				i++
				value = args[i]
			}
			a.cfg.Project = strings.Trim(value, "/")
		default:
			result = append(result, arg)
		}
	}
	return result, nil
}

func (a *app) help() error {
	fmt.Fprintf(a.out, `ghealth %s

Unofficial Google-Health-CLI for the Google Health API, written in Go.

Usage:
  ghealth [global flags] <command> [flags]

Core:
  auth login|status|revoke       Manage OAuth tokens
  doctor                         Check local setup
  types list|describe            Inspect Google Health data types
  endpoints list                 Inspect the v4 REST surface
  data list|reconcile|get|create|patch|delete|export-tcx
  rollup daily|physical          Query civil or physical rollups
  profile get|update             Read or update profile data
  settings get|update            Read or update settings
  identity get                   Read user identity
  irn-profile get                Read irregular-rhythm-notification profile
  paired-devices list|get        Read paired Fitbit devices
  subscribers list|create|patch|delete
  subscribers subscriptions ...  Manage manual webhook subscriptions
  api METHOD PATH                Raw API escape hatch
  agent manifest|capabilities|schema|context

Global flags:
  --json                         Shortcut for --format json
  --format table|json|ndjson|markdown|csv
  --pretty                       Pretty-print JSON
  --base-url URL                 Google Health origin (official or literal loopback)
  --user users/me                Override user resource
  --project PROJECT_NUMBER       Override Google Cloud project number

`, a.version)
	return nil
}

func (a *app) helpFor(args []string) error {
	if len(args) == 0 {
		return a.help()
	}
	path := strings.Join(args, " ")
	switch path {
	case "doctor":
		fmt.Fprintln(a.out, `Usage:
  ghealth doctor`)
	case "data list":
		fmt.Fprintln(a.out, `Usage:
  ghealth data list <type> [--from TIME] [--to TIME] [--filter FILTER] [--limit N] [--page-token TOKEN]

Time literals depend on the data type. Run "ghealth types describe <type>" for its timeFilter metadata.`)
	case "data reconcile":
		fmt.Fprintln(a.out, `Usage:
  ghealth data reconcile <type> [--from TIME] [--to TIME] [--filter FILTER] [--limit N] [--page-token TOKEN] [--family RESOURCE]`)
	case "data get":
		fmt.Fprintln(a.out, `Usage:
  ghealth data get <type> <id>`)
	case "data create":
		fmt.Fprintln(a.out, `Usage:
  ghealth data create <type> [--file payload.json|-]`)
	case "data patch":
		fmt.Fprintln(a.out, `Usage:
  ghealth data patch <type> <id> [--file payload.json|-]`)
	case "data delete":
		fmt.Fprintln(a.out, `Usage:
  ghealth data delete <type> [--file payload.json|-] --yes`)
	case "data export-tcx":
		fmt.Fprintln(a.out, `Usage:
  ghealth data export-tcx <exercise-id> [--partial-data]`)
	case "data":
		fmt.Fprintln(a.out, `Usage:
  ghealth data list <type> [flags]
  ghealth data reconcile <type> [flags]
  ghealth data get <type> <id>
  ghealth data create <type> [--file payload.json|-]
  ghealth data patch <type> <id> [--file payload.json|-]
  ghealth data delete <type> [--file payload.json|-] --yes
  ghealth data export-tcx <exercise-id> [--partial-data]`)
	case "auth login":
		fmt.Fprintln(a.out, `Usage:
  ghealth auth login [--write] [--scope read|write|all|management|URL,...] [--no-open]
                     [--client-id ID] [--redirect-uri URI]

Set GHEALTH_CLIENT_SECRET in the environment when a client secret is required.`)
	case "auth status":
		fmt.Fprintln(a.out, `Usage:
  ghealth auth status`)
	case "auth revoke":
		fmt.Fprintln(a.out, `Usage:
  ghealth auth revoke --yes

Deletes the local token. It does not revoke the credential at Google.`)
	case "auth":
		fmt.Fprintln(a.out, `Usage:
  ghealth auth login [--write] [--scope read|write|all|management|comma,separated] [--no-open]
  ghealth auth status
  ghealth auth revoke --yes`)
	case "config get", "config path":
		fmt.Fprintf(a.out, "Usage:\n  ghealth %s\n", path)
	case "config set":
		fmt.Fprintln(a.out, `Usage:
  ghealth config set base-url|user|project|client-id|redirect-uri VALUE`)
	case "config":
		fmt.Fprintln(a.out, `Usage:
  ghealth config get
  ghealth config path
  ghealth config set <key> <value>`)
	case "types list":
		fmt.Fprintln(a.out, `Usage:
  ghealth types list`)
	case "types describe":
		fmt.Fprintln(a.out, `Usage:
  ghealth types describe <type>`)
	case "types":
		fmt.Fprintln(a.out, `Usage:
  ghealth types list
  ghealth types describe <type>`)
	case "endpoints", "endpoints list":
		fmt.Fprintln(a.out, `Usage:
  ghealth endpoints list`)
	case "agent manifest", "agent capabilities":
		fmt.Fprintf(a.out, "Usage:\n  ghealth %s\n", path)
	case "agent schema":
		fmt.Fprintln(a.out, `Usage:
  ghealth agent schema [--type TYPE]`)
	case "agent context":
		fmt.Fprintln(a.out, `Usage:
  ghealth agent context today`)
	case "agent":
		fmt.Fprintln(a.out, `Usage:
  ghealth agent manifest
  ghealth agent capabilities
  ghealth agent schema [--type TYPE]
  ghealth agent context today`)
	case "profile get", "settings get":
		fmt.Fprintf(a.out, "Usage:\n  ghealth %s\n", path)
	case "profile update", "settings update":
		fmt.Fprintf(a.out, "Usage:\n  ghealth %s [--file payload.json|-] [--update-mask FIELD,...]\n", path)
	case "profile":
		fmt.Fprintln(a.out, `Usage:
  ghealth profile get
  ghealth profile update --file payload.json [--update-mask FIELD,...]`)
	case "settings":
		fmt.Fprintln(a.out, `Usage:
  ghealth settings get
  ghealth settings update --file payload.json [--update-mask FIELD,...]`)
	case "identity", "identity get":
		fmt.Fprintln(a.out, `Usage:
  ghealth identity get`)
	case "irn-profile", "irn-profile get":
		fmt.Fprintln(a.out, `Usage:
  ghealth irn-profile get`)
	case "paired-devices list":
		fmt.Fprintln(a.out, `Usage:
  ghealth paired-devices list [--limit N] [--page-token TOKEN]`)
	case "paired-devices get":
		fmt.Fprintln(a.out, `Usage:
  ghealth paired-devices get <device-id-or-resource-name>`)
	case "paired-devices":
		fmt.Fprintln(a.out, `Usage:
  ghealth paired-devices list [--limit N] [--page-token TOKEN]
  ghealth paired-devices get <device-id-or-resource-name>`)
	case "rollup daily":
		fmt.Fprintln(a.out, `Usage:
  ghealth rollup daily <type> [--file payload.json] [--from YYYY-MM-DD] [--to YYYY-MM-DD]
                              [--family RESOURCE] [--page-size N] [--page-token TOKEN] [--window-days N]`)
	case "rollup physical":
		fmt.Fprintln(a.out, `Usage:
  ghealth rollup physical <type> [--file payload.json] [--from RFC3339] [--to RFC3339]
                                 [--family RESOURCE] [--page-size N] [--page-token TOKEN] [--window-size DURATION]`)
	case "rollup":
		fmt.Fprintln(a.out, `Usage:
  ghealth rollup daily <type> [flags]
  ghealth rollup physical <type> [flags]`)
	case "subscribers list":
		fmt.Fprintln(a.out, `Usage:
  ghealth subscribers list --project PROJECT_NUMBER [--limit N] [--page-token TOKEN]`)
	case "subscribers create":
		fmt.Fprintln(a.out, `Usage:
  ghealth subscribers create --project PROJECT_NUMBER [--subscriber-id ID] [--file payload.json|-]`)
	case "subscribers patch":
		fmt.Fprintln(a.out, `Usage:
  ghealth subscribers patch --name RESOURCE [--update-mask FIELD,...] [--file payload.json|-]`)
	case "subscribers delete":
		fmt.Fprintln(a.out, `Usage:
  ghealth subscribers delete --name RESOURCE [--force] --yes`)
	case "subscribers subscriptions list":
		fmt.Fprintln(a.out, `Usage:
  ghealth subscribers subscriptions list --parent RESOURCE [--filter FILTER] [--limit N] [--page-token TOKEN]`)
	case "subscribers subscriptions create":
		fmt.Fprintln(a.out, `Usage:
  ghealth subscribers subscriptions create --parent RESOURCE [--subscription-id ID] [--file payload.json|-]`)
	case "subscribers subscriptions patch":
		fmt.Fprintln(a.out, `Usage:
  ghealth subscribers subscriptions patch --name RESOURCE [--update-mask dataTypes] [--file payload.json|-]`)
	case "subscribers subscriptions delete":
		fmt.Fprintln(a.out, `Usage:
  ghealth subscribers subscriptions delete --name RESOURCE --yes`)
	case "subscribers subscriptions":
		fmt.Fprintln(a.out, `Usage:
  ghealth subscribers subscriptions list --parent projects/PROJECT_NUMBER/subscribers/SUBSCRIBER [--filter FILTER] [--limit N] [--page-token TOKEN]
  ghealth subscribers subscriptions create --parent projects/PROJECT_NUMBER/subscribers/SUBSCRIBER [--subscription-id ID] --file payload.json
  ghealth subscribers subscriptions patch --name projects/PROJECT_NUMBER/subscribers/SUBSCRIBER/subscriptions/SUBSCRIPTION [--update-mask dataTypes] --file payload.json
  ghealth subscribers subscriptions delete --name projects/PROJECT_NUMBER/subscribers/SUBSCRIBER/subscriptions/SUBSCRIPTION --yes`)
	case "subscribers":
		fmt.Fprintln(a.out, `Usage:
  ghealth subscribers list --project PROJECT_NUMBER [--limit N] [--page-token TOKEN]
  ghealth subscribers create --project PROJECT_NUMBER [--subscriber-id ID] --file payload.json
  ghealth subscribers patch --name projects/PROJECT_NUMBER/subscribers/SUBSCRIBER [--update-mask FIELD,...] --file payload.json
  ghealth subscribers delete --name projects/PROJECT_NUMBER/subscribers/SUBSCRIBER [--force] --yes
  ghealth subscribers subscriptions ...`)
	case "api":
		fmt.Fprintln(a.out, `Usage:
  ghealth api METHOD PATH [--query key=value] [--file payload.json] [--raw] [--no-auth] [--yes]

POST, PUT, PATCH, and DELETE require --yes. Use --raw for non-JSON response bytes and --no-auth only for public endpoints.`)
	default:
		if args[0] == "api" {
			return a.helpFor([]string{"api"})
		}
		return a.help()
	}
	return nil
}

func (a *app) doctor(args []string) error {
	fs := newFlagSet("doctor")
	if err := fs.Parse(args); err != nil {
		return err
	}
	configPath, _ := config.ConfigPath()
	tokenPath, _ := config.TokenPath()
	status, err := auth.CurrentStatus()
	if err != nil {
		return fmt.Errorf("inspect authentication status: %w", err)
	}
	value := map[string]any{
		"binary":        "ghealth",
		"version":       a.version,
		"baseURL":       a.cfg.BaseURL,
		"user":          a.cfg.User,
		"project":       a.cfg.Project,
		"configPath":    configPath,
		"tokenPath":     tokenPath,
		"authenticated": status.Authenticated,
		"tokenValid":    status.Valid,
		"dataTypes":     len(registry.Types()),
		"restMethods":   len(registry.RESTOperations()),
	}
	return output.Print(a.out, value, a.opts)
}

func (a *app) auth(args []string) error {
	if len(args) == 0 {
		return usagef("auth requires login, status, or revoke")
	}
	switch args[0] {
	case "login":
		fs := newFlagSet("auth login")
		scopeValue := fs.String("scope", "read", "read, write, all, management, or comma-separated OAuth scopes")
		write := fs.Bool("write", false, "compatibility shortcut for --scope all")
		noOpen := fs.Bool("no-open", false, "print URL instead of opening a browser")
		clientID := fs.String("client-id", "", "OAuth client ID")
		redirectURL := fs.String("redirect-uri", "", "OAuth redirect URI")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		scopes := resolveScopes(*scopeValue, *write)
		token, err := auth.Login(context.Background(), a.cfg, auth.LoginOptions{
			ClientID:    *clientID,
			RedirectURL: *redirectURL,
			Scopes:      scopes,
			OpenBrowser: !*noOpen,
			OnAuthURL: func(rawURL string) {
				if *noOpen {
					fmt.Fprintf(a.errOut, "Open this URL to continue login:\n%s\n", rawURL)
				}
			},
		})
		if err != nil {
			return fmt.Errorf("%w: %w", errAuth, err)
		}
		return output.Print(a.out, map[string]any{"authenticated": true, "expiry": token.Expiry, "scopes": scopes}, a.opts)
	case "status":
		status, err := auth.CurrentStatus()
		if err != nil {
			return fmt.Errorf("inspect authentication status: %w", err)
		}
		return output.Print(a.out, status, a.opts)
	case "revoke":
		fs := newFlagSet("auth revoke")
		yes := fs.Bool("yes", false, "confirm local token deletion")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return usagef("auth revoke does not accept positional arguments")
		}
		if !*yes {
			return usagef("refusing to delete token without --yes")
		}
		deleted, err := auth.RevokeLocal()
		if err != nil {
			return err
		}
		return output.Print(a.out, map[string]any{
			"revocationScope":         "local",
			"localTokenDeleted":       deleted,
			"remoteCredentialRevoked": false,
		}, a.opts)
	default:
		return usagef("unknown auth command %q", args[0])
	}
}

func (a *app) config(args []string) error {
	if len(args) == 0 {
		return usagef("config requires get, set, or path")
	}
	switch args[0] {
	case "path":
		path, err := config.ConfigPath()
		if err != nil {
			return err
		}
		return output.Print(a.out, map[string]any{"path": path}, a.opts)
	case "get":
		return output.Print(a.out, a.cfg.Redacted(), a.opts)
	case "set":
		if len(args) < 3 {
			return usagef("config set requires key and value")
		}
		stored, err := config.LoadStored()
		if err != nil {
			return err
		}
		switch args[1] {
		case "base-url":
			stored.BaseURL = args[2]
		case "user":
			stored.User = args[2]
		case "project":
			stored.Project = args[2]
		case "client-id":
			stored.ClientID = args[2]
		case "redirect-uri":
			stored.RedirectURL = args[2]
		default:
			return usagef("unknown config key %q", args[1])
		}
		// Client secrets are environment-only. Scrub a legacy persisted value
		// whenever the user next updates their configuration.
		stored.ClientSecret = ""
		if err := config.Save(stored); err != nil {
			return err
		}
		return output.Print(a.out, map[string]any{"updated": args[1]}, a.opts)
	default:
		return usagef("unknown config command %q", args[0])
	}
}

func (a *app) types(args []string) error {
	if len(args) == 0 {
		return usagef("types requires list or describe")
	}
	switch args[0] {
	case "list":
		return output.Print(a.out, registry.Types(), a.opts)
	case "describe":
		if len(args) < 2 {
			return usagef("types describe requires a data type")
		}
		dataType, err := registry.MustLookup(args[1])
		if err != nil {
			return err
		}
		return output.Print(a.out, dataType, a.opts)
	default:
		return usagef("unknown types command %q", args[0])
	}
}

func (a *app) endpoints(args []string) error {
	if len(args) == 0 {
		return usagef("endpoints requires list")
	}
	if args[0] != "list" {
		return usagef("unknown endpoints command %q", args[0])
	}
	return output.Print(a.out, registry.RESTOperations(), a.opts)
}

func (a *app) agent(args []string) error {
	if len(args) == 0 {
		return usagef("agent requires manifest, capabilities, or schema")
	}
	switch args[0] {
	case "manifest":
		return output.Print(a.out, agentManifest(a.version), output.Options{Format: output.FormatJSON, Pretty: true})
	case "capabilities":
		return output.Print(a.out, map[string]any{
			"restMethods": len(registry.RESTOperations()),
			"dataTypes":   registry.Types(),
			"commands":    agentCommands(),
			"exitCodes": map[string]int{
				"success": ExitSuccess, "error": ExitError, "usage": ExitUsage, "auth": ExitAuth, "notFound": ExitNotFound, "conflict": ExitConflict,
			},
		}, output.Options{Format: output.FormatJSON, Pretty: true})
	case "schema":
		fs := newFlagSet("agent schema")
		typeName := fs.String("type", "", "data type to describe")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *typeName != "" {
			dataType, err := registry.MustLookup(*typeName)
			if err != nil {
				return err
			}
			return output.Print(a.out, dataType, output.Options{Format: output.FormatJSON, Pretty: true})
		}
		return output.Print(a.out, registry.Types(), output.Options{Format: output.FormatJSON, Pretty: true})
	case "context":
		if len(args) < 2 || args[1] != "today" {
			return usagef("agent context currently supports `today`")
		}
		now := time.Now()
		return output.Print(a.out, map[string]any{
			"date":        now.Format("2006-01-02"),
			"from":        now.Format("2006-01-02") + "T00:00:00Z",
			"to":          now.AddDate(0, 0, 1).Format("2006-01-02") + "T00:00:00Z",
			"suggested":   []string{"steps", "heart-rate", "sleep", "daily-heart-rate-variability", "daily-resting-heart-rate"},
			"jsonDefault": output.DefaultFormat() == output.FormatJSON,
		}, output.Options{Format: output.FormatJSON, Pretty: true})
	default:
		return usagef("unknown agent command %q", args[0])
	}
}

func (a *app) profile(args []string) error {
	return a.simpleResource("profile", args)
}

func (a *app) settings(args []string) error {
	return a.simpleResource("settings", args)
}

func (a *app) identity(args []string) error {
	if len(args) != 1 || args[0] != "get" {
		return usagef("identity supports only get")
	}
	client, err := a.client()
	if err != nil {
		return err
	}
	value, err := client.GetIdentity(context.Background())
	if err != nil {
		return err
	}
	return output.Print(a.out, value, a.opts)
}

func (a *app) irnProfile(args []string) error {
	if len(args) != 1 || args[0] != "get" {
		return usagef("irn-profile supports only get")
	}
	client, err := a.client()
	if err != nil {
		return err
	}
	value, err := client.GetIRNProfile(context.Background())
	if err != nil {
		return err
	}
	return output.Print(a.out, value, a.opts)
}

func (a *app) pairedDevices(args []string) error {
	if len(args) == 0 {
		return usagef("paired-devices requires list or get")
	}
	switch args[0] {
	case "list":
		fs := newFlagSet("paired-devices list")
		limit := fs.Int("limit", 5, "page size")
		pageToken := fs.String("page-token", "", "next page token")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *limit <= 0 {
			return usagef("--limit must be greater than zero")
		}
		if *limit > maxPairedDevicePageSize {
			return usagef("--limit must not exceed %d for paired devices", maxPairedDevicePageSize)
		}
		client, err := a.client()
		if err != nil {
			return err
		}
		value, err := client.ListPairedDevicesWithOptions(context.Background(), healthapi.PairedDeviceListOptions{
			PageSize:  *limit,
			PageToken: *pageToken,
		})
		if err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	case "get":
		if len(args) != 2 {
			return usagef("paired-devices get requires a device ID or resource name")
		}
		client, err := a.client()
		if err != nil {
			return err
		}
		value, err := client.GetPairedDevice(context.Background(), args[1])
		if err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	default:
		return usagef("unknown paired-devices command %q", args[0])
	}
}

func (a *app) simpleResource(resource string, args []string) error {
	if len(args) == 0 {
		return usagef("%s requires get or update", resource)
	}
	switch args[0] {
	case "get":
		client, err := a.client()
		if err != nil {
			return err
		}
		if resource == "profile" {
			value, err := client.GetProfile(context.Background())
			if err != nil {
				return err
			}
			return output.Print(a.out, value, a.opts)
		}
		value, err := client.GetSettings(context.Background())
		if err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	case "update":
		fs := newFlagSet(resource + " update")
		file := fs.String("file", "-", "JSON payload file, or - for stdin")
		updateMask := fs.String("update-mask", "", "comma-separated fields to update")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if resource == "profile" {
			var body healthapi.UpdateProfileRequest
			if err := readJSONInto(*file, &body); err != nil {
				return err
			}
			client, err := a.client()
			if err != nil {
				return err
			}
			value, err := client.UpdateProfileWithOptions(context.Background(), body, healthapi.UpdateOptions{UpdateMask: *updateMask})
			if err != nil {
				return err
			}
			return output.Print(a.out, value, a.opts)
		}
		var body healthapi.UpdateSettingsRequest
		if err := readJSONInto(*file, &body); err != nil {
			return err
		}
		client, err := a.client()
		if err != nil {
			return err
		}
		value, err := client.UpdateSettingsWithOptions(context.Background(), body, healthapi.UpdateOptions{UpdateMask: *updateMask})
		if err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	default:
		return usagef("%s supports get or update", resource)
	}
}

func (a *app) data(args []string) error {
	if len(args) == 0 {
		return usagef("data requires list, reconcile, get, create, patch, delete, or export-tcx")
	}
	switch args[0] {
	case "list", "reconcile":
		if len(args) < 2 {
			return usagef("data %s requires a data type", args[0])
		}
		dataType, err := registry.MustLookup(args[1])
		if err != nil {
			return err
		}
		if !registry.HasOperation(dataType, args[0]) {
			return usagef("%s does not support %s", dataType.EndpointName, args[0])
		}
		fs := newFlagSet("data " + args[0])
		from := fs.String("from", "", "start time or date")
		to := fs.String("to", "", "end time or date")
		filter := fs.String("filter", "", "raw Google Health filter")
		limit := fs.Int("limit", 0, "page size (provider default when omitted)")
		pageToken := fs.String("page-token", "", "next page token")
		family := fs.String("family", "", "data source family")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if *limit < 0 {
			return usagef("--limit must not be negative")
		}
		pageMaximum := maxDataPageSizeFor(dataType)
		if *limit > pageMaximum {
			return usagef("--limit must not exceed %d for %s", pageMaximum, dataType.EndpointName)
		}
		finalFilter := *filter
		if finalFilter == "" {
			finalFilter, err = registry.BuildRangeFilter(dataType, *from, *to)
			if err != nil {
				return usagef("%v", err)
			}
		}
		client, err := a.client()
		if err != nil {
			return err
		}
		if args[0] == "reconcile" {
			value, err := client.ReconcileDataPoints(context.Background(), dataType.EndpointName, healthapi.ReconcileOptions{Filter: finalFilter, PageSize: *limit, PageToken: *pageToken, DataSourceFamily: *family})
			if err != nil {
				return err
			}
			return output.Print(a.out, value, a.opts)
		}
		value, err := client.ListDataPoints(context.Background(), dataType.EndpointName, healthapi.ListOptions{Filter: finalFilter, PageSize: *limit, PageToken: *pageToken})
		if err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	case "get":
		if len(args) < 3 {
			return usagef("data get requires type and id")
		}
		dataType, err := registry.MustLookup(args[1])
		if err != nil {
			return err
		}
		if !registry.HasOperation(dataType, "get") {
			return usagef("%s does not support get", dataType.EndpointName)
		}
		client, err := a.client()
		if err != nil {
			return err
		}
		value, err := client.GetDataPoint(context.Background(), dataType.EndpointName, args[2])
		if err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	case "create", "patch", "delete":
		return a.writeData(args)
	case "export-tcx":
		if len(args) < 2 {
			return usagef("data export-tcx requires an exercise data point ID")
		}
		fs := newFlagSet("data export-tcx")
		partialData := fs.Bool("partial-data", false, "include TCX data points when GPS data is unavailable")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		client, err := a.client()
		if err != nil {
			return err
		}
		bytes, err := client.ExportExerciseTCXWithOptions(context.Background(), args[1], healthapi.ExportExerciseTCXOptions{PartialData: *partialData})
		if err != nil {
			return err
		}
		_, err = a.out.Write(bytes)
		return err
	default:
		return usagef("unknown data command %q", args[0])
	}
}

func (a *app) writeData(args []string) error {
	command := args[0]
	if len(args) < 2 {
		return usagef("data %s requires a data type", command)
	}
	dataType, err := registry.MustLookup(args[1])
	if err != nil {
		return err
	}
	fs := newFlagSet("data " + command)
	file := fs.String("file", "-", "JSON payload file, or - for stdin")
	yes := fs.Bool("yes", false, "confirm destructive command")
	id := ""
	flagArgs := args[2:]
	if command == "patch" {
		if len(flagArgs) == 0 {
			return usagef("data patch requires an id")
		}
		id = flagArgs[0]
		flagArgs = flagArgs[1:]
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if command == "delete" && !*yes {
		return usagef("refusing to batch delete without --yes")
	}
	operation := map[string]string{
		"create": "create",
		"patch":  "update",
		"delete": "batchDelete",
	}[command]
	if !registry.HasOperation(dataType, operation) {
		return usagef("%s does not support %s", dataType.EndpointName, command)
	}
	client, err := a.client()
	if err != nil {
		return err
	}
	switch command {
	case "create":
		body, err := readJSONRaw(*file)
		if err != nil {
			return err
		}
		value, err := client.CreateDataPoint(context.Background(), dataType.EndpointName, body)
		if err != nil {
			return err
		}
		if err := value.Err(); err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	case "patch":
		body, err := readJSONRaw(*file)
		if err != nil {
			return err
		}
		value, err := client.PatchDataPoint(context.Background(), dataType.EndpointName, id, body)
		if err != nil {
			return err
		}
		if err := value.Err(); err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	case "delete":
		var body healthapi.BatchDeleteDataPointsRequest
		if err := readJSONInto(*file, &body); err != nil {
			return err
		}
		value, err := client.BatchDeleteDataPoints(context.Background(), dataType.EndpointName, body)
		if err != nil {
			return err
		}
		if err := value.Err(); err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	}
	return usagef("unknown data mutation %q", command)
}

func (a *app) rollup(args []string) error {
	if len(args) < 2 {
		return usagef("rollup requires daily|physical and data type")
	}
	mode := args[0]
	if mode != "daily" && mode != "physical" {
		return usagef("rollup requires daily or physical")
	}
	dataType, err := registry.MustLookup(args[1])
	if err != nil {
		return err
	}
	operation := "rollup"
	if mode == "daily" {
		operation = "dailyRollUp"
	}
	if !registry.HasOperation(dataType, operation) {
		return usagef("%s does not support %s rollup", dataType.EndpointName, mode)
	}
	fs := newFlagSet("rollup " + mode)
	file := fs.String("file", "", "raw JSON request body")
	from := fs.String("from", "", "start date or time")
	to := fs.String("to", "", "end date or time")
	family := fs.String("family", "", "data source family")
	pageSize := fs.Int("page-size", 0, "page size")
	pageToken := fs.String("page-token", "", "next page token")
	windowSize := fs.String("window-size", "", "physical rollup window size, such as 3600s")
	windowDays := fs.Int("window-days", 0, "daily rollup window size in days")
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}
	windowDaysProvided := flagWasProvided(args[2:], "window-days")
	body := map[string]any{}
	if *file != "" {
		body, err = readJSON(*file)
		if err != nil {
			return err
		}
		_, windowDaysProvided = body["windowSizeDays"]
	} else {
		if *pageSize < 0 || *pageSize > maxRollupPageSize {
			return usagef("--page-size must be between 0 and %d", maxRollupPageSize)
		}
		switch mode {
		case "daily":
			if *from == "" || *to == "" {
				return usagef("daily rollup requires both --from and --to")
			}
			if windowDaysProvided && *windowDays < 1 {
				return usagef("--window-days must be at least 1 when supplied")
			}
			rangeBody, err := civilRange(*from, *to)
			if err != nil {
				return usagef("%v", err)
			}
			body["range"] = rangeBody
			if windowDaysProvided {
				body["windowSizeDays"] = *windowDays
			}
		case "physical":
			if *from == "" || *to == "" {
				return usagef("physical rollup requires both --from and --to")
			}
			if *windowSize == "" {
				return usagef("physical rollup requires --window-size")
			}
			body["range"] = map[string]any{"startTime": *from, "endTime": *to}
			body["windowSize"] = *windowSize
		}
		if *family != "" {
			body["dataSourceFamily"] = *family
		}
		if *pageSize > 0 {
			body["pageSize"] = *pageSize
		}
		if *pageToken != "" {
			body["pageToken"] = *pageToken
		}
	}
	switch mode {
	case "daily":
		var request healthapi.DailyRollUpDataPointsRequest
		if err := decodeJSONValue(body, &request); err != nil {
			return err
		}
		if err := validateDailyRollUpRequest(dataType, request, windowDaysProvided); err != nil {
			return usagef("%v", err)
		}
		client, err := a.client()
		if err != nil {
			return err
		}
		value, err := client.DailyRollUp(context.Background(), dataType.EndpointName, request)
		if err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	case "physical":
		var request healthapi.RollUpDataPointsRequest
		if err := decodeJSONValue(body, &request); err != nil {
			return err
		}
		if err := validatePhysicalRollUpRequest(dataType, request); err != nil {
			return usagef("%v", err)
		}
		client, err := a.client()
		if err != nil {
			return err
		}
		value, err := client.RollUp(context.Background(), dataType.EndpointName, request)
		if err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	}
	return nil
}

func (a *app) subscribers(args []string) error {
	if len(args) == 0 {
		return usagef("subscribers requires list, create, patch, delete, or subscriptions")
	}
	project := a.cfg.Project
	switch args[0] {
	case "list":
		fs := newFlagSet("subscribers list")
		projectFlag := fs.String("project", project, "Google Cloud project number")
		limit := fs.Int("limit", 50, "page size")
		pageToken := fs.String("page-token", "", "next page token")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *projectFlag == "" {
			return usagef("subscribers list requires --project with a Google Cloud project number")
		}
		if *limit <= 0 {
			return usagef("--limit must be greater than zero")
		}
		if *limit > maxSubscriberPageSize {
			return usagef("--limit must not exceed %d for subscribers", maxSubscriberPageSize)
		}
		client, err := a.managementClient()
		if err != nil {
			return err
		}
		value, err := client.ListSubscribersWithOptions(context.Background(), *projectFlag, healthapi.SubscriberListOptions{PageSize: *limit, PageToken: *pageToken})
		if err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	case "create":
		fs := newFlagSet("subscribers create")
		projectFlag := fs.String("project", project, "Google Cloud project number")
		file := fs.String("file", "-", "JSON payload file, or - for stdin")
		subscriberID := fs.String("subscriber-id", "", "optional subscriber ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *projectFlag == "" {
			return usagef("subscribers create requires --project with a Google Cloud project number")
		}
		var body healthapi.CreateSubscriberRequest
		if err := readJSONInto(*file, &body); err != nil {
			return err
		}
		client, err := a.managementClient()
		if err != nil {
			return err
		}
		value, err := client.CreateSubscriberWithOptions(context.Background(), *projectFlag, body, healthapi.CreateSubscriberOptions{SubscriberID: *subscriberID})
		if err != nil {
			return err
		}
		if err := value.Err(); err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	case "patch":
		fs := newFlagSet("subscribers patch")
		name := fs.String("name", "", "subscriber resource name")
		file := fs.String("file", "-", "JSON payload file, or - for stdin")
		updateMask := fs.String("update-mask", "", "comma-separated fields to update")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *name == "" {
			return usagef("subscribers patch requires --name")
		}
		var body healthapi.UpdateSubscriberRequest
		if err := readJSONInto(*file, &body); err != nil {
			return err
		}
		client, err := a.managementClient()
		if err != nil {
			return err
		}
		value, err := client.PatchSubscriberWithOptions(context.Background(), *name, body, healthapi.PatchSubscriberOptions{UpdateMask: *updateMask})
		if err != nil {
			return err
		}
		if err := value.Err(); err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	case "delete":
		fs := newFlagSet("subscribers delete")
		name := fs.String("name", "", "subscriber resource name")
		yes := fs.Bool("yes", false, "confirm deletion")
		force := fs.Bool("force", false, "delete child resources if present")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *name == "" {
			return usagef("subscribers delete requires --name")
		}
		if !*yes {
			return usagef("refusing to delete subscriber without --yes")
		}
		client, err := a.managementClient()
		if err != nil {
			return err
		}
		value, err := client.DeleteSubscriberWithOptions(context.Background(), *name, healthapi.DeleteSubscriberOptions{Force: *force})
		if err != nil {
			return err
		}
		if err := value.Err(); err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	case "subscriptions":
		return a.subscriptions(args[1:])
	default:
		return usagef("unknown subscribers command %q", args[0])
	}
}

func (a *app) subscriptions(args []string) error {
	if len(args) == 0 {
		return usagef("subscribers subscriptions requires list, create, patch, or delete")
	}
	switch args[0] {
	case "list":
		fs := newFlagSet("subscribers subscriptions list")
		parent := fs.String("parent", "", "parent subscriber resource name")
		filter := fs.String("filter", "", "raw subscription filter using user or data_type")
		limit := fs.Int("limit", 50, "page size")
		pageToken := fs.String("page-token", "", "next page token")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *parent == "" {
			return usagef("subscribers subscriptions list requires --parent")
		}
		if *limit <= 0 {
			return usagef("--limit must be greater than zero")
		}
		if *limit > maxSubscriptionPageSize {
			return usagef("--limit must not exceed %d for subscriptions", maxSubscriptionPageSize)
		}
		client, err := a.managementClient()
		if err != nil {
			return err
		}
		value, err := client.ListSubscriptionsWithOptions(context.Background(), *parent, healthapi.SubscriptionListOptions{
			Filter:    *filter,
			PageSize:  *limit,
			PageToken: *pageToken,
		})
		if err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	case "create":
		fs := newFlagSet("subscribers subscriptions create")
		parent := fs.String("parent", "", "parent subscriber resource name")
		file := fs.String("file", "-", "JSON payload file, or - for stdin")
		subscriptionID := fs.String("subscription-id", "", "optional subscription ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *parent == "" {
			return usagef("subscribers subscriptions create requires --parent")
		}
		var body healthapi.CreateSubscriptionRequest
		if err := readJSONInto(*file, &body); err != nil {
			return err
		}
		client, err := a.managementClient()
		if err != nil {
			return err
		}
		value, err := client.CreateSubscriptionWithOptions(context.Background(), *parent, body, healthapi.CreateSubscriptionOptions{SubscriptionID: *subscriptionID})
		if err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	case "patch":
		fs := newFlagSet("subscribers subscriptions patch")
		name := fs.String("name", "", "subscription resource name")
		file := fs.String("file", "-", "JSON payload file, or - for stdin")
		updateMask := fs.String("update-mask", "", "comma-separated fields to update")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *name == "" {
			return usagef("subscribers subscriptions patch requires --name")
		}
		var body healthapi.UpdateSubscriptionRequest
		if err := readJSONInto(*file, &body); err != nil {
			return err
		}
		client, err := a.managementClient()
		if err != nil {
			return err
		}
		value, err := client.PatchSubscriptionWithOptions(context.Background(), *name, body, healthapi.PatchSubscriptionOptions{UpdateMask: *updateMask})
		if err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	case "delete":
		fs := newFlagSet("subscribers subscriptions delete")
		name := fs.String("name", "", "subscription resource name")
		yes := fs.Bool("yes", false, "confirm deletion")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *name == "" {
			return usagef("subscribers subscriptions delete requires --name")
		}
		if !*yes {
			return usagef("refusing to delete subscription without --yes")
		}
		client, err := a.managementClient()
		if err != nil {
			return err
		}
		value, err := client.DeleteSubscription(context.Background(), *name)
		if err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	default:
		return usagef("unknown subscribers subscriptions command %q", args[0])
	}
}

func (a *app) api(args []string) error {
	if len(args) < 2 {
		return usagef("api requires METHOD and PATH")
	}
	method := strings.ToUpper(args[0])
	requestPath := args[1]
	fs := newFlagSet("api")
	file := fs.String("file", "", "JSON payload file")
	rawResponse := fs.Bool("raw", false, "write response bytes without JSON decoding")
	noAuth := fs.Bool("no-auth", false, "make an unauthenticated request")
	yes := fs.Bool("yes", false, "confirm a potentially mutating raw request")
	queryValues := multiFlag{}
	fs.Var(&queryValues, "query", "query parameter as key=value")
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}
	if method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions && !*yes {
		return usagef("refusing to send %s through the raw API without --yes", method)
	}
	var body json.RawMessage
	var err error
	if *file != "" {
		body, err = readJSONRaw(*file)
		if err != nil {
			return err
		}
	}
	query := url.Values{}
	for _, item := range queryValues {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			return usagef("--query must be key=value")
		}
		query.Add(key, value)
	}
	var client *healthapi.Client
	if *noAuth {
		client = healthapi.New(a.cfg.BaseURL, a.cfg.User, nil)
	} else {
		client, err = a.client()
		if err != nil {
			return err
		}
	}
	if *rawResponse {
		value, err := client.RawBytes(context.Background(), method, requestPath, query, body)
		if err != nil {
			return err
		}
		_, err = a.out.Write(value)
		return err
	}
	value, err := client.Raw(context.Background(), method, requestPath, query, body)
	if err != nil {
		return err
	}
	return output.Print(a.out, value, a.opts)
}

func (a *app) client() (*healthapi.Client, error) {
	source, err := auth.TokenSource(context.Background(), a.cfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errAuth, err)
	}
	httpClient := oauth2.NewClient(context.Background(), source)
	return healthapi.New(a.cfg.BaseURL, a.cfg.User, httpClient), nil
}

func (a *app) managementClient() (*healthapi.Client, error) {
	ctx := context.Background()
	httpClient, err := google.DefaultClient(ctx, registry.ManagementScopes()...)
	if err != nil {
		return nil, fmt.Errorf("%w: load application default credentials for subscriber management: %w", errAuth, err)
	}
	return healthapi.New(a.cfg.BaseURL, a.cfg.User, httpClient), nil
}

func resolveScopes(value string, write bool) []string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "read", "readonly":
		if write {
			return registry.AllScopes()
		}
		return registry.ReadOnlyScopes()
	case "write", "writeonly":
		return registry.WriteScopes()
	case "all":
		return registry.AllScopes()
	case "management":
		return registry.ManagementScopes()
	default:
		parts := strings.Split(value, ",")
		scopes := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				scopes = append(scopes, trimmed)
			}
		}
		return scopes
	}
}

func maxDataPageSizeFor(dataType registry.DataType) int {
	if dataType.EndpointName == "exercise" || dataType.EndpointName == "sleep" {
		return maxSessionDataPageSize
	}
	return maxDataPageSize
}

func flagWasProvided(args []string, name string) bool {
	wanted := "--" + name
	for _, arg := range args {
		key, _, _ := strings.Cut(arg, "=")
		if key == wanted {
			return true
		}
	}
	return false
}

func readJSON(file string) (map[string]any, error) {
	value := map[string]any{}
	if err := readJSONInto(file, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func readJSONInto(file string, destination any) error {
	contents, err := readInput(file)
	if err != nil {
		return err
	}
	return decodeJSONBytes(contents, destination)
}

func readJSONRaw(file string) (json.RawMessage, error) {
	contents, err := readInput(file)
	if err != nil {
		return nil, err
	}
	if !json.Valid(contents) {
		return nil, fmt.Errorf("decode JSON payload: invalid JSON")
	}
	return json.RawMessage(contents), nil
}

func readInput(file string) ([]byte, error) {
	var reader io.Reader = os.Stdin
	if file != "" && file != "-" {
		opened, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		defer opened.Close()
		reader = opened
	}

	contents, err := io.ReadAll(io.LimitReader(reader, int64(maxInputBytes)+1))
	if err != nil {
		return nil, err
	}
	if len(contents) > maxInputBytes {
		return nil, fmt.Errorf("input exceeds the 16 MiB limit")
	}
	return contents, nil
}

func decodeJSONValue(value any, destination any) error {
	contents, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode JSON payload: %w", err)
	}
	return decodeJSONBytes(contents, destination)
}

func decodeJSONBytes(contents []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode JSON payload: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode JSON payload: multiple JSON values")
		}
		return fmt.Errorf("decode JSON payload: %w", err)
	}
	return nil
}

func civilRange(from, to string) (map[string]any, error) {
	result := map[string]any{}
	if from != "" {
		start, err := civilDateTime(from)
		if err != nil {
			return nil, fmt.Errorf("parse --from: %w", err)
		}
		result["start"] = start
	}
	if to != "" {
		end, err := civilDateTime(to)
		if err != nil {
			return nil, fmt.Errorf("parse --to: %w", err)
		}
		result["end"] = end
	}
	return result, nil
}

func validateDailyRollUpRequest(dataType registry.DataType, request healthapi.DailyRollUpDataPointsRequest, windowDaysProvided bool) error {
	if request.Range == nil || request.Range.Start == nil || request.Range.End == nil {
		return fmt.Errorf("daily rollup range requires both start and end")
	}
	start, err := typedCivilDateTime("range.start", request.Range.Start)
	if err != nil {
		return err
	}
	end, err := typedCivilDateTime("range.end", request.Range.End)
	if err != nil {
		return err
	}
	if !start.Before(end) {
		return fmt.Errorf("daily rollup range must end after it starts")
	}
	if err := validateRollUpRangeMaximum(dataType, end.Sub(start)); err != nil {
		return err
	}
	if request.PageSize < 0 || request.PageSize > maxRollupPageSize {
		return fmt.Errorf("daily rollup pageSize must be between 0 and %d", maxRollupPageSize)
	}

	windowDays := int32(1)
	if windowDaysProvided {
		if request.WindowSizeDays < 1 {
			return fmt.Errorf("daily rollup windowSizeDays must be at least 1 when supplied")
		}
		windowDays = request.WindowSizeDays
	}
	rangeDays := int64(end.Sub(start) / (24 * time.Hour))
	if int64(windowDays) > rangeDays {
		return fmt.Errorf("daily rollup windowSizeDays must not exceed the request range")
	}
	return nil
}

func validatePhysicalRollUpRequest(dataType registry.DataType, request healthapi.RollUpDataPointsRequest) error {
	if request.Range == nil || request.Range.StartTime == "" || request.Range.EndTime == "" {
		return fmt.Errorf("physical rollup range requires both startTime and endTime")
	}
	start, err := time.Parse(time.RFC3339, request.Range.StartTime)
	if err != nil {
		return fmt.Errorf("physical rollup range.startTime must be RFC3339: %w", err)
	}
	end, err := time.Parse(time.RFC3339, request.Range.EndTime)
	if err != nil {
		return fmt.Errorf("physical rollup range.endTime must be RFC3339: %w", err)
	}
	if !start.Before(end) {
		return fmt.Errorf("physical rollup range must end after it starts")
	}
	if err := validateRollUpRangeMaximum(dataType, end.Sub(start)); err != nil {
		return err
	}
	if request.PageSize < 0 || request.PageSize > maxRollupPageSize {
		return fmt.Errorf("physical rollup pageSize must be between 0 and %d", maxRollupPageSize)
	}
	if request.WindowSize == "" {
		return fmt.Errorf("physical rollup windowSize is required")
	}
	window, err := parseGoogleDuration(request.WindowSize)
	if err != nil {
		return fmt.Errorf("physical rollup windowSize is invalid: %w", err)
	}
	if window < time.Second {
		return fmt.Errorf("physical rollup windowSize must be at least 1 second")
	}
	if window > end.Sub(start) {
		return fmt.Errorf("physical rollup windowSize must not exceed the request range")
	}
	return nil
}

func parseGoogleDuration(value string) (time.Duration, error) {
	if !strings.HasSuffix(value, "s") {
		return 0, fmt.Errorf("expected a Google duration in seconds, such as 3600s")
	}
	number := strings.TrimSuffix(value, "s")
	if number == "" {
		return 0, fmt.Errorf("missing seconds")
	}
	digitsBeforeDecimal := 0
	fractionDigits := 0
	seenDecimal := false
	for _, character := range number {
		switch {
		case character >= '0' && character <= '9':
			if seenDecimal {
				fractionDigits++
			} else {
				digitsBeforeDecimal++
			}
		case character == '.' && !seenDecimal:
			seenDecimal = true
		default:
			return 0, fmt.Errorf("expected seconds with an optional fractional part")
		}
	}
	if digitsBeforeDecimal == 0 || (seenDecimal && (fractionDigits == 0 || fractionDigits > 9)) {
		return 0, fmt.Errorf("expected seconds with up to 9 fractional digits")
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func validateRollUpRangeMaximum(dataType registry.DataType, span time.Duration) error {
	if dataType.MaxQueryDays <= 0 {
		return nil
	}
	maximum := time.Duration(dataType.MaxQueryDays) * 24 * time.Hour
	if span > maximum {
		return fmt.Errorf("rollup range for %s must not exceed %d days", dataType.EndpointName, dataType.MaxQueryDays)
	}
	return nil
}

func typedCivilDateTime(field string, value *healthapi.CivilDateTime) (time.Time, error) {
	if value == nil || value.Date == nil {
		return time.Time{}, fmt.Errorf("daily rollup %s.date is required", field)
	}
	year := int(value.Date.Year)
	month := int(value.Date.Month)
	day := int(value.Date.Day)
	if year < 1 || year > 9999 || month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}, fmt.Errorf("daily rollup %s has an invalid date", field)
	}
	hour, minute, second, nanos := 0, 0, 0, 0
	if value.Time != nil {
		hour = int(value.Time.Hours)
		minute = int(value.Time.Minutes)
		second = int(value.Time.Seconds)
		nanos = int(value.Time.Nanos)
		if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 59 || nanos < 0 || nanos > 999_999_999 {
			return time.Time{}, fmt.Errorf("daily rollup %s has an invalid time", field)
		}
	}
	parsed := time.Date(year, time.Month(month), day, hour, minute, second, nanos, time.UTC)
	if parsed.Year() != year || int(parsed.Month()) != month || parsed.Day() != day {
		return time.Time{}, fmt.Errorf("daily rollup %s has an invalid calendar date", field)
	}
	return parsed, nil
}

func civilDateTime(value string) (map[string]any, error) {
	datePart, timePart, _ := strings.Cut(value, "T")
	parts := strings.Split(datePart, "-")
	if len(parts) != 3 {
		return nil, fmt.Errorf("expected YYYY-MM-DD or YYYY-MM-DDTHH:MM:SS")
	}
	year, err := parsePositiveInt(parts[0])
	if err != nil {
		return nil, err
	}
	month, err := parsePositiveInt(parts[1])
	if err != nil {
		return nil, err
	}
	if month < 1 || month > 12 {
		return nil, fmt.Errorf("invalid month %d: must be between 1 and 12", month)
	}
	day, err := parsePositiveInt(parts[2])
	if err != nil {
		return nil, err
	}
	if day < 1 || day > 31 {
		return nil, fmt.Errorf("invalid day %d: must be between 1 and 31", day)
	}
	parsedDate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if parsedDate.Year() != year || int(parsedDate.Month()) != month || parsedDate.Day() != day {
		return nil, fmt.Errorf("invalid calendar date %04d-%02d-%02d", year, month, day)
	}
	result := map[string]any{"date": map[string]any{"year": year, "month": month, "day": day}}
	if timePart != "" {
		timePart = strings.TrimSuffix(timePart, "Z")
		timeParts := strings.Split(timePart, ":")
		if len(timeParts) < 2 || len(timeParts) > 3 {
			return nil, fmt.Errorf("expected time as HH:MM or HH:MM:SS")
		}
		hour, err := parsePositiveInt(timeParts[0])
		if err != nil {
			return nil, err
		}
		if hour > 23 {
			return nil, fmt.Errorf("invalid hour %d: must be between 0 and 23", hour)
		}
		minute, err := parsePositiveInt(timeParts[1])
		if err != nil {
			return nil, err
		}
		if minute > 59 {
			return nil, fmt.Errorf("invalid minute %d: must be between 0 and 59", minute)
		}
		second := 0
		if len(timeParts) == 3 {
			second, err = parsePositiveInt(timeParts[2])
			if err != nil {
				return nil, err
			}
			if second > 59 {
				return nil, fmt.Errorf("invalid second %d: must be between 0 and 59", second)
			}
		}
		result["time"] = map[string]any{"hours": hour, "minutes": minute, "seconds": second}
	}
	return result, nil
}

func parsePositiveInt(value string) (int, error) {
	if value == "" {
		return 0, fmt.Errorf("missing number")
	}
	n := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid number %q", value)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

func agentManifest(version string) map[string]any {
	return map[string]any{
		"name":        "ghealth",
		"version":     version,
		"description": "Unofficial Google-Health-CLI for the Google Health API, written in Go",
		"api": map[string]any{
			"baseURL":     config.DefaultBaseURL,
			"version":     "v4",
			"dataTypes":   len(registry.Types()),
			"restMethods": len(registry.RESTOperations()),
		},
		"defaults": map[string]any{
			"interactiveOutput":    "table",
			"nonInteractiveOutput": "json",
			"destructiveFlag":      "--yes",
			"rawEscapeHatch":       "ghealth api METHOD PATH",
			"healthAuth":           "end-user OAuth",
			"managementAuth":       "Application Default Credentials",
		},
		"commands": agentCommands(),
		"docs": []string{
			"https://developers.google.com/health/data-types",
			"https://developers.google.com/health/reference/rest",
			"https://developers.google.com/health/scopes",
		},
	}
}

func agentCommands() []map[string]string {
	return []map[string]string{
		{"command": "ghealth types list --json", "purpose": "list all Google Health data types"},
		{"command": "ghealth endpoints list --json", "purpose": "list all v4 REST methods"},
		{"command": "ghealth data list steps --from RFC3339 --to RFC3339 --json", "purpose": "query data points"},
		{"command": "ghealth rollup daily steps --from YYYY-MM-DD --to YYYY-MM-DD --json", "purpose": "query daily rollups"},
		{"command": "ghealth irn-profile get --json", "purpose": "read the user's IRN profile"},
		{"command": "ghealth paired-devices list --json", "purpose": "list paired devices"},
		{"command": "ghealth subscribers list --project PROJECT_NUMBER --json", "purpose": "list webhook subscribers using ADC"},
		{"command": "ghealth subscribers subscriptions list --parent projects/PROJECT_NUMBER/subscribers/SUBSCRIBER --json", "purpose": "list manual webhook subscriptions using ADC"},
		{"command": "ghealth api GET /v4/users/me/profile --json", "purpose": "raw API call"},
	}
}

func usagef(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errUsage, fmt.Sprintf(format, args...))
}

func unwrapUsage(err error) error {
	if errors.Is(err, errUsage) {
		message := strings.TrimSpace(strings.TrimPrefix(err.Error(), errUsage.Error()+":"))
		if message != "" {
			return errors.New(message)
		}
	}
	return err
}

func hintFor(err error) string {
	if errors.Is(err, errAuth) {
		return "For health data, run `ghealth auth login` after setting GHEALTH_CLIENT_ID. For subscriber management, configure Application Default Credentials with `gcloud auth application-default login`."
	}
	if errors.Is(err, errUsage) {
		return "Run `ghealth help` for the command map."
	}
	return ""
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}
