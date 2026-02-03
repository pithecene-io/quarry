package runtime

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/justapithecus/quarry/log"
	"github.com/justapithecus/quarry/policy"
	"github.com/justapithecus/quarry/types"
)

// Executor abstracts executor process lifecycle for testing.
type Executor interface {
	Start(ctx context.Context) error
	Stdout() io.Reader
	Wait() (*ExecutorResult, error)
	Kill() error
}

// ExecutorFactory creates an Executor. Used for test injection.
type ExecutorFactory func(config *ExecutorConfig) Executor

// RunConfig configures a single run.
type RunConfig struct {
	// ExecutorPath is the path to the executor binary.
	ExecutorPath string
	// ScriptPath is the path to the script file.
	ScriptPath string
	// Job is the job payload.
	Job any
	// RunMeta is the run identity and lineage metadata.
	RunMeta *types.RunMeta
	// Proxy is the optional resolved proxy endpoint per CONTRACT_PROXY.md.
	// If nil, executor launches without a proxy.
	Proxy *types.ProxyEndpoint
	// Policy is the ingestion policy.
	Policy policy.Policy
	// ExecutorFactory overrides executor creation (for testing).
	// If nil, uses NewExecutorManager.
	ExecutorFactory ExecutorFactory
}

// RunResult represents the result of a run.
type RunResult struct {
	// RunMeta is the run identity and lineage.
	RunMeta *types.RunMeta
	// Outcome is the run outcome.
	Outcome *types.RunOutcome
	// Duration is the total run duration.
	Duration time.Duration
	// PolicyStats is the policy statistics.
	PolicyStats policy.Stats
	// ArtifactStats is the artifact accumulation statistics.
	ArtifactStats ArtifactStats
	// OrphanIDs is the list of orphaned artifact IDs.
	OrphanIDs []string
	// StderrOutput is the captured executor stderr.
	StderrOutput string
	// EventCount is the total number of events processed.
	EventCount int64
	// ProxyUsed is the proxy endpoint used (redacted, no password).
	// Nil if no proxy was configured.
	ProxyUsed *types.ProxyEndpointRedacted
}

// RunOrchestrator orchestrates a single run.
type RunOrchestrator struct {
	config    *RunConfig
	logger    *log.Logger
	startTime time.Time
}

// NewRunOrchestrator creates a new run orchestrator.
// Returns error if run metadata is invalid.
func NewRunOrchestrator(config *RunConfig) (*RunOrchestrator, error) {
	// Validate run metadata per CONTRACT_RUN.md
	if err := config.RunMeta.Validate(); err != nil {
		return nil, fmt.Errorf("invalid run metadata: %w", err)
	}

	// Create logger with run context
	logger := log.NewLogger(config.RunMeta)

	return &RunOrchestrator{
		config: config,
		logger: logger,
	}, nil
}

// Execute executes the run end-to-end.
// This is the main entry point for run orchestration.
//
// Execution flow:
//  1. Start executor process
//  2. Run IPC ingestion loop (concurrent)
//  3. Wait for executor exit
//  4. Flush policy
//  5. Determine outcome
//  6. Return result
func (r *RunOrchestrator) Execute(ctx context.Context) (*RunResult, error) {
	r.startTime = time.Now()

	r.logger.Info("starting run", map[string]any{
		"script":   r.config.ScriptPath,
		"executor": r.config.ExecutorPath,
	})

	// Create executor
	execConfig := &ExecutorConfig{
		ExecutorPath: r.config.ExecutorPath,
		ScriptPath:   r.config.ScriptPath,
		Job:          r.config.Job,
		RunMeta:      r.config.RunMeta,
		Proxy:        r.config.Proxy,
	}

	var executor Executor
	if r.config.ExecutorFactory != nil {
		executor = r.config.ExecutorFactory(execConfig)
	} else {
		executor = NewExecutorManager(execConfig)
	}

	// Start executor
	if err := executor.Start(ctx); err != nil {
		r.logger.Error("failed to start executor", map[string]any{
			"error": err.Error(),
		})
		// Best-effort flush even on start failure for strict termination semantics
		flushCtx, flushCancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		if flushErr := r.config.Policy.Flush(flushCtx); flushErr != nil {
			r.logger.Warn("policy flush failed (best effort)", map[string]any{
				"error": flushErr.Error(),
			})
		}
		flushCancel()
		return r.buildResult(&types.RunOutcome{
			Status:  types.OutcomeExecutorCrash,
			Message: fmt.Sprintf("failed to start executor: %v", err),
		}, "", nil, nil), nil
	}

	// Create artifact manager
	artifacts := NewArtifactManager()

	// Create ingestion engine
	ingestion := NewIngestionEngine(
		executor.Stdout(),
		r.config.Policy,
		artifacts,
		r.logger,
		r.config.RunMeta,
	)

	// Run ingestion in goroutine
	ingestionDone := make(chan error, 1)
	go func() {
		ingestionDone <- ingestion.Run(ctx)
	}()

	// Wait for executor in goroutine
	type execResultPair struct {
		result *ExecutorResult
		err    error
	}
	executorDone := make(chan execResultPair, 1)
	go func() {
		result, err := executor.Wait()
		executorDone <- execResultPair{result, err}
	}()

	// Wait for both, but kill executor on any ingestion error (policy or stream)
	var execResult *ExecutorResult
	var execErr error
	var ingErr error
	executorFinished := false
	ingestionFinished := false

	for !executorFinished || !ingestionFinished {
		select {
		case pair := <-executorDone:
			execResult = pair.result
			execErr = pair.err
			executorFinished = true
		case err := <-ingestionDone:
			ingErr = err
			ingestionFinished = true

			// On ANY ingestion error (policy, stream, or canceled), kill executor immediately
			// This prevents the executor from continuing to emit after we've decided to terminate
			if err != nil && !executorFinished {
				r.logger.Warn("killing executor due to ingestion error", map[string]any{
					"error":     err.Error(),
					"is_policy": IsPolicyError(err),
				})
				_ = executor.Kill()
			}
		}
	}

	// Always attempt policy flush (best effort) on all termination paths
	// Per CONTRACT_POLICY.md: "Buffered events must be flushed on run_complete, run_error, runtime termination (best effort)"
	// Use WithoutCancel to preserve context values (tracing) while ignoring parent cancellation
	flushCtx, flushCancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	flushErr := r.config.Policy.Flush(flushCtx)
	flushCancel()
	if flushErr != nil {
		r.logger.Warn("policy flush failed (best effort)", map[string]any{
			"error": flushErr.Error(),
		})
	}

	// Handle executor wait error
	if execErr != nil {
		r.logger.Error("executor wait failed", map[string]any{
			"error": execErr.Error(),
		})
		return r.buildResult(&types.RunOutcome{
			Status:  types.OutcomeExecutorCrash,
			Message: fmt.Sprintf("executor wait failed: %v", execErr),
		}, "", artifacts, ingestion), nil
	}

	// Handle ingestion errors
	if ingErr != nil {
		r.logger.Error("ingestion failed", map[string]any{
			"error":     ingErr.Error(),
			"exit_code": execResult.ExitCode,
		})

		// Classify the error
		var outcome *types.RunOutcome
		switch {
		case IsPolicyError(ingErr):
			outcome = &types.RunOutcome{
				Status:  types.OutcomePolicyFailure,
				Message: fmt.Sprintf("policy failure: %v", ingErr),
			}
		case IsCanceledError(ingErr):
			outcome = &types.RunOutcome{
				Status:  types.OutcomeExecutorCrash,
				Message: fmt.Sprintf("run canceled: %v", ingErr),
			}
		default:
			// Stream/frame errors are executor crash
			outcome = &types.RunOutcome{
				Status:  types.OutcomeExecutorCrash,
				Message: fmt.Sprintf("stream error: %v", ingErr),
			}
		}

		return r.buildResult(outcome, string(execResult.StderrBytes), artifacts, ingestion), nil
	}

	// If flush failed and there were no other errors, report policy failure
	if flushErr != nil {
		return r.buildResult(&types.RunOutcome{
			Status:  types.OutcomePolicyFailure,
			Message: fmt.Sprintf("policy flush failed: %v", flushErr),
		}, string(execResult.StderrBytes), artifacts, ingestion), nil
	}

	// Determine outcome based on exit code and terminal event
	// If we have a run_result frame from the executor, use it as the primary source
	// Otherwise fall back to exit code + terminal event analysis
	var outcome *types.RunOutcome
	runResultFrame := ingestion.GetRunResult()

	if runResultFrame != nil {
		// Use run_result frame from executor
		outcome = runResultOutcomeToRunOutcome(runResultFrame)
		r.logger.Info("run completed (from run_result)", map[string]any{
			"outcome":   outcome.Status,
			"exit_code": execResult.ExitCode,
			"duration":  time.Since(r.startTime).String(),
		})
	} else {
		// Fall back to exit code + terminal event analysis
		terminalEvent, hasTerminal := ingestion.GetTerminalEvent()
		outcome = DetermineOutcome(execResult.ExitCode, hasTerminal, terminalEvent)
		r.logger.Info("run completed", map[string]any{
			"outcome":      outcome.Status,
			"exit_code":    execResult.ExitCode,
			"duration":     time.Since(r.startTime).String(),
			"has_terminal": hasTerminal,
		})
	}

	return r.buildResult(outcome, string(execResult.StderrBytes), artifacts, ingestion), nil
}

// runResultOutcomeToRunOutcome converts a RunResultFrame to a RunOutcome.
func runResultOutcomeToRunOutcome(frame *types.RunResultFrame) *types.RunOutcome {
	var status types.OutcomeStatus
	switch frame.Outcome.Status {
	case types.RunResultStatusCompleted:
		status = types.OutcomeSuccess
	case types.RunResultStatusError:
		status = types.OutcomeScriptError
	case types.RunResultStatusCrash:
		status = types.OutcomeExecutorCrash
	default:
		status = types.OutcomeExecutorCrash
	}

	var message string
	if frame.Outcome.Message != nil {
		message = *frame.Outcome.Message
	} else {
		message = string(frame.Outcome.Status)
	}

	outcome := &types.RunOutcome{
		Status:    status,
		Message:   message,
		ErrorType: frame.Outcome.ErrorType,
		Stack:     frame.Outcome.Stack,
	}

	return outcome
}

// buildResult constructs the final run result.
func (r *RunOrchestrator) buildResult(
	outcome *types.RunOutcome,
	stderrOutput string,
	artifacts *ArtifactManager,
	ingestion *IngestionEngine,
) *RunResult {
	result := &RunResult{
		RunMeta:      r.config.RunMeta,
		Outcome:      outcome,
		Duration:     time.Since(r.startTime),
		PolicyStats:  r.config.Policy.Stats(),
		StderrOutput: stderrOutput,
	}

	// Set redacted proxy (per CONTRACT_PROXY.md: exclude password)
	// Prefer run_result.proxy_used if available, otherwise use config.Proxy
	if ingestion != nil {
		if runResult := ingestion.GetRunResult(); runResult != nil && runResult.ProxyUsed != nil {
			result.ProxyUsed = runResult.ProxyUsed
		}
	}
	if result.ProxyUsed == nil && r.config.Proxy != nil {
		redacted := r.config.Proxy.Redact()
		result.ProxyUsed = &redacted
	}

	if artifacts != nil {
		result.ArtifactStats = artifacts.Stats()
		result.OrphanIDs = artifacts.GetOrphanIDs()
	}

	if ingestion != nil {
		result.EventCount = ingestion.CurrentSeq()
	}

	return result
}
