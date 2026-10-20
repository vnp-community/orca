package httpgateway

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// planRevisionsClient answers the CR-REQ-045 RPCs on top of the flow fake.
type planRevisionsClient struct {
	*fakeFlowClient
	mu        sync.Mutex
	revisions []*requestv1.ListPlanRevisionsRequest
	generates []*requestv1.GeneratePlanRequest
	err       error
}

func (c *planRevisionsClient) ListPlanRevisions(_ context.Context, in *requestv1.ListPlanRevisionsRequest, _ ...grpc.CallOption) (*requestv1.ListPlanRevisionsResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.revisions = append(c.revisions, in)
	if c.err != nil {
		return nil, c.err
	}
	return &requestv1.ListPlanRevisionsResponse{
		Revisions:     []*requestv1.PlanRevision{{Revision: 2, RunId: "run-2", Status: "current", Engine: "openspec", RevisionFeedback: "split", FileCount: 3, FilesChanged: 1, TextKept: false}},
		NextPageToken: "2",
	}, nil
}

func (c *planRevisionsClient) GeneratePlan(_ context.Context, in *requestv1.GeneratePlanRequest, _ ...grpc.CallOption) (*requestv1.GeneratePlanResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generates = append(c.generates, in)
	return &requestv1.GeneratePlanResponse{RunId: "run-9"}, nil
}

func newPlanRevisionsClient() *planRevisionsClient {
	f := &fakeFlowClient{}
	return &planRevisionsClient{fakeFlowClient: f}
}

func TestRequestFlowRoutes_PlanRevisionsList(t *testing.T) {
	c := newPlanRevisionsClient()
	c.fakeFlowClient.RequestServiceClient = c
	r := flowRouter(c.fakeFlowClient)
	rec := doRequestRoute(r, http.MethodGet, "/v1/requests/r1/plan/revisions?pageSize=2&pageToken=3", "", routesIdentity)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	if in := c.revisions[0]; in.GetRequestId() != "r1" || in.GetPageSize() != 2 || in.GetPageToken() != "3" {
		t.Errorf("rpc = %+v", in)
	}
	for _, want := range []string{`"revisions":[`, `"revision":2`, `"revisionFeedback":"split"`, `"textKept":false`, `"filesChanged":1`, `"nextPageToken":"2"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("body lacks %s: %s", want, rec.Body.String())
		}
	}
	if rec := doRequestRoute(r, http.MethodGet, "/v1/requests/r1/plan/revisions", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: %d", rec.Code)
	}
}

func TestRequestFlowRoutes_PlanRevisionArgumentIsForwardedAsANumber(t *testing.T) {
	f := &fakeFlowClient{}
	r := flowRouter(f)
	if rec := doRequestRoute(r, http.MethodGet, "/v1/requests/r1/plan/bundle?revision=2", "", routesIdentity); rec.Code != http.StatusOK {
		t.Fatalf("bundle: %d %s", rec.Code, rec.Body.String())
	}
	if in := f.plans[0]; in.GetRevision() != 2 {
		t.Errorf("proposal rpc = %+v", in)
	}
	if rec := doRequestRoute(r, http.MethodGet, "/v1/requests/r1/plan/bundle/file?path=tasks.md&revision=1", "", routesIdentity); rec.Code != http.StatusOK {
		t.Fatalf("file: %d %s", rec.Code, rec.Body.String())
	}
	if in := f.bundles[0]; in.GetRevision() != 1 || in.GetPath() != "tasks.md" {
		t.Errorf("file rpc = %+v", in)
	}
	if rec := doRequestRoute(r, http.MethodGet, "/v1/requests/r1/plan/bundle", "", routesIdentity); rec.Code != http.StatusOK || f.plans[1].GetRevision() != 0 {
		t.Errorf("no revision means the current version: %d %+v", rec.Code, f.plans[1])
	}
	for _, bad := range []string{"abc", "-1", "1.5"} {
		rec := doRequestRoute(r, http.MethodGet, "/v1/requests/r1/plan/bundle?revision="+bad, "", routesIdentity)
		if rec.Code != http.StatusBadRequest || len(f.plans) != 2 {
			t.Errorf("revision %q: %d, rpcs=%d", bad, rec.Code, len(f.plans))
		}
	}
}

func TestRequestFlowRoutes_GeneratePlanCarriesTheIdempotencyKey(t *testing.T) {
	c := newPlanRevisionsClient()
	c.fakeFlowClient.RequestServiceClient = c
	r := flowRouter(c.fakeFlowClient)
	rec := doRequestRoute(r, http.MethodPost, "/v1/requests/r1/plan", `{"feedback":"split it","idempotencyKey":"click-1"}`, routesIdentity)
	if rec.Code != http.StatusOK {
		t.Fatalf("generate: %d %s", rec.Code, rec.Body.String())
	}
	if in := c.generates[0]; in.GetRequestId() != "r1" || in.GetFeedback() != "split it" || in.GetIdempotencyKey() != "click-1" {
		t.Errorf("rpc = %+v", in)
	}
}

func TestRequestFlowRoutes_PlanRevisionErrorsKeepTheirCodeAndGetASensibleStatus(t *testing.T) {
	cases := map[string]struct {
		grpc codes.Code
		http int
	}{
		"REQUEST_PLAN_REVISION_IN_PROGRESS":        {codes.FailedPrecondition, http.StatusPreconditionFailed},
		"REQUEST_PLAN_REVISION_NOT_APPLICABLE":     {codes.FailedPrecondition, http.StatusPreconditionFailed},
		"REQUEST_PLAN_REVISION_NO_CHANGE":          {codes.FailedPrecondition, http.StatusPreconditionFailed},
		"REQUEST_PLAN_REVISION_TEXT_PRUNED":        {codes.FailedPrecondition, http.StatusPreconditionFailed},
		"REQUEST_PLAN_REVISION_NOT_FOUND":          {codes.NotFound, http.StatusNotFound},
		"REQUEST_PLAN_REVISION_PAGE_TOKEN_INVALID": {codes.InvalidArgument, http.StatusBadRequest},
	}
	for code, want := range cases {
		c := newPlanRevisionsClient()
		c.fakeFlowClient.RequestServiceClient = c
		c.err = status.Error(want.grpc, code+": nope")
		r := flowRouter(c.fakeFlowClient)
		rec := doRequestRoute(r, http.MethodGet, "/v1/requests/r1/plan/revisions", "", routesIdentity)
		if rec.Code != want.http || !strings.Contains(rec.Body.String(), `"`+code+`"`) {
			t.Errorf("%s: %d %s", code, rec.Code, rec.Body.String())
		}
	}
}
