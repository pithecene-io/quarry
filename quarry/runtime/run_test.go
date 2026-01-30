package runtime

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"sync"
	"testing"

	"github.com/vmihailenco/msgpack/v5"

	"github.com/justapithecus/quarry/policy"
	"github.com/justapithecus/quarry/types"
)

// mockExecutor is a test executor that produces configurable stdout.
// It simulates a real executor by blocking Wait() until killed or released.
type mockExecutor struct {
	mu           sync.Mutex
	stdout       *bytes.Buffer
	started      bool
	killed       bool
	exitCode     int
	startErr     error
	killChan     chan struct{} // signals Wait to return when Kill is called
	releaseChan  chan struct{} // signals Wait to return for normal completion
	blockOnWait  bool          // if true, Wait blocks until kill or release
}

func newMockExecutor(stdout []byte, exitCode int) *mockExecutor {
	return &mockExecutor{
		stdout:      bytes.NewBuffer(stdout),
		exitCode:    exitCode,
		killChan:    make(chan struct{}),
		releaseChan: make(chan struct{}),
		blockOnWait: false,
	}
}

// newBlockingMockExecutor creates a mock that blocks Wait() until killed.
// This simulates a long-running executor process.
func newBlockingMockExecutor(stdout []byte, exitCode int) *mockExecutor {
	return &mockExecutor{
		stdout:      bytes.NewBuffer(stdout),
		exitCode:    exitCode,
		killChan:    make(chan struct{}),
		releaseChan: make(chan struct{}),
		blockOnWait: true,
	}
}

func (m *mockExecutor) Start(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startErr != nil {
		return m.startErr
	}
	m.started = true
	return nil
}

func (m *mockExecutor) Stdout() io.Reader {
	return m.stdout
}

func (m *mockExecutor) Wait() (*ExecutorResult, error) {
	if m.blockOnWait {
		// Block until killed or released
		select {
		case <-m.killChan:
			// Killed - return with configured exit code
		case <-m.releaseChan:
			// Released for normal completion
		}
	}
	return &ExecutorResult{
		ExitCode:    m.exitCode,
		StderrBytes: []byte{},
	}, nil
}

func (m *mockExecutor) Kill() error {
	m.mu.Lock()
	alreadyKilled := m.killed
	m.killed = true
	m.mu.Unlock()

	// Signal Wait to return (only once)
	if !alreadyKilled {
		close(m.killChan)
	}
	return nil
}

func (m *mockExecutor) Release() {
	// Signal Wait to return for normal completion
	select {
	case <-m.releaseChan:
		// already closed
	default:
		close(m.releaseChan)
	}
}

func (m *mockExecutor) WasKilled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.killed
}

// flushTrackingPolicy wraps a policy and tracks flush calls.
type flushTrackingPolicy struct {
	policy.Policy
	mu          sync.Mutex
	flushCalled bool
	flushErr    error
}

func newFlushTrackingPolicy() *flushTrackingPolicy {
	return &flushTrackingPolicy{
		Policy: policy.NewNoopPolicy(),
	}
}

func (p *flushTrackingPolicy) Flush(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.flushCalled = true
	if p.flushErr != nil {
		return p.flushErr
	}
	return p.Policy.Flush(ctx)
}

func (p *flushTrackingPolicy) WasFlushed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.flushCalled
}

// encodeTestFrame encodes a payload with length prefix for IPC.
func encodeTestFrame(payload []byte) []byte {
	buf := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(payload)))
	copy(buf[4:], payload)
	return buf
}

// encodeTestEventFrame creates a framed event envelope.
func encodeTestEventFrame(envelope *types.EventEnvelope) []byte {
	payload, _ := msgpack.Marshal(envelope)
	return encodeTestFrame(payload)
}

// makeValidEventStream creates a valid event stream with run_complete.
func makeValidEventStream(runMeta *types.RunMeta) []byte {
	envelope := &types.EventEnvelope{
		ContractVersion: types.ContractVersion,
		EventID:         "evt-1",
		RunID:           runMeta.RunID,
		Seq:             1,
		Type:            types.EventTypeRunComplete,
		Ts:              "2024-01-01T00:00:00Z",
		Payload:         map[string]any{},
		Attempt:         runMeta.Attempt,
	}
	return encodeTestEventFrame(envelope)
}

// makeInvalidFrame creates an invalid msgpack frame (stream error).
func makeInvalidFrame() []byte {
	return encodeTestFrame([]byte{0xFF, 0xFF, 0xFF})
}

func TestRunOrchestrator_FlushCalledOnStreamError(t *testing.T) {
	runMeta := &types.RunMeta{
		RunID:   "run-flush-stream",
		Attempt: 1,
	}

	// Create executor that produces invalid frame (causes stream error)
	mockExec := newMockExecutor(makeInvalidFrame(), 1)

	// Create tracking policy
	trackingPol := newFlushTrackingPolicy()

	config := &RunConfig{
		ExecutorPath: "/fake/executor",
		ScriptPath:   "/fake/script.js",
		Job:          map[string]any{},
		RunMeta:      runMeta,
		Policy:       trackingPol,
		ExecutorFactory: func(_ *ExecutorConfig) Executor {
			return mockExec
		},
	}

	orchestrator, err := NewRunOrchestrator(config)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	result, err := orchestrator.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	// Verify outcome is executor crash (stream error)
	if result.Outcome.Status != types.OutcomeExecutorCrash {
		t.Errorf("expected OutcomeExecutorCrash, got %s", result.Outcome.Status)
	}

	// Verify flush was called despite the error
	if !trackingPol.WasFlushed() {
		t.Error("expected policy Flush to be called on stream error path")
	}
}

func TestRunOrchestrator_FlushCalledOnPolicyError(t *testing.T) {
	runMeta := &types.RunMeta{
		RunID:   "run-flush-policy",
		Attempt: 1,
	}

	// Create executor that produces a valid event
	eventData := makeValidEventStream(runMeta)
	mockExec := newMockExecutor(eventData, 0)

	// Create policy that fails on ingestion
	failingPol := &failingIngestPolicy{
		flushTrackingPolicy: newFlushTrackingPolicy(),
	}

	config := &RunConfig{
		ExecutorPath: "/fake/executor",
		ScriptPath:   "/fake/script.js",
		Job:          map[string]any{},
		RunMeta:      runMeta,
		Policy:       failingPol,
		ExecutorFactory: func(_ *ExecutorConfig) Executor {
			return mockExec
		},
	}

	orchestrator, err := NewRunOrchestrator(config)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	result, err := orchestrator.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	// Verify outcome is policy failure
	if result.Outcome.Status != types.OutcomePolicyFailure {
		t.Errorf("expected OutcomePolicyFailure, got %s", result.Outcome.Status)
	}

	// Verify flush was called despite the error
	if !failingPol.WasFlushed() {
		t.Error("expected policy Flush to be called on policy error path")
	}
}

// failingIngestPolicy fails on IngestEvent.
type failingIngestPolicy struct {
	*flushTrackingPolicy
}

func (p *failingIngestPolicy) IngestEvent(_ context.Context, _ *types.EventEnvelope) error {
	return &policyError{msg: "simulated policy failure"}
}

type policyError struct {
	msg string
}

func (e *policyError) Error() string {
	return e.msg
}

func TestRunOrchestrator_ExecutorKilledOnStreamError(t *testing.T) {
	runMeta := &types.RunMeta{
		RunID:   "run-kill-stream",
		Attempt: 1,
	}

	// Create blocking executor that produces invalid frame
	// The executor blocks on Wait() until killed, simulating a long-running process
	mockExec := newBlockingMockExecutor(makeInvalidFrame(), 1)

	config := &RunConfig{
		ExecutorPath: "/fake/executor",
		ScriptPath:   "/fake/script.js",
		Job:          map[string]any{},
		RunMeta:      runMeta,
		Policy:       policy.NewNoopPolicy(),
		ExecutorFactory: func(_ *ExecutorConfig) Executor {
			return mockExec
		},
	}

	orchestrator, err := NewRunOrchestrator(config)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	_, err = orchestrator.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	// Verify executor was killed due to stream error
	if !mockExec.WasKilled() {
		t.Error("expected executor to be killed on stream error")
	}
}

func TestRunOrchestrator_ExecutorKilledOnPolicyError(t *testing.T) {
	runMeta := &types.RunMeta{
		RunID:   "run-kill-policy",
		Attempt: 1,
	}

	// Create blocking executor that produces a valid event
	// The executor blocks on Wait() until killed, simulating a long-running process
	eventData := makeValidEventStream(runMeta)
	mockExec := newBlockingMockExecutor(eventData, 0)

	// Create policy that fails on ingestion
	failingPol := &failingIngestPolicy{
		flushTrackingPolicy: newFlushTrackingPolicy(),
	}

	config := &RunConfig{
		ExecutorPath: "/fake/executor",
		ScriptPath:   "/fake/script.js",
		Job:          map[string]any{},
		RunMeta:      runMeta,
		Policy:       failingPol,
		ExecutorFactory: func(_ *ExecutorConfig) Executor {
			return mockExec
		},
	}

	orchestrator, err := NewRunOrchestrator(config)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	_, err = orchestrator.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	// Verify executor was killed due to policy error
	if !mockExec.WasKilled() {
		t.Error("expected executor to be killed on policy error")
	}
}

func TestRunOrchestrator_SuccessfulRun(t *testing.T) {
	runMeta := &types.RunMeta{
		RunID:   "run-success",
		Attempt: 1,
	}

	// Create executor that produces valid run_complete event
	eventData := makeValidEventStream(runMeta)
	mockExec := newMockExecutor(eventData, 0)

	trackingPol := newFlushTrackingPolicy()

	config := &RunConfig{
		ExecutorPath: "/fake/executor",
		ScriptPath:   "/fake/script.js",
		Job:          map[string]any{},
		RunMeta:      runMeta,
		Policy:       trackingPol,
		ExecutorFactory: func(_ *ExecutorConfig) Executor {
			return mockExec
		},
	}

	orchestrator, err := NewRunOrchestrator(config)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	result, err := orchestrator.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	// Verify successful outcome
	if result.Outcome.Status != types.OutcomeSuccess {
		t.Errorf("expected OutcomeSuccess, got %s: %s", result.Outcome.Status, result.Outcome.Message)
	}

	// Verify flush was called on success path too
	if !trackingPol.WasFlushed() {
		t.Error("expected policy Flush to be called on success path")
	}

	// Verify executor was NOT killed on success
	if mockExec.WasKilled() {
		t.Error("executor should not be killed on successful run")
	}
}

func TestIsStreamError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "stream error",
			err:      &IngestionError{Kind: IngestionErrorStream, Err: io.EOF},
			expected: true,
		},
		{
			name:     "policy error",
			err:      &IngestionError{Kind: IngestionErrorPolicy, Err: io.EOF},
			expected: false,
		},
		{
			name:     "canceled error",
			err:      &IngestionError{Kind: IngestionErrorCanceled, Err: context.Canceled},
			expected: false,
		},
		{
			name:     "non-ingestion error",
			err:      io.EOF,
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsStreamError(tt.err)
			if got != tt.expected {
				t.Errorf("IsStreamError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}
