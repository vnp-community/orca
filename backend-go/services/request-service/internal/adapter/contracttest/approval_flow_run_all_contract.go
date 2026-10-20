package contracttest

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// The "approve and run all steps" cases (CR-REQ-043). What a fake cannot show is that the pin and the decision are one
// commit on a real database, and that they roll back together.

func (r *approvalRig) openPlanApproval(t *testing.T, tenantID string, req domain.Request) domain.Approval {
	t.Helper()
	a, err := r.open.Execute(asUser(tenantID, req.ReporterID, "user"), usecase.OpenApprovalInput{RequestID: req.ID, SubjectType: domain.SubjectPlan})
	if err != nil {
		t.Fatal(err)
	}
	return *a
}

func (r *approvalRig) pinner(t *testing.T) usecase.ExecutionModePinner {
	t.Helper()
	p, ok := r.env.Requests.(usecase.ExecutionModePinner)
	if !ok {
		t.Fatal("the request repository must pin execution modes")
	}
	r.decide.Pinner = p
	return p
}

func decidedModes(t *testing.T, rig *approvalRig, tenantID string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, row := range rowsOf(rig.env, t, tenantID, domain.SubjectApprovalDecided) {
		var m map[string]any
		if err := json.Unmarshal(row.Payload, &m); err != nil {
			t.Fatal(err)
		}
		out = append(out, m)
	}
	return out
}

func approveWith(a domain.Approval, mode string) usecase.DecideApprovalInput {
	return usecase.DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest, ExecutionMode: mode}
}

func flowRunAllPinsWithTheDecision(t *testing.T, rig *approvalRig) {
	rig.pinner(t)
	tenantID := newTenant()
	req := rig.planRequest(t, tenantID)
	a := rig.openPlanApproval(t, tenantID, req)
	ctx := asUser(tenantID, req.ReporterID, "user")

	res, err := rig.decide.Execute(ctx, approveWith(a, "continuous"))
	if err != nil {
		t.Fatal(err)
	}
	got := mustGetRequest(t, rig, tenantID, req.ID)
	if got.ExecutionMode != domain.ExecutionContinuous || got.Status != domain.RequestStatusExecuting || res.ExecutionMode != domain.ExecutionContinuous {
		t.Fatalf("request = %s/%q, result mode = %q", got.Status, got.ExecutionMode, res.ExecutionMode)
	}
	if rows := decidedModes(t, rig, tenantID); len(rows) != 1 || rows[0]["execution_mode"] != "continuous" {
		t.Fatalf("decided rows = %v", rows)
	}

	// A double click: the decision is not repeated, and the answer states the mode that is pinned.
	again, err := rig.decide.Execute(ctx, approveWith(a, "continuous"))
	if err != nil || again.ExecutionMode != domain.ExecutionContinuous {
		t.Fatalf("second call: %+v %v", again, err)
	}
	if rows := decidedModes(t, rig, tenantID); len(rows) != 1 {
		t.Fatalf("a repeat must not write a second decided row, got %d", len(rows))
	}
}

func flowApproveWithoutModeLeavesItUnpinned(t *testing.T, rig *approvalRig) {
	rig.pinner(t)
	tenantID := newTenant()
	req := rig.planRequest(t, tenantID)
	a := rig.openPlanApproval(t, tenantID, req)
	res, err := rig.decide.Execute(asUser(tenantID, req.ReporterID, "user"), approveWith(a, ""))
	if err != nil {
		t.Fatal(err)
	}
	if got := mustGetRequest(t, rig, tenantID, req.ID); got.ExecutionMode != "" || res.ExecutionMode != "" || got.Status != domain.RequestStatusExecuting {
		t.Fatalf("a plain approval pins nothing: %s/%q/%q", got.Status, got.ExecutionMode, res.ExecutionMode)
	}
	rows := decidedModes(t, rig, tenantID)
	if len(rows) != 1 {
		t.Fatalf("decided rows = %d", len(rows))
	}
	if _, has := rows[0]["execution_mode"]; has {
		t.Fatalf("the key must be absent when no mode was chosen: %v", rows[0])
	}
}

func flowRunAllRollsBackWithTheHandler(t *testing.T, rig *approvalRig) {
	rig.pinner(t)
	tenantID := newTenant()
	req := rig.planRequest(t, tenantID)
	a := rig.openPlanApproval(t, tenantID, req)
	ctx := asUser(tenantID, req.ReporterID, "user")

	rig.art.setFail(true)
	if _, err := rig.decide.Execute(ctx, approveWith(a, "continuous")); err == nil {
		t.Fatal("the handler failure must surface")
	}
	rig.art.setFail(false)
	got, err := rig.env.Approvals.Get(ctx, tenantID, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	cur := mustGetRequest(t, rig, tenantID, req.ID)
	if got.Status != domain.ApprovalStatusPending || got.Version != 1 || cur.ExecutionMode != "" || cur.Status != domain.RequestStatusAwaitingPlanApproval {
		t.Fatalf("pin must roll back with the decision: approval %s v%d, request %s/%q", got.Status, got.Version, cur.Status, cur.ExecutionMode)
	}
	if n := len(decidedModes(t, rig, tenantID)); n != 0 {
		t.Fatalf("decided rows = %d", n)
	}
	if _, err := rig.decide.Execute(ctx, approveWith(a, "continuous")); err != nil {
		t.Fatalf("a retry after the failure must work: %v", err)
	}
	if mustGetRequest(t, rig, tenantID, req.ID).ExecutionMode != domain.ExecutionContinuous {
		t.Fatal("the retry pins the mode")
	}
}

func flowRunAllRefusedWhenPinnedStepByStep(t *testing.T, rig *approvalRig) {
	pinner := rig.pinner(t)
	tenantID := newTenant()
	req := rig.planRequest(t, tenantID)
	a := rig.openPlanApproval(t, tenantID, req)
	ctx := asUser(tenantID, req.ReporterID, "user")
	if got, err := pinner.PinExecutionModeIfUnset(ctx, req.ID, domain.ExecutionStepByStep); err != nil || got != domain.ExecutionStepByStep {
		t.Fatalf("seed pin: %q %v", got, err)
	}

	_, err := rig.decide.Execute(ctx, approveWith(a, "continuous"))
	requireCode(t, err, "REQUEST_EXECUTION_MODE_ALREADY_PINNED")
	stored, _ := rig.env.Approvals.Get(ctx, tenantID, a.ID)
	cur := mustGetRequest(t, rig, tenantID, req.ID)
	if stored.Status != domain.ApprovalStatusPending || cur.Status != domain.RequestStatusAwaitingPlanApproval || cur.ExecutionMode != domain.ExecutionStepByStep {
		t.Fatalf("a refused decision leaves everything as it was: approval %s, request %s/%q", stored.Status, cur.Status, cur.ExecutionMode)
	}
	res, err := rig.decide.Execute(ctx, approveWith(a, ""))
	if err != nil || res.ExecutionMode != domain.ExecutionStepByStep {
		t.Fatalf("a plain approval still works and reports the pinned mode: %+v %v", res, err)
	}
}

func flowRunAllRacesAPlainApproval(t *testing.T, rig *approvalRig) {
	rig.pinner(t)
	for i := 0; i < 12; i++ {
		tenantID := newTenant()
		req := rig.planRequest(t, tenantID)
		a := rig.openPlanApproval(t, tenantID, req)
		inputs := []usecase.DecideApprovalInput{approveWith(a, "continuous"), approveWith(a, "")}
		errs := make([]error, len(inputs))
		var wg sync.WaitGroup
		for j, in := range inputs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[j] = retryDeadlock(func() error {
					_, err := rig.decide.Execute(asUser(tenantID, uuid.NewString(), "admin"), in)
					return err
				})
			}()
		}
		wg.Wait()
		winner := -1
		for j, err := range errs {
			switch {
			case err == nil && winner == -1:
				winner = j
			case err == nil:
				t.Fatalf("iteration %d: both decisions succeeded", i)
			case errCode(err) != "REQUEST_APPROVAL_ALREADY_DECIDED":
				t.Fatalf("iteration %d: the loser must see ALREADY_DECIDED, got %v", i, err)
			}
		}
		if winner == -1 {
			t.Fatalf("iteration %d: nobody won: %v", i, errs)
		}
		want := domain.ExecutionMode("")
		if winner == 0 {
			want = domain.ExecutionContinuous
		}
		if got := mustGetRequest(t, rig, tenantID, req.ID).ExecutionMode; got != want {
			t.Fatalf("iteration %d: winner %d pinned %q, want %q", i, winner, got, want)
		}
		if n := len(decidedModes(t, rig, tenantID)); n != 1 {
			t.Fatalf("iteration %d: decided rows = %d", i, n)
		}
	}
}
