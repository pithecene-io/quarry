// Package lode provides Lode integration per CONTRACT_LODE.md.
//
// This package defines the interface and stub for Lode persistence.
// Real implementations will connect to the actual Lode storage system.
package lode

import (
	"context"

	"github.com/justapithecus/quarry/policy"
	"github.com/justapithecus/quarry/types"
)

// Config holds Lode sink configuration.
type Config struct {
	// Dataset is the logical collection/dataset for partitioning.
	Dataset string
	// RunID is the run identifier for partitioning.
	RunID string
}

// Sink is a Lode-backed implementation of policy.Sink.
// Writes events and chunks to Lode storage per CONTRACT_LODE.md.
type Sink struct {
	config Config
	client Client
}

// Client abstracts the Lode storage client.
// Real implementations connect to Lode; stubs are used for testing.
type Client interface {
	// WriteEvents writes a batch of events to Lode.
	// Must preserve ordering within the batch.
	WriteEvents(ctx context.Context, dataset, runID string, events []*types.EventEnvelope) error

	// WriteChunks writes a batch of artifact chunks to Lode.
	// Must preserve ordering within the batch.
	WriteChunks(ctx context.Context, dataset, runID string, chunks []*types.ArtifactChunk) error

	// Close releases client resources.
	Close() error
}

// NewSink creates a new Lode sink.
func NewSink(config Config, client Client) *Sink {
	return &Sink{
		config: config,
		client: client,
	}
}

// WriteEvents implements policy.Sink.
func (s *Sink) WriteEvents(ctx context.Context, events []*types.EventEnvelope) error {
	return s.client.WriteEvents(ctx, s.config.Dataset, s.config.RunID, events)
}

// WriteChunks implements policy.Sink.
func (s *Sink) WriteChunks(ctx context.Context, chunks []*types.ArtifactChunk) error {
	return s.client.WriteChunks(ctx, s.config.Dataset, s.config.RunID, chunks)
}

// Close implements policy.Sink.
func (s *Sink) Close() error {
	return s.client.Close()
}

// Verify Sink implements policy.Sink.
var _ policy.Sink = (*Sink)(nil)

// StubClient is a test client that accepts writes without persisting.
// Use for integration testing before real Lode is available.
type StubClient struct {
	Events []EventRecord
	Chunks []ChunkRecord
	Closed bool
}

// EventRecord is a recorded event write.
type EventRecord struct {
	Dataset string
	RunID   string
	Events  []*types.EventEnvelope
}

// ChunkRecord is a recorded chunk write.
type ChunkRecord struct {
	Dataset string
	RunID   string
	Chunks  []*types.ArtifactChunk
}

// NewStubClient creates a new stub client.
func NewStubClient() *StubClient {
	return &StubClient{}
}

// WriteEvents implements Client.
func (c *StubClient) WriteEvents(ctx context.Context, dataset, runID string, events []*types.EventEnvelope) error {
	c.Events = append(c.Events, EventRecord{
		Dataset: dataset,
		RunID:   runID,
		Events:  events,
	})
	return nil
}

// WriteChunks implements Client.
func (c *StubClient) WriteChunks(ctx context.Context, dataset, runID string, chunks []*types.ArtifactChunk) error {
	c.Chunks = append(c.Chunks, ChunkRecord{
		Dataset: dataset,
		RunID:   runID,
		Chunks:  chunks,
	})
	return nil
}

// Close implements Client.
func (c *StubClient) Close() error {
	c.Closed = true
	return nil
}

// Verify StubClient implements Client.
var _ Client = (*StubClient)(nil)
