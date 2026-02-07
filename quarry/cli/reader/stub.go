package reader

import (
	"errors"
	"time"
)

// StubReader returns shape-correct stub data for development and testing.
// Replace with a real implementation when Lode integration is ready.
type StubReader struct{}

// NewStubReader creates a new stub reader.
func NewStubReader() *StubReader {
	return &StubReader{}
}

// InspectRun returns stub run details.
func (r *StubReader) InspectRun(runID string) *InspectRunResponse {
	now := time.Now()
	ended := now.Add(-time.Minute)
	return &InspectRunResponse{
		RunID:     runID,
		JobID:     "stub-job-001",
		State:     "succeeded",
		Attempt:   1,
		ParentRun: nil,
		Policy:    "strict",
		StartedAt: now.Add(-5 * time.Minute),
		EndedAt:   &ended,
	}
}

// InspectJob returns stub job details.
func (r *StubReader) InspectJob(jobID string) *InspectJobResponse {
	return &InspectJobResponse{
		JobID:  jobID,
		State:  "completed",
		RunIDs: []string{"stub-run-001", "stub-run-002"},
	}
}

// InspectTask returns stub task details.
func (r *StubReader) InspectTask(taskID string) *InspectTaskResponse {
	runID := "stub-run-001"
	return &InspectTaskResponse{
		TaskID: taskID,
		State:  "completed",
		RunID:  &runID,
	}
}

// InspectProxy returns stub proxy pool details.
func (r *StubReader) InspectProxy(poolName string) *InspectProxyPoolResponse {
	now := time.Now()
	ttlMs := int64(3600000) // 1 hour in milliseconds
	return &InspectProxyPoolResponse{
		Name:        poolName,
		Strategy:    "round_robin",
		EndpointCnt: 3,
		Sticky: &ProxySticky{
			Scope: "job", // Valid scopes: job, domain, origin
			TTLMs: &ttlMs,
		},
		Runtime: ProxyRuntime{
			RoundRobinIndex: 1,
			StickyEntries:   5,
			RecencyWindow:   3,
			RecencyFill:     2,
			LastUsedAt:      &now,
		},
	}
}

// InspectExecutor returns stub executor details.
func (r *StubReader) InspectExecutor(executorID string) *InspectExecutorResponse {
	now := time.Now()
	return &InspectExecutorResponse{
		ExecutorID: executorID,
		State:      "idle",
		LastSeenAt: &now,
	}
}

// StatsRuns returns stub run statistics.
func (r *StubReader) StatsRuns() *RunStats {
	return &RunStats{
		Total:     100,
		Running:   5,
		Succeeded: 90,
		Failed:    5,
	}
}

// StatsJobs returns stub job statistics.
func (r *StubReader) StatsJobs() *JobStats {
	return &JobStats{
		Total:     50,
		Running:   2,
		Succeeded: 45,
		Failed:    3,
	}
}

// StatsTasks returns stub task statistics.
func (r *StubReader) StatsTasks() *TaskStats {
	return &TaskStats{
		Total:     200,
		Running:   10,
		Succeeded: 180,
		Failed:    10,
	}
}

// StatsProxies returns stub proxy statistics.
func (r *StubReader) StatsProxies() []ProxyStats {
	now := time.Now()
	return []ProxyStats{
		{
			Pool:       "default",
			Requests:   1000,
			Failures:   5,
			LastUsedAt: &now,
		},
		{
			Pool:       "premium",
			Requests:   500,
			Failures:   1,
			LastUsedAt: &now,
		},
	}
}

// StatsExecutors returns stub executor statistics.
func (r *StubReader) StatsExecutors() *ExecutorStats {
	return &ExecutorStats{
		Total:   10,
		Running: 3,
		Idle:    6,
		Failed:  1,
	}
}

// StatsMetrics returns stub metrics statistics.
func (r *StubReader) StatsMetrics() *MetricsSnapshot {
	return &MetricsSnapshot{
		Ts:                    time.Now().UTC().Format(time.RFC3339),
		RunsStarted:           100,
		RunsCompleted:         90,
		RunsFailed:            5,
		RunsCrashed:           5,
		EventsReceived:        50000,
		EventsPersisted:       49500,
		EventsDropped:         500,
		DroppedByType:         map[string]int64{"log": 300, "debug": 200},
		ExecutorLaunchSuccess: 100,
		ExecutorLaunchFailure: 0,
		ExecutorCrash:         5,
		IPCDecodeErrors:       2,
		LodeWriteSuccess:      980,
		LodeWriteFailure:      3,
		LodeWriteRetry:        0,
		Policy:                "strict",
		Executor:              "executor.js",
		StorageBackend:        "fs",
		RunID:                 "stub-run-001",
		JobID:                 "stub-job-001",
	}
}

// ListRuns returns stub run list.
func (r *StubReader) ListRuns(opts ListRunsOptions) []ListRunItem {
	now := time.Now()
	runs := []ListRunItem{
		{RunID: "run-001", State: "succeeded", StartedAt: now.Add(-1 * time.Hour)},
		{RunID: "run-002", State: "succeeded", StartedAt: now.Add(-2 * time.Hour)},
		{RunID: "run-003", State: "running", StartedAt: now.Add(-5 * time.Minute)},
		{RunID: "run-004", State: "failed", StartedAt: now.Add(-30 * time.Minute)},
	}

	// Filter by state if specified
	if opts.State != "" {
		filtered := make([]ListRunItem, 0)
		for _, run := range runs {
			if run.State == opts.State {
				filtered = append(filtered, run)
			}
		}
		runs = filtered
	}

	// Apply limit
	if opts.Limit > 0 && len(runs) > opts.Limit {
		runs = runs[:opts.Limit]
	}

	return runs
}

// ListJobs returns stub job list.
func (r *StubReader) ListJobs() []ListJobItem {
	return []ListJobItem{
		{JobID: "job-001", State: "completed"},
		{JobID: "job-002", State: "running"},
		{JobID: "job-003", State: "pending"},
	}
}

// ListPools returns stub proxy pool list.
func (r *StubReader) ListPools() []ListPoolItem {
	return []ListPoolItem{
		{Name: "default", Strategy: "round_robin"},
		{Name: "premium", Strategy: "sticky"},
		{Name: "backup", Strategy: "random"},
	}
}

// ListExecutors returns stub executor list.
func (r *StubReader) ListExecutors() []ListExecutorItem {
	return []ListExecutorItem{
		{ExecutorID: "exec-001", State: "running"},
		{ExecutorID: "exec-002", State: "idle"},
		{ExecutorID: "exec-003", State: "idle"},
	}
}

// DebugResolveProxy returns stub proxy resolution.
func (r *StubReader) DebugResolveProxy(pool string, commit bool) (*ResolveProxyResponse, error) {
	if pool == "" {
		return nil, errors.New("pool name required")
	}

	return &ResolveProxyResponse{
		Endpoint: ProxyEndpoint{
			Host:     "proxy.example.com",
			Port:     8080,
			Protocol: "http",
			Username: nil,
		},
		Committed: commit,
	}, nil
}

// DebugIPC returns stub IPC debug information.
func (r *StubReader) DebugIPC(verbose bool) *IPCDebugResponse {
	var lastErr *string
	if verbose {
		errMsg := "connection timeout at 2024-01-15T10:30:00Z"
		lastErr = &errMsg
	}

	return &IPCDebugResponse{
		Transport:    "stdio",
		Encoding:     "msgpack",
		MessagesSent: 1500,
		Errors:       3,
		LastError:    lastErr,
	}
}

// Verify StubReader implements Reader.
var _ Reader = (*StubReader)(nil)
