// Package reader provides the read-side data access layer for the quarry CLI.
//
// This package isolates all read operations from runtime internals.
// All read-only commands use this wrapper exclusively per the implementation plan.
//
// The package uses dependency injection via SetReader() to allow swapping
// between stub and real implementations. Default is StubReader.
package reader

// InspectRun returns details for a specific run.
// Delegates to the package-level reader.
func InspectRun(runID string) *InspectRunResponse {
	return defaultReader.InspectRun(runID)
}

// InspectJob returns details for a specific job.
// Delegates to the package-level reader.
func InspectJob(jobID string) *InspectJobResponse {
	return defaultReader.InspectJob(jobID)
}

// InspectTask returns details for a specific task.
// Delegates to the package-level reader.
func InspectTask(taskID string) *InspectTaskResponse {
	return defaultReader.InspectTask(taskID)
}

// InspectProxy returns details for a specific proxy pool.
// Delegates to the package-level reader.
func InspectProxy(poolName string) *InspectProxyPoolResponse {
	return defaultReader.InspectProxy(poolName)
}

// InspectExecutor returns details for a specific executor.
// Delegates to the package-level reader.
func InspectExecutor(executorID string) *InspectExecutorResponse {
	return defaultReader.InspectExecutor(executorID)
}

// StatsRuns returns run statistics.
// Delegates to the package-level reader.
func StatsRuns() *RunStats {
	return defaultReader.StatsRuns()
}

// StatsJobs returns job statistics.
// Delegates to the package-level reader.
func StatsJobs() *JobStats {
	return defaultReader.StatsJobs()
}

// StatsTasks returns task statistics.
// Delegates to the package-level reader.
func StatsTasks() *TaskStats {
	return defaultReader.StatsTasks()
}

// StatsProxies returns proxy statistics.
// Delegates to the package-level reader.
func StatsProxies() []ProxyStats {
	return defaultReader.StatsProxies()
}

// StatsExecutors returns executor statistics.
// Delegates to the package-level reader.
func StatsExecutors() *ExecutorStats {
	return defaultReader.StatsExecutors()
}

// ListRuns returns a list of runs with optional filtering.
// Delegates to the package-level reader.
func ListRuns(opts ListRunsOptions) []ListRunItem {
	return defaultReader.ListRuns(opts)
}

// ListJobs returns a list of jobs.
// Delegates to the package-level reader.
func ListJobs() []ListJobItem {
	return defaultReader.ListJobs()
}

// ListPools returns a list of proxy pools.
// Delegates to the package-level reader.
func ListPools() []ListPoolItem {
	return defaultReader.ListPools()
}

// ListExecutors returns a list of executors.
// Delegates to the package-level reader.
func ListExecutors() []ListExecutorItem {
	return defaultReader.ListExecutors()
}

// DebugResolveProxy resolves a proxy endpoint from a pool.
// If commit is true, advances rotation counters (ephemeral mutation).
// Delegates to the package-level reader.
func DebugResolveProxy(pool string, commit bool) (*ResolveProxyResponse, error) {
	return defaultReader.DebugResolveProxy(pool, commit)
}

// DebugIPC returns IPC debug information.
// Delegates to the package-level reader.
func DebugIPC(verbose bool) *IPCDebugResponse {
	return defaultReader.DebugIPC(verbose)
}
