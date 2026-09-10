package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type fakeAIProviderContextResolver struct {
	ctx string
	err error
}

func (f *fakeAIProviderContextResolver) ResolveContext(ctx context.Context, tenantID, userID string) (string, error) {
	return f.ctx, f.err
}

type fakeProjectExecutionResolver struct {
	connectionID string
	worktreePath string
	connected    bool
	err          error
}

func (f *fakeProjectExecutionResolver) ResolveConnection(ctx context.Context, tenantID, projectID string) (string, string, bool, error) {
	return f.connectionID, f.worktreePath, f.connected, f.err
}

type fakeAICompleter struct {
	content   string
	err       error
	gotConn   string
	gotPrompt string
}

func (f *fakeAICompleter) Complete(ctx context.Context, connectionID, prompt string) (string, error) {
	f.gotConn = connectionID
	f.gotPrompt = prompt
	return f.content, f.err
}

type fakeTechStackDetector struct {
	stack []string
	err   error
}

func (f *fakeTechStackDetector) Detect(ctx context.Context, id string) ([]string, error) {
	return f.stack, f.err
}

func TestAIDecompose_RequiresTenantContext(t *testing.T) {
	uc := NewAIDecompose(newFakeTaskRepository(), &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{}, &fakeAICompleter{}, &fakeTechStackDetector{})
	if _, err := uc.Execute(context.Background(), AIDecomposeInput{TaskID: "t1"}); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// TestAIDecompose_NotConnected_ReturnsFailedPrecondition is TASK-224's
// core regression guard: a not-connected project must never silently
// return an empty proposal list.
func TestAIDecompose_NotConnected_ReturnsFailedPrecondition(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1"}
	uc := NewAIDecompose(tasks, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{connected: false}, &fakeAICompleter{}, &fakeTechStackDetector{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "t1"})
	if err == nil {
		t.Fatal("expected error when project has no connected dev server")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindFailedPrecondition {
		t.Fatalf("expected KindFailedPrecondition — a not-connected project must never silently return an empty proposal list, got %v", err)
	}
}

func TestAIDecompose_Connected_ReturnsParsedProposals(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1", Title: "Build widget"}
	completer := &fakeAICompleter{content: `[{"title": "Design API"}, {"title": "Implement handler"}]`}
	uc := NewAIDecompose(tasks, &fakeAIProviderContextResolver{ctx: "anthropic"}, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, completer, &fakeTechStackDetector{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one parsed proposal")
	}
	if completer.gotConn != "conn-1" {
		t.Errorf("expected resolved connectionID to be passed through, got %q", completer.gotConn)
	}
}

// TestAIDecompose_PromptIncludesAllFiveContextSources locks in BE-SOL-002's
// 5-source context bundle: title, description, AI context, tech stack, and
// existing-subtask titles must all appear in the generated ai.complete
// prompt.
func TestAIDecompose_PromptIncludesAllFiveContextSources(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{
		ID: "t1", TenantID: "tenant-1", ProjectID: "p1",
		Title: "Build widget", Description: "A widget that does X",
		AIContext: []byte(`"repo uses feature flags"`),
	}
	tasks.tasks["existing-1"] = domain.Task{ID: "existing-1", TenantID: "tenant-1", ParentID: "t1", Title: "Already-proposed subtask"}
	completer := &fakeAICompleter{content: `[{"title": "Design API"}]`}
	techStack := &fakeTechStackDetector{stack: []string{"go", "node"}}
	uc := NewAIDecompose(tasks, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, completer, techStack)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "t1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prompt := completer.gotPrompt
	for _, want := range []string{"Build widget", "A widget that does X", "repo uses feature flags", "go", "node", "Already-proposed subtask"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("expected prompt to contain %q, got:\n%s", want, prompt)
		}
	}
}

// TestAIDecompose_TechStackDetectorFailure_NeverFailsExecute is the
// best-effort regression test: a TechStackDetector error must never
// propagate to AIDecompose.Execute's own error.
func TestAIDecompose_TechStackDetectorFailure_NeverFailsExecute(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1", Title: "Build widget"}
	uc := NewAIDecompose(tasks, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, &fakeAICompleter{content: `[{"title": "Do X"}]`}, &fakeTechStackDetector{err: errors.New("git-gateway-service unreachable")})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "t1"}); err != nil {
		t.Fatalf("expected TechStackDetector's failure to be swallowed, got %v", err)
	}
}

// TestAIDecompose_BuildContext_ListsExistingSubtaskTitles is
// BE-SOL-002's own named test-plan item: ListChildren on a task with 2
// existing subtasks returns exactly those 2 titles in ExistingSubtasks.
func TestAIDecompose_BuildContext_ListsExistingSubtaskTitles(t *testing.T) {
	tasks := newFakeTaskRepository()
	parent := domain.Task{ID: "parent", TenantID: "tenant-1", Title: "Parent"}
	tasks.tasks["parent"] = parent
	tasks.tasks["child-1"] = domain.Task{ID: "child-1", TenantID: "tenant-1", ParentID: "parent", Title: "Child one"}
	tasks.tasks["child-2"] = domain.Task{ID: "child-2", TenantID: "tenant-1", ParentID: "parent", Title: "Child two"}
	uc := NewAIDecompose(tasks, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{}, &fakeAICompleter{}, &fakeTechStackDetector{})

	decCtx := uc.buildContext(withIdentity(context.Background(), "tenant-1", "user-1"), "tenant-1", parent)
	if len(decCtx.ExistingSubtasks) != 2 {
		t.Fatalf("expected exactly 2 existing subtask titles, got %d: %+v", len(decCtx.ExistingSubtasks), decCtx.ExistingSubtasks)
	}
	seen := map[string]bool{}
	for _, title := range decCtx.ExistingSubtasks {
		seen[title] = true
	}
	if !seen["Child one"] || !seen["Child two"] {
		t.Errorf("expected both child titles present, got %+v", decCtx.ExistingSubtasks)
	}
}

// TestAIDecompose_MalformedJSON_ReturnsErrorNotEmptyResult is
// TASK-TG-002-02's explicit-error-contract regression test: malformed AI
// output must surface a real error, never silently degrade to an empty
// proposal list the way the old free-text parser would have.
func TestAIDecompose_MalformedJSON_ReturnsErrorNotEmptyResult(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1"}
	uc := NewAIDecompose(tasks, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, &fakeAICompleter{content: "not json at all"}, &fakeTechStackDetector{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "t1"})
	if err == nil {
		t.Fatal("expected an error for malformed AI JSON output")
	}
	if got != nil {
		t.Errorf("expected a nil proposal slice on parse failure, got %+v", got)
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "TASK_AI_DECOMPOSE_PARSE_FAILED" {
		t.Errorf("expected TASK_AI_DECOMPOSE_PARSE_FAILED, got %v", err)
	}
}

func TestAIDecompose_TaskNotFound(t *testing.T) {
	uc := NewAIDecompose(newFakeTaskRepository(), &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{}, &fakeAICompleter{}, &fakeTechStackDetector{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "does-not-exist"}); err == nil {
		t.Fatal("expected an error for a nonexistent task")
	}
}

func TestAIDecompose_ProviderResolveFailurePropagates(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1"}
	uc := NewAIDecompose(tasks, &fakeAIProviderContextResolver{err: errors.New("boom")}, &fakeProjectExecutionResolver{connected: true}, &fakeAICompleter{}, &fakeTechStackDetector{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "t1"}); err == nil {
		t.Fatal("expected an error when AI provider context resolution fails")
	}
}

func TestAIDecompose_RelayFailurePropagates(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1"}
	uc := NewAIDecompose(tasks, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, &fakeAICompleter{err: errors.New("boom")}, &fakeTechStackDetector{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "t1"}); err == nil {
		t.Fatal("expected an error when the AI relay call fails")
	}
}
