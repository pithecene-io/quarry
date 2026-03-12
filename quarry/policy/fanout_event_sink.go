package policy

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/pithecene-io/quarry/types"
)

// SinkEntry pairs an EventSink with its delivery mode.
type SinkEntry struct {
	Sink     EventSink
	Delivery DeliveryMode
	// Label identifies this sink in log messages (e.g. "lode", "redisstream").
	Label string
}

// FanoutEventSink dispatches WriteEvents to multiple EventSink implementations
// concurrently. Delivery mode per sink controls error propagation:
//   - DeliveryMandatory: write failure propagates to the caller.
//   - DeliveryBestEffort: write failure is logged, not propagated.
//
// If multiple mandatory sinks fail, the first error is returned.
type FanoutEventSink struct {
	sinks []SinkEntry
}

// NewFanoutEventSink creates a fanout dispatcher for the given sink entries.
// Panics if sinks is empty — at least one event sink is required.
func NewFanoutEventSink(sinks []SinkEntry) *FanoutEventSink {
	if len(sinks) == 0 {
		panic("fanout event sink requires at least one sink entry")
	}
	entries := make([]SinkEntry, len(sinks))
	copy(entries, sinks)
	return &FanoutEventSink{sinks: entries}
}

// WriteEvents dispatches events to all sinks concurrently.
// Returns the first mandatory sink error, if any. Best-effort failures are logged.
func (f *FanoutEventSink) WriteEvents(ctx context.Context, events []*types.EventEnvelope) error {
	if len(f.sinks) == 1 {
		return f.writeSingle(ctx, events)
	}
	return f.writeFanout(ctx, events)
}

// writeSingle is the fast path for a single sink — no goroutines.
func (f *FanoutEventSink) writeSingle(ctx context.Context, events []*types.EventEnvelope) error {
	entry := f.sinks[0]
	err := entry.Sink.WriteEvents(ctx, events)
	if err == nil {
		return nil
	}
	if entry.Delivery == DeliveryBestEffort {
		log.Printf("Warning: best-effort event sink %q: %v", entry.Label, err)
		return nil
	}
	return err
}

// writeFanout dispatches to multiple sinks concurrently.
func (f *FanoutEventSink) writeFanout(ctx context.Context, events []*types.EventEnvelope) error {
	type result struct {
		idx int
		err error
	}

	var wg sync.WaitGroup
	results := make([]result, len(f.sinks))

	for i, entry := range f.sinks {
		wg.Add(1)
		go func(idx int, e SinkEntry) {
			defer wg.Done()
			results[idx] = result{idx: idx, err: e.Sink.WriteEvents(ctx, events)}
		}(i, entry)
	}
	wg.Wait()

	var mandatoryErrs []string
	for i, r := range results {
		if r.err == nil {
			continue
		}
		entry := f.sinks[i]
		if entry.Delivery == DeliveryBestEffort {
			log.Printf("Warning: best-effort event sink %q: %v", entry.Label, r.err)
			continue
		}
		mandatoryErrs = append(mandatoryErrs, fmt.Sprintf("%s: %v", entry.Label, r.err))
	}

	if len(mandatoryErrs) == 0 {
		return nil
	}
	if len(mandatoryErrs) == 1 {
		return fmt.Errorf("event sink write failed: %s", mandatoryErrs[0])
	}
	return fmt.Errorf("event sink writes failed: %s", strings.Join(mandatoryErrs, "; "))
}

// Close closes all sinks. Returns the first error encountered.
func (f *FanoutEventSink) Close() error {
	var firstErr error
	for _, entry := range f.sinks {
		if err := entry.Sink.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Verify FanoutEventSink implements EventSink.
var _ EventSink = (*FanoutEventSink)(nil)
