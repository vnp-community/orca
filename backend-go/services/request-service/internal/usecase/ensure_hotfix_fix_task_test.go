package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// fixTaskFake is task-service for the hotfix: the shared in-memory tasks plus the one create call it needs.
type fixTaskFake struct {
	*exTasks
	created   []PlanTreeInput
	createErr error
	statusErr error
}

func newFixTaskFake() *fixTaskFake { return &fixTaskFake{exTasks: newExTasks()} }

func (f *fixTaskFake) CreateFixTask(_ context.Context, in PlanTreeInput) (string, error) {
	if f.createErr != nil {
		return "", f.createErr
	}
	f.created = append(f.created, in)
	t := in.Proposal.Tasks[0]
	return f.add(domain.TaskView{
		Type: t.TaskType, Status: domain.TaskStatusOpen, RequestID: in.Request.ID, Title: t.Title, Description: t.Description,
	}).ID, nil
}

func (f *fixTaskFake) SetStatus(ctx context.Context, taskID, status string) error {
	if f.statusErr != nil {
		return f.statusErr
	}
	return f.exTasks.SetStatus(ctx, taskID, status)
}

func hotfixRequest() domain.Request {
	return domain.Request{ID: uuid.NewString(), TenantID: "t1", Type: domain.RequestTypeHotfix, Size: domain.RequestSizeS, Status: domain.RequestStatusAnalyzing, Title: "Checkout crashes"}
}

func newEnsure(f *fixTaskFake) *EnsureHotfixFixTask {
	return NewEnsureHotfixFixTask(f, f, fakePolicies{domain.NewPolicyRegistry(domain.PolicyDeps{})}, domain.DefaultPlanLimits)
}

func hotfixCtx() context.Context {
	return tenant.WithUserID(tenant.WithTenantID(context.Background(), "t1"), "u1")
}

func diagnosisBytes(t *testing.T) []byte { return []byte(analysisReply(t, "diagnosis_valid.json")) }

func TestEnsureHotfixFixTask_CreatesOneParentlessBugTask(t *testing.T) {
	f := newFixTaskFake()
	req := hotfixRequest()
	res, err := newEnsure(f).Ensure(hotfixCtx(), req, diagnosisBytes(t))
	if err != nil || !res.Created || res.TaskID == "" {
		t.Fatalf("Ensure = %+v, %v", res, err)
	}
	if len(f.created) != 1 {
		t.Fatalf("created %d tasks", len(f.created))
	}
	in := f.created[0]
	if in.Shape != domain.ShapeSingleTask || in.CreatorID != "u1" || in.Request.ID != req.ID || len(in.Proposal.Tasks) != 1 || len(in.Proposal.Phases) != 0 {
		t.Fatalf("create input = %+v", in)
	}
	if in.Proposal.Tasks[0].TaskType != "bug" || in.AIPlanJSON == "" {
		t.Fatalf("task = %+v plan json %q", in.Proposal.Tasks[0], in.AIPlanJSON)
	}
}

func TestEnsureHotfixFixTask_RepeatReusesTheTask(t *testing.T) {
	f := newFixTaskFake()
	uc, req := newEnsure(f), hotfixRequest()
	first, err := uc.Ensure(hotfixCtx(), req, diagnosisBytes(t))
	if err != nil {
		t.Fatal(err)
	}
	second, err := uc.Ensure(hotfixCtx(), req, diagnosisBytes(t))
	if err != nil || second.Created || second.TaskID != first.TaskID {
		t.Fatalf("second Ensure = %+v, %v (first %s)", second, err, first.TaskID)
	}
	if len(f.created) != 1 || len(f.tasks) != 1 {
		t.Fatalf("tasks created %d, held %d", len(f.created), len(f.tasks))
	}
}

func TestEnsureHotfixFixTask_ChangedDiagnosisRetiresTheStaleTask(t *testing.T) {
	f := newFixTaskFake()
	uc, req := newEnsure(f), hotfixRequest()
	old, err := uc.Ensure(hotfixCtx(), req, diagnosisBytes(t))
	if err != nil {
		t.Fatal(err)
	}
	changed := []byte(setRootCause(t, string(diagnosisBytes(t)), "A different root cause"))
	got, err := uc.Ensure(hotfixCtx(), req, changed)
	if err != nil || !got.Created || got.TaskID == old.TaskID {
		t.Fatalf("Ensure = %+v, %v", got, err)
	}
	if f.get(old.TaskID).Status != domain.TaskStatusCancelled || f.get(got.TaskID).Status != domain.TaskStatusOpen {
		t.Fatalf("old %s, new %s", f.get(old.TaskID).Status, f.get(got.TaskID).Status)
	}
}

func TestEnsureHotfixFixTask_NeverTouchesWorkAlreadyStarted(t *testing.T) {
	for _, status := range []string{domain.TaskStatusInProgress, domain.TaskStatusReview, domain.TaskStatusDone} {
		t.Run(status, func(t *testing.T) {
			f := newFixTaskFake()
			req := hotfixRequest()
			running := f.add(domain.TaskView{Type: "bug", Status: status, RequestID: req.ID, Title: "whatever"})
			res, err := newEnsure(f).Ensure(hotfixCtx(), req, diagnosisBytes(t))
			if err != nil || res.Created || res.TaskID != running.ID || len(f.created) != 0 {
				t.Fatalf("Ensure = %+v, %v, created %d", res, err, len(f.created))
			}
			if f.get(running.ID).Status != status {
				t.Fatalf("status rewritten to %s", f.get(running.ID).Status)
			}
		})
	}
}

func TestEnsureHotfixFixTask_IgnoresCancelledAndNestedTasks(t *testing.T) {
	f := newFixTaskFake()
	req := hotfixRequest()
	f.add(domain.TaskView{Type: "bug", Status: domain.TaskStatusCancelled, RequestID: req.ID, Title: "old attempt"})
	f.add(domain.TaskView{Type: "task", Status: domain.TaskStatusOpen, RequestID: req.ID, ParentID: "some-plan", Title: "nested"})
	other := f.add(domain.TaskView{Type: "bug", Status: domain.TaskStatusOpen, RequestID: uuid.NewString(), Title: "another request"})
	res, err := newEnsure(f).Ensure(hotfixCtx(), req, diagnosisBytes(t))
	if err != nil || !res.Created {
		t.Fatalf("Ensure = %+v, %v", res, err)
	}
	if f.get(other.ID).Status != domain.TaskStatusOpen {
		t.Fatal("a task of another Request was cancelled")
	}
}

func TestEnsureHotfixFixTask_Refusals(t *testing.T) {
	cases := []struct {
		name string
		run  func(f *fixTaskFake) (HotfixFixTaskResult, error)
		code string
	}{
		{"not a single_task type", func(f *fixTaskFake) (HotfixFixTaskResult, error) {
			r := hotfixRequest()
			r.Type = domain.RequestTypeBug
			return newEnsure(f).Ensure(hotfixCtx(), r, diagnosisBytes(t))
		}, "REQUEST_PLAN_NOT_APPLICABLE"},
		{"no user", func(f *fixTaskFake) (HotfixFixTaskResult, error) {
			return newEnsure(f).Ensure(tenant.WithTenantID(context.Background(), "t1"), hotfixRequest(), diagnosisBytes(t))
		}, "REQUEST_REPORTER_REQUIRED"},
		{"no tenant", func(f *fixTaskFake) (HotfixFixTaskResult, error) {
			return newEnsure(f).Ensure(context.Background(), hotfixRequest(), diagnosisBytes(t))
		}, "REQUEST_TENANT_REQUIRED"},
		{"task-service cannot list", func(f *fixTaskFake) (HotfixFixTaskResult, error) {
			f.listErr = errors.New("down")
			return newEnsure(f).Ensure(hotfixCtx(), hotfixRequest(), diagnosisBytes(t))
		}, "REQUEST_HOTFIX_FIX_TASK_UNAVAILABLE"},
		{"task-service cannot retire the stale task", func(f *fixTaskFake) (HotfixFixTaskResult, error) {
			r := hotfixRequest()
			f.add(domain.TaskView{Type: "bug", Status: domain.TaskStatusOpen, RequestID: r.ID, Title: "stale"})
			f.statusErr = errors.New("down")
			return newEnsure(f).Ensure(hotfixCtx(), r, diagnosisBytes(t))
		}, "REQUEST_HOTFIX_FIX_TASK_UNAVAILABLE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixTaskFake()
			_, err := tc.run(f)
			requireCode(t, err, tc.code)
			if len(f.created) != 0 {
				t.Fatalf("a task was created despite the refusal")
			}
		})
	}
}

func TestEnsureHotfixFixTask_CreateErrorPropagates(t *testing.T) {
	f := newFixTaskFake()
	f.createErr = domain.ErrHotfixFixTaskUnavailable(errors.New("down"))
	_, err := newEnsure(f).Ensure(hotfixCtx(), hotfixRequest(), diagnosisBytes(t))
	requireCode(t, err, "REQUEST_HOTFIX_FIX_TASK_UNAVAILABLE")
}

func TestEnsureHotfixFixTask_InvalidDiagnosisIsRefused(t *testing.T) {
	f := newFixTaskFake()
	_, err := newEnsure(f).Ensure(hotfixCtx(), hotfixRequest(), []byte(analysisReply(t, "diagnosis_invalid_no_statement.json")))
	if !errors.Is(err, domain.ErrAnalysisDocumentInvalid) || len(f.created) != 0 {
		t.Fatalf("err = %v, created %d", err, len(f.created))
	}
}

func TestEnsureHotfixFixTask_AppliesToSingleTaskTypesOnly(t *testing.T) {
	uc := newEnsure(newFixTaskFake())
	for _, typ := range domain.AllFlowTypes() {
		want := typ == domain.RequestTypeHotfix
		if got := uc.AppliesTo(domain.Request{Type: typ, Size: domain.RequestSizeM}); got != want {
			t.Errorf("AppliesTo(%s) = %v", typ, got)
		}
	}
	if uc.AppliesTo(domain.Request{Type: "nonsense"}) {
		t.Error("an unknown type must not apply")
	}
}

// setRootCause changes the diagnosis a hotfix is fixed from, to prove a re-run does not keep an outdated task.
func setRootCause(t *testing.T, doc, statement string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(doc), &m); err != nil {
		t.Fatal(err)
	}
	m["root_cause"].(map[string]any)["statement"] = statement
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
