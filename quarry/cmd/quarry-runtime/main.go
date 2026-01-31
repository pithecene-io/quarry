// Package main provides the quarry-runtime CLI entrypoint.
//
// Usage:
//
//	quarry-runtime run -script <path> -run-id <id> [options]
//
// Exit codes:
//   - 0: success (run_complete)
//   - 1: script error (run_error)
//   - 2: executor crash
//   - 3: policy failure
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/justapithecus/quarry/policy"
	"github.com/justapithecus/quarry/runtime"
	"github.com/justapithecus/quarry/types"
	"github.com/urfave/cli/v2"
)

// Exit codes per CONTRACT_RUN.md
const (
	exitSuccess       = 0
	exitScriptError   = 1
	exitExecutorCrash = 2
	exitPolicyFailure = 3
)

// policyChoice holds parsed policy configuration from CLI flags.
type policyChoice struct {
	name      string
	flushMode string
	maxEvents int
	maxBytes  int64
}

func main() {
	app := &cli.App{
		Name:    "quarry-runtime",
		Usage:   "Quarry runtime orchestrator - supervises executor and ingests events",
		Version: "0.1.0",
		Commands: []*cli.Command{
			runCommand(),
		},
		ExitErrHandler: exitErrHandler,
	}

	if err := app.Run(os.Args); err != nil {
		// ExitErrHandler already handled the exit
		// This branch is only reached if ExitErrHandler didn't exit
		os.Exit(exitExecutorCrash)
	}
}

// exitErrHandler handles errors from the CLI, respecting cli.ExitCoder.
func exitErrHandler(c *cli.Context, err error) {
	if err == nil {
		return
	}

	// Check for ExitCoder (from cli.Exit), handles wrapped errors
	var exitCoder cli.ExitCoder
	if errors.As(err, &exitCoder) {
		code := exitCoder.ExitCode()
		msg := exitCoder.Error()

		// Only print if there's a real message (not just "exit status N")
		// cli.Exit("", N).Error() returns "exit status N", so skip those
		if msg != "" && msg != fmt.Sprintf("exit status %d", code) {
			fmt.Fprintln(os.Stderr, msg)
		}
		os.Exit(code)
	}

	// Unexpected error - print and exit with crash code
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(exitExecutorCrash)
}

func runCommand() *cli.Command {
	return &cli.Command{
		Name:  "run",
		Usage: "Execute a script run",
		Flags: []cli.Flag{
			// Execution flags
			&cli.StringFlag{
				Name:     "script",
				Usage:    "Path to script file",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "run-id",
				Usage:    "Run ID",
				Required: true,
			},
			&cli.IntFlag{
				Name:  "attempt",
				Usage: "Attempt number (starts at 1)",
				Value: 1,
			},
			&cli.StringFlag{
				Name:  "job-id",
				Usage: "Job ID (optional)",
			},
			&cli.StringFlag{
				Name:  "parent-run-id",
				Usage: "Parent run ID (required for retries)",
			},
			&cli.StringFlag{
				Name:  "job",
				Usage: "Job payload as JSON",
				Value: "{}",
			},
			&cli.StringFlag{
				Name:  "executor",
				Usage: "Path to executor binary",
				Value: "quarry-executor",
			},
			&cli.BoolFlag{
				Name:  "quiet",
				Usage: "Suppress result output",
			},
			// Policy flags
			&cli.StringFlag{
				Name:  "policy",
				Usage: "Ingestion policy: strict or buffered",
				Value: "strict",
			},
			&cli.StringFlag{
				Name:  "flush-mode",
				Usage: "Flush mode for buffered policy: at_least_once, chunks_first, two_phase",
				Value: "at_least_once",
			},
			&cli.IntFlag{
				Name:  "buffer-events",
				Usage: "Max buffered events (buffered policy)",
				Value: 0,
			},
			&cli.Int64Flag{
				Name:  "buffer-bytes",
				Usage: "Max buffer size in bytes (buffered policy)",
				Value: 0,
			},
		},
		Action: runAction,
	}
}

func runAction(c *cli.Context) error {
	// Parse policy config
	choice := policyChoice{
		name:      c.String("policy"),
		flushMode: c.String("flush-mode"),
		maxEvents: c.Int("buffer-events"),
		maxBytes:  c.Int64("buffer-bytes"),
	}

	// Validate policy config
	if err := validatePolicyConfig(choice); err != nil {
		return cli.Exit(fmt.Sprintf("invalid policy config: %v", err), exitExecutorCrash)
	}

	// Parse job payload
	var job any
	if err := json.Unmarshal([]byte(c.String("job")), &job); err != nil {
		return fmt.Errorf("invalid job JSON: %w", err)
	}

	// Build run metadata
	runMeta := &types.RunMeta{
		RunID:   c.String("run-id"),
		Attempt: c.Int("attempt"),
	}
	if jobID := c.String("job-id"); jobID != "" {
		runMeta.JobID = &jobID
	}
	if parentRunID := c.String("parent-run-id"); parentRunID != "" {
		runMeta.ParentRunID = &parentRunID
	}

	// Build policy
	pol, err := buildPolicy(choice)
	if err != nil {
		return fmt.Errorf("failed to create policy: %w", err)
	}
	defer func() { _ = pol.Close() }()

	// Create run config
	config := &runtime.RunConfig{
		ExecutorPath: c.String("executor"),
		ScriptPath:   c.String("script"),
		Job:          job,
		RunMeta:      runMeta,
		Policy:       pol,
	}

	// Create orchestrator
	orchestrator, err := runtime.NewRunOrchestrator(config)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Set up context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Execute run
	startTime := time.Now()
	result, err := orchestrator.Execute(ctx)
	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}
	duration := time.Since(startTime)

	// Print result
	if !c.Bool("quiet") {
		printResult(result, choice, duration)
	}

	return cli.Exit("", outcomeToExitCode(result.Outcome.Status))
}

// validatePolicyConfig validates the policy configuration.
func validatePolicyConfig(choice policyChoice) error {
	switch choice.name {
	case "strict":
		// Warn if buffer/flush flags are set (ignored)
		if choice.maxEvents > 0 || choice.maxBytes > 0 || choice.flushMode != "at_least_once" {
			fmt.Fprintf(os.Stderr, "Warning: buffer/flush flags ignored for strict policy\n")
		}
		return nil

	case "buffered":
		// Require at least one buffer limit
		if choice.maxEvents <= 0 && choice.maxBytes <= 0 {
			return fmt.Errorf("buffered policy requires --buffer-events > 0 or --buffer-bytes > 0")
		}
		// Validate flush mode
		switch policy.FlushMode(choice.flushMode) {
		case policy.FlushAtLeastOnce, policy.FlushChunksFirst, policy.FlushTwoPhase:
			return nil
		default:
			return fmt.Errorf("invalid flush-mode: %s (must be at_least_once, chunks_first, or two_phase)", choice.flushMode)
		}

	default:
		return fmt.Errorf("invalid policy: %s (must be strict or buffered)", choice.name)
	}
}

// buildPolicy creates a Policy from the CLI configuration.
func buildPolicy(choice policyChoice) (policy.Policy, error) {
	// Use stub sink until Lode integration is ready
	sink := policy.NewStubSink()

	switch choice.name {
	case "strict":
		return policy.NewStrictPolicy(sink), nil

	case "buffered":
		config := policy.BufferedConfig{
			MaxBufferEvents: choice.maxEvents,
			MaxBufferBytes:  choice.maxBytes,
			FlushMode:       policy.FlushMode(choice.flushMode),
		}
		return policy.NewBufferedPolicy(sink, config)

	default:
		return nil, fmt.Errorf("unknown policy: %s", choice.name)
	}
}

// outcomeToExitCode maps outcome status to exit code.
func outcomeToExitCode(status types.OutcomeStatus) int {
	switch status {
	case types.OutcomeSuccess:
		return exitSuccess
	case types.OutcomeScriptError:
		return exitScriptError
	case types.OutcomeExecutorCrash:
		return exitExecutorCrash
	case types.OutcomePolicyFailure:
		return exitPolicyFailure
	default:
		return exitScriptError
	}
}

// printResult prints the run result and summary.
func printResult(result *runtime.RunResult, choice policyChoice, duration time.Duration) {
	// Summary line
	fmt.Printf("\nrun_id=%s, attempt=%d, outcome=%s, duration=%s\n",
		result.RunMeta.RunID,
		result.RunMeta.Attempt,
		result.Outcome.Status,
		duration.Round(time.Millisecond),
	)

	// Policy summary
	if choice.name == "buffered" {
		fmt.Printf("policy=%s, flush_mode=%s, drops=%d, buffer_bytes=%d\n",
			choice.name,
			choice.flushMode,
			result.PolicyStats.EventsDropped,
			result.PolicyStats.BufferSize,
		)
	} else {
		fmt.Printf("policy=%s\n", choice.name)
	}

	// Detailed stats
	fmt.Printf("\n=== Run Result ===\n")
	fmt.Printf("Run ID:       %s\n", result.RunMeta.RunID)
	if result.RunMeta.JobID != nil {
		fmt.Printf("Job ID:       %s\n", *result.RunMeta.JobID)
	}
	if result.RunMeta.ParentRunID != nil {
		fmt.Printf("Parent Run:   %s\n", *result.RunMeta.ParentRunID)
	}
	fmt.Printf("Attempt:      %d\n", result.RunMeta.Attempt)
	fmt.Printf("Outcome:      %s\n", result.Outcome.Status)
	fmt.Printf("Message:      %s\n", result.Outcome.Message)
	fmt.Printf("Duration:     %s\n", result.Duration)
	fmt.Printf("Events:       %d\n", result.EventCount)

	fmt.Printf("\n=== Policy Stats ===\n")
	fmt.Printf("Events Total:     %d\n", result.PolicyStats.TotalEvents)
	fmt.Printf("Events Persisted: %d\n", result.PolicyStats.EventsPersisted)
	fmt.Printf("Events Dropped:   %d\n", result.PolicyStats.EventsDropped)
	fmt.Printf("Chunks Total:     %d\n", result.PolicyStats.TotalChunks)
	fmt.Printf("Flushes:          %d\n", result.PolicyStats.FlushCount)

	if result.ArtifactStats.TotalArtifacts > 0 {
		fmt.Printf("\n=== Artifact Stats ===\n")
		fmt.Printf("Total Artifacts:   %d\n", result.ArtifactStats.TotalArtifacts)
		fmt.Printf("Committed:         %d\n", result.ArtifactStats.CommittedArtifacts)
		fmt.Printf("Orphaned:          %d\n", result.ArtifactStats.OrphanedArtifacts)
		fmt.Printf("Total Chunks:      %d\n", result.ArtifactStats.TotalChunks)
		fmt.Printf("Total Bytes:       %d\n", result.ArtifactStats.TotalBytes)
	}

	if len(result.OrphanIDs) > 0 {
		fmt.Printf("\n=== Orphan Artifacts (eligible for GC) ===\n")
		for _, id := range result.OrphanIDs {
			fmt.Printf("  - %s\n", id)
		}
	}

	if result.StderrOutput != "" {
		fmt.Printf("\n=== Executor Stderr ===\n")
		fmt.Printf("%s", result.StderrOutput)
	}
}
