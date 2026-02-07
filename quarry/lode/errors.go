// Package lode provides storage error classification for Quarry.
//
// This file defines sentinel errors and error wrappers for classifying
// storage failures. These enable callers to use errors.Is/errors.As
// for typed assertions rather than string matching.
package lode

import (
	"errors"
	"fmt"
	"strings"
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

// errorPattern pairs a set of message substrings with a sentinel error.
// Order matters: more-specific patterns must appear before general ones.
type errorPattern struct {
	patterns []string
	kind     error
}

// classifierTable is a declarative list of error message patterns.
// Entries are checked in order; the first match wins.
// ErrAccessDenied appears before ErrPermissionDenied so that
// "AccessDenied"/"Forbidden"/"403" is not shadowed by "access denied".
var classifierTable = []errorPattern{
	{[]string{"AccessDenied", "Forbidden", "403"}, ErrAccessDenied},
	{[]string{"permission denied", "EACCES"}, ErrPermissionDenied},
	{[]string{"no such file", "does not exist", "not found", "ENOENT", "404", "NoSuchKey"}, ErrNotFound},
	{[]string{"no space left", "disk full", "ENOSPC", "quota exceeded"}, ErrDiskFull},
	{[]string{"timeout", "timed out", "deadline exceeded"}, ErrTimeout},
	{[]string{"SlowDown", "rate exceeded", "throttl", "429", "TooManyRequests"}, ErrThrottled},
	{[]string{"NoCredentialProviders", "credentials", "InvalidAccessKeyId",
		"SignatureDoesNotMatch", "ExpiredToken", "401", "Unauthorized"}, ErrAuth},
	{[]string{"connection refused", "no route to host", "network unreachable",
		"DNS", "dial tcp", "i/o timeout"}, ErrNetwork},
}

// classifyError determines the appropriate sentinel error for the given error.
// Classification checks typed errors first, then walks the classifier table.
func classifyError(err error) error {
	if err == nil {
		return nil
	}

	// Check for typed errors first via errors.As
	var timeoutErr interface{ Timeout() bool }
	if errors.As(err, &timeoutErr) && timeoutErr.Timeout() {
		return ErrTimeout
	}

	// Walk the classifier table (first match wins)
	errStr := err.Error()
	for _, entry := range classifierTable {
		if containsAny(errStr, entry.patterns...) {
			return entry.kind
		}
	}

	return errors.New("storage error")
}

// containsAny checks if s contains any of the substrings (case-insensitive).
func containsAny(s string, substrs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
