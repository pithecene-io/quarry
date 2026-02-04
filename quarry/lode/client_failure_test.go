package lode

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justapithecus/lode/lode"

	"github.com/justapithecus/quarry/types"
)

// =============================================================================
// Phase 8: Storage Failure Hardening Tests
// =============================================================================

// FailingStore is a lode.Store that returns configurable errors.
type FailingStore struct {
	PutErr    error
	GetErr    error
	ExistsErr error
	ListErr   error
	DeleteErr error

	// Track calls for verification
	PutCalls    int
	PutPaths    []string
	CloseCalled bool
}

func (s *FailingStore) Put(_ context.Context, path string, _ io.Reader) error {
	s.PutCalls++
	s.PutPaths = append(s.PutPaths, path)
	return s.PutErr
}

func (s *FailingStore) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, s.GetErr
}

func (s *FailingStore) Exists(_ context.Context, _ string) (bool, error) {
	return false, s.ExistsErr
}

func (s *FailingStore) List(_ context.Context, _ string) ([]string, error) {
	return nil, s.ListErr
}

func (s *FailingStore) Delete(_ context.Context, _ string) error {
	return s.DeleteErr
}

func (s *FailingStore) ReadRange(_ context.Context, _ string, _, _ int64) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func (s *FailingStore) ReaderAt(_ context.Context, _ string) (io.ReaderAt, error) {
	return nil, errors.New("not implemented")
}

var _ lode.Store = (*FailingStore)(nil)

// FailingStoreFactory creates a factory that returns a FailingStore.
func FailingStoreFactory(store *FailingStore) lode.StoreFactory {
	return func() (lode.Store, error) {
		return store, nil
	}
}

// FailingFactoryFactory creates a factory that fails to create a store.
func FailingFactoryFactory(err error) lode.StoreFactory {
	return func() (lode.Store, error) {
		return nil, err
	}
}

// =============================================================================
// FS: Directory Creation Failure Tests
// =============================================================================

func TestLodeClient_FSDirectoryCreationFailure_NonExistentParent(t *testing.T) {
	// Use t.TempDir() for isolation, then reference a non-existent subdirectory
	tmpDir := t.TempDir()
	nonExistentPath := filepath.Join(tmpDir, "does-not-exist", "nested", "path")

	cfg := Config{
		Dataset:  "quarry",
		Source:   "test",
		Category: "test",
		Day:      "2026-02-03",
		RunID:    "run-1",
	}

	// Failure can occur at factory creation OR at write time
	client, factoryErr := NewLodeClient(cfg, nonExistentPath)

	if factoryErr != nil {
		// Factory creation failed - this is valid behavior
		// Assert it's a path-related error
		errStr := factoryErr.Error()
		if !strings.Contains(errStr, "no such file") &&
			!strings.Contains(errStr, "does not exist") &&
			!strings.Contains(errStr, "not a directory") {
			t.Errorf("factory error should be path-related, got: %v", factoryErr)
		}
		return
	}
	defer func() { _ = client.Close() }()

	// Factory succeeded - write must fail
	events := []*types.EventEnvelope{
		{Type: types.EventTypeItem, Seq: 1, Payload: map[string]any{"key": "value"}},
	}

	writeErr := client.WriteEvents(t.Context(), cfg.Dataset, cfg.RunID, events)
	if writeErr == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}

	// Assert it's a path-related error
	errStr := writeErr.Error()
	if !strings.Contains(errStr, "no such file") &&
		!strings.Contains(errStr, "does not exist") &&
		!strings.Contains(errStr, "not a directory") {
		t.Errorf("write error should be path-related, got: %v", writeErr)
	}
}

func TestLodeClient_FSDirectoryCreationFailure_ReadOnlyParent(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping: test requires non-root user")
	}

	// Create a read-only directory
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0o555); err != nil {
		t.Fatalf("failed to create read-only dir: %v", err)
	}

	cfg := Config{
		Dataset:  "quarry",
		Source:   "test",
		Category: "test",
		Day:      "2026-02-03",
		RunID:    "run-1",
	}

	// Try to use a subdirectory of the read-only dir
	storePath := filepath.Join(readOnlyDir, "data")
	client, factoryErr := NewLodeClient(cfg, storePath)

	if factoryErr != nil {
		// Factory creation failed - this is valid behavior
		// The factory may fail with:
		// - "permission denied" if it tries to create the subdirectory
		// - "no such file" if it stats the non-existent subdirectory first
		// Both are acceptable since the root cause is the read-only parent
		errStr := factoryErr.Error()
		if !strings.Contains(errStr, "permission denied") &&
			!strings.Contains(errStr, "read-only") &&
			!strings.Contains(errStr, "EACCES") &&
			!strings.Contains(errStr, "no such file") &&
			!strings.Contains(errStr, "does not exist") {
			t.Errorf("factory error should be path/permission-related, got: %v", factoryErr)
		}
		return
	}
	defer func() { _ = client.Close() }()

	// Factory succeeded - write must fail with permission error
	events := []*types.EventEnvelope{
		{Type: types.EventTypeItem, Seq: 1, Payload: map[string]any{"key": "value"}},
	}

	writeErr := client.WriteEvents(t.Context(), cfg.Dataset, cfg.RunID, events)
	if writeErr == nil {
		t.Fatal("expected permission error, got nil")
	}

	// Assert it's a permission or path-related error
	errStr := writeErr.Error()
	if !strings.Contains(errStr, "permission denied") &&
		!strings.Contains(errStr, "read-only") &&
		!strings.Contains(errStr, "EACCES") &&
		!strings.Contains(errStr, "no such file") {
		t.Errorf("write error should be path/permission-related, got: %v", writeErr)
	}
}

// =============================================================================
// FS: File Write Failure Tests
// =============================================================================

// DiskFullError simulates ENOSPC
type DiskFullError struct {
	Path string
}

func (e *DiskFullError) Error() string {
	return fmt.Sprintf("write %s: no space left on device", e.Path)
}

// PermissionDeniedError simulates EACCES
type PermissionDeniedError struct {
	Path string
}

func (e *PermissionDeniedError) Error() string {
	return fmt.Sprintf("write %s: permission denied", e.Path)
}

func TestLodeClient_WriteFailure_DiskFull(t *testing.T) {
	store := &FailingStore{
		PutErr: &DiskFullError{Path: "/data/quarry/events.jsonl"},
	}

	cfg := Config{
		Dataset:  "quarry",
		Source:   "test",
		Category: "test",
		Day:      "2026-02-03",
		RunID:    "run-1",
	}

	client, err := NewLodeClientWithFactory(cfg, FailingStoreFactory(store))
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	events := []*types.EventEnvelope{
		{Type: types.EventTypeItem, Seq: 1, Payload: map[string]any{"key": "value"}},
	}

	err = client.WriteEvents(t.Context(), cfg.Dataset, cfg.RunID, events)
	if err == nil {
		t.Fatal("expected disk full error, got nil")
	}

	// Verify error propagates
	var diskFullErr *DiskFullError
	if !errors.As(err, &diskFullErr) {
		// Check if error message contains the info
		if !strings.Contains(err.Error(), "no space left on device") {
			t.Errorf("expected disk full error, got: %v", err)
		}
	}

	// Verify write was attempted
	if store.PutCalls != 1 {
		t.Errorf("expected 1 put call, got %d", store.PutCalls)
	}
}

func TestLodeClient_WriteFailure_PermissionDenied(t *testing.T) {
	store := &FailingStore{
		PutErr: &PermissionDeniedError{Path: "/data/quarry/events.jsonl"},
	}

	cfg := Config{
		Dataset:  "quarry",
		Source:   "test",
		Category: "test",
		Day:      "2026-02-03",
		RunID:    "run-1",
	}

	client, err := NewLodeClientWithFactory(cfg, FailingStoreFactory(store))
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	events := []*types.EventEnvelope{
		{Type: types.EventTypeItem, Seq: 1, Payload: map[string]any{"key": "value"}},
	}

	err = client.WriteEvents(t.Context(), cfg.Dataset, cfg.RunID, events)
	if err == nil {
		t.Fatal("expected permission error, got nil")
	}

	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected permission denied error, got: %v", err)
	}
}

func TestLodeClient_ChunkWriteFailure_DiskFull(t *testing.T) {
	store := &FailingStore{
		PutErr: &DiskFullError{Path: "/data/quarry/chunks.jsonl"},
	}

	cfg := Config{
		Dataset:  "quarry",
		Source:   "test",
		Category: "test",
		Day:      "2026-02-03",
		RunID:    "run-1",
	}

	client, err := NewLodeClientWithFactory(cfg, FailingStoreFactory(store))
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	chunks := []*types.ArtifactChunk{
		{ArtifactID: "art-1", Seq: 1, Data: []byte("data")},
	}

	err = client.WriteChunks(t.Context(), cfg.Dataset, cfg.RunID, chunks)
	if err == nil {
		t.Fatal("expected disk full error, got nil")
	}

	if !strings.Contains(err.Error(), "no space left on device") {
		t.Errorf("expected disk full error, got: %v", err)
	}
}

// =============================================================================
// FS: Atomic Write Semantics / Partial Write Detection
// =============================================================================

func TestLodeClient_PartialWriteDetection_ChunksNotMarkedOnFailure(t *testing.T) {
	// Verify that on write failure, chunks are not marked as seen
	store := &FailingStore{
		PutErr: errors.New("simulated write failure"),
	}

	cfg := Config{
		Dataset:  "quarry",
		Source:   "test",
		Category: "test",
		Day:      "2026-02-03",
		RunID:    "run-1",
	}

	client, err := NewLodeClientWithFactory(cfg, FailingStoreFactory(store))
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	chunks := []*types.ArtifactChunk{
		{ArtifactID: "art-1", Seq: 1, Data: []byte("data")},
	}

	// Write should fail
	err = client.WriteChunks(t.Context(), cfg.Dataset, cfg.RunID, chunks)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify chunks are NOT marked as seen (no state mutation on failure)
	if _, seen := client.chunksSeen["art-1"]; seen {
		t.Error("chunks should not be marked as seen after write failure")
	}

	// Verify offset is NOT updated
	if offset := client.offsets["art-1"]; offset != 0 {
		t.Errorf("offset should be 0 after failure, got %d", offset)
	}
}

func TestLodeClient_PartialWriteDetection_OffsetsNotUpdatedOnFailure(t *testing.T) {
	cfg := Config{
		Dataset:  "quarry",
		Source:   "test",
		Category: "test",
		Day:      "2026-02-03",
		RunID:    "run-1",
	}

	// First, successfully write some chunks
	successStore := &FailingStore{PutErr: nil}
	client, err := NewLodeClientWithFactory(cfg, FailingStoreFactory(successStore))
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	chunks1 := []*types.ArtifactChunk{
		{ArtifactID: "art-1", Seq: 1, Data: []byte("12345")}, // 5 bytes
	}
	if err := client.WriteChunks(t.Context(), cfg.Dataset, cfg.RunID, chunks1); err != nil {
		t.Fatalf("first write failed: %v", err)
	}

	// Verify offset is 5
	if client.offsets["art-1"] != 5 {
		t.Errorf("offset after first write = %d, want 5", client.offsets["art-1"])
	}

	// Now make the store fail
	successStore.PutErr = errors.New("simulated failure")

	chunks2 := []*types.ArtifactChunk{
		{ArtifactID: "art-1", Seq: 2, Data: []byte("67890")}, // 5 more bytes
	}
	err = client.WriteChunks(t.Context(), cfg.Dataset, cfg.RunID, chunks2)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify offset is STILL 5 (not updated due to failure)
	if client.offsets["art-1"] != 5 {
		t.Errorf("offset after failed write = %d, want 5 (unchanged)", client.offsets["art-1"])
	}
}

// =============================================================================
// S3: Auth Failure Tests
// =============================================================================

// S3AuthError simulates AWS authentication failure
type S3AuthError struct {
	Message string
}

func (e *S3AuthError) Error() string {
	return fmt.Sprintf("NoCredentialProviders: %s", e.Message)
}

func TestLodeClient_S3AuthFailure(t *testing.T) {
	store := &FailingStore{
		PutErr: &S3AuthError{Message: "no valid credentials found"},
	}

	cfg := Config{
		Dataset:  "quarry",
		Source:   "test",
		Category: "test",
		Day:      "2026-02-03",
		RunID:    "run-1",
	}

	client, err := NewLodeClientWithFactory(cfg, FailingStoreFactory(store))
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	events := []*types.EventEnvelope{
		{Type: types.EventTypeItem, Seq: 1, Payload: map[string]any{"key": "value"}},
	}

	err = client.WriteEvents(t.Context(), cfg.Dataset, cfg.RunID, events)
	if err == nil {
		t.Fatal("expected auth error, got nil")
	}

	if !strings.Contains(err.Error(), "NoCredentialProviders") &&
		!strings.Contains(err.Error(), "credentials") {
		t.Errorf("expected auth-related error, got: %v", err)
	}
}

// =============================================================================
// S3: Access Denied Tests
// =============================================================================

// S3AccessDeniedError simulates AWS access denied
type S3AccessDeniedError struct {
	Bucket string
	Key    string
}

func (e *S3AccessDeniedError) Error() string {
	return fmt.Sprintf("AccessDenied: Access Denied for s3://%s/%s", e.Bucket, e.Key)
}

func TestLodeClient_S3AccessDenied(t *testing.T) {
	store := &FailingStore{
		PutErr: &S3AccessDeniedError{Bucket: "my-bucket", Key: "quarry/data.jsonl"},
	}

	cfg := Config{
		Dataset:  "quarry",
		Source:   "test",
		Category: "test",
		Day:      "2026-02-03",
		RunID:    "run-1",
	}

	client, err := NewLodeClientWithFactory(cfg, FailingStoreFactory(store))
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	events := []*types.EventEnvelope{
		{Type: types.EventTypeItem, Seq: 1, Payload: map[string]any{"key": "value"}},
	}

	err = client.WriteEvents(t.Context(), cfg.Dataset, cfg.RunID, events)
	if err == nil {
		t.Fatal("expected access denied error, got nil")
	}

	if !strings.Contains(err.Error(), "AccessDenied") &&
		!strings.Contains(err.Error(), "Access Denied") {
		t.Errorf("expected access denied error, got: %v", err)
	}
}

// =============================================================================
// S3: Network Timeout Tests
// =============================================================================

// S3TimeoutError simulates network timeout
type S3TimeoutError struct {
	Operation string
}

func (e *S3TimeoutError) Error() string {
	return fmt.Sprintf("RequestTimeout: %s timed out after 30s", e.Operation)
}

func (e *S3TimeoutError) Timeout() bool { return true }

func TestLodeClient_S3NetworkTimeout(t *testing.T) {
	store := &FailingStore{
		PutErr: &S3TimeoutError{Operation: "PutObject"},
	}

	cfg := Config{
		Dataset:  "quarry",
		Source:   "test",
		Category: "test",
		Day:      "2026-02-03",
		RunID:    "run-1",
	}

	client, err := NewLodeClientWithFactory(cfg, FailingStoreFactory(store))
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	events := []*types.EventEnvelope{
		{Type: types.EventTypeItem, Seq: 1, Payload: map[string]any{"key": "value"}},
	}

	err = client.WriteEvents(t.Context(), cfg.Dataset, cfg.RunID, events)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "Timeout") &&
		!strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

// =============================================================================
// S3: Throttling (429) Tests
// =============================================================================

// S3ThrottlingError simulates rate limiting
type S3ThrottlingError struct {
	RetryAfter int
}

func (e *S3ThrottlingError) Error() string {
	return fmt.Sprintf("SlowDown: Rate exceeded, retry after %ds", e.RetryAfter)
}

func TestLodeClient_S3Throttling(t *testing.T) {
	store := &FailingStore{
		PutErr: &S3ThrottlingError{RetryAfter: 5},
	}

	cfg := Config{
		Dataset:  "quarry",
		Source:   "test",
		Category: "test",
		Day:      "2026-02-03",
		RunID:    "run-1",
	}

	client, err := NewLodeClientWithFactory(cfg, FailingStoreFactory(store))
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	events := []*types.EventEnvelope{
		{Type: types.EventTypeItem, Seq: 1, Payload: map[string]any{"key": "value"}},
	}

	err = client.WriteEvents(t.Context(), cfg.Dataset, cfg.RunID, events)
	if err == nil {
		t.Fatal("expected throttling error, got nil")
	}

	if !strings.Contains(err.Error(), "SlowDown") &&
		!strings.Contains(err.Error(), "Rate exceeded") {
		t.Errorf("expected throttling error, got: %v", err)
	}
}

// =============================================================================
// Error Messages Include Storage Context
// =============================================================================

func TestLodeClient_ErrorContainsStorageContext(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantText []string
	}{
		{
			name:     "disk full includes path",
			err:      &DiskFullError{Path: "/var/quarry/data/events.jsonl"},
			wantText: []string{"/var/quarry/data", "no space left"},
		},
		{
			name:     "permission denied includes path",
			err:      &PermissionDeniedError{Path: "/var/quarry/data/events.jsonl"},
			wantText: []string{"/var/quarry/data", "permission denied"},
		},
		{
			name:     "S3 access denied includes bucket",
			err:      &S3AccessDeniedError{Bucket: "my-bucket", Key: "quarry/run-1/data.jsonl"},
			wantText: []string{"my-bucket", "AccessDenied"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &FailingStore{PutErr: tt.err}

			cfg := Config{
				Dataset:  "quarry",
				Source:   "test",
				Category: "test",
				Day:      "2026-02-03",
				RunID:    "run-1",
			}

			client, err := NewLodeClientWithFactory(cfg, FailingStoreFactory(store))
			if err != nil {
				t.Fatalf("NewLodeClientWithFactory failed: %v", err)
			}

			events := []*types.EventEnvelope{
				{Type: types.EventTypeItem, Seq: 1, Payload: map[string]any{"key": "value"}},
			}

			err = client.WriteEvents(t.Context(), cfg.Dataset, cfg.RunID, events)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			errStr := err.Error()
			for _, want := range tt.wantText {
				if !strings.Contains(errStr, want) {
					t.Errorf("error %q should contain %q", errStr, want)
				}
			}
		})
	}
}

// =============================================================================
// Policy Failure Propagation (Storage Errors → Run Outcome)
// =============================================================================

func TestLodeClient_ErrorPropagation_EventWrite(t *testing.T) {
	originalErr := errors.New("storage backend unavailable")
	store := &FailingStore{PutErr: originalErr}

	cfg := Config{
		Dataset:  "quarry",
		Source:   "test",
		Category: "test",
		Day:      "2026-02-03",
		RunID:    "run-1",
	}

	client, err := NewLodeClientWithFactory(cfg, FailingStoreFactory(store))
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	events := []*types.EventEnvelope{
		{Type: types.EventTypeItem, Seq: 1, Payload: map[string]any{"key": "value"}},
	}

	err = client.WriteEvents(t.Context(), cfg.Dataset, cfg.RunID, events)

	// Error must propagate (not be swallowed)
	if err == nil {
		t.Fatal("error was swallowed, expected propagation")
	}

	// Original error should be in the chain
	if !strings.Contains(err.Error(), "storage backend unavailable") {
		t.Errorf("original error not in chain: %v", err)
	}
}

func TestLodeClient_ErrorPropagation_ChunkWrite(t *testing.T) {
	originalErr := errors.New("storage backend unavailable")
	store := &FailingStore{PutErr: originalErr}

	cfg := Config{
		Dataset:  "quarry",
		Source:   "test",
		Category: "test",
		Day:      "2026-02-03",
		RunID:    "run-1",
	}

	client, err := NewLodeClientWithFactory(cfg, FailingStoreFactory(store))
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	chunks := []*types.ArtifactChunk{
		{ArtifactID: "art-1", Seq: 1, Data: []byte("data")},
	}

	err = client.WriteChunks(t.Context(), cfg.Dataset, cfg.RunID, chunks)

	// Error must propagate
	if err == nil {
		t.Fatal("error was swallowed, expected propagation")
	}

	if !strings.Contains(err.Error(), "storage backend unavailable") {
		t.Errorf("original error not in chain: %v", err)
	}
}

// =============================================================================
// No Silent Corruption Paths
// =============================================================================

func TestLodeClient_NoSilentCorruption_FailedWritePreservesState(t *testing.T) {
	cfg := Config{
		Dataset:  "quarry",
		Source:   "test",
		Category: "test",
		Day:      "2026-02-03",
		RunID:    "run-1",
	}

	// Start with working store
	store := &FailingStore{PutErr: nil}
	client, err := NewLodeClientWithFactory(cfg, FailingStoreFactory(store))
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	ctx := t.Context()

	// Write first batch successfully
	chunks1 := []*types.ArtifactChunk{
		{ArtifactID: "art-1", Seq: 1, Data: []byte("first-chunk")},
	}
	if err := client.WriteChunks(ctx, cfg.Dataset, cfg.RunID, chunks1); err != nil {
		t.Fatalf("first write failed: %v", err)
	}

	initialOffset := client.offsets["art-1"]
	if initialOffset != 11 { // len("first-chunk")
		t.Fatalf("initial offset = %d, want 11", initialOffset)
	}

	// Now make store fail
	store.PutErr = errors.New("simulated corruption scenario")

	// Try to write second batch (will fail)
	chunks2 := []*types.ArtifactChunk{
		{ArtifactID: "art-1", Seq: 2, Data: []byte("second-chunk")},
	}
	err = client.WriteChunks(ctx, cfg.Dataset, cfg.RunID, chunks2)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify state is unchanged (no partial mutation)
	if client.offsets["art-1"] != initialOffset {
		t.Errorf("offset changed on failure: got %d, want %d", client.offsets["art-1"], initialOffset)
	}

	// Restore store and verify we can continue from correct offset
	store.PutErr = nil
	if err := client.WriteChunks(ctx, cfg.Dataset, cfg.RunID, chunks2); err != nil {
		t.Fatalf("retry failed: %v", err)
	}

	// Offset should now reflect both batches
	expectedOffset := int64(11 + 12) // "first-chunk" + "second-chunk"
	if client.offsets["art-1"] != expectedOffset {
		t.Errorf("final offset = %d, want %d", client.offsets["art-1"], expectedOffset)
	}
}

func TestLodeClient_NoSilentCorruption_CommitFailurePreservesChunksState(t *testing.T) {
	cfg := Config{
		Dataset:  "quarry",
		Source:   "test",
		Category: "test",
		Day:      "2026-02-03",
		RunID:    "run-1",
	}

	// Start with working store
	store := &FailingStore{PutErr: nil}
	client, err := NewLodeClientWithFactory(cfg, FailingStoreFactory(store))
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	ctx := t.Context()

	// Write chunks
	chunks := []*types.ArtifactChunk{
		{ArtifactID: "art-1", Seq: 1, Data: []byte("data")},
	}
	if err := client.WriteChunks(ctx, cfg.Dataset, cfg.RunID, chunks); err != nil {
		t.Fatalf("chunk write failed: %v", err)
	}

	// Verify chunks are marked
	if _, seen := client.chunksSeen["art-1"]; !seen {
		t.Fatal("chunks should be marked after write")
	}

	// Make commit fail
	store.PutErr = errors.New("commit failed")

	commitEvent := &types.EventEnvelope{
		Type: types.EventTypeArtifact,
		Seq:  2,
		Payload: map[string]any{
			"artifact_id":  "art-1",
			"name":         "test.txt",
			"content_type": "text/plain",
			"size_bytes":   float64(4),
		},
	}

	err = client.WriteEvents(ctx, cfg.Dataset, cfg.RunID, []*types.EventEnvelope{commitEvent})
	if err == nil {
		t.Fatal("expected commit error, got nil")
	}

	// Verify chunks are STILL marked (commit failure doesn't clear state)
	if _, seen := client.chunksSeen["art-1"]; !seen {
		t.Error("chunks should still be marked after failed commit")
	}

	// Verify offset is preserved
	if client.offsets["art-1"] != 4 {
		t.Errorf("offset should be preserved: got %d, want 4", client.offsets["art-1"])
	}
}
