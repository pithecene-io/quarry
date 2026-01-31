// Package cmd provides CLI commands for the quarry binary.
package cmd

import "github.com/urfave/cli/v2"

// Shared flags for read-only commands per CONTRACT_CLI.md.
var (
	// FormatFlag selects output format: json, table, yaml.
	FormatFlag = &cli.StringFlag{
		Name:    "format",
		Aliases: []string{"f"},
		Usage:   "Output format: json, table, yaml",
	}

	// NoColorFlag disables colored output.
	NoColorFlag = &cli.BoolFlag{
		Name:  "no-color",
		Usage: "Disable colored output",
	}

	// TUIFlag enables Bubble Tea interactive mode.
	// Only valid for select read-only commands (inspect, stats).
	TUIFlag = &cli.BoolFlag{
		Name:  "tui",
		Usage: "Enable interactive TUI mode (inspect, stats only)",
	}
)

// ReadOnlyFlags returns the shared flags for all read-only commands.
// Includes --tui so that unsupported commands can provide explicit error messages
// instead of generic "flag not defined" errors.
func ReadOnlyFlags() []cli.Flag {
	return []cli.Flag{
		FormatFlag,
		NoColorFlag,
		TUIFlag,
	}
}

// TUIReadOnlyFlags returns flags for commands that support TUI mode.
// This is an alias for ReadOnlyFlags, kept for documentation clarity.
func TUIReadOnlyFlags() []cli.Flag {
	return ReadOnlyFlags()
}
