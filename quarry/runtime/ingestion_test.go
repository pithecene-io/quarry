package runtime

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/vmihailenco/msgpack/v5"

	"github.com/justapithecus/quarry/log"
	"github.com/justapithecus/quarry/policy"
	"github.com/justapithecus/quarry/types"
)

// encodeFrame encodes a payload with length prefix
func encodeFrame(payload []byte) []byte {
	buf := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(payload)))
	copy(buf[4:], payload)
	return buf
}

// encodeEventFrame creates a framed event envelope
func encodeEventFrame(envelope *types.EventEnvelope) []byte {
	payload, _ := msgpack.Marshal(envelope)
	return encodeFrame(payload)
}

func TestIngestionEngine_EnvelopeValidation_ContractVersionMismatch(t *testing.T) {
	runMeta := &types.RunMeta{
		RunID:   "run-123",
		Attempt: 1,
	}

	envelope := &types.EventEnvelope{
		ContractVersion: "0.99.0", // Wrong version
		EventID:         "evt-1",
		RunID:           "run-123",
		Seq:             1,
		Type:            types.EventTypeLog,
		Ts:              "2024-01-01T00:00:00Z",
		Payload:         map[string]any{"level": "info", "message": "test"},
		Attempt:         1,
	}

	data := encodeEventFrame(envelope)
	reader := bytes.NewReader(data)

	logger := log.NewLogger(runMeta)
	engine := NewIngestionEngine(reader, policy.NewNoopPolicy(), NewArtifactManager(), logger, runMeta)

	err := engine.Run(context.Background())
	if err == nil {
		t.Fatal("expected error for contract version mismatch")
	}
	if IsPolicyError(err) {
		t.Error("contract version mismatch should be stream error, not policy error")
	}
}

func TestIngestionEngine_EnvelopeValidation_RunIDMismatch(t *testing.T) {
	runMeta := &types.RunMeta{
		RunID:   "run-123",
		Attempt: 1,
	}

	envelope := &types.EventEnvelope{
		ContractVersion: types.ContractVersion,
		EventID:         "evt-1",
		RunID:           "run-WRONG", // Wrong run_id
		Seq:             1,
		Type:            types.EventTypeLog,
		Ts:              "2024-01-01T00:00:00Z",
		Payload:         map[string]any{"level": "info", "message": "test"},
		Attempt:         1,
	}

	data := encodeEventFrame(envelope)
	reader := bytes.NewReader(data)

	logger := log.NewLogger(runMeta)
	engine := NewIngestionEngine(reader, policy.NewNoopPolicy(), NewArtifactManager(), logger, runMeta)

	err := engine.Run(context.Background())
	if err == nil {
		t.Fatal("expected error for run_id mismatch")
	}
	if IsPolicyError(err) {
		t.Error("run_id mismatch should be stream error, not policy error")
	}
}

func TestIngestionEngine_EnvelopeValidation_AttemptMismatch(t *testing.T) {
	runMeta := &types.RunMeta{
		RunID:   "run-123",
		Attempt: 1,
	}

	envelope := &types.EventEnvelope{
		ContractVersion: types.ContractVersion,
		EventID:         "evt-1",
		RunID:           "run-123",
		Seq:             1,
		Type:            types.EventTypeLog,
		Ts:              "2024-01-01T00:00:00Z",
		Payload:         map[string]any{"level": "info", "message": "test"},
		Attempt:         2, // Wrong attempt
	}

	data := encodeEventFrame(envelope)
	reader := bytes.NewReader(data)

	logger := log.NewLogger(runMeta)
	engine := NewIngestionEngine(reader, policy.NewNoopPolicy(), NewArtifactManager(), logger, runMeta)

	err := engine.Run(context.Background())
	if err == nil {
		t.Fatal("expected error for attempt mismatch")
	}
	if IsPolicyError(err) {
		t.Error("attempt mismatch should be stream error, not policy error")
	}
}

func TestIngestionEngine_SequenceViolation(t *testing.T) {
	runMeta := &types.RunMeta{
		RunID:   "run-123",
		Attempt: 1,
	}

	// Create two events with wrong sequence
	envelope1 := &types.EventEnvelope{
		ContractVersion: types.ContractVersion,
		EventID:         "evt-1",
		RunID:           "run-123",
		Seq:             1,
		Type:            types.EventTypeLog,
		Ts:              "2024-01-01T00:00:00Z",
		Payload:         map[string]any{"level": "info", "message": "test"},
		Attempt:         1,
	}
	envelope2 := &types.EventEnvelope{
		ContractVersion: types.ContractVersion,
		EventID:         "evt-2",
		RunID:           "run-123",
		Seq:             3, // Wrong: should be 2
		Type:            types.EventTypeLog,
		Ts:              "2024-01-01T00:00:01Z",
		Payload:         map[string]any{"level": "info", "message": "test2"},
		Attempt:         1,
	}

	var buf bytes.Buffer
	buf.Write(encodeEventFrame(envelope1))
	buf.Write(encodeEventFrame(envelope2))

	logger := log.NewLogger(runMeta)
	engine := NewIngestionEngine(&buf, policy.NewNoopPolicy(), NewArtifactManager(), logger, runMeta)

	err := engine.Run(context.Background())
	if err == nil {
		t.Fatal("expected error for sequence violation")
	}
	if IsPolicyError(err) {
		t.Error("sequence violation should be stream error, not policy error")
	}
}

func TestIngestionEngine_FrameDecodeError(t *testing.T) {
	runMeta := &types.RunMeta{
		RunID:   "run-123",
		Attempt: 1,
	}

	// Create invalid msgpack data
	invalidPayload := []byte{0xFF, 0xFF, 0xFF} // Invalid msgpack
	data := encodeFrame(invalidPayload)

	logger := log.NewLogger(runMeta)
	engine := NewIngestionEngine(bytes.NewReader(data), policy.NewNoopPolicy(), NewArtifactManager(), logger, runMeta)

	err := engine.Run(context.Background())
	if err == nil {
		t.Fatal("expected error for frame decode error")
	}
	if IsPolicyError(err) {
		t.Error("frame decode error should be stream error, not policy error")
	}
}

// failingPolicy is a policy that fails on certain events
type failingPolicy struct {
	*policy.NoopPolicy
	failOnType types.EventType
}

func (p *failingPolicy) IngestEvent(ctx context.Context, envelope *types.EventEnvelope) error {
	if envelope.Type == p.failOnType {
		return errors.New("policy failure")
	}
	return p.NoopPolicy.IngestEvent(ctx, envelope)
}

// trackingPolicy tracks flush calls for testing
type trackingPolicy struct {
	*policy.NoopPolicy
	FlushCalled bool
}

func newTrackingPolicy() *trackingPolicy {
	return &trackingPolicy{
		NoopPolicy: policy.NewNoopPolicy(),
	}
}

func (p *trackingPolicy) Flush(ctx context.Context) error {
	p.FlushCalled = true
	return p.NoopPolicy.Flush(ctx)
}

// TestIngestionEngine_PolicyFlushAfterError verifies that policy tracking works
func TestIngestionEngine_PolicyFlushAfterError(t *testing.T) {
	// This test verifies that the trackingPolicy works correctly
	// The actual flush-on-error behavior is tested at the RunOrchestrator level
	pol := newTrackingPolicy()

	// Flush should set the flag
	if err := pol.Flush(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !pol.FlushCalled {
		t.Error("expected FlushCalled to be true")
	}
}

func TestIngestionEngine_PolicyFailure(t *testing.T) {
	runMeta := &types.RunMeta{
		RunID:   "run-123",
		Attempt: 1,
	}

	envelope := &types.EventEnvelope{
		ContractVersion: types.ContractVersion,
		EventID:         "evt-1",
		RunID:           "run-123",
		Seq:             1,
		Type:            types.EventTypeItem,
		Ts:              "2024-01-01T00:00:00Z",
		Payload:         map[string]any{"item_type": "test", "data": map[string]any{}},
		Attempt:         1,
	}

	data := encodeEventFrame(envelope)
	reader := bytes.NewReader(data)

	failPolicy := &failingPolicy{
		NoopPolicy: policy.NewNoopPolicy(),
		failOnType: types.EventTypeItem,
	}

	logger := log.NewLogger(runMeta)
	engine := NewIngestionEngine(reader, failPolicy, NewArtifactManager(), logger, runMeta)

	err := engine.Run(context.Background())
	if err == nil {
		t.Fatal("expected error for policy failure")
	}
	if !IsPolicyError(err) {
		t.Error("policy failure should be classified as policy error")
	}
}

func TestIngestionEngine_ValidEvent(t *testing.T) {
	runMeta := &types.RunMeta{
		RunID:   "run-123",
		Attempt: 1,
	}

	envelope := &types.EventEnvelope{
		ContractVersion: types.ContractVersion,
		EventID:         "evt-1",
		RunID:           "run-123",
		Seq:             1,
		Type:            types.EventTypeRunComplete,
		Ts:              "2024-01-01T00:00:00Z",
		Payload:         map[string]any{},
		Attempt:         1,
	}

	data := encodeEventFrame(envelope)
	reader := bytes.NewReader(data)

	logger := log.NewLogger(runMeta)
	engine := NewIngestionEngine(reader, policy.NewNoopPolicy(), NewArtifactManager(), logger, runMeta)

	err := engine.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	terminal, hasTerminal := engine.GetTerminalEvent()
	if !hasTerminal {
		t.Error("expected terminal event to be recorded")
	}
	if terminal.Type != types.EventTypeRunComplete {
		t.Errorf("expected run_complete, got %s", terminal.Type)
	}
}
