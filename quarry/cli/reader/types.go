// Package reader provides the read-side data access layer for the quarry CLI.
//
// This package isolates all read operations from runtime internals.
// All read-only commands use this wrapper exclusively per the implementation plan.
//
// Current implementation returns stub data. When Lode integration is ready,
// these will be wired to actual data sources.
package reader

import "time"

// InspectRunResponse per CONTRACT_CLI.md.
type InspectRunResponse struct {
	RunID     string     `json:"run_id"`
	JobID     string     `json:"job_id"`
	State     string     `json:"state"`
	Attempt   int        `json:"attempt"`
	ParentRun *string    `json:"parent_run"`
	Policy    string     `json:"policy"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
}

// InspectJobResponse per CONTRACT_CLI.md.
type InspectJobResponse struct {
	JobID  string   `json:"job_id"`
	State  string   `json:"state"`
	RunIDs []string `json:"run_ids"`
}

// InspectTaskResponse per CONTRACT_CLI.md.
type InspectTaskResponse struct {
	TaskID string  `json:"task_id"`
	State  string  `json:"state"`
	RunID  *string `json:"run_id"`
}

// ProxySticky represents proxy sticky configuration.
type ProxySticky struct {
	Scope string `json:"scope"`
	TTL   string `json:"ttl"`
}

// ProxyRuntime represents runtime proxy state.
type ProxyRuntime struct {
	RoundRobinIndex int        `json:"round_robin_index"`
	StickyEntries   int        `json:"sticky_entries"`
	LastUsedAt      *time.Time `json:"last_used_at"`
}

// InspectProxyPoolResponse per CONTRACT_CLI.md.
type InspectProxyPoolResponse struct {
	Name        string       `json:"name"`
	Strategy    string       `json:"strategy"`
	EndpointCnt int          `json:"endpoint_cnt"`
	Sticky      *ProxySticky `json:"sticky"`
	Runtime     ProxyRuntime `json:"runtime"`
}

// InspectExecutorResponse per CONTRACT_CLI.md.
type InspectExecutorResponse struct {
	ExecutorID string     `json:"executor_id"`
	State      string     `json:"state"`
	LastSeenAt *time.Time `json:"last_seen_at"`
}

// RunStats per CONTRACT_CLI.md.
type RunStats struct {
	Total     int `json:"total"`
	Running   int `json:"running"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

// JobStats per CONTRACT_CLI.md.
type JobStats struct {
	Total     int `json:"total"`
	Running   int `json:"running"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

// TaskStats per CONTRACT_CLI.md.
type TaskStats struct {
	Total     int `json:"total"`
	Running   int `json:"running"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

// ProxyStats per CONTRACT_CLI.md.
type ProxyStats struct {
	Pool       string     `json:"pool"`
	Requests   int        `json:"requests"`
	Failures   int        `json:"failures"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

// ExecutorStats per CONTRACT_CLI.md.
type ExecutorStats struct {
	Total   int `json:"total"`
	Running int `json:"running"`
	Idle    int `json:"idle"`
	Failed  int `json:"failed"`
}

// ListRunItem per CONTRACT_CLI.md.
type ListRunItem struct {
	RunID     string    `json:"run_id"`
	State     string    `json:"state"`
	StartedAt time.Time `json:"started_at"`
}

// ListJobItem per CONTRACT_CLI.md.
type ListJobItem struct {
	JobID string `json:"job_id"`
	State string `json:"state"`
}

// ListPoolItem per CONTRACT_CLI.md.
type ListPoolItem struct {
	Name     string `json:"name"`
	Strategy string `json:"strategy"`
}

// ListExecutorItem per CONTRACT_CLI.md.
type ListExecutorItem struct {
	ExecutorID string `json:"executor_id"`
	State      string `json:"state"`
}

// ListRunsOptions for filtering list runs.
type ListRunsOptions struct {
	State string
	Limit int
}

// ProxyEndpoint represents a resolved proxy endpoint.
type ProxyEndpoint struct {
	Host     string  `json:"host"`
	Port     int     `json:"port"`
	Protocol string  `json:"protocol"`
	Username *string `json:"username,omitempty"`
}

// ResolveProxyResponse per CONTRACT_CLI.md.
type ResolveProxyResponse struct {
	Endpoint  ProxyEndpoint `json:"endpoint"`
	Committed bool          `json:"committed"`
}

// IPCDebugResponse per CONTRACT_CLI.md.
type IPCDebugResponse struct {
	Transport    string  `json:"transport"`
	Encoding     string  `json:"encoding"`
	MessagesSent int     `json:"messages_sent"`
	Errors       int     `json:"errors"`
	LastError    *string `json:"last_error"`
}
