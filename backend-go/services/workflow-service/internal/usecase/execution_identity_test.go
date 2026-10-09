package usecase

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// identityRecorder is a step executor that records who the step ran as.
type identityRecorder struct {
	mu     sync.Mutex
	tenant string
	user   string
	seen   chan struct{}
}

func newIdentityRecorder() *identityRecorder { return &identityRecorder{seen: make(chan struct{}, 4)} }

func (r *identityRecorder) Execute(ctx context.Context, _ string) (domain.StepResult, error) {
	r.mu.Lock()
	r.tenant, _ = tenant.TenantID(ctx)
	r.user, _ = tenant.UserID(ctx)
	r.mu.Unlock()
	r.seen <- struct{}{}
	return domain.StepResult{Status: domain.ResultStatusCompleted}, nil
}

func (r *identityRecorder) who() (string, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tenant, r.user
}

func waitSeen(t *testing.T, r *identityRecorder) {
	t.Helper()
	select {
	case <-r.seen:
	case <-time.After(5 * time.Second):
		t.Fatal("the step never ran")
	}
}

func TestExecute_StepsRunAsTheUserWhoStartedTheExecution(t *testing.T) {
	templates := newFakeTemplateRepository()
	tmpl, _ := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "t", `{"steps":[{"id":"a","type":"shell"}]}`, domain.ScopePersonal, "", "owner-1")
	_ = templates.CreateTemplate(context.Background(), tmpl)
	executions := newFakeExecutionRepository()
	rec := newIdentityRecorder()
	registry := newFakeRegistry()
	registry.executors[domain.StepTypeShell] = rec

	uc := NewExecute(templates, executions, newFakeStepExecutionRepository(), registry, &fakeTaskClient{})
	// The RPC context is cancelled as soon as Execute returns, like a real handler's.
	rpcCtx, cancel := context.WithCancel(tenant.WithUserID(withTenantContext(context.Background(), "tenant-1"), "user-7"))
	exec, err := uc.Execute(rpcCtx, ExecuteInput{TemplateID: "tmpl-1"})
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	waitSeen(t, rec)

	if got, user := rec.who(); got != "tenant-1" || user != "user-7" {
		t.Fatalf("step ran as tenant=%q user=%q, want tenant-1 / user-7", got, user)
	}
	if exec.StartedBy != "user-7" {
		t.Fatalf("StartedBy = %q, want user-7", exec.StartedBy)
	}
	stored, _ := executions.GetExecution(context.Background(), "tenant-1", exec.ID)
	if stored.StartedBy != "user-7" {
		t.Fatalf("persisted StartedBy = %q", stored.StartedBy)
	}
}

func TestRecoverExecutions_ResumesAsTheUserWhoStartedIt(t *testing.T) {
	templates := newFakeTemplateRepository()
	tmpl, _ := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "t", `{"steps":[{"id":"a","type":"shell"}]}`, domain.ScopePersonal, "", "owner-1")
	_ = templates.CreateTemplate(context.Background(), tmpl)
	exec, _ := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1", "", "")
	exec.StartedBy = "user-7"
	executions := newFakeExecutionRepository()
	_ = executions.CreateExecution(context.Background(), exec)

	rec := newIdentityRecorder()
	registry := newFakeRegistry()
	registry.executors[domain.StepTypeShell] = rec

	uc := NewRecoverExecutions(templates, executions, newFakeStepExecutionRepository(), registry, &fakeTaskClient{})
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitSeen(t, rec)
	if got, user := rec.who(); got != "tenant-1" || user != "user-7" {
		t.Fatalf("recovered step ran as tenant=%q user=%q, want tenant-1 / user-7", got, user)
	}
}

func TestRecoverExecutions_RowWithoutStartedByResumesTenantOnly(t *testing.T) {
	templates := newFakeTemplateRepository()
	tmpl, _ := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "t", `{"steps":[{"id":"a","type":"shell"}]}`, domain.ScopePersonal, "", "owner-1")
	_ = templates.CreateTemplate(context.Background(), tmpl)
	exec, _ := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1", "", "")
	executions := newFakeExecutionRepository()
	_ = executions.CreateExecution(context.Background(), exec)

	rec := newIdentityRecorder()
	registry := newFakeRegistry()
	registry.executors[domain.StepTypeShell] = rec

	uc := NewRecoverExecutions(templates, executions, newFakeStepExecutionRepository(), registry, &fakeTaskClient{})
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitSeen(t, rec)
	if got, user := rec.who(); got != "tenant-1" || user != "" {
		t.Fatalf("legacy row ran as tenant=%q user=%q, want tenant-1 and no user", got, user)
	}
}
