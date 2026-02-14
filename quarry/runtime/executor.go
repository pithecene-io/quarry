package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/pithecene-io/quarry/types"
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
	// Proxy is the optional resolved proxy endpoint per CONTRACT_PROXY.md.
	// If nil, executor launches without a proxy.
	Proxy *types.ProxyEndpoint
	// BrowserWSEndpoint is the optional WebSocket URL of an externally managed browser.
	// When set, the executor connects instead of launching a new Chromium instance.
	BrowserWSEndpoint string
	// ResolveFrom is the optional path to a node_modules directory used for
	// bare-specifier ESM resolution fallback. When set, the executor registers
	// a custom resolve hook via module.register().
	ResolveFrom string
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
	RunID       string               `json:"run_id"`
	Attempt     int                  `json:"attempt"`
	JobID       *string              `json:"job_id,omitempty"`
	ParentRunID *string              `json:"parent_run_id,omitempty"`
	Job               any                  `json:"job"`
	Proxy             *types.ProxyEndpoint `json:"proxy,omitempty"`
	BrowserWSEndpoint string               `json:"browser_ws_endpoint,omitempty"`
}

// Start starts the executor process.
// The process reads run metadata and job from stdin (JSON).
// Stdout is used for IPC frames.
// Stderr is captured for diagnostics.
func (m *ExecutorManager) Start(ctx context.Context) error {
	// Build command: quarry-executor <script-path>
	m.cmd = exec.CommandContext(ctx, m.config.ExecutorPath, m.config.ScriptPath)

	// Set module resolution env vars when --resolve-from is configured.
	// QUARRY_RESOLVE_FROM tells the executor's ESM hook where to look.
	// NODE_PATH provides CJS require() compat (ESM ignores NODE_PATH).
	if m.config.ResolveFrom != "" {
		m.cmd.Env = os.Environ()
		m.cmd.Env = append(m.cmd.Env, "QUARRY_RESOLVE_FROM="+m.config.ResolveFrom)

		// Prepend to NODE_PATH for CJS fallback
		existing := os.Getenv("NODE_PATH")
		if existing != "" {
			m.cmd.Env = append(m.cmd.Env, "NODE_PATH="+m.config.ResolveFrom+string(os.PathListSeparator)+existing)
		} else {
			m.cmd.Env = append(m.cmd.Env, "NODE_PATH="+m.config.ResolveFrom)
		}

		// Remove duplicate NODE_PATH entries from inherited env
		m.cmd.Env = deduplicateEnv(m.cmd.Env)
	}

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
		RunID:             m.config.RunMeta.RunID,
		Attempt:           m.config.RunMeta.Attempt,
		JobID:             m.config.RunMeta.JobID,
		ParentRunID:       m.config.RunMeta.ParentRunID,
		Job:               m.config.Job,
		Proxy:             m.config.Proxy,
		BrowserWSEndpoint: m.config.BrowserWSEndpoint,
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
		return nil, errors.New("executor not started")
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

// deduplicateEnv keeps the last occurrence of each env var key.
// This ensures our appended values (NODE_PATH, QUARRY_RESOLVE_FROM) win
// over inherited duplicates from os.Environ().
func deduplicateEnv(env []string) []string {
	seen := make(map[string]int, len(env))
	for i, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		seen[key] = i
	}
	result := make([]string, 0, len(seen))
	for i, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if seen[key] == i {
			result = append(result, entry)
		}
	}
	return result
}
