package httpgateway

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// fakeFlowClient records the RPCs the flow routes make; unscripted RPCs panic.
type fakeFlowClient struct {
	requestv1.RequestServiceClient
	requestv1.ApprovalServiceClient

	mu         sync.Mutex
	classifies []*requestv1.ClassifyRequestRequest
	confirms   []*requestv1.ConfirmRequestTypeRequest
	chooses    []*requestv1.ChooseSolutionOptionRequest
	lists      []*requestv1.ListSolutionsRequest
	phases     []*requestv1.StartPhaseRequest
	answers    []*requestv1.AnswerClarificationRequest
	cancels    []*requestv1.CancelRequestRequest
	plans      []*requestv1.GetPlanProposalRequest
	bundles    []*requestv1.GetPlanBundleFileRequest
	bundleErr  error
}

func (f *fakeFlowClient) CancelRequest(_ context.Context, in *requestv1.CancelRequestRequest, _ ...grpc.CallOption) (*requestv1.CancelRequestResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancels = append(f.cancels, in)
	return &requestv1.CancelRequestResponse{Request: &requestv1.Request{Id: in.GetRequestId()}}, nil
}

func (f *fakeFlowClient) ClassifyRequest(_ context.Context, in *requestv1.ClassifyRequestRequest, _ ...grpc.CallOption) (*requestv1.ClassifyRequestResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.classifies = append(f.classifies, in)
	return &requestv1.ClassifyRequestResponse{Request: &requestv1.Request{Id: in.GetRequestId()}}, nil
}

func (f *fakeFlowClient) ConfirmRequestType(_ context.Context, in *requestv1.ConfirmRequestTypeRequest, _ ...grpc.CallOption) (*requestv1.ConfirmRequestTypeResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.confirms = append(f.confirms, in)
	return &requestv1.ConfirmRequestTypeResponse{Request: &requestv1.Request{Id: in.GetRequestId()}}, nil
}

func (f *fakeFlowClient) ChooseSolutionOption(_ context.Context, in *requestv1.ChooseSolutionOptionRequest, _ ...grpc.CallOption) (*requestv1.ChooseSolutionOptionResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chooses = append(f.chooses, in)
	return &requestv1.ChooseSolutionOptionResponse{Solution: &requestv1.Solution{Id: in.GetSolutionId()}, ApprovalDigest: "dg"}, nil
}

func (f *fakeFlowClient) ListSolutions(_ context.Context, in *requestv1.ListSolutionsRequest, _ ...grpc.CallOption) (*requestv1.ListSolutionsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lists = append(f.lists, in)
	return &requestv1.ListSolutionsResponse{}, nil
}

func (f *fakeFlowClient) StartPhase(_ context.Context, in *requestv1.StartPhaseRequest, _ ...grpc.CallOption) (*requestv1.StartPhaseResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.phases = append(f.phases, in)
	return &requestv1.StartPhaseResponse{PhaseTaskId: in.GetPhaseTaskId()}, nil
}

func (f *fakeFlowClient) AnswerClarification(_ context.Context, in *requestv1.AnswerClarificationRequest, _ ...grpc.CallOption) (*requestv1.AnswerClarificationResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answers = append(f.answers, in)
	return &requestv1.AnswerClarificationResponse{Clarification: &requestv1.Clarification{Id: in.GetClarificationId()}, RequestStatus: "analyzing"}, nil
}

func (f *fakeFlowClient) GetPlanProposal(_ context.Context, in *requestv1.GetPlanProposalRequest, _ ...grpc.CallOption) (*requestv1.GetPlanProposalResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.plans = append(f.plans, in)
	return &requestv1.GetPlanProposalResponse{
		RunId: "run-1", Status: "succeeded", Engine: "openspec",
		Bundle:    &requestv1.PlanBundle{ChangeId: "req-1-x", Branch: "request/1-proposal", Commit: "abc", Files: []*requestv1.PlanBundleFile{{Path: "tasks.md", Size: 10}}},
		Checks:    []*requestv1.PlanCheck{{Code: "KNOWLEDGE_CONFLICT", Message: "ADR-12", Severity: "warning"}},
		Knowledge: &requestv1.PlanKnowledge{Available: true, Snapshot: "snap-1", Read: []string{"FR-1"}},
	}, nil
}

func (f *fakeFlowClient) GetPlanBundleFile(_ context.Context, in *requestv1.GetPlanBundleFileRequest, _ ...grpc.CallOption) (*requestv1.GetPlanBundleFileResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bundles = append(f.bundles, in)
	if f.bundleErr != nil {
		return nil, f.bundleErr
	}
	return &requestv1.GetPlanBundleFileResponse{Path: in.GetPath(), Content: "# tasks\n"}, nil
}

func flowRouter(f *fakeFlowClient) http.Handler {
	r := chi.NewRouter()
	mountRequestRoutes(r, f, f)
	return r
}

func TestRequestFlowRoutes_EmptyBodyAndURLParameterWins(t *testing.T) {
	f := &fakeFlowClient{}
	r := flowRouter(f)

	if rec := doRequestRoute(r, http.MethodPost, "/v1/requests/r1/classify", "", routesIdentity); rec.Code != http.StatusOK {
		t.Fatalf("classify with an empty body: %d %s", rec.Code, rec.Body.String())
	}
	// A body naming another Request must not redirect the route.
	if rec := doRequestRoute(r, http.MethodPost, "/v1/requests/r1/confirm-type", `{"id":"OTHER","type":"change_request","size":"S","expectedVersion":3}`, routesIdentity); rec.Code != http.StatusOK {
		t.Fatalf("confirm-type: %d %s", rec.Code, rec.Body.String())
	}
	if got := f.classifies[0].GetRequestId(); got != "r1" {
		t.Errorf("classify request id = %q", got)
	}
	c := f.confirms[0]
	if c.GetRequestId() != "r1" || c.GetType() != "change_request" || c.GetSize() != "S" || c.GetExpectedVersion() != 3 {
		t.Errorf("confirm rpc = %+v", c)
	}
}

func TestRequestFlowRoutes_SolutionPlanPhaseAndClarification(t *testing.T) {
	f := &fakeFlowClient{}
	r := flowRouter(f)

	if rec := doRequestRoute(r, http.MethodGet, "/v1/requests/r1/solutions?status=proposed&pageSize=500", "", routesIdentity); rec.Code != http.StatusOK {
		t.Fatalf("solutions: %d %s", rec.Code, rec.Body.String())
	}
	if in := f.lists[0]; in.GetRequestId() != "r1" || in.GetPageSize() != 100 {
		t.Errorf("list rpc = %+v", in)
	}
	rec := doRequestRoute(r, http.MethodPost, "/v1/requests/r1/solutions/s7/choose", `{"optionId":"o2","rationale":"cheaper"}`, routesIdentity)
	if rec.Code != http.StatusOK {
		t.Fatalf("choose: %d %s", rec.Code, rec.Body.String())
	}
	if in := f.chooses[0]; in.GetRequestId() != "r1" || in.GetSolutionId() != "s7" || in.GetOptionId() != "o2" || in.GetRationale() != "cheaper" {
		t.Errorf("choose rpc = %+v", in)
	}
	if rec = doRequestRoute(r, http.MethodPost, "/v1/requests/r1/phases/ph3/start", "", routesIdentity); rec.Code != http.StatusOK {
		t.Fatalf("start phase: %d %s", rec.Code, rec.Body.String())
	}
	if in := f.phases[0]; in.GetRequestId() != "r1" || in.GetPhaseTaskId() != "ph3" {
		t.Errorf("start rpc = %+v", in)
	}
	body := `{"answers":[{"questionId":"q1","valueJson":"\"yes\""}],"complete":true,"expectedVersion":2}`
	if rec = doRequestRoute(r, http.MethodPost, "/v1/clarifications/c5/answer", body, routesIdentity); rec.Code != http.StatusOK {
		t.Fatalf("answer: %d %s", rec.Code, rec.Body.String())
	}
	if in := f.answers[0]; in.GetClarificationId() != "c5" || !in.GetComplete() || len(in.GetAnswers()) != 1 || in.GetAnswers()[0].GetQuestionId() != "q1" {
		t.Errorf("answer rpc = %+v", in)
	}
}

func TestRequestFlowRoutes_BadInput(t *testing.T) {
	f := &fakeFlowClient{}
	r := flowRouter(f)
	if rec := doRequestRoute(r, http.MethodPost, "/v1/requests/r1/confirm-type", `{not json`, routesIdentity); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed body: %d", rec.Code)
	}
	if rec := doRequestRoute(r, http.MethodPost, "/v1/requests/r1/confirm-type", `{"type":"odd"}`, routesIdentity); rec.Code != http.StatusBadRequest || len(f.confirms) != 0 {
		t.Errorf("unknown type: %d, rpcs=%d", rec.Code, len(f.confirms))
	}
	if rec := doRequestRoute(r, http.MethodPost, "/v1/requests/r1/classify", "", nil); rec.Code != http.StatusUnauthorized || len(f.classifies) != 0 {
		t.Errorf("anonymous: %d, rpcs=%d", rec.Code, len(f.classifies))
	}
	// A JSON null body is an empty argument set, not a nil-map write.
	if rec := doRequestRoute(r, http.MethodPost, "/v1/requests/r1/cancel", `null`, routesIdentity); rec.Code != http.StatusOK || f.cancels[0].GetRequestId() != "r1" {
		t.Errorf("null body: %d %s", rec.Code, rec.Body.String())
	}
}

func TestRequestFlowRoutes_PlanBundle(t *testing.T) {
	f := &fakeFlowClient{}
	r := flowRouter(f)
	rec := doRequestRoute(r, http.MethodGet, "/v1/requests/r1/plan/bundle?runId=run-1", "", routesIdentity)
	if rec.Code != http.StatusOK {
		t.Fatalf("bundle: %d %s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"engine":"openspec"`, `"changeId":"req-1-x"`, `"branch":"request/1-proposal"`, `"tasks.md"`, `KNOWLEDGE_CONFLICT`, `"snapshot":"snap-1"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("body lacks %s: %s", want, rec.Body.String())
		}
	}
	if in := f.plans[0]; in.GetRequestId() != "r1" || in.GetRunId() != "run-1" {
		t.Errorf("rpc = %+v", in)
	}

	rec = doRequestRoute(r, http.MethodGet, "/v1/requests/r1/plan/bundle/file?path=specs%2Fai%2Fspec.md", "", routesIdentity)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"content":"# tasks\n"`) {
		t.Fatalf("file: %d %s", rec.Code, rec.Body.String())
	}
	if in := f.bundles[0]; in.GetRequestId() != "r1" || in.GetPath() != "specs/ai/spec.md" {
		t.Errorf("rpc = %+v", in)
	}
}

func TestRequestFlowRoutes_PlanBundleFileInputAndErrors(t *testing.T) {
	f := &fakeFlowClient{}
	r := flowRouter(f)
	if rec := doRequestRoute(r, http.MethodGet, "/v1/requests/r1/plan/bundle/file", "", routesIdentity); rec.Code != http.StatusBadRequest || len(f.bundles) != 0 {
		t.Errorf("no path: %d rpcs=%d", rec.Code, len(f.bundles))
	}
	if rec := doRequestRoute(r, http.MethodGet, "/v1/requests/r1/plan/bundle/file?path=tasks.md", "", nil); rec.Code != http.StatusUnauthorized || len(f.bundles) != 0 {
		t.Errorf("anonymous: %d rpcs=%d", rec.Code, len(f.bundles))
	}
	// The gateway does not decide what lies inside the change: it forwards the path and request-service refuses it.
	f.bundleErr = status.Error(codes.InvalidArgument, "the path must name a file inside the change")
	rec := doRequestRoute(r, http.MethodGet, "/v1/requests/r1/plan/bundle/file?path=..%2F..%2Fetc%2Fpasswd", "", routesIdentity)
	if rec.Code != http.StatusBadRequest || len(f.bundles) != 1 || f.bundles[0].GetPath() != "../../etc/passwd" {
		t.Errorf("a bad path must come back as 400 from the service: %d %s %+v", rec.Code, rec.Body.String(), f.bundles)
	}
	f.bundleErr = status.Error(codes.NotFound, "no such file")
	if rec := doRequestRoute(r, http.MethodGet, "/v1/requests/r1/plan/bundle/file?path=x.md", "", routesIdentity); rec.Code != http.StatusNotFound {
		t.Errorf("missing file: %d", rec.Code)
	}
}
