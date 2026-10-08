package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// seedPlanPhaseLeaves builds plan-1 > phase-1 > leaf-1, leaf-2 (all open).
func seedPlanPhaseLeaves(repo *fakeTaskRepository) {
	repo.tasks["plan-1"] = domain.Task{ID: "plan-1", TenantID: "tenant-1", Title: "plan", Type: domain.TypePlan, ProjectID: "proj-1", RequestID: "req-1", Status: domain.StatusOpen}
	repo.tasks["phase-1"] = domain.Task{ID: "phase-1", TenantID: "tenant-1", Title: "phase", Type: domain.TypePhase, ProjectID: "proj-1", ParentID: "plan-1", RequestID: "req-1", Status: domain.StatusOpen}
	for _, id := range []string{"leaf-1", "leaf-2"} {
		repo.tasks[id] = domain.Task{ID: id, TenantID: "tenant-1", Title: id, Type: domain.TypeTask, ProjectID: "proj-1", ParentID: "phase-1", Status: domain.StatusOpen}
	}
}

func setStatus(repo *fakeTaskRepository, id string, s domain.Status) {
	t := repo.tasks[id]
	t.Status = s
	repo.tasks[id] = t
}

func TestSync_PhaseThenPlan_TwoLevels(t *testing.T) {
	repo := newFakeTaskRepository()
	seedPlanPhaseLeaves(repo)
	uc := NewSyncContainerStatus(repo, NewRecalculateProgress(repo))
	ctx := withIdentity(context.Background(), "tenant-1", "u")

	setStatus(repo, "leaf-1", domain.StatusInProgress)
	if err := uc.Execute(ctx, "leaf-1"); err != nil {
		t.Fatal(err)
	}
	if repo.tasks["phase-1"].Status != domain.StatusInProgress || repo.tasks["plan-1"].Status != domain.StatusInProgress {
		t.Fatalf("phase=%s plan=%s", repo.tasks["phase-1"].Status, repo.tasks["plan-1"].Status)
	}

	setStatus(repo, "leaf-1", domain.StatusDone)
	setStatus(repo, "leaf-2", domain.StatusDone)
	if err := uc.Execute(ctx, "leaf-2"); err != nil {
		t.Fatal(err)
	}
	if repo.tasks["phase-1"].Status != domain.StatusDone || repo.tasks["plan-1"].Status != domain.StatusDone {
		t.Fatalf("phase=%s plan=%s", repo.tasks["phase-1"].Status, repo.tasks["plan-1"].Status)
	}
	if repo.tasks["plan-1"].ProgressPercent != 100 {
		t.Errorf("progress should be recalculated through the plan, got %d", repo.tasks["plan-1"].ProgressPercent)
	}
}

func TestSync_LostCAS_Retries(t *testing.T) {
	repo := newFakeTaskRepository()
	seedPlanPhaseLeaves(repo)
	repo.loseCASTimes = 1
	uc := NewSyncContainerStatus(repo, nil)
	setStatus(repo, "leaf-1", domain.StatusInProgress)
	if err := uc.Execute(withIdentity(context.Background(), "tenant-1", "u"), "leaf-1"); err != nil {
		t.Fatal(err)
	}
	if repo.tasks["phase-1"].Status != domain.StatusInProgress {
		t.Fatalf("retry after a lost CAS must still converge, phase=%s", repo.tasks["phase-1"].Status)
	}
	if repo.updateContainerStatusCalls < 3 { // lost + phase retry + plan
		t.Errorf("expected a retry, calls=%d", repo.updateContainerStatusCalls)
	}
}

func TestSync_GivesUpAfterRepeatedCASLoss(t *testing.T) {
	repo := newFakeTaskRepository()
	seedPlanPhaseLeaves(repo)
	repo.loseCASTimes = 100
	setStatus(repo, "leaf-1", domain.StatusInProgress)
	if err := NewSyncContainerStatus(repo, nil).Execute(withIdentity(context.Background(), "tenant-1", "u"), "leaf-1"); err != nil {
		t.Fatalf("losing the CAS is not an error: %v", err)
	}
	if repo.tasks["phase-1"].Status != domain.StatusOpen {
		t.Errorf("phase must be untouched, got %s", repo.tasks["phase-1"].Status)
	}
}

func TestSync_NoChildren_NoWrite(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.tasks["plan-1"] = domain.Task{ID: "plan-1", TenantID: "tenant-1", Type: domain.TypePlan, Status: domain.StatusInProgress}
	repo.tasks["phase-1"] = domain.Task{ID: "phase-1", TenantID: "tenant-1", Type: domain.TypePhase, ParentID: "plan-1", Status: domain.StatusInProgress}
	repo.tasks["only"] = domain.Task{ID: "only", TenantID: "tenant-1", Type: domain.TypeTask, ParentID: "phase-1", Status: domain.StatusOpen}
	delete(repo.tasks, "only") // phase now has no children
	changed, err := NewSyncContainerStatus(repo, nil).SyncOne(withIdentity(context.Background(), "tenant-1", "u"), "phase-1")
	if err != nil || changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if repo.updateContainerStatusCalls != 0 {
		t.Errorf("expected no write, got %d", repo.updateContainerStatusCalls)
	}
}

func TestSync_CancelledContainer_Untouched(t *testing.T) {
	repo := newFakeTaskRepository()
	seedPlanPhaseLeaves(repo)
	setStatus(repo, "phase-1", domain.StatusCancelled)
	setStatus(repo, "leaf-1", domain.StatusInProgress)
	if err := NewSyncContainerStatus(repo, nil).Execute(withIdentity(context.Background(), "tenant-1", "u"), "leaf-1"); err != nil {
		t.Fatal(err)
	}
	if repo.tasks["phase-1"].Status != domain.StatusCancelled || repo.updateContainerStatusCalls != 0 {
		t.Errorf("cancelled phase must stay cancelled without writes: %s calls=%d", repo.tasks["phase-1"].Status, repo.updateContainerStatusCalls)
	}
}

func TestSync_EmitsStatusChangedWithCauseDerived(t *testing.T) {
	repo := newFakeTaskRepository()
	seedPlanPhaseLeaves(repo)
	setStatus(repo, "leaf-1", domain.StatusDone)
	setStatus(repo, "leaf-2", domain.StatusDone)
	if err := NewSyncContainerStatus(repo, nil).Execute(withIdentity(context.Background(), "tenant-1", "u"), "leaf-1"); err != nil {
		t.Fatal(err)
	}
	if len(repo.containerEvents) != 2 {
		t.Fatalf("one event per changed container (phase, plan), got %d", len(repo.containerEvents))
	}
	for _, ev := range repo.containerEvents {
		if ev.Subject != "orca.task.task.statuschanged" {
			t.Errorf("subject %q: containers must never emit completed", ev.Subject)
		}
		var p taskStatusChangedPayload
		if err := json.Unmarshal(ev.PayloadJSON, &p); err != nil {
			t.Fatal(err)
		}
		if p.Cause != "derived" || p.NewStatus != "done" || p.PreviousStatus != "open" || p.RequestID != "req-1" {
			t.Errorf("payload: %+v", p)
		}
		if p.TaskType != "phase" && p.TaskType != "plan" {
			t.Errorf("task_type: %q", p.TaskType)
		}
	}
}

func TestSync_NonContainerParent_IsNoop(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.tasks["p"] = domain.Task{ID: "p", TenantID: "tenant-1", Type: domain.TypeTask, Status: domain.StatusOpen}
	repo.tasks["c"] = domain.Task{ID: "c", TenantID: "tenant-1", Type: domain.TypeTask, ParentID: "p", Status: domain.StatusDone}
	if err := NewSyncContainerStatus(repo, nil).Execute(withIdentity(context.Background(), "tenant-1", "u"), "c"); err != nil {
		t.Fatal(err)
	}
	if repo.tasks["p"].Status != domain.StatusOpen || repo.updateContainerStatusCalls != 0 {
		t.Error("work-task parent must not be derived")
	}
}

func TestSync_ReopensDoneContainerWhenNewChildUnfinished(t *testing.T) {
	repo := newFakeTaskRepository()
	seedPlanPhaseLeaves(repo)
	setStatus(repo, "plan-1", domain.StatusDone)
	setStatus(repo, "phase-1", domain.StatusDone)
	setStatus(repo, "leaf-1", domain.StatusDone)
	// leaf-2 is open: a late-added unfinished child
	if err := NewSyncContainerStatus(repo, nil).Execute(withIdentity(context.Background(), "tenant-1", "u"), "leaf-2"); err != nil {
		t.Fatal(err)
	}
	if repo.tasks["phase-1"].Status != domain.StatusInProgress {
		t.Errorf("done phase should reopen, got %s", repo.tasks["phase-1"].Status)
	}
}

func TestSync_RepositoryErrorsSurface(t *testing.T) {
	repo := newFakeTaskRepository()
	seedPlanPhaseLeaves(repo)
	repo.listChildStatusesErr = errors.New("boom")
	if err := NewSyncContainerStatus(repo, nil).Execute(withIdentity(context.Background(), "tenant-1", "u"), "leaf-1"); err == nil {
		t.Fatal("expected error")
	}
	if err := NewSyncContainerStatus(repo, nil).Execute(context.Background(), "leaf-1"); err == nil {
		t.Fatal("expected tenant error")
	}
}
