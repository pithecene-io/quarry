package lode

import (
	"context"
	"testing"

	"github.com/justapithecus/quarry/types"
)

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

	err := sink.WriteEvents(context.Background(), events)
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

	err := sink.WriteChunks(context.Background(), chunks)
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
