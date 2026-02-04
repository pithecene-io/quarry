// Package lode provides storage error classification for Quarry.
//
// This file defines sentinel errors and error wrappers for classifying
// storage failures. These enable callers to use errors.Is/errors.As
// for typed assertions rather than string matching.
package lode

import (
	"errors"
	"fmt"
)

// Sentinel errors for storage failure classification.
// Use errors.Is(err, ErrXxx) for typed assertions.
var (
	// ErrPermissionDenied indicates a permission/access failure (EACCES, 403).
	ErrPermissionDenied = errors.New("permission denied")

	// ErrNotFound indicates the target path/resource does not exist (ENOENT, 404).
	ErrNotFound = errors.New("not found")

	// ErrDiskFull indicates storage is out of space (ENOSPC).
	ErrDiskFull = errors.New("no space left on device")

	// ErrTimeout indicates an operation timed out.
	ErrTimeout = errors.New("operation timed out")

	// ErrThrottled indicates rate limiting (429, SlowDown).
	ErrThrottled = errors.New("rate limited")

	// ErrAuth indicates authentication failure (no credentials, expired token).
	ErrAuth = errors.New("authentication failed")

	// ErrAccessDenied indicates authorization failure (valid creds but no permission).
	ErrAccessDenied = errors.New("access denied")

	// ErrNetwork indicates a network-level failure (connection refused, DNS).
	ErrNetwork = errors.New("network error")
)

// StorageError wraps an underlying error with storage classification.
// It preserves the original error in the chain for inspection via errors.As.
type StorageError struct {
	// Kind is the sentinel error for classification (e.g., ErrPermissionDenied).
	Kind error
	// Op is the operation that failed (e.g., "write", "read", "list").
	Op string
	// Path is the storage path involved, if any.
	Path string
	// Err is the underlying error.
	Err error
}

func (e *StorageError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s %s: %v: %v", e.Op, e.Path, e.Kind, e.Err)
	}
	return fmt.Sprintf("%s: %v: %v", e.Op, e.Kind, e.Err)
}

// Unwrap returns the underlying error for errors.Is/As chain traversal.
func (e *StorageError) Unwrap() error {
	return e.Err
}

// Is reports whether the error matches the target sentinel.
func (e *StorageError) Is(target error) bool {
	return errors.Is(e.Kind, target)
}

// NewStorageError creates a classified storage error.
func NewStorageError(kind error, op, path string, err error) *StorageError {
	return &StorageError{
		Kind: kind,
		Op:   op,
		Path: path,
		Err:  err,
	}
}

// WrapWriteError classifies and wraps a write operation error.
// Returns nil if err is nil.
func WrapWriteError(err error, path string) error {
	if err == nil {
		return nil
	}
	kind := classifyError(err)
	return NewStorageError(kind, "write", path, err)
}

// WrapReadError classifies and wraps a read operation error.
// Returns nil if err is nil.
func WrapReadError(err error, path string) error {
	if err == nil {
		return nil
	}
	kind := classifyError(err)
	return NewStorageError(kind, "read", path, err)
}

// WrapInitError classifies and wraps a client initialization error.
// Returns nil if err is nil.
func WrapInitError(err error, dataset string) error {
	if err == nil {
		return nil
	}
	kind := classifyError(err)
	return NewStorageError(kind, "init", dataset, err)
}

// classifyError determines the appropriate sentinel error for the given error.
// Classification is based on error type and message patterns.
func classifyError(err error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Check for typed errors first via errors.As
	var timeoutErr interface{ Timeout() bool }
	if errors.As(err, &timeoutErr) && timeoutErr.Timeout() {
		return ErrTimeout
	}

	// Classify by error message patterns
	switch {
	// Permission/access errors
	case containsAny(errStr, "permission denied", "EACCES", "access denied"):
		// Distinguish auth vs access denied
		if containsAny(errStr, "AccessDenied", "Forbidden", "403") {
			return ErrAccessDenied
		}
		return ErrPermissionDenied

	// Not found errors
	case containsAny(errStr, "no such file", "does not exist", "not found", "ENOENT", "404", "NoSuchKey"):
		return ErrNotFound

	// Disk full errors
	case containsAny(errStr, "no space left", "disk full", "ENOSPC", "quota exceeded"):
		return ErrDiskFull

	// Timeout errors
	case containsAny(errStr, "timeout", "timed out", "deadline exceeded"):
		return ErrTimeout

	// Throttling errors
	case containsAny(errStr, "SlowDown", "rate exceeded", "throttl", "429", "TooManyRequests"):
		return ErrThrottled

	// Auth errors
	case containsAny(errStr, "NoCredentialProviders", "credentials", "InvalidAccessKeyId",
		"SignatureDoesNotMatch", "ExpiredToken", "401", "Unauthorized"):
		return ErrAuth

	// Access denied (AWS-specific)
	case containsAny(errStr, "AccessDenied", "Forbidden", "403"):
		return ErrAccessDenied

	// Network errors
	case containsAny(errStr, "connection refused", "no route to host", "network unreachable",
		"DNS", "dial tcp", "i/o timeout"):
		return ErrNetwork

	default:
		// Return a generic wrapped error for unclassified errors
		return errors.New("storage error")
	}
}

// containsAny checks if s contains any of the substrings (case-insensitive).
func containsAny(s string, substrs ...string) bool {
	lower := toLower(s)
	for _, sub := range substrs {
		if contains(lower, toLower(sub)) {
			return true
		}
	}
	return false
}

// toLower is a simple ASCII lowercase function to avoid importing strings.
func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(substr) <= len(s) && indexSubstr(s, substr) >= 0
}

// indexSubstr finds the first occurrence of substr in s.
func indexSubstr(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
