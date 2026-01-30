// Package log provides structured logging with run context per CONTRACT_RUN.md.
package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/justapithecus/quarry/types"
)

// Logger provides structured logging with run context.
// All log entries include run identity fields per CONTRACT_RUN.md.
type Logger struct {
	runMeta *types.RunMeta
	output  io.Writer
}

// NewLogger creates a new logger with run context.
// Output defaults to os.Stderr.
func NewLogger(runMeta *types.RunMeta) *Logger {
	return &Logger{
		runMeta: runMeta,
		output:  os.Stderr,
	}
}

// WithOutput returns a new logger with a different output writer.
func (l *Logger) WithOutput(w io.Writer) *Logger {
	return &Logger{
		runMeta: l.runMeta,
		output:  w,
	}
}

// Entry represents a structured log entry.
type Entry struct {
	Timestamp   string         `json:"timestamp"`
	Level       string         `json:"level"`
	Message     string         `json:"message"`
	RunID       string         `json:"run_id"`
	JobID       *string        `json:"job_id,omitempty"`
	ParentRunID *string        `json:"parent_run_id,omitempty"`
	Attempt     int            `json:"attempt"`
	Fields      map[string]any `json:"fields,omitempty"`
}

// log writes a structured log entry.
func (l *Logger) log(level, message string, fields map[string]any) {
	entry := Entry{
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		Level:       level,
		Message:     message,
		RunID:       l.runMeta.RunID,
		JobID:       l.runMeta.JobID,
		ParentRunID: l.runMeta.ParentRunID,
		Attempt:     l.runMeta.Attempt,
		Fields:      fields,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		// Fallback to simple format if JSON encoding fails
		_, _ = fmt.Fprintf(l.output, "[%s] %s run_id=%s: %s\n",
			level, entry.Timestamp, l.runMeta.RunID, message)
		return
	}

	_, _ = fmt.Fprintf(l.output, "%s\n", data)
}

// Debug logs a debug message.
func (l *Logger) Debug(message string, fields map[string]any) {
	l.log("debug", message, fields)
}

// Info logs an info message.
func (l *Logger) Info(message string, fields map[string]any) {
	l.log("info", message, fields)
}

// Warn logs a warning message.
func (l *Logger) Warn(message string, fields map[string]any) {
	l.log("warn", message, fields)
}

// Error logs an error message.
func (l *Logger) Error(message string, fields map[string]any) {
	l.log("error", message, fields)
}
