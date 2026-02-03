// Package ipc implements IPC framing per CONTRACT_IPC.md.
package ipc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/vmihailenco/msgpack/v5"

	"github.com/justapithecus/quarry/types"
)

// Frame size constants per CONTRACT_IPC.md.
const (
	// MaxFrameSize is the maximum frame size (16 MiB), including length prefix.
	MaxFrameSize = 16 * 1024 * 1024
	// MaxPayloadSize is the maximum payload size (MaxFrameSize - 4 bytes).
	MaxPayloadSize = MaxFrameSize - LengthPrefixSize
	// MaxChunkSize is the maximum artifact chunk size (8 MiB raw bytes).
	MaxChunkSize = 8 * 1024 * 1024
	// LengthPrefixSize is the size of the length prefix in bytes.
	LengthPrefixSize = 4
)

// ArtifactChunkType is the type discriminant for artifact chunk frames.
const ArtifactChunkType = "artifact_chunk"

// RunResultType is the type discriminant for run result control frames.
const RunResultType = "run_result"

// FrameErrorKind classifies frame decoding errors.
type FrameErrorKind int

const (
	// FrameErrorPartial indicates a truncated or incomplete frame.
	FrameErrorPartial FrameErrorKind = iota
	// FrameErrorTooLarge indicates a frame exceeding MaxFrameSize.
	FrameErrorTooLarge
	// FrameErrorDecode indicates a msgpack decoding error.
	FrameErrorDecode
)

// FrameError represents a frame decoding error.
type FrameError struct {
	Kind FrameErrorKind
	Msg  string
	Err  error
}

func (e *FrameError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Err)
	}
	return e.Msg
}

func (e *FrameError) Unwrap() error {
	return e.Err
}

// IsFatal returns true if this error is fatal (terminate run).
// Per CONTRACT_IPC.md, partial and oversized frames are fatal.
func (e *FrameError) IsFatal() bool {
	return e.Kind == FrameErrorPartial || e.Kind == FrameErrorTooLarge
}

// IsFatalFrameError returns true if the error is a fatal frame error.
func IsFatalFrameError(err error) bool {
	var frameErr *FrameError
	if errors.As(err, &frameErr) {
		return frameErr.IsFatal()
	}
	return false
}

// FrameDecoder decodes length-prefixed msgpack frames from a stream.
type FrameDecoder struct {
	reader io.Reader
}

// NewFrameDecoder creates a new frame decoder.
func NewFrameDecoder(r io.Reader) *FrameDecoder {
	return &FrameDecoder{reader: r}
}

// ReadFrame reads a single frame from the stream.
// Returns the raw payload bytes (msgpack-encoded).
//
// Errors:
//   - io.EOF: stream ended cleanly (no more frames)
//   - *FrameError with Kind=FrameErrorPartial: incomplete frame (fatal)
//   - *FrameError with Kind=FrameErrorTooLarge: frame exceeds limit (fatal)
func (d *FrameDecoder) ReadFrame() ([]byte, error) {
	// Read 4-byte big-endian length prefix
	var lengthBuf [LengthPrefixSize]byte
	_, err := io.ReadFull(d.reader, lengthBuf[:])
	if err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		// Partial read of length prefix
		return nil, &FrameError{
			Kind: FrameErrorPartial,
			Msg:  "failed to read length prefix",
			Err:  err,
		}
	}

	payloadSize := binary.BigEndian.Uint32(lengthBuf[:])

	// Validate frame size per CONTRACT_IPC.md
	if payloadSize > MaxPayloadSize {
		return nil, &FrameError{
			Kind: FrameErrorTooLarge,
			Msg:  fmt.Sprintf("payload size %d exceeds maximum %d", payloadSize, MaxPayloadSize),
		}
	}

	// Read payload
	payload := make([]byte, payloadSize)
	_, err = io.ReadFull(d.reader, payload)
	if err != nil {
		return nil, &FrameError{
			Kind: FrameErrorPartial,
			Msg:  "failed to read payload",
			Err:  err,
		}
	}

	return payload, nil
}

// frameTypeProbe is used to peek at the type field without full decode.
type frameTypeProbe struct {
	Type string `msgpack:"type"`
}

// DecodeFrame decodes a payload and returns either an EventEnvelope or ArtifactChunkFrame.
// Discriminates based on the type field: "artifact_chunk" vs event types.
func DecodeFrame(payload []byte) (any, error) {
	// Peek at type field
	var probe frameTypeProbe
	if err := msgpack.Unmarshal(payload, &probe); err != nil {
		return nil, &FrameError{
			Kind: FrameErrorDecode,
			Msg:  "failed to decode frame type",
			Err:  err,
		}
	}

	switch probe.Type {
	case ArtifactChunkType:
		return DecodeArtifactChunk(payload)
	case RunResultType:
		return DecodeRunResult(payload)
	default:
		return DecodeEventEnvelope(payload)
	}
}

// DecodeEventEnvelope decodes a payload as an EventEnvelope.
func DecodeEventEnvelope(payload []byte) (*types.EventEnvelope, error) {
	var envelope types.EventEnvelope
	if err := msgpack.Unmarshal(payload, &envelope); err != nil {
		return nil, &FrameError{
			Kind: FrameErrorDecode,
			Msg:  "failed to decode event envelope",
			Err:  err,
		}
	}
	return &envelope, nil
}

// DecodeArtifactChunk decodes a payload as an ArtifactChunkFrame.
func DecodeArtifactChunk(payload []byte) (*types.ArtifactChunkFrame, error) {
	var chunk types.ArtifactChunkFrame
	if err := msgpack.Unmarshal(payload, &chunk); err != nil {
		return nil, &FrameError{
			Kind: FrameErrorDecode,
			Msg:  "failed to decode artifact chunk",
			Err:  err,
		}
	}
	return &chunk, nil
}

// DecodeRunResult decodes a payload as a RunResultFrame.
func DecodeRunResult(payload []byte) (*types.RunResultFrame, error) {
	var result types.RunResultFrame
	if err := msgpack.Unmarshal(payload, &result); err != nil {
		return nil, &FrameError{
			Kind: FrameErrorDecode,
			Msg:  "failed to decode run result",
			Err:  err,
		}
	}
	return &result, nil
}
