// Package policy defines the ingestion policy interface per CONTRACT_POLICY.md.
package policy

import (
	"context"
	"sync"

	"github.com/pithecene-io/quarry/types"
)

// Policy defines the ingestion policy interface.
// Policies control buffering, dropping, and persistence behavior.
//
// Per CONTRACT_POLICY.md:
//   - May drop: log, enqueue, rotate_proxy
//   - Must NOT drop: item, artifact, checkpoint, run_error, run_complete
//   - Policy must not alter event shapes
//   - Policy failure terminates the run
type Policy interface {
	// IngestEvent handles an event envelope.
	// May drop droppable event types (log, enqueue, rotate_proxy).
	// Must not drop non-droppable types; return error to terminate run.
	IngestEvent(ctx context.Context, envelope *types.EventEnvelope) error

	// IngestArtifactChunk handles an artifact chunk.
	// Must buffer/persist chunks in order.
	// Returns error on failure (terminates run).
	IngestArtifactChunk(ctx context.Context, chunk *types.ArtifactChunk) error

	// Flush flushes any buffered data.
	// Called on run_complete, run_error, or runtime termination.
	Flush(ctx context.Context) error

	// Close cleans up policy resources.
	Close() error

	// Stats returns policy statistics for observability.
	// Returns an atomic snapshot of policy metrics at a point in time.
	// All counters in the returned Stats are consistent with each other.
	Stats() Stats
}

// Stats represents policy observability metrics per CONTRACT_POLICY.md.
type Stats struct {
	// TotalEvents is the total number of events received.
	TotalEvents int64
	// EventsPersisted is the number of events persisted.
	EventsPersisted int64
	// EventsDropped is the total number of events dropped.
	EventsDropped int64
	// DroppedByType maps event types to drop counts.
	DroppedByType map[types.EventType]int64
	// TotalChunks is the total number of artifact chunks received.
	TotalChunks int64
	// ChunksPersisted is the number of chunks persisted.
	ChunksPersisted int64
	// BufferSize is the current buffer size in bytes (if buffered).
	BufferSize int64
	// FlushCount is the number of flush operations.
	FlushCount int64
	// Errors is the count of non-fatal errors encountered.
	Errors int64
}

// droppableTypes defines which event types may be dropped per CONTRACT_POLICY.md.
var droppableTypes = map[types.EventType]bool{
	types.EventTypeLog:         true,
	types.EventTypeEnqueue:     true,
	types.EventTypeRotateProxy: true,
}

// IsDroppable returns true if the event type may be dropped by policy.
func IsDroppable(eventType types.EventType) bool {
	return droppableTypes[eventType]
}

// DroppableTypes returns the set of event types that may be dropped.
func DroppableTypes() map[types.EventType]bool {
	// Return a copy to prevent mutation
	result := make(map[types.EventType]bool, len(droppableTypes))
	for k, v := range droppableTypes {
		result[k] = v
	}
	return result
}

// statsRecorder is an internal helper for thread-safe stats management.
// Policies call explicit methods to record mutations; recorder does not
// infer or automate any policy decisions.
//
// Lock discipline:
//   - StrictPolicy uses the locking methods (incTotalEvents, snapshot, etc.)
//   - BufferedPolicy uses the Locked methods (incTotalEventsLocked, snapshotLocked, etc.)
//     only while holding BufferedPolicy.mu. This ensures atomicity between buffer
//     state and stats counters.
type statsRecorder struct {
	mu    sync.Mutex
	stats Stats
}

// newStatsRecorder creates a new recorder with initialized DroppedByType map.
func newStatsRecorder() *statsRecorder {
	return &statsRecorder{
		stats: Stats{
			DroppedByType: make(map[types.EventType]int64),
		},
	}
}

func (r *statsRecorder) incTotalEvents() {
	r.mu.Lock()
	r.stats.TotalEvents++
	r.mu.Unlock()
}

func (r *statsRecorder) incEventsPersisted(n int64) {
	r.mu.Lock()
	r.stats.EventsPersisted += n
	r.mu.Unlock()
}

func (r *statsRecorder) incTotalChunks() {
	r.mu.Lock()
	r.stats.TotalChunks++
	r.mu.Unlock()
}

func (r *statsRecorder) incChunksPersisted(n int64) {
	r.mu.Lock()
	r.stats.ChunksPersisted += n
	r.mu.Unlock()
}

func (r *statsRecorder) incErrors() {
	r.mu.Lock()
	r.stats.Errors++
	r.mu.Unlock()
}

func (r *statsRecorder) incFlush() {
	r.mu.Lock()
	r.stats.FlushCount++
	r.mu.Unlock()
}

func (r *statsRecorder) snapshot() Stats {
	r.mu.Lock()
	defer r.mu.Unlock()

	s := r.stats
	s.DroppedByType = make(map[types.EventType]int64, len(r.stats.DroppedByType))
	for k, v := range r.stats.DroppedByType {
		s.DroppedByType[k] = v
	}
	return s
}

// --- Locked methods for BufferedPolicy ---
// Caller must hold BufferedPolicy.mu.

func (r *statsRecorder) incTotalEventsLocked() {
	r.stats.TotalEvents++
}

func (r *statsRecorder) incEventsPersistedLocked(n int64) {
	r.stats.EventsPersisted += n
}

func (r *statsRecorder) incEventsDroppedLocked(eventType types.EventType) {
	r.stats.EventsDropped++
	r.stats.DroppedByType[eventType]++
}

func (r *statsRecorder) incTotalChunksLocked() {
	r.stats.TotalChunks++
}

func (r *statsRecorder) incChunksPersistedLocked(n int64) {
	r.stats.ChunksPersisted += n
}

func (r *statsRecorder) incErrorsLocked() {
	r.stats.Errors++
}

func (r *statsRecorder) incFlushLocked() {
	r.stats.FlushCount++
}

func (r *statsRecorder) setBufferSizeLocked(bytes int64) {
	r.stats.BufferSize = bytes
}

// snapshotLocked returns an atomic snapshot of stats with the given bufferSize.
// Caller must hold BufferedPolicy.mu.
func (r *statsRecorder) snapshotLocked(bufferSize int64) Stats {
	s := r.stats
	s.BufferSize = bufferSize
	s.DroppedByType = make(map[types.EventType]int64, len(r.stats.DroppedByType))
	for k, v := range r.stats.DroppedByType {
		s.DroppedByType[k] = v
	}
	return s
}
