package lode

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"sync"

	"github.com/justapithecus/lode/lode"
	"github.com/justapithecus/quarry/types"
)

// checksumEnabled controls whether MD5 checksums are computed for chunks.
// Default is false per CONTRACT_LODE.md (checksum is optional).
const checksumEnabled = false

// LodeClient is a real Lode-backed implementation of Client.
// Uses Lode's HiveLayout with partition keys: source/category/day/run_id/event_type.
type LodeClient struct {
	dataset lode.Dataset
	config  Config

	mu      sync.Mutex       // guards chunkOffsets
	offsets map[string]int64 // cumulative offset per artifact across batches
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
		dataset: ds,
		config:  cfg,
		offsets: make(map[string]int64),
	}, nil
}

// WriteEvents writes a batch of events to Lode.
// Artifact events (type=artifact) are converted to ArtifactCommitRecord format.
// Other events use EventRecord format.
// Events are partitioned by event_type (included in each record).
func (c *LodeClient) WriteEvents(ctx context.Context, dataset, runID string, events []*types.EventEnvelope) error {
	if len(events) == 0 {
		return nil
	}

	records := make([]any, 0, len(events))
	for _, e := range events {
		var record map[string]any
		if e.Type == types.EventTypeArtifact {
			record = toArtifactCommitRecordMap(e, c.config)
		} else {
			record = toEventRecordMap(e, c.config)
		}
		records = append(records, record)
	}

	_, err := c.dataset.Write(ctx, records, lode.Metadata{})
	return err
}

// WriteChunks writes a batch of artifact chunks to Lode.
// Chunks are written to event_type=artifact partition with record_kind=artifact_chunk.
// Offset is computed cumulatively across batches per artifact.
func (c *LodeClient) WriteChunks(ctx context.Context, dataset, runID string, chunks []*types.ArtifactChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	records := make([]any, 0, len(chunks))

	for _, chunk := range chunks {
		offset := c.offsets[chunk.ArtifactID]
		record := toChunkRecordMap(chunk, offset, c.config)

		// Add checksum if enabled
		if checksumEnabled {
			checksum := computeMD5(chunk.Data)
			record["checksum"] = checksum
			record["checksum_algo"] = "md5"
		}

		records = append(records, record)
		c.offsets[chunk.ArtifactID] = offset + int64(len(chunk.Data))
	}

	_, err := c.dataset.Write(ctx, records, lode.Metadata{})
	return err
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

// Verify LodeClient implements Client.
var _ Client = (*LodeClient)(nil)
