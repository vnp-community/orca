package tddagentclient

import (
	"sort"
	"time"
)

// Run statuses of tdd-agent (orchestrator/engine.py).
const (
	RunRunning          = "running"
	RunAwaitingApproval = "awaiting_approval"
	RunAwaitingEvidence = "awaiting_evidence"
	RunBlocked          = "blocked"
	RunCompleted        = "completed"
	RunFailed           = "failed"
)

// runView is the part of tdd-agent's run_get result the handlers read.
type runView struct {
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
	Tasks    map[string]taskView `json:"tasks"`
	Children []childView         `json:"children"`
	Process  *processView        `json:"process"`
}

type taskView struct {
	Status       string         `json:"status"`
	Decision     any            `json:"decision,omitempty"`
	DecisionHash string         `json:"decision_hash,omitempty"`
	ApprovalRole string         `json:"approval_role,omitempty"`
	Blockers     []string       `json:"blockers,omitempty"`
	Output       map[string]any `json:"output,omitempty"`
}

type childView struct {
	RunID     string `json:"run_id"`
	Flow      string `json:"flow"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type processView struct {
	PID                int     `json:"pid"`
	Alive              bool    `json:"alive"`
	HeartbeatAt        *string `json:"heartbeat_at"`
	FinishedAt         *string `json:"finished_at"`
	HeartbeatIntervalS float64 `json:"heartbeat_interval_s"`
}

func (v runView) status() string { return v.Run.Status }

func (v runView) terminal() bool { return v.Run.Status == RunCompleted || v.Run.Status == RunFailed }

// processAlive reports whether a background advance of the run is running on the dev server.
func (v runView) processAlive() bool { return v.Process != nil && v.Process.Alive }

// heartbeatStale is true when a live advance has not beaten for three intervals (hung, not dead).
func (v runView) heartbeatStale(now time.Time) bool {
	if !v.processAlive() || v.Process.HeartbeatAt == nil || v.Process.HeartbeatIntervalS <= 0 {
		return false
	}
	at, err := time.Parse(time.RFC3339Nano, *v.Process.HeartbeatAt)
	if err != nil {
		return false
	}
	return now.Sub(at) > time.Duration(3*v.Process.HeartbeatIntervalS*float64(time.Second))
}

func (v runView) gateStatuses() map[string]string {
	out := make(map[string]string, len(v.Tasks))
	for name, t := range v.Tasks {
		out[name] = t.Status
	}
	return out
}

// newestChild returns the latest child run of flow (GT starts a new one when it re-runs).
func (v runView) newestChild(flow string) (childView, bool) {
	var matches []childView
	for _, c := range v.Children {
		if flow == "" || c.Flow == flow {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		return childView{}, false
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].CreatedAt > matches[j].CreatedAt })
	return matches[0], true
}

// output is the step output shared by every handler (contracts/tdd-agent-step-output.schema.json).
func (v runView) output(action, projectDir string) map[string]any {
	blockers := v.Run.Result.Blockers
	if blockers == nil {
		blockers = []string{}
	}
	children := make([]map[string]string, 0, len(v.Children))
	for _, c := range v.Children {
		children = append(children, map[string]string{"runId": c.RunID, "flow": c.Flow, "status": c.Status})
	}
	return map[string]any{
		"action": action, "runId": v.Run.RunID, "flow": v.Run.Flow, "status": v.Run.Status,
		"pendingGate": nilIfEmpty(v.Run.Result.PendingGate), "blockedGate": nilIfEmpty(v.Run.Result.BlockedGate),
		"blockers": blockers, "gates": v.gateStatuses(), "children": children, "projectDir": projectDir,
	}
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
