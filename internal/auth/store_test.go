package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// fakeSecurity emulates /usr/bin/security generic-password subcommands.
type fakeSecurity struct {
	items  map[string]string
	calls  [][]string
	inputs [][]byte
}

func (f *fakeSecurity) run(input []byte, args ...string) ([]byte, error) {
	f.calls = append(f.calls, args)
	f.inputs = append(f.inputs, append([]byte(nil), input...))
	flags := map[string]string{}
	for i := 1; i < len(args)-1; i++ {
		if args[i] == "-s" || args[i] == "-a" || args[i] == "-w" {
			flags[args[i]] = args[i+1]
		}
	}
	key := flags["-s"] + "/" + flags["-a"]
	switch args[0] {
	case "add-generic-password":
		f.items[key] = string(input)
		return nil, nil
	case "find-generic-password":
		value, ok := f.items[key]
		if !ok {
			return nil, fmt.Errorf("keychain item not found: %w", os.ErrNotExist)
		}
		return []byte(value + "\n"), nil
	case "delete-generic-password":
		if _, ok := f.items[key]; !ok {
			return nil, fmt.Errorf("keychain item not found: %w", os.ErrNotExist)
		}
		delete(f.items, key)
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected security command %q", args[0])
	}
}

func TestKeychainStoreRoundTrip(t *testing.T) {
	fake := &fakeSecurity{items: map[string]string{}}
	store := keychainStore{run: fake.run}

	if _, err := store.Read(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Read before Write: err = %v, want os.ErrNotExist", err)
	}

	payload := []byte("{\n  \"access_token\": \"abc\"\n}\n")
	if err := store.Write(payload); err != nil {
		t.Fatalf("Write: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(payload)
	for _, call := range fake.calls {
		for _, argument := range call {
			if argument == encoded {
				t.Fatal("keychain password was exposed in process arguments")
			}
		}
	}
	got, err := store.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("Read = %q, want %q", got, payload)
	}

	deleted, err := store.Delete()
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !deleted {
		t.Fatal("Delete reported no deletion")
	}
	if _, err := store.Read(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Read after Delete: err = %v, want os.ErrNotExist", err)
	}
	// Deleting an absent item is not an error.
	deleted, err = store.Delete()
	if err != nil {
		t.Fatalf("Delete of absent item: %v", err)
	}
	if deleted {
		t.Fatal("Delete of absent item reported deletion")
	}
}

func TestFileStoreRoundTrip(t *testing.T) {
	t.Setenv("GHEALTH_TOKEN_FILE", filepath.Join(t.TempDir(), "token.json"))
	store := fileStore{}

	if _, err := store.Read(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Read before Write: err = %v, want os.ErrNotExist", err)
	}
	if err := store.Write([]byte("{}")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := store.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != "{}" {
		t.Fatalf("Read = %q, want {}", got)
	}
	deleted, err := store.Delete()
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !deleted {
		t.Fatal("Delete reported no deletion")
	}
	if _, err := store.Read(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Read after Delete: err = %v, want os.ErrNotExist", err)
	}
	deleted, err = store.Delete()
	if err != nil {
		t.Fatalf("Delete of absent item: %v", err)
	}
	if deleted {
		t.Fatal("Delete of absent item reported deletion")
	}
}

func TestFileStoreAtomicallyRepairsPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")
	t.Setenv("GHEALTH_TOKEN_FILE", path)
	if err := os.WriteFile(path, []byte("old-token"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}

	store := fileStore{}
	if err := store.Write([]byte("new-token")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-token" {
		t.Fatalf("token contents = %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("token mode = %v, want 0600", got)
	}
	files, err := filepath.Glob(filepath.Join(dir, ".token.json.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("temporary files remain after Write: %v", files)
	}
}

func TestActiveStoreSelection(t *testing.T) {
	t.Setenv("GHEALTH_TOKEN_STORAGE", "")
	store, err := activeStore()
	if err != nil {
		t.Fatalf("default store: %v", err)
	}
	if _, ok := store.(fileStore); !ok {
		t.Fatalf("default store = %T, want fileStore", store)
	}

	t.Setenv("GHEALTH_TOKEN_STORAGE", "vault")
	if _, err := activeStore(); err == nil {
		t.Fatal("unknown backend: expected error")
	}

	t.Setenv("GHEALTH_TOKEN_STORAGE", "keychain")
	store, err = activeStore()
	if runtime.GOOS == "darwin" {
		if err != nil {
			t.Fatalf("keychain store on darwin: %v", err)
		}
		if _, ok := store.(keychainStore); !ok {
			t.Fatalf("keychain store = %T, want keychainStore", store)
		}
	} else if err == nil {
		t.Fatal("keychain store off darwin: expected error")
	}
}
