package wscompat

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var reviewAdmin = Identity{TenantID: "t1", UserID: "u1", Role: roleAdmin}

func reviewChannelCases() []requestChannelCase {
	short := [2]time.Duration{7 * time.Second, 8 * time.Second}
	return []requestChannelCase{
		{channel: "request.review.estimate", args: `{"id":"r1","scope":"all","sections":["flow"]}`, rpc: "EstimateOutcomeReview", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				m := in.(*requestv1.EstimateOutcomeReviewRequest)
				mustEqual(t, m.GetRequestId(), "r1")
				mustEqual(t, m.GetScope(), "all")
				mustEqual(t, m.GetSections()[0], "flow")
				mustEqual(t, res["contextRunnable"], true)
				if res["warnings"] == nil || res["contextGaps"] == nil {
					t.Error("lists must be [] not null")
				}
				reg := res["regeneration"].(map[string]any)
				mustEqual(t, reg["costKnown"], true)
				mustEqual(t, reg["confirmRequired"], true)
			}},
		{channel: "request.review.generate", args: `{"id":"r1","phase":"design"}`, rpc: "GenerateOutcomeReview", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				m := in.(*requestv1.GenerateOutcomeReviewRequest)
				if !m.GetRunContext() {
					t.Error("the context run starts unless the client says it wants the facts only")
				}
				mustEqual(t, m.GetPhase(), "design")
				rv := res["review"].(map[string]any)
				mustEqual(t, rv["model"].(map[string]any)["schemaVersion"], float64(1))
				mustEqual(t, res["run"].(map[string]any)["status"], "running")
				mustEqual(t, res["drawingsPending"], float64(2))
			}},
		{channel: "request.review.generate", args: `{"id":"r1","runContext":false,"allowCodeOnly":true,"force":true}`, rpc: "GenerateOutcomeReview", lo: short[0], hi: short[1],
			check: func(t *testing.T, _ map[string]any, in proto.Message) {
				m := in.(*requestv1.GenerateOutcomeReviewRequest)
				if m.GetRunContext() || !m.GetAllowCodeOnly() || !m.GetForce() {
					t.Errorf("flags: %+v", m)
				}
			}},
		{channel: "request.review.get", args: `{"id":"r1","reviewId":"rv1"}`, rpc: "GetOutcomeReview", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				mustEqual(t, in.(*requestv1.GetOutcomeReviewRequest).GetReviewId(), "rv1")
				rv := res["review"].(map[string]any)
				// A model that is not JSON never reaches the screen as a broken value.
				if m, ok := rv["model"].(map[string]any); !ok || len(m) != 0 {
					t.Errorf("model = %v", rv["model"])
				}
				mustEqual(t, rv["approvedBy"], "u1")
			}},
		{channel: "request.review.list", args: `{"id":"r1","pageSize":10}`, rpc: "ListOutcomeReviews", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				mustEqual(t, in.(*requestv1.ListOutcomeReviewsRequest).GetPageSize(), int32(10))
				mustEqual(t, len(res["reviews"].([]any)), 1)
				mustEqual(t, res["nextPageToken"], "rv1")
			}},
		{channel: "request.review.runs", args: `{"id":"r1"}`, rpc: "ListOutcomeReviewRuns", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) {
				run := res["runs"].([]any)[0].(map[string]any)
				mustEqual(t, run["kind"], "draw")
				mustEqual(t, run["costActual"], "0.1000 USD")
			}},
		{channel: "request.review.artifact", args: `{"id":"r1","artifactId":"a2"}`, rpc: "GetReviewArtifact", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) {
				mustEqual(t, res["content"], "<svg></svg>")
				mustEqual(t, res["mediaType"], "text/html")
				mustEqual(t, res["artifact"].(map[string]any)["viewBox"], "0 0 800 400")
			}},
		{channel: "request.review.artifact", args: `{"id":"r1","artifactId":"a2","ifDigest":"bd"}`, rpc: "GetReviewArtifact", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) {
				mustEqual(t, res["notModified"], true)
				if _, has := res["content"]; has {
					t.Error("no content when it has not changed")
				}
			}},
		{channel: "request.review.revisions", args: `{"id":"r1","section":"flow","variant":"design","name":"Bug status"}`, rpc: "ListReviewArtifactRevisions", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				m := in.(*requestv1.ListReviewArtifactRevisionsRequest)
				mustEqual(t, m.GetName(), "Bug status")
				mustEqual(t, len(res["revisions"].([]any)), 2)
			}},
		{channel: "request.review.regenerate", args: `{"id":"r1","scope":"artifact","sections":["flow"],"reason":"looks_wrong","note":"n","confirmCost":true,"clientRequestId":"again","expectedVersion":4}`, rpc: "RegenerateReviewArtifacts", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				m := in.(*requestv1.RegenerateReviewArtifactsRequest)
				mustEqual(t, m.GetScope(), "artifact")
				mustEqual(t, m.GetReason(), "looks_wrong")
				mustEqual(t, m.GetClientRequestId(), "again")
				mustEqual(t, m.GetExpectedVersion(), int64(4))
				if !m.GetConfirmCost() {
					t.Error("confirmCost was not passed on")
				}
				mustEqual(t, res["replayed"], true)
				mustEqual(t, res["drawRun"].(map[string]any)["runId"], "run2")
			}},
		{channel: "request.review.restore", args: `{"id":"r1","artifactId":"a1","note":"first was better"}`, rpc: "RestoreReviewArtifactRevision", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) {
				mustEqual(t, res["artifact"].(map[string]any)["state"], "active")
				mustEqual(t, res["diff"].(map[string]any)["addedNodes"].([]any)[0], "n")
				mustEqual(t, len(res["diff"].(map[string]any)["removedNodes"].([]any)), 0)
			}},
		{channel: "request.review.compare", args: `{"id":"r1","fromArtifactId":"a1","toArtifactId":"a2"}`, rpc: "CompareReviewArtifactRevisions", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				mustEqual(t, in.(*requestv1.CompareReviewArtifactRevisionsRequest).GetToArtifactId(), "a2")
				mustEqual(t, len(res["diff"].(map[string]any)["addedEdges"].([]any)), 0)
			}},
		{channel: "request.review.decide", args: `{"id":"r1","reviewId":"rv1","itemKey":"k1","decision":"accept_exception","note":"ADR-09","expectedVersion":5}`, rpc: "DecideOutcomeReviewItem", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				m := in.(*requestv1.DecideOutcomeReviewItemRequest)
				mustEqual(t, m.GetNote(), "ADR-09")
				mustEqual(t, m.GetExpectedVersion(), int64(5))
				mustEqual(t, res["reviewVersion"], float64(6))
				mustEqual(t, res["openDecisions"], float64(1))
			}},
		{channel: "request.review.approve", args: `{"id":"r1","reviewId":"rv1","note":"ok","expectedVersion":6}`, rpc: "ApproveOutcomeReview", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) {
				rv := res["review"].(map[string]any)
				mustEqual(t, rv["state"], "approved")
				mustEqual(t, rv["approvalNote"], "ok")
			}},
		{channel: "request.review.decisions", args: `{"id":"r1","reviewId":"rv1"}`, rpc: "ListOutcomeReviewDecisions", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) {
				mustEqual(t, res["decisions"].([]any)[0].(map[string]any)["orphaned"], true)
				mustEqual(t, res["openDecisions"], float64(2))
			}},
		{channel: "request.review.linkChild", args: `{"id":"r1","reviewId":"rv1","itemKeys":["a","b"],"childRequestId":"c1"}`, rpc: "LinkOutcomeReviewChild", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) { mustEqual(t, res["linked"], float64(2)) }},
		{channel: "request.review.profile.get", args: `{"projectId":"p1"}`, rpc: "GetReviewDrawingProfile", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) {
				mustEqual(t, res["projectLayer"].(map[string]any)["version"], float64(3))
				mustEqual(t, res["projectLayer"].(map[string]any)["spec"].(map[string]any)["schema"], "review-drawing-profile/1")
				mustEqual(t, res["sources"].(map[string]any)["style.detail"], "project")
				if _, has := res["tenantLayer"]; has {
					t.Error("a layer that is not there is left out")
				}
			}},
		{channel: "request.review.profile.save", args: `{"projectId":"p1","spec":{"schema":"review-drawing-profile/1","language":"vi"},"expectedVersion":3}`, rpc: "SaveReviewDrawingProfile", lo: short[0], hi: short[1], identity: reviewAdmin,
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				m := in.(*requestv1.SaveReviewDrawingProfileRequest)
				if !strings.Contains(m.GetSpecJson(), `"language":"vi"`) {
					t.Errorf("spec = %s", m.GetSpecJson())
				}
				mustEqual(t, res["layer"].(map[string]any)["version"], float64(4))
			}},
		{channel: "request.review.profile.reset", args: `{"projectId":"p1","expectedVersion":4}`, rpc: "ResetReviewDrawingProfile", lo: short[0], hi: short[1], identity: reviewAdmin,
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				mustEqual(t, in.(*requestv1.ResetReviewDrawingProfileRequest).GetExpectedVersion(), int64(4))
				mustEqual(t, res["reset"], true)
			}},
	}
}

func TestReviewChannels_MapParametersDeadlinesAndIdentity(t *testing.T) {
	for _, tc := range reviewChannelCases() {
		t.Run(tc.channel+"/"+tc.rpc, func(t *testing.T) {
			fake := &fakeRequestClient{}
			r := newRequestTestRegistry(fake)
			id := tc.identity
			if id.TenantID == "" {
				id = testRequestIdentity
			}
			args := strings.Replace(tc.args, "{", `{"tenantId":"EVIL","userId":"EVIL","decidedBy":"EVIL",`, 1)
			res, err := callRequestChannel(t, r, id, context.Background(), tc.channel, args)
			if err != nil {
				t.Fatalf("dispatch: %v", err)
			}
			call := fake.last()
			if call.rpc != tc.rpc {
				t.Fatalf("rpc = %q, want %q", call.rpc, tc.rpc)
			}
			if call.deadline < tc.lo || call.deadline > tc.hi {
				t.Errorf("deadline budget = %v, want within [%v, %v]", call.deadline, tc.lo, tc.hi)
			}
			if got := call.md.Get("x-orca-tenant-id"); len(got) == 0 || got[0] != "t1" {
				t.Errorf("tenant metadata = %v, want the session tenant", got)
			}
			if b, _ := protojson.Marshal(call.in); strings.Contains(string(b), "EVIL") {
				t.Errorf("a spoofed identity field reached the RPC: %s", b)
			}
			tc.check(t, res, call.in)
		})
	}
}

func TestReviewChannels_RefuseWhatIsMissingBeforeCallingTheService(t *testing.T) {
	cases := map[string]string{
		"request.review.estimate":     `{}`,
		"request.review.generate":     `{"phase":"design"}`,
		"request.review.get":          `{}`,
		"request.review.artifact":     `{"id":"r1"}`,
		"request.review.restore":      `{"id":"r1"}`,
		"request.review.compare":      `{"id":"r1","fromArtifactId":"a"}`,
		"request.review.decide":       `{"id":"r1","reviewId":"rv1"}`,
		"request.review.approve":      `{"id":"r1"}`,
		"request.review.decisions":    `{"id":"r1"}`,
		"request.review.linkChild":    `{"id":"r1","reviewId":"rv1","itemKeys":[]}`,
		"request.review.profile.save": `{"projectId":"p1"}`,
	}
	for channel, args := range cases {
		fake := &fakeRequestClient{}
		_, err := callRequestChannel(t, newRequestTestRegistry(fake), reviewAdmin, context.Background(), channel, args)
		if err == nil || fake.count() != 0 {
			t.Errorf("%s %s: err=%v calls=%d", channel, args, err, fake.count())
		}
	}
}

func TestReviewChannels_ProfileWritesAreForAdmins(t *testing.T) {
	for _, channel := range []string{"request.review.profile.save", "request.review.profile.reset"} {
		fake := &fakeRequestClient{}
		_, err := callRequestChannel(t, newRequestTestRegistry(fake), testRequestIdentity, context.Background(), channel, `{"projectId":"p1","spec":{"schema":"x"}}`)
		if err == nil || !strings.Contains(err.Error(), "REQUEST_FORBIDDEN") || fake.count() != 0 {
			t.Errorf("%s by a member: err=%v calls=%d", channel, err, fake.count())
		}
	}
}

func TestReviewChannels_ServiceErrorsKeepTheirCodeAndNothingElse(t *testing.T) {
	fake := &fakeRequestClient{err: status.Error(codes.FailedPrecondition, "REQUEST_REVIEW_DECISIONS_OPEN: 2 decision(s) still need an answer before the review can be approved")}
	_, err := callRequestChannel(t, newRequestTestRegistry(fake), testRequestIdentity, context.Background(), "request.review.approve", `{"id":"r1","reviewId":"rv1"}`)
	if err == nil || !strings.HasPrefix(err.Error(), "REQUEST_REVIEW_DECISIONS_OPEN: ") {
		t.Errorf("a service code is relayed as it is: %v", err)
	}
	fake = &fakeRequestClient{err: errors.New("dial tcp 10.1.2.3:9000: refused SECRET-BODY")}
	_, err = callRequestChannel(t, newRequestTestRegistry(fake), testRequestIdentity, context.Background(), "request.review.get", `{"id":"r1"}`)
	if err == nil || strings.Contains(err.Error(), "SECRET-BODY") || strings.Contains(err.Error(), "10.1.2.3") {
		t.Errorf("an unexpected failure must not leak: %v", err)
	}
}

func TestReviewChannels_GetOfAReviewThatIsNotThereIsNotAnError(t *testing.T) {
	fake := &fakeRequestClient{empty: true}
	res, err := callRequestChannel(t, newRequestTestRegistry(fake), testRequestIdentity, context.Background(), "request.review.get", `{"id":"r1"}`)
	if err != nil || res["found"] != false {
		t.Errorf("res=%v err=%v", res, err)
	}
	if _, has := res["review"]; has {
		t.Error("no review member when there is none")
	}
}
