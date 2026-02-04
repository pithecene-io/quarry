package cmd

import (
	"github.com/urfave/cli/v2"

	"github.com/justapithecus/quarry/cli/reader"
	"github.com/justapithecus/quarry/cli/render"
)

// StatsCommand returns the stats command with subcommands.
// Stats returns aggregated, derived facts per CONTRACT_CLI.md.
func StatsCommand() *cli.Command {
	return &cli.Command{
		Name:  "stats",
		Usage: "Show aggregated statistics (runs, jobs, tasks, proxies, executors)",
		Subcommands: []*cli.Command{
			statsRunsCommand(),
			statsJobsCommand(),
			statsTasksCommand(),
			statsProxiesCommand(),
			statsExecutorsCommand(),
		},
	}
}

func statsRunsCommand() *cli.Command {
	return &cli.Command{
		Name:   "runs",
		Usage:  "Show run statistics",
		Flags:  TUIReadOnlyFlags(),
		Action: statsRunsAction,
	}
}

func statsRunsAction(c *cli.Context) error {
	r, err := render.NewRenderer(c)
	if err != nil {
		return err
	}

	if c.Bool("tui") {
		return r.RenderTUI("stats_runs", reader.StatsRuns())
	}

	return r.Render(reader.StatsRuns())
}

func statsJobsCommand() *cli.Command {
	return &cli.Command{
		Name:   "jobs",
		Usage:  "Show job statistics",
		Flags:  TUIReadOnlyFlags(),
		Action: statsJobsAction,
	}
}

func statsJobsAction(c *cli.Context) error {
	r, err := render.NewRenderer(c)
	if err != nil {
		return err
	}

	if c.Bool("tui") {
		return r.RenderTUI("stats_jobs", reader.StatsJobs())
	}

	return r.Render(reader.StatsJobs())
}

func statsTasksCommand() *cli.Command {
	return &cli.Command{
		Name:   "tasks",
		Usage:  "Show task statistics",
		Flags:  TUIReadOnlyFlags(),
		Action: statsTasksAction,
	}
}

func statsTasksAction(c *cli.Context) error {
	r, err := render.NewRenderer(c)
	if err != nil {
		return err
	}

	if c.Bool("tui") {
		return r.RenderTUI("stats_tasks", reader.StatsTasks())
	}

	return r.Render(reader.StatsTasks())
}

func statsProxiesCommand() *cli.Command {
	return &cli.Command{
		Name:   "proxies",
		Usage:  "Show proxy statistics",
		Flags:  TUIReadOnlyFlags(),
		Action: statsProxiesAction,
	}
}

func statsProxiesAction(c *cli.Context) error {
	r, err := render.NewRenderer(c)
	if err != nil {
		return err
	}

	if c.Bool("tui") {
		return r.RenderTUI("stats_proxies", reader.StatsProxies())
	}

	return r.Render(reader.StatsProxies())
}

func statsExecutorsCommand() *cli.Command {
	return &cli.Command{
		Name:   "executors",
		Usage:  "Show executor statistics",
		Flags:  TUIReadOnlyFlags(),
		Action: statsExecutorsAction,
	}
}

func statsExecutorsAction(c *cli.Context) error {
	r, err := render.NewRenderer(c)
	if err != nil {
		return err
	}

	if c.Bool("tui") {
		return r.RenderTUI("stats_executors", reader.StatsExecutors())
	}

	return r.Render(reader.StatsExecutors())
}
