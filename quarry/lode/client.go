package lode

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/justapithecus/lode/lode"

	"github.com/justapithecus/quarry/types"
)

// checksumEnabled controls whether MD5 checksums are computed for chunks.
// Default is false per CONTRACT_LODE.md (checksum is optional).
const checksumEnabled = false

// ErrCommitWithoutChunks is returned when an artifact commit is attempted
// without any chunks having been written for that artifact.
var ErrCommitWithoutChunks = fmt.Errorf("artifact commit rejected: no chunks written for artifact")

// ErrMissingArtifactID is returned when an artifact commit event is missing
// the required artifact_id field.
var ErrMissingArtifactID = fmt.Errorf("artifact commit rejected: missing or empty artifact_id")

// LodeClient is a real Lode-backed implementation of Client.
// Uses Lode's HiveLayout with partition keys: source/category/day/run_id/event_type.
type LodeClient struct {
	dataset lode.Dataset
	config  Config

	mu         sync.Mutex          // guards offsets and chunksSeen
	offsets    map[string]int64    // cumulative offset per artifact across batches
	chunksSeen map[string]struct{} // tracks artifacts that have had chunks written
}

// NewLodeClient creates a new Lode client with filesystem storage.
// The root parameter is the base directory for Hive-partitioned storage.
func NewLodeClient(cfg Config, root string) (*LodeClient, error) {
	return NewLodeClientWithFactory(cfg, lode.NewFSFactory(root))
}

// NewLodeClientWithFactory creates a new Lode client with a custom store factory.
// Use lode.NewMemoryFactory() for testing.
func NewLodeClientWithFactory(cfg Config, factory lode.StoreFactory) (*LodeClient, error) {
	ds, err := lode.NewDataset(
		lode.DatasetID(cfg.Dataset),
		factory,
		lode.WithHiveLayout("source", "category", "day", "run_id", "event_type"),
		lode.WithCodec(lode.NewJSONLCodec()),
	)
	if err != nil {
		return nil, err
	}

	return &LodeClient{
		dataset:    ds,
		config:     cfg,
		offsets:    make(map[string]int64),
		chunksSeen: make(map[string]struct{}),
	}, nil
}

// WriteEvents writes a batch of events to Lode.
// Artifact events (type=artifact) are converted to ArtifactCommitRecord format.
// Other events use EventRecord format.
// Events are partitioned by event_type (included in each record).
//
// Enforces "chunks before commit" invariant: artifact commit events are rejected
// if no chunks have been written for that artifact. After a successful commit,
// the artifact's offset and chunksSeen state are reset.
func (c *LodeClient) WriteEvents(ctx context.Context, dataset, runID string, events []*types.EventEnvelope) error {
	if len(events) == 0 {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Collect artifact IDs being committed for post-write cleanup
	var committedArtifacts []string

	records := make([]any, 0, len(events))
	for _, e := range events {
		var record map[string]any
		if e.Type == types.EventTypeArtifact {
			// Extract artifact_id from payload and validate
			artifactID := extractArtifactID(e.Payload)
			if artifactID == "" {
				return ErrMissingArtifactID
			}
			if _, seen := c.chunksSeen[artifactID]; !seen {
				return fmt.Errorf("%w: %s", ErrCommitWithoutChunks, artifactID)
			}
			committedArtifacts = append(committedArtifacts, artifactID)
			record = toArtifactCommitRecordMap(e, c.config)
		} else {
			record = toEventRecordMap(e, c.config)
		}
		records = append(records, record)
	}

	_, err := c.dataset.Write(ctx, records, lode.Metadata{})
	if err != nil {
		return err
	}

	// Reset state for committed artifacts
	for _, artifactID := range committedArtifacts {
		delete(c.offsets, artifactID)
		delete(c.chunksSeen, artifactID)
	}

	return nil
}

// WriteChunks writes a batch of artifact chunks to Lode.
// Chunks are written to event_type=artifact partition with record_kind=artifact_chunk.
// Offset is computed cumulatively across batches per artifact.
// Marks artifacts as having chunks for the "chunks before commit" invariant.
// State (offsets, chunksSeen) is only updated after successful write.
func (c *LodeClient) WriteChunks(ctx context.Context, dataset, runID string, chunks []*types.ArtifactChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	records := make([]any, 0, len(chunks))

	// Compute offsets locally first (don't modify state yet)
	localOffsets := make(map[string]int64)
	for id, offset := range c.offsets {
		localOffsets[id] = offset
	}

	for _, chunk := range chunks {
		offset := localOffsets[chunk.ArtifactID]
		record := toChunkRecordMap(chunk, offset, c.config)

		// Add checksum if enabled
		if checksumEnabled {
			checksum := computeMD5(chunk.Data)
			record["checksum"] = checksum
			record["checksum_algo"] = "md5"
		}

		records = append(records, record)
		localOffsets[chunk.ArtifactID] = offset + int64(len(chunk.Data))
	}

	// Write to storage
	_, err := c.dataset.Write(ctx, records, lode.Metadata{})
	if err != nil {
		return err
	}

	// Only update state after successful write
	for _, chunk := range chunks {
		c.offsets[chunk.ArtifactID] = localOffsets[chunk.ArtifactID]
		c.chunksSeen[chunk.ArtifactID] = struct{}{}
	}

	return nil
}

// Close releases client resources.
func (c *LodeClient) Close() error {
	// Dataset doesn't require explicit close in current Lode API
	return nil
}

// computeMD5 returns the hex-encoded MD5 digest of data.
func computeMD5(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// extractArtifactID extracts the artifact_id from an event payload.
// Returns empty string if not found or not a string.
func extractArtifactID(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if id, ok := payload["artifact_id"].(string); ok {
		return id
	}
	return ""
}

// Verify LodeClient implements Client.
var _ Client = (*LodeClient)(nil)
