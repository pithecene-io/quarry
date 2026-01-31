package policy

import (
	"context"
	"sync"

	"github.com/justapithecus/quarry/types"
)

// StrictPolicy implements synchronous, unbuffered persistence.
//
// Per CONTRACT_POLICY.md:
//   - No buffering: each event/chunk is written immediately
//   - No drops: all events are persisted
//   - Backpressure: caller blocks on sink latency
//   - Sink errors fail the run
type StrictPolicy struct {
	sink Sink

	mu    sync.Mutex
	stats Stats
}

// NewStrictPolicy creates a new strict policy writing to the given sink.
func NewStrictPolicy(sink Sink) *StrictPolicy {
	return &StrictPolicy{
		sink: sink,
		stats: Stats{
			DroppedByType: make(map[types.EventType]int64),
		},
	}
}

// IngestEvent writes the event immediately to the sink.
// Returns error on sink failure (terminates run).
func (p *StrictPolicy) IngestEvent(ctx context.Context, envelope *types.EventEnvelope) error {
	p.mu.Lock()
	p.stats.TotalEvents++
	p.mu.Unlock()

	// Write immediately (batch of 1)
	if err := p.sink.WriteEvents(ctx, []*types.EventEnvelope{envelope}); err != nil {
		p.mu.Lock()
		p.stats.Errors++
		p.mu.Unlock()
		return err
	}

	p.mu.Lock()
	p.stats.EventsPersisted++
	p.mu.Unlock()

	return nil
}

// IngestArtifactChunk writes the chunk immediately to the sink.
// Returns error on sink failure (terminates run).
func (p *StrictPolicy) IngestArtifactChunk(ctx context.Context, chunk *types.ArtifactChunk) error {
	p.mu.Lock()
	p.stats.TotalChunks++
	p.mu.Unlock()

	// Write immediately (batch of 1)
	if err := p.sink.WriteChunks(ctx, []*types.ArtifactChunk{chunk}); err != nil {
		p.mu.Lock()
		p.stats.Errors++
		p.mu.Unlock()
		return err
	}

	p.mu.Lock()
	p.stats.ChunksPersisted++
	p.mu.Unlock()

	return nil
}

// Flush is a no-op for strict policy (nothing is buffered).
func (p *StrictPolicy) Flush(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.FlushCount++
	return nil
}

// Close closes the underlying sink.
func (p *StrictPolicy) Close() error {
	return p.sink.Close()
}

// Stats returns policy statistics.
func (p *StrictPolicy) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Return a copy with a copied map
	stats := p.stats
	stats.DroppedByType = make(map[types.EventType]int64, len(p.stats.DroppedByType))
	for k, v := range p.stats.DroppedByType {
		stats.DroppedByType[k] = v
	}

	return stats
}
