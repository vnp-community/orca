package tddagentclient

import (
	"time"

	"github.com/stablyai/orca-go/common/tddagent"
)

// Run statuses of tdd-agent (orchestrator/engine.py), shared with request-service in common/tddagent.
const (
	RunRunning          = tddagent.RunRunning
	RunAwaitingApproval = tddagent.RunAwaitingApproval
	RunAwaitingEvidence = tddagent.RunAwaitingEvidence
	RunBlocked          = tddagent.RunBlocked
	RunCompleted        = tddagent.RunCompleted
	RunFailed           = tddagent.RunFailed
)

// runView is the part of tdd-agent's run_get result the handlers read: the shared reading of the reply
// (common/tddagent.RunView) plus what only workflow steps need (the step output of contracts/tdd-agent-step-output.schema.json).
type runView struct {
	tddagent.RunView
}

func (v runView) status() string { return v.Status() }

func (v runView) terminal() bool { return v.Terminal() }

// processAlive reports whether a background advance of the run is running on the dev server.
func (v runView) processAlive() bool { return v.ProcessAlive() }

// heartbeatStale is true when a live advance has not beaten for three intervals (hung, not dead).
func (v runView) heartbeatStale(now time.Time) bool { return v.HeartbeatStale(now) }

func (v runView) gateStatuses() map[string]string { return v.GateStatuses() }

// newestChild returns the latest child run of flow (GT starts a new one when it re-runs).
func (v runView) newestChild(flow string) (tddagent.ChildView, bool) { return v.NewestChild(flow) }

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
