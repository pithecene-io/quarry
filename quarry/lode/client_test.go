package lode

import (
	"context"
	"testing"

	"github.com/justapithecus/lode/lode"
	"github.com/justapithecus/quarry/types"
)

func TestLodeClient_WriteEvents(t *testing.T) {
	cfg := Config{
		Dataset:  "quarry",
		Source:   "test-source",
		Category: "test-category",
		Day:      "2026-02-03",
		RunID:    "run-123",
	}

	client, err := NewLodeClientWithFactory(cfg, lode.NewMemoryFactory())
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	events := []*types.EventEnvelope{
		{
			ContractVersion: "1.0.0",
			EventID:         "evt-1",
			RunID:           "run-123",
			Seq:             1,
			Type:            types.EventTypeItem,
			Ts:              "2026-02-03T12:00:00Z",
			Payload:         map[string]any{"key": "value"},
			Attempt:         1,
		},
		{
			ContractVersion: "1.0.0",
			EventID:         "evt-2",
			RunID:           "run-123",
			Seq:             2,
			Type:            types.EventTypeLog,
			Ts:              "2026-02-03T12:00:01Z",
			Payload:         map[string]any{"message": "test log"},
			Attempt:         1,
		},
	}

	err = client.WriteEvents(context.Background(), cfg.Dataset, cfg.RunID, events)
	if err != nil {
		t.Fatalf("WriteEvents failed: %v", err)
	}
	// Success: records written without error
}

func TestLodeClient_WriteArtifactEvent(t *testing.T) {
	cfg := Config{
		Dataset:  "quarry",
		Source:   "test-source",
		Category: "test-category",
		Day:      "2026-02-03",
		RunID:    "run-123",
	}

	client, err := NewLodeClientWithFactory(cfg, lode.NewMemoryFactory())
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	events := []*types.EventEnvelope{
		{
			ContractVersion: "1.0.0",
			EventID:         "evt-3",
			RunID:           "run-123",
			Seq:             3,
			Type:            types.EventTypeArtifact,
			Ts:              "2026-02-03T12:00:02Z",
			Payload: map[string]any{
				"artifact_id":  "art-1",
				"name":         "screenshot.png",
				"content_type": "image/png",
				"size_bytes":   float64(1024), // JSON unmarshals numbers as float64
			},
			Attempt: 1,
		},
	}

	err = client.WriteEvents(context.Background(), cfg.Dataset, cfg.RunID, events)
	if err != nil {
		t.Fatalf("WriteEvents (artifact) failed: %v", err)
	}
	// Success: artifact commit record written
}

func TestLodeClient_WriteChunks(t *testing.T) {
	cfg := Config{
		Dataset:  "quarry",
		Source:   "test-source",
		Category: "test-category",
		Day:      "2026-02-03",
		RunID:    "run-123",
	}

	client, err := NewLodeClientWithFactory(cfg, lode.NewMemoryFactory())
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	chunks := []*types.ArtifactChunk{
		{
			ArtifactID: "art-1",
			Seq:        1,
			IsLast:     false,
			Data:       []byte("hello "),
		},
		{
			ArtifactID: "art-1",
			Seq:        2,
			IsLast:     true,
			Data:       []byte("world"),
		},
	}

	err = client.WriteChunks(context.Background(), cfg.Dataset, cfg.RunID, chunks)
	if err != nil {
		t.Fatalf("WriteChunks failed: %v", err)
	}
	// Success: chunk records written
}

func TestLodeClient_ChunkOffset(t *testing.T) {
	cfg := Config{
		Dataset:  "quarry",
		Source:   "test-source",
		Category: "test-category",
		Day:      "2026-02-03",
		RunID:    "run-123",
	}

	// Test that offsets are computed correctly per-artifact
	chunks := []*types.ArtifactChunk{
		{ArtifactID: "art-1", Seq: 1, Data: []byte("12345")},      // offset 0, length 5
		{ArtifactID: "art-1", Seq: 2, Data: []byte("67890")},      // offset 5, length 5
		{ArtifactID: "art-2", Seq: 1, Data: []byte("abc")},        // offset 0 (different artifact)
		{ArtifactID: "art-1", Seq: 3, IsLast: true, Data: []byte("!")}, // offset 10, length 1
	}

	// Verify the toChunkRecord offset computation
	offsets := make(map[string]int64)
	for _, chunk := range chunks {
		offset := offsets[chunk.ArtifactID]
		record := toChunkRecord(chunk, offset, cfg)

		expectedOffset := offsets[chunk.ArtifactID]
		if record.Offset != expectedOffset {
			t.Errorf("chunk %s seq %d: offset = %d, want %d",
				chunk.ArtifactID, chunk.Seq, record.Offset, expectedOffset)
		}
		if record.Length != int64(len(chunk.Data)) {
			t.Errorf("chunk %s seq %d: length = %d, want %d",
				chunk.ArtifactID, chunk.Seq, record.Length, len(chunk.Data))
		}

		offsets[chunk.ArtifactID] = offset + int64(len(chunk.Data))
	}
}

func TestLodeClient_ChunkOffset_AcrossBatches(t *testing.T) {
	cfg := Config{
		Dataset:  "quarry",
		Source:   "test-source",
		Category: "test-category",
		Day:      "2026-02-03",
		RunID:    "run-123",
	}

	client, err := NewLodeClientWithFactory(cfg, lode.NewMemoryFactory())
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	ctx := context.Background()

	// First batch: 10 bytes for art-1
	batch1 := []*types.ArtifactChunk{
		{ArtifactID: "art-1", Seq: 1, Data: []byte("0123456789")}, // 10 bytes
	}
	if err := client.WriteChunks(ctx, cfg.Dataset, cfg.RunID, batch1); err != nil {
		t.Fatalf("WriteChunks batch 1 failed: %v", err)
	}

	// Verify offset tracked
	if client.offsets["art-1"] != 10 {
		t.Errorf("after batch 1: offset = %d, want 10", client.offsets["art-1"])
	}

	// Second batch: 5 more bytes for art-1
	batch2 := []*types.ArtifactChunk{
		{ArtifactID: "art-1", Seq: 2, IsLast: true, Data: []byte("abcde")}, // 5 bytes
	}
	if err := client.WriteChunks(ctx, cfg.Dataset, cfg.RunID, batch2); err != nil {
		t.Fatalf("WriteChunks batch 2 failed: %v", err)
	}

	// Verify cumulative offset
	if client.offsets["art-1"] != 15 {
		t.Errorf("after batch 2: offset = %d, want 15", client.offsets["art-1"])
	}
}

func TestComputeMD5(t *testing.T) {
	data := []byte("hello world")
	got := computeMD5(data)
	want := "5eb63bbbe01eeed093cb22bb8f5acdc3" // known MD5 of "hello world"

	if got != want {
		t.Errorf("computeMD5() = %q, want %q", got, want)
	}
}

func TestToEventRecord(t *testing.T) {
	cfg := Config{
		Dataset:  "quarry",
		Source:   "my-source",
		Category: "my-category",
		Day:      "2026-02-03",
		RunID:    "run-abc",
	}

	jobID := "job-xyz"
	e := &types.EventEnvelope{
		ContractVersion: "1.0.0",
		EventID:         "evt-1",
		RunID:           "run-abc",
		Seq:             1,
		Type:            types.EventTypeItem,
		Ts:              "2026-02-03T12:00:00Z",
		Payload:         map[string]any{"foo": "bar"},
		JobID:           &jobID,
		Attempt:         1,
	}

	record := toEventRecord(e, cfg)

	if record.RecordKind != RecordKindEvent {
		t.Errorf("RecordKind = %q, want %q", record.RecordKind, RecordKindEvent)
	}
	if record.Source != cfg.Source {
		t.Errorf("Source = %q, want %q", record.Source, cfg.Source)
	}
	if record.Category != cfg.Category {
		t.Errorf("Category = %q, want %q", record.Category, cfg.Category)
	}
	if record.Day != cfg.Day {
		t.Errorf("Day = %q, want %q", record.Day, cfg.Day)
	}
	if record.EventID != e.EventID {
		t.Errorf("EventID = %q, want %q", record.EventID, e.EventID)
	}
}

func TestS3Config_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     S3Config
		wantErr bool
	}{
		{
			name:    "empty bucket fails",
			cfg:     S3Config{Bucket: ""},
			wantErr: true,
		},
		{
			name:    "valid bucket only",
			cfg:     S3Config{Bucket: "my-bucket"},
			wantErr: false,
		},
		{
			name:    "valid bucket with prefix",
			cfg:     S3Config{Bucket: "my-bucket", Prefix: "quarry/data"},
			wantErr: false,
		},
		{
			name:    "valid bucket with region",
			cfg:     S3Config{Bucket: "my-bucket", Region: "us-west-2"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseS3Path(t *testing.T) {
	tests := []struct {
		path       string
		wantBucket string
		wantPrefix string
	}{
		{"my-bucket", "my-bucket", ""},
		{"my-bucket/prefix", "my-bucket", "prefix"},
		{"my-bucket/multi/level/prefix", "my-bucket", "multi/level/prefix"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			bucket, prefix := ParseS3Path(tt.path)
			if bucket != tt.wantBucket {
				t.Errorf("bucket = %q, want %q", bucket, tt.wantBucket)
			}
			if prefix != tt.wantPrefix {
				t.Errorf("prefix = %q, want %q", prefix, tt.wantPrefix)
			}
		})
	}
}

func TestToArtifactCommitRecord(t *testing.T) {
	cfg := Config{
		Dataset:  "quarry",
		Source:   "my-source",
		Category: "my-category",
		Day:      "2026-02-03",
		RunID:    "run-abc",
	}

	e := &types.EventEnvelope{
		ContractVersion: "1.0.0",
		EventID:         "evt-2",
		RunID:           "run-abc",
		Seq:             2,
		Type:            types.EventTypeArtifact,
		Ts:              "2026-02-03T12:00:01Z",
		Payload: map[string]any{
			"artifact_id":  "art-123",
			"name":         "report.pdf",
			"content_type": "application/pdf",
			"size_bytes":   float64(2048),
		},
		Attempt: 1,
	}

	record := toArtifactCommitRecord(e, cfg)

	if record.RecordKind != RecordKindArtifactEvent {
		t.Errorf("RecordKind = %q, want %q", record.RecordKind, RecordKindArtifactEvent)
	}
	if record.ArtifactID != "art-123" {
		t.Errorf("ArtifactID = %q, want %q", record.ArtifactID, "art-123")
	}
	if record.Name != "report.pdf" {
		t.Errorf("Name = %q, want %q", record.Name, "report.pdf")
	}
	if record.ContentType != "application/pdf" {
		t.Errorf("ContentType = %q, want %q", record.ContentType, "application/pdf")
	}
	if record.SizeBytes != 2048 {
		t.Errorf("SizeBytes = %d, want %d", record.SizeBytes, 2048)
	}
}
