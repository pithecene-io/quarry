package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/justapithecus/quarry/lode"
	"github.com/justapithecus/quarry/policy"
	"github.com/justapithecus/quarry/proxy"
	"github.com/justapithecus/quarry/runtime"
	"github.com/justapithecus/quarry/types"
	"github.com/urfave/cli/v2"
)

// Exit codes per CONTRACT_RUN.md.
const (
	exitSuccess       = 0
	exitScriptError   = 1
	exitExecutorCrash = 2
	exitPolicyFailure = 3
)

// RunCommand returns the run command.
// This is the only command that executes work per CONTRACT_CLI.md.
func RunCommand() *cli.Command {
	return &cli.Command{
		Name:  "run",
		Usage: "Execute a script run (the only execution entrypoint)",
		Flags: []cli.Flag{
			// Execution flags
			&cli.StringFlag{
				Name:     "script",
				Usage:    "Path to script file",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "run-id",
				Usage:    "Run ID",
				Required: true,
			},
			&cli.IntFlag{
				Name:  "attempt",
				Usage: "Attempt number (starts at 1)",
				Value: 1,
			},
			&cli.StringFlag{
				Name:  "job-id",
				Usage: "Job ID (optional)",
			},
			&cli.StringFlag{
				Name:  "parent-run-id",
				Usage: "Parent run ID (required for retries)",
			},
			&cli.StringFlag{
				Name:  "job",
				Usage: "Job payload as JSON",
				Value: "{}",
			},
			&cli.StringFlag{
				Name:  "executor",
				Usage: "Path to executor binary",
				Value: "quarry-executor",
			},
			&cli.BoolFlag{
				Name:  "quiet",
				Usage: "Suppress result output",
			},
			// Partition key flags
			&cli.StringFlag{
				Name:     "source",
				Usage:    "Source identifier for partitioning (required)",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "category",
				Usage: "Category identifier for partitioning",
				Value: "default",
			},
			// Policy flags
			&cli.StringFlag{
				Name:  "policy",
				Usage: "Ingestion policy: strict or buffered",
				Value: "strict",
			},
			&cli.StringFlag{
				Name:  "flush-mode",
				Usage: "Flush mode for buffered policy: at_least_once, chunks_first, two_phase",
				Value: "at_least_once",
			},
			&cli.IntFlag{
				Name:  "buffer-events",
				Usage: "Max buffered events (buffered policy)",
				Value: 0,
			},
			&cli.Int64Flag{
				Name:  "buffer-bytes",
				Usage: "Max buffer size in bytes (buffered policy)",
				Value: 0,
			},
			// Proxy flags
			&cli.StringFlag{
				Name:  "proxy-config",
				Usage: "Path to proxy pools config file (JSON)",
			},
			&cli.StringFlag{
				Name:  "proxy-pool",
				Usage: "Pool name to select proxy from",
			},
			&cli.StringFlag{
				Name:  "proxy-strategy",
				Usage: "Strategy override: round_robin, random, or sticky",
			},
			&cli.StringFlag{
				Name:  "proxy-sticky-key",
				Usage: "Sticky key override for proxy selection",
			},
			&cli.StringFlag{
				Name:  "proxy-domain",
				Usage: "Domain for sticky scope derivation (when scope=domain)",
			},
			&cli.StringFlag{
				Name:  "proxy-origin",
				Usage: "Origin for sticky scope derivation (when scope=origin, format: scheme://host:port)",
			},
			// Lode storage flags
			&cli.StringFlag{
				Name:  "lode-backend",
				Usage: "Lode storage backend: fs or s3 (experimental)",
				Value: "fs",
			},
			&cli.StringFlag{
				Name:  "lode-path",
				Usage: "Lode storage path (fs: directory, s3: bucket/prefix)",
			},
			&cli.StringFlag{
				Name:  "lode-s3-region",
				Usage: "AWS region for S3 backend (optional, uses default chain)",
			},
		},
		Action: runAction,
	}
}

// policyChoice holds parsed policy configuration.
type policyChoice struct {
	name      string
	flushMode string
	maxEvents int
	maxBytes  int64
}

// proxyChoice holds parsed proxy configuration.
type proxyChoice struct {
	configPath string
	poolName   string
	strategy   string
	stickyKey  string
	domain     string
	origin     string
}

// lodeChoice holds parsed Lode storage configuration.
type lodeChoice struct {
	backend  string // "fs" or "s3"
	path     string // fs: directory, s3: bucket/prefix
	s3Region string
}

func runAction(c *cli.Context) error {
	// Parse policy config
	choice := policyChoice{
		name:      c.String("policy"),
		flushMode: c.String("flush-mode"),
		maxEvents: c.Int("buffer-events"),
		maxBytes:  c.Int64("buffer-bytes"),
	}

	// Validate policy config
	if err := validatePolicyConfig(choice); err != nil {
		return cli.Exit(fmt.Sprintf("invalid policy config: %v", err), exitExecutorCrash)
	}

	// Parse job payload
	var job any
	if err := json.Unmarshal([]byte(c.String("job")), &job); err != nil {
		return fmt.Errorf("invalid job JSON: %w", err)
	}

	// Build run metadata
	runMeta := &types.RunMeta{
		RunID:   c.String("run-id"),
		Attempt: c.Int("attempt"),
	}
	if jobID := c.String("job-id"); jobID != "" {
		runMeta.JobID = &jobID
	}
	if parentRunID := c.String("parent-run-id"); parentRunID != "" {
		runMeta.ParentRunID = &parentRunID
	}

	// Parse Lode config
	lodeConfig := lodeChoice{
		backend:  c.String("lode-backend"),
		path:     c.String("lode-path"),
		s3Region: c.String("lode-s3-region"),
	}

	// Build policy with Lode sink
	// Start time is "now" - used to derive partition day
	startTime := time.Now()
	pol, err := buildPolicy(choice, lodeConfig, c.String("source"), c.String("category"), runMeta.RunID, startTime)
	if err != nil {
		return fmt.Errorf("failed to create policy: %w", err)
	}
	defer func() { _ = pol.Close() }()

	// Parse proxy config
	proxyConfig := proxyChoice{
		configPath: c.String("proxy-config"),
		poolName:   c.String("proxy-pool"),
		strategy:   c.String("proxy-strategy"),
		stickyKey:  c.String("proxy-sticky-key"),
		domain:     c.String("proxy-domain"),
		origin:     c.String("proxy-origin"),
	}

	// Select proxy if configured
	var resolvedProxy *types.ProxyEndpoint
	if proxyConfig.poolName != "" {
		endpoint, err := selectProxy(proxyConfig, runMeta)
		if err != nil {
			return cli.Exit(fmt.Sprintf("proxy selection failed: %v", err), exitExecutorCrash)
		}
		resolvedProxy = endpoint
	}

	// Create run config
	config := &runtime.RunConfig{
		ExecutorPath: c.String("executor"),
		ScriptPath:   c.String("script"),
		Job:          job,
		RunMeta:      runMeta,
		Policy:       pol,
		Proxy:        resolvedProxy,
		Source:       c.String("source"),
		Category:     c.String("category"),
	}

	// Create orchestrator
	orchestrator, err := runtime.NewRunOrchestrator(config)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Set up context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Execute run (startTime was set earlier for Lode day derivation)
	result, err := orchestrator.Execute(ctx)
	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}
	duration := time.Since(startTime)

	// Print result
	if !c.Bool("quiet") {
		printRunResult(result, choice, duration)
	}

	return cli.Exit("", outcomeToExitCode(result.Outcome.Status))
}

func validatePolicyConfig(choice policyChoice) error {
	switch choice.name {
	case "strict":
		if choice.maxEvents > 0 || choice.maxBytes > 0 || choice.flushMode != "at_least_once" {
			fmt.Fprintf(os.Stderr, "Warning: buffer/flush flags ignored for strict policy\n")
		}
		return nil

	case "buffered":
		if choice.maxEvents <= 0 && choice.maxBytes <= 0 {
			return fmt.Errorf("buffered policy requires --buffer-events > 0 or --buffer-bytes > 0")
		}
		switch policy.FlushMode(choice.flushMode) {
		case policy.FlushAtLeastOnce, policy.FlushChunksFirst, policy.FlushTwoPhase:
			return nil
		default:
			return fmt.Errorf("invalid flush-mode: %s", choice.flushMode)
		}

	default:
		return fmt.Errorf("invalid policy: %s (must be strict or buffered)", choice.name)
	}
}

func buildPolicy(choice policyChoice, lodeConfig lodeChoice, source, category, runID string, startTime time.Time) (policy.Policy, error) {
	// Build Lode sink
	sink, err := buildLodeSink(lodeConfig, source, category, runID, startTime)
	if err != nil {
		return nil, fmt.Errorf("failed to create Lode sink: %w", err)
	}

	switch choice.name {
	case "strict":
		return policy.NewStrictPolicy(sink), nil

	case "buffered":
		config := policy.BufferedConfig{
			MaxBufferEvents: choice.maxEvents,
			MaxBufferBytes:  choice.maxBytes,
			FlushMode:       policy.FlushMode(choice.flushMode),
		}
		return policy.NewBufferedPolicy(sink, config)

	default:
		return nil, fmt.Errorf("unknown policy: %s", choice.name)
	}
}

// buildLodeSink creates a Lode sink based on CLI configuration.
// Returns StubSink if --lode-path is not specified (for backwards compatibility).
func buildLodeSink(lodeConfig lodeChoice, source, category, runID string, startTime time.Time) (policy.Sink, error) {
	// If no path specified, use stub sink (backwards compatible)
	if lodeConfig.path == "" {
		return policy.NewStubSink(), nil
	}

	// Build Lode config with partition keys
	cfg := lode.Config{
		Dataset:  "quarry",
		Source:   source,
		Category: category,
		Day:      lode.DeriveDay(startTime),
		RunID:    runID,
	}

	var client lode.Client
	var err error

	switch lodeConfig.backend {
	case "fs", "":
		client, err = lode.NewLodeClient(cfg, lodeConfig.path)
	case "s3":
		bucket, prefix := lode.ParseS3Path(lodeConfig.path)
		s3cfg := lode.S3Config{
			Bucket: bucket,
			Prefix: prefix,
			Region: lodeConfig.s3Region,
		}
		client, err = lode.NewLodeS3Client(cfg, s3cfg)
	default:
		return nil, fmt.Errorf("unknown lode-backend: %s (must be fs or s3)", lodeConfig.backend)
	}

	if err != nil {
		return nil, err
	}

	return lode.NewSink(cfg, client), nil
}

func outcomeToExitCode(status types.OutcomeStatus) int {
	switch status {
	case types.OutcomeSuccess:
		return exitSuccess
	case types.OutcomeScriptError:
		return exitScriptError
	case types.OutcomeExecutorCrash:
		return exitExecutorCrash
	case types.OutcomePolicyFailure:
		return exitPolicyFailure
	default:
		return exitScriptError
	}
}

// selectProxy loads proxy pools and selects an endpoint.
// Note: The selector is created fresh per invocation (CLI is one-shot).
// Round-robin counters and sticky maps do not persist across runs.
// This is intentional - each run is independent.
func selectProxy(config proxyChoice, runMeta *types.RunMeta) (*types.ProxyEndpoint, error) {
	// Load proxy pools from config file
	if config.configPath == "" {
		return nil, fmt.Errorf("--proxy-config required when --proxy-pool is specified")
	}

	pools, err := loadProxyPools(config.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load proxy pools: %w", err)
	}

	// Create selector and register pools
	// Note: Selector is per-invocation; state doesn't persist across CLI runs.
	selector := proxy.NewSelector()
	for _, pool := range pools {
		if err := selector.RegisterPool(&pool); err != nil {
			return nil, fmt.Errorf("failed to register pool %q: %w", pool.Name, err)
		}
	}

	// Warn about domain/origin sticky scopes without the required input
	for _, pool := range pools {
		if pool.Name == config.poolName && pool.Sticky != nil {
			scope := pool.Sticky.Scope
			// Check if required input is missing for the scope
			switch scope {
			case types.ProxyStickyDomain:
				if config.stickyKey == "" && config.domain == "" {
					fmt.Fprintf(os.Stderr, "Warning: pool %q uses domain sticky scope but no --proxy-sticky-key or --proxy-domain provided\n", pool.Name)
				}
			case types.ProxyStickyOrigin:
				if config.stickyKey == "" && config.origin == "" {
					fmt.Fprintf(os.Stderr, "Warning: pool %q uses origin sticky scope but no --proxy-sticky-key or --proxy-origin provided\n", pool.Name)
				}
			}
		}
	}

	// Build selection request (commit for actual runs)
	req := proxy.SelectRequest{
		Pool:      config.poolName,
		StickyKey: config.stickyKey,
		Domain:    config.domain,
		Origin:    config.origin,
		Commit:    true,
	}

	// Set job ID for sticky scope derivation
	if runMeta.JobID != nil {
		req.JobID = *runMeta.JobID
	}

	// Set strategy override if specified
	if config.strategy != "" {
		strategy := types.ProxyStrategy(config.strategy)
		req.StrategyOverride = &strategy
	}

	// Select endpoint
	endpoint, err := selector.Select(req)
	if err != nil {
		return nil, fmt.Errorf("selection failed: %w", err)
	}

	return endpoint, nil
}

// loadProxyPools loads proxy pools from a JSON config file.
func loadProxyPools(path string) ([]types.ProxyPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var pools []types.ProxyPool
	if err := json.Unmarshal(data, &pools); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return pools, nil
}

func printRunResult(result *runtime.RunResult, choice policyChoice, duration time.Duration) {
	fmt.Printf("\nrun_id=%s, attempt=%d, outcome=%s, duration=%s\n",
		result.RunMeta.RunID,
		result.RunMeta.Attempt,
		result.Outcome.Status,
		duration.Round(time.Millisecond),
	)

	if choice.name == "buffered" {
		fmt.Printf("policy=%s, flush_mode=%s, drops=%d, buffer_bytes=%d\n",
			choice.name,
			choice.flushMode,
			result.PolicyStats.EventsDropped,
			result.PolicyStats.BufferSize,
		)
	} else {
		fmt.Printf("policy=%s\n", choice.name)
	}

	fmt.Printf("\n=== Run Result ===\n")
	fmt.Printf("Run ID:       %s\n", result.RunMeta.RunID)
	if result.RunMeta.JobID != nil {
		fmt.Printf("Job ID:       %s\n", *result.RunMeta.JobID)
	}
	if result.RunMeta.ParentRunID != nil {
		fmt.Printf("Parent Run:   %s\n", *result.RunMeta.ParentRunID)
	}
	fmt.Printf("Attempt:      %d\n", result.RunMeta.Attempt)
	fmt.Printf("Outcome:      %s\n", result.Outcome.Status)
	fmt.Printf("Message:      %s\n", result.Outcome.Message)
	fmt.Printf("Duration:     %s\n", result.Duration)
	fmt.Printf("Events:       %d\n", result.EventCount)

	if result.ProxyUsed != nil {
		fmt.Printf("\n=== Proxy ===\n")
		fmt.Printf("Protocol:     %s\n", result.ProxyUsed.Protocol)
		fmt.Printf("Host:         %s\n", result.ProxyUsed.Host)
		fmt.Printf("Port:         %d\n", result.ProxyUsed.Port)
		if result.ProxyUsed.Username != nil {
			fmt.Printf("Username:     %s\n", *result.ProxyUsed.Username)
		}
	}

	fmt.Printf("\n=== Policy Stats ===\n")
	fmt.Printf("Events Total:     %d\n", result.PolicyStats.TotalEvents)
	fmt.Printf("Events Persisted: %d\n", result.PolicyStats.EventsPersisted)
	fmt.Printf("Events Dropped:   %d\n", result.PolicyStats.EventsDropped)
	fmt.Printf("Chunks Total:     %d\n", result.PolicyStats.TotalChunks)
	fmt.Printf("Flushes:          %d\n", result.PolicyStats.FlushCount)

	if result.ArtifactStats.TotalArtifacts > 0 {
		fmt.Printf("\n=== Artifact Stats ===\n")
		fmt.Printf("Total Artifacts:   %d\n", result.ArtifactStats.TotalArtifacts)
		fmt.Printf("Committed:         %d\n", result.ArtifactStats.CommittedArtifacts)
		fmt.Printf("Orphaned:          %d\n", result.ArtifactStats.OrphanedArtifacts)
		fmt.Printf("Total Chunks:      %d\n", result.ArtifactStats.TotalChunks)
		fmt.Printf("Total Bytes:       %d\n", result.ArtifactStats.TotalBytes)
	}

	if len(result.OrphanIDs) > 0 {
		fmt.Printf("\n=== Orphan Artifacts ===\n")
		for _, id := range result.OrphanIDs {
			fmt.Printf("  - %s\n", id)
		}
	}

	if result.StderrOutput != "" {
		fmt.Printf("\n=== Executor Stderr ===\n")
		fmt.Printf("%s", result.StderrOutput)
	}
}
