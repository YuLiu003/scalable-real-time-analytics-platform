package alpaca

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const testCheckpointScope = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestFileCheckpointsRoundTripIsPrivateAndAtomic(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "private")
	path := filepath.Join(directory, "checkpoint.json")
	store := NewFileCheckpoints(path, testCheckpointScope)
	timestamp := time.Date(2026, 8, 3, 12, 0, 0, 123, time.FixedZone("private", -7*60*60))
	want := map[string]Cursor{"LOAD-A": {Timestamp: timestamp}, "LOAD-B": {Timestamp: timestamp.Add(time.Minute)}}
	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("checkpoint mode = %#o, want 0600", got)
	}
	directoryInfo, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	if got := directoryInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("directory mode = %#o, want 0700", got)
	}
	if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary checkpoint remains: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	wantUTC := map[string]Cursor{
		"LOAD-A": {Timestamp: timestamp.UTC()},
		"LOAD-B": {Timestamp: timestamp.Add(time.Minute).UTC()},
	}
	if !reflect.DeepEqual(got, wantUTC) {
		t.Fatalf("Load() = %#v, want %#v", got, wantUTC)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "FAKEPACA_KEY") || strings.Contains(string(data), "FAKEPACA_SECRET") ||
		!strings.HasSuffix(string(data), "\n") {
		t.Fatalf("checkpoint content violates privacy/format: %q", data)
	}
}

func TestFileCheckpointsLoadMissingAndValidationErrors(t *testing.T) {
	missing := NewFileCheckpoints(filepath.Join(t.TempDir(), "missing.json"), testCheckpointScope)
	got, err := missing.Load()
	if err != nil || len(got) != 0 {
		t.Fatalf("missing Load() = %#v, %v", got, err)
	}

	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "malformed", data: `{`, want: "invalid JSON"},
		{name: "unknown", data: `{"schema_version":2,"scope_sha256":"` + testCheckpointScope + `","cursors":{},"private":"secret"}`, want: "invalid JSON"},
		{name: "multiple", data: `{"schema_version":2,"scope_sha256":"` + testCheckpointScope + `","cursors":{}} {}`, want: "multiple JSON values"},
		{name: "schema", data: `{"schema_version":1,"scope_sha256":"` + testCheckpointScope + `","cursors":{}}`, want: "unsupported schema"},
		{name: "scope", data: `{"schema_version":2,"scope_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","cursors":{}}`, want: "unsupported schema"},
		{name: "nil cursors", data: `{"schema_version":2,"scope_sha256":"` + testCheckpointScope + `","cursors":null}`, want: "unsupported schema"},
		{name: "instrument", data: `{"schema_version":2,"scope_sha256":"` + testCheckpointScope + `","cursors":{"private":{"timestamp":"2026-08-03T12:00:00Z"}}}`, want: "invalid instrument"},
		{name: "timestamp", data: `{"schema_version":2,"scope_sha256":"` + testCheckpointScope + `","cursors":{"LOAD-A":{"timestamp":"private"}}}`, want: "invalid timestamp"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := NewFileCheckpoints("/not-used/private.json", testCheckpointScope)
			store.readFile = func(string) ([]byte, error) { return []byte(test.data), nil }
			_, err := store.Load()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want substring %q", err, test.want)
			}
			if strings.Contains(err.Error(), test.data) {
				t.Fatalf("Load() leaked input: %v", err)
			}
		})
	}

	store := NewFileCheckpoints("/private/checkpoint.json", testCheckpointScope)
	store.readFile = func(string) ([]byte, error) { return nil, errors.New("FAKEPACA_SECRET") }
	_, err = store.Load()
	if err == nil || !strings.Contains(err.Error(), "read private market checkpoint") {
		t.Fatalf("Load() read error = %v", err)
	}
}

func TestFileCheckpointsMigratesLegacyStateToScopedReplay(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "checkpoint.json")
	legacy := []byte(`{"schema_version":1,"cursors":{"LOAD-A":{"timestamp":"2026-08-03T12:00:00Z"}}}`)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewFileCheckpoints(path, testCheckpointScope)
	cursors, err := store.Load()
	if err != nil || len(cursors) != 0 {
		t.Fatalf("legacy Load() = %#v, %v", cursors, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document checkpointDocument
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if document.SchemaVersion != 2 || document.ScopeSHA256 != testCheckpointScope || len(document.Cursors) != 0 {
		t.Fatalf("migrated checkpoint = %#v", document)
	}

	sentinel := errors.New("migration write failed")
	failing := NewFileCheckpoints("/private/checkpoint.json", testCheckpointScope)
	failing.readFile = func(string) ([]byte, error) { return legacy, nil }
	failing.mkdirAll = func(string, os.FileMode) error { return sentinel }
	if _, err := failing.Load(); !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "migrate private market checkpoint") {
		t.Fatalf("legacy migration error = %v", err)
	}
}

func TestFileCheckpointsSaveRejectsInvalidCursor(t *testing.T) {
	store := NewFileCheckpoints(filepath.Join(t.TempDir(), "checkpoint.json"), testCheckpointScope)
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	for _, cursors := range []map[string]Cursor{
		{"private": {Timestamp: now}},
		{"LOAD-A": {}},
	} {
		if err := store.Save(cursors); err == nil {
			t.Fatalf("Save(%#v) succeeded", cursors)
		}
	}
}

func TestFileCheckpointsSaveFailureStagesPreservePublishedFile(t *testing.T) {
	sentinel := errors.New("FAKEPACA_SECRET should remain wrapped")
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	cursors := map[string]Cursor{"LOAD-A": {Timestamp: now}}
	tests := []struct {
		name       string
		configure  func(*FileCheckpoints, *bool)
		want       string
		wantRemove bool
	}{
		{name: "mkdir", want: "create private market checkpoint directory", configure: func(store *FileCheckpoints, _ *bool) {
			store.mkdirAll = func(string, os.FileMode) error { return sentinel }
		}},
		{name: "write", want: "write private market checkpoint", configure: func(store *FileCheckpoints, _ *bool) {
			store.writeFile = func(string, []byte, os.FileMode) error { return sentinel }
		}},
		{name: "protect temporary", want: "protect private market checkpoint", wantRemove: true, configure: func(store *FileCheckpoints, _ *bool) {
			store.chmod = func(string, os.FileMode) error { return sentinel }
		}},
		{name: "rename", want: "publish private market checkpoint", wantRemove: true, configure: func(store *FileCheckpoints, _ *bool) {
			store.rename = func(string, string) error { return sentinel }
		}},
		{name: "protect published", want: "protect published private market checkpoint", configure: func(store *FileCheckpoints, _ *bool) {
			calls := 0
			store.chmod = func(string, os.FileMode) error {
				calls++
				if calls == 2 {
					return sentinel
				}
				return nil
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			removed := false
			store := &FileCheckpoints{
				Path:      "/private/checkpoint.json",
				Scope:     testCheckpointScope,
				mkdirAll:  func(string, os.FileMode) error { return nil },
				writeFile: func(string, []byte, os.FileMode) error { return nil },
				rename:    func(string, string) error { return nil },
				chmod:     func(string, os.FileMode) error { return nil },
				remove:    func(string) error { removed = true; return sentinel },
			}
			test.configure(store, &removed)
			err := store.Save(cursors)
			if err == nil || !strings.Contains(err.Error(), test.want) || !errors.Is(err, sentinel) {
				t.Fatalf("Save() error = %v, want wrapped %q", err, test.want)
			}
			if removed != test.wantRemove {
				t.Fatalf("remove called = %v, want %v", removed, test.wantRemove)
			}
		})
	}
}

func TestFileCheckpointsFailedPublishLeavesPriorCheckpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkpoint.json")
	prior := []byte("prior private checkpoint")
	if err := os.WriteFile(path, prior, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewFileCheckpoints(path, testCheckpointScope)
	store.rename = func(string, string) error { return errors.New("publish failure") }
	err := store.Save(map[string]Cursor{"LOAD-A": {Timestamp: time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)}})
	if err == nil {
		t.Fatal("Save() succeeded")
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || string(got) != string(prior) {
		t.Fatalf("published checkpoint changed: %q, %v", got, readErr)
	}
	if _, statErr := os.Stat(path + ".tmp"); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("temporary checkpoint remains: %v", statErr)
	}
}

func TestCheckpointScopeAndScopeValidation(t *testing.T) {
	config := validTestConfig()
	scope := CheckpointScope(config)
	if !validCheckpointScope(scope) || scope != CheckpointScope(config) {
		t.Fatalf("CheckpointScope() = %q", scope)
	}
	config.TenantID = "another-private-tenant"
	if scope == CheckpointScope(config) {
		t.Fatal("checkpoint scope did not change with feed identity")
	}
	for _, invalid := range []string{"", strings.Repeat("A", 64), strings.Repeat("a", 63), strings.Repeat("g", 64)} {
		if validCheckpointScope(invalid) {
			t.Fatalf("validCheckpointScope(%q) = true", invalid)
		}
	}

	store := NewFileCheckpoints(filepath.Join(t.TempDir(), "checkpoint.json"), "invalid")
	if _, err := store.Load(); err == nil || !strings.Contains(err.Error(), "scope is invalid") {
		t.Fatalf("Load() invalid scope error = %v", err)
	}
	if err := store.Save(map[string]Cursor{}); err == nil {
		t.Fatal("Save() accepted an invalid scope")
	}
}
