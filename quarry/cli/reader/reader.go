package reader

import (
	"fmt"
	"time"
)

// Stub data source indicator.
// These functions return shape-correct stub data until Lode integration is ready.

// InspectRun returns details for a specific run.
func InspectRun(runID string) *InspectRunResponse {
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

// InspectJob returns details for a specific job.
func InspectJob(jobID string) *InspectJobResponse {
	return &InspectJobResponse{
		JobID:  jobID,
		State:  "completed",
		RunIDs: []string{"stub-run-001", "stub-run-002"},
	}
}

// InspectTask returns details for a specific task.
func InspectTask(taskID string) *InspectTaskResponse {
	runID := "stub-run-001"
	return &InspectTaskResponse{
		TaskID: taskID,
		State:  "completed",
		RunID:  &runID,
	}
}

// InspectProxy returns details for a specific proxy pool.
func InspectProxy(poolName string) *InspectProxyPoolResponse {
	now := time.Now()
	return &InspectProxyPoolResponse{
		Name:        poolName,
		Strategy:    "round_robin",
		EndpointCnt: 3,
		Sticky: &ProxySticky{
			Scope: "run",
			TTL:   "1h",
		},
		Runtime: ProxyRuntime{
			RoundRobinIndex: 1,
			StickyEntries:   5,
			LastUsedAt:      &now,
		},
	}
}

// InspectExecutor returns details for a specific executor.
func InspectExecutor(executorID string) *InspectExecutorResponse {
	now := time.Now()
	return &InspectExecutorResponse{
		ExecutorID: executorID,
		State:      "idle",
		LastSeenAt: &now,
	}
}

// StatsRuns returns run statistics.
func StatsRuns() *RunStats {
	return &RunStats{
		Total:     100,
		Running:   5,
		Succeeded: 90,
		Failed:    5,
	}
}

// StatsJobs returns job statistics.
func StatsJobs() *JobStats {
	return &JobStats{
		Total:     50,
		Running:   2,
		Succeeded: 45,
		Failed:    3,
	}
}

// StatsTasks returns task statistics.
func StatsTasks() *TaskStats {
	return &TaskStats{
		Total:     200,
		Running:   10,
		Succeeded: 180,
		Failed:    10,
	}
}

// StatsProxies returns proxy statistics.
func StatsProxies() []ProxyStats {
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

// StatsExecutors returns executor statistics.
func StatsExecutors() *ExecutorStats {
	return &ExecutorStats{
		Total:   10,
		Running: 3,
		Idle:    6,
		Failed:  1,
	}
}

// ListRuns returns a list of runs with optional filtering.
func ListRuns(opts ListRunsOptions) []ListRunItem {
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
		for _, r := range runs {
			if r.State == opts.State {
				filtered = append(filtered, r)
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

// ListJobs returns a list of jobs.
func ListJobs() []ListJobItem {
	return []ListJobItem{
		{JobID: "job-001", State: "completed"},
		{JobID: "job-002", State: "running"},
		{JobID: "job-003", State: "pending"},
	}
}

// ListPools returns a list of proxy pools.
func ListPools() []ListPoolItem {
	return []ListPoolItem{
		{Name: "default", Strategy: "round_robin"},
		{Name: "premium", Strategy: "sticky"},
		{Name: "backup", Strategy: "random"},
	}
}

// ListExecutors returns a list of executors.
func ListExecutors() []ListExecutorItem {
	return []ListExecutorItem{
		{ExecutorID: "exec-001", State: "running"},
		{ExecutorID: "exec-002", State: "idle"},
		{ExecutorID: "exec-003", State: "idle"},
	}
}

// DebugResolveProxy resolves a proxy endpoint from a pool.
// If commit is true, advances rotation counters (ephemeral mutation).
func DebugResolveProxy(pool string, commit bool) (*ResolveProxyResponse, error) {
	// Stub implementation
	if pool == "" {
		return nil, fmt.Errorf("pool name required")
	}

	// In real implementation, if commit is true, this would advance
	// the rotation counter in memory (ephemeral, non-persistent mutation)
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

// DebugIPC returns IPC debug information.
func DebugIPC(verbose bool) *IPCDebugResponse {
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
