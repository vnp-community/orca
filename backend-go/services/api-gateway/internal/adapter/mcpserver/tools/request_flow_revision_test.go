package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// solution_generate passes the revision ask and the double-click key through; who may revise is request-service's decision.
func TestSolutionGenerate_ForwardsFeedbackAndKeyWithoutAGatewayRoleCheck(t *testing.T) {
	fake := &fakeRequestService{}
	ex := newRequestFlowExec(t, fake, DefaultConfig())
	res, err := ex.CallTool(context.Background(), alice, "solution_generate",
		json.RawMessage(`{"request_id":"r1","idempotency_key":"click-1","feedback":"keep the public API"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.generates) != 1 {
		t.Fatalf("generate calls = %d", len(fake.generates))
	}
	in := fake.generates[0]
	if in.GetRequestId() != "r1" || in.GetIdempotencyKey() != "click-1" || in.GetFeedback() != "keep the public API" {
		t.Errorf("request-service saw %+v", in)
	}
	sc := res.StructuredContent.(map[string]any)
	if sc["runId"] != "run2" || sc["solutionId"] != "s2" {
		t.Errorf("result = %v", sc)
	}

	over := strings.Repeat("é", 2001)
	if _, err := ex.CallTool(context.Background(), alice, "solution_generate", json.RawMessage(`{"request_id":"r1","feedback":"`+over+`"}`)); err == nil {
		t.Error("feedback over 2000 characters must be rejected")
	}
	if len(fake.generates) != 1 {
		t.Errorf("a rejected call reached the service: %d", len(fake.generates))
	}
}

func TestSolutionList_ShowsSeqAndRevisionFeedbackAndKeepsOptionKeys(t *testing.T) {
	fake := &fakeRequestService{solResp: &requestv1.ListSolutionsResponse{Solutions: []*requestv1.Solution{
		{Id: "s1", RequestId: "r1", Kind: requestv1.SolutionKind_SOLUTION_KIND_SOLUTION, Status: requestv1.SolutionStatus_SOLUTION_STATUS_SUPERSEDED,
			OptionsJson: `{"options":[{"id":"opt-0","risk_level":"low"}]}`, Seq: 1},
		{Id: "s2", RequestId: "r1", Kind: requestv1.SolutionKind_SOLUTION_KIND_SOLUTION, Status: requestv1.SolutionStatus_SOLUTION_STATUS_PROPOSED,
			OptionsJson: `{"options":[{"id":"opt-0","risk_level":"low"}]}`, Seq: 2, RevisionFeedback: "keep the public API"},
	}}}
	ex := newRequestFlowExec(t, fake, DefaultConfig())
	res, err := ex.CallTool(context.Background(), alice, "solution_list", json.RawMessage(`{"request_id":"r1"}`))
	if err != nil {
		t.Fatal(err)
	}
	sols := res.StructuredContent.(map[string]any)["items"].([]any)
	first, second := sols[0].(map[string]any), sols[1].(map[string]any)
	if fmt.Sprint(first["seq"]) != "1" || fmt.Sprint(second["seq"]) != "2" || second["revisionFeedback"] != "keep the public API" {
		t.Errorf("versions = %v", sols)
	}
	if _, has := first["revisionFeedback"]; has {
		t.Errorf("the first attempt has no ask to show: %v", first)
	}
	opts := second["options"].(map[string]any)["options"].([]any)[0].(map[string]any)
	if opts["risk_level"] != "low" {
		t.Errorf("option keys must stay as written: %v", opts)
	}
	if res.StructuredContent.(map[string]any)["untrusted"] != true {
		t.Error("the feedback is third-party text; the result must stay marked untrusted")
	}
}

func TestSolutionTools_DescribeRevisionRules(t *testing.T) {
	want := map[string][]string{
		"solution_generate": {"approver", "superseded", "REQUEST_SOLUTION_WRONG_STATE"},
		"solution_list":     {"seq", "revisionFeedback"},
	}
	for _, s := range requestFlowSpecs() {
		for _, frag := range want[s.Name] {
			if !strings.Contains(s.Description, frag) {
				t.Errorf("%s description lacks %q", s.Name, frag)
			}
		}
	}
}
