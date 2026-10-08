package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type recordingConfirm struct {
	got []ConfirmInput
	err error
}

func (c *recordingConfirm) Execute(_ context.Context, in ConfirmInput) (domain.Request, error) {
	c.got = append(c.got, in)
	return domain.Request{}, c.err
}

func typeEnv() (*apprEnv, *recordingConfirm, *RequestTypeApprovalRecorder, domain.Request) {
	e := newApprEnv()
	conf := &recordingConfirm{}
	h := e.registry.Get(domain.SubjectRequestType).(*RequestTypeApprovalHandler)
	h.Confirm = conf
	rec := &RequestTypeApprovalRecorder{Open: e.open, Repo: apprRepo{e.s}, Tx: e.s, Locker: apprLocker{e.s}, Outbox: e.s}
	r := e.s.seed(func(r *domain.Request) {
		r.Status, r.Type, r.Size, r.Urgency, r.ReporterID = domain.RequestStatusAwaitingTypeConfirmation, domain.RequestTypeBug, domain.RequestSizeM, domain.UrgencyNormal, uuid.NewString()
	})
	return e, conf, rec, r
}

func TestApprovalRequestType_ProposalOpensApprovalOwnedBySystem(t *testing.T) {
	e, _, rec, r := typeEnv()
	if err := rec.RequestTypeApproval(lcCtx(), r.ID); err != nil {
		t.Fatal(err)
	}
	if len(e.s.approvals) != 1 {
		t.Fatalf("approvals = %d", len(e.s.approvals))
	}
	for _, a := range e.s.approvals {
		if a.SubjectType != domain.SubjectRequestType || a.SubjectID != r.ID || a.RequestedBy != "system" || a.Stage != "awaiting_type_confirmation" {
			t.Fatalf("approval = %+v", a)
		}
	}
	if err := rec.RequestTypeApproval(lcCtx(), r.ID); err != nil || len(e.s.approvals) != 1 {
		t.Fatalf("opening twice must stay one pending approval: %v", err)
	}
}

func TestApprovalRequestType_ApproveConfirmsTheProposedType(t *testing.T) {
	e, conf, rec, r := typeEnv()
	_ = rec.RequestTypeApproval(lcCtx(), r.ID)
	var a domain.Approval
	for _, x := range e.s.approvals {
		a = x
	}
	if _, err := e.decide.Execute(userCtx(r.ReporterID, "user"), DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest}); err != nil {
		t.Fatal(err)
	}
	if len(conf.got) != 1 || conf.got[0].Type != "bug" || conf.got[0].Size != "M" || conf.got[0].ActorID != r.ReporterID || conf.got[0].ActorKind != domain.ActorKindUser {
		t.Fatalf("confirm input = %+v", conf.got)
	}
}

func TestApprovalRequestType_ConfirmFailureRollsBackApproval(t *testing.T) {
	e, conf, rec, r := typeEnv()
	conf.err = errBoom
	_ = rec.RequestTypeApproval(lcCtx(), r.ID)
	var a domain.Approval
	for _, x := range e.s.approvals {
		a = x
	}
	if _, err := e.decide.Execute(userCtx(r.ReporterID, "user"), DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest}); err == nil {
		t.Fatal("confirm failure must fail the approval")
	}
	if e.s.approvals[a.ID].Status != domain.ApprovalStatusPending {
		t.Fatal("approval must stay pending")
	}
}

func TestApprovalRequestType_RejectReturnsToBacklogFromClassification(t *testing.T) {
	e, _, rec, r := typeEnv()
	_ = rec.RequestTypeApproval(lcCtx(), r.ID)
	var a domain.Approval
	for _, x := range e.s.approvals {
		a = x
	}
	if _, err := e.decide.Execute(userCtx(r.ReporterID, "user"), DecideApprovalInput{ID: a.ID, Decision: "reject", Comment: "wrong type", ExpectedDigest: a.SubjectDigest}); err != nil {
		t.Fatal(err)
	}
	got := e.s.requests[r.ID]
	if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedFromStage != domain.ReturnStageClassification || got.ReturnReason != "wrong type" {
		t.Fatalf("request = %+v", got)
	}
}

func TestApprovalRequestType_DirectConfirmationSettlesPendingApprovalOnce(t *testing.T) {
	e, _, rec, r := typeEnv()
	_ = rec.RequestTypeApproval(lcCtx(), r.ID)
	user := uuid.NewString()
	if err := rec.Approve(userCtx(user, "user"), r.ID, user); err != nil {
		t.Fatal(err)
	}
	for _, a := range e.s.approvals {
		if a.Status != domain.ApprovalStatusApproved || a.DecidedBy == nil || *a.DecidedBy != user {
			t.Fatalf("approval = %+v", a)
		}
	}
	events := len(e.s.events)
	if err := rec.Approve(userCtx(user, "user"), r.ID, user); err != nil || len(e.s.events) != events {
		t.Fatalf("settling with nothing pending is a silent no-op: %v", err)
	}
}

func TestApprovalRequestType_ValidateNeedsProposedTypeAndStage(t *testing.T) {
	h := &RequestTypeApprovalHandler{}
	_, _, err := h.ValidateForRequest(context.Background(), context.Background(), domain.Request{Status: domain.RequestStatusClassifying, Type: domain.RequestTypeBug}, domain.SubjectRequestType)
	wantCode(t, err, "REQUEST_APPROVAL_STAGE_MISMATCH")
	_, _, err = h.ValidateForRequest(context.Background(), context.Background(), domain.Request{Status: domain.RequestStatusAwaitingTypeConfirmation}, domain.SubjectRequestType)
	wantCode(t, err, "REQUEST_APPROVAL_SUBJECT_NOT_FOUND")
	r := domain.Request{ID: "r", Status: domain.RequestStatusAwaitingTypeConfirmation, Type: domain.RequestTypeBug, Size: domain.RequestSizeM}
	id1, d1, _ := h.ValidateForRequest(context.Background(), context.Background(), r, domain.SubjectRequestType)
	_, d2, _ := h.ValidateForRequest(context.Background(), context.Background(), r, domain.SubjectRequestType)
	r.Size = domain.RequestSizeL
	_, d3, _ := h.ValidateForRequest(context.Background(), context.Background(), r, domain.SubjectRequestType)
	if id1 != "r" || d1 != d2 || d1 == d3 {
		t.Fatalf("digest must be stable for the same proposal and change with it: %s %s %s", d1, d2, d3)
	}
}
