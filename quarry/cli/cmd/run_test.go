package cmd

import (
	"os"
	"path/filepath"
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

func TestResolveExecutor(t *testing.T) {
	// Create a temp file to use as mock executor
	tmpDir := t.TempDir()
	mockExecutor := filepath.Join(tmpDir, "mock-executor.js")
	if err := os.WriteFile(mockExecutor, []byte("// mock"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Run("explicit path found", func(t *testing.T) {
		path, err := resolveExecutor(mockExecutor)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if path != mockExecutor {
			t.Errorf("got path %q, want %q", path, mockExecutor)
		}
	})

	t.Run("explicit path not found", func(t *testing.T) {
		_, err := resolveExecutor("/nonexistent/executor.js")
		if err == nil {
			t.Error("expected error, got nil")
		} else if !strings.Contains(err.Error(), "executor not found") {
			t.Errorf("error %q should contain %q", err.Error(), "executor not found")
		}
	})

	t.Run("auto-resolution returns valid path or error", func(t *testing.T) {
		// This test verifies the auto-resolution behavior without being environment-dependent.
		// It succeeds if either: (1) a valid path is returned, or (2) an actionable error is returned.
		path, err := resolveExecutor("")
		if err != nil {
			// Verify error is actionable
			if !strings.Contains(err.Error(), "executor not found") {
				t.Errorf("error should mention 'executor not found', got: %v", err)
			}
			if !strings.Contains(err.Error(), "pnpm") {
				t.Errorf("error should include build instructions, got: %v", err)
			}
		} else {
			// Verify returned path exists
			if _, statErr := os.Stat(path); statErr != nil {
				t.Errorf("resolved path %q does not exist: %v", path, statErr)
			}
		}
	})
}

func TestResolveExecutorErrorIsActionable(t *testing.T) {
	// Test that executor not found error includes actionable guidance
	_, err := resolveExecutor("")
	if err == nil {
		t.Skip("executor found in environment, cannot test not-found error")
	}

	errMsg := err.Error()
	mustContain := []string{
		"executor not found",
		"pnpm",        // build instruction
		"--executor",  // manual override option
	}

	for _, must := range mustContain {
		if !strings.Contains(errMsg, must) {
			t.Errorf("error should contain %q for actionability\nGot: %s", must, errMsg)
		}
	}
}

func TestParseJobPayload(t *testing.T) {
	t.Run("neither flag returns empty object", func(t *testing.T) {
		job, err := parseJobPayload("", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(job) != 0 {
			t.Errorf("expected empty map, got %v", job)
		}
	})

	t.Run("inline object accepted", func(t *testing.T) {
		job, err := parseJobPayload(`{"url": "https://example.com", "page": 1}`, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if job["url"] != "https://example.com" {
			t.Errorf("expected url=https://example.com, got %v", job["url"])
		}
	})

	t.Run("file object accepted", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "job.json")
		if err := os.WriteFile(tmpFile, []byte(`{"target": "test"}`), 0644); err != nil {
			t.Fatal(err)
		}

		job, err := parseJobPayload("", tmpFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if job["target"] != "test" {
			t.Errorf("expected target=test, got %v", job["target"])
		}
	})

	t.Run("inline array rejected", func(t *testing.T) {
		_, err := parseJobPayload(`[1, 2, 3]`, "")
		if err == nil {
			t.Fatal("expected error for array payload")
		}
		if !strings.Contains(err.Error(), "must be a JSON object") {
			t.Errorf("error should mention 'must be a JSON object', got: %v", err)
		}
		if !strings.Contains(err.Error(), "array") {
			t.Errorf("error should mention 'array', got: %v", err)
		}
	})

	t.Run("file array rejected", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "array.json")
		if err := os.WriteFile(tmpFile, []byte(`["item1", "item2"]`), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := parseJobPayload("", tmpFile)
		if err == nil {
			t.Fatal("expected error for array payload")
		}
		if !strings.Contains(err.Error(), "must contain a JSON object") {
			t.Errorf("error should mention 'must contain a JSON object', got: %v", err)
		}
		if !strings.Contains(err.Error(), "array") {
			t.Errorf("error should mention 'array', got: %v", err)
		}
	})

	t.Run("inline primitive string rejected", func(t *testing.T) {
		_, err := parseJobPayload(`"just a string"`, "")
		if err == nil {
			t.Fatal("expected error for primitive payload")
		}
		if !strings.Contains(err.Error(), "must be a JSON object") {
			t.Errorf("error should mention 'must be a JSON object', got: %v", err)
		}
		if !strings.Contains(err.Error(), "string") {
			t.Errorf("error should mention 'string', got: %v", err)
		}
	})

	t.Run("inline primitive number rejected", func(t *testing.T) {
		_, err := parseJobPayload(`42`, "")
		if err == nil {
			t.Fatal("expected error for primitive payload")
		}
		if !strings.Contains(err.Error(), "must be a JSON object") {
			t.Errorf("error should mention 'must be a JSON object', got: %v", err)
		}
		if !strings.Contains(err.Error(), "number") {
			t.Errorf("error should mention 'number', got: %v", err)
		}
	})

	t.Run("file primitive rejected", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "primitive.json")
		if err := os.WriteFile(tmpFile, []byte(`"hello"`), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := parseJobPayload("", tmpFile)
		if err == nil {
			t.Fatal("expected error for primitive payload")
		}
		if !strings.Contains(err.Error(), "must contain a JSON object") {
			t.Errorf("error should mention 'must contain a JSON object', got: %v", err)
		}
	})

	t.Run("inline null rejected", func(t *testing.T) {
		_, err := parseJobPayload(`null`, "")
		if err == nil {
			t.Fatal("expected error for null payload")
		}
		if !strings.Contains(err.Error(), "must be a JSON object") {
			t.Errorf("error should mention 'must be a JSON object', got: %v", err)
		}
		if !strings.Contains(err.Error(), "null") {
			t.Errorf("error should mention 'null', got: %v", err)
		}
	})

	t.Run("file null rejected", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "null.json")
		if err := os.WriteFile(tmpFile, []byte(`null`), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := parseJobPayload("", tmpFile)
		if err == nil {
			t.Fatal("expected error for null payload")
		}
		if !strings.Contains(err.Error(), "must contain a JSON object") {
			t.Errorf("error should mention 'must contain a JSON object', got: %v", err)
		}
	})

	t.Run("malformed inline JSON rejected", func(t *testing.T) {
		_, err := parseJobPayload(`{invalid}`, "")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "malformed --job JSON") {
			t.Errorf("error should mention 'malformed --job JSON', got: %v", err)
		}
	})

	t.Run("malformed file JSON rejected", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "bad.json")
		if err := os.WriteFile(tmpFile, []byte(`{not valid json}`), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := parseJobPayload("", tmpFile)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "malformed JSON in job file") {
			t.Errorf("error should mention 'malformed JSON in job file', got: %v", err)
		}
	})

	t.Run("file not found error", func(t *testing.T) {
		_, err := parseJobPayload("", "/nonexistent/job.json")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "job file not found") {
			t.Errorf("error should mention 'job file not found', got: %v", err)
		}
	})

	t.Run("both flags set rejected", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "job.json")
		if err := os.WriteFile(tmpFile, []byte(`{}`), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := parseJobPayload(`{"inline": true}`, tmpFile)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "cannot use both --job and --job-json") {
			t.Errorf("error should mention conflict, got: %v", err)
		}
	})
}

func TestParseJobPayloadErrorsAreActionable(t *testing.T) {
	tests := []struct {
		name        string
		jobInline   string
		jobFile     string
		mustContain []string
		description string
	}{
		{
			name:        "conflict lists both options",
			jobInline:   `{}`,
			jobFile:     "/some/file.json",
			mustContain: []string{"--job", "--job-json", "ONE of"},
			description: "should list both options and clarify to use one",
		},
		{
			name:        "file not found suggests ls",
			jobInline:   "",
			jobFile:     "/nonexistent/payload.json",
			mustContain: []string{"not found", "ls -la"},
			description: "should suggest checking file existence",
		},
		{
			name:        "malformed inline shows examples",
			jobInline:   `{broken`,
			jobFile:     "",
			mustContain: []string{"--job '{}'", `{"key": "value"}`},
			description: "should show valid JSON examples",
		},
		{
			name:        "array inline shows object requirement",
			jobInline:   `[]`,
			jobFile:     "",
			mustContain: []string{"must be a JSON object", "array", "not an array"},
			description: "should explain object-only requirement",
		},
		{
			name:        "null inline shows object requirement",
			jobInline:   `null`,
			jobFile:     "",
			mustContain: []string{"must be a JSON object", "null"},
			description: "should explain object-only requirement for null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseJobPayload(tt.jobInline, tt.jobFile)
			if err == nil {
				t.Fatal("expected error")
			}
			errMsg := err.Error()
			for _, must := range tt.mustContain {
				if !strings.Contains(errMsg, must) {
					t.Errorf("%s: error should contain %q\nGot: %s", tt.description, must, errMsg)
				}
			}
		})
	}
}

func TestDescribeJSONType(t *testing.T) {
	tests := []struct {
		input    any
		contains string
	}{
		{nil, "null"},
		{map[string]any{}, "object"},
		{[]any{1, 2}, "array"},
		{"hello", "string"},
		{float64(42), "number"},
		{true, "boolean"},
	}

	for _, tt := range tests {
		t.Run(tt.contains, func(t *testing.T) {
			result := describeJSONType(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("describeJSONType(%v) = %q, should contain %q", tt.input, result, tt.contains)
			}
		})
	}
}

func TestExitCodeConstants(t *testing.T) {
	// Verify exit code semantics
	if exitConfigError != exitExecutorCrash {
		t.Error("exitConfigError should map to exitExecutorCrash (non-script error)")
	}
	if exitScriptError == exitConfigError {
		t.Error("exitScriptError should differ from exitConfigError")
	}
}
