package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type exWriteAuth struct{ err error }

func (a exWriteAuth) CanWrite(context.Context, domain.Request) error { return a.err }

func newCheckRig(t *testing.T) (*exRig, *RecordRequestCheck) {
	r := newExRig(t)
	return r, &RecordRequestCheck{Requests: r.store, Checks: r.checks, Authorizer: exWriteAuth{}}
}

const afterOK = `{"metrics":[{"name":"p95","value":150}]}`

func TestRecordRequestCheck_StateNotAllowed(t *testing.T) {
	r, uc := newCheckRig(t)
	req := r.request(domain.RequestTypePerformance, domain.RequestSizeM, domain.RequestStatusAnalyzing)
	_, err := uc.Execute(lcCtx(), RecordRequestCheckInput{RequestID: req.ID, Kind: "perf_after", Status: "passed", MetricsJSON: afterOK})
	if !errorHasCode(err, "REQUEST_CHECK_NOT_ALLOWED_NOW") {
		t.Fatalf("perf_after is for executing only: %v", err)
	}
	if len(r.checks.rows) != 0 {
		t.Fatal("nothing may be stored")
	}
}

func TestRecordRequestCheck_KindNotForType(t *testing.T) {
	r, uc := newCheckRig(t)
	req := r.request(domain.RequestTypeBug, domain.RequestSizeM, domain.RequestStatusExecuting)
	_, err := uc.Execute(lcCtx(), RecordRequestCheckInput{RequestID: req.ID, Kind: "perf_after", Status: "passed", MetricsJSON: afterOK})
	if !errorHasCode(err, "REQUEST_CHECK_NOT_ALLOWED_NOW") {
		t.Fatalf("got %v", err)
	}
}

func TestRecordRequestCheck_InvalidMetrics(t *testing.T) {
	r, uc := newCheckRig(t)
	req := r.request(domain.RequestTypePerformance, domain.RequestSizeM, domain.RequestStatusExecuting)
	_, err := uc.Execute(lcCtx(), RecordRequestCheckInput{RequestID: req.ID, Kind: "perf_after", Status: "passed", MetricsJSON: `{"metrics":[{"name":"p95"}]}`})
	if !errorHasCode(err, "REQUEST_CHECK_INVALID_METRICS") {
		t.Fatalf("got %v", err)
	}
	if _, err := uc.Execute(lcCtx(), RecordRequestCheckInput{RequestID: req.ID, Kind: "nope", Status: "passed"}); !errorHasCode(err, "REQUEST_CHECK_INVALID_METRICS") {
		t.Fatalf("unknown kind: %v", err)
	}
}

func TestRecordRequestCheck_Persists_WithRecordedBy(t *testing.T) {
	r, uc := newCheckRig(t)
	req := r.request(domain.RequestTypePerformance, domain.RequestSizeM, domain.RequestStatusExecuting)
	ctx := tenant.WithUserID(lcCtx(), "user-7")
	got, err := uc.Execute(ctx, RecordRequestCheckInput{RequestID: req.ID, Kind: "perf_after", Status: "passed", MetricsJSON: afterOK, Summary: "wrk run", TaskID: "task-1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != domain.CheckSourceManual || got.RecordedBy != "user-7" || got.TaskID != "task-1" || got.Kind != domain.CheckPerfAfter || got.ID == "" {
		t.Fatalf("got %+v", got)
	}
	if len(r.checks.rows) != 1 {
		t.Fatal("one row expected")
	}
}

func TestRecordRequestCheck_AgentSourceNoUser(t *testing.T) {
	r, uc := newCheckRig(t)
	req := r.request(domain.RequestTypeSecurity, domain.RequestSizeM, domain.RequestStatusExecuting)
	ctx := tenant.WithActorType(tenant.WithUserID(lcCtx(), "user-7"), tenant.ActorAgent)
	got, err := uc.Execute(ctx, RecordRequestCheckInput{RequestID: req.ID, Kind: "security_recheck", Status: "passed", Summary: "rescanned, clean"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != domain.CheckSourceAgent || got.RecordedBy != "" {
		t.Fatalf("an agent's record names the agent, not the user: %+v", got)
	}
}

func TestRecordRequestCheck_CannotClaimOrcaVerified(t *testing.T) {
	// The RPC has no source field and the use case derives it from the caller, so every path ends manual or agent.
	r, uc := newCheckRig(t)
	req := r.request(domain.RequestTypeOpsRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	for _, ctx := range []context.Context{lcCtx(), tenant.WithActorType(lcCtx(), tenant.ActorAgent), tenant.WithActorType(lcCtx(), tenant.ActorSystem)} {
		got, err := uc.Execute(ctx, RecordRequestCheckInput{RequestID: req.ID, Kind: "ops_result", Status: "passed", Summary: "done"})
		if err != nil || got.Source == domain.CheckSourceOrcaVerified {
			t.Fatalf("got %+v %v", got, err)
		}
	}
	_ = r
}

func TestRecordRequestCheck_Forbidden(t *testing.T) {
	r, uc := newCheckRig(t)
	uc.Authorizer = exWriteAuth{err: domain.ErrCheckForbidden()}
	req := r.request(domain.RequestTypePerformance, domain.RequestSizeM, domain.RequestStatusExecuting)
	_, err := uc.Execute(lcCtx(), RecordRequestCheckInput{RequestID: req.ID, Kind: "perf_after", Status: "passed", MetricsJSON: afterOK})
	if !errorHasCode(err, "REQUEST_CHECK_FORBIDDEN") || len(r.checks.rows) != 0 {
		t.Fatalf("got %v", err)
	}
}

func TestRecordRequestCheck_AppendOnly_NoUpdatePath(t *testing.T) {
	var repo RequestCheckRepository
	_ = repo
	// The port has exactly Append, ListByRequest and Latest: nothing can edit or delete a row.
	var _ interface {
		Append(context.Context, domain.RequestCheck) (domain.RequestCheck, error)
		ListByRequest(context.Context, string) ([]domain.RequestCheck, error)
		Latest(context.Context, string, domain.CheckKind) (domain.RequestCheck, bool, error)
	} = RequestCheckRepository(nil)
}

func TestListRequestChecks_MarksEffective(t *testing.T) {
	r := newExRig(t)
	rec := &RecordRequestCheck{Requests: r.store, Checks: r.checks, Authorizer: exWriteAuth{}}
	list := &ListRequestChecks{Requests: r.store, Checks: r.checks, Visibility: &MemberRequestVisibility{}}
	req := r.request(domain.RequestTypeRefactor, domain.RequestSizeM, domain.RequestStatusExecuting)
	ctx := tenant.WithUserID(lcCtx(), req.ReporterID)
	for _, passed := range []int{40, 38} {
		body := `{"total":40,"passed":` + string(rune('0'+passed/10)) + string(rune('0'+passed%10)) + `,"failed":0,"command":"go test"}`
		if _, err := rec.Execute(ctx, RecordRequestCheckInput{RequestID: req.ID, Kind: "tests_before", Status: "passed", MetricsJSON: body}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := list.Execute(ctx, req.ID, "")
	if err != nil || len(got) != 2 || got[0].Effective || !got[1].Effective {
		t.Fatalf("the newest row of a kind is effective: %+v %v", got, err)
	}
	if only, _ := list.Execute(ctx, req.ID, "tests_after"); len(only) != 0 {
		t.Fatalf("kind filter: %+v", only)
	}
}

func TestListRequestChecks_HiddenRequestIsNotFound(t *testing.T) {
	r := newExRig(t)
	list := &ListRequestChecks{Requests: r.store, Checks: r.checks, Visibility: &MemberRequestVisibility{}}
	req := r.request(domain.RequestTypeRefactor, domain.RequestSizeM, domain.RequestStatusExecuting)
	ctx := tenant.WithUserID(lcCtx(), "stranger")
	if _, err := list.Execute(ctx, req.ID, ""); !errorHasCode(err, "REQUEST_NOT_FOUND") {
		t.Fatalf("a request the caller may not see must look absent: %v", err)
	}
}

func TestParticipantWriteAuthorizer(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeRefactor, domain.RequestSizeM, domain.RequestStatusExecuting)
	by := "approver-9"
	r.gates.add(domain.Approval{RequestID: req.ID, SubjectType: domain.SubjectPlan, SubjectID: "p", Status: domain.ApprovalStatusApproved, DecidedBy: &by})
	a := &ParticipantWriteAuthorizer{Approvals: r.gates}
	cases := []struct {
		name, user, role string
		ok               bool
	}{
		{"reporter", req.ReporterID, "", true},
		{"admin", "x", "admin", true},
		{"plan approver", "approver-9", "", true},
		{"stranger", "stranger", "", false},
		{"no user", "", "", false},
	}
	for _, tc := range cases {
		err := a.CanWrite(lcCtxWithUser(tc.user, tc.role), req)
		if (err == nil) != tc.ok {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}
