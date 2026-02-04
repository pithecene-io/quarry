package lode

import (
	"testing"
	"time"

	"github.com/justapithecus/quarry/types"
)

func TestDeriveDay(t *testing.T) {
	tests := []struct {
		name      string
		startTime time.Time
		want      string
	}{
		{
			name:      "UTC time",
			startTime: time.Date(2026, 2, 3, 14, 30, 0, 0, time.UTC),
			want:      "2026-02-03",
		},
		{
			name:      "Non-UTC time converts to UTC",
			startTime: time.Date(2026, 2, 3, 22, 0, 0, 0, time.FixedZone("EST", -5*3600)),
			want:      "2026-02-04", // 22:00 EST = 03:00 UTC next day
		},
		{
			name:      "Single digit month and day",
			startTime: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
			want:      "2026-01-05",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveDay(tt.startTime)
			if got != tt.want {
				t.Errorf("DeriveDay() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSink_WriteEvents(t *testing.T) {
	client := NewStubClient()
	sink := NewSink(Config{
		Dataset: "test-dataset",
		RunID:   "run-123",
	}, client)

	events := []*types.EventEnvelope{
		{Type: "item", RunID: "run-123", Seq: 1},
		{Type: "log", RunID: "run-123", Seq: 2},
	}

	err := sink.WriteEvents(t.Context(), events)
	if err != nil {
		t.Fatalf("WriteEvents failed: %v", err)
	}

	if len(client.Events) != 1 {
		t.Fatalf("expected 1 event record, got %d", len(client.Events))
	}

	record := client.Events[0]
	if record.Dataset != "test-dataset" {
		t.Errorf("Dataset = %q, want %q", record.Dataset, "test-dataset")
	}
	if record.RunID != "run-123" {
		t.Errorf("RunID = %q, want %q", record.RunID, "run-123")
	}
	if len(record.Events) != 2 {
		t.Errorf("len(Events) = %d, want 2", len(record.Events))
	}
}

func TestSink_WriteChunks(t *testing.T) {
	client := NewStubClient()
	sink := NewSink(Config{
		Dataset: "test-dataset",
		RunID:   "run-123",
	}, client)

	chunks := []*types.ArtifactChunk{
		{ArtifactID: "art-1", Seq: 1, Data: []byte("hello")},
		{ArtifactID: "art-1", Seq: 2, Data: []byte("world"), IsLast: true},
	}

	err := sink.WriteChunks(t.Context(), chunks)
	if err != nil {
		t.Fatalf("WriteChunks failed: %v", err)
	}

	if len(client.Chunks) != 1 {
		t.Fatalf("expected 1 chunk record, got %d", len(client.Chunks))
	}

	record := client.Chunks[0]
	if record.Dataset != "test-dataset" {
		t.Errorf("Dataset = %q, want %q", record.Dataset, "test-dataset")
	}
	if len(record.Chunks) != 2 {
		t.Errorf("len(Chunks) = %d, want 2", len(record.Chunks))
	}
}

func TestSink_Close(t *testing.T) {
	client := NewStubClient()
	sink := NewSink(Config{
		Dataset: "test-dataset",
		RunID:   "run-123",
	}, client)

	if client.Closed {
		t.Error("client should not be closed before Close()")
	}

	err := sink.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !client.Closed {
		t.Error("client should be closed after Close()")
	}
}
