package wscompat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

var testRequestIdentity = Identity{TenantID: "t1", UserID: "u1", Role: "user"}

func newRequestTestRegistry(c *fakeRequestClient) *Registry {
	r := NewRegistry()
	if c == nil {
		registerRequestChannels(r, nil, nil, nil, false)
		return r
	}
	registerRequestChannels(r, c, c, nil, false)
	return r
}

// callRequestChannel dispatches through the registry (so identity metadata and the
// outer deadline are attached as in production) and decodes the result to a map.
func callRequestChannel(t *testing.T, r *Registry, id Identity, ctx context.Context, channel, args string) (map[string]any, error) {
	t.Helper()
	var raw []json.RawMessage
	if args != "" {
		raw = []json.RawMessage{json.RawMessage(args)}
	}
	res, err := r.Dispatch(ctx, id, channel, raw)
	if err != nil {
		return nil, err
	}
	b, jerr := json.Marshal(res)
	if jerr != nil {
		t.Fatalf("%s: result is not JSON: %v", channel, jerr)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s: result is not an object: %s", channel, b)
	}
	return m, nil
}

type requestChannelCase struct {
	channel  string
	args     string
	rpc      string
	lo, hi   time.Duration // allowed remaining budget when the RPC starts
	identity Identity
	check    func(t *testing.T, res map[string]any, in proto.Message)
}

func requestChannelCases() []requestChannelCase {
	short := [2]time.Duration{7 * time.Second, 8 * time.Second}
	ai := [2]time.Duration{23 * time.Second, 24 * time.Second}
	mid := [2]time.Duration{14 * time.Second, 15 * time.Second}
	admin := Identity{TenantID: "t1", UserID: "u1", Role: "admin"}
	return []requestChannelCase{
		{channel: "request.flowStatus", args: `{}`, rpc: "GetRequestFlowSettings", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) { mustEqual(t, res["enabled"], true) }},
		{channel: "request.flowSet", args: `{"enabled":false}`, rpc: "SetRequestFlowSettings", lo: short[0], hi: short[1], identity: admin,
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				mustEqual(t, res["enabled"], false)
				mustEqual(t, in.(*requestv1.SetRequestFlowSettingsRequest).GetEnabled(), false)
			}},
		{channel: "request.create", args: `{"projectId":"p1","title":"T","body":"B","clientRequestId":"c1","hints":{"issueType":"Bug","labels":["a"],"priority":"High"}}`,
			rpc: "CreateRequest", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				m := in.(*requestv1.CreateRequestRequest)
				mustEqual(t, m.GetSource().GetProvider(), "manual")
				mustEqual(t, m.GetClientRequestId(), "c1")
				mustEqual(t, m.GetHints().GetIssueType(), "Bug")
				mustEqual(t, res["created"], true)
				if _, has := res["request"].(map[string]any)["body"]; has {
					t.Error("request.create must not return body")
				}
			}},
		{channel: "request.get", args: `{"id":"r1"}`, rpc: "GetRequest", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) {
				rq := res["request"].(map[string]any)
				mustEqual(t, rq["body"], "B")
				mustEqual(t, rq["projectId"], "p1")
				mustEqual(t, rq["createdAt"], "2026-10-08T01:02:03Z")
				mustEqual(t, rq["confidence"], 0.9)
			}},
		{channel: "request.list", args: `{"projectId":"p1","status":["new"],"type":["bug"],"sourceProvider":"jira","pageSize":500,"pageToken":"tk"}`,
			rpc: "ListRequests", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				m := in.(*requestv1.ListRequestsRequest)
				mustEqual(t, m.GetPageSize(), int32(100))
				mustEqual(t, m.GetPageToken(), "tk")
				mustEqual(t, m.GetSourceProvider(), "jira")
				mustEqual(t, res["nextPageToken"], "np")
				if _, has := res["requests"].([]any)[0].(map[string]any)["body"]; has {
					t.Error("request.list must not return body")
				}
			}},
		{channel: "request.typeHistory", args: `{"id":"r1"}`, rpc: "ListRequestTypeHistory", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) {
				e := res["changes"].([]any)[0].(map[string]any)
				mustEqual(t, e["toType"], "bug")
				if e["fromType"] != nil {
					t.Errorf("fromType = %v, want null", e["fromType"])
				}
			}},
		{channel: "request.classify", args: `{"id":"r1"}`, rpc: "ClassifyRequest", lo: ai[0], hi: ai[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) { mustEqual(t, res["runId"], "run1") }},
		{channel: "request.confirmType", args: `{"id":"r1","type":"bug","size":"S","urgency":"urgent","reason":"x","expectedVersion":3}`, rpc: "ConfirmRequestType", lo: short[0], hi: short[1],
			check: func(t *testing.T, _ map[string]any, in proto.Message) {
				m := in.(*requestv1.ConfirmRequestTypeRequest)
				mustEqual(t, m.GetRequestId(), "r1")
				mustEqual(t, m.GetExpectedVersion(), int64(3))
				mustEqual(t, m.GetUrgency(), "urgent")
			}},
		{channel: "request.changeType", args: `{"id":"r1","toType":"hotfix","reason":"why","expectedVersion":4}`, rpc: "ChangeRequestType", lo: short[0], hi: short[1],
			check: func(t *testing.T, _ map[string]any, in proto.Message) {
				m := in.(*requestv1.ChangeRequestTypeRequest)
				mustEqual(t, m.GetNewType(), "hotfix")
				mustEqual(t, m.GetExpectedVersion(), int64(4))
			}},
		{channel: "request.returnToBacklog", args: `{"id":"r1","stage":"plan","category":"c","reason":"r","expectedVersion":5}`, rpc: "ReturnToBacklog", lo: short[0], hi: short[1],
			check: func(t *testing.T, _ map[string]any, in proto.Message) {
				m := in.(*requestv1.ReturnToBacklogRequest)
				mustEqual(t, m.GetStage(), "plan")
				mustEqual(t, m.GetCategory(), "c")
			}},
		{channel: "request.reopen", args: `{"id":"r1","note":"n","expectedVersion":6}`, rpc: "ReopenRequest", lo: short[0], hi: short[1],
			check: func(t *testing.T, _ map[string]any, in proto.Message) {
				mustEqual(t, in.(*requestv1.ReopenRequestRequest).GetNote(), "n")
			}},
		{channel: "request.cancel", args: `{"id":"r1","reason":"dup","expectedVersion":6}`, rpc: "CancelRequest", lo: short[0], hi: short[1],
			check: func(t *testing.T, _ map[string]any, in proto.Message) {
				mustEqual(t, in.(*requestv1.CancelRequestRequest).GetReason(), "dup")
			}},
		{channel: "request.spawnChild", args: `{"id":"r1","linkReason":"escalation","title":"c","typeHint":"bug","clientRequestId":"cc"}`, rpc: "SpawnChildRequest", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				m := in.(*requestv1.SpawnChildRequestRequest)
				mustEqual(t, m.GetParentRequestId(), "r1")
				mustEqual(t, m.GetLinkReason(), "escalation")
				mustEqual(t, m.GetTypeHint(), "bug")
				mustEqual(t, res["created"], true)
			}},
		{channel: "request.generatePlan", args: `{"id":"r1"}`, rpc: "GeneratePlan", lo: ai[0], hi: ai[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) {
				mustEqual(t, res["runId"], "plan-run")
				if res["proposal"] != nil {
					t.Errorf("proposal = %v, want null while the run is going", res["proposal"])
				}
			}},
		{channel: "request.generatePlan", args: `{"id":"r1","mode":"commit","proposal":{"title":"P","phases":[{"title":"ph","tasks":[{"title":"t","taskType":"task"}]}]},"rawAiResponse":"raw"}`,
			rpc: "CommitPlan", lo: mid[0], hi: mid[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				m := in.(*requestv1.CommitPlanRequest)
				mustEqual(t, m.GetProposal().GetPhases()[0].GetTasks()[0].GetTaskType(), "task")
				mustEqual(t, m.GetRawAiResponse(), "raw")
				mustEqual(t, res["planTaskId"], "pt")
				if res["taskIds"] == nil {
					t.Error("taskIds must be [] not null")
				}
			}},
		{channel: "request.planProposal", args: `{"id":"r1","runId":"plan-run"}`, rpc: "GetPlanProposal", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) {
				mustEqual(t, res["status"], "succeeded")
				if res["proposal"].(map[string]any)["title"] != "P" {
					t.Errorf("proposal = %v", res["proposal"])
				}
			}},
		{channel: "request.startPhase", args: `{"id":"r1","phaseTaskId":"ph1"}`, rpc: "StartPhase", lo: mid[0], hi: mid[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) {
				if res["dispatchedTaskIds"] == nil {
					t.Error("dispatchedTaskIds must be [] not null")
				}
			}},
		{channel: "solution.list", args: `{"requestId":"r1","kind":"solution","status":"proposed"}`, rpc: "ListSolutions", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				m := in.(*requestv1.ListSolutionsRequest)
				mustEqual(t, m.GetKind(), requestv1.SolutionKind_SOLUTION_KIND_SOLUTION)
				mustEqual(t, m.GetPageSize(), int32(20))
				s := res["solutions"].([]any)[0].(map[string]any)
				mustEqual(t, s["chosenOptionId"], "opt-1")
				mustEqual(t, s["kind"], "solution")
				mustEqual(t, s["status"], "proposed")
				if s["options"].(map[string]any)["options"].([]any)[1].(map[string]any)["risk_level"] != "low" {
					t.Errorf("options keys must stay as the AI wrote them: %v", s["options"])
				}
				mustEqual(t, res["runs"].([]any)[0].(map[string]any)["errorCode"], "REQUEST_SOLUTION_INVALID_OUTPUT")
			}},
		{channel: "solution.generate", args: `{"requestId":"r1","idempotencyKey":"k","feedback":"f","analysisMode":"agent_readonly"}`, rpc: "GenerateSolution", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				mustEqual(t, in.(*requestv1.GenerateSolutionRequest).GetAnalysisMode(), requestv1.AnalysisMode_ANALYSIS_MODE_AGENT_READONLY)
				mustEqual(t, res["runId"], "run1")
				mustEqual(t, res["solutionId"], "s1")
			}},
		{channel: "solution.choose", args: `{"requestId":"r1","solutionId":"s1","optionId":"opt-0","comment":"c"}`, rpc: "ChooseSolutionOption", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				mustEqual(t, in.(*requestv1.ChooseSolutionOptionRequest).GetOptionId(), "opt-0")
				mustEqual(t, res["approvalDigest"], "newdigest")
			}},
		{channel: "approval.get", args: `{"id":"a1"}`, rpc: "GetApproval", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				mustEqual(t, in.(*requestv1.GetApprovalRequest).GetId(), "a1")
				a := res["approval"].(map[string]any)
				mustEqual(t, a["subjectType"], "solution")
				mustEqual(t, a["status"], "pending")
				mustEqual(t, a["subjectDigest"], "dg")
			}},
		{channel: "approval.list", args: `{"requestId":"r1","subjectType":"plan","status":"approved"}`, rpc: "ListApprovals", lo: short[0], hi: short[1],
			check: func(t *testing.T, _ map[string]any, in proto.Message) {
				mustEqual(t, in.(*requestv1.ListApprovalsRequest).GetSubjectType(), requestv1.ApprovalSubjectType_APPROVAL_SUBJECT_TYPE_PLAN)
			}},
		{channel: "approval.listPending", args: `{"subjectType":"solution","pageSize":1}`, rpc: "ListPendingForUser", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				mustEqual(t, in.(*requestv1.ListPendingForUserRequest).GetPageSize(), int32(1))
				mustEqual(t, res["approvals"].([]any)[0].(map[string]any)["requestTitle"], "T")
				mustEqual(t, res["nextPageToken"], "ap")
			}},
		{channel: "approval.approve", args: `{"id":"a1","expectedVersion":2,"expectedDigest":"dg","comment":"ok"}`, rpc: "Approve", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				m := in.(*requestv1.ApproveRequest)
				mustEqual(t, m.GetId(), "a1")
				mustEqual(t, m.GetExpectedDigest(), "dg")
				mustEqual(t, m.GetExpectedVersion(), int64(2))
				mustEqual(t, res["requestStatus"], "planning")
			}},
		{channel: "approval.reject", args: `{"id":"a1","expectedVersion":2,"expectedDigest":"dg","comment":"no"}`, rpc: "Reject", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) {
				mustEqual(t, res["requestStatus"], "analyzing")
			}},
		{channel: "approval.cancel", args: `{"id":"a1","reason":"stale"}`, rpc: "Cancel", lo: short[0], hi: short[1],
			check: func(t *testing.T, _ map[string]any, in proto.Message) {
				mustEqual(t, in.(*requestv1.ApprovalServiceCancelRequest).GetReason(), "stale")
			}},
		{channel: "backlog.requests", args: `{"projectId":"p1","requestTypes":["bug"],"categories":["c"],"assigneeId":"zz","planTaskId":"zz"}`, rpc: "ListBacklog", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				m := in.(*requestv1.ListBacklogRequest)
				mustEqual(t, m.GetView(), requestv1.BacklogView_BACKLOG_VIEW_REQUEST)
				mustEqual(t, m.GetAssigneeId(), "")
				mustEqual(t, m.GetPlanTaskId(), "")
				mustEqual(t, res["requestRows"].([]any)[0].(map[string]any)["returnedFromStage"], "plan")
				mustEqual(t, res["nextPageToken"], "bn")
			}},
		{channel: "backlog.tasks", args: `{"projectId":"p1","requestId":"r1","planTaskId":"pl","assigneeId":"a","phaseTaskId":"zz","requestTypes":["bug"]}`, rpc: "ListBacklog", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				m := in.(*requestv1.ListBacklogRequest)
				mustEqual(t, m.GetView(), requestv1.BacklogView_BACKLOG_VIEW_TASK)
				mustEqual(t, m.GetPlanTaskId(), "pl")
				mustEqual(t, m.GetPhaseTaskId(), "")
				if len(m.GetRequestTypes()) != 0 {
					t.Error("requestTypes belongs to backlog.requests only")
				}
				g := res["groups"].([]any)[0].(map[string]any)
				mustEqual(t, g["planTaskId"], "pl")
				mustEqual(t, g["tasks"].([]any)[0].(map[string]any)["estimatedHours"], 1.5)
			}},
		{channel: "backlog.execute", args: `{"projectId":"p1","phaseTaskId":"ph","planTaskId":"zz"}`, rpc: "ListBacklog", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, in proto.Message) {
				m := in.(*requestv1.ListBacklogRequest)
				mustEqual(t, m.GetView(), requestv1.BacklogView_BACKLOG_VIEW_EXECUTE)
				mustEqual(t, m.GetPhaseTaskId(), "ph")
				mustEqual(t, m.GetPlanTaskId(), "")
				if len(res["groups"].([]any)) != 1 {
					t.Errorf("groups = %v", res["groups"])
				}
			}},
		{channel: "request.links", args: `{"id":"r1"}`, rpc: "ListRequestLinks", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) {
				mustEqual(t, len(res["parents"].([]any)), 0)
				mustEqual(t, res["children"].([]any)[0].(map[string]any)["childRequestId"], "c1")
			}},
		{channel: "request.flow", args: `{"type":"bug","size":"S"}`, rpc: "GetRequestFlow", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) {
				mustEqual(t, res["hasPhases"], true)
				mustEqual(t, len(res["executionGates"].([]any)), 0)
			}},
		{channel: "request.checks", args: `{"id":"r1"}`, rpc: "ListRequestChecks", lo: short[0], hi: short[1],
			check: func(t *testing.T, res map[string]any, _ proto.Message) {
				if res["checks"].([]any)[0].(map[string]any)["metrics"].(map[string]any)["p95_ms"] != float64(12) {
					t.Errorf("checks = %v", res["checks"])
				}
			}},
	}
}

func mustEqual(t *testing.T, got, want any) {
	t.Helper()
	if got != want {
		t.Errorf("got %v (%T), want %v (%T)", got, got, want, want)
	}
}

func TestRequestChannels_MapParametersDeadlinesAndIdentity(t *testing.T) {
	for _, tc := range requestChannelCases() {
		t.Run(tc.channel+"/"+tc.rpc, func(t *testing.T) {
			fake := &fakeRequestClient{}
			r := newRequestTestRegistry(fake)
			id := tc.identity
			if id.TenantID == "" {
				id = testRequestIdentity
			}
			// Spoofed identity members must never reach the RPC (CONTRACT C2).
			args := strings.Replace(tc.args, "{", `{"tenantId":"EVIL","userId":"EVIL","reporterId":"EVIL",`, 1)
			if tc.args == `{}` {
				args = `{"tenantId":"EVIL"}`
			}
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
				t.Errorf("tenant metadata = %v, want the session tenant t1", got)
			}
			if b, _ := protojson.Marshal(call.in); strings.Contains(string(b), "EVIL") {
				t.Errorf("a spoofed identity field reached the RPC: %s", b)
			}
			tc.check(t, res, call.in)
		})
	}
}

func TestRequestChannels_ResultsAreCamelCase(t *testing.T) {
	skipSnake := map[string]bool{"options": true, "metrics": true}
	var walk func(t *testing.T, channel, path string, v any)
	walk = func(t *testing.T, channel, path string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				if strings.Contains(k, "_") {
					t.Errorf("%s: key %q at %s is not camelCase", channel, k, path)
				}
				if !skipSnake[k] {
					walk(t, channel, path+"."+k, child)
				}
			}
		case []any:
			for _, child := range x {
				walk(t, channel, path+"[]", child)
			}
		}
	}
	for _, tc := range requestChannelCases() {
		fake := &fakeRequestClient{}
		id := tc.identity
		if id.TenantID == "" {
			id = testRequestIdentity
		}
		res, err := callRequestChannel(t, newRequestTestRegistry(fake), id, context.Background(), tc.channel, tc.args)
		if err != nil {
			t.Fatalf("%s: %v", tc.channel, err)
		}
		walk(t, tc.channel, "$", res)
	}
}

func TestRequestChannels_NilClientsAnswerUnavailable(t *testing.T) {
	r := newRequestTestRegistry(nil)
	for _, tc := range requestChannelCases() {
		_, err := callRequestChannel(t, r, Identity{TenantID: "t1", UserID: "u1", Role: "admin"}, context.Background(), tc.channel, tc.args)
		if err == nil || !strings.HasPrefix(err.Error(), "REQUEST_UNAVAILABLE:") {
			t.Errorf("%s: err = %v, want REQUEST_UNAVAILABLE", tc.channel, err)
		}
	}
}

func TestRequestChannels_Inventory(t *testing.T) {
	r := NewRegistry()
	registerRequestChannels(r, nil, nil, &fakeEphemeralSubscriber{}, true)
	want := []string{
		// 28 channels of CONTRACT section 2.1 to 2.4 (request.subscribe is the stream).
		"request.flowStatus", "request.flowSet", "request.create", "request.get", "request.list", "request.typeHistory", "request.classify",
		"request.confirmType", "request.changeType", "request.returnToBacklog", "request.reopen", "request.cancel", "request.spawnChild",
		"request.generatePlan", "request.startPhase", "request.subscribe",
		"solution.list", "solution.generate", "solution.choose",
		"approval.get", "approval.list", "approval.listPending", "approval.approve", "approval.reject", "approval.cancel",
		"backlog.requests", "backlog.tasks", "backlog.execute",
		// CONTRACT 2.5 and the plan proposal reader.
		"request.links", "request.flow", "request.checks", "request.planProposal",
	}
	got := map[string]ChannelKind{}
	for _, ch := range r.Channels() {
		got[ch.Name] = ch.Kind
	}
	if len(got) != len(want) {
		t.Errorf("registered %d channels, want %d: %v", len(got), len(want), got)
	}
	for _, name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("channel %s is not registered", name)
		}
	}
	if got["request.subscribe"] != ChannelStream {
		t.Errorf("request.subscribe kind = %v", got["request.subscribe"])
	}
	if _, bad := got["approval.request"]; bad {
		t.Error("approval.request must not exist: approvals come from the state machine")
	}
	// Without NATS the stream channel is absent and the rest stay.
	r2 := NewRegistry()
	registerRequestChannels(r2, nil, nil, nil, false)
	for _, ch := range r2.Channels() {
		if ch.Name == "request.subscribe" {
			t.Error("request.subscribe registered without NATS")
		}
	}
}

func TestRegisterProductionChannels_WiresRequestChannels(t *testing.T) {
	// Empty deps (CI inventory, REQUEST_SERVICE_ADDR unset): registered, answering REQUEST_UNAVAILABLE.
	r := NewRegistry()
	RegisterProductionChannels(r, ChannelDeps{})
	_, err := callRequestChannel(t, r, testRequestIdentity, context.Background(), "request.get", `{"id":"r1"}`)
	if err == nil || !strings.HasPrefix(err.Error(), "REQUEST_UNAVAILABLE:") {
		t.Fatalf("request.get with no client: %v", err)
	}
	if _, ok := r.StreamHandlerFor("request.subscribe"); ok {
		t.Error("request.subscribe must need NATS (TaskActivityEnabled)")
	}

	// A real client reaches the RPC through the production wiring.
	fake := &fakeRequestClient{}
	r = NewRegistry()
	RegisterProductionChannels(r, ChannelDeps{Request: fake, Approval: fake, TaskActivityEnabled: true})
	if _, err := callRequestChannel(t, r, testRequestIdentity, context.Background(), "approval.get", `{"id":"a1"}`); err != nil {
		t.Fatal(err)
	}
	if fake.last().rpc != "GetApproval" {
		t.Errorf("rpc = %q", fake.last().rpc)
	}
	if _, ok := r.StreamHandlerFor("request.subscribe"); !ok {
		t.Error("request.subscribe missing when NATS is enabled")
	}
}
