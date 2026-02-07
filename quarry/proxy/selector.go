// Package proxy implements proxy pool selection per CONTRACT_PROXY.md.
package proxy

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/justapithecus/quarry/types"
)

// Selector manages proxy selection from pools.
// Thread-safe for concurrent access.
type Selector struct {
	mu    sync.Mutex
	pools map[string]*poolState
}

// poolState holds runtime state for a single pool.
type poolState struct {
	pool      *types.ProxyPool
	rrIndex   int64                   // round-robin counter
	stickyMap map[string]*stickyEntry // sticky key -> entry

	// Recency tracking for random strategy (nil when RecencyWindow unset).
	recencyRing []int // circular buffer of recently-used endpoint indices
	recencyPos  int   // next write position in the ring
	recencyLen  int   // number of valid entries (grows from 0 to cap)
}

// stickyEntry holds a sticky assignment with optional TTL.
type stickyEntry struct {
	endpointIdx int
	expiresAt   *time.Time
}

// NewSelector creates a new proxy selector.
func NewSelector() *Selector {
	return &Selector{
		pools: make(map[string]*poolState),
	}
}

// RegisterPool registers a proxy pool.
// Returns error if pool validation fails.
// Emits soft warnings per CONTRACT_PROXY.md to stderr.
func (s *Selector) RegisterPool(pool *types.ProxyPool) error {
	if err := pool.Validate(); err != nil {
		return fmt.Errorf("pool validation failed: %w", err)
	}

	// Emit soft warnings per CONTRACT_PROXY.md
	warnings := pool.Warnings()
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state := &poolState{
		pool:      pool,
		rrIndex:   0,
		stickyMap: make(map[string]*stickyEntry),
	}

	// Allocate recency ring buffer when configured.
	// Sentinel value -1 distinguishes empty slots from valid index 0.
	if pool.RecencyWindow != nil {
		state.recencyRing = make([]int, *pool.RecencyWindow)
		for i := range state.recencyRing {
			state.recencyRing[i] = -1
		}
	}

	s.pools[pool.Name] = state

	return nil
}

// SelectRequest contains parameters for endpoint selection.
type SelectRequest struct {
	// Pool is the pool name to select from.
	Pool string
	// StrategyOverride optionally overrides the pool's strategy.
	StrategyOverride *types.ProxyStrategy
	// StickyKey is the key for sticky selection.
	// Required when strategy is sticky (either from pool or override).
	StickyKey string
	// Domain is used to derive sticky key when scope is "domain".
	Domain string
	// Origin is used to derive sticky key when scope is "origin" (scheme+host+port).
	Origin string
	// JobID is used to derive sticky key when scope is "job".
	JobID string
	// Commit determines whether to advance rotation counters.
	// When false, returns what would be selected without mutating state.
	Commit bool
}

// Select selects a proxy endpoint from the specified pool.
// Returns error if pool is not found or selection fails.
func (s *Selector) Select(req SelectRequest) (*types.ProxyEndpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.pools[req.Pool]
	if !ok {
		return nil, fmt.Errorf("pool %q not found", req.Pool)
	}

	// Determine effective strategy
	strategy := state.pool.Strategy
	if req.StrategyOverride != nil {
		strategy = *req.StrategyOverride
	}

	var idx int
	var err error

	switch strategy {
	case types.ProxyStrategyRoundRobin:
		idx = s.selectRoundRobin(state, req.Commit)
	case types.ProxyStrategyRandom:
		idx, err = s.selectRandom(state, req.Commit)
		if err != nil {
			return nil, err
		}
	case types.ProxyStrategySticky:
		idx, err = s.selectSticky(state, req, req.Commit)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown strategy %q", strategy)
	}

	// Return a copy of the endpoint
	ep := state.pool.Endpoints[idx]
	return &ep, nil
}

// selectRoundRobin selects using round-robin.
// Increments counter only when commit is true.
func (s *Selector) selectRoundRobin(state *poolState, commit bool) int {
	idx := int(state.rrIndex % int64(len(state.pool.Endpoints)))
	if commit {
		state.rrIndex++
	}
	return idx
}

// selectRandom selects uniformly at random.
// When the pool has a recency window, recently-used indices are excluded.
// Ring buffer is advanced only when commit is true.
func (s *Selector) selectRandom(state *poolState, commit bool) (int, error) {
	n := len(state.pool.Endpoints)
	if n == 1 {
		return 0, nil
	}

	// No recency tracking — pure random (original behavior)
	if state.recencyRing == nil {
		return s.randInt(n)
	}

	// Build exclusion set from ring buffer
	excluded := make(map[int]bool, state.recencyLen)
	for i := range state.recencyLen {
		if idx := state.recencyRing[i]; idx >= 0 {
			excluded[idx] = true
		}
	}

	// Build candidates not in exclusion set
	candidates := make([]int, 0, n-len(excluded))
	for i := range n {
		if !excluded[i] {
			candidates = append(candidates, i)
		}
	}

	var selectedIdx int

	if len(candidates) == 0 {
		// All endpoints excluded (window >= endpoint count).
		// Pick the LRU: oldest entry in the ring at the next write position.
		selectedIdx = state.recencyRing[state.recencyPos]
	} else {
		ci, err := s.randInt(len(candidates))
		if err != nil {
			return 0, err
		}
		selectedIdx = candidates[ci]
	}

	if commit {
		state.recencyRing[state.recencyPos] = selectedIdx
		state.recencyPos = (state.recencyPos + 1) % len(state.recencyRing)
		if state.recencyLen < len(state.recencyRing) {
			state.recencyLen++
		}
	}

	return selectedIdx, nil
}

// randInt returns a cryptographically random int in [0, n).
func (s *Selector) randInt(n int) (int, error) {
	bigIdx, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, fmt.Errorf("random selection failed: %w", err)
	}
	return int(bigIdx.Int64()), nil
}

// selectSticky selects using sticky assignment.
// Stores new assignment only when commit is true.
func (s *Selector) selectSticky(state *poolState, req SelectRequest, commit bool) (int, error) {
	// Derive sticky key per CONTRACT_PROXY.md precedence
	stickyKey := s.deriveStickyKey(state, req)
	if stickyKey == "" {
		return 0, errors.New("sticky selection requires a sticky key")
	}

	now := time.Now()

	// Check existing entry
	if entry, ok := state.stickyMap[stickyKey]; ok {
		// Check TTL expiration
		if entry.expiresAt == nil || entry.expiresAt.After(now) {
			return entry.endpointIdx, nil
		}
		// Entry expired, remove it
		delete(state.stickyMap, stickyKey)
	}

	// Select new endpoint (use random for new assignments).
	// Pass commit=false: sticky has its own persistence; don't pollute recency ring.
	idx, err := s.selectRandom(state, false)
	if err != nil {
		return 0, err
	}

	// Store assignment only when commit is true
	if commit {
		entry := &stickyEntry{endpointIdx: idx}
		if state.pool.Sticky != nil && state.pool.Sticky.TTLMs != nil {
			ttl := time.Duration(*state.pool.Sticky.TTLMs) * time.Millisecond
			expiresAt := now.Add(ttl)
			entry.expiresAt = &expiresAt
		}
		state.stickyMap[stickyKey] = entry
	}

	return idx, nil
}

// deriveStickyKey derives the sticky key per CONTRACT_PROXY.md precedence:
// 1. req.StickyKey if provided
// 2. if scope = job: req.JobID
// 3. if scope = domain: req.Domain
// 4. if scope = origin: req.Origin
func (s *Selector) deriveStickyKey(state *poolState, req SelectRequest) string {
	// 1. Explicit sticky key takes precedence
	if req.StickyKey != "" {
		return req.StickyKey
	}

	// 2. Derive from scope
	if state.pool.Sticky == nil {
		// No sticky config, use JobID as default
		return req.JobID
	}

	switch state.pool.Sticky.Scope {
	case types.ProxyStickyJob:
		return req.JobID
	case types.ProxyStickyDomain:
		return req.Domain
	case types.ProxyStickyOrigin:
		return req.Origin
	default:
		return req.JobID
	}
}

// PoolStats returns statistics for a pool.
type PoolStats struct {
	RoundRobinIndex int64
	StickyEntries   int
	RecencyWindow   int // configured window size (0 if not set)
	RecencyFill     int // how many ring slots are currently occupied
}

// Stats returns statistics for a pool.
func (s *Selector) Stats(poolName string) (*PoolStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.pools[poolName]
	if !ok {
		return nil, fmt.Errorf("pool %q not found", poolName)
	}

	stats := &PoolStats{
		RoundRobinIndex: state.rrIndex,
		StickyEntries:   len(state.stickyMap),
	}

	if state.recencyRing != nil {
		stats.RecencyWindow = len(state.recencyRing)
		stats.RecencyFill = state.recencyLen
	}

	return stats, nil
}

// CleanExpiredSticky removes expired sticky entries from all pools.
// Call periodically to prevent unbounded growth.
func (s *Selector) CleanExpiredSticky() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, state := range s.pools {
		for key, entry := range state.stickyMap {
			if entry.expiresAt != nil && entry.expiresAt.Before(now) {
				delete(state.stickyMap, key)
			}
		}
	}
}
