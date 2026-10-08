package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type handlerKit struct {
	w     *solWorld
	solH  SubjectHandler
	findH SubjectHandler
	ansH  SubjectHandler
}

func newHandlerKit() *handlerKit {
	w := newSolWorld()
	returner := NewReturnRequestToBacklog(w, w.transitioner(), lcHistory{w.lcStore}, NoopApprovalCanceller{}, NoActiveExecutionGuard{}, w, w)
	d := AnalysisApprovalHandlerDeps{Requests: w, Solutions: fakeSolutions{w}, Returner: returner, Transition: w.transitioner(), Outbox: w}
	return &handlerKit{w: w, solH: NewSolutionApprovalHandler(d), findH: NewFindingsApprovalHandler(d), ansH: NewAnswerApprovalHandler(d)}
}

// proposed seeds a request waiting for approval with one proposed Solution of the given kind.
func (k *handlerKit) proposed(typ domain.RequestType, kind domain.SolutionKind, doc string, chosen *int) (domain.Request, domain.Solution) {
	r := k.w.seedRequest(typ, domain.RequestStatusAwaitingAnalysisApproval)
	s := domain.Solution{ID: uuid.NewString(), TenantID: "t1", RequestID: r.ID, Kind: kind, Status: domain.SolutionStatusProposed,
		OptionsJSON: []byte(doc), ChosenOption: chosen, Version: 1, CreatedAt: time.Now()}
	k.w.solutions[s.ID] = s
	return r, s
}

func approvalFor(r domain.Request, s domain.Solution, st domain.SubjectType, digest string) domain.Approval {
	by := "approver-1"
	return domain.Approval{ID: uuid.NewString(), TenantID: "t1", RequestID: r.ID, SubjectType: st, SubjectID: s.ID, SubjectDigest: digest, DecidedBy: &by, Comment: "chưa đủ rõ"}
}

func intp(i int) *int { return &i }

func TestSolutionHandler_ValidateForRequestBindsTheDigestOfTheCurrentChoice(t *testing.T) {
	k := newHandlerKit()
	doc := validSolutionReply(t)
	r, s := k.proposed(domain.RequestTypeChangeRequest, domain.SolutionKindSolution, doc, nil)
	ctx := k.w.ctx()
	// Opening happens before any choice: the digest is the no-choice one.
	id, digest, err := k.solH.ValidateForRequest(ctx, ctx, r, domain.SubjectSolution)
	if err != nil || id != s.ID || digest != must(domain.DigestOptions([]byte(doc), nil)) {
		t.Fatalf("open: id=%s digest=%s err=%v", id, digest, err)
	}
	s.ChosenOption = intp(1)
	k.w.solutions[s.ID] = s
	_, digest, err = k.solH.ValidateForRequest(ctx, ctx, r, domain.SubjectSolution)
	if err != nil || digest != must(domain.DigestOptions([]byte(doc), intp(1))) {
		t.Fatalf("after choosing: digest=%s err=%v", digest, err)
	}

	empty := k.w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAwaitingAnalysisApproval)
	_, _, err = k.solH.ValidateForRequest(ctx, ctx, empty, domain.SubjectSolution)
	requireCode(t, err, "REQUEST_SOLUTION_NOT_PROPOSED")

	// The contract the approval engine relies on: a request outside the gate and a foreign subject are refused.
	moved := r
	moved.Status = domain.RequestStatusNew
	if _, _, err := k.solH.ValidateForRequest(ctx, ctx, moved, domain.SubjectSolution); err == nil {
		t.Fatal("a request outside awaiting_analysis_approval must be refused")
	}
	if _, _, err := k.solH.ValidateForRequest(ctx, ctx, r, domain.SubjectPlan); err == nil {
		t.Fatal("a foreign subject type must be refused")
	}
}

func TestSolutionHandler_DiagnosisNeedsNoChoice(t *testing.T) {
	k := newHandlerKit()
	r, s := k.proposed(domain.RequestTypeBug, domain.SolutionKindDiagnosis, analysisReply(t, "diagnosis_valid.json"), nil)
	ctx := k.w.ctx()
	id, digest, err := k.solH.ValidateForRequest(ctx, ctx, r, domain.SubjectSolution)
	if err != nil || id != s.ID || digest == "" {
		t.Fatalf("id=%s digest=%q err=%v", id, digest, err)
	}
	a := approvalFor(r, s, domain.SubjectSolution, digest)
	if err := k.solH.OnApproved(ctx, ctx, a); err != nil {
		t.Fatal(err)
	}
	if k.w.solutions[s.ID].Status != domain.SolutionStatusApproved || k.w.requests[r.ID].Status != domain.RequestStatusPlanning {
		t.Fatalf("solution=%s request=%s", k.w.solutions[s.ID].Status, k.w.requests[r.ID].Status)
	}
}

func TestSolutionHandler_OnApprovedMovesTheRequestToPlanning(t *testing.T) {
	k := newHandlerKit()
	doc := validSolutionReply(t)
	r, s := k.proposed(domain.RequestTypeChangeRequest, domain.SolutionKindSolution, doc, intp(0))
	ctx := k.w.ctx()
	digest := must(domain.DigestOptions([]byte(doc), intp(0)))
	if err := k.solH.OnApproved(ctx, ctx, approvalFor(r, s, domain.SubjectSolution, digest)); err != nil {
		t.Fatal(err)
	}
	if got := k.w.solutions[s.ID]; got.Status != domain.SolutionStatusApproved || got.ChosenOption == nil || *got.ChosenOption != 0 {
		t.Fatalf("solution = %+v", got)
	}
	if st := k.w.requests[r.ID].Status; st != domain.RequestStatusPlanning {
		t.Fatalf("request = %s", st)
	}
	var found bool
	for _, e := range k.w.events {
		if e.Subject != domain.SubjectSolutionApproved {
			continue
		}
		found = true
		var p map[string]any
		_ = json.Unmarshal(e.Payload, &p)
		if _, leaks := p["options"]; leaks || p["request_id"] != r.ID || p["solution_id"] != s.ID || p["kind"] != "solution" || p["chosen_option"] != float64(0) {
			t.Fatalf("payload = %v", p)
		}
	}
	if !found {
		t.Fatalf("no solution.approved event in %v", k.w.outboxSubjects())
	}
	// Redelivery of the same decision changes nothing.
	before := len(k.w.events)
	if err := k.solH.OnApproved(ctx, ctx, approvalFor(r, s, domain.SubjectSolution, digest)); err != nil || len(k.w.events) != before {
		t.Fatalf("redelivery: err=%v events %d -> %d", err, before, len(k.w.events))
	}
}

func TestSolutionHandler_OnApprovedRefusesChangedContentAndMissingChoice(t *testing.T) {
	k := newHandlerKit()
	doc := validSolutionReply(t)
	r, s := k.proposed(domain.RequestTypeChangeRequest, domain.SolutionKindSolution, doc, intp(0))
	ctx := k.w.ctx()
	stale := must(domain.DigestOptions([]byte(doc), intp(1))) // reviewer saw option 2, option 1 is stored
	err := k.solH.OnApproved(ctx, ctx, approvalFor(r, s, domain.SubjectSolution, stale))
	requireCode(t, err, "REQUEST_APPROVAL_DIGEST_MISMATCH")

	edited := s
	edited.OptionsJSON = []byte(`{"schema_version":1,"edited":true}`)
	k.w.solutions[s.ID] = edited
	good := must(domain.DigestOptions([]byte(doc), intp(0)))
	requireCode(t, k.solH.OnApproved(ctx, ctx, approvalFor(r, s, domain.SubjectSolution, good)), "REQUEST_APPROVAL_DIGEST_MISMATCH")

	k2 := newHandlerKit()
	r2, s2 := k2.proposed(domain.RequestTypeChangeRequest, domain.SolutionKindSolution, doc, nil)
	requireCode(t, k2.solH.OnApproved(ctx, ctx, approvalFor(r2, s2, domain.SubjectSolution, "")), "REQUEST_SOLUTION_OPTION_NOT_CHOSEN")
	if k2.w.requests[r2.ID].Status != domain.RequestStatusAwaitingAnalysisApproval {
		t.Fatal("request moved without a choice")
	}
}

func TestSolutionHandler_OnRejectedReturnsToBacklogWithoutLeakingTheChoice(t *testing.T) {
	k := newHandlerKit()
	r, s := k.proposed(domain.RequestTypeChangeRequest, domain.SolutionKindSolution, validSolutionReply(t), intp(1))
	ctx := k.w.ctx()
	if err := k.solH.OnRejected(ctx, ctx, approvalFor(r, s, domain.SubjectSolution, "")); err != nil {
		t.Fatal(err)
	}
	got := k.w.solutions[s.ID]
	if got.Status != domain.SolutionStatusRejected || got.ChosenOption != nil {
		t.Fatalf("solution = %+v", got)
	}
	req := k.w.requests[r.ID]
	if req.Status != domain.RequestStatusRequestBacklog || req.ReturnedFromStage != domain.ReturnStageAnalysis || req.ReturnReason != "chưa đủ rõ" {
		t.Fatalf("request = %+v", req)
	}
	if len(k.w.history) != 1 {
		t.Fatalf("return history = %d", len(k.w.history))
	}
	if err := k.solH.OnRejected(ctx, ctx, approvalFor(r, s, domain.SubjectSolution, "")); err != nil {
		t.Fatalf("redelivery: %v", err)
	}
}

func TestFindingsAndAnswerHandlers_CompleteTheRequestWithoutPlanning(t *testing.T) {
	cases := []struct {
		name    string
		typ     domain.RequestType
		kind    domain.SolutionKind
		doc     string
		subject domain.SubjectType
		pick    func(k *handlerKit) SubjectHandler
	}{
		{"findings", domain.RequestTypeSpike, domain.SolutionKindFindings, "findings_valid.json", domain.SubjectFindings, func(k *handlerKit) SubjectHandler { return k.findH }},
		{"answer", domain.RequestTypeQuestion, domain.SolutionKindAnswer, "answer_valid.json", domain.SubjectAnswer, func(k *handlerKit) SubjectHandler { return k.ansH }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := newHandlerKit()
			h := tc.pick(k)
			ctx := k.w.ctx()
			r, s := k.proposed(tc.typ, tc.kind, analysisReply(t, tc.doc), nil)
			id, digest, err := h.ValidateForRequest(ctx, ctx, r, tc.subject)
			if err != nil || id != s.ID || digest != must(domain.DigestOptions(s.OptionsJSON, nil)) {
				t.Fatalf("validate: id=%s digest=%s err=%v", id, digest, err)
			}
			if err := h.OnApproved(ctx, ctx, approvalFor(r, s, tc.subject, digest)); err != nil {
				t.Fatal(err)
			}
			if st := k.w.requests[r.ID].Status; st != domain.RequestStatusCompleted {
				t.Fatalf("request = %s, want completed (no plan, no task)", st)
			}
			if len(k.w.solutions) != 1 || k.w.solutions[s.ID].Status != domain.SolutionStatusApproved {
				t.Fatalf("solutions = %+v", k.w.solutions)
			}
			if k.w.requests[r.ID].Type != tc.typ {
				t.Fatal("type changed")
			}
		})
	}
}

func TestFindingsHandler_RejectGoesBackToTheBacklogAndIgnoresOtherKinds(t *testing.T) {
	k := newHandlerKit()
	ctx := k.w.ctx()
	r, s := k.proposed(domain.RequestTypeSpike, domain.SolutionKindFindings, analysisReply(t, "findings_valid.json"), nil)
	if err := k.findH.OnRejected(ctx, ctx, approvalFor(r, s, domain.SubjectFindings, "")); err != nil {
		t.Fatal(err)
	}
	if k.w.requests[r.ID].Status != domain.RequestStatusRequestBacklog || k.w.solutions[s.ID].Status != domain.SolutionStatusRejected {
		t.Fatalf("request=%s solution=%s", k.w.requests[r.ID].Status, k.w.solutions[s.ID].Status)
	}
	// A findings handler must not decide a solution-kind row.
	r2, s2 := k.proposed(domain.RequestTypeChangeRequest, domain.SolutionKindSolution, validSolutionReply(t), intp(0))
	requireCode(t, k.findH.OnApproved(ctx, ctx, approvalFor(r2, s2, domain.SubjectFindings, "")), "REQUEST_SOLUTION_NOT_FOUND")
	// ...and a solution of another request is never reachable.
	r3 := k.w.seedRequest(domain.RequestTypeSpike, domain.RequestStatusAwaitingAnalysisApproval)
	_, s3 := k.proposed(domain.RequestTypeSpike, domain.SolutionKindFindings, analysisReply(t, "findings_valid.json"), nil)
	requireCode(t, k.findH.OnApproved(ctx, ctx, approvalFor(r3, s3, domain.SubjectFindings, "")), "REQUEST_SOLUTION_NOT_FOUND")
}

func TestSolutionHandler_OnClosedWithoutDecisionSupersedesOnlyForTypeChange(t *testing.T) {
	k := newHandlerKit()
	ctx := k.w.ctx()
	r, s := k.proposed(domain.RequestTypeChangeRequest, domain.SolutionKindSolution, validSolutionReply(t), intp(0))
	a := approvalFor(r, s, domain.SubjectSolution, "")
	for _, why := range []string{"cancelled", "expired", "revision_requested"} {
		if err := k.solH.OnClosedWithoutDecision(ctx, ctx, a, why); err != nil || k.w.solutions[s.ID].Status != domain.SolutionStatusProposed {
			t.Fatalf("%s: err=%v status=%s", why, err, k.w.solutions[s.ID].Status)
		}
	}
	if err := k.solH.OnClosedWithoutDecision(ctx, ctx, a, "type_changed"); err != nil || k.w.solutions[s.ID].Status != domain.SolutionStatusSuperseded {
		t.Fatalf("type_changed: err=%v status=%s", err, k.w.solutions[s.ID].Status)
	}
	if err := k.solH.OnClosedWithoutDecision(ctx, ctx, a, "type_changed"); err != nil {
		t.Fatalf("second delivery: %v", err)
	}
	gone := a
	gone.SubjectID = uuid.NewString()
	if err := k.solH.OnClosedWithoutDecision(ctx, ctx, gone, "type_changed"); err != nil {
		t.Fatalf("a missing solution must not break cancellation: %v", err)
	}
}

func TestSupersedeForTypeChange_RetiresEveryOpenKind(t *testing.T) {
	k := newHandlerKit()
	r, s := k.proposed(domain.RequestTypeBug, domain.SolutionKindDiagnosis, analysisReply(t, "diagnosis_valid.json"), nil)
	if err := SupersedeForTypeChange(k.w.ctx(), fakeSolutions{k.w}, r.ID); err != nil {
		t.Fatal(err)
	}
	if k.w.solutions[s.ID].Status != domain.SolutionStatusSuperseded {
		t.Fatalf("status = %s", k.w.solutions[s.ID].Status)
	}
	if err := SupersedeForTypeChange(context.Background(), fakeSolutions{k.w}, r.ID); err == nil {
		t.Fatal("tenant is required")
	}
}

// The shared contract itself runs against real stores in contracttest (SubjectHandlerContract).
func TestAnalysisHandlers_RegisterAsRealHandlers(t *testing.T) {
	k := newHandlerKit()
	reg := NewSubjectHandlerRegistry()
	reg.Register(domain.SubjectSolution, k.solH)
	reg.Register(domain.SubjectFindings, k.findH)
	reg.Register(domain.SubjectAnswer, k.ansH)
	for _, st := range []domain.SubjectType{domain.SubjectSolution, domain.SubjectFindings, domain.SubjectAnswer} {
		if _, isNoop := reg.Get(st).(*NoopSubjectHandler); isNoop || reg.Get(st) == nil {
			t.Fatalf("%s must have a real handler", st)
		}
	}
	_ = errors.New
}
