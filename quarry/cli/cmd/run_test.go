package cmd

import (
	"strings"
	"testing"
)

func TestValidatePolicyConfig(t *testing.T) {
	tests := []struct {
		name        string
		choice      policyChoice
		wantErr     bool
		errContains string
	}{
		{
			name:    "strict policy valid",
			choice:  policyChoice{name: "strict", flushMode: "at_least_once"},
			wantErr: false,
		},
		{
			name:    "buffered with events limit valid",
			choice:  policyChoice{name: "buffered", flushMode: "at_least_once", maxEvents: 1000},
			wantErr: false,
		},
		{
			name:    "buffered with bytes limit valid",
			choice:  policyChoice{name: "buffered", flushMode: "at_least_once", maxBytes: 1048576},
			wantErr: false,
		},
		{
			name:        "buffered without limits invalid",
			choice:      policyChoice{name: "buffered", flushMode: "at_least_once"},
			wantErr:     true,
			errContains: "buffer limits",
		},
		{
			name:        "invalid policy name",
			choice:      policyChoice{name: "invalid"},
			wantErr:     true,
			errContains: "invalid --policy",
		},
		{
			name:        "invalid flush mode",
			choice:      policyChoice{name: "buffered", flushMode: "invalid", maxEvents: 100},
			wantErr:     true,
			errContains: "invalid --flush-mode",
		},
		{
			name:    "buffered with chunks_first valid",
			choice:  policyChoice{name: "buffered", flushMode: "chunks_first", maxEvents: 100},
			wantErr: false,
		},
		{
			name:    "buffered with two_phase valid",
			choice:  policyChoice{name: "buffered", flushMode: "two_phase", maxBytes: 1000},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePolicyConfig(tt.choice)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateStorageConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      storageChoice
		wantErr     bool
		errContains string
	}{
		{
			name:    "fs with valid directory",
			config:  storageChoice{backend: "fs", path: "/tmp"},
			wantErr: false,
		},
		{
			name:        "fs with nonexistent path",
			config:      storageChoice{backend: "fs", path: "/nonexistent/path/that/does/not/exist"},
			wantErr:     true,
			errContains: "does not exist",
		},
		{
			name:        "fs with file instead of directory",
			config:      storageChoice{backend: "fs", path: "/etc/passwd"},
			wantErr:     true,
			errContains: "not a directory",
		},
		{
			name:    "s3 with path",
			config:  storageChoice{backend: "s3", path: "my-bucket/prefix"},
			wantErr: false,
		},
		{
			name:        "s3 without path",
			config:      storageChoice{backend: "s3", path: ""},
			wantErr:     true,
			errContains: "--storage-path required",
		},
		{
			name:        "invalid backend",
			config:      storageChoice{backend: "invalid", path: "/tmp"},
			wantErr:     true,
			errContains: "invalid --storage-backend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStorageConfig(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestErrorMessagesAreActionable(t *testing.T) {
	// Test that error messages include actionable guidance
	tests := []struct {
		name           string
		config         storageChoice
		mustContain    []string
		description    string
	}{
		{
			name:        "nonexistent path suggests mkdir",
			config:      storageChoice{backend: "fs", path: "/nonexistent/test/path"},
			mustContain: []string{"mkdir -p"},
			description: "should suggest creating directory",
		},
		{
			name:        "s3 missing path explains format",
			config:      storageChoice{backend: "s3", path: ""},
			mustContain: []string{"bucket-name", "Format:"},
			description: "should explain S3 path format",
		},
		{
			name:        "invalid backend lists options",
			config:      storageChoice{backend: "gcs", path: "/tmp"},
			mustContain: []string{"fs", "s3", "Valid options"},
			description: "should list valid storage backends",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStorageConfig(tt.config)
			if err == nil {
				t.Fatal("expected error")
			}
			errMsg := err.Error()
			for _, must := range tt.mustContain {
				if !strings.Contains(errMsg, must) {
					t.Errorf("%s: error message should contain %q for actionability\nGot: %s",
						tt.description, must, errMsg)
				}
			}
		})
	}
}

func TestPolicyErrorMessagesAreActionable(t *testing.T) {
	tests := []struct {
		name        string
		choice      policyChoice
		mustContain []string
		description string
	}{
		{
			name:        "buffered without limits suggests flags",
			choice:      policyChoice{name: "buffered", flushMode: "at_least_once"},
			mustContain: []string{"--buffer-events", "--buffer-bytes"},
			description: "should suggest buffer limit flags",
		},
		{
			name:        "invalid policy lists options",
			choice:      policyChoice{name: "unknown"},
			mustContain: []string{"strict", "buffered", "Valid options"},
			description: "should list valid policies",
		},
		{
			name:        "invalid flush mode lists options",
			choice:      policyChoice{name: "buffered", flushMode: "unknown", maxEvents: 100},
			mustContain: []string{"at_least_once", "chunks_first", "two_phase"},
			description: "should list valid flush modes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePolicyConfig(tt.choice)
			if err == nil {
				t.Fatal("expected error")
			}
			errMsg := err.Error()
			for _, must := range tt.mustContain {
				if !strings.Contains(errMsg, must) {
					t.Errorf("%s: error message should contain %q for actionability\nGot: %s",
						tt.description, must, errMsg)
				}
			}
		})
	}
}
