package wscompat

import (
	"context"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// planRevisionsClient lets a test choose what the service lists.
type planRevisionsClient struct {
	*fakeRequestClient
	revisions []*requestv1.PlanRevision
	next      string
	err       error
}

func (c *planRevisionsClient) ListPlanRevisions(ctx context.Context, in *requestv1.ListPlanRevisionsRequest, _ ...grpc.CallOption) (*requestv1.ListPlanRevisionsResponse, error) {
	c.rec(ctx, "ListPlanRevisions", in)
	if c.err != nil {
		return nil, c.err
	}
	return &requestv1.ListPlanRevisionsResponse{Revisions: c.revisions, NextPageToken: c.next}, nil
}

func newPlanRevisionsClient(revs ...*requestv1.PlanRevision) *planRevisionsClient {
	return &planRevisionsClient{fakeRequestClient: &fakeRequestClient{}, revisions: revs}
}

func TestRequestPlanRevisions_ListsTheVersionsWithEveryFieldTheScreenReads(t *testing.T) {
	at := timestamppb.New(time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC))
	c := newPlanRevisionsClient(
		&requestv1.PlanRevision{Revision: 3, RunId: "run-3", Status: "current", Engine: "openspec", RevisionFeedback: "split it", ActorId: "u-1", GeneratedAt: at, Commit: "abc", FilesChanged: 2, FileCount: 5, TextKept: true,
			Stats: &requestv1.RunStats{DurationMs: 1200, Turns: 3}},
		&requestv1.PlanRevision{Revision: 2, RunId: "run-2", Status: "superseded", Engine: "openspec", GeneratedAt: at, FileCount: 4},
	)
	c.next = "2"
	reg := NewRegistry()
	registerRequestChannels(reg, c, nil, nil, false)
	res, err := callRequestChannel(t, reg, testRequestIdentity, context.Background(), "request.planRevisions", `{"id":"r1","pageSize":2,"pageToken":"4"}`)
	if err != nil {
		t.Fatal(err)
	}
	in := c.last().in.(*requestv1.ListPlanRevisionsRequest)
	if in.GetRequestId() != "r1" || in.GetPageSize() != 2 || in.GetPageToken() != "4" {
		t.Errorf("rpc = %+v", in)
	}
	mustEqual(t, res["nextPageToken"], "2")
	revs := res["revisions"].([]any)
	if len(revs) != 2 {
		t.Fatalf("revisions = %v", revs)
	}
	first := revs[0].(map[string]any)
	mustEqual(t, first["revision"], float64(3))
	mustEqual(t, first["runId"], "run-3")
	mustEqual(t, first["status"], "current")
	mustEqual(t, first["engine"], "openspec")
	mustEqual(t, first["revisionFeedback"], "split it")
	mustEqual(t, first["actorId"], "u-1")
	mustEqual(t, first["generatedAt"], "2026-10-20T09:00:00Z")
	mustEqual(t, first["commit"], "abc")
	mustEqual(t, first["filesChanged"], float64(2))
	mustEqual(t, first["fileCount"], float64(5))
	mustEqual(t, first["textKept"], true)
	mustEqual(t, first["stats"].(map[string]any)["turns"], float64(3))
	// A version whose text was dropped must say so: an omitted `false` would read as "kept".
	second := revs[1].(map[string]any)
	v, present := second["textKept"]
	if !present || v != false {
		t.Errorf("textKept = %v (present %v), want an explicit false", v, present)
	}
}

func TestRequestPlanRevisions_AnEmptyHistoryIsAnEmptyListNotNull(t *testing.T) {
	reg := NewRegistry()
	registerRequestChannels(reg, newPlanRevisionsClient(), nil, nil, false)
	res, err := callRequestChannel(t, reg, testRequestIdentity, context.Background(), "request.planRevisions", `{"id":"r1"}`)
	if err != nil {
		t.Fatal(err)
	}
	revs, ok := res["revisions"].([]any)
	if !ok || len(revs) != 0 {
		t.Fatalf("revisions = %#v", res["revisions"])
	}
	mustEqual(t, res["nextPageToken"], "")
}

func TestRequestPlanRevisions_PageSizeIsBoundedAndErrorsKeepTheirCodes(t *testing.T) {
	c := newPlanRevisionsClient()
	reg := NewRegistry()
	registerRequestChannels(reg, c, nil, nil, false)
	if _, err := callRequestChannel(t, reg, testRequestIdentity, context.Background(), "request.planRevisions", `{"id":"r1","pageSize":5000}`); err != nil {
		t.Fatal(err)
	}
	if got := c.last().in.(*requestv1.ListPlanRevisionsRequest).GetPageSize(); got != requestMaxPageSize {
		t.Errorf("page size sent = %d", got)
	}
	for _, code := range []string{"REQUEST_PLAN_REVISION_PAGE_TOKEN_INVALID"} {
		c.err = status.Error(codes.InvalidArgument, code+": nope")
		_, err := callRequestChannel(t, reg, testRequestIdentity, context.Background(), "request.planRevisions", `{"id":"r1","pageToken":"x"}`)
		if err == nil || !strings.HasPrefix(err.Error(), code+": ") {
			t.Errorf("%s: err = %v", code, err)
		}
	}
}

func TestRequestPlanProposalAndBundleFile_ForwardTheRevision(t *testing.T) {
	c := &fakeRequestClient{}
	reg := NewRegistry()
	registerRequestChannels(reg, c, nil, nil, false)
	if _, err := callRequestChannel(t, reg, testRequestIdentity, context.Background(), "request.planProposal", `{"id":"r1","revision":2}`); err != nil {
		t.Fatal(err)
	}
	if in := c.last().in.(*requestv1.GetPlanProposalRequest); in.GetRevision() != 2 || in.GetRequestId() != "r1" {
		t.Errorf("proposal rpc = %+v", in)
	}
	if _, err := callRequestChannel(t, reg, testRequestIdentity, context.Background(), "request.planProposal", `{"id":"r1"}`); err != nil {
		t.Fatal(err)
	}
	if in := c.last().in.(*requestv1.GetPlanProposalRequest); in.GetRevision() != 0 {
		t.Errorf("absent revision must read the current version: %+v", in)
	}
	if _, err := callRequestChannel(t, reg, testRequestIdentity, context.Background(), "request.planBundleFile", `{"id":"r1","path":"tasks.md","revision":1}`); err != nil {
		t.Fatal(err)
	}
	if in := c.last().in.(*requestv1.GetPlanBundleFileRequest); in.GetRevision() != 1 || in.GetPath() != "tasks.md" {
		t.Errorf("file rpc = %+v", in)
	}
	before := c.count()
	for _, ch := range []string{"request.planProposal", "request.planBundleFile"} {
		_, err := callRequestChannel(t, reg, testRequestIdentity, context.Background(), ch, `{"id":"r1","path":"tasks.md","revision":-1}`)
		if err == nil || !strings.HasPrefix(err.Error(), "INVALID_ARGUMENT: ") {
			t.Errorf("%s with revision -1: %v", ch, err)
		}
	}
	if c.count() != before {
		t.Error("a bad revision must not reach the service")
	}
}

func TestRequestPlanBundleFile_PlanProposalViewCarriesHashAndChange(t *testing.T) {
	c := &bundleViewClient{fakeRequestClient: &fakeRequestClient{}}
	reg := NewRegistry()
	registerRequestChannels(reg, c, nil, nil, false)
	res, err := callRequestChannel(t, reg, testRequestIdentity, context.Background(), "request.planProposal", `{"id":"r1","revision":2}`)
	if err != nil {
		t.Fatal(err)
	}
	files := res["bundle"].(map[string]any)["files"].([]any)
	f := files[0].(map[string]any)
	mustEqual(t, f["sha256"], strings.Repeat("a", 64))
	mustEqual(t, f["change"], "changed")
}

type bundleViewClient struct{ *fakeRequestClient }

func (c *bundleViewClient) GetPlanProposal(ctx context.Context, in *requestv1.GetPlanProposalRequest, _ ...grpc.CallOption) (*requestv1.GetPlanProposalResponse, error) {
	c.rec(ctx, "GetPlanProposal", in)
	return &requestv1.GetPlanProposalResponse{RunId: "run-2", Status: "succeeded", Engine: "openspec",
		Bundle: &requestv1.PlanBundle{ChangeId: "req-1-x", Files: []*requestv1.PlanBundleFile{{Path: "tasks.md", Size: 10, Sha256: strings.Repeat("a", 64), Change: "changed"}}}}, nil
}

func TestRequestGeneratePlan_ForwardsTheIdempotencyKeyOnProposeOnly(t *testing.T) {
	c := &fakeRequestClient{}
	reg := NewRegistry()
	registerRequestChannels(reg, c, nil, nil, false)
	if _, err := callRequestChannel(t, reg, testRequestIdentity, context.Background(), "request.generatePlan", `{"id":"r1","feedback":"split","idempotencyKey":"click-1"}`); err != nil {
		t.Fatal(err)
	}
	if in := c.last().in.(*requestv1.GeneratePlanRequest); in.GetIdempotencyKey() != "click-1" || in.GetFeedback() != "split" {
		t.Errorf("rpc = %+v", in)
	}
	if _, err := callRequestChannel(t, reg, testRequestIdentity, context.Background(), "request.generatePlan", `{"id":"r1"}`); err != nil {
		t.Fatal(err)
	}
	if in := c.last().in.(*requestv1.GeneratePlanRequest); in.GetIdempotencyKey() != "" {
		t.Errorf("no key was given: %+v", in)
	}
}

func TestRequestGeneratePlan_RevisionErrorsReachTheScreenAsCodedMessages(t *testing.T) {
	for _, code := range []string{
		"REQUEST_PLAN_REVISION_IN_PROGRESS", "REQUEST_PLAN_REVISION_NOT_APPLICABLE", "REQUEST_PLAN_REVISION_NO_CHANGE",
		"REQUEST_PLAN_REVISION_NOT_FOUND", "REQUEST_PLAN_REVISION_TEXT_PRUNED", "REQUEST_PLAN_IDEMPOTENCY_KEY_INVALID", "REQUEST_PLAN_FORBIDDEN",
	} {
		st := codes.FailedPrecondition
		if strings.HasSuffix(code, "NOT_FOUND") {
			st = codes.NotFound
		}
		c := &fakeRequestClient{err: status.Error(st, code+": the message")}
		reg := NewRegistry()
		registerRequestChannels(reg, c, nil, nil, false)
		for _, call := range []struct{ channel, args string }{
			{"request.generatePlan", `{"id":"r1","feedback":"x"}`},
			{"request.planProposal", `{"id":"r1","revision":1}`},
			{"request.planBundleFile", `{"id":"r1","path":"tasks.md","revision":1}`},
		} {
			_, err := callRequestChannel(t, reg, testRequestIdentity, context.Background(), call.channel, call.args)
			if err == nil || err.Error() != code+": the message" {
				t.Errorf("%s via %s: %v", code, call.channel, err)
			}
		}
	}
}
