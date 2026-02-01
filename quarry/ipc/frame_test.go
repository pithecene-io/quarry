package ipc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/vmihailenco/msgpack/v5"

	"github.com/justapithecus/quarry/types"
)

// encodeFrame encodes a payload with length prefix (matches Node executor output).
func encodeFrame(payload []byte) []byte {
	buf := make([]byte, LengthPrefixSize+len(payload))
	binary.BigEndian.PutUint32(buf[:LengthPrefixSize], uint32(len(payload)))
	copy(buf[LengthPrefixSize:], payload)
	return buf
}

// encodeEventFrame encodes an event envelope as a framed msgpack payload.
func encodeEventFrame(envelope *types.EventEnvelope) ([]byte, error) {
	payload, err := msgpack.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	return encodeFrame(payload), nil
}

// encodeArtifactChunkFrame encodes an artifact chunk as a framed msgpack payload.
func encodeArtifactChunkFrame(chunk *types.ArtifactChunkFrame) ([]byte, error) {
	payload, err := msgpack.Marshal(chunk)
	if err != nil {
		return nil, err
	}
	return encodeFrame(payload), nil
}

func TestFrameDecoder_SingleEvent(t *testing.T) {
	envelope := &types.EventEnvelope{
		ContractVersion: types.Version,
		EventID:         "evt-001",
		RunID:           "run-001",
		Seq:             1,
		Type:            types.EventTypeItem,
		Ts:              "2024-01-15T10:00:00Z",
		Attempt:         1,
		Payload: map[string]any{
			"item_type": "product",
			"data":      map[string]any{"name": "test"},
		},
	}

	frame, err := encodeEventFrame(envelope)
	if err != nil {
		t.Fatalf("encodeEventFrame failed: %v", err)
	}

	decoder := NewFrameDecoder(bytes.NewReader(frame))
	payload, err := decoder.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}

	decoded, err := DecodeEventEnvelope(payload)
	if err != nil {
		t.Fatalf("DecodeEventEnvelope failed: %v", err)
	}

	if decoded.EventID != envelope.EventID {
		t.Errorf("EventID = %q, want %q", decoded.EventID, envelope.EventID)
	}
	if decoded.Type != envelope.Type {
		t.Errorf("Type = %q, want %q", decoded.Type, envelope.Type)
	}
	if decoded.Seq != envelope.Seq {
		t.Errorf("Seq = %d, want %d", decoded.Seq, envelope.Seq)
	}
}

func TestFrameDecoder_MultipleEvents(t *testing.T) {
	events := []*types.EventEnvelope{
		{
			ContractVersion: types.Version,
			EventID:         "evt-001",
			RunID:           "run-001",
			Seq:             1,
			Type:            types.EventTypeItem,
			Ts:              "2024-01-15T10:00:00Z",
			Attempt:         1,
			Payload:         map[string]any{"item_type": "product"},
		},
		{
			ContractVersion: types.Version,
			EventID:         "evt-002",
			RunID:           "run-001",
			Seq:             2,
			Type:            types.EventTypeLog,
			Ts:              "2024-01-15T10:00:01Z",
			Attempt:         1,
			Payload:         map[string]any{"level": "info", "message": "test"},
		},
		{
			ContractVersion: types.Version,
			EventID:         "evt-003",
			RunID:           "run-001",
			Seq:             3,
			Type:            types.EventTypeRunComplete,
			Ts:              "2024-01-15T10:00:02Z",
			Attempt:         1,
			Payload:         map[string]any{},
		},
	}

	// Encode all events into a single buffer
	var buf bytes.Buffer
	for _, env := range events {
		frame, err := encodeEventFrame(env)
		if err != nil {
			t.Fatalf("encodeEventFrame failed: %v", err)
		}
		buf.Write(frame)
	}

	// Decode all events
	decoder := NewFrameDecoder(&buf)
	decoded := make([]*types.EventEnvelope, 0, len(events))

	for {
		payload, err := decoder.ReadFrame()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("ReadFrame failed: %v", err)
		}

		env, err := DecodeEventEnvelope(payload)
		if err != nil {
			t.Fatalf("DecodeEventEnvelope failed: %v", err)
		}
		decoded = append(decoded, env)
	}

	if len(decoded) != len(events) {
		t.Fatalf("decoded %d events, want %d", len(decoded), len(events))
	}

	for i, env := range decoded {
		if env.EventID != events[i].EventID {
			t.Errorf("events[%d].EventID = %q, want %q", i, env.EventID, events[i].EventID)
		}
		if env.Type != events[i].Type {
			t.Errorf("events[%d].Type = %q, want %q", i, env.Type, events[i].Type)
		}
		if env.Seq != events[i].Seq {
			t.Errorf("events[%d].Seq = %d, want %d", i, env.Seq, events[i].Seq)
		}
	}
}

func TestFrameDecoder_TerminalEvents(t *testing.T) {
	tests := []struct {
		name     string
		envelope *types.EventEnvelope
		terminal bool
	}{
		{
			name: "run_complete is terminal",
			envelope: &types.EventEnvelope{
				ContractVersion: types.Version,
				EventID:         "evt-001",
				RunID:           "run-001",
				Seq:             1,
				Type:            types.EventTypeRunComplete,
				Ts:              "2024-01-15T10:00:00Z",
				Attempt:         1,
				Payload:         map[string]any{},
			},
			terminal: true,
		},
		{
			name: "run_error is terminal",
			envelope: &types.EventEnvelope{
				ContractVersion: types.Version,
				EventID:         "evt-001",
				RunID:           "run-001",
				Seq:             1,
				Type:            types.EventTypeRunError,
				Ts:              "2024-01-15T10:00:00Z",
				Attempt:         1,
				Payload: map[string]any{
					"error_type": "script_error",
					"message":    "test error",
				},
			},
			terminal: true,
		},
		{
			name: "item is not terminal",
			envelope: &types.EventEnvelope{
				ContractVersion: types.Version,
				EventID:         "evt-001",
				RunID:           "run-001",
				Seq:             1,
				Type:            types.EventTypeItem,
				Ts:              "2024-01-15T10:00:00Z",
				Attempt:         1,
				Payload:         map[string]any{"item_type": "product"},
			},
			terminal: false,
		},
		{
			name: "log is not terminal",
			envelope: &types.EventEnvelope{
				ContractVersion: types.Version,
				EventID:         "evt-001",
				RunID:           "run-001",
				Seq:             1,
				Type:            types.EventTypeLog,
				Ts:              "2024-01-15T10:00:00Z",
				Attempt:         1,
				Payload:         map[string]any{"level": "info", "message": "test"},
			},
			terminal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := encodeEventFrame(tt.envelope)
			if err != nil {
				t.Fatalf("encodeEventFrame failed: %v", err)
			}

			decoder := NewFrameDecoder(bytes.NewReader(frame))
			payload, err := decoder.ReadFrame()
			if err != nil {
				t.Fatalf("ReadFrame failed: %v", err)
			}

			decoded, err := DecodeEventEnvelope(payload)
			if err != nil {
				t.Fatalf("DecodeEventEnvelope failed: %v", err)
			}

			if decoded.Type.IsTerminal() != tt.terminal {
				t.Errorf("IsTerminal() = %v, want %v", decoded.Type.IsTerminal(), tt.terminal)
			}
		})
	}
}

func TestFrameDecoder_ArtifactChunk(t *testing.T) {
	chunk := &types.ArtifactChunkFrame{
		Type:       "artifact_chunk",
		ArtifactID: "art-001",
		Seq:        1,
		IsLast:     true,
		Data:       []byte("hello world"),
	}

	frame, err := encodeArtifactChunkFrame(chunk)
	if err != nil {
		t.Fatalf("encodeArtifactChunkFrame failed: %v", err)
	}

	decoder := NewFrameDecoder(bytes.NewReader(frame))
	payload, err := decoder.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}

	// Use DecodeFrame to discriminate
	result, err := DecodeFrame(payload)
	if err != nil {
		t.Fatalf("DecodeFrame failed: %v", err)
	}

	decoded, ok := result.(*types.ArtifactChunkFrame)
	if !ok {
		t.Fatalf("DecodeFrame returned %T, want *types.ArtifactChunkFrame", result)
	}

	if decoded.ArtifactID != chunk.ArtifactID {
		t.Errorf("ArtifactID = %q, want %q", decoded.ArtifactID, chunk.ArtifactID)
	}
	if decoded.Seq != chunk.Seq {
		t.Errorf("Seq = %d, want %d", decoded.Seq, chunk.Seq)
	}
	if decoded.IsLast != chunk.IsLast {
		t.Errorf("IsLast = %v, want %v", decoded.IsLast, chunk.IsLast)
	}
	if !bytes.Equal(decoded.Data, chunk.Data) {
		t.Errorf("Data = %q, want %q", decoded.Data, chunk.Data)
	}
}

func TestFrameDecoder_MixedEventsAndChunks(t *testing.T) {
	// Simulate a typical run: item event, artifact event, artifact chunks, run_complete
	var buf bytes.Buffer

	// 1. Item event
	itemEnv := &types.EventEnvelope{
		ContractVersion: types.Version,
		EventID:         "evt-001",
		RunID:           "run-001",
		Seq:             1,
		Type:            types.EventTypeItem,
		Ts:              "2024-01-15T10:00:00Z",
		Attempt:         1,
		Payload:         map[string]any{"item_type": "product"},
	}
	frame, _ := encodeEventFrame(itemEnv)
	buf.Write(frame)

	// 2. Artifact event (metadata)
	artifactEnv := &types.EventEnvelope{
		ContractVersion: types.Version,
		EventID:         "evt-002",
		RunID:           "run-001",
		Seq:             2,
		Type:            types.EventTypeArtifact,
		Ts:              "2024-01-15T10:00:01Z",
		Attempt:         1,
		Payload: map[string]any{
			"artifact_id":  "art-001",
			"name":         "screenshot.png",
			"content_type": "image/png",
			"size_bytes":   1024,
		},
	}
	frame, _ = encodeEventFrame(artifactEnv)
	buf.Write(frame)

	// 3. Artifact chunks
	chunk1 := &types.ArtifactChunkFrame{
		Type:       "artifact_chunk",
		ArtifactID: "art-001",
		Seq:        1,
		IsLast:     false,
		Data:       []byte("chunk1"),
	}
	frame, _ = encodeArtifactChunkFrame(chunk1)
	buf.Write(frame)

	chunk2 := &types.ArtifactChunkFrame{
		Type:       "artifact_chunk",
		ArtifactID: "art-001",
		Seq:        2,
		IsLast:     true,
		Data:       []byte("chunk2"),
	}
	frame, _ = encodeArtifactChunkFrame(chunk2)
	buf.Write(frame)

	// 4. Run complete
	completeEnv := &types.EventEnvelope{
		ContractVersion: types.Version,
		EventID:         "evt-003",
		RunID:           "run-001",
		Seq:             3,
		Type:            types.EventTypeRunComplete,
		Ts:              "2024-01-15T10:00:02Z",
		Attempt:         1,
		Payload:         map[string]any{},
	}
	frame, _ = encodeEventFrame(completeEnv)
	buf.Write(frame)

	// Decode and verify order
	decoder := NewFrameDecoder(&buf)
	var events []*types.EventEnvelope
	var chunks []*types.ArtifactChunkFrame

	for {
		payload, err := decoder.ReadFrame()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("ReadFrame failed: %v", err)
		}

		result, err := DecodeFrame(payload)
		if err != nil {
			t.Fatalf("DecodeFrame failed: %v", err)
		}

		switch v := result.(type) {
		case *types.EventEnvelope:
			events = append(events, v)
		case *types.ArtifactChunkFrame:
			chunks = append(chunks, v)
		default:
			t.Fatalf("unexpected type: %T", v)
		}
	}

	if len(events) != 3 {
		t.Errorf("got %d events, want 3", len(events))
	}
	if len(chunks) != 2 {
		t.Errorf("got %d chunks, want 2", len(chunks))
	}

	// Verify terminal event is last
	if len(events) > 0 && !events[len(events)-1].Type.IsTerminal() {
		t.Error("last event should be terminal")
	}

	// Verify chunk ordering
	if len(chunks) >= 2 {
		if chunks[0].Seq != 1 || chunks[1].Seq != 2 {
			t.Errorf("chunks out of order: seq %d, %d", chunks[0].Seq, chunks[1].Seq)
		}
		if chunks[0].IsLast || !chunks[1].IsLast {
			t.Error("IsLast flags incorrect")
		}
	}
}

func TestFrameDecoder_PartialFrame(t *testing.T) {
	// Create a valid frame but truncate it
	envelope := &types.EventEnvelope{
		ContractVersion: types.Version,
		EventID:         "evt-001",
		RunID:           "run-001",
		Seq:             1,
		Type:            types.EventTypeItem,
		Ts:              "2024-01-15T10:00:00Z",
		Attempt:         1,
		Payload:         map[string]any{},
	}

	frame, _ := encodeEventFrame(envelope)

	// Truncate the frame (keep only length prefix + half payload)
	truncated := frame[:LengthPrefixSize+len(frame[LengthPrefixSize:])/2]

	decoder := NewFrameDecoder(bytes.NewReader(truncated))
	_, err := decoder.ReadFrame()

	if err == nil {
		t.Fatal("expected error for truncated frame")
	}

	if !IsFatalFrameError(err) {
		t.Errorf("expected fatal frame error, got: %v", err)
	}

	var frameErr *FrameError
	if !errors.As(err, &frameErr) {
		t.Fatalf("expected *FrameError, got %T", err)
	}

	if frameErr.Kind != FrameErrorPartial {
		t.Errorf("Kind = %v, want FrameErrorPartial", frameErr.Kind)
	}
}

func TestFrameDecoder_OversizedFrame(t *testing.T) {
	// Create a length prefix claiming a huge payload
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint32(MaxPayloadSize+1))

	decoder := NewFrameDecoder(&buf)
	_, err := decoder.ReadFrame()

	if err == nil {
		t.Fatal("expected error for oversized frame")
	}

	if !IsFatalFrameError(err) {
		t.Errorf("expected fatal frame error, got: %v", err)
	}

	var frameErr *FrameError
	if !errors.As(err, &frameErr) {
		t.Fatalf("expected *FrameError, got %T", err)
	}

	if frameErr.Kind != FrameErrorTooLarge {
		t.Errorf("Kind = %v, want FrameErrorTooLarge", frameErr.Kind)
	}
}

func TestFrameDecoder_EmptyStream(t *testing.T) {
	decoder := NewFrameDecoder(bytes.NewReader(nil))
	_, err := decoder.ReadFrame()

	if err != io.EOF {
		t.Errorf("expected io.EOF, got: %v", err)
	}
}
