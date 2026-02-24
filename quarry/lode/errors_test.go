package lode

import (
	"errors"
	"testing"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		wantKind error
	}{
		// write_timeout / timeout
		{
			name:     "context deadline exceeded",
			errMsg:   "context deadline exceeded",
			wantKind: ErrTimeout,
		},
		{
			name:     "operation timed out",
			errMsg:   "operation timed out",
			wantKind: ErrTimeout,
		},
		{
			name:     "timeout in message",
			errMsg:   "connection timeout after 30s",
			wantKind: ErrTimeout,
		},

		// access_denied (before permission_denied in table)
		{
			name:     "AccessDenied response",
			errMsg:   "AccessDenied: you do not have access",
			wantKind: ErrAccessDenied,
		},
		{
			name:     "Forbidden response",
			errMsg:   "Forbidden",
			wantKind: ErrAccessDenied,
		},
		{
			name:     "HTTP 403",
			errMsg:   "received status 403",
			wantKind: ErrAccessDenied,
		},

		// permission_denied
		{
			name:     "permission denied",
			errMsg:   "permission denied for /data/output",
			wantKind: ErrPermissionDenied,
		},
		{
			name:     "EACCES errno",
			errMsg:   "open /tmp/file: EACCES",
			wantKind: ErrPermissionDenied,
		},

		// disk_full
		{
			name:     "no space left on device",
			errMsg:   "write /data/output: no space left on device",
			wantKind: ErrDiskFull,
		},
		{
			name:     "ENOSPC errno",
			errMsg:   "ENOSPC: write failed",
			wantKind: ErrDiskFull,
		},
		{
			name:     "disk full",
			errMsg:   "disk full, cannot write",
			wantKind: ErrDiskFull,
		},
		{
			name:     "quota exceeded",
			errMsg:   "quota exceeded for user",
			wantKind: ErrDiskFull,
		},

		// not_found
		{
			name:     "no such file",
			errMsg:   "no such file or directory",
			wantKind: ErrNotFound,
		},
		{
			name:     "ENOENT errno",
			errMsg:   "open /missing: ENOENT",
			wantKind: ErrNotFound,
		},
		{
			name:     "NoSuchKey S3",
			errMsg:   "NoSuchKey: The specified key does not exist",
			wantKind: ErrNotFound,
		},
		{
			name:     "HTTP 404",
			errMsg:   "received status 404",
			wantKind: ErrNotFound,
		},

		// rate_limited
		{
			name:     "HTTP 429",
			errMsg:   "received status 429",
			wantKind: ErrThrottled,
		},
		{
			name:     "SlowDown S3",
			errMsg:   "SlowDown: please reduce request rate",
			wantKind: ErrThrottled,
		},
		{
			name:     "TooManyRequests",
			errMsg:   "TooManyRequests: rate limit exceeded",
			wantKind: ErrThrottled,
		},
		{
			name:     "throttled message",
			errMsg:   "request was throttled",
			wantKind: ErrThrottled,
		},

		// auth
		{
			name:     "NoCredentialProviders",
			errMsg:   "NoCredentialProviders: no valid credential providers",
			wantKind: ErrAuth,
		},
		{
			name:     "ExpiredToken",
			errMsg:   "ExpiredToken: the security token has expired",
			wantKind: ErrAuth,
		},
		{
			name:     "HTTP 401",
			errMsg:   "received status 401",
			wantKind: ErrAuth,
		},

		// network
		{
			name:     "connection refused",
			errMsg:   "dial tcp 127.0.0.1:9000: connection refused",
			wantKind: ErrNetwork,
		},
		{
			name:     "no route to host",
			errMsg:   "no route to host",
			wantKind: ErrNetwork,
		},
		{
			name:     "DNS resolution failure",
			errMsg:   "DNS lookup failed for bucket.s3.amazonaws.com",
			wantKind: ErrNetwork,
		},

		// unknown (fallback)
		{
			name:   "unrecognized error",
			errMsg: "something completely unexpected happened",
			// classifyError returns a new errors.New("storage error") for unknown
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errMsg)
			got := classifyError(err)

			if tt.wantKind != nil {
				if !errors.Is(got, tt.wantKind) {
					t.Errorf("classifyError(%q) = %v, want %v", tt.errMsg, got, tt.wantKind)
				}
			} else {
				// Fallback: should return an error with "storage error" message
				if got == nil {
					t.Errorf("classifyError(%q) = nil, want non-nil fallback", tt.errMsg)
				} else if got.Error() != "storage error" {
					t.Errorf("classifyError(%q) = %q, want %q", tt.errMsg, got.Error(), "storage error")
				}
			}
		})
	}
}

func TestClassifyError_Nil(t *testing.T) {
	got := classifyError(nil)
	if got != nil {
		t.Errorf("classifyError(nil) = %v, want nil", got)
	}
}
