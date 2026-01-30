package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"syscall"

	"github.com/justapithecus/quarry/types"
)

// ExecutorConfig configures executor execution.
type ExecutorConfig struct {
	// ExecutorPath is the path to the executor binary.
	ExecutorPath string
	// ScriptPath is the path to the script file.
	ScriptPath string
	// Job is the job payload.
	Job any
	// RunMeta is the run metadata.
	RunMeta *types.RunMeta
}

// ExecutorResult represents the result of executor execution.
type ExecutorResult struct {
	// ExitCode is the process exit code.
	ExitCode int
	// StderrBytes is the captured stderr output.
	StderrBytes []byte
}

// ExecutorManager manages executor process lifecycle.
type ExecutorManager struct {
	config *ExecutorConfig
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

// NewExecutorManager creates a new executor manager.
func NewExecutorManager(config *ExecutorConfig) *ExecutorManager {
	return &ExecutorManager{
		config: config,
	}
}

// executorInput is the JSON structure written to executor stdin.
type executorInput struct {
	RunID       string  `json:"run_id"`
	Attempt     int     `json:"attempt"`
	JobID       *string `json:"job_id,omitempty"`
	ParentRunID *string `json:"parent_run_id,omitempty"`
	Job         any     `json:"job"`
}

// Start starts the executor process.
// The process reads run metadata and job from stdin (JSON).
// Stdout is used for IPC frames.
// Stderr is captured for diagnostics.
func (m *ExecutorManager) Start(ctx context.Context) error {
	// Build command: quarry-executor <script-path>
	m.cmd = exec.CommandContext(ctx, m.config.ExecutorPath, m.config.ScriptPath)

	// Set up pipes
	stdin, err := m.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	m.stdin = stdin

	stdout, err := m.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	m.stdout = stdout

	stderr, err := m.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	m.stderr = stderr

	// Start process
	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start executor: %w", err)
	}

	// Write run metadata and job to stdin
	input := executorInput{
		RunID:       m.config.RunMeta.RunID,
		Attempt:     m.config.RunMeta.Attempt,
		JobID:       m.config.RunMeta.JobID,
		ParentRunID: m.config.RunMeta.ParentRunID,
		Job:         m.config.Job,
	}

	if err := json.NewEncoder(stdin).Encode(input); err != nil {
		_ = m.Kill()
		return fmt.Errorf("failed to write input: %w", err)
	}

	// Close stdin to signal input complete
	if err := stdin.Close(); err != nil {
		_ = m.Kill()
		return fmt.Errorf("failed to close stdin: %w", err)
	}

	return nil
}

// Stdout returns the stdout reader for IPC frame reading.
func (m *ExecutorManager) Stdout() io.Reader {
	return m.stdout
}

// Stderr returns the stderr reader for diagnostic capture.
func (m *ExecutorManager) Stderr() io.Reader {
	return m.stderr
}

// Wait waits for the executor to exit and returns the result.
// Must be called after Start.
func (m *ExecutorManager) Wait() (*ExecutorResult, error) {
	if m.cmd == nil {
		return nil, fmt.Errorf("executor not started")
	}

	// Read stderr (non-blocking capture)
	stderrBytes, _ := io.ReadAll(m.stderr)

	// Wait for exit
	err := m.cmd.Wait()

	result := &ExecutorResult{
		StderrBytes: stderrBytes,
	}

	// Determine exit code
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				result.ExitCode = status.ExitStatus()
			} else {
				result.ExitCode = -1
			}
		} else {
			return nil, fmt.Errorf("executor wait failed: %w", err)
		}
	} else {
		result.ExitCode = 0
	}

	return result, nil
}

// Kill terminates the executor process.
func (m *ExecutorManager) Kill() error {
	if m.cmd != nil && m.cmd.Process != nil {
		return m.cmd.Process.Kill()
	}
	return nil
}
