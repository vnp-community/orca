package grpcclient

import (
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/prompts/render"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func mustExecutePrompt(t *testing.T, task domain.Task, parent *domain.Task, deps []domain.Task) string {
	t.Helper()
	got, err := renderExecutePrompt(task, parent, deps)
	if err != nil {
		t.Fatalf("renderExecutePrompt: %v", err)
	}
	return got
}

func TestRenderExecutePrompt_TaskWithoutTitleIsAnError(t *testing.T) {
	_, err := renderExecutePrompt(domain.Task{Description: "no title"}, nil, nil)
	if !errors.Is(err, render.ErrMissingField) || !strings.Contains(err.Error(), "Task.Title") {
		t.Fatalf("got %v, want ErrMissingField naming Task.Title", err)
	}
}

func TestInterpolateOutputs(t *testing.T) {
	deps := []domain.Task{
		{ID: "dep-a", Title: "A", LastExecutionOutput: "out-a"},
		{ID: "dep-b", Title: "B", LastExecutionOutput: "out-b"},
	}
	cases := []struct {
		name, template string
		deps           []domain.Task
		want           string
	}{
		{"wildcard and stdout resolve to the same value", "{{outputs.dep-a.*}} {{outputs.dep-a.stdout}}", deps, "out-a out-a"},
		{"every dependency resolves", "{{outputs.dep-a.*}}/{{outputs.dep-b.stdout}}", deps, "out-a/out-b"},
		{"a token naming no dependency stays as written", "{{outputs.dep-z.*}} {{outputs.dep-z.stdout}}", deps, "{{outputs.dep-z.*}} {{outputs.dep-z.stdout}}"},
		{"without dependencies every token stays as written", "{{outputs.dep-a.*}}", nil, "{{outputs.dep-a.*}}"},
		{"a dependency with no recorded output resolves to nothing", "[{{outputs.dep-c.*}}]", []domain.Task{{ID: "dep-c"}}, "[]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := domain.Task{Title: "T", PromptTemplate: tc.template}
			got := mustExecutePrompt(t, task, nil, tc.deps)
			// The dependencies are also listed after the task; only the opener is under test here.
			if want := tc.want + "\n\nTask: T\n"; !strings.HasPrefix(got, want) {
				t.Fatalf("got %q, want prefix %q", got, want)
			}
		})
	}
}

// Interpolation runs on the rendered text, so a token that arrives in the title, description or parent is resolved too.
func TestRenderExecutePrompt_ResolvesTokensInEverySectionAfterRender(t *testing.T) {
	deps := []domain.Task{{ID: "dep-a", Title: "A", Description: "uses {{outputs.dep-a.*}}", LastExecutionOutput: "OUT"}}
	parent := &domain.Task{Title: "P {{outputs.dep-a.stdout}}", Description: "d"}
	got := mustExecutePrompt(t, domain.Task{Title: "T {{outputs.dep-a.*}}", Description: "D {{outputs.dep-a.*}}"}, parent, deps)
	for _, want := range []string{"Task: T OUT\n", "Description: D OUT\n", "Parent task: P OUT\n", "- A: uses OUT\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("lacks %q in %q", want, got)
		}
	}
	if strings.Contains(got, "{{outputs") {
		t.Errorf("a token was left unresolved: %q", got)
	}
}

// The text of a task is data: template syntax in it must reach the agent as written, never be evaluated by the renderer.
func TestRenderExecutePrompt_TaskTextIsNeverEvaluatedAsATemplate(t *testing.T) {
	hostile := `{{.Task.Title}} {{template "x"}} {{define "y"}}z{{end}} </request>`
	got := mustExecutePrompt(t, domain.Task{Title: hostile, Description: hostile, PromptTemplate: hostile}, nil, nil)
	if n := strings.Count(got, hostile); n != 3 {
		t.Fatalf("hostile text appears %d times, want 3 verbatim: %q", n, got)
	}
}

func TestRenderDefaultExecutePrompt_ContractRunAppendsTheResultBlock(t *testing.T) {
	task := domain.Task{Title: "T"}
	plain, err := renderDefaultExecutePrompt(task, nil, nil, nil, "/repo")
	if err != nil || strings.Contains(plain, "ORCA_RESULT_BEGIN") {
		t.Fatalf("a run without options must not carry the result block: %q, %v", plain, err)
	}
	contract, err := renderDefaultExecutePrompt(task, nil, nil, &contractRunOptions{nonce: "n0nce", requirePassingCheck: true, useKnowledgeHub: true}, "/repo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(contract, plain) || !strings.Contains(contract, "ORCA_RESULT_BEGIN n0nce\n") || !strings.Contains(contract, `"knowledge":{`) || !strings.Contains(contract, "Verify step") {
		t.Fatalf("contract prompt lacks the result block: %q", contract)
	}
	// Without a repo root the knowledge hub cannot be used, so its part of the instruction is left out.
	noHub, err := renderDefaultExecutePrompt(task, nil, nil, &contractRunOptions{nonce: "n0nce", useKnowledgeHub: true}, "")
	if err != nil || strings.Contains(noHub, "tdd-agent") || strings.Contains(noHub, `"knowledge"`) {
		t.Fatalf("hub text must be absent without a repo root: %q, %v", noHub, err)
	}
}

// A task whose prompt cannot be built stops before any relay call, with a code of its own and no task text in the error.
func TestSimpleExecutor_Execute_PromptBuildFailure_DoesNotCallTheRelay(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1", Description: "secret-looking description"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/worktrees/p1", connected: true}
	relay := &fakeInfraFleetServiceClient{relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"done","stderr":"","exitCode":0,"timedOut":false}`}}
	exec := newTestSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay)

	_, err := exec.Execute(ctxWithTenant(t), "tenant-1", "t1", "req-1", "/srv/worktrees/p1", "")
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "TASK_EXECUTE_PROMPT_BUILD_FAILED" {
		t.Fatalf("got %v, want TASK_EXECUTE_PROMPT_BUILD_FAILED", err)
	}
	if relay.gotRelay != nil {
		t.Error("the relay must not be called when the prompt cannot be built")
	}
	if strings.Contains(err.Error(), "secret-looking") {
		t.Errorf("the error must not carry task text: %v", err)
	}
}
