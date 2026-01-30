//nolint:revive // types is a common Go package naming convention
package types

// ArtifactChunkFrame represents an artifact chunk frame per CONTRACT_IPC.md.
// This is a stream-level construct, not a normal emit event.
// Discriminated from event envelopes by Type == "artifact_chunk".
type ArtifactChunkFrame struct {
	// Type is always "artifact_chunk" for chunk frames.
	Type string `msgpack:"type"`
	// ArtifactID identifies the artifact this chunk belongs to.
	ArtifactID string `msgpack:"artifact_id"`
	// Seq is the sequence number, starts at 1.
	Seq int64 `msgpack:"seq"`
	// IsLast is true if this is the final chunk.
	IsLast bool `msgpack:"is_last"`
	// Data is the raw binary data.
	Data []byte `msgpack:"data"`
}

// ArtifactChunk is an internal representation of a chunk (after decoding).
type ArtifactChunk struct {
	ArtifactID string
	Seq        int64
	IsLast     bool
	Data       []byte
}

// ArtifactAccumulator tracks chunks for a single artifact.
type ArtifactAccumulator struct {
	// ArtifactID is the artifact identifier.
	ArtifactID string
	// Chunks holds the accumulated chunks in order.
	Chunks []*ArtifactChunk
	// TotalBytes is the sum of all chunk data lengths.
	TotalBytes int64
	// Committed is true if the artifact event has been received and validated.
	Committed bool
	// NextSeq is the expected next sequence number.
	NextSeq int64
	// Complete is true if is_last has been seen.
	Complete bool
	// ErrorState is true if the artifact encountered an unrecoverable error (e.g., size mismatch).
	ErrorState bool
}
