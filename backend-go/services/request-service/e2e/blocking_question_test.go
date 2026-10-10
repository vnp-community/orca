//go:build e2e

package e2e

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"google.golang.org/grpc/codes"
)

// A proposal with a blocking open question cannot be approved. The reviewer asks it again as a Clarification, answers it,
// and the analysis restarts with the answer, so the new proposal can be approved (CR-REQ-028 section 2.5).
func TestBlockingOpenQuestion_AskAnswerThenApprove(t *testing.T) {
	k := newKit(t)
	k.enableFlow()
	w := &world{k: k, typ: "change_request", size: "M"}
	r := k.confirm(k.classified("e2e "+blockingQuestionMarker, w.typ, w.size), w.typ, w.size)
	w.request = k.waitStatus(r.GetId(), "analyzing")
	first := w.solveAndWait()

	var doc struct {
		OpenQuestions []struct {
			ID       string `json:"id"`
			Blocking bool   `json:"blocking"`
		} `json:"open_questions"`
	}
	if err := json.Unmarshal([]byte(first.GetOptionsJson()), &doc); err != nil || len(doc.OpenQuestions) != 1 || !doc.OpenQuestions[0].Blocking {
		t.Fatalf("the stub model should ask one blocking question, got %s (%v)", first.GetOptionsJson(), err)
	}
	qid := doc.OpenQuestions[0].ID

	// Approval is refused and names the question.
	chosen := retry(k, "ChooseSolutionOption", func() (*requestv1.ChooseSolutionOptionResponse, error) {
		return k.req.ChooseSolutionOption(k.asAdmin(), &requestv1.ChooseSolutionOptionRequest{RequestId: w.id(), SolutionId: first.GetId(), OptionId: "opt-1", Comment: "the recommended option"})
	})
	pending := w.pendingApproval("solution", "solution")
	_, err := k.appr.Approve(k.asAdmin(), &requestv1.ApproveRequest{Id: pending.GetId(), ExpectedDigest: chosen.GetApprovalDigest()})
	if code(err) != codes.FailedPrecondition || !strings.Contains(errText(err), "REQUEST_SOLUTION_BLOCKING_QUESTIONS") || !strings.Contains(errText(err), qid) {
		t.Fatalf("Approve = %v, want FailedPrecondition REQUEST_SOLUTION_BLOCKING_QUESTIONS naming %s", err, qid)
	}

	// Ask it again: the Request waits for information and the stale approval is gone.
	asked, err := k.req.RequestClarification(k.asReporter(), &requestv1.RequestClarificationRequest{
		RequestId: w.id(), Source: "solution_open_question", SourceRef: first.GetId(),
		Questions: []*requestv1.ClarificationQuestion{{
			QuestionKey: "open_question:" + qid, Kind: requestv1.QuestionKind_QUESTION_KIND_TEXT, Prompt: "Which database should hold the data?",
			Reason: "A blocking open question of the proposal", Required: true,
		}},
	})
	if err != nil {
		t.Fatalf("RequestClarification: %v", err)
	}
	c := asked.GetClarification()
	if got := k.waitStatus(w.id(), "awaiting_information"); got.GetStatus() != "awaiting_information" {
		t.Fatalf("status = %s", got.GetStatus())
	}
	if _, err := k.req.AnswerClarification(k.asReporter(), &requestv1.AnswerClarificationRequest{
		ClarificationId: c.GetId(), Complete: true, ExpectedVersion: c.GetVersion(),
		Answers: []*requestv1.AnswerItem{{QuestionId: c.GetQuestions()[0].GetId(), ValueJson: `"postgres"`}},
	}); err != nil {
		t.Fatalf("AnswerClarification: %v", err)
	}

	// The answer retires the old proposal and a new one is made with the answer in its prompt, without the question.
	var second *requestv1.Solution
	eventually(t, 45*time.Second, "a new proposal after the answer", func() (bool, string) {
		resp, err := k.req.ListSolutions(k.asReporter(), &requestv1.ListSolutionsRequest{RequestId: w.id(), Status: requestv1.SolutionStatus_SOLUTION_STATUS_PROPOSED})
		if err != nil {
			return false, err.Error()
		}
		for _, run := range resp.GetRuns() {
			if run.GetStatus() == "failed" {
				t.Fatalf("regeneration failed: %s: %s", run.GetErrorCode(), run.GetErrorMessage())
			}
		}
		for _, s := range resp.GetSolutions() {
			if s.GetId() != first.GetId() {
				second = s
				return true, ""
			}
		}
		return false, "no proposed Solution besides the first"
	})
	var answered bool
	for _, call := range theStack.stubs.Script.Calls() {
		if p, _ := call.Params["prompt"].(string); strings.Contains(p, blockingQuestionMarker) && strings.Contains(p, "<clarifications>") && strings.Contains(p, `"postgres"`) {
			answered = true
		}
	}
	if !answered {
		t.Fatal("the regenerated analysis never saw the answer")
	}
	if strings.Contains(second.GetOptionsJson(), `"blocking":true`) {
		t.Fatalf("the new proposal still blocks: %s", second.GetOptionsJson())
	}

	w.approveAnalysis() // choose, then approve: no refusal now
}
