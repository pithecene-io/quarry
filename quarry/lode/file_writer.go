package lode

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/justapithecus/lode/lode"
)

// FileWriter writes sidecar files to Lode Store.
// Files land at Hive-partitioned paths under files/, bypassing Dataset
// segment/manifest machinery entirely.
type FileWriter interface {
	// PutFile writes a file to the Hive-partitioned files/ prefix.
	// The filename must not contain path separators or "..".
	PutFile(ctx context.Context, filename, contentType string, data []byte) error
}

// Verify LodeClient implements FileWriter.
var _ FileWriter = (*LodeClient)(nil)

// PutFile writes a sidecar file to Lode Store at the computed Hive path.
// Uses lazy store initialization via storeFactory.
func (c *LodeClient) PutFile(ctx context.Context, filename, _ string, data []byte) error {
	store, err := c.getOrCreateStore()
	if err != nil {
		return fmt.Errorf("file write store init failed: %w", err)
	}

	path := c.buildFilePath(filename)
	return store.Put(ctx, path, bytes.NewReader(data))
}

// getOrCreateStore lazily initializes the Store from the factory.
func (c *LodeClient) getOrCreateStore() (lode.Store, error) {
	c.storeOnce.Do(func() {
		c.store, c.storeErr = c.storeFactory()
	})
	return c.store, c.storeErr
}

// buildFilePath computes the Hive-partitioned path for a sidecar file.
// Format: datasets/<dataset>/partitions/source=<s>/category=<c>/day=<d>/run_id=<r>/files/<filename>
func (c *LodeClient) buildFilePath(filename string) string {
	return fmt.Sprintf("datasets/%s/partitions/source=%s/category=%s/day=%s/run_id=%s/files/%s",
		c.config.Dataset,
		c.config.Source,
		c.config.Category,
		c.config.Day,
		c.config.RunID,
		filename,
	)
}

// StubFileWriter records PutFile calls for testing.
type StubFileWriter struct {
	mu    sync.Mutex
	Files []StubFileRecord
}

// StubFileRecord is a recorded file write for testing.
type StubFileRecord struct {
	Filename    string
	ContentType string
	Data        []byte
}

// NewStubFileWriter creates a new stub file writer.
func NewStubFileWriter() *StubFileWriter {
	return &StubFileWriter{}
}

// PutFile implements FileWriter by recording the call.
func (w *StubFileWriter) PutFile(_ context.Context, filename, contentType string, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Files = append(w.Files, StubFileRecord{
		Filename:    filename,
		ContentType: contentType,
		Data:        data,
	})
	return nil
}

// Verify StubFileWriter implements FileWriter.
var _ FileWriter = (*StubFileWriter)(nil)
