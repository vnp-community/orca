package usecase

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// fakeExecutionRepository is an in-memory ExecutionRepository. Safe for
// concurrent use: usecase.Execute's wave dispatch updates it from a
// background goroutine, which execute_test.go's tests exercise directly.
type fakeExecutionRepository struct {
	mu               sync.Mutex
	executions       map[string]domain.WorkflowExecution
	getErr           error
	updateErr        error
	hasActiveExecErr error
	listRunningErr   error
	listExecErr      error
	// insertOrder records creation order (oldest first) — domain.WorkflowExecution
	// has no CreatedAt field, so ListExecutions' "newest first" fake
	// ordering is derived from this instead of a stored timestamp.
	insertOrder []string
	// onUpdate, if set, is invoked synchronously inside UpdateExecution
	// after the row is stored — a deterministic hook execute_test.go uses
	// to observe "the background dispatch goroutine reached its final
	// status update" without polling or sleeping.
	onUpdate func(domain.WorkflowExecution)
}

func newFakeExecutionRepository() *fakeExecutionRepository {
	return &fakeExecutionRepository{executions: make(map[string]domain.WorkflowExecution)}
}

func (f *fakeExecutionRepository) CreateExecution(ctx context.Context, exec domain.WorkflowExecution) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.executions[exec.ID]; !exists {
		f.insertOrder = append(f.insertOrder, exec.ID)
	}
	f.executions[exec.ID] = exec
	return nil
}

func (f *fakeExecutionRepository) GetExecution(ctx context.Context, tenantID, id string) (domain.WorkflowExecution, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return domain.WorkflowExecution{}, f.getErr
	}
	e, ok := f.executions[id]
	if !ok || e.TenantID != tenantID {
		return domain.WorkflowExecution{}, domain.ErrExecutionNotFound
	}
	return e, nil
}

func (f *fakeExecutionRepository) UpdateExecution(ctx context.Context, exec domain.WorkflowExecution) error {
	f.mu.Lock()
	if f.updateErr != nil {
		f.mu.Unlock()
		return f.updateErr
	}
	f.executions[exec.ID] = exec
	hook := f.onUpdate
	f.mu.Unlock()
	if hook != nil {
		hook(exec)
	}
	return nil
}

func (f *fakeExecutionRepository) HasActiveExecutions(ctx context.Context, tenantID, projectID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.hasActiveExecErr != nil {
		return false, f.hasActiveExecErr
	}
	for _, e := range f.executions {
		if e.TenantID != tenantID || e.ProjectID != projectID {
			continue
		}
		switch e.Status {
		case domain.StatusPending, domain.StatusRunning, domain.StatusPaused:
			return true, nil
		}
	}
	return false, nil
}

// ListRunning implements usecase.ExecutionRepository.ListRunning for
// recover_executions_test.go — deliberately ignores tenant, matching the
// real port method's process-wide contract (see that method's doc
// comment).
func (f *fakeExecutionRepository) ListRunning(ctx context.Context) ([]domain.WorkflowExecution, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listRunningErr != nil {
		return nil, f.listRunningErr
	}
	var out []domain.WorkflowExecution
	for _, e := range f.executions {
		if e.Status == domain.StatusRunning {
			out = append(out, e)
		}
	}
	return out, nil
}

// ListExecutions implements usecase.ExecutionRepository.ListExecutions —
// newest-first (reverse insertion order, since domain.WorkflowExecution
// carries no CreatedAt to sort by), keyset-paginated on the opaque
// last-seen-id cursor, same contract as the real Postgres implementation.
func (f *fakeExecutionRepository) ListExecutions(ctx context.Context, tenantID, projectID, cursor string, limit int32) ([]domain.WorkflowExecution, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listExecErr != nil {
		return nil, "", f.listExecErr
	}

	// newestFirst: insertOrder is oldest-first, so walk it in reverse.
	var newestFirst []domain.WorkflowExecution
	for i := len(f.insertOrder) - 1; i >= 0; i-- {
		e := f.executions[f.insertOrder[i]]
		if e.TenantID != tenantID {
			continue
		}
		if projectID != "" && e.ProjectID != projectID {
			continue
		}
		newestFirst = append(newestFirst, e)
	}

	start := 0
	if cursor != "" {
		for i, e := range newestFirst {
			if e.ID == cursor {
				start = i + 1
				break
			}
		}
	}
	if start > len(newestFirst) {
		start = len(newestFirst)
	}
	page := newestFirst[start:]
	if int32(len(page)) > limit {
		page = page[:limit]
	}

	next := ""
	if len(page) > 0 && start+len(page) < len(newestFirst) {
		next = page[len(page)-1].ID
	}
	return page, next, nil
}

// snapshot returns a copy of id's current stored execution, for tests to
// inspect after synchronizing via onUpdate.
func (f *fakeExecutionRepository) snapshot(id string) (domain.WorkflowExecution, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.executions[id]
	return e, ok
}

// count returns how many executions are currently stored — used to assert
// a synchronous validation failure never persisted anything.
func (f *fakeExecutionRepository) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.executions)
}

func TestPauseExecution_RunningTransitionsToPaused(t *testing.T) {
	repo := newFakeExecutionRepository()
	exec, err := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1", "")
	if err != nil {
		t.Fatalf("building execution: %v", err)
	}
	_ = repo.CreateExecution(context.Background(), exec)

	uc := NewPauseExecution(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	got, err := uc.Execute(ctx, PauseExecutionInput{ID: "exec-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != domain.StatusPaused {
		t.Errorf("expected status=paused, got %v", got.Status)
	}
	if got.PausedAt == nil {
		t.Error("expected PausedAt to be set")
	}
	if repo.executions["exec-1"].Status != domain.StatusPaused {
		t.Error("expected the repository's stored execution to reflect the pause")
	}
}

func TestPauseExecution_RejectsNonRunningExecution(t *testing.T) {
	repo := newFakeExecutionRepository()
	exec, _ := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1", "")
	// Force it into a terminal state a real running execution would reach
	// on its own — pausing from here must be rejected.
	exec.Status = domain.StatusCompleted
	_ = repo.CreateExecution(context.Background(), exec)

	uc := NewPauseExecution(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, PauseExecutionInput{ID: "exec-1"})
	if err == nil {
		t.Fatal("expected an error pausing a non-running execution")
	}
}

func TestResumeExecution_PausedTransitionsToRunning(t *testing.T) {
	repo := newFakeExecutionRepository()
	exec, _ := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1", "")
	_ = exec.Pause(time.Now().UTC())
	_ = repo.CreateExecution(context.Background(), exec)

	uc := NewResumeExecution(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	got, err := uc.Execute(ctx, ResumeExecutionInput{ID: "exec-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != domain.StatusRunning {
		t.Errorf("expected status=running, got %v", got.Status)
	}
	if got.PausedAt != nil {
		t.Error("expected PausedAt to be cleared on resume")
	}
}

func TestResumeExecution_RejectsNonPausedExecution(t *testing.T) {
	repo := newFakeExecutionRepository()
	// Still running — never paused.
	exec, _ := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1", "")
	_ = repo.CreateExecution(context.Background(), exec)

	uc := NewResumeExecution(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ResumeExecutionInput{ID: "exec-1"})
	if err == nil {
		t.Fatal("expected an error resuming a non-paused (running) execution — this is the real invariant check")
	}
}

func TestResumeExecution_ExecutionNotFound(t *testing.T) {
	repo := newFakeExecutionRepository()
	uc := NewResumeExecution(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ResumeExecutionInput{ID: "does-not-exist"})
	if err == nil {
		t.Fatal("expected a not-found error")
	}
}
