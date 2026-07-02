// Package model holds the shared execution-domain types used across the
// DAG, runtime, state and API layers. These are intentionally free of
// dependencies on the parser/AST so any layer can import them.
package model

import "time"

// Status is the lifecycle state of a Run, Task or Step.
type Status string

const (
	StatusPending   Status = "pending"
	StatusReady     Status = "ready"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusSkipped   Status = "skipped"
	StatusCancelled Status = "cancelled"
)

// Terminal reports whether the status is a final state.
func (s Status) Terminal() bool {
	switch s {
	case StatusSucceeded, StatusFailed, StatusSkipped, StatusCancelled:
		return true
	default:
		return false
	}
}

// Run is a single execution of a workflow.
type Run struct {
	ID         string            `json:"id"`
	Workflow   string            `json:"workflow"`
	Status     Status            `json:"status"`
	Params     map[string]string `json:"params,omitempty"`
	Tasks      []*TaskRun        `json:"tasks"`
	StartedAt  time.Time         `json:"startedAt"`
	FinishedAt time.Time         `json:"finishedAt,omitempty"`
	Error      string            `json:"error,omitempty"`
	DryRun     bool              `json:"dryRun,omitempty"`
}

// TaskRun is the execution record for one task within a Run.
type TaskRun struct {
	Name       string            `json:"name"`
	Status     Status            `json:"status"`
	ExitCode   int               `json:"exitCode"`
	Attempts   int               `json:"attempts"`
	DependsOn  []string          `json:"dependsOn,omitempty"`
	Stdout     string            `json:"stdout,omitempty"`
	Stderr     string            `json:"stderr,omitempty"`
	Outputs    map[string]string `json:"outputs,omitempty"`
	Error      string            `json:"error,omitempty"`
	StartedAt  time.Time         `json:"startedAt,omitempty"`
	FinishedAt time.Time         `json:"finishedAt,omitempty"`
}

// Duration returns the wall-clock duration of the run.
func (r *Run) Duration() time.Duration {
	if r.FinishedAt.IsZero() {
		return 0
	}
	return r.FinishedAt.Sub(r.StartedAt)
}
