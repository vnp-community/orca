package tddagent

import (
	"encoding/json"
	"testing"
	"time"
)

const runJSON = `{
  "run": {"run_id": "run-1", "flow": "change-request", "status": "running", "parent_run_id": "",
          "result": {"blockers": ["LLM_NOT_CONFIGURED"], "pending_gate": "G1", "blocked_gate": ""}},
  "tasks": {"G0": {"status": "completed"}, "G1": {"status": "awaiting_approval", "approval_role": "BA", "decision_hash": "h"}},
  "children": [
    {"run_id": "run-a", "flow": "testspec", "status": "completed", "created_at": "2026-10-19T04:00:00Z"},
    {"run_id": "run-b", "flow": "testspec", "status": "running", "created_at": "2026-10-19T05:00:00Z"},
    {"run_id": "run-c", "flow": "other", "status": "running", "created_at": "2026-10-19T06:00:00Z"}
  ],
  "process": {"pid": 42, "alive": true, "heartbeat_at": "2026-10-19T05:00:00Z", "finished_at": null, "heartbeat_interval_s": 15}
}`

func decode(t *testing.T) RunView {
	t.Helper()
	var v RunView
	if err := json.Unmarshal([]byte(runJSON), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestRunViewReadsTheRunGetReply(t *testing.T) {
	v := decode(t)
	if v.Status() != RunRunning || v.Run.Result.PendingGate != "G1" || len(v.Run.Result.Blockers) != 1 || v.Terminal() {
		t.Fatalf("%+v", v.Run)
	}
	gates := v.GateStatuses()
	if gates["G0"] != "completed" || gates["G1"] != "awaiting_approval" || v.Tasks["G1"].ApprovalRole != "BA" {
		t.Fatalf("gates %v", gates)
	}
}

func TestTerminalIsCompletedOrFailedNotBlocked(t *testing.T) {
	for status, want := range map[string]bool{RunCompleted: true, RunFailed: true, RunBlocked: false, RunRunning: false, RunAwaitingApproval: false} {
		var v RunView
		v.Run.Status = status
		if v.Terminal() != want {
			t.Fatalf("%s: terminal %v", status, v.Terminal())
		}
	}
}

func TestNewestChildPicksTheLatestOfTheFlow(t *testing.T) {
	v := decode(t)
	if c, ok := v.NewestChild("testspec"); !ok || c.RunID != "run-b" {
		t.Fatalf("testspec: %+v %v", c, ok)
	}
	if c, ok := v.NewestChild(""); !ok || c.RunID != "run-c" {
		t.Fatalf("any flow: %+v %v", c, ok)
	}
	if _, ok := v.NewestChild("nope"); ok {
		t.Fatal("no child of an unknown flow")
	}
}

func TestHeartbeatStaleAfterThreeIntervals(t *testing.T) {
	v := decode(t)
	beat := time.Date(2026, 10, 19, 5, 0, 0, 0, time.UTC)
	if !v.ProcessAlive() || v.HeartbeatStale(beat.Add(40*time.Second)) {
		t.Fatal("40 s after a beat every 15 s is still fresh")
	}
	if !v.HeartbeatStale(beat.Add(46 * time.Second)) {
		t.Fatal("more than three intervals without a beat is stale")
	}
	v.Process.Alive = false
	if v.HeartbeatStale(beat.Add(time.Hour)) {
		t.Fatal("a dead process is not hung")
	}
	var none RunView
	if none.ProcessAlive() || none.HeartbeatStale(time.Now()) {
		t.Fatal("no process: nothing to be stale")
	}
}
