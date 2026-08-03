package alpaca

import (
	"bytes"
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
	Cursors       map[string]checkpointCursor `json:"cursors"`
}

type checkpointCursor struct {
	Timestamp string `json:"timestamp"`
}

type FileCheckpoints struct {
	Path      string
	readFile  func(string) ([]byte, error)
	mkdirAll  func(string, os.FileMode) error
	writeFile func(string, []byte, os.FileMode) error
	rename    func(string, string) error
	chmod     func(string, os.FileMode) error
	remove    func(string) error
}

func NewFileCheckpoints(path string) *FileCheckpoints {
	return &FileCheckpoints{
		Path:      path,
		readFile:  os.ReadFile,
		mkdirAll:  os.MkdirAll,
		writeFile: os.WriteFile,
		rename:    os.Rename,
		chmod:     os.Chmod,
		remove:    os.Remove,
	}
}

func (store *FileCheckpoints) Load() (map[string]Cursor, error) {
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
	if document.SchemaVersion != 1 || document.Cursors == nil {
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
	document := checkpointDocument{SchemaVersion: 1, Cursors: make(map[string]checkpointCursor, len(cursors))}
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
