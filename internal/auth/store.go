package auth

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/rudrankriyam/Google-Health-CLI/internal/config"
)

// TokenStore persists the raw OAuth token JSON. The default backend is a
// plaintext file; set GHEALTH_TOKEN_STORAGE=keychain to opt in to the macOS
// Keychain backend instead.
type TokenStore interface {
	// Read returns the stored token bytes, or an error wrapping
	// os.ErrNotExist when no token is stored.
	Read() ([]byte, error)
	Write(data []byte) error
	// Delete returns true only when a stored token existed and was removed.
	Delete() (bool, error)
	// Location is a human-readable description of where the token lives.
	Location() string
}

func activeStore() (TokenStore, error) {
	switch value := strings.ToLower(strings.TrimSpace(os.Getenv("GHEALTH_TOKEN_STORAGE"))); value {
	case "", "file":
		return fileStore{}, nil
	case "keychain":
		if runtime.GOOS != "darwin" {
			return nil, errors.New("GHEALTH_TOKEN_STORAGE=keychain is only supported on macOS")
		}
		return keychainStore{run: securityRun}, nil
	default:
		return nil, fmt.Errorf("unknown GHEALTH_TOKEN_STORAGE %q (expected \"file\" or \"keychain\")", value)
	}
}

// fileStore stores the token as a plaintext JSON file with mode 0600.
type fileStore struct{}

func (fileStore) Read() ([]byte, error) {
	path, err := config.TokenPath()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func (fileStore) Write(data []byte) error {
	path, err := config.TokenPath()
	if err != nil {
		return err
	}
	return config.WritePrivateFile(path, data)
}

func (fileStore) Delete() (bool, error) {
	path, err := config.TokenPath()
	if err != nil {
		return false, err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (fileStore) Location() string {
	path, _ := config.TokenPath()
	return path
}

const (
	keychainService = "ghealth"
	keychainAccount = "oauth-token"
)

// keychainStore stores the token as a base64-encoded generic password in the
// macOS Keychain via /usr/bin/security. run is swappable for tests.
type keychainStore struct {
	run func(input []byte, args ...string) ([]byte, error)
}

func (s keychainStore) Read() ([]byte, error) {
	out, err := s.run(nil, "find-generic-password", "-s", keychainService, "-a", keychainAccount, "-w")
	if err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
	if err != nil {
		return nil, fmt.Errorf("decode keychain token: %w", err)
	}
	return data, nil
}

func (s keychainStore) Write(data []byte) error {
	encoded := base64.StdEncoding.EncodeToString(data)
	// Passing the password after -w exposes it through the process argument
	// list. A trailing -w makes security prompt on stdin instead.
	_, err := s.run([]byte(encoded+"\n"), "add-generic-password", "-U", "-s", keychainService, "-a", keychainAccount, "-w")
	return err
}

func (s keychainStore) Delete() (bool, error) {
	_, err := s.run(nil, "delete-generic-password", "-s", keychainService, "-a", keychainAccount)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (keychainStore) Location() string {
	return fmt.Sprintf("macOS Keychain (service %q, account %q)", keychainService, keychainAccount)
}

// securityRun executes /usr/bin/security, mapping its "item not found" exit
// code (44) to os.ErrNotExist.
func securityRun(input []byte, args ...string) ([]byte, error) {
	cmd := exec.Command("/usr/bin/security", args...)
	var stdout, stderr bytes.Buffer
	if len(input) > 0 {
		cmd.Stdin = bytes.NewReader(input)
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 44 {
			return nil, fmt.Errorf("keychain item not found: %w", os.ErrNotExist)
		}
		return nil, fmt.Errorf("security %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
