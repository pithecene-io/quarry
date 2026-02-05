package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

// ParityArtifact represents the CLI parity artifact structure.
type ParityArtifact struct {
	Version     string                     `json:"version"`
	Description string                     `json:"description"`
	Commands    map[string]ParityCommand   `json:"commands"`
}

// ParityCommand represents a command in the parity artifact.
type ParityCommand struct {
	Description string                     `json:"description"`
	Flags       map[string]ParityFlag      `json:"flags,omitempty"`
	Subcommands map[string]ParitySubcommand `json:"subcommands,omitempty"`
}

// ParitySubcommand represents a subcommand in the parity artifact.
type ParitySubcommand struct {
	Flags map[string]ParityFlag `json:"flags"`
}

// ParityFlag represents a flag in the parity artifact.
type ParityFlag struct {
	Type        string   `json:"type"`
	Aliases     []string `json:"aliases,omitempty"`
	Required    bool     `json:"required"`
	Default     any      `json:"default,omitempty"`
	Description string   `json:"description"`
	Validation  string   `json:"validation,omitempty"`
	ExclusiveWith []string `json:"exclusiveWith,omitempty"`
	DependsOn   []string `json:"dependsOn,omitempty"`
	Notes       string   `json:"notes,omitempty"`
}

// loadParityArtifact loads the CLI parity artifact from docs/CLI_PARITY.json.
func loadParityArtifact(t *testing.T) *ParityArtifact {
	t.Helper()

	// Find the repo root by looking for docs/CLI_PARITY.json
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file location")
	}

	// Walk up from quarry/cli/cmd to find repo root
	dir := filepath.Dir(filename)
	for i := 0; i < 5; i++ {
		candidate := filepath.Join(dir, "docs", "CLI_PARITY.json")
		if _, err := os.Stat(candidate); err == nil {
			data, err := os.ReadFile(candidate)
			if err != nil {
				t.Fatalf("failed to read parity artifact: %v", err)
			}

			var artifact ParityArtifact
			if err := json.Unmarshal(data, &artifact); err != nil {
				t.Fatalf("failed to parse parity artifact: %v", err)
			}
			return &artifact
		}
		dir = filepath.Dir(dir)
	}

	t.Fatal("could not find docs/CLI_PARITY.json - run from repo root")
	return nil
}

// extractFlags extracts flag names from a cli.Command.
func extractFlags(cmd *cli.Command) map[string]cli.Flag {
	flags := make(map[string]cli.Flag)
	for _, f := range cmd.Flags {
		names := f.Names()
		if len(names) > 0 {
			// Use the first (primary) name
			flags[names[0]] = f
		}
	}
	return flags
}

// TestCLIParityRunCommand validates the run command flags against the parity artifact.
func TestCLIParityRunCommand(t *testing.T) {
	artifact := loadParityArtifact(t)
	runCmd := RunCommand()
	actualFlags := extractFlags(runCmd)

	parityRun, ok := artifact.Commands["run"]
	if !ok {
		t.Fatal("parity artifact missing 'run' command")
	}

	// Check all parity flags exist in actual CLI
	for flagName, parityFlag := range parityRun.Flags {
		actualFlag, exists := actualFlags[flagName]
		if !exists {
			t.Errorf("parity artifact declares flag --%s but it does not exist in CLI", flagName)
			continue
		}

		// Validate flag type matches
		actualType := getFlagType(actualFlag)
		if actualType != parityFlag.Type {
			t.Errorf("flag --%s: parity says type %q but actual is %q", flagName, parityFlag.Type, actualType)
		}

		// Validate required status
		actualRequired := isFlagRequired(actualFlag)
		if actualRequired != parityFlag.Required {
			t.Errorf("flag --%s: parity says required=%v but actual is %v", flagName, parityFlag.Required, actualRequired)
		}
	}

	// Check all actual flags exist in parity artifact
	for flagName := range actualFlags {
		if _, exists := parityRun.Flags[flagName]; !exists {
			t.Errorf("CLI has flag --%s but it is not in parity artifact", flagName)
		}
	}
}

// TestCLIParityListCommand validates the list command flags against the parity artifact.
func TestCLIParityListCommand(t *testing.T) {
	artifact := loadParityArtifact(t)
	listCmd := ListCommand()

	parityList, ok := artifact.Commands["list"]
	if !ok {
		t.Fatal("parity artifact missing 'list' command")
	}

	// Check each subcommand
	for _, subCmd := range listCmd.Subcommands {
		subName := subCmd.Name
		paritySubCmd, ok := parityList.Subcommands[subName]
		if !ok {
			t.Errorf("CLI has list subcommand %q but it is not in parity artifact", subName)
			continue
		}

		actualFlags := extractFlags(subCmd)

		// Check all parity flags exist
		for flagName := range paritySubCmd.Flags {
			if _, exists := actualFlags[flagName]; !exists {
				t.Errorf("parity artifact declares flag --%s for 'list %s' but it does not exist", flagName, subName)
			}
		}

		// Check all actual flags exist in parity
		for flagName := range actualFlags {
			if _, exists := paritySubCmd.Flags[flagName]; !exists {
				t.Errorf("CLI 'list %s' has flag --%s but it is not in parity artifact", subName, flagName)
			}
		}
	}
}

// TestCLIParityDebugCommand validates the debug command flags against the parity artifact.
func TestCLIParityDebugCommand(t *testing.T) {
	artifact := loadParityArtifact(t)
	debugCmd := DebugCommand()

	parityDebug, ok := artifact.Commands["debug"]
	if !ok {
		t.Fatal("parity artifact missing 'debug' command")
	}

	// Build a map of subcommand paths (e.g., "resolve proxy", "ipc")
	for _, subCmd := range debugCmd.Subcommands {
		if len(subCmd.Subcommands) > 0 {
			// Nested subcommand (e.g., resolve -> proxy)
			for _, nestedCmd := range subCmd.Subcommands {
				subPath := subCmd.Name + " " + nestedCmd.Name
				paritySubCmd, ok := parityDebug.Subcommands[subPath]
				if !ok {
					t.Errorf("CLI has debug subcommand %q but it is not in parity artifact", subPath)
					continue
				}

				actualFlags := extractFlags(nestedCmd)

				for flagName := range paritySubCmd.Flags {
					if _, exists := actualFlags[flagName]; !exists {
						t.Errorf("parity declares flag --%s for 'debug %s' but it does not exist", flagName, subPath)
					}
				}

				for flagName := range actualFlags {
					if _, exists := paritySubCmd.Flags[flagName]; !exists {
						t.Errorf("CLI 'debug %s' has flag --%s but it is not in parity artifact", subPath, flagName)
					}
				}
			}
		} else {
			// Direct subcommand (e.g., ipc)
			subPath := subCmd.Name
			paritySubCmd, ok := parityDebug.Subcommands[subPath]
			if !ok {
				t.Errorf("CLI has debug subcommand %q but it is not in parity artifact", subPath)
				continue
			}

			actualFlags := extractFlags(subCmd)

			for flagName := range paritySubCmd.Flags {
				if _, exists := actualFlags[flagName]; !exists {
					t.Errorf("parity declares flag --%s for 'debug %s' but it does not exist", flagName, subPath)
				}
			}

			for flagName := range actualFlags {
				if _, exists := paritySubCmd.Flags[flagName]; !exists {
					t.Errorf("CLI 'debug %s' has flag --%s but it is not in parity artifact", subPath, flagName)
				}
			}
		}
	}
}

// TestCLIParityJobPayloadContract validates the job payload contract is correctly documented.
func TestCLIParityJobPayloadContract(t *testing.T) {
	artifact := loadParityArtifact(t)

	parityRun, ok := artifact.Commands["run"]
	if !ok {
		t.Fatal("parity artifact missing 'run' command")
	}

	// Validate --job flag documentation
	jobFlag, ok := parityRun.Flags["job"]
	if !ok {
		t.Fatal("parity artifact missing 'job' flag")
	}

	if !strings.Contains(strings.ToLower(jobFlag.Validation), "object") {
		t.Error("--job flag validation should mention 'object' requirement")
	}

	if len(jobFlag.ExclusiveWith) == 0 || jobFlag.ExclusiveWith[0] != "job-json" {
		t.Error("--job flag should be exclusive with --job-json")
	}

	// Validate --job-json flag documentation
	jobJsonFlag, ok := parityRun.Flags["job-json"]
	if !ok {
		t.Fatal("parity artifact missing 'job-json' flag")
	}

	if !strings.Contains(strings.ToLower(jobJsonFlag.Validation), "object") {
		t.Error("--job-json flag validation should mention 'object' requirement")
	}

	if len(jobJsonFlag.ExclusiveWith) == 0 || jobJsonFlag.ExclusiveWith[0] != "job" {
		t.Error("--job-json flag should be exclusive with --job")
	}
}

// getFlagType returns the type string for a cli.Flag.
func getFlagType(f cli.Flag) string {
	switch f.(type) {
	case *cli.StringFlag:
		return "string"
	case *cli.IntFlag:
		return "int"
	case *cli.Int64Flag:
		return "int64"
	case *cli.BoolFlag:
		return "bool"
	case *cli.Float64Flag:
		return "float64"
	case *cli.DurationFlag:
		return "duration"
	default:
		return "unknown"
	}
}

// isFlagRequired returns whether a cli.Flag is required.
func isFlagRequired(f cli.Flag) bool {
	switch tf := f.(type) {
	case *cli.StringFlag:
		return tf.Required
	case *cli.IntFlag:
		return tf.Required
	case *cli.Int64Flag:
		return tf.Required
	case *cli.BoolFlag:
		return tf.Required
	default:
		return false
	}
}
