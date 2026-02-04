package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/justapithecus/quarry/executor"
	"github.com/justapithecus/quarry/lode"
	"github.com/justapithecus/quarry/policy"
	"github.com/justapithecus/quarry/proxy"
	"github.com/justapithecus/quarry/runtime"
	"github.com/justapithecus/quarry/types"
)

// Exit codes per CONTRACT_RUN.md.
const (
	exitSuccess       = 0
	exitScriptError   = 1
	exitExecutorCrash = 2
	exitPolicyFailure = 3
)

// exitConfigError is used for CLI/input validation failures.
// These are pre-execution errors (not script failures).
// Maps to exitExecutorCrash since they prevent execution.
const exitConfigError = exitExecutorCrash

// RunCommand returns the run command.
// This is the only command that executes work per CONTRACT_CLI.md.
func RunCommand() *cli.Command {
	return &cli.Command{
		Name:  "run",
		Usage: "Execute a script run (the only execution entrypoint)",
		UsageText: `quarry run --script <path> --run-id <id> --source <name> \
    --storage-backend <fs|s3> --storage-path <path> [options]

EXAMPLES:
  # Run a script with filesystem storage
  quarry run --script ./script.ts --run-id run-001 --source my-source \
    --storage-backend fs --storage-path ./data

  # Run with inline job payload
  quarry run --script ./script.ts --run-id run-002 --source my-source \
    --storage-backend fs --storage-path ./data \
    --job '{"url": "https://example.com"}'

  # Run with job payload from file
  quarry run --script ./script.ts --run-id run-003 --source my-source \
    --storage-backend fs --storage-path ./data \
    --job-json ./jobs/crawl-config.json

  # Run with S3 storage
  quarry run --script ./script.ts --run-id run-004 --source my-source \
    --storage-backend s3 --storage-path my-bucket/prefix \
    --storage-region us-east-1

ADVANCED:
  # Override executor path (troubleshooting)
  quarry run --script ./script.ts --run-id run-005 --source my-source \
    --storage-backend fs --storage-path ./data \
    --executor /custom/path/to/executor.js`,
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
				Usage: "Job payload as inline JSON",
			},
			&cli.StringFlag{
				Name:  "job-json",
				Usage: "Path to JSON file containing job payload",
			},
			&cli.StringFlag{
				Name:  "executor",
				Usage: "Path to executor binary (advanced: auto-resolved by default)",
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
			// Storage flags
			&cli.StringFlag{
				Name:     "storage-backend",
				Usage:    "Storage backend: fs or s3",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "storage-path",
				Usage:    "Storage path (fs: directory, s3: bucket/prefix)",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "storage-region",
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

// storageChoice holds parsed storage configuration.
type storageChoice struct {
	backend string // "fs" or "s3"
	path    string // fs: directory, s3: bucket/prefix
	region  string // AWS region for S3 (optional)
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

	// Parse job payload (--job or --job-json, not both)
	job, err := parseJobPayload(c.String("job"), c.String("job-json"))
	if err != nil {
		return cli.Exit(err.Error(), exitConfigError)
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

	// Parse and validate storage config
	storageConfig := storageChoice{
		backend: c.String("storage-backend"),
		path:    c.String("storage-path"),
		region:  c.String("storage-region"),
	}
	if err := validateStorageConfig(storageConfig); err != nil {
		return cli.Exit(err.Error(), exitConfigError)
	}

	// Build policy with storage sink
	// Start time is "now" - used to derive partition day
	startTime := time.Now()
	pol, err := buildPolicy(choice, storageConfig, c.String("source"), c.String("category"), runMeta.RunID, startTime)
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

	// Resolve executor path
	executorPath, err := resolveExecutor(c.String("executor"))
	if err != nil {
		return cli.Exit(err.Error(), exitConfigError)
	}

	// Create run config
	config := &runtime.RunConfig{
		ExecutorPath: executorPath,
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
			return fmt.Errorf(`buffered policy requires buffer limits

Add one or both of:
  --buffer-events <n>   Maximum events to buffer (e.g., --buffer-events 1000)
  --buffer-bytes <n>    Maximum bytes to buffer (e.g., --buffer-bytes 1048576)`)
		}
		switch policy.FlushMode(choice.flushMode) {
		case policy.FlushAtLeastOnce, policy.FlushChunksFirst, policy.FlushTwoPhase:
			return nil
		default:
			return fmt.Errorf(`invalid --flush-mode: %q

Valid options:
  at_least_once   Flush all buffered data at least once (default)
  chunks_first    Flush artifact chunks before events
  two_phase       Two-phase commit for transactional semantics`, choice.flushMode)
		}

	default:
		return fmt.Errorf(`invalid --policy: %q

Valid options:
  strict     Write events immediately, fail on any error (default)
  buffered   Buffer events in memory, flush periodically`, choice.name)
	}
}

func validateStorageConfig(config storageChoice) error {
	switch config.backend {
	case "fs":
		// Validate path exists and is a directory
		info, err := os.Stat(config.path)
		if os.IsNotExist(err) {
			return fmt.Errorf(`storage path does not exist: %s

Create the directory first:
  mkdir -p %s`, config.path, config.path)
		}
		if err != nil {
			return fmt.Errorf("cannot access storage path %q: %v (ensure the path exists and is readable)", config.path, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("storage path is not a directory: %s (--storage-path for fs backend must be a directory, not a file)", config.path)
		}
		return nil

	case "s3":
		// Basic validation for S3 path format
		if config.path == "" {
			return fmt.Errorf(`--storage-path required for s3 backend

Format: bucket-name/optional-prefix
Example: --storage-path my-bucket/quarry-data`)
		}
		// S3 credentials are validated at runtime by AWS SDK
		return nil

	default:
		return fmt.Errorf(`invalid --storage-backend: %q

Valid options:
  fs   Filesystem storage (requires writable directory)
  s3   Amazon S3 storage (requires AWS credentials)`, config.backend)
	}
}

// parseJobPayload parses job payload from --job (inline) or --job-json (file).
// Using both flags is an explicit error. If neither is specified, returns empty object.
func parseJobPayload(jobInline, jobFile string) (any, error) {
	hasInline := jobInline != ""
	hasFile := jobFile != ""

	// Conflict: both specified
	if hasInline && hasFile {
		return nil, fmt.Errorf(`cannot use both --job and --job-json

Provide job payload via ONE of:
  --job '{"key": "value"}'     (inline JSON)
  --job-json ./payload.json    (path to JSON file)`)
	}

	// Load from file
	if hasFile {
		data, err := os.ReadFile(jobFile)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf(`job file not found: %s

Ensure the file exists:
  ls -la %s`, jobFile, jobFile)
			}
			return nil, fmt.Errorf("cannot read job file %q: %v", jobFile, err)
		}

		var job any
		if err := json.Unmarshal(data, &job); err != nil {
			return nil, fmt.Errorf(`invalid JSON in job file %s: %v

The file must contain valid JSON. Example:
  {"url": "https://example.com", "page": 1}`, jobFile, err)
		}
		return job, nil
	}

	// Parse inline JSON
	if hasInline {
		var job any
		if err := json.Unmarshal([]byte(jobInline), &job); err != nil {
			return nil, fmt.Errorf(`invalid --job JSON: %v

The --job flag must contain valid JSON. Examples:
  --job '{}'
  --job '{"key": "value"}'
  --job '{"url": "https://example.com", "page": 1}'

Received: %s`, err, jobInline)
		}
		return job, nil
	}

	// Neither specified: empty object
	return map[string]any{}, nil
}

// resolveExecutor finds the executor binary path.
// Resolution order:
//  1. Explicit --executor flag (if provided)
//  2. Embedded executor (extracted to temp dir)
//  3. Bundled path relative to quarry binary (development layout)
//  4. "quarry-executor" in PATH
func resolveExecutor(explicit string) (string, error) {
	// 1. Explicit override takes priority
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("executor not found at %s (check the path and try again)", explicit)
		}
		return explicit, nil
	}

	// 2. Try embedded executor (primary method for distributed binary)
	if executor.IsEmbedded() {
		path, err := executor.ExtractedPath()
		if err == nil {
			return path, nil
		}
		// Log extraction failure but continue to fallbacks
		fmt.Fprintf(os.Stderr, "Warning: failed to extract embedded executor: %v\n", err)
	}

	// 3. Try bundled path relative to this binary (development layout)
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		// Try common bundled locations
		bundledPaths := []string{
			filepath.Join(execDir, "..", "executor-node", "dist", "bin", "executor.js"),
			filepath.Join(execDir, "executor-node", "dist", "bin", "executor.js"),
			filepath.Join(execDir, "executor.js"),
		}
		for _, p := range bundledPaths {
			absPath, err := filepath.Abs(p)
			if err != nil {
				continue
			}
			if _, err := os.Stat(absPath); err == nil {
				return absPath, nil
			}
		}
	}

	// 4. Try PATH lookup
	if path, err := exec.LookPath("quarry-executor"); err == nil {
		return path, nil
	}

	// Not found - provide actionable error
	return "", fmt.Errorf(`executor not found

Quarry could not locate the executor. To fix:

  1. Build the executor:
     pnpm -C executor-node run build

  2. Or specify the path manually:
     --executor ./executor-node/dist/bin/executor.js

  3. Or add quarry-executor to your PATH`)
}

func buildPolicy(choice policyChoice, storageConfig storageChoice, source, category, runID string, startTime time.Time) (policy.Policy, error) {
	// Build storage sink
	sink, err := buildStorageSink(storageConfig, source, category, runID, startTime)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage sink: %w", err)
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

// buildStorageSink creates a Lode storage sink based on CLI configuration.
// Storage backend and path are required - no silent fallback to stub.
func buildStorageSink(storageConfig storageChoice, source, category, runID string, startTime time.Time) (policy.Sink, error) {
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

	switch storageConfig.backend {
	case "fs":
		client, err = lode.NewLodeClient(cfg, storageConfig.path)
		if err != nil {
			return nil, fmt.Errorf("filesystem storage initialization failed: %w (ensure directory %s exists and is writable)", err, storageConfig.path)
		}
	case "s3":
		bucket, prefix := lode.ParseS3Path(storageConfig.path)
		s3cfg := lode.S3Config{
			Bucket: bucket,
			Prefix: prefix,
			Region: storageConfig.region,
		}
		client, err = lode.NewLodeS3Client(cfg, s3cfg)
		if err != nil {
			return nil, fmt.Errorf("S3 storage initialization failed: %w (check AWS credentials and bucket permissions)", err)
		}
	default:
		// Should not reach here due to validateStorageConfig
		return nil, fmt.Errorf("unknown storage-backend: %s", storageConfig.backend)
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
