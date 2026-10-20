package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func reviewEventSubjects(out *memOutbox) []string {
	var s []string
	for _, ev := range out.events {
		s = append(s, ev.Subject)
	}
	return s
}

func reviewEventCount(out *memOutbox, subject string) int {
	n := 0
	for _, ev := range out.events {
		if ev.Subject == subject {
			n++
		}
	}
	return n
}

func TestReviewEvents_NilPublishesNothing(t *testing.T) {
	var e *ReviewEvents
	e.Publish(context.Background(), domain.SubjectReviewGenerated, ReviewEventPayload{})
	e.runFinished(context.Background(), ReviewRun{}, ReviewRunSucceeded, "")
	(&ReviewEvents{}).Publish(context.Background(), domain.SubjectReviewGenerated, ReviewEventPayload{})
}

func TestReviewEvents_RunLifecycleAndPayloadHoldsNoText(t *testing.T) {
	e := newRunEnv(t, domain.RequestStatusExecuting)
	out := &memOutbox{}
	e.svc.Events = &ReviewEvents{Outbox: out}
	e.agent.replies = []func(AgentPromptInput) (AgentPromptResult, error){agentReply(goodDesignResult())}
	if _, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID}); err != nil {
		t.Fatal(err)
	}
	finishAll(e.reviewEnv)

	if got := reviewEventCount(out, domain.SubjectReviewProgress); got != 2 {
		t.Errorf("a run is announced when it starts and when it ends: %d in %v", got, reviewEventSubjects(out))
	}
	if got := reviewEventCount(out, domain.SubjectReviewGenerated); got < 1 {
		t.Errorf("a review that was built is announced: %v", reviewEventSubjects(out))
	}
	for _, ev := range out.events {
		var m map[string]any
		if err := json.Unmarshal(ev.Payload, &m); err != nil {
			t.Fatal(err)
		}
		for k := range m {
			switch k {
			case "request_id", "review_id", "phase", "run_id", "run_kind", "run_status", "decision", "actor_id", "error_code", "approved", "inputs_digest":
			default:
				t.Errorf("%s carries %q: only ids and enums may leave the service", ev.Subject, k)
			}
		}
		if m["request_id"] != e.req.ID {
			t.Errorf("%s names another request: %v", ev.Subject, m["request_id"])
		}
	}
}

func TestReviewEvents_FailedRunIsProgressOnly(t *testing.T) {
	out := &memOutbox{}
	ev := &ReviewEvents{Outbox: out}
	ev.runFinished(tctxT1(), ReviewRun{ID: "r1", RequestID: "q1", ReviewID: "v1", Kind: ReviewRunContext}, ReviewRunFailed, "REQUEST_REVIEW_AGENT_FAILED")
	if got := reviewEventSubjects(out); len(got) != 1 || got[0] != domain.SubjectReviewProgress {
		t.Fatalf("a failed run changes nothing for the reader but its progress: %v", got)
	}
	ev.runFinished(tctxT1(), ReviewRun{ID: "r2", RequestID: "q1", ReviewID: "v1", Kind: ReviewRunDraw}, ReviewRunSucceeded, "")
	if reviewEventCount(out, domain.SubjectReviewArtifactRegenerated) != 1 {
		t.Fatalf("a draw run that ended well announces its drawings: %v", reviewEventSubjects(out))
	}
}

func TestReviewEvents_DecideAndApproveAreAnnounced(t *testing.T) {
	e := newDecideEnv(t)
	out := &memOutbox{}
	e.svc.Events = &ReviewEvents{Outbox: out}
	if _, err := e.decide(t, e.breaking, domain.DecisionSendBack, "note that must not leave"); err != nil {
		t.Fatal(err)
	}
	e.refresh(t)
	if _, err := e.decider.Approve(e.ctx, ApproveInput{RequestID: e.req.ID, ReviewID: e.review.Stored.ID, Note: "approval note", ExpectedVersion: e.review.Stored.Version, ApprovedBy: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	if got := reviewEventCount(out, domain.SubjectReviewDecided); got != 2 {
		t.Fatalf("one event for the decision and one for the approval: %v", reviewEventSubjects(out))
	}
	for _, ev := range out.events {
		if strings.Contains(string(ev.Payload), "must not leave") || strings.Contains(string(ev.Payload), "approval note") {
			t.Errorf("a note travelled in %s: %s", ev.Subject, ev.Payload)
		}
	}
}
