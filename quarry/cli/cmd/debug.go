package cmd

import (
	"github.com/justapithecus/quarry/cli/reader"
	"github.com/justapithecus/quarry/cli/render"
	"github.com/urfave/cli/v2"
)

// DebugCommand returns the debug command with subcommands.
// Debug commands are opt-in diagnostic tools per CONTRACT_CLI.md.
// They are read-only by default; any mutation must be explicitly requested.
func DebugCommand() *cli.Command {
	return &cli.Command{
		Name:  "debug",
		Usage: "Diagnostic tools (resolve proxy, ipc)",
		Subcommands: []*cli.Command{
			debugResolveCommand(),
			debugIPCCommand(),
		},
	}
}

func debugResolveCommand() *cli.Command {
	return &cli.Command{
		Name:  "resolve",
		Usage: "Resolve entities for debugging",
		Subcommands: []*cli.Command{
			debugResolveProxyCommand(),
		},
	}
}

func debugResolveProxyCommand() *cli.Command {
	return &cli.Command{
		Name:      "proxy",
		Usage:     "Resolve a proxy endpoint from a pool",
		ArgsUsage: "<pool>",
		Flags: append(ReadOnlyFlags(),
			&cli.BoolFlag{
				Name:  "commit",
				Usage: "Commit the resolution (advance rotation counters)",
			},
		),
		Action: debugResolveProxyAction,
	}
}

func debugResolveProxyAction(c *cli.Context) error {
	if c.NArg() < 1 {
		return cli.Exit("pool name required", 1)
	}
	pool := c.Args().First()
	commit := c.Bool("commit")

	r, err := render.NewRenderer(c)
	if err != nil {
		return err
	}

	// TUI not supported for debug commands
	if c.Bool("tui") {
		return cli.Exit("--tui is not supported for debug commands", 1)
	}

	resp, err := reader.DebugResolveProxy(pool, commit)
	if err != nil {
		return err
	}

	return r.Render(resp)
}

func debugIPCCommand() *cli.Command {
	return &cli.Command{
		Name:  "ipc",
		Usage: "Show IPC debug information",
		Flags: append(ReadOnlyFlags(),
			&cli.BoolFlag{
				Name:  "verbose",
				Usage: "Include payload details",
			},
		),
		Action: debugIPCAction,
	}
}

func debugIPCAction(c *cli.Context) error {
	r, err := render.NewRenderer(c)
	if err != nil {
		return err
	}

	// TUI not supported for debug commands
	if c.Bool("tui") {
		return cli.Exit("--tui is not supported for debug commands", 1)
	}

	verbose := c.Bool("verbose")
	return r.Render(reader.DebugIPC(verbose))
}
