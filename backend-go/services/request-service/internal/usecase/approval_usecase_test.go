package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func errCode(err error) string {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	if err == nil {
		return ""
	}
	return "NOT_APP_ERROR:" + err.Error()
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if got := errCode(err); got != code {
		t.Fatalf("error code = %q (%v), want %q", got, err, code)
	}
}

func approverCtx(role string) context.Context { return userCtx(uuid.NewString(), role) }

func TestApproval_Open_SnapshotDueDigestAndEvent(t *testing.T) {
	e := newApprEnv()
	r := e.planRequest()
	a, err := e.openPlan(r, r.ReporterID)
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != domain.ApprovalStatusPending || a.Stage != "awaiting_plan_approval" || a.RequestedBy != r.ReporterID || a.Version != 1 {
		t.Fatalf("approval = %+v", a)
	}
	if a.DueAt == nil || !a.DueAt.Equal(e.s.now.Add(7*24*time.Hour)) {
		t.Fatalf("due_at must be db-now + 7d for plan, got %v", a.DueAt)
	}
	if a.SubjectDigest != domain.SubjectDigest(domain.SubjectPlan, a.SubjectID, "v1") {
		t.Fatalf("digest = %s", a.SubjectDigest)
	}
	snap := e.s.approvers[a.ID]
	if len(snap) != 2 || snap[0].Kind != domain.PrincipalKindReporter || snap[1].String() != "role:admin" {
		t.Fatalf("approver snapshot = %v", snap)
	}
	if len(e.s.events) != 1 || e.s.events[0].Subject != "orca.request.approval.requested" {
		t.Fatalf("events = %v", e.eventSubjects())
	}
	var p map[string]any
	_ = json.Unmarshal(e.s.events[0].Payload, &p)
	if _, has := p["comment"]; has || p["reporter_id"] != r.ReporterID || p["request_number"] == nil {
		t.Fatalf("payload = %v", p)
	}
}

func TestApproval_Open_IdempotentSameDigestAndPendingExists(t *testing.T) {
	e := newApprEnv()
	r := e.planRequest()
	first, err := e.openPlan(r, r.ReporterID)
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.openPlan(r, r.ReporterID)
	if err != nil || again.ID != first.ID || len(e.s.events) != 1 {
		t.Fatalf("same digest must return the pending row without a new event: %v %v events=%d", again, err, len(e.s.events))
	}
	e.art.digest = "v2"
	_, err = e.openPlan(r, r.ReporterID)
	wantCode(t, err, "REQUEST_APPROVAL_PENDING_EXISTS")

	key := "k1"
	e2 := newApprEnv()
	r2 := e2.planRequest()
	in := OpenApprovalInput{RequestID: r2.ID, SubjectType: domain.SubjectPlan, IdempotencyKey: &key}
	a1, err := e2.open.Execute(userCtx(r2.ReporterID, "user"), in)
	if err != nil {
		t.Fatal(err)
	}
	a2, err := e2.open.Execute(userCtx(r2.ReporterID, "user"), in)
	if err != nil || a2.ID != a1.ID {
		t.Fatalf("idempotency key must return the first approval: %v %v", a2, err)
	}
	in.SubjectType = domain.SubjectTaskList
	_, err = e2.open.Execute(userCtx(r2.ReporterID, "user"), in)
	wantCode(t, err, "REQUEST_APPROVAL_IDEMPOTENCY_CONFLICT")
}

func TestApproval_Open_RejectsSubjectOutsideFlowOrStage(t *testing.T) {
	e := newApprEnv()
	r := e.planRequest()
	// task_list is not a gate of change_request.
	_, err := e.open.Execute(userCtx(r.ReporterID, "user"), OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectTaskList})
	wantCode(t, err, "REQUEST_APPROVAL_SUBJECT_TYPE_NOT_ALLOWED")
	// solution is a gate of change_request but the Request is not waiting for analysis approval.
	_, err = e.open.Execute(userCtx(r.ReporterID, "user"), OpenApprovalInput{RequestID: r.ID, SubjectType: domain.SubjectSolution})
	wantCode(t, err, "REQUEST_APPROVAL_STAGE_MISMATCH")
	_, err = e.open.Execute(userCtx(r.ReporterID, "user"), OpenApprovalInput{RequestID: r.ID, SubjectType: "bogus"})
	wantCode(t, err, "REQUEST_APPROVAL_SUBJECT_TYPE_INVALID")
	if len(e.s.approvals) != 0 || len(e.s.events) != 0 {
		t.Fatal("refused opens must not write")
	}
}

func TestApproval_Open_UnboundSubjectFailsClosed(t *testing.T) {
	e := newApprEnv()
	e.registry.Register(domain.SubjectPlan, &TransitionSubjectHandler{}) // no Artifacts bound
	r := e.planRequest()
	_, err := e.openPlan(r, r.ReporterID)
	wantCode(t, err, "REQUEST_APPROVAL_SUBJECT_UNAVAILABLE")
}

func TestApproval_Open_NoEligibleApproverWhenOnlyRequesterCouldDecide(t *testing.T) {
	e := newApprEnv()
	r := e.planRequest()
	e.s.policies = []domain.ApprovalPolicy{{
		SubjectType: domain.SubjectPlan, Enabled: true, AllowRequesterApprove: false,
		Approvers: []domain.Principal{{Kind: domain.PrincipalKindReporter}, {Kind: domain.PrincipalKindUser, ID: r.ReporterID}},
	}}
	_, err := e.openPlan(r, r.ReporterID)
	wantCode(t, err, "REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER")
	if len(e.s.approvals) != 0 {
		t.Fatal("NO_ELIGIBLE_APPROVER must not create an approvals row")
	}
	// a team whose only member is the reporter is not eligible either
	e.s.policies[0].Approvers = []domain.Principal{{Kind: domain.PrincipalKindTeam, ID: "team-1"}}
	e.teams.members["team-1"] = []string{r.ReporterID}
	_, err = e.openPlan(r, r.ReporterID)
	wantCode(t, err, "REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER")
	e.teams.members["team-1"] = []string{r.ReporterID, uuid.NewString()}
	if _, err = e.openPlan(r, r.ReporterID); err != nil {
		t.Fatalf("a second team member makes it eligible: %v", err)
	}
}

func TestApproval_Open_DirectoryOutageDoesNotBlock(t *testing.T) {
	e := newApprEnv()
	r := e.planRequest()
	e.s.policies = []domain.ApprovalPolicy{{
		SubjectType: domain.SubjectPlan, Enabled: true, AllowRequesterApprove: false,
		Approvers: []domain.Principal{{Kind: domain.PrincipalKindTeam, ID: "team-1"}},
	}}
	e.teams.err = domain.ErrApprovalDirectoryUnavailable
	if _, err := e.openPlan(r, r.ReporterID); err != nil {
		t.Fatalf("a directory outage must not fail OpenApproval: %v", err)
	}
}

func TestApproval_Resolver_UsesRealSizeAndUrgency(t *testing.T) {
	e := newApprEnv()
	large, urgent := "L", "urgent"
	e.s.policies = []domain.ApprovalPolicy{
		{ID: "generic", SubjectType: domain.SubjectPlan, Enabled: true, Approvers: []domain.Principal{{Kind: domain.PrincipalKindRole, ID: "admin"}}, CreatedAt: e.s.now},
		{ID: "large-urgent", SubjectType: domain.SubjectPlan, Enabled: true, Size: &large, Urgency: &urgent,
			Approvers: []domain.Principal{{Kind: domain.PrincipalKindUser, ID: "cto"}}, CreatedAt: e.s.now},
	}
	res := &ResolveApproverPolicy{Repo: apprPolicies{e.s}}
	small := domain.Request{Type: domain.RequestTypeChangeRequest, Size: domain.RequestSizeS, Urgency: domain.UrgencyNormal}
	big := domain.Request{Type: domain.RequestTypeChangeRequest, Size: domain.RequestSizeL, Urgency: domain.UrgencyUrgent}
	p1, _ := res.Resolve(lcCtx(), small, domain.SubjectPlan)
	p2, _ := res.Resolve(lcCtx(), big, domain.SubjectPlan)
	if p1.ID != "generic" || p2.ID != "large-urgent" {
		t.Fatalf("size/urgency must come from the Request: got %q and %q", p1.ID, p2.ID)
	}
	if def, _ := res.Resolve(lcCtx(), small, domain.SubjectAnswer); def.ID != "" || len(def.Approvers) == 0 {
		t.Fatalf("no candidate must fall back to the built-in default: %+v", def)
	}
}

func approvedSetup(t *testing.T) (*apprEnv, domain.Request, *domain.Approval) {
	t.Helper()
	e := newApprEnv()
	r := e.planRequest()
	a, err := e.openPlan(r, r.ReporterID)
	if err != nil {
		t.Fatal(err)
	}
	e.s.calls, e.s.events = nil, nil
	return e, r, a
}

func TestApproval_Decide_ApproveMovesRequestInOneTransaction(t *testing.T) {
	e, r, a := approvedSetup(t)
	ctx := userCtx(r.ReporterID, "user") // reporter is a snapshot approver and self-approval is allowed for size M
	res, err := e.decide.Execute(ctx, DecideApprovalInput{ID: a.ID, Decision: "approve", Comment: "ok", ExpectedDigest: a.SubjectDigest})
	if err != nil {
		t.Fatal(err)
	}
	if res.Approval.Status != domain.ApprovalStatusApproved || res.RequestStatus != domain.RequestStatusExecuting {
		t.Fatalf("result = %+v", res)
	}
	if got := e.s.requests[r.ID].Status; got != domain.RequestStatusExecuting {
		t.Fatalf("request status = %s", got)
	}
	if len(e.art.decided) != 1 || !e.art.decided[0] {
		t.Fatalf("artifact side effect missing: %v", e.art.decided)
	}
	if e.calls0() != "lock:request@t1" || e.s.calls[1] != "lock:approval" {
		t.Fatalf("lock order must be Request then Approval, got %v", e.s.calls)
	}
	subjects := e.eventSubjects()
	if !strings.Contains(subjects, "orca.request.request.status_changed") || !strings.Contains(subjects, "orca.request.approval.decided") {
		t.Fatalf("events = %s", subjects)
	}
	for _, ev := range e.s.events {
		if strings.Contains(string(ev.Payload), `"comment"`) {
			t.Fatalf("payload must not carry the comment: %s", ev.Payload)
		}
	}
}

func (e *apprEnv) calls0() string {
	if len(e.s.calls) == 0 {
		return ""
	}
	return e.s.calls[0]
}

func TestApproval_Decide_RejectReturnsRequestToBacklog(t *testing.T) {
	e, r, a := approvedSetup(t)
	ctx := userCtx(r.ReporterID, "user")
	_, err := e.decide.Execute(ctx, DecideApprovalInput{ID: a.ID, Decision: "reject", Comment: "  ", ExpectedDigest: a.SubjectDigest})
	wantCode(t, err, "REQUEST_APPROVAL_COMMENT_REQUIRED")
	res, err := e.decide.Execute(ctx, DecideApprovalInput{ID: a.ID, Decision: "reject", Comment: "scope too wide", ExpectedDigest: a.SubjectDigest})
	if err != nil {
		t.Fatal(err)
	}
	got := e.s.requests[r.ID]
	if res.Approval.Status != domain.ApprovalStatusRejected || got.Status != domain.RequestStatusRequestBacklog ||
		got.ReturnedFromStage != domain.ReturnStagePlan || got.ReturnedCategory != domain.ReturnCategoryRejected || got.ReturnReason != "scope too wide" {
		t.Fatalf("approval=%+v request=%+v", res.Approval, got)
	}
	if len(e.art.decided) != 1 || e.art.decided[0] {
		t.Fatalf("artifact must see the rejection: %v", e.art.decided)
	}
}

func TestApproval_Decide_ErrorTable(t *testing.T) {
	tests := []struct {
		name string
		mod  func(e *apprEnv, r domain.Request, a *domain.Approval) (context.Context, DecideApprovalInput)
		code string
	}{
		{"digest mismatch", func(e *apprEnv, r domain.Request, a *domain.Approval) (context.Context, DecideApprovalInput) {
			return userCtx(r.ReporterID, "user"), DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: "stale"}
		}, "REQUEST_APPROVAL_DIGEST_MISMATCH"},
		{"approve without digest", func(e *apprEnv, r domain.Request, a *domain.Approval) (context.Context, DecideApprovalInput) {
			return userCtx(r.ReporterID, "user"), DecideApprovalInput{ID: a.ID, Decision: "approve"}
		}, "REQUEST_APPROVAL_DIGEST_MISMATCH"},
		{"stage moved on", func(e *apprEnv, r domain.Request, a *domain.Approval) (context.Context, DecideApprovalInput) {
			req := e.s.requests[r.ID]
			req.Status = domain.RequestStatusExecuting
			e.s.requests[r.ID] = req
			return userCtx(r.ReporterID, "user"), DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest}
		}, "REQUEST_APPROVAL_STAGE_MISMATCH"},
		{"version conflict", func(e *apprEnv, r domain.Request, a *domain.Approval) (context.Context, DecideApprovalInput) {
			return userCtx(r.ReporterID, "user"), DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest, ExpectedVersion: 99}
		}, "REQUEST_APPROVAL_VERSION_CONFLICT"},
		{"not an approver", func(e *apprEnv, r domain.Request, a *domain.Approval) (context.Context, DecideApprovalInput) {
			return approverCtx("user"), DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest}
		}, "REQUEST_APPROVAL_NOT_APPROVER"},
		{"agent forbidden", func(e *apprEnv, r domain.Request, a *domain.Approval) (context.Context, DecideApprovalInput) {
			return tenant.WithActorType(userCtx(r.ReporterID, "admin"), tenant.ActorAgent), DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest}
		}, "REQUEST_APPROVAL_AGENT_FORBIDDEN"},
		{"no user", func(e *apprEnv, r domain.Request, a *domain.Approval) (context.Context, DecideApprovalInput) {
			return lcCtx(), DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest}
		}, "REQUEST_APPROVAL_NO_USER"},
		{"unknown id", func(e *apprEnv, r domain.Request, a *domain.Approval) (context.Context, DecideApprovalInput) {
			return userCtx(r.ReporterID, "user"), DecideApprovalInput{ID: uuid.NewString(), Decision: "approve", ExpectedDigest: "x"}
		}, "REQUEST_APPROVAL_NOT_FOUND"},
		{"bad decision", func(e *apprEnv, r domain.Request, a *domain.Approval) (context.Context, DecideApprovalInput) {
			return userCtx(r.ReporterID, "user"), DecideApprovalInput{ID: a.ID, Decision: "maybe"}
		}, "REQUEST_APPROVAL_DECISION_INVALID"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e, r, a := approvedSetup(t)
			ctx, in := tc.mod(e, r, a)
			_, err := e.decide.Execute(ctx, in)
			wantCode(t, err, tc.code)
			if e.s.approvals[a.ID].Status != domain.ApprovalStatusPending {
				t.Fatal("a refused decision must leave the approval pending")
			}
		})
	}
}

func TestApproval_Decide_SelfApprovalForbidden(t *testing.T) {
	e := newApprEnv()
	r := e.planRequest()
	e.s.policies = []domain.ApprovalPolicy{{
		SubjectType: domain.SubjectPlan, Enabled: true, AllowRequesterApprove: false,
		Approvers: []domain.Principal{{Kind: domain.PrincipalKindReporter}, {Kind: domain.PrincipalKindRole, ID: "admin"}},
	}}
	e.admins.admins = []string{r.ReporterID, uuid.NewString()}
	a, err := e.openPlan(r, r.ReporterID)
	if err != nil {
		t.Fatal(err)
	}
	// even an admin who is the reporter is blocked (separation of duties)
	_, err = e.decide.Execute(userCtx(r.ReporterID, "admin"), DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest})
	wantCode(t, err, "REQUEST_APPROVAL_SELF_APPROVAL_FORBIDDEN")
	if _, err = e.decide.Execute(approverCtx("admin"), DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest}); err != nil {
		t.Fatalf("another admin may approve: %v", err)
	}
}

func TestApproval_Decide_TeamApproverUsesDirectory(t *testing.T) {
	e := newApprEnv()
	r := e.planRequest()
	e.s.policies = []domain.ApprovalPolicy{{SubjectType: domain.SubjectPlan, Enabled: true, Approvers: []domain.Principal{{Kind: domain.PrincipalKindTeam, ID: "team-1"}}}}
	e.teams.members["team-1"] = []string{uuid.NewString()}
	a, err := e.openPlan(r, r.ReporterID)
	if err != nil {
		t.Fatal(err)
	}
	member := uuid.NewString()
	e.teams.teamsOf[member] = []string{"team-1"}
	if _, err := e.decide.Execute(approverCtx("user"), DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest}); errCode(err) != "REQUEST_APPROVAL_NOT_APPROVER" {
		t.Fatalf("outsider = %v", err)
	}
	e.teams.err = domain.ErrApprovalDirectoryUnavailable
	_, err = e.decide.Execute(userCtx(member, "user"), DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest})
	wantCode(t, err, "REQUEST_APPROVAL_DIRECTORY_UNAVAILABLE")
	e.teams.err = nil
	if _, err = e.decide.Execute(userCtx(member, "user"), DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest}); err != nil {
		t.Fatalf("team member must be able to approve: %v", err)
	}
}

func TestApproval_Decide_HandlerErrorRollsBackEverything(t *testing.T) {
	e, r, a := approvedSetup(t)
	e.art.decidedEr = errBoom
	_, err := e.decide.Execute(userCtx(r.ReporterID, "user"), DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	if e.s.approvals[a.ID].Status != domain.ApprovalStatusPending || e.s.requests[r.ID].Status != domain.RequestStatusAwaitingPlanApproval || len(e.s.events) != 0 {
		t.Fatalf("handler failure must roll back approval, request and events: %+v %s %d", e.s.approvals[a.ID].Status, e.s.requests[r.ID].Status, len(e.s.events))
	}
}

func TestApproval_Decide_RetryIsIdempotentAndLateDecisionIsRefused(t *testing.T) {
	e, r, a := approvedSetup(t)
	ctx := userCtx(r.ReporterID, "user")
	in := DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest}
	if _, err := e.decide.Execute(ctx, in); err != nil {
		t.Fatal(err)
	}
	events := len(e.s.events)
	if _, err := e.decide.Execute(ctx, in); err != nil || len(e.s.events) != events {
		t.Fatalf("retry by the same user must succeed silently: %v events %d->%d", err, events, len(e.s.events))
	}
	_, err := e.decide.Execute(ctx, DecideApprovalInput{ID: a.ID, Decision: "reject", Comment: "x"})
	wantCode(t, err, "REQUEST_APPROVAL_ALREADY_DECIDED")
	_, err = e.decide.Execute(approverCtx("admin"), in)
	wantCode(t, err, "REQUEST_APPROVAL_ALREADY_DECIDED")
}

func TestApproval_Decide_LateDecisionPersistsExpiryAndReturnsToBacklog(t *testing.T) {
	e, r, a := approvedSetup(t)
	e.s.now = a.DueAt.Add(time.Minute)
	_, err := e.decide.Execute(userCtx(r.ReporterID, "user"), DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest})
	wantCode(t, err, "REQUEST_APPROVAL_EXPIRED")
	if e.s.approvals[a.ID].Status != domain.ApprovalStatusExpired {
		t.Fatal("the expiry must be committed even though the call returns an error")
	}
	got := e.s.requests[r.ID]
	if got.Status != domain.RequestStatusRequestBacklog || got.ReturnReason != "approval_expired" || got.ReturnedFromStage != domain.ReturnStagePlan {
		t.Fatalf("request = %+v", got)
	}
}

func TestApproval_Cancel_RequesterOrAdminOnly(t *testing.T) {
	e, r, a := approvedSetup(t)
	_, err := e.cancel.Execute(approverCtx("user"), a.ID, "no longer needed")
	wantCode(t, err, "REQUEST_APPROVAL_FORBIDDEN")
	_, err = e.cancel.Execute(userCtx(r.ReporterID, "user"), a.ID, " ")
	wantCode(t, err, "REQUEST_APPROVAL_COMMENT_REQUIRED")
	got, err := e.cancel.Execute(userCtx(r.ReporterID, "user"), a.ID, "no longer needed")
	if err != nil || got.Status != domain.ApprovalStatusCancelled {
		t.Fatalf("%+v %v", got, err)
	}
	if len(e.art.closed) != 1 || e.art.closed[0] != "cancelled" {
		t.Fatalf("handler must hear about the close: %v", e.art.closed)
	}
	if _, err = e.cancel.Execute(userCtx(r.ReporterID, "user"), a.ID, "no longer needed"); err != nil {
		t.Fatalf("same caller retry must succeed: %v", err)
	}
	_, err = e.cancel.Execute(approverCtx("admin"), a.ID, "again")
	wantCode(t, err, "REQUEST_APPROVAL_ALREADY_DECIDED")
}

func TestApproval_CancelPendingForRequest_ClosesAllAndLocksRequestFirst(t *testing.T) {
	e, r, a := approvedSetup(t)
	if err := e.canceler.Execute(lcCtx(), r.ID, "returned"); err != nil {
		t.Fatal(err)
	}
	if e.s.approvals[a.ID].Status != domain.ApprovalStatusCancelled || len(e.art.closed) != 1 {
		t.Fatalf("status %s closed %v", e.s.approvals[a.ID].Status, e.art.closed)
	}
	if e.calls0() != "lock:request@t1" {
		t.Fatalf("Request must be locked first: %v", e.s.calls)
	}
	var p domain.ApprovalDecidedPayload
	_ = json.Unmarshal(e.s.events[len(e.s.events)-1].Payload, &p)
	if p.Decision != "cancelled" || p.RequestID != r.ID {
		t.Fatalf("payload = %+v", p)
	}
	if err := e.canceler.Execute(lcCtx(), r.ID, "returned"); err != nil {
		t.Fatalf("second call finds nothing pending and must be a no-op: %v", err)
	}
	// with approval disabled the registry is nil: rows still close
	off := &CancelPendingApprovalsForRequest{Repo: apprRepo{e.s}, Tx: e.s, Locker: apprLocker{e.s}, Outbox: e.s}
	if err := off.Execute(lcCtx(), r.ID, "x"); err != nil {
		t.Fatal(err)
	}
}

func TestApproval_Expire_ReturnStagePerSubject(t *testing.T) {
	tests := []struct {
		subject domain.SubjectType
		reqType domain.RequestType
		status  domain.RequestStatus
		want    domain.ReturnStage
	}{
		{domain.SubjectRequestType, domain.RequestTypeBug, domain.RequestStatusAwaitingTypeConfirmation, domain.ReturnStageClassification},
		{domain.SubjectSolution, domain.RequestTypeChangeRequest, domain.RequestStatusAwaitingAnalysisApproval, domain.ReturnStageAnalysis},
		{domain.SubjectFindings, domain.RequestTypeSpike, domain.RequestStatusAwaitingAnalysisApproval, domain.ReturnStageAnalysis},
		{domain.SubjectAnswer, domain.RequestTypeQuestion, domain.RequestStatusAwaitingAnalysisApproval, domain.ReturnStageAnalysis},
		{domain.SubjectPlan, domain.RequestTypeChangeRequest, domain.RequestStatusAwaitingPlanApproval, domain.ReturnStagePlan},
		{domain.SubjectTaskList, domain.RequestTypeTask, domain.RequestStatusAwaitingPlanApproval, domain.ReturnStagePlan},
		{domain.SubjectPreDeploy, domain.RequestTypeHotfix, domain.RequestStatusAwaitingPlanApproval, domain.ReturnStagePlan},
		{domain.SubjectPreDeploy, domain.RequestTypeOpsRequest, domain.RequestStatusExecuting, domain.ReturnStageTask}, // no phases: stage task
		{domain.SubjectPreDeploy, domain.RequestTypeChangeRequest, domain.RequestStatusExecuting, domain.ReturnStagePhase},
		{domain.SubjectPhase, domain.RequestTypeChangeRequest, domain.RequestStatusExecuting, domain.ReturnStagePhase},
	}
	for _, tc := range tests {
		t.Run(string(tc.subject)+"/"+string(tc.status), func(t *testing.T) {
			e := newApprEnv()
			r := e.s.seed(func(r *domain.Request) { r.Status, r.Type, r.Size = tc.status, tc.reqType, domain.RequestSizeM })
			due := e.s.now.Add(-time.Minute)
			a := domain.Approval{ID: uuid.NewString(), TenantID: "t1", RequestID: r.ID, SubjectType: tc.subject, SubjectID: "s1", Stage: string(tc.status),
				Status: domain.ApprovalStatusPending, RequestedBy: "system", DueAt: &due, Version: 1, CreatedAt: e.s.now.Add(-time.Hour), UpdatedAt: e.s.now}
			e.s.approvals[a.ID] = a
			n, err := e.expire.Execute(context.Background(), 10)
			if err != nil || n != 1 {
				t.Fatalf("expired %d, err %v", n, err)
			}
			got := e.s.requests[r.ID]
			if e.s.approvals[a.ID].Status != domain.ApprovalStatusExpired || got.Status != domain.RequestStatusRequestBacklog ||
				got.ReturnedFromStage != tc.want || got.ReturnReason != "approval_expired" {
				t.Fatalf("approval=%s request=%+v want stage %s", e.s.approvals[a.ID].Status, got, tc.want)
			}
		})
	}
}

func TestApproval_Expire_RunsUnderClaimTenantAndSurvivesOneFailure(t *testing.T) {
	e := newApprEnv()
	mk := func(fail bool) domain.Approval {
		r := e.s.seed(func(r *domain.Request) {
			r.Status, r.Type, r.Size = domain.RequestStatusAwaitingPlanApproval, domain.RequestTypeChangeRequest, domain.RequestSizeM
		})
		due := e.s.now.Add(-time.Minute)
		a := domain.Approval{ID: uuid.NewString(), TenantID: "t1", RequestID: r.ID, SubjectType: domain.SubjectPlan, SubjectID: r.ID, Stage: string(r.Status),
			Status: domain.ApprovalStatusPending, RequestedBy: "system", DueAt: &due, Version: 1, CreatedAt: e.s.now.Add(-time.Hour), UpdatedAt: e.s.now}
		e.s.approvals[a.ID] = a
		return a
	}
	bad, good := mk(true), mk(false)
	if bad.ID > good.ID {
		bad, good = good, bad // claims are sorted by id: make the failing one first
	}
	e.registry.Register(domain.SubjectPlan, failingFor{inner: e.registry.Get(domain.SubjectPlan), approvalID: bad.ID})
	n, err := e.expire.Execute(context.Background(), 10) // sweeper ctx has no tenant at all
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if e.s.approvals[bad.ID].Status != domain.ApprovalStatusPending || e.s.approvals[good.ID].Status != domain.ApprovalStatusExpired {
		t.Fatalf("bad=%s good=%s", e.s.approvals[bad.ID].Status, e.s.approvals[good.ID].Status)
	}
	for _, c := range e.s.calls {
		if strings.HasPrefix(c, "lock:request@") && c != "lock:request@t1" {
			t.Fatalf("the Request lock must run under the claim's tenant, got %s", c)
		}
	}
}

type failingFor struct {
	inner      SubjectHandler
	approvalID string
}

func (f failingFor) ValidateForRequest(ctx, tx context.Context, r domain.Request, st domain.SubjectType) (string, string, error) {
	return f.inner.ValidateForRequest(ctx, tx, r, st)
}
func (f failingFor) OnApproved(ctx, tx context.Context, a domain.Approval) error {
	return f.inner.OnApproved(ctx, tx, a)
}
func (f failingFor) OnRejected(ctx, tx context.Context, a domain.Approval) error {
	return f.inner.OnRejected(ctx, tx, a)
}
func (f failingFor) OnClosedWithoutDecision(ctx, tx context.Context, a domain.Approval, why string) error {
	if a.ID == f.approvalID {
		return errBoom
	}
	return f.inner.OnClosedWithoutDecision(ctx, tx, a, why)
}

func TestApproval_Expire_SkipsExtendedAndDecided(t *testing.T) {
	e, r, a := approvedSetup(t)
	_ = r
	e.s.now = a.DueAt.Add(time.Second)
	// extended between the claim and the lock: the per-candidate re-check must keep it pending
	if _, err := e.extend.Execute(userCtx(a.RequestedBy, "admin"), ExtendApprovalInput{ID: a.ID, ExtendSeconds: 3600}); err != nil {
		t.Fatal(err)
	}
	n, err := e.expire.Execute(context.Background(), 10)
	if err != nil || n != 0 || e.s.approvals[a.ID].Status != domain.ApprovalStatusPending {
		t.Fatalf("n=%d err=%v status=%s", n, err, e.s.approvals[a.ID].Status)
	}
}

func TestApproval_Remind_OncePerApproval(t *testing.T) {
	e, _, a := approvedSetup(t)
	e.s.now = a.CreatedAt.Add(a.DueAt.Sub(a.CreatedAt) / 2)
	if n, _ := e.remind.Execute(context.Background(), 10); n != 0 {
		t.Fatalf("before 75%% nothing is sent, got %d", n)
	}
	e.s.now = a.CreatedAt.Add(a.DueAt.Sub(a.CreatedAt) * 8 / 10)
	if n, err := e.remind.Execute(context.Background(), 10); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if n, _ := e.remind.Execute(context.Background(), 10); n != 0 {
		t.Fatalf("a second tick must not remind again, got %d", n)
	}
	last := e.s.events[len(e.s.events)-1]
	var p map[string]any
	_ = json.Unmarshal(last.Payload, &p)
	if last.Subject != "orca.request.approval.requested" || p["reason"] != "reminder" || p["reporter_id"] == "" {
		t.Fatalf("event %s %v", last.Subject, p)
	}
	// extending re-arms the reminder
	if _, err := e.extend.Execute(userCtx(a.RequestedBy, "admin"), ExtendApprovalInput{ID: a.ID, ExtendSeconds: 3600}); err != nil {
		t.Fatal(err)
	}
	if e.s.approvals[a.ID].RemindedAt != nil {
		t.Fatal("extend must clear reminded_at")
	}
}

func TestApproval_Extend_RulesAndEffect(t *testing.T) {
	e, r, a := approvedSetup(t)
	_, err := e.extend.Execute(approverCtx("user"), ExtendApprovalInput{ID: a.ID, ExtendSeconds: 60})
	wantCode(t, err, "REQUEST_APPROVAL_FORBIDDEN")
	_, err = e.extend.Execute(userCtx(a.RequestedBy, "user"), ExtendApprovalInput{ID: a.ID, ExtendSeconds: 0})
	wantCode(t, err, "REQUEST_APPROVAL_EXTEND_INVALID")
	_, err = e.extend.Execute(userCtx(a.RequestedBy, "user"), ExtendApprovalInput{ID: a.ID, ExtendSeconds: 60, ExpectedVersion: 7})
	wantCode(t, err, "REQUEST_APPROVAL_VERSION_CONFLICT")
	got, err := e.extend.Execute(userCtx(r.ReporterID, "user"), ExtendApprovalInput{ID: a.ID, ExtendSeconds: 3600, ExpectedVersion: 1})
	if err != nil || !got.DueAt.Equal(a.DueAt.Add(time.Hour)) || got.Version != 2 {
		t.Fatalf("%+v %v", got, err)
	}
	e.s.approvals[a.ID] = func() domain.Approval { x := e.s.approvals[a.ID]; x.Status = domain.ApprovalStatusApproved; return x }()
	_, err = e.extend.Execute(userCtx(r.ReporterID, "user"), ExtendApprovalInput{ID: a.ID, ExtendSeconds: 60})
	wantCode(t, err, "REQUEST_APPROVAL_ALREADY_DECIDED")
}

func TestApproval_PendingForUser_MatchesSnapshotAndSeparation(t *testing.T) {
	e := newApprEnv()
	r := e.planRequest()
	member, outsider := uuid.NewString(), uuid.NewString()
	e.s.policies = []domain.ApprovalPolicy{{SubjectType: domain.SubjectPlan, Enabled: true, Approvers: []domain.Principal{{Kind: domain.PrincipalKindTeam, ID: "team-1"}}}}
	e.teams.members["team-1"] = []string{member}
	if _, err := e.openPlan(r, r.ReporterID); err != nil {
		t.Fatal(err)
	}
	e.teams.teamsOf[member] = []string{"team-1"}
	list, _, err := e.pending.Execute(userCtx(member, "user"), "", 0, "")
	if err != nil || len(list) != 1 || list[0].RequestNumber != r.Number {
		t.Fatalf("member inbox = %v %v", list, err)
	}
	if list, _, _ = e.pending.Execute(userCtx(outsider, "user"), "", 0, ""); len(list) != 0 {
		t.Fatalf("outsider must see nothing: %v", list)
	}
	if list, _, _ = e.pending.Execute(userCtx(outsider, "admin"), "", 0, ""); len(list) != 1 {
		t.Fatalf("admin sees every pending approval: %v", list)
	}
	e.teams.err = domain.ErrApprovalDirectoryUnavailable
	if _, _, err = e.pending.Execute(userCtx(member, "user"), "", 0, ""); err != nil {
		t.Fatalf("a directory outage must degrade the inbox, not fail it: %v", err)
	}
	if _, _, err = e.pending.Execute(lcCtx(), "", 0, ""); errCode(err) != "REQUEST_APPROVAL_NO_USER" {
		t.Fatalf("anonymous = %v", err)
	}
}

func TestApproval_CrossTenantIsInvisible(t *testing.T) {
	e, r, a := approvedSetup(t)
	other := tenant.WithRole(tenant.WithUserID(tenant.WithTenantID(context.Background(), "t2"), r.ReporterID), "admin")
	_, err := e.decide.Execute(other, DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest})
	wantCode(t, err, "REQUEST_APPROVAL_NOT_FOUND")
	if _, err = (&GetApproval{Repo: apprRepo{e.s}}).Execute(other, a.ID); errCode(err) != "REQUEST_APPROVAL_NOT_FOUND" {
		t.Fatalf("get = %v", err)
	}
}

func TestApproval_RegistryMustCoverAllAndNoopStillLogsOnly(t *testing.T) {
	r := NewSubjectHandlerRegistry()
	if err := r.MustCoverAll(); err == nil {
		t.Fatal("empty registry must report missing handlers")
	}
	for _, st := range domain.AllSubjectTypes {
		r.Register(st, &TransitionSubjectHandler{})
	}
	if err := r.MustCoverAll(); err != nil {
		t.Fatalf("all eight subjects registered: %v", err)
	}
	var nilReg *SubjectHandlerRegistry
	if nilReg.Get(domain.SubjectPlan) != nil {
		t.Fatal("nil registry must resolve to no handler")
	}
}

func TestApproval_NoDirectoryCallInsideTransaction(t *testing.T) {
	e := newApprEnv()
	r := e.planRequest()
	member := uuid.NewString()
	e.teams.members["team-1"], e.teams.teamsOf[member] = []string{member, uuid.NewString()}, []string{"team-1"}
	e.admins.admins = []string{uuid.NewString()}
	e.s.policies = []domain.ApprovalPolicy{{SubjectType: domain.SubjectPlan, Enabled: true, AllowRequesterApprove: false,
		Approvers: []domain.Principal{{Kind: domain.PrincipalKindTeam, ID: "team-1"}, {Kind: domain.PrincipalKindRole, ID: "admin"}}}}
	a, err := e.openPlan(r, r.ReporterID)
	if err != nil {
		t.Fatal(err)
	}
	if e.teams.calls == 0 {
		t.Fatal("the eligibility check must have consulted the directory")
	}
	if _, err := e.decide.Execute(userCtx(member, "user"), DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest}); err != nil {
		t.Fatal(err)
	}
	if e.teams.inTx != 0 || e.admins.inTx != 0 {
		t.Fatalf("gRPC lookups ran inside a transaction: teams=%d admins=%d", e.teams.inTx, e.admins.inTx)
	}
}

func TestApproval_Open_RetriesWhenPolicyChangesBetweenLookupAndLock(t *testing.T) {
	e := newApprEnv()
	r := e.planRequest()
	e.teams.members["team-1"] = []string{uuid.NewString()}
	e.teams.members["team-2"] = []string{uuid.NewString()}
	policy := func(team string) []domain.ApprovalPolicy {
		return []domain.ApprovalPolicy{{SubjectType: domain.SubjectPlan, Enabled: true, Approvers: []domain.Principal{{Kind: domain.PrincipalKindTeam, ID: team}}}}
	}
	e.s.policies = policy("team-1")
	flipped := false
	e.teams.onCall = func() { // an admin edits the policy right after the first lookup started
		if !flipped {
			flipped = true
			e.s.policies = policy("team-2")
		}
	}
	a, err := e.openPlan(r, r.ReporterID)
	if err != nil {
		t.Fatal(err)
	}
	snap := e.s.approvers[a.ID]
	if len(snap) != 1 || snap[0].ID != "team-2" {
		t.Fatalf("the snapshot must reflect the policy in force at insert time: %v", snap)
	}
	if e.teams.inTx != 0 {
		t.Fatal("the retry must also look up outside the transaction")
	}
}
