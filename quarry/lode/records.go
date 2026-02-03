package lode

import "github.com/justapithecus/quarry/types"

// RecordKind discriminator values per CONTRACT_LODE.md.
const (
	RecordKindEvent         = "event"
	RecordKindArtifactEvent = "artifact_event"
	RecordKindArtifactChunk = "artifact_chunk"
)

// EventRecord is the storage format for non-artifact events.
// Includes record_kind discriminator and partition keys per CONTRACT_LODE.md.
type EventRecord struct {
	// Record discriminator
	RecordKind string `json:"record_kind"`

	// Event envelope fields
	ContractVersion string         `json:"contract_version"`
	EventID         string         `json:"event_id"`
	RunID           string         `json:"run_id"`
	Seq             int64          `json:"seq"`
	Type            string         `json:"type"`
	Ts              string         `json:"ts"`
	Payload         map[string]any `json:"payload"`
	JobID           *string        `json:"job_id,omitempty"`
	ParentRunID     *string        `json:"parent_run_id,omitempty"`
	Attempt         int            `json:"attempt"`

	// Partition keys (used by Lode HiveLayout)
	Source   string `json:"source"`
	Category string `json:"category"`
	Day      string `json:"day"`
}

// ArtifactCommitRecord is the storage format for artifact commit events.
// Marks an artifact as complete per CONTRACT_LODE.md.
type ArtifactCommitRecord struct {
	// Record discriminator
	RecordKind string `json:"record_kind"`

	// Artifact commit fields
	ArtifactID  string `json:"artifact_id"`
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`

	// Event metadata (from envelope)
	ContractVersion string  `json:"contract_version"`
	EventID         string  `json:"event_id"`
	RunID           string  `json:"run_id"`
	Seq             int64   `json:"seq"`
	Ts              string  `json:"ts"`
	JobID           *string `json:"job_id,omitempty"`
	ParentRunID     *string `json:"parent_run_id,omitempty"`
	Attempt         int     `json:"attempt"`

	// Partition keys
	Source   string `json:"source"`
	Category string `json:"category"`
	Day      string `json:"day"`
}

// ArtifactChunkRecord is the storage format for artifact binary chunks.
// Written to event_type=artifact partition per CONTRACT_LODE.md.
type ArtifactChunkRecord struct {
	// Record discriminator
	RecordKind string `json:"record_kind"`

	// Chunk fields
	ArtifactID string `json:"artifact_id"`
	Seq        int64  `json:"seq"`
	IsLast     bool   `json:"is_last"`
	Offset     int64  `json:"offset"`
	Length     int64  `json:"length"`
	Data       []byte `json:"data"` // base64 encoded in JSON

	// Optional checksum per CONTRACT_LODE.md
	Checksum     *string `json:"checksum,omitempty"`
	ChecksumAlgo *string `json:"checksum_algo,omitempty"`

	// Partition keys
	Source   string `json:"source"`
	Category string `json:"category"`
	Day      string `json:"day"`
	RunID    string `json:"run_id"`
}

// toEventRecordMap converts an EventEnvelope to a map for Lode storage.
// Lode HiveLayout requires records as map[string]any.
func toEventRecordMap(e *types.EventEnvelope, cfg Config) map[string]any {
	m := map[string]any{
		"record_kind":      RecordKindEvent,
		"contract_version": e.ContractVersion,
		"event_id":         e.EventID,
		"run_id":           e.RunID,
		"seq":              e.Seq,
		"type":             string(e.Type),
		"event_type":       string(e.Type), // partition key
		"ts":               e.Ts,
		"payload":          e.Payload,
		"attempt":          e.Attempt,
		"source":           cfg.Source,
		"category":         cfg.Category,
		"day":              cfg.Day,
	}
	if e.JobID != nil {
		m["job_id"] = *e.JobID
	}
	if e.ParentRunID != nil {
		m["parent_run_id"] = *e.ParentRunID
	}
	return m
}

// toArtifactCommitRecordMap converts an artifact EventEnvelope to a map for storage.
func toArtifactCommitRecordMap(e *types.EventEnvelope, cfg Config) map[string]any {
	// Extract artifact payload fields
	var artifactID, name, contentType string
	var sizeBytes int64

	if payload := e.Payload; payload != nil {
		if v, ok := payload["artifact_id"].(string); ok {
			artifactID = v
		}
		if v, ok := payload["name"].(string); ok {
			name = v
		}
		if v, ok := payload["content_type"].(string); ok {
			contentType = v
		}
		if v, ok := payload["size_bytes"].(float64); ok {
			sizeBytes = int64(v)
		}
	}

	m := map[string]any{
		"record_kind":      RecordKindArtifactEvent,
		"artifact_id":      artifactID,
		"name":             name,
		"content_type":     contentType,
		"size_bytes":       sizeBytes,
		"contract_version": e.ContractVersion,
		"event_id":         e.EventID,
		"run_id":           e.RunID,
		"seq":              e.Seq,
		"event_type":       string(e.Type), // partition key
		"ts":               e.Ts,
		"attempt":          e.Attempt,
		"source":           cfg.Source,
		"category":         cfg.Category,
		"day":              cfg.Day,
	}
	if e.JobID != nil {
		m["job_id"] = *e.JobID
	}
	if e.ParentRunID != nil {
		m["parent_run_id"] = *e.ParentRunID
	}
	return m
}

// toChunkRecordMap converts an ArtifactChunk to a map for storage.
func toChunkRecordMap(chunk *types.ArtifactChunk, offset int64, cfg Config) map[string]any {
	return map[string]any{
		"record_kind": RecordKindArtifactChunk,
		"artifact_id": chunk.ArtifactID,
		"seq":         chunk.Seq,
		"is_last":     chunk.IsLast,
		"offset":      offset,
		"length":      int64(len(chunk.Data)),
		"data":        chunk.Data,
		"event_type":  "artifact", // partition key for chunks
		"source":      cfg.Source,
		"category":    cfg.Category,
		"day":         cfg.Day,
		"run_id":      cfg.RunID,
	}
}

// Legacy struct-based conversion for testing (not used with Lode)
func toEventRecord(e *types.EventEnvelope, cfg Config) EventRecord {
	return EventRecord{
		RecordKind:      RecordKindEvent,
		ContractVersion: e.ContractVersion,
		EventID:         e.EventID,
		RunID:           e.RunID,
		Seq:             e.Seq,
		Type:            string(e.Type),
		Ts:              e.Ts,
		Payload:         e.Payload,
		JobID:           e.JobID,
		ParentRunID:     e.ParentRunID,
		Attempt:         e.Attempt,
		Source:          cfg.Source,
		Category:        cfg.Category,
		Day:             cfg.Day,
	}
}

func toArtifactCommitRecord(e *types.EventEnvelope, cfg Config) ArtifactCommitRecord {
	var artifactID, name, contentType string
	var sizeBytes int64

	if payload := e.Payload; payload != nil {
		if v, ok := payload["artifact_id"].(string); ok {
			artifactID = v
		}
		if v, ok := payload["name"].(string); ok {
			name = v
		}
		if v, ok := payload["content_type"].(string); ok {
			contentType = v
		}
		if v, ok := payload["size_bytes"].(float64); ok {
			sizeBytes = int64(v)
		}
	}

	return ArtifactCommitRecord{
		RecordKind:      RecordKindArtifactEvent,
		ArtifactID:      artifactID,
		Name:            name,
		ContentType:     contentType,
		SizeBytes:       sizeBytes,
		ContractVersion: e.ContractVersion,
		EventID:         e.EventID,
		RunID:           e.RunID,
		Seq:             e.Seq,
		Ts:              e.Ts,
		JobID:           e.JobID,
		ParentRunID:     e.ParentRunID,
		Attempt:         e.Attempt,
		Source:          cfg.Source,
		Category:        cfg.Category,
		Day:             cfg.Day,
	}
}

func toChunkRecord(chunk *types.ArtifactChunk, offset int64, cfg Config) ArtifactChunkRecord {
	return ArtifactChunkRecord{
		RecordKind: RecordKindArtifactChunk,
		ArtifactID: chunk.ArtifactID,
		Seq:        chunk.Seq,
		IsLast:     chunk.IsLast,
		Offset:     offset,
		Length:     int64(len(chunk.Data)),
		Data:       chunk.Data,
		Source:     cfg.Source,
		Category:   cfg.Category,
		Day:        cfg.Day,
		RunID:      cfg.RunID,
	}
}
