package alpaca

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/services/market-pipeline/internal/event"
)

type Cursor struct {
	Timestamp time.Time
}

type Checkpoints interface {
	Load() (map[string]Cursor, error)
	Save(map[string]Cursor) error
}

type checkpointDocument struct {
	SchemaVersion int                         `json:"schema_version"`
	ScopeSHA256   string                      `json:"scope_sha256"`
	Cursors       map[string]checkpointCursor `json:"cursors"`
}

type checkpointCursor struct {
	Timestamp string `json:"timestamp"`
}

type FileCheckpoints struct {
	Path      string
	Scope     string
	readFile  func(string) ([]byte, error)
	mkdirAll  func(string, os.FileMode) error
	writeFile func(string, []byte, os.FileMode) error
	rename    func(string, string) error
	chmod     func(string, os.FileMode) error
	remove    func(string) error
}

func NewFileCheckpoints(path, scope string) *FileCheckpoints {
	return &FileCheckpoints{
		Path:      path,
		Scope:     scope,
		readFile:  os.ReadFile,
		mkdirAll:  os.MkdirAll,
		writeFile: os.WriteFile,
		rename:    os.Rename,
		chmod:     os.Chmod,
		remove:    os.Remove,
	}
}

func (store *FileCheckpoints) Load() (map[string]Cursor, error) {
	if !validCheckpointScope(store.Scope) {
		return nil, errors.New("private market checkpoint scope is invalid")
	}
	data, err := store.readFile(store.Path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Cursor{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read private market checkpoint: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var document checkpointDocument
	if err := decoder.Decode(&document); err != nil {
		return nil, errors.New("private market checkpoint is invalid JSON")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("private market checkpoint contains multiple JSON values")
	}
	if document.SchemaVersion == 1 && document.ScopeSHA256 == "" && document.Cursors != nil {
		cursors := map[string]Cursor{}
		if err := store.Save(cursors); err != nil {
			return nil, fmt.Errorf("migrate private market checkpoint: %w", err)
		}
		return cursors, nil
	}
	if document.SchemaVersion != 2 || document.ScopeSHA256 != store.Scope || document.Cursors == nil {
		return nil, errors.New("private market checkpoint has an unsupported schema")
	}
	cursors := make(map[string]Cursor, len(document.Cursors))
	for instrument, value := range document.Cursors {
		if !event.ValidInstrument(instrument) {
			return nil, errors.New("private market checkpoint contains an invalid instrument")
		}
		timestamp, err := time.Parse(time.RFC3339Nano, value.Timestamp)
		if err != nil {
			return nil, errors.New("private market checkpoint contains an invalid timestamp")
		}
		cursors[instrument] = Cursor{Timestamp: timestamp.UTC()}
	}
	return cursors, nil
}

func (store *FileCheckpoints) Save(cursors map[string]Cursor) error {
	if !validCheckpointScope(store.Scope) {
		return errors.New("refuse to persist an invalid private market checkpoint")
	}
	document := checkpointDocument{
		SchemaVersion: 2,
		ScopeSHA256:   store.Scope,
		Cursors:       make(map[string]checkpointCursor, len(cursors)),
	}
	for instrument, cursor := range cursors {
		if !event.ValidInstrument(instrument) || cursor.Timestamp.IsZero() {
			return errors.New("refuse to persist an invalid private market checkpoint")
		}
		document.Cursors[instrument] = checkpointCursor{Timestamp: cursor.Timestamp.UTC().Format(time.RFC3339Nano)}
	}
	// checkpointDocument contains only JSON-supported scalar and map types.
	data, _ := json.Marshal(document)
	data = append(data, '\n')
	directory := filepath.Dir(store.Path)
	if err := store.mkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create private market checkpoint directory: %w", err)
	}
	temporary := store.Path + ".tmp"
	if err := store.writeFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("write private market checkpoint: %w", err)
	}
	if err := store.chmod(temporary, 0o600); err != nil {
		_ = store.remove(temporary)
		return fmt.Errorf("protect private market checkpoint: %w", err)
	}
	if err := store.rename(temporary, store.Path); err != nil {
		_ = store.remove(temporary)
		return fmt.Errorf("publish private market checkpoint: %w", err)
	}
	if err := store.chmod(store.Path, 0o600); err != nil {
		return fmt.Errorf("protect published private market checkpoint: %w", err)
	}
	return nil
}

// CheckpointScope binds cursors to one private tenant/provider feed without
// writing those identifiers to disk.
func CheckpointScope(config Config) string {
	value := config.TenantID + "\x00" + config.Source + "\x00" + config.Feed
	return fmt.Sprintf("%x", sha256.Sum256([]byte(value)))
}

func validCheckpointScope(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
