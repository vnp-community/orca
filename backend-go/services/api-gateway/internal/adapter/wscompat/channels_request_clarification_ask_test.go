package wscompat

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

type askClient struct {
	requestv1.RequestServiceClient
	got *requestv1.RequestClarificationRequest
	err error
}

func (a *askClient) RequestClarification(_ context.Context, in *requestv1.RequestClarificationRequest, _ ...grpc.CallOption) (*requestv1.RequestClarificationResponse, error) {
	a.got = in
	if a.err != nil {
		return nil, a.err
	}
	return &requestv1.RequestClarificationResponse{Clarification: &requestv1.Clarification{
		Id: "c1", RequestId: in.GetRequestId(), Source: in.GetSource(), SourceRef: in.GetSourceRef(),
		Status: requestv1.ClarificationStatus_CLARIFICATION_STATUS_OPEN, ResumeStatus: "analyzing", Version: 1,
		Questions: []*requestv1.ClarificationQuestion{{Id: "q1", QuestionKey: in.GetQuestions()[0].GetQuestionKey(), Kind: requestv1.QuestionKind_QUESTION_KIND_TEXT, Prompt: "p", Required: true}},
	}}, nil
}

func askRegistry(c *askClient) *Registry {
	r := NewRegistry()
	registerRequestChannels(r, c, nil, nil, false)
	return r
}

// A reviewer turns a blocking open question of a Solution into a Clarification; the service gets the ids and JSON texts as-is.
func TestClarificationRequest_AsksTheOpenQuestionsOfASolution(t *testing.T) {
	c := &askClient{}
	res, err := callRequestChannel(t, askRegistry(c), testRequestIdentity, context.Background(), "clarification.request", `{
		"requestId":"r1","source":"solution_open_question","sourceRef":"sol-1",
		"questions":[
			{"questionKey":"open_question:Q-1","kind":"text","prompt":"Which database?","reason":"Blocks approval of the Solution","required":true},
			{"questionKey":"open_question:Q-2","kind":"single_choice","prompt":"Which region?","reason":"Blocks approval","options":[{"id":"eu","label":"EU"}],"suggestedDefault":"eu"}
		]}`)
	if err != nil {
		t.Fatal(err)
	}
	in := c.got
	if in.GetRequestId() != "r1" || in.GetSource() != "solution_open_question" || in.GetSourceRef() != "sol-1" || len(in.GetQuestions()) != 2 {
		t.Fatalf("request = %v", in)
	}
	q1, q2 := in.GetQuestions()[0], in.GetQuestions()[1]
	if q1.GetKind() != requestv1.QuestionKind_QUESTION_KIND_TEXT || !q1.GetRequired() || q1.GetOptionsJson() != "" || q1.GetSuggestedDefaultJson() != "" {
		t.Fatalf("q1 = %v", q1)
	}
	if q2.GetKind() != requestv1.QuestionKind_QUESTION_KIND_SINGLE_CHOICE || q2.GetOptionsJson() != `[{"id":"eu","label":"EU"}]` || q2.GetSuggestedDefaultJson() != `"eu"` {
		t.Fatalf("q2 = %v", q2)
	}
	cl, _ := res["clarification"].(map[string]any)
	if cl["id"] != "c1" || cl["status"] != "open" || cl["sourceRef"] != "sol-1" || cl["resumeStatus"] != "analyzing" {
		t.Fatalf("view = %v", res)
	}
}

func TestClarificationRequest_RejectsBadShapesBeforeTheService(t *testing.T) {
	for name, args := range map[string]string{
		"no request":   `{"source":"manual","questions":[{"questionKey":"k","kind":"text"}]}`,
		"no source":    `{"requestId":"r1","questions":[{"questionKey":"k","kind":"text"}]}`,
		"no questions": `{"requestId":"r1","source":"manual","questions":[]}`,
		"unknown kind": `{"requestId":"r1","source":"manual","questions":[{"questionKey":"k","kind":"essay"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			c := &askClient{}
			if _, err := callRequestChannel(t, askRegistry(c), testRequestIdentity, context.Background(), "clarification.request", args); err == nil || c.got != nil {
				t.Fatalf("err = %v, service called = %v", err, c.got != nil)
			}
		})
	}
}

// The service refuses platform-only sources and states that cannot ask; its coded message reaches the client unchanged.
func TestClarificationRequest_RelaysTheServiceRefusalCode(t *testing.T) {
	c := &askClient{err: status.Error(codes.FailedPrecondition, "REQUEST_CLARIFICATION_STATE_NOT_ALLOWED: a readiness clarification is raised by the platform, not through this RPC")}
	_, err := callRequestChannel(t, askRegistry(c), testRequestIdentity, context.Background(), "clarification.request",
		`{"requestId":"r1","source":"readiness","questions":[{"questionKey":"k","kind":"text","prompt":"p","reason":"r"}]}`)
	if err == nil || !strings.HasPrefix(err.Error(), "REQUEST_CLARIFICATION_STATE_NOT_ALLOWED: ") {
		t.Fatalf("err = %v", err)
	}
}

// The UI lists the unanswered questions from this message (frontend parseBlockingQuestionIds), so the gateway must not
// replace a FailedPrecondition carrying it with the generic "state stale" text.
func TestRequestChannelError_BlockingQuestionsKeepTheirIds(t *testing.T) {
	err := status.Error(codes.FailedPrecondition, "REQUEST_SOLUTION_BLOCKING_QUESTIONS: blocking open questions are unanswered: Q-1, Q-2")
	if got := approvalChannelError(err).Error(); got != "REQUEST_SOLUTION_BLOCKING_QUESTIONS: blocking open questions are unanswered: Q-1, Q-2" {
		t.Fatalf("got %q", got)
	}
}
