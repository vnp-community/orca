package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// taskLister is a TaskClient whose only working call is ListTasks.
type taskLister struct {
	TaskClient
	mu    sync.Mutex
	tasks []domain.TaskView
	err   error
	calls []ListTasksQuery
}

func (l *taskLister) ListTasks(_ context.Context, q ListTasksQuery) ([]domain.TaskView, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, q)
	return l.tasks, l.err
}

func TestWorktreePresence_Has(t *testing.T) {
	req := uuid.NewString()
	t.Run("the stored answer wins and task-service is not asked", func(t *testing.T) {
		stored, tasks := newFakePresence(), &taskLister{}
		stored.set(req, domain.BaseRefLockedProposal)
		p := &TaskAwareWorktreePresence{Stored: stored, Tasks: tasks}
		present, reason, err := p.Has(tctx(), req)
		if err != nil || !present || reason != domain.BaseRefLockedProposal || len(tasks.calls) != 0 {
			t.Fatalf("%v %q %v calls=%d", present, reason, err, len(tasks.calls))
		}
	})
	t.Run("an executing Request is already locked by the stored part, without asking task-service", func(t *testing.T) {
		stored, tasks := newFakePresence(), &taskLister{err: errors.New("must not be called")}
		stored.set(req, domain.BaseRefLockedExecution)
		p := &TaskAwareWorktreePresence{Stored: stored, Tasks: tasks}
		present, reason, err := p.Has(tctx(), req)
		if err != nil || !present || reason != domain.BaseRefLockedExecution || len(tasks.calls) != 0 {
			t.Fatalf("%v %q %v calls=%d", present, reason, err, len(tasks.calls))
		}
		many, err := p.HasMany(tctx(), []string{req})
		if err != nil || many[req] != domain.BaseRefLockedExecution || len(tasks.calls) != 0 {
			t.Fatalf("%v %v calls=%d", many, err, len(tasks.calls))
		}
	})
	t.Run("a task with a worktree locks", func(t *testing.T) {
		tasks := &taskLister{tasks: []domain.TaskView{{ID: "t1", RequestID: req}, {ID: "t2", RequestID: req, WorktreeID: "wt-1"}}}
		p := &TaskAwareWorktreePresence{Stored: newFakePresence(), Tasks: tasks}
		present, reason, err := p.Has(tctx(), req)
		if err != nil || !present || reason != domain.BaseRefLockedTask {
			t.Fatalf("%v %q %v", present, reason, err)
		}
		if len(tasks.calls) != 1 || len(tasks.calls[0].RequestIDs) != 1 || tasks.calls[0].RequestIDs[0] != req {
			t.Fatalf("one ListTasks scoped to the request: %+v", tasks.calls)
		}
	})
	t.Run("tasks without a worktree do not lock", func(t *testing.T) {
		tasks := &taskLister{tasks: []domain.TaskView{{ID: "t1", RequestID: req}, {ID: "t2", RequestID: req}}}
		p := &TaskAwareWorktreePresence{Stored: newFakePresence(), Tasks: tasks}
		if present, reason, err := p.Has(tctx(), req); err != nil || present || reason != domain.BaseRefUnlocked {
			t.Fatalf("%v %q %v", present, reason, err)
		}
	})
	t.Run("no tasks at all", func(t *testing.T) {
		p := &TaskAwareWorktreePresence{Stored: newFakePresence(), Tasks: &taskLister{}}
		if present, _, err := p.Has(tctx(), req); err != nil || present {
			t.Fatalf("%v %v", present, err)
		}
	})
	t.Run("task-service down is an error, never 'no worktree'", func(t *testing.T) {
		p := &TaskAwareWorktreePresence{Stored: newFakePresence(), Tasks: &taskLister{err: errors.New("connection refused")}}
		present, _, err := p.Has(tctx(), req)
		var ae *apperrors.AppError
		if present || !errors.As(err, &ae) || ae.Code != "REQUEST_EXECUTION_TASK_SERVICE_UNAVAILABLE" || ae.Kind != apperrors.KindUnavailable {
			t.Fatalf("%v %v", present, err)
		}
	})
	t.Run("a stored failure stops before task-service", func(t *testing.T) {
		stored, tasks := newFakePresence(), &taskLister{}
		stored.err = domain.ErrRequestNotFound(req)
		p := &TaskAwareWorktreePresence{Stored: stored, Tasks: tasks}
		_, _, err := p.Has(tctx(), req)
		mustCode(t, err, "REQUEST_NOT_FOUND")
		if len(tasks.calls) != 0 {
			t.Fatal("no task lookup for an unknown Request")
		}
	})
}

func TestWorktreePresence_HasMany_OneTaskLookupForTheUnlockedRest(t *testing.T) {
	stored, tasks := newFakePresence(), &taskLister{}
	var ids []string
	for i := 0; i < 30; i++ {
		ids = append(ids, uuid.NewString())
	}
	stored.set(ids[0], domain.BaseRefLockedProposal)
	stored.set(ids[1], domain.BaseRefLockedTerminal)
	tasks.tasks = []domain.TaskView{{RequestID: ids[2], WorktreeID: "wt"}, {RequestID: ids[2], WorktreeID: "wt2"}, {RequestID: ids[3]}, {RequestID: ids[4], WorktreeID: "wt"}}
	p := &TaskAwareWorktreePresence{Stored: stored, Tasks: tasks}

	got, err := p.HasMany(tctx(), ids)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]domain.BaseRefLockReason{ids[0]: domain.BaseRefLockedProposal, ids[1]: domain.BaseRefLockedTerminal, ids[2]: domain.BaseRefLockedTask, ids[4]: domain.BaseRefLockedTask}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for id, reason := range want {
		if got[id] != reason {
			t.Errorf("%s: %q, want %q", id, got[id], reason)
		}
	}
	if stored.manyCalls != 1 || len(tasks.calls) != 1 {
		t.Fatalf("one stored lookup and one task lookup for the page, got %d and %d", stored.manyCalls, len(tasks.calls))
	}
	if len(tasks.calls[0].RequestIDs) != 28 {
		t.Fatalf("task-service is asked only about the %d Requests the stored part did not lock, got %d", 28, len(tasks.calls[0].RequestIDs))
	}
}

func TestWorktreePresence_HasMany_NothingLeftSkipsTaskService(t *testing.T) {
	stored, tasks := newFakePresence(), &taskLister{}
	a := uuid.NewString()
	stored.set(a, domain.BaseRefLockedProposal)
	p := &TaskAwareWorktreePresence{Stored: stored, Tasks: tasks}
	if got, err := p.HasMany(tctx(), []string{a}); err != nil || got[a] != domain.BaseRefLockedProposal || len(tasks.calls) != 0 {
		t.Fatalf("%v %v calls=%d", got, err, len(tasks.calls))
	}
}

func TestWorktreePresence_HasMany_Failures(t *testing.T) {
	ids := []string{uuid.NewString()}
	stored := newFakePresence()
	stored.err = errors.New("db down")
	if _, err := (&TaskAwareWorktreePresence{Stored: stored, Tasks: &taskLister{}}).HasMany(tctx(), ids); err == nil {
		t.Fatal("a stored failure must surface")
	}
	_, err := (&TaskAwareWorktreePresence{Stored: newFakePresence(), Tasks: &taskLister{err: errors.New("x")}}).HasMany(tctx(), ids)
	mustCode(t, err, "REQUEST_EXECUTION_TASK_SERVICE_UNAVAILABLE")
}

func lockPageOf(n int, mod func(i int, r *domain.Request)) []domain.Request {
	out := make([]domain.Request, n)
	for i := range out {
		out[i] = domain.Request{ID: uuid.NewString(), Status: domain.RequestStatusExecuting}
		if mod != nil {
			mod(i, &out[i])
		}
	}
	return out
}

func TestBaseRefLockAnnotator_OneLookupForThePage_NoNPlusOne(t *testing.T) {
	presence := newFakePresence()
	reqs := lockPageOf(50, func(i int, r *domain.Request) {
		if i%10 == 9 {
			r.Status = domain.RequestStatusCompleted
		}
	})
	presence.set(reqs[0].ID, domain.BaseRefLockedProposal)
	presence.set(reqs[1].ID, domain.BaseRefLockedTask)
	(&BaseRefLockAnnotator{Presence: presence}).Annotate(tctx(), reqs)

	if presence.manyCalls != 1 || presence.hasCalls != 0 {
		t.Fatalf("a page of 50 must cost one HasMany and no Has, got %d and %d", presence.manyCalls, presence.hasCalls)
	}
	if got := len(presence.manyIDs[0]); got != 45 {
		t.Fatalf("finished Requests are decided from their status and not looked up: asked about %d", got)
	}
	for i, r := range reqs {
		wantLocked, wantReason := false, domain.BaseRefUnlocked
		switch {
		case i == 0:
			wantLocked, wantReason = true, domain.BaseRefLockedProposal
		case i == 1:
			wantLocked, wantReason = true, domain.BaseRefLockedTask
		case i%10 == 9:
			wantLocked, wantReason = true, domain.BaseRefLockedTerminal
		}
		if r.BaseRefLocked != wantLocked || r.BaseRefLockReason != wantReason {
			t.Errorf("request %d: locked=%v reason=%q, want %v %q", i, r.BaseRefLocked, r.BaseRefLockReason, wantLocked, wantReason)
		}
	}
}

func TestBaseRefLockAnnotator_AllFinishedMakesNoLookup(t *testing.T) {
	presence := newFakePresence()
	reqs := lockPageOf(3, func(_ int, r *domain.Request) { r.Status = domain.RequestStatusCancelled })
	(&BaseRefLockAnnotator{Presence: presence}).Annotate(tctx(), reqs)
	if presence.manyCalls != 0 || !reqs[0].BaseRefLocked || reqs[0].BaseRefLockReason != domain.BaseRefLockedTerminal {
		t.Fatalf("calls=%d %+v", presence.manyCalls, reqs[0])
	}
}

func TestBaseRefLockAnnotator_LookupFailureLeavesTheReadWorking(t *testing.T) {
	presence := newFakePresence()
	presence.err = errors.New("task-service down")
	reqs := lockPageOf(2, nil)
	(&BaseRefLockAnnotator{Presence: presence}).Annotate(tctx(), reqs)
	if reqs[0].BaseRefLocked || reqs[1].BaseRefLocked {
		t.Fatal("an unreadable lock must not claim a lock")
	}
}

func TestBaseRefLockAnnotator_NilIsSafe(t *testing.T) {
	var a *BaseRefLockAnnotator
	a.Annotate(tctx(), lockPageOf(1, nil))
	(&BaseRefLockAnnotator{}).Annotate(tctx(), lockPageOf(1, nil))
	(&BaseRefLockAnnotator{Presence: newFakePresence()}).Annotate(tctx(), nil)
}

// countingRepo serves one page from List and one Request from Get.
type countingRepo struct {
	RequestRepository
	page []domain.Request
}

func (r *countingRepo) List(context.Context, ListFilter) (ListResult, error) {
	return ListResult{Requests: append([]domain.Request(nil), r.page...)}, nil
}
func (r *countingRepo) Get(_ context.Context, id string) (domain.Request, error) {
	for _, q := range r.page {
		if q.ID == id {
			return q, nil
		}
	}
	return domain.Request{}, domain.ErrRequestNotFound(id)
}

func TestListRequests_BaseRefLock_CostsOneLookup(t *testing.T) {
	presence := newFakePresence()
	repo := &countingRepo{page: lockPageOf(40, nil)}
	presence.set(repo.page[7].ID, domain.BaseRefLockedProposal)
	uc := NewListRequests(repo).WithBaseRefLock(&BaseRefLockAnnotator{Presence: presence})
	res, err := uc.Execute(tctx(), ListFilter{})
	if err != nil || len(res.Requests) != 40 {
		t.Fatalf("%v %d", err, len(res.Requests))
	}
	if presence.manyCalls != 1 || presence.hasCalls != 0 {
		t.Fatalf("List of 40 cost %d HasMany and %d Has calls", presence.manyCalls, presence.hasCalls)
	}
	for i, r := range res.Requests {
		if r.BaseRefLocked != (i == 7) {
			t.Errorf("request %d locked=%v", i, r.BaseRefLocked)
		}
	}
}

func TestGetRequest_BaseRefLock(t *testing.T) {
	presence := newFakePresence()
	repo := &countingRepo{page: lockPageOf(1, nil)}
	presence.set(repo.page[0].ID, domain.BaseRefLockedTask)
	got, err := NewGetRequest(repo).WithBaseRefLock(&BaseRefLockAnnotator{Presence: presence}).Execute(tctx(), repo.page[0].ID)
	if err != nil || !got.BaseRefLocked || got.BaseRefLockReason != domain.BaseRefLockedTask || presence.manyCalls != 1 {
		t.Fatalf("%+v %v calls=%d", got, err, presence.manyCalls)
	}
	// Without an annotator the read still works and says nothing about the lock.
	plain, err := NewGetRequest(repo).Execute(tctx(), repo.page[0].ID)
	if err != nil || plain.BaseRefLocked {
		t.Fatalf("%+v %v", plain, err)
	}
}
