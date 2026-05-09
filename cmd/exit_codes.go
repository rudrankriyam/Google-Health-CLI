package cmd

import (
	"errors"
	"net/http"

	"github.com/rudrankriyam/google-health-cli/internal/healthapi"
)

const (
	ExitSuccess  = 0
	ExitError    = 1
	ExitUsage    = 2
	ExitCanceled = 130
	ExitAuth     = 3
	ExitNotFound = 4
	ExitConflict = 5
)

func exitCodeFromError(err error) int {
	if err == nil {
		return ExitSuccess
	}
	if errors.Is(err, errUsage) {
		return ExitUsage
	}
	if errors.Is(err, errAuth) {
		return ExitAuth
	}

	var apiErr *healthapi.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return ExitAuth
		case http.StatusNotFound:
			return ExitNotFound
		case http.StatusConflict:
			return ExitConflict
		default:
			if apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
				return 10 + min(apiErr.StatusCode-400, 49)
			}
			if apiErr.StatusCode >= 500 && apiErr.StatusCode < 600 {
				return 60 + min(apiErr.StatusCode-500, 39)
			}
		}
	}

	return ExitError
}
