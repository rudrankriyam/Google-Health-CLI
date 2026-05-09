package cmd

import (
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
	cfg, err := config.Load()
	if err == nil {
		a.cfg = cfg
	}
	if err == nil {
		err = a.run()
	}
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
	cfg, err := config.Load()
	if err == nil {
		a.cfg = cfg
	}
	if err == nil {
		err = a.run()
	}
	if err != nil {
		output.PrintError(a.errOut, unwrapUsage(err), a.opts, hintFor(err))
	}
	return exitCodeFromError(err)
}

func (a *app) run() error {
	args, err := a.parseGlobal(a.args)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return a.help()
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
  types list|describe            Inspect all 31 Google Health data types
  endpoints list                 Inspect the v4 REST surface
  data list|get|create|patch     Work with data points
  rollup daily|physical          Query civil or physical rollups
  profile get|update             Read or update profile data
  settings get|update            Read or update settings
  identity get                   Read user identity
  subscribers list|create|patch|delete
  api METHOD PATH                Raw API escape hatch
  agent manifest|capabilities    Stable JSON for agent tooling

Global flags:
  --json                         Shortcut for --format json
  --format table|json|ndjson|markdown|csv
  --pretty                       Pretty-print JSON
  --base-url URL                 Override API base URL
  --user users/me                Override user resource
  --project PROJECT              Override Google Cloud project

`, a.version)
	return nil
}

func (a *app) helpFor(args []string) error {
	if len(args) == 0 {
		return a.help()
	}
	switch args[0] {
	case "data":
		fmt.Fprintln(a.out, `Usage:
  ghealth data list <type> --from RFC3339 --to RFC3339 [--limit N]
  ghealth data reconcile <type> --from RFC3339 --to RFC3339 [--limit N]
  ghealth data get <type> <id>
  ghealth data create <type> --file payload.json
  ghealth data patch <type> <id> --file payload.json
  ghealth data delete <type> --file payload.json --yes
  ghealth data export-tcx <exercise-id>`)
	case "auth":
		fmt.Fprintln(a.out, `Usage:
  ghealth auth login [--write] [--scope read|all|comma,separated] [--no-open]
  ghealth auth status
  ghealth auth revoke --yes`)
	default:
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
	status := auth.CurrentStatus()
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
		scopeValue := fs.String("scope", "read", "read, all, write, or comma-separated OAuth scopes")
		write := fs.Bool("write", false, "request write scopes")
		noOpen := fs.Bool("no-open", false, "print URL instead of opening a browser")
		clientID := fs.String("client-id", "", "OAuth client ID")
		clientSecret := fs.String("client-secret", "", "OAuth client secret")
		redirectURL := fs.String("redirect-uri", "", "OAuth redirect URI")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		scopes := resolveScopes(*scopeValue, *write)
		token, err := auth.Login(context.Background(), a.cfg, auth.LoginOptions{
			ClientID:     *clientID,
			ClientSecret: *clientSecret,
			RedirectURL:  *redirectURL,
			Scopes:       scopes,
			OpenBrowser:  !*noOpen,
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
		return output.Print(a.out, auth.CurrentStatus(), a.opts)
	case "revoke":
		fs := newFlagSet("auth revoke")
		yes := fs.Bool("yes", false, "confirm local token deletion")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if !*yes {
			return usagef("refusing to delete token without --yes")
		}
		if err := auth.RevokeLocal(); err != nil {
			return err
		}
		return output.Print(a.out, map[string]any{"revoked": true}, a.opts)
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
		return output.Print(a.out, a.cfg, a.opts)
	case "set":
		if len(args) < 3 {
			return usagef("config set requires key and value")
		}
		switch args[1] {
		case "base-url":
			a.cfg.BaseURL = args[2]
		case "user":
			a.cfg.User = args[2]
		case "project":
			a.cfg.Project = args[2]
		case "client-id":
			a.cfg.ClientID = args[2]
		case "client-secret":
			a.cfg.ClientSecret = args[2]
		case "redirect-uri":
			a.cfg.RedirectURL = args[2]
		default:
			return usagef("unknown config key %q", args[1])
		}
		if err := config.Save(a.cfg); err != nil {
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

func (a *app) simpleResource(resource string, args []string) error {
	if len(args) == 0 {
		return usagef("%s requires get or update", resource)
	}
	client, err := a.client()
	if err != nil {
		return err
	}
	switch args[0] {
	case "get":
		var value map[string]any
		if resource == "profile" {
			value, err = client.GetProfile(context.Background())
		} else {
			value, err = client.GetSettings(context.Background())
		}
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
		body, err := readJSON(*file)
		if err != nil {
			return err
		}
		var value map[string]any
		if resource == "profile" {
			value, err = client.UpdateProfileWithOptions(context.Background(), body, healthapi.UpdateOptions{UpdateMask: *updateMask})
		} else {
			value, err = client.UpdateSettingsWithOptions(context.Background(), body, healthapi.UpdateOptions{UpdateMask: *updateMask})
		}
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
	client, err := a.client()
	if err != nil {
		return err
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
		fs := newFlagSet("data " + args[0])
		from := fs.String("from", "", "start time or date")
		to := fs.String("to", "", "end time or date")
		filter := fs.String("filter", "", "raw Google Health filter")
		limit := fs.Int("limit", 100, "page size")
		pageToken := fs.String("page-token", "", "next page token")
		view := fs.String("view", "", "API view")
		family := fs.String("family", "", "data source family")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		finalFilter := *filter
		if finalFilter == "" {
			finalFilter = registry.FilterFromRange(dataType, *from, *to)
		}
		if args[0] == "reconcile" {
			value, err := client.ReconcileDataPoints(context.Background(), dataType.EndpointName, healthapi.ReconcileOptions{Filter: finalFilter, PageSize: *limit, PageToken: *pageToken, DataSourceFamily: *family})
			if err != nil {
				return err
			}
			return output.Print(a.out, value, a.opts)
		}
		value, err := client.ListDataPoints(context.Background(), dataType.EndpointName, healthapi.ListOptions{Filter: finalFilter, PageSize: *limit, PageToken: *pageToken, View: *view})
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
		value, err := client.GetDataPoint(context.Background(), dataType.EndpointName, args[2])
		if err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	case "create", "patch", "delete":
		return a.writeData(client, args)
	case "export-tcx":
		if len(args) < 2 {
			return usagef("data export-tcx requires an exercise data point ID")
		}
		fs := newFlagSet("data export-tcx")
		partialData := fs.Bool("partial-data", false, "include TCX data points when GPS data is unavailable")
		if err := fs.Parse(args[2:]); err != nil {
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

func (a *app) writeData(client *healthapi.Client, args []string) error {
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
	body, err := readJSON(*file)
	if err != nil {
		return err
	}
	var value map[string]any
	switch command {
	case "create":
		value, err = client.CreateDataPoint(context.Background(), dataType.EndpointName, body)
	case "patch":
		value, err = client.PatchDataPoint(context.Background(), dataType.EndpointName, id, body)
	case "delete":
		value, err = client.BatchDeleteDataPoints(context.Background(), dataType.EndpointName, body)
	}
	if err != nil {
		return err
	}
	return output.Print(a.out, value, a.opts)
}

func (a *app) rollup(args []string) error {
	if len(args) < 2 {
		return usagef("rollup requires daily|physical and data type")
	}
	client, err := a.client()
	if err != nil {
		return err
	}
	mode := args[0]
	dataType, err := registry.MustLookup(args[1])
	if err != nil {
		return err
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
	body := map[string]any{}
	if *file != "" {
		body, err = readJSON(*file)
		if err != nil {
			return err
		}
	} else {
		switch mode {
		case "daily":
			if *from != "" || *to != "" {
				rangeBody, err := civilRange(*from, *to)
				if err != nil {
					return err
				}
				body["range"] = rangeBody
			}
			if *windowDays > 0 {
				body["windowSizeDays"] = *windowDays
			}
		case "physical":
			if *from != "" || *to != "" {
				body["range"] = map[string]any{"startTime": *from, "endTime": *to}
			}
			if *windowSize != "" {
				body["windowSize"] = *windowSize
			}
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
	var value map[string]any
	switch mode {
	case "daily":
		value, err = client.DailyRollUp(context.Background(), dataType.EndpointName, body)
	case "physical":
		value, err = client.RollUp(context.Background(), dataType.EndpointName, body)
	default:
		return usagef("rollup requires daily or physical")
	}
	if err != nil {
		return err
	}
	return output.Print(a.out, value, a.opts)
}

func (a *app) subscribers(args []string) error {
	if len(args) == 0 {
		return usagef("subscribers requires list, create, patch, or delete")
	}
	client, err := a.client()
	if err != nil {
		return err
	}
	project := a.cfg.Project
	switch args[0] {
	case "list":
		fs := newFlagSet("subscribers list")
		projectFlag := fs.String("project", project, "Google Cloud project")
		limit := fs.Int("limit", 50, "page size")
		pageToken := fs.String("page-token", "", "next page token")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		value, err := client.ListSubscribersWithOptions(context.Background(), *projectFlag, healthapi.SubscriberListOptions{PageSize: *limit, PageToken: *pageToken})
		if err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	case "create":
		fs := newFlagSet("subscribers create")
		projectFlag := fs.String("project", project, "Google Cloud project")
		file := fs.String("file", "-", "JSON payload file, or - for stdin")
		subscriberID := fs.String("subscriber-id", "", "optional subscriber ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		body, err := readJSON(*file)
		if err != nil {
			return err
		}
		value, err := client.CreateSubscriberWithOptions(context.Background(), *projectFlag, body, healthapi.CreateSubscriberOptions{SubscriberID: *subscriberID})
		if err != nil {
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
		body, err := readJSON(*file)
		if err != nil {
			return err
		}
		value, err := client.PatchSubscriberWithOptions(context.Background(), *name, body, healthapi.PatchSubscriberOptions{UpdateMask: *updateMask})
		if err != nil {
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
		value, err := client.DeleteSubscriberWithOptions(context.Background(), *name, healthapi.DeleteSubscriberOptions{Force: *force})
		if err != nil {
			return err
		}
		return output.Print(a.out, value, a.opts)
	default:
		return usagef("unknown subscribers command %q", args[0])
	}
}

func (a *app) api(args []string) error {
	if len(args) < 2 {
		return usagef("api requires METHOD and PATH")
	}
	client, err := a.client()
	if err != nil {
		return err
	}
	method := strings.ToUpper(args[0])
	requestPath := args[1]
	fs := newFlagSet("api")
	file := fs.String("file", "", "JSON payload file")
	queryValues := multiFlag{}
	fs.Var(&queryValues, "query", "query parameter as key=value")
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}
	var body any
	if *file != "" {
		body, err = readJSON(*file)
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
	if httpClient == nil {
		httpClient = http.DefaultClient
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
	case "write", "all":
		return registry.AllScopes()
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

func readJSON(file string) (map[string]any, error) {
	var bytes []byte
	var err error
	if file == "" || file == "-" {
		bytes, err = io.ReadAll(os.Stdin)
	} else {
		bytes, err = os.ReadFile(file)
	}
	if err != nil {
		return nil, err
	}
	var value map[string]any
	if err := json.Unmarshal(bytes, &value); err != nil {
		return nil, fmt.Errorf("decode JSON payload: %w", err)
	}
	return value, nil
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
	day, err := parsePositiveInt(parts[2])
	if err != nil {
		return nil, err
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
		minute, err := parsePositiveInt(timeParts[1])
		if err != nil {
			return nil, err
		}
		second := 0
		if len(timeParts) == 3 {
			second, err = parsePositiveInt(timeParts[2])
			if err != nil {
				return nil, err
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
		{"command": "ghealth api GET /v4/users/me/profile --json", "purpose": "raw API call"},
	}
}

func usagef(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errUsage, fmt.Sprintf(format, args...))
}

func unwrapUsage(err error) error {
	if errors.Is(err, errUsage) {
		if unwrapped := errors.Unwrap(err); unwrapped != nil {
			return unwrapped
		}
	}
	return err
}

func hintFor(err error) string {
	if errors.Is(err, errAuth) {
		return "Run `ghealth auth login` after setting GHEALTH_CLIENT_ID, or use `ghealth config set client-id <value>`."
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
