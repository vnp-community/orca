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
	cleanups   []*requestv1.CleanupRequestProposalRequest
	engineGets []*requestv1.GetProjectEngineSettingsRequest
	engineSets []*requestv1.SetProjectEngineSettingsRequest
	profGets   []*requestv1.GetReviewDrawingProfileRequest
	profSaves  []*requestv1.SaveReviewDrawingProfileRequest
	profResets []*requestv1.ResetReviewDrawingProfileRequest
}

func (f *fakeFlowClient) GetReviewDrawingProfile(_ context.Context, in *requestv1.GetReviewDrawingProfileRequest, _ ...grpc.CallOption) (*requestv1.GetReviewDrawingProfileResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.profGets = append(f.profGets, in)
	return &requestv1.GetReviewDrawingProfileResponse{EffectiveJson: `{"style":{"detail":"balanced"}}`, Sources: map[string]string{"style.detail": "default"}}, nil
}

func (f *fakeFlowClient) SaveReviewDrawingProfile(_ context.Context, in *requestv1.SaveReviewDrawingProfileRequest, _ ...grpc.CallOption) (*requestv1.SaveReviewDrawingProfileResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.profSaves = append(f.profSaves, in)
	return &requestv1.SaveReviewDrawingProfileResponse{Layer: &requestv1.ReviewDrawingProfileLayer{ProjectId: in.GetProjectId(), SpecJson: in.GetSpecJson(), Version: in.GetExpectedVersion() + 1}}, nil
}

func (f *fakeFlowClient) ResetReviewDrawingProfile(_ context.Context, in *requestv1.ResetReviewDrawingProfileRequest, _ ...grpc.CallOption) (*requestv1.ResetReviewDrawingProfileResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.profResets = append(f.profResets, in)
	return &requestv1.ResetReviewDrawingProfileResponse{}, nil
}

func (f *fakeFlowClient) CleanupRequestProposal(_ context.Context, in *requestv1.CleanupRequestProposalRequest, _ ...grpc.CallOption) (*requestv1.CleanupRequestProposalResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleanups = append(f.cleanups, in)
	return &requestv1.CleanupRequestProposalResponse{Removed: true, Branch: "request/1-proposal"}, nil
}

func (f *fakeFlowClient) GetProjectEngineSettings(_ context.Context, in *requestv1.GetProjectEngineSettingsRequest, _ ...grpc.CallOption) (*requestv1.GetProjectEngineSettingsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.engineGets = append(f.engineGets, in)
	resp := &requestv1.GetProjectEngineSettingsResponse{Settings: &requestv1.ProjectEngineSettings{ProjectId: in.GetProjectId(), SolutionEngine: "openspec", SolutionAgent: true, Version: 2}}
	if in.GetCheck() {
		resp.Preflight = &requestv1.EnginePreflight{Ran: true, Ok: true, Gaps: []*requestv1.EnginePreflightNote{{Code: "REQUEST_KNOWLEDGE_SKILLS_MISSING", FixHint: "npx skills add"}}}
	}
	return resp, nil
}

func (f *fakeFlowClient) SetProjectEngineSettings(_ context.Context, in *requestv1.SetProjectEngineSettingsRequest, _ ...grpc.CallOption) (*requestv1.SetProjectEngineSettingsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.engineSets = append(f.engineSets, in)
	return &requestv1.SetProjectEngineSettingsResponse{Settings: &requestv1.ProjectEngineSettings{ProjectId: in.GetProjectId(), SolutionEngine: in.GetSolutionEngine(), SolutionAgent: in.GetSolutionAgent(), Version: 3}}, nil
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

func TestProjectEngineRoutes_ReadWithAndWithoutTheCheck_AndOnlyAnAdminSets(t *testing.T) {
	f := &fakeFlowClient{}
	r := flowRouter(f)
	rec := doRequestRoute(r, http.MethodGet, "/v1/projects/p1/request-engine", "", routesIdentity)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"solutionAgent":true`) || strings.Contains(rec.Body.String(), "preflight") {
		t.Fatalf("plain read: %d %s", rec.Code, rec.Body.String())
	}
	rec = doRequestRoute(r, http.MethodGet, "/v1/projects/p1/request-engine?check=true", "", routesIdentity)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "REQUEST_KNOWLEDGE_SKILLS_MISSING") || !strings.Contains(rec.Body.String(), "npx skills add") {
		t.Fatalf("checked read: %d %s", rec.Code, rec.Body.String())
	}
	if len(f.engineGets) != 2 || f.engineGets[0].GetCheck() || !f.engineGets[1].GetCheck() || f.engineGets[1].GetProjectId() != "p1" {
		t.Fatalf("rpcs = %+v", f.engineGets)
	}

	body := `{"projectId":"OTHER","engine":"openspec","solutionAgent":true,"expectedVersion":2}`
	if rec := doRequestRoute(r, http.MethodPut, "/v1/projects/p1/request-engine", body, routesIdentity); rec.Code != http.StatusForbidden || len(f.engineSets) != 0 {
		t.Fatalf("member: %d rpcs=%d %s", rec.Code, len(f.engineSets), rec.Body.String())
	}
	admin := *routesIdentity
	admin.Role = "admin"
	rec = doRequestRoute(r, http.MethodPut, "/v1/projects/p1/request-engine", body, &admin)
	if rec.Code != http.StatusOK || len(f.engineSets) != 1 {
		t.Fatalf("admin: %d %s", rec.Code, rec.Body.String())
	}
	if in := f.engineSets[0]; in.GetProjectId() != "p1" || in.GetSolutionEngine() != "openspec" || !in.GetSolutionAgent() || in.GetExpectedVersion() != 2 {
		t.Fatalf("a body must not redirect the route to another project: %+v", in)
	}
	if rec := doRequestRoute(r, http.MethodGet, "/v1/projects/p1/request-engine", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}
}

// The project routes own the /v1/projects prefix; the engine routes must live next to them in either mount order.
func TestProjectEngineRoutes_CoexistWithTheProjectRoutes(t *testing.T) {
	f := &fakeFlowClient{}
	orders := map[string]func(chi.Router){
		"projects first": func(r chi.Router) { mountProjectRoutes(r, nil); mountRequestRoutes(r, f, f) },
		"requests first": func(r chi.Router) { mountRequestRoutes(r, f, f); mountProjectRoutes(r, nil) },
	}
	for name, mount := range orders {
		t.Run(name, func(t *testing.T) {
			r := chi.NewRouter()
			mount(r)
			for _, tc := range []struct{ method, path string }{
				{http.MethodGet, "/v1/projects/p1/request-engine"}, {http.MethodPut, "/v1/projects/p1/request-engine"},
				{http.MethodGet, "/v1/projects/p1/repos"}, {http.MethodGet, "/v1/projects/p1"},
			} {
				if !r.Match(chi.NewRouteContext(), tc.method, tc.path) {
					t.Errorf("%s %s is not routed", tc.method, tc.path)
				}
			}
			rec := doRequestRoute(r, http.MethodGet, "/v1/projects/p1/request-engine", "", routesIdentity)
			if rec.Code != http.StatusOK {
				t.Fatalf("engine route shadowed: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRequestFlowRoutes_ProposalCleanupPostsTheRequestId(t *testing.T) {
	f := &fakeFlowClient{}
	r := flowRouter(f)
	rec := doRequestRoute(r, http.MethodPost, "/v1/requests/r1/proposal/cleanup", `{"id":"OTHER"}`, routesIdentity)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"removed":true`) || !strings.Contains(rec.Body.String(), "request/1-proposal") {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if len(f.cleanups) != 1 || f.cleanups[0].GetRequestId() != "r1" {
		t.Fatalf("a body must not redirect the cleanup to another Request: %+v", f.cleanups)
	}
	if rec := doRequestRoute(r, http.MethodPost, "/v1/requests/r1/proposal/cleanup", "", nil); rec.Code != http.StatusUnauthorized || len(f.cleanups) != 1 {
		t.Fatalf("anonymous: %d", rec.Code)
	}
}

func TestReviewProfileRoutes_ProjectAndTenantLayersAndOnlyAnAdminWrites(t *testing.T) {
	f := &fakeFlowClient{}
	r := flowRouter(f)
	admin := *routesIdentity
	admin.Role = "admin"

	if rec := doRequestRoute(r, http.MethodGet, "/v1/projects/p1/review-drawing-profile", "", routesIdentity); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"detail":"balanced"`) {
		t.Fatalf("project read: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doRequestRoute(r, http.MethodGet, "/v1/review-drawing-profile?projectId=OTHER", "", routesIdentity); rec.Code != http.StatusOK {
		t.Fatalf("tenant read: %d %s", rec.Code, rec.Body.String())
	}
	if len(f.profGets) != 2 || f.profGets[0].GetProjectId() != "p1" || f.profGets[1].GetProjectId() != "" {
		t.Fatalf("the URL decides the layer, a query cannot move the tenant route to a project: %+v", f.profGets)
	}

	body := `{"projectId":"OTHER","spec":{"schema":"review-drawing-profile/1"},"expectedVersion":2}`
	if rec := doRequestRoute(r, http.MethodPut, "/v1/projects/p1/review-drawing-profile", body, routesIdentity); rec.Code != http.StatusForbidden || len(f.profSaves) != 0 {
		t.Fatalf("member save: %d saves=%d %s", rec.Code, len(f.profSaves), rec.Body.String())
	}
	if rec := doRequestRoute(r, http.MethodPut, "/v1/projects/p1/review-drawing-profile", body, &admin); rec.Code != http.StatusOK || len(f.profSaves) != 1 {
		t.Fatalf("admin save: %d %s", rec.Code, rec.Body.String())
	}
	if in := f.profSaves[0]; in.GetProjectId() != "p1" || in.GetExpectedVersion() != 2 || !strings.Contains(in.GetSpecJson(), "review-drawing-profile/1") {
		t.Fatalf("a body must not redirect the route to another project: %+v", in)
	}
	if rec := doRequestRoute(r, http.MethodPut, "/v1/review-drawing-profile", body, &admin); rec.Code != http.StatusOK || f.profSaves[1].GetProjectId() != "" {
		t.Fatalf("tenant save: %d %+v", rec.Code, f.profSaves)
	}
	if rec := doRequestRoute(r, http.MethodPost, "/v1/projects/p1/review-drawing-profile/reset", `{"expectedVersion":3}`, &admin); rec.Code != http.StatusOK || len(f.profResets) != 1 || f.profResets[0].GetProjectId() != "p1" {
		t.Fatalf("reset: %d %+v", rec.Code, f.profResets)
	}
	if rec := doRequestRoute(r, http.MethodGet, "/v1/projects/p1/review-drawing-profile", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}
}
