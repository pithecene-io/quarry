package runtime

import (
	"testing"

	"github.com/justapithecus/quarry/types"
)

func TestArtifactManager_MaxArtifactSize(t *testing.T) {
	m := NewArtifactManager()

	// Try to commit an artifact that exceeds max size
	err := m.CommitArtifact("oversized", MaxArtifactSize+1)
	if err == nil {
		t.Fatal("expected error for oversized artifact commit")
	}
}

func TestArtifactManager_ChunkExceedsMaxChunkSize(t *testing.T) {
	m := NewArtifactManager()

	// Chunk data exceeds max chunk size (8 MiB)
	oversizedData := make([]byte, 8*1024*1024+1)
	err := m.AddChunk(&types.ArtifactChunk{
		ArtifactID: "test",
		Seq:        1,
		IsLast:     true,
		Data:       oversizedData,
	})
	if err == nil {
		t.Fatal("expected error for chunk exceeding max chunk size")
	}
}

func TestArtifactManager_CommitBeforeChunks_SizeMismatch(t *testing.T) {
	m := NewArtifactManager()

	// Commit arrives before chunks with declared size
	err := m.CommitArtifact("test", 100)
	if err != nil {
		t.Fatalf("unexpected error on commit before chunks: %v", err)
	}

	// Add chunks with different total size
	err = m.AddChunk(&types.ArtifactChunk{
		ArtifactID: "test",
		Seq:        1,
		IsLast:     true,
		Data:       make([]byte, 50), // Different from declared 100
	})
	if err == nil {
		t.Fatal("expected error for size mismatch when is_last arrives")
	}

	// Verify consistent state: pending commit should be cleaned up
	orphans := m.GetOrphanIDs()
	for _, id := range orphans {
		if id == "test" {
			t.Error("artifact with size mismatch should not be reported as orphan")
		}
	}

	// Verify accumulator is in error state
	acc, _ := m.GetArtifact("test")
	if !acc.ErrorState {
		t.Error("accumulator should be in error state after size mismatch")
	}
}

func TestArtifactManager_CommitBeforeChunks_SizeMatch(t *testing.T) {
	m := NewArtifactManager()

	// Commit arrives before chunks with declared size
	err := m.CommitArtifact("test", 100)
	if err != nil {
		t.Fatalf("unexpected error on commit before chunks: %v", err)
	}

	// Add chunks with matching total size
	err = m.AddChunk(&types.ArtifactChunk{
		ArtifactID: "test",
		Seq:        1,
		IsLast:     true,
		Data:       make([]byte, 100), // Matches declared 100
	})
	if err != nil {
		t.Fatalf("unexpected error for matching size: %v", err)
	}

	// Verify artifact is now committed
	if !m.IsCommitted("test") {
		t.Error("artifact should be committed after size reconciliation")
	}
}

func TestArtifactManager_SequenceViolation(t *testing.T) {
	m := NewArtifactManager()

	// Add chunk with seq 1
	err := m.AddChunk(&types.ArtifactChunk{
		ArtifactID: "test",
		Seq:        1,
		IsLast:     false,
		Data:       []byte("chunk1"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Try to add chunk with seq 3 (should be 2)
	err = m.AddChunk(&types.ArtifactChunk{
		ArtifactID: "test",
		Seq:        3,
		IsLast:     true,
		Data:       []byte("chunk3"),
	})
	if err == nil {
		t.Fatal("expected error for sequence violation")
	}
}

func TestArtifactManager_ChunkAfterIsLast(t *testing.T) {
	m := NewArtifactManager()

	// Add final chunk
	err := m.AddChunk(&types.ArtifactChunk{
		ArtifactID: "test",
		Seq:        1,
		IsLast:     true,
		Data:       []byte("final"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Try to add another chunk after is_last
	err = m.AddChunk(&types.ArtifactChunk{
		ArtifactID: "test",
		Seq:        2,
		IsLast:     false,
		Data:       []byte("extra"),
	})
	if err == nil {
		t.Fatal("expected error for chunk after is_last")
	}
}

func TestArtifactManager_OrphanTracking(t *testing.T) {
	m := NewArtifactManager()

	// Add chunks without committing
	_ = m.AddChunk(&types.ArtifactChunk{
		ArtifactID: "orphan1",
		Seq:        1,
		IsLast:     true,
		Data:       []byte("data"),
	})

	_ = m.AddChunk(&types.ArtifactChunk{
		ArtifactID: "orphan2",
		Seq:        1,
		IsLast:     true,
		Data:       []byte("data"),
	})

	// Commit one
	_ = m.CommitArtifact("orphan1", 4)

	// Check orphans
	orphans := m.GetOrphanIDs()
	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(orphans))
	}
	if orphans[0] != "orphan2" {
		t.Errorf("expected orphan2, got %s", orphans[0])
	}
}

func TestArtifactManager_CommitBeforeChunks_NotOrphan(t *testing.T) {
	m := NewArtifactManager()

	// Commit arrives before chunks
	err := m.CommitArtifact("pending", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Add some data (not complete yet)
	err = m.AddChunk(&types.ArtifactChunk{
		ArtifactID: "pending",
		Seq:        1,
		IsLast:     false,
		Data:       make([]byte, 5),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Artifact with pending commit should NOT be reported as orphan
	orphans := m.GetOrphanIDs()
	for _, id := range orphans {
		if id == "pending" {
			t.Error("artifact with pending commit should not be reported as orphan")
		}
	}

	// Stats should also not count it as orphaned
	stats := m.Stats()
	if stats.OrphanedArtifacts != 0 {
		t.Errorf("expected 0 orphaned artifacts, got %d", stats.OrphanedArtifacts)
	}
}
