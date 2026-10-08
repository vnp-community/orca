package contracttest

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
)

// solArtifactsDecisionsAndGates runs one Solution through the real stores and the real Approve RPC with the
// CR-REQ-027/028 pieces wired: coverage retry, seq/provenance/index/relations, Decision record, approval gates.
func solArtifactsDecisionsAndGates(t *testing.T, env SolutionEnv) {
	k := newSolKitWith(t, env, true)

	var doc map[string]any
	if err := json.Unmarshal([]byte(validSolutionDoc(t)), &doc); err != nil {
		t.Fatal(err)
	}
	withoutCoverage, _ := json.Marshal(doc)
	doc["requirement_coverage"] = []map[string]any{{"ac_id": "AC-1", "option_ids": []string{"opt-1"}, "status": "covered", "note": ""}}
	doc["open_questions"] = []map[string]any{{"id": "Q-1", "text": "TTL chấp nhận được là bao nhiêu?", "blocking": true}}
	doc["assumptions"] = []map[string]any{{"id": "A-1", "text": "Tải đọc gấp mười lần tải ghi", "needs_confirmation": false}}
	covered, _ := json.Marshal(doc)
	k.completer.replies = []string{string(withoutCoverage), string(covered)}

	r := seedRequest(t, k.env.IntakeEnv, k.ctx, domain.RequestStatusAnalyzing, func(r *domain.Request) {
		r.Type, r.TypeSource, r.Size, r.ReporterID = domain.RequestTypeChangeRequest, domain.TypeSourceHuman, domain.RequestSizeM, k.userID
		c := mustContent(t, "Cache cho trang chủ", "Trang chủ tải chậm, cần cache đọc để giảm tải.", "Trang chủ tải dưới 1 giây")
		*r = mustWithContent(t, *r, c)
	})

	gen, err := k.generate.Execute(k.ctx, usecase.GenerateSolutionInput{RequestID: r.ID})
	if err != nil {
		t.Fatal(err)
	}
	if run := k.waitRun(t, gen.RunID); run.Status != domain.RunStatusSucceeded {
		t.Fatalf("run = %+v", run)
	}
	// The first reply left AC-1 without a coverage entry: the model got one corrective retry naming it.
	if len(k.completer.prompts) != 2 || !strings.Contains(k.completer.prompts[1], "AC-1 has no requirement_coverage entry") {
		t.Fatalf("prompts = %d, retry note missing", len(k.completer.prompts))
	}
	if !strings.Contains(k.completer.prompts[0], "- AC-1: Trang chủ tải dưới 1 giây") {
		t.Fatal("the prompt must list the active acceptance criteria")
	}

	sol, err := env.SolutionStore.Get(k.ctx, gen.SolutionID)
	if err != nil {
		t.Fatal(err)
	}
	if sol.Seq != 1 || sol.ContentDigest == "" || sol.InputRequestRevision != r.ContentRevision || len(sol.ProvenanceJSON) == 0 {
		t.Fatalf("artifact columns = seq %d digest %q rev %d prov %d bytes", sol.Seq, sol.ContentDigest, sol.InputRequestRevision, len(sol.ProvenanceJSON))
	}
	prov, err := domain.ParseProvenance(sol.ProvenanceJSON)
	if err != nil || prov.RunID != gen.RunID || prov.Generator.Tool != "ai.complete" || prov.Actor.ID != k.userID {
		t.Fatalf("provenance = %+v %v", prov, err)
	}
	if want, _ := domain.DigestRaw(sol.OptionsJSON); sol.ContentDigest != want {
		t.Fatalf("content digest %s != %s", sol.ContentDigest, want)
	}
	display := domain.FormatSolutionID(r.Number, sol.Seq)
	for _, id := range []string{display, domain.FormatOptionID(display, 1), domain.FormatOptionID(display, 2)} {
		e, err := env.Artifacts.Index.Resolve(k.ctx, id)
		if err != nil || e.ArtifactID != sol.ID {
			t.Fatalf("resolve %s: %+v %v", id, e, err)
		}
	}
	rels, err := env.Artifacts.Relations.ListByRequest(k.ctx, r.ID)
	if err != nil || len(rels) != 1 || rels[0].Rel != domain.RelationDerivedFrom || rels[0].FromID != sol.ID || rels[0].ToID != r.ID {
		t.Fatalf("relations = %+v %v", rels, err)
	}

	// A pick that is not the recommendation needs a rationale; the refusal rolls the whole pick back.
	as := k.rpc()
	_, err = k.client.ChooseSolutionOption(as, &requestv1.ChooseSolutionOptionRequest{RequestId: r.ID, SolutionId: sol.ID, OptionId: "opt-2"})
	wantStatus(t, err, codes.InvalidArgument, "REQUEST_DECISION_RATIONALE_REQUIRED")
	if d, err := env.Artifacts.Decisions.GetLiveBySubject(k.ctx, domain.DecisionSubjectSolutionOption, sol.ID); err != nil || d != nil {
		t.Fatalf("a refused pick left a decision: %+v %v", d, err)
	}
	if got, _ := env.SolutionStore.Get(k.ctx, sol.ID); got.ChosenOption != nil {
		t.Fatalf("a refused pick left chosen_option = %d", *got.ChosenOption)
	}

	chosen, err := k.client.ChooseSolutionOption(as, &requestv1.ChooseSolutionOptionRequest{RequestId: r.ID, SolutionId: sol.ID, OptionId: "opt-1"})
	if err != nil || chosen.GetDecisionStatus() != string(domain.DecisionStatusEffective) || chosen.GetRequiresConfirmation() {
		t.Fatalf("choose: %+v %v", chosen, err)
	}
	live, err := env.Artifacts.Decisions.GetLiveBySubject(k.ctx, domain.DecisionSubjectSolutionOption, sol.ID)
	if err != nil || live == nil || live.SubjectDigest != chosen.GetApprovalDigest() {
		t.Fatalf("decision = %+v %v", live, err)
	}

	// Approve through the real engine: the blocking question stops it until a clarification answers it.
	pending := k.pendingApproval(t, domain.SubjectSolution, sol.ID)
	_, err = k.approvals.Approve(as, approveRequest(pending.ID, chosen.GetApprovalDigest()))
	wantStatus(t, err, codes.FailedPrecondition, "REQUEST_SOLUTION_BLOCKING_QUESTIONS")
	if got := k.requestStatus(t, r.ID); got != domain.RequestStatusAwaitingAnalysisApproval {
		t.Fatalf("a refused approval moved the request to %s", got)
	}

	now := time.Now().UTC()
	rev := r.ContentRevision
	clar := domain.Clarification{
		ID: uuid.NewString(), TenantID: k.tenantID, RequestID: r.ID, Seq: 1, Source: domain.ClarificationSourceSolutionOpenQuestion, SourceRef: sol.ID,
		Status: domain.ClarificationStatusOpen, ResumeStatus: domain.RequestStatusAnalyzing, Round: 1, AskedRequestRevision: rev,
		DueAt: now.Add(24 * time.Hour), CreatedBy: k.userID, CreatedAt: now, Version: 1,
		Questions: []domain.ClarificationQuestion{{
			ID: uuid.NewString(), Seq: 1, QuestionKey: "open_question:Q-1", Kind: domain.QuestionKindText, Prompt: "TTL chấp nhận được là bao nhiêu?",
			Reason: "câu hỏi chặn của phương án", Required: true,
		}},
		Assignees: []domain.Principal{{Kind: domain.PrincipalKindUser, ID: k.userID}},
	}
	if err := env.Artifacts.Clarifications.Insert(k.ctx, clar); err != nil {
		t.Fatalf("insert clarification: %v", err)
	}
	// Open is not enough: only an answered question lifts the gate.
	_, err = k.approvals.Approve(as, approveRequest(pending.ID, chosen.GetApprovalDigest()))
	wantStatus(t, err, codes.FailedPrecondition, "REQUEST_SOLUTION_BLOCKING_QUESTIONS")
	if err := env.Tx.InTx(k.ctx, func(ctx context.Context) error {
		if err := env.Artifacts.Clarifications.UpdateAnswers(ctx, clar.ID, []usecase.AnswerRecord{{
			QuestionID: clar.Questions[0].ID, Value: json.RawMessage(`"30 giây"`), Source: "user", By: k.userID, At: now,
		}}, 1); err != nil {
			return err
		}
		return env.Artifacts.Clarifications.MarkAnswered(ctx, clar.ID, rev, now, 2)
	}); err != nil {
		t.Fatalf("answer clarification: %v", err)
	}
	res, err := k.approvals.Approve(as, approveRequest(pending.ID, chosen.GetApprovalDigest()))
	if err != nil || res.GetRequestStatus() != string(domain.RequestStatusPlanning) {
		t.Fatalf("approve: %+v %v", res, err)
	}
	if got, _ := env.SolutionStore.Get(k.ctx, sol.ID); got.Status != domain.SolutionStatusApproved || got.Seq != 1 || len(got.ProvenanceJSON) == 0 {
		t.Fatalf("approved solution lost its artifact columns: %+v", got)
	}
	subjects := env.OutboxSubjects(t, k.tenantID)
	if countOf(subjects, domain.SubjectDecisionRecorded) != 1 {
		t.Fatalf("decision.recorded events: %v", subjects)
	}
	solRegenerateSupersedesDecision(t, env)
}

// A regenerated proposal retires the old Solution; its Decision must go with it or it would gate nothing real.
func solRegenerateSupersedesDecision(t *testing.T, env SolutionEnv) {
	k := newSolKitWith(t, env, true)
	var doc map[string]any
	_ = json.Unmarshal([]byte(validSolutionDoc(t)), &doc)
	reply, _ := json.Marshal(doc) // the request has no AC, so no coverage is required
	k.completer.replies = []string{string(reply)}
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)

	first, err := k.generate.Execute(k.ctx, usecase.GenerateSolutionInput{RequestID: r.ID})
	if err != nil {
		t.Fatal(err)
	}
	k.waitRun(t, first.RunID)
	if _, err := k.choose.Execute(k.ctx, usecase.ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: first.SolutionID, OptionID: "opt-1"}); err != nil {
		t.Fatal(err)
	}
	if d, _ := env.Artifacts.Decisions.GetLiveBySubject(k.ctx, domain.DecisionSubjectSolutionOption, first.SolutionID); d == nil {
		t.Fatal("the pick should have recorded a live decision")
	}
	second, err := k.generate.Execute(k.ctx, usecase.GenerateSolutionInput{RequestID: r.ID, Feedback: "thử phương án khác"})
	if err != nil {
		t.Fatal(err)
	}
	k.waitRun(t, second.RunID)
	old, _ := env.SolutionStore.Get(k.ctx, first.SolutionID)
	fresh, _ := env.SolutionStore.Get(k.ctx, second.SolutionID)
	if old.Status != domain.SolutionStatusSuperseded || fresh.Seq != 2 {
		t.Fatalf("old=%s fresh seq=%d", old.Status, fresh.Seq)
	}
	if d, err := env.Artifacts.Decisions.GetLiveBySubject(k.ctx, domain.DecisionSubjectSolutionOption, first.SolutionID); err != nil || d != nil {
		t.Fatalf("the superseded solution kept a live decision: %+v %v", d, err)
	}
	rels, _ := env.Artifacts.Relations.ListByRequest(k.ctx, r.ID)
	var supersedes int
	for _, rel := range rels {
		if rel.Rel == domain.RelationSupersedes && rel.FromID == second.SolutionID && rel.ToID == first.SolutionID {
			supersedes++
		}
	}
	if supersedes != 1 {
		t.Fatalf("supersedes edges = %d in %+v", supersedes, rels)
	}
}

func mustContent(t *testing.T, title, body, ac string) domain.RequestContent {
	t.Helper()
	c, err := domain.InitialContent(title, body, []domain.ACInput{{Text: ac}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func mustWithContent(t *testing.T, r domain.Request, c domain.RequestContent) domain.Request {
	t.Helper()
	out, err := r.WithContent(c)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func approveRequest(id, digest string) *requestv1.ApproveRequest {
	return &requestv1.ApproveRequest{Id: id, Comment: "ok", ExpectedDigest: digest}
}
