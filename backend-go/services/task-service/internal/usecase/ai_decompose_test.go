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
	worktreeID   string
	connected    bool
	err          error
}

func (f *fakeProjectExecutionResolver) ResolveConnection(ctx context.Context, tenantID, projectID string) (string, string, string, bool, error) {
	return f.connectionID, f.worktreePath, f.worktreeID, f.connected, f.err
}

type fakeProjectContextResolver struct {
	name    string
	repoURL string
	err     error
}

func (f *fakeProjectContextResolver) Resolve(ctx context.Context, tenantID, projectID string) (string, string, error) {
	return f.name, f.repoURL, f.err
}

type fakeTechStackDetector struct {
	stack domain.TechStack
	err   error
}

func (f *fakeTechStackDetector) Detect(ctx context.Context, tenantID, projectID string) (domain.TechStack, error) {
	return f.stack, f.err
}

type fakeVelocityResolver struct {
	tasks []domain.Task
	err   error
}

func (f *fakeVelocityResolver) RecentCompletedTasks(ctx context.Context, tenantID, projectID string, n int) ([]domain.Task, error) {
	return f.tasks, f.err
}

type fakeAICompleter struct {
	content     string
	err         error
	gotConn     string
	gotPromptTx string
}

func (f *fakeAICompleter) Complete(ctx context.Context, connectionID, prompt string) (string, error) {
	f.gotConn = connectionID
	f.gotPromptTx = prompt
	return f.content, f.err
}

func (f *fakeAICompleter) gotPrompt() string { return f.gotPromptTx }

// newAIDecomposeForTest builds an AIDecompose wired with harmless default
// fakes for every new five-source-context port, letting each test override
// only what it cares about.
func newAIDecomposeForTest(tasks TaskRepository, edges EdgeRepository, aiProvider AIProviderContextResolver, resolver ProjectExecutionResolver, relay AICompleter) *AIDecompose {
	return NewAIDecompose(tasks, edges, aiProvider, resolver, &fakeProjectContextResolver{}, &fakeTechStackDetector{}, &fakeVelocityResolver{}, relay)
}

func TestAIDecompose_RequiresTenantContext(t *testing.T) {
	uc := newAIDecomposeForTest(newFakeTaskRepository(), &fakeEdgeRepository{}, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{}, &fakeAICompleter{})
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
	uc := newAIDecomposeForTest(tasks, &fakeEdgeRepository{}, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{connected: false}, &fakeAICompleter{})
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

func validDecomposeJSON() string {
	return `{"subtasks":[{"title":"Design API","description":"","type":"task","estimated_hours":2,"depends_on":[],"prompt_template":""},{"title":"Implement handler","description":"","type":"task","depends_on":[0],"prompt_template":""}],"notes":"n/a"}`
}

func TestAIDecompose_Connected_ReturnsParsedProposals(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1", Title: "Build widget"}
	completer := &fakeAICompleter{content: validDecomposeJSON()}
	uc := newAIDecomposeForTest(tasks, &fakeEdgeRepository{}, &fakeAIProviderContextResolver{ctx: "anthropic"}, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, completer)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Proposals) != 2 {
		t.Fatalf("expected 2 parsed proposals, got %d", len(got.Proposals))
	}
	if got.RawResponse != completer.content {
		t.Errorf("expected RawResponse to echo the AI's raw content")
	}
	if completer.gotConn != "conn-1" {
		t.Errorf("expected resolved connectionID to be passed through, got %q", completer.gotConn)
	}
}

func TestAIDecompose_TaskNotFound(t *testing.T) {
	uc := newAIDecomposeForTest(newFakeTaskRepository(), &fakeEdgeRepository{}, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{}, &fakeAICompleter{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "does-not-exist"}); err == nil {
		t.Fatal("expected an error for a nonexistent task")
	}
}

func TestAIDecompose_ProviderResolveFailurePropagates(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1"}
	uc := newAIDecomposeForTest(tasks, &fakeEdgeRepository{}, &fakeAIProviderContextResolver{err: errors.New("boom")}, &fakeProjectExecutionResolver{connected: true}, &fakeAICompleter{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "t1"}); err == nil {
		t.Fatal("expected an error when AI provider context resolution fails")
	}
}

func TestAIDecompose_RelayFailurePropagates(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1"}
	uc := newAIDecomposeForTest(tasks, &fakeEdgeRepository{}, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, &fakeAICompleter{err: errors.New("boom")})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "t1"}); err == nil {
		t.Fatal("expected an error when the AI relay call fails")
	}
}

// TestAIDecompose_MalformedJSON_ReturnsInvalidJSONCode locks in the
// structured-JSON parse failure path — a distinct, inspectable error code,
// not a generic internal error.
func TestAIDecompose_MalformedJSON_ReturnsInvalidJSONCode(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1"}
	uc := newAIDecomposeForTest(tasks, &fakeEdgeRepository{}, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, &fakeAICompleter{content: "not json at all"})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "t1"})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "TASK_AI_DECOMPOSE_INVALID_JSON" {
		t.Fatalf("expected TASK_AI_DECOMPOSE_INVALID_JSON, got %v", err)
	}
}

// TestAIDecompose_TechStackDetectorFailure_DoesNotFailExecute locks in
// SOL-TG-02's best-effort contract: a tech-stack detection error degrades
// prompt richness, never fails the whole call.
func TestAIDecompose_TechStackDetectorFailure_DoesNotFailExecute(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1", Title: "Build widget"}
	uc := NewAIDecompose(
		tasks, &fakeEdgeRepository{}, &fakeAIProviderContextResolver{},
		&fakeProjectExecutionResolver{connectionID: "conn-1", connected: true},
		&fakeProjectContextResolver{}, &fakeTechStackDetector{err: errors.New("detect failed")},
		&fakeVelocityResolver{}, &fakeAICompleter{content: validDecomposeJSON()},
	)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "t1"}); err != nil {
		t.Fatalf("expected TechStackDetector failure to be swallowed (best-effort), got %v", err)
	}
}

// TestAIDecompose_PromptIncludesEveryContextSource asserts the five-source
// context bundle actually lands in the built prompt string.
func TestAIDecompose_PromptIncludesEveryContextSource(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1", Title: "Build widget", Description: "a widget desc", AIContext: "extra ai context"}
	tasks.tasks["sibling"] = domain.Task{ID: "sibling", TenantID: "tenant-1", Title: "Existing sibling subtask"}
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{{FromTaskID: "t1", ToTaskID: "sibling", Kind: domain.EdgeKindParentChild}}}
	completer := &fakeAICompleter{content: validDecomposeJSON()}
	uc := NewAIDecompose(
		tasks, edges, &fakeAIProviderContextResolver{ctx: "anthropic-provider-ctx"},
		&fakeProjectExecutionResolver{connectionID: "conn-1", connected: true},
		&fakeProjectContextResolver{name: "MyProject", repoURL: "https://example.com/repo.git"},
		&fakeTechStackDetector{stack: domain.TechStack{Languages: []string{"Go"}, Frameworks: []string{"gRPC"}}},
		&fakeVelocityResolver{tasks: []domain.Task{{Title: "Recently finished task"}}},
		completer,
	)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "t1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prompt := completer.gotPrompt()
	for _, want := range []string{
		"Build widget", "a widget desc", "extra ai context", // task detail
		"MyProject", "https://example.com/repo.git", // project
		"Go", "gRPC", // tech stack
		"Existing sibling subtask", // existing subtasks
		"Recently finished task",   // velocity
		"anthropic-provider-ctx",   // provider context
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("expected prompt to contain %q, prompt was:\n%s", want, prompt)
		}
	}
}
