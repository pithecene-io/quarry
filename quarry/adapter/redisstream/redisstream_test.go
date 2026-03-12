package redisstream

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/pithecene-io/quarry/iox"
	"github.com/pithecene-io/quarry/types"
)

func testEvents() []*types.EventEnvelope {
	return []*types.EventEnvelope{
		{
			ContractVersion: "0.12.2",
			EventID:         "evt-001",
			RunID:           "run-001",
			Seq:             1,
			Type:            types.EventTypeItem,
			Ts:              "2026-03-12T12:00:00Z",
			Payload:         map[string]any{"item_type": "page", "data": map[string]any{"url": "https://example.com"}},
			Attempt:         1,
		},
		{
			ContractVersion: "0.12.2",
			EventID:         "evt-002",
			RunID:           "run-001",
			Seq:             2,
			Type:            types.EventTypeLog,
			Ts:              "2026-03-12T12:00:01Z",
			Payload:         map[string]any{"level": "info", "message": "page loaded"},
			Attempt:         1,
		},
	}
}

// streamValues converts a miniredis StreamEntry's Values (alternating k/v slice)
// into a map for easier assertions.
func streamValues(entry miniredis.StreamEntry) map[string]string {
	m := make(map[string]string, len(entry.Values)/2)
	for i := 0; i+1 < len(entry.Values); i += 2 {
		m[entry.Values[i]] = entry.Values[i+1]
	}
	return m
}

func TestWriteEvents_Success(t *testing.T) {
	mr := miniredis.RunT(t)

	s, err := New(Config{
		URL:      "redis://" + mr.Addr(),
		Source:   "test-source",
		Category: "default",
		Retries:  0,
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer iox.DiscardClose(s)

	events := testEvents()
	if err := s.WriteEvents(t.Context(), events); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Verify stream entries via miniredis
	stream, err := mr.Stream(DefaultStreamKey)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if len(stream) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(stream))
	}

	// Check first entry fields
	vals := streamValues(stream[0])
	if vals["run_id"] != "run-001" {
		t.Errorf("expected run_id=run-001, got %s", vals["run_id"])
	}
	if vals["event_type"] != "item" {
		t.Errorf("expected event_type=item, got %s", vals["event_type"])
	}
	if vals["seq"] != "1" {
		t.Errorf("expected seq=1, got %s", vals["seq"])
	}
	if vals["source"] != "test-source" {
		t.Errorf("expected source=test-source, got %s", vals["source"])
	}
	if vals["category"] != "default" {
		t.Errorf("expected category=default, got %s", vals["category"])
	}

	// Verify payload is valid JSON
	var payload map[string]any
	if err := json.Unmarshal([]byte(vals["payload"]), &payload); err != nil {
		t.Fatalf("payload not valid JSON: %v", err)
	}
	if payload["item_type"] != "page" {
		t.Errorf("expected item_type=page in payload, got %v", payload["item_type"])
	}

	// Check second entry
	vals2 := streamValues(stream[1])
	if vals2["event_type"] != "log" {
		t.Errorf("expected event_type=log, got %s", vals2["event_type"])
	}
	if vals2["seq"] != "2" {
		t.Errorf("expected seq=2, got %s", vals2["seq"])
	}
}

func TestWriteEvents_EmptyBatch(t *testing.T) {
	mr := miniredis.RunT(t)

	s, err := New(Config{URL: "redis://" + mr.Addr(), Retries: 0})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer iox.DiscardClose(s)

	// Empty batch should be a no-op
	if err := s.WriteEvents(t.Context(), nil); err != nil {
		t.Fatalf("write nil: %v", err)
	}
	if err := s.WriteEvents(t.Context(), []*types.EventEnvelope{}); err != nil {
		t.Fatalf("write empty: %v", err)
	}
}

func TestWriteEvents_CustomStreamKey(t *testing.T) {
	mr := miniredis.RunT(t)

	customKey := "myapp:events"
	s, err := New(Config{
		URL:       "redis://" + mr.Addr(),
		StreamKey: customKey,
		Retries:   0,
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer iox.DiscardClose(s)

	if err := s.WriteEvents(t.Context(), testEvents()); err != nil {
		t.Fatalf("write: %v", err)
	}

	stream, err := mr.Stream(customKey)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if len(stream) != 2 {
		t.Fatalf("expected 2 entries in custom stream, got %d", len(stream))
	}
}

func TestWriteEvents_ExhaustsRetries(t *testing.T) {
	s, err := New(Config{
		URL:     "redis://127.0.0.1:1",
		Retries: 1,
		Timeout: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer iox.DiscardClose(s)

	err = s.WriteEvents(t.Context(), testEvents())
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
}

func TestWriteEvents_ContextCanceled(t *testing.T) {
	s, err := New(Config{
		URL:     "redis://127.0.0.1:1",
		Retries: 5,
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer iox.DiscardClose(s)

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	err = s.WriteEvents(ctx, testEvents())
	if err == nil {
		t.Fatal("expected error on canceled context")
	}
}

func TestNew_RequiresURL(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestNew_InvalidURL(t *testing.T) {
	_, err := New(Config{URL: "not-a-redis-url"})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestNew_RejectsNegativeRetries(t *testing.T) {
	_, err := New(Config{URL: "redis://localhost:6379", Retries: -1})
	if err == nil {
		t.Fatal("expected error for negative retries")
	}
}

func TestNew_DefaultsApplied(t *testing.T) {
	mr := miniredis.RunT(t)

	s, err := New(Config{URL: "redis://" + mr.Addr()})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer iox.DiscardClose(s)

	if s.config.StreamKey != DefaultStreamKey {
		t.Errorf("expected default stream key %q, got %q", DefaultStreamKey, s.config.StreamKey)
	}
	if s.config.MaxLen != DefaultMaxLen {
		t.Errorf("expected default max len %d, got %d", DefaultMaxLen, s.config.MaxLen)
	}
	if s.config.TTL != DefaultTTL {
		t.Errorf("expected default TTL %v, got %v", DefaultTTL, s.config.TTL)
	}
	if s.config.Timeout != DefaultTimeout {
		t.Errorf("expected default timeout %v, got %v", DefaultTimeout, s.config.Timeout)
	}
}

func TestClose_ClosesConnection(t *testing.T) {
	mr := miniredis.RunT(t)

	s, err := New(Config{URL: "redis://" + mr.Addr()})
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Write after close should fail
	err = s.WriteEvents(t.Context(), testEvents())
	if err == nil {
		t.Fatal("expected error after close")
	}
}

func TestWriteEvents_AppliesTTL(t *testing.T) {
	mr := miniredis.RunT(t)

	s, err := New(Config{
		URL:     "redis://" + mr.Addr(),
		TTL:     1 * time.Hour,
		Retries: 0,
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer iox.DiscardClose(s)

	if err := s.WriteEvents(t.Context(), testEvents()); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Verify TTL was set on the stream key
	ttl := mr.TTL(DefaultStreamKey)
	if ttl <= 0 {
		t.Errorf("expected positive TTL on stream key, got %v", ttl)
	}
}

func TestWriteEvents_TTLDisabled(t *testing.T) {
	mr := miniredis.RunT(t)

	s, err := New(Config{
		URL:     "redis://" + mr.Addr(),
		TTL:     -1, // disabled
		Retries: 0,
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer iox.DiscardClose(s)

	if err := s.WriteEvents(t.Context(), testEvents()); err != nil {
		t.Fatalf("write: %v", err)
	}

	// No TTL should be set
	ttl := mr.TTL(DefaultStreamKey)
	if ttl > 0 {
		t.Errorf("expected no TTL when disabled, got %v", ttl)
	}
}

func TestWriteEvents_MaxLenDisabled(t *testing.T) {
	mr := miniredis.RunT(t)

	s, err := New(Config{
		URL:     "redis://" + mr.Addr(),
		MaxLen:  -1, // disabled
		TTL:     -1,
		Retries: 0,
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer iox.DiscardClose(s)

	if err := s.WriteEvents(t.Context(), testEvents()); err != nil {
		t.Fatalf("write: %v", err)
	}

	// All entries should be present (no trimming)
	stream, err := mr.Stream(DefaultStreamKey)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if len(stream) != 2 {
		t.Fatalf("expected 2 entries (no trimming), got %d", len(stream))
	}
}
