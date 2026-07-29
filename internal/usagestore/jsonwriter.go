package usagestore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// JSONWriter appends usage records as JSON Lines (one JSON object per line)
// to a local file. It is safe for concurrent use and survives process restarts
// as long as the file is stored on a persistent volume.
type JSONWriter struct {
	path string
	file *os.File
	enc  *json.Encoder
	mu   sync.Mutex
}

// NewJSONWriter opens (or creates) the JSON Lines file at path.
func NewJSONWriter(path string) (*JSONWriter, error) {
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("jsonwriter: create directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("jsonwriter: open file: %w", err)
	}
	return &JSONWriter{
		path: path,
		file: file,
		enc:  json.NewEncoder(file),
	}, nil
}

// Write serializes the record as a single JSON line and appends it to the file.
func (w *JSONWriter) Write(_ context.Context, r Record) error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return fmt.Errorf("jsonwriter: closed")
	}
	return w.enc.Encode(r)
}

// Close flushes and closes the underlying file.
func (w *JSONWriter) Close() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}
