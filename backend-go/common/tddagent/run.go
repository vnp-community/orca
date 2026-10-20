package tddagent

import (
	"sort"
	"time"
)

// Run statuses of tdd-agent (tdd-agentic-ai orchestrator/engine.py).
const (
	RunRunning          = "running"
	RunAwaitingApproval = "awaiting_approval"
	RunAwaitingEvidence = "awaiting_evidence"
	RunBlocked          = "blocked"
	RunCompleted        = "completed"
	RunFailed           = "failed"
)

// RunView is the part of tdd-agent's run_get result the callers read. Fields a caller does not use are left out on purpose:
// tdd-agent may add to the reply without this package noticing.
type RunView struct {
	Run struct {
		RunID       string `json:"run_id"`
		Flow        string `json:"flow"`
		Status      string `json:"status"`
		ParentRunID string `json:"parent_run_id"`
		Result      struct {
			Blockers    []string `json:"blockers"`
			PendingGate string   `json:"pending_gate"`
			BlockedGate string   `json:"blocked_gate"`
		} `json:"result"`
	} `json:"run"`
	Tasks    map[string]TaskView `json:"tasks"`
	Children []ChildView         `json:"children"`
	Process  *ProcessView        `json:"process"`
}

// TaskView is one gate of the run.
type TaskView struct {
	Status       string         `json:"status"`
	Decision     any            `json:"decision,omitempty"`
	DecisionHash string         `json:"decision_hash,omitempty"`
	ApprovalRole string         `json:"approval_role,omitempty"`
	Blockers     []string       `json:"blockers,omitempty"`
	Output       map[string]any `json:"output,omitempty"`
}

// ChildView is a child run (for example the `testspec` run a change-request gate starts).
type ChildView struct {
	RunID     string `json:"run_id"`
	Flow      string `json:"flow"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// ProcessView is the background advance of the run on the dev server (run_dispatch).
type ProcessView struct {
	PID                int     `json:"pid"`
	Alive              bool    `json:"alive"`
	HeartbeatAt        *string `json:"heartbeat_at"`
	FinishedAt         *string `json:"finished_at"`
	HeartbeatIntervalS float64 `json:"heartbeat_interval_s"`
}

// Status is the run status.
func (v RunView) Status() string { return v.Run.Status }

// Terminal is true when the run can no longer change (completed or failed). A blocked run is not terminal: it can be run again.
func (v RunView) Terminal() bool { return v.Run.Status == RunCompleted || v.Run.Status == RunFailed }

// ProcessAlive reports whether a background advance of the run is running on the dev server.
func (v RunView) ProcessAlive() bool { return v.Process != nil && v.Process.Alive }

// HeartbeatStale is true when a live advance has not beaten for three intervals (hung, not dead).
func (v RunView) HeartbeatStale(now time.Time) bool {
	if !v.ProcessAlive() || v.Process.HeartbeatAt == nil || v.Process.HeartbeatIntervalS <= 0 {
		return false
	}
	at, err := time.Parse(time.RFC3339Nano, *v.Process.HeartbeatAt)
	if err != nil {
		return false
	}
	return now.Sub(at) > time.Duration(3*v.Process.HeartbeatIntervalS*float64(time.Second))
}

// GateStatuses maps each gate to its status.
func (v RunView) GateStatuses() map[string]string {
	out := make(map[string]string, len(v.Tasks))
	for name, t := range v.Tasks {
		out[name] = t.Status
	}
	return out
}

// NewestChild returns the latest child run of flow (empty flow: of any flow).
func (v RunView) NewestChild(flow string) (ChildView, bool) {
	var matches []ChildView
	for _, c := range v.Children {
		if flow == "" || c.Flow == flow {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		return ChildView{}, false
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].CreatedAt > matches[j].CreatedAt })
	return matches[0], true
}
