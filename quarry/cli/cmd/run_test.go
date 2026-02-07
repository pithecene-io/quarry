package cmd

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	quarryconfig "github.com/justapithecus/quarry/cli/config"
	"github.com/urfave/cli/v2"
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

// --- Config precedence and validation tests ---

// newTestCLIContext builds a minimal *cli.Context with the given flags set.
// flagValues maps flag names to their string values. All listed flags are
// registered and marked as explicitly set (c.IsSet returns true).
// defaultFlags maps flag names to default values (not explicitly set).
func newTestCLIContext(t *testing.T, flagValues map[string]string, defaultFlags map[string]string) *cli.Context {
	t.Helper()
	app := cli.NewApp()

	// Register all flags
	allFlags := make(map[string]string)
	for k, v := range defaultFlags {
		allFlags[k] = v
	}
	for k, v := range flagValues {
		allFlags[k] = v
	}

	var cliFlags []cli.Flag
	for name, val := range allFlags {
		cliFlags = append(cliFlags, &cli.StringFlag{Name: name, Value: val})
	}
	app.Flags = cliFlags

	// Build a flagset with only the explicitly set flags
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	for name, val := range allFlags {
		fs.String(name, val, "")
	}

	// Only set the flagValues (not defaults) so c.IsSet works
	for name, val := range flagValues {
		if err := fs.Set(name, val); err != nil {
			t.Fatalf("failed to set flag %s: %v", name, err)
		}
	}

	return cli.NewContext(app, fs, nil)
}

func TestResolveString_CLIWins(t *testing.T) {
	c := newTestCLIContext(t, map[string]string{"source": "cli-val"}, nil)
	got := resolveString(c, "source", "config-val")
	if got != "cli-val" {
		t.Errorf("expected CLI to win, got %q", got)
	}
}

func TestResolveString_ConfigFallback(t *testing.T) {
	c := newTestCLIContext(t, nil, map[string]string{"source": ""})
	got := resolveString(c, "source", "config-val")
	if got != "config-val" {
		t.Errorf("expected config fallback, got %q", got)
	}
}

func TestResolveString_UfaveDefault(t *testing.T) {
	c := newTestCLIContext(t, nil, map[string]string{"category": "default"})
	got := resolveString(c, "category", "")
	if got != "default" {
		t.Errorf("expected urfave default, got %q", got)
	}
}

func TestConfigVal_NilConfig(t *testing.T) {
	got := configVal(nil, func(c *quarryconfig.Config) string { return c.Source })
	if got != "" {
		t.Errorf("expected empty for nil config, got %q", got)
	}
}

func TestConfigVal_NonNil(t *testing.T) {
	cfg := &quarryconfig.Config{Source: "from-config"}
	got := configVal(cfg, func(c *quarryconfig.Config) string { return c.Source })
	if got != "from-config" {
		t.Errorf("expected from-config, got %q", got)
	}
}

func TestResolveInt_CLIWins(t *testing.T) {
	app := cli.NewApp()
	app.Flags = []cli.Flag{&cli.IntFlag{Name: "buffer-events"}}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Int("buffer-events", 0, "")
	_ = fs.Set("buffer-events", "500")
	c := cli.NewContext(app, fs, nil)

	got := resolveInt(c, "buffer-events", 1000)
	if got != 500 {
		t.Errorf("expected CLI to win with 500, got %d", got)
	}
}

func TestResolveInt_ConfigFallback(t *testing.T) {
	app := cli.NewApp()
	app.Flags = []cli.Flag{&cli.IntFlag{Name: "buffer-events"}}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Int("buffer-events", 0, "")
	c := cli.NewContext(app, fs, nil)

	got := resolveInt(c, "buffer-events", 1000)
	if got != 1000 {
		t.Errorf("expected config fallback 1000, got %d", got)
	}
}

func TestResolveBool_CLIWins(t *testing.T) {
	app := cli.NewApp()
	app.Flags = []cli.Flag{&cli.BoolFlag{Name: "storage-s3-path-style"}}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Bool("storage-s3-path-style", false, "")
	_ = fs.Set("storage-s3-path-style", "true")
	c := cli.NewContext(app, fs, nil)

	got := resolveBool(c, "storage-s3-path-style", false)
	if !got {
		t.Error("expected CLI true to win")
	}
}

func TestResolveDuration_CLIWins(t *testing.T) {
	app := cli.NewApp()
	app.Flags = []cli.Flag{&cli.DurationFlag{Name: "adapter-timeout"}}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Duration("adapter-timeout", 0, "")
	_ = fs.Set("adapter-timeout", "30s")
	c := cli.NewContext(app, fs, nil)

	got := resolveDuration(c, "adapter-timeout", 10*time.Second)
	if got != 30*time.Second {
		t.Errorf("expected CLI 30s to win, got %v", got)
	}
}

func TestResolveDuration_ConfigFallback(t *testing.T) {
	app := cli.NewApp()
	app.Flags = []cli.Flag{&cli.DurationFlag{Name: "adapter-timeout"}}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Duration("adapter-timeout", 0, "")
	c := cli.NewContext(app, fs, nil)

	got := resolveDuration(c, "adapter-timeout", 10*time.Second)
	if got != 10*time.Second {
		t.Errorf("expected config fallback 10s, got %v", got)
	}
}

// newTestApp creates a cli.App with RunCommand wired up and ExitErrHandler
// suppressed so errors are returned instead of calling os.Exit.
func newTestApp() *cli.App {
	app := cli.NewApp()
	cmd := RunCommand()
	app.Commands = []*cli.Command{cmd}
	app.ExitErrHandler = func(c *cli.Context, err error) {} // suppress os.Exit
	return app
}

// TestRunAction_MissingSource validates that runAction returns actionable error
// when source is missing from both CLI and config.
func TestRunAction_MissingSource(t *testing.T) {
	app := newTestApp()

	// Invoke with script and run-id but no source, no config
	err := app.Run([]string{"quarry", "run",
		"--script", "./test.ts",
		"--run-id", "run-001",
	})
	if err == nil {
		t.Fatal("expected error for missing source")
	}
	if !strings.Contains(err.Error(), "--source is required") {
		t.Errorf("error should mention --source is required, got: %v", err)
	}
}

// TestRunAction_MissingStorageBackend validates that runAction returns
// actionable error when storage-backend is missing.
func TestRunAction_MissingStorageBackend(t *testing.T) {
	app := newTestApp()

	err := app.Run([]string{"quarry", "run",
		"--script", "./test.ts",
		"--run-id", "run-001",
		"--source", "test",
	})
	if err == nil {
		t.Fatal("expected error for missing storage-backend")
	}
	if !strings.Contains(err.Error(), "--storage-backend is required") {
		t.Errorf("error should mention --storage-backend is required, got: %v", err)
	}
}

// TestRunAction_MissingStoragePath validates that runAction returns
// actionable error when storage-path is missing.
func TestRunAction_MissingStoragePath(t *testing.T) {
	app := newTestApp()

	err := app.Run([]string{"quarry", "run",
		"--script", "./test.ts",
		"--run-id", "run-001",
		"--source", "test",
		"--storage-backend", "fs",
	})
	if err == nil {
		t.Fatal("expected error for missing storage-path")
	}
	if !strings.Contains(err.Error(), "--storage-path is required") {
		t.Errorf("error should mention --storage-path is required, got: %v", err)
	}
}

// TestRunAction_ConfigProvidesRequiredFields validates that a config file
// can satisfy source, storage-backend, storage-path requirements.
func TestRunAction_ConfigProvidesRequiredFields(t *testing.T) {
	// Create a config file with required fields
	dir := t.TempDir()
	storageDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "quarry.yaml")
	configContent := "source: test-source\nstorage:\n  backend: fs\n  path: " + storageDir + "\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	app := newTestApp()

	// Run with --config providing the required fields. Execution will fail
	// at executor resolution but that's past the validation we're testing.
	err := app.Run([]string{"quarry", "run",
		"--config", configPath,
		"--script", "./test.ts",
		"--run-id", "run-001",
	})
	if err == nil {
		t.Skip("executor found; cannot test validation-only path")
	}
	// Should NOT be a missing-required-field error
	errMsg := err.Error()
	if strings.Contains(errMsg, "--source is required") {
		t.Error("source should be satisfied by config file")
	}
	if strings.Contains(errMsg, "--storage-backend is required") {
		t.Error("storage-backend should be satisfied by config file")
	}
	if strings.Contains(errMsg, "--storage-path is required") {
		t.Error("storage-path should be satisfied by config file")
	}
}

// TestRunAction_CLIOverridesConfig validates that CLI flags take precedence
// over config file values.
func TestRunAction_CLIOverridesConfig(t *testing.T) {
	dir := t.TempDir()
	storageDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "quarry.yaml")
	configContent := "source: config-source\nstorage:\n  backend: fs\n  path: " + storageDir + "\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	app := newTestApp()

	// Pass --source on CLI (should override config-source).
	// The run will fail at executor resolution but that's past the validation gate.
	err := app.Run([]string{"quarry", "run",
		"--config", configPath,
		"--script", "./test.ts",
		"--run-id", "run-001",
		"--source", "cli-source",
	})
	if err == nil {
		t.Skip("executor found; cannot test validation-only path")
	}
	// Should NOT fail on source validation
	if strings.Contains(err.Error(), "--source is required") {
		t.Error("CLI --source should override config")
	}
}

// TestRunAction_ConfigFileNotFound validates actionable error for bad --config path.
func TestRunAction_ConfigFileNotFound(t *testing.T) {
	app := newTestApp()

	err := app.Run([]string{"quarry", "run",
		"--config", "/nonexistent/quarry.yaml",
		"--script", "./test.ts",
		"--run-id", "run-001",
	})
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Errorf("error should mention config file not found, got: %v", err)
	}
}

// TestRunAction_ProxyConflict validates that --proxy-config and config
// proxies: together is an error.
func TestRunAction_ProxyConflict(t *testing.T) {
	dir := t.TempDir()
	storageDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(dir, "quarry.yaml")
	configContent := `source: test
storage:
  backend: fs
  path: ` + storageDir + `
proxies:
  pool_a:
    strategy: round_robin
    endpoints:
      - protocol: http
        host: proxy.example.com
        port: 8080
proxy:
  pool: pool_a
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Also create a dummy proxy config JSON
	proxyConfigPath := filepath.Join(dir, "proxies.json")
	if err := os.WriteFile(proxyConfigPath, []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := newTestApp()

	err := app.Run([]string{"quarry", "run",
		"--config", configPath,
		"--script", "./test.ts",
		"--run-id", "run-001",
		"--proxy-config", proxyConfigPath,
	})
	if err == nil {
		t.Fatal("expected error for proxy config conflict")
	}
	if !strings.Contains(err.Error(), "cannot use --proxy-config and config file proxies") {
		t.Errorf("error should mention conflict, got: %v", err)
	}
}
