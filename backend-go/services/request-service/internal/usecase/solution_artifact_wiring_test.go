package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// decisionKit is the real Choose + handler pair over the solution fakes, with a Decision record behind them.
type decisionKit struct {
	*handlerKit
	env    *artEnv
	choose *ChooseSolutionOption
}

func newDecisionKit(t *testing.T, selfPick SelfChoicePolicy) *decisionKit {
	t.Helper()
	k := newHandlerKit()
	env := newArtEnv()
	rec := NewRecordDecision(decView{env}, k.w).WithHighRiskServices(3)
	gates := NewApprovalGates(decView{env}, clarView{env})
	d := AnalysisApprovalHandlerDeps{Requests: k.w, Solutions: fakeSolutions{k.w}, Returner: nil, Transition: k.w.transitioner(), Outbox: k.w, Gates: gates}
	k.solH = NewSolutionApprovalHandler(d)
	return &decisionKit{handlerKit: k, env: env, choose: newChoose(k.w).WithDecisions(rec, selfPick)}
}

func TestChooseSolutionOption_RecordsDecisionAndRequiresRationaleForNonRecommended(t *testing.T) {
	k := newDecisionKit(t, nil)
	r, s := k.proposed(domain.RequestTypeChangeRequest, domain.SolutionKindSolution, validSolutionReply(t), nil)
	ctx := k.w.ctx()

	// opt-2 is not the recommendation: the pick is refused and leaves nothing behind.
	_, err := k.choose.Execute(ctx, ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: s.ID, OptionID: "opt-2"})
	requireCode(t, err, "REQUEST_DECISION_RATIONALE_REQUIRED")

	res, err := k.choose.Execute(ctx, ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: s.ID, OptionID: "opt-1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.DecisionStatus != string(domain.DecisionStatusEffective) || res.RequiresConfirmation {
		t.Fatalf("a normal-risk recommended pick is effective at once: %+v", res)
	}
	if len(k.env.decisions) != 1 {
		t.Fatalf("decisions = %d", len(k.env.decisions))
	}
}

func TestSolutionHandler_OnApprovedNeedsAnEffectiveDecisionForTheSameDigest(t *testing.T) {
	k := newDecisionKit(t, nil)
	r, s := k.proposed(domain.RequestTypeChangeRequest, domain.SolutionKindSolution, validSolutionReply(t), nil)
	ctx := k.w.ctx()

	// A choice written around the use case (no decision) cannot be approved.
	s.ChosenOption = intp(0)
	k.w.solutions[s.ID] = s
	digest := must(domain.DigestOptions(s.OptionsJSON, s.ChosenOption))
	requireCode(t, k.solH.OnApproved(ctx, ctx, approvalFor(r, s, domain.SubjectSolution, digest)), "REQUEST_DECISION_NOT_EFFECTIVE")

	res := must(k.choose.Execute(ctx, ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: s.ID, OptionID: "opt-1"}))
	if err := k.solH.OnApproved(ctx, ctx, approvalFor(r, s, domain.SubjectSolution, res.ApprovalDigest)); err != nil {
		t.Fatalf("approval with an effective decision: %v", err)
	}
	if k.w.solutions[s.ID].Status != domain.SolutionStatusApproved {
		t.Fatalf("status = %s", k.w.solutions[s.ID].Status)
	}
}

func TestSolutionHandler_BlockingOpenQuestionStopsApprovalUntilAnswered(t *testing.T) {
	k := newDecisionKit(t, nil)
	var doc map[string]any
	if err := json.Unmarshal([]byte(validSolutionReply(t)), &doc); err != nil {
		t.Fatal(err)
	}
	doc["open_questions"] = []map[string]any{{"id": "Q-1", "text": "Có MFA không?", "blocking": true}}
	raw, _ := json.Marshal(doc)
	r, s := k.proposed(domain.RequestTypeChangeRequest, domain.SolutionKindSolution, string(raw), nil)
	ctx := k.w.ctx()

	res := must(k.choose.Execute(ctx, ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: s.ID, OptionID: "opt-1"}))
	err := k.solH.OnApproved(ctx, ctx, approvalFor(r, s, domain.SubjectSolution, res.ApprovalDigest))
	requireCode(t, err, "REQUEST_SOLUTION_BLOCKING_QUESTIONS")
	if !strings.Contains(err.Error(), "Q-1") {
		t.Fatalf("the error should name the question: %v", err)
	}
}

func TestChooseSolutionOption_SelfChoicePolicyBlocksTheReporter(t *testing.T) {
	k := newDecisionKit(t, func(_ context.Context, _ domain.Request) (bool, error) { return false, nil })
	r, s := k.proposed(domain.RequestTypeChangeRequest, domain.SolutionKindSolution, validSolutionReply(t), nil)
	ctx := tenant.WithUserID(lcCtx(), r.ReporterID)
	_, err := k.choose.Execute(ctx, ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: s.ID, OptionID: "opt-1"})
	requireCode(t, err, "REQUEST_DECISION_SELF_CHOICE_FORBIDDEN")
}

func TestSolutionArtifactRecorder_StampsProvenanceAndChecksCoverage(t *testing.T) {
	rec := NewSolutionArtifactRecorder(NewMintArtifactIDs(newArtIndex()), &artRelations{})
	content := must(domain.InitialContent("Cache cho trang chủ", "Trang chủ tải chậm, cần cache đọc.", []domain.ACInput{{Text: "Trang chủ tải dưới 1 giây"}}, nil))
	req := must(domain.Request{ID: newID(), TenantID: "t1", Number: 42, ContentRevision: 3}.WithContent(content))

	// No coverage entry for AC-1: the document is refused with the AC id in the message.
	err := rec.CheckCoverage(req, []byte(validSolutionReply(t)))
	if err == nil || !strings.Contains(err.Error(), "AC-1") {
		t.Fatalf("an uncovered AC must be reported: %v", err)
	}
	var doc map[string]any
	_ = json.Unmarshal([]byte(validSolutionReply(t)), &doc)
	doc["requirement_coverage"] = []map[string]any{{"ac_id": "AC-1", "option_ids": []string{"opt-1"}, "status": "covered", "note": ""}}
	covered, _ := json.Marshal(doc)
	if err := rec.CheckCoverage(req, covered); err != nil {
		t.Fatalf("covered document: %v", err)
	}
	// A request without ACs has nothing to cover.
	if err := rec.CheckCoverage(domain.Request{ID: newID()}, []byte(validSolutionReply(t))); err != nil {
		t.Fatalf("no ACs: %v", err)
	}

	sol := domain.Solution{ID: newID(), RequestID: req.ID, Kind: domain.SolutionKindSolution, OptionsJSON: covered}
	run := domain.AnalysisRun{ID: newID(), Mode: domain.AnalysisModeComplete, Attempt: 2, ActorID: "user-1"}
	if err := rec.Annotate(req, run, &sol); err != nil {
		t.Fatal(err)
	}
	prov, err := domain.ParseProvenance(sol.ProvenanceJSON)
	if err != nil {
		t.Fatalf("stored provenance must parse back: %v", err)
	}
	if prov.RunID != run.ID || prov.Attempt != 2 || prov.Generator.Tool != "ai.complete" || prov.InputRefs[0].Revision != 3 || prov.Actor.ID != "user-1" {
		t.Fatalf("provenance = %+v", prov)
	}
	want := must(domain.DigestRaw(covered))
	if sol.ContentDigest != want || sol.InputRequestRevision != 3 || sol.SchemaVersion != 1 {
		t.Fatalf("digest=%s rev=%d schema=%d", sol.ContentDigest, sol.InputRequestRevision, sol.SchemaVersion)
	}
}

func TestSolutionHandler_ChosenOptionMustAnswerEveryAcceptanceCriterion(t *testing.T) {
	k := newHandlerKit()
	rec := NewSolutionArtifactRecorder(NewMintArtifactIDs(newArtIndex()), &artRelations{})
	d := AnalysisApprovalHandlerDeps{Requests: k.w, Solutions: fakeSolutions{k.w}, Transition: k.w.transitioner(), Outbox: k.w, Coverage: rec}
	h := NewSolutionApprovalHandler(d)

	var doc map[string]any
	_ = json.Unmarshal([]byte(validSolutionReply(t)), &doc)
	doc["requirement_coverage"] = []map[string]any{{"ac_id": "AC-1", "option_ids": []string{"opt-2"}, "status": "covered", "note": ""}}
	raw, _ := json.Marshal(doc)
	r, s := k.proposed(domain.RequestTypeChangeRequest, domain.SolutionKindSolution, string(raw), intp(0)) // opt-1 chosen
	content := must(domain.InitialContent("Cache cho trang chủ", "Trang chủ tải chậm, cần cache đọc.", []domain.ACInput{{Text: "Trang chủ tải dưới 1 giây"}}, nil))
	r = must(r.WithContent(content))
	k.w.requests[r.ID] = r

	ctx := k.w.ctx()
	digest := must(domain.DigestOptions(s.OptionsJSON, s.ChosenOption))
	err := h.OnApproved(ctx, ctx, approvalFor(r, s, domain.SubjectSolution, digest))
	if err == nil || !strings.Contains(err.Error(), "AC-1") {
		t.Fatalf("opt-1 does not cover AC-1: %v", err)
	}
	s.ChosenOption = intp(1) // opt-2 does
	k.w.solutions[s.ID] = s
	digest = must(domain.DigestOptions(s.OptionsJSON, s.ChosenOption))
	if err := h.OnApproved(ctx, ctx, approvalFor(r, s, domain.SubjectSolution, digest)); err != nil {
		t.Fatalf("opt-2 covers AC-1: %v", err)
	}
}

func TestSolutionHandler_HighRiskPickBlocksApprovalUntilConfirmed(t *testing.T) {
	k := newDecisionKit(t, nil)
	var doc map[string]any
	_ = json.Unmarshal([]byte(validSolutionReply(t)), &doc)
	doc["options"].([]any)[1].(map[string]any)["breaking_change"] = true // opt-2 breaks compatibility: high risk
	raw, _ := json.Marshal(doc)
	r, s := k.proposed(domain.RequestTypeChangeRequest, domain.SolutionKindSolution, string(raw), nil)
	ctx := k.w.ctx()

	res := must(k.choose.Execute(ctx, ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: s.ID, OptionID: "opt-2", Rationale: "Cần tách bảng đọc để giảm tải."}))
	if res.DecisionStatus != string(domain.DecisionStatusChosen) || !res.RequiresConfirmation {
		t.Fatalf("a breaking pick waits for confirmation: %+v", res)
	}
	requireCode(t, k.solH.OnApproved(ctx, ctx, approvalFor(r, s, domain.SubjectSolution, res.ApprovalDigest)), "REQUEST_DECISION_NOT_EFFECTIVE")

	live, _ := decView{k.env}.GetLiveBySubject(ctx, domain.DecisionSubjectSolutionOption, s.ID)
	confirm := NewConfirmDecision(k.w, decView{k.env}, k.w, k.w)
	if _, err := confirm.Execute(ctx, ConfirmDecisionInput{DecisionID: live.ID, ConfirmationText: "Tách bảng đọc"}); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if err := k.solH.OnApproved(ctx, ctx, approvalFor(r, s, domain.SubjectSolution, res.ApprovalDigest)); err != nil {
		t.Fatalf("approval after confirmation: %v", err)
	}
}

func TestSolutionArtifactRecorder_RetiredSolutionTakesItsDecisionWithIt(t *testing.T) {
	k := newDecisionKit(t, nil)
	r, s := k.proposed(domain.RequestTypeChangeRequest, domain.SolutionKindSolution, validSolutionReply(t), nil)
	ctx := k.w.ctx()
	must(k.choose.Execute(ctx, ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: s.ID, OptionID: "opt-1"}))

	rec := NewSolutionArtifactRecorder(NewMintArtifactIDs(newArtIndex()), &artRelations{}).WithDecisions(decView{k.env})
	fresh := domain.Solution{ID: newID(), RequestID: r.ID, Kind: domain.SolutionKindSolution, Seq: 2, OptionsJSON: []byte(validSolutionReply(t))}
	if err := rec.Record(ctx, r, fresh, domain.AnalysisRun{ID: newID()}, []string{s.ID}); err != nil {
		t.Fatal(err)
	}
	d, _ := decView{k.env}.GetLiveBySubject(ctx, domain.DecisionSubjectSolutionOption, s.ID)
	if d != nil {
		t.Fatalf("the retired solution still has a live decision: %+v", d)
	}
	var actions []domain.DecisionAction
	for _, h := range k.env.decHistory {
		actions = append(actions, h.Action)
	}
	if len(actions) < 2 || actions[len(actions)-1] != domain.DecisionActionSuperseded {
		t.Fatalf("history = %v", actions)
	}
}

func TestSolutionHandler_DiagnosisNeedsNoDecisionEvenWithGatesWired(t *testing.T) {
	k := newDecisionKit(t, nil)
	r, s := k.proposed(domain.RequestTypeBug, domain.SolutionKindDiagnosis, analysisReply(t, "diagnosis_valid.json"), nil)
	ctx := k.w.ctx()
	_, digest, err := k.solH.ValidateForRequest(ctx, ctx, r, domain.SubjectSolution)
	if err != nil {
		t.Fatal(err)
	}
	if err := k.solH.OnApproved(ctx, ctx, approvalFor(r, s, domain.SubjectSolution, digest)); err != nil {
		t.Fatalf("a diagnosis has nothing to choose, so no decision gates it: %v", err)
	}
	if len(k.env.decisions) != 0 {
		t.Fatalf("decisions = %d", len(k.env.decisions))
	}
}
