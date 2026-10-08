package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

func TestRequestCreate_SourceComesFromMCPSessionNotInput(t *testing.T) {
	fake := &fakeRequestService{}
	ex := newRequestFlowExec(t, fake, DefaultConfig())

	res, err := ex.CallTool(context.Background(), alice, "request_create",
		json.RawMessage(`{"project_id":"p1","title":"Add retry","body":"b","client_request_id":"c-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.creates) != 1 {
		t.Fatalf("creates = %d", len(fake.creates))
	}
	in := fake.creates[0]
	if in.GetSource().GetProvider() != "mcp" || in.GetSource().GetSite() != "claude-code" || in.GetSource().GetRef() != "" {
		t.Errorf("source = %+v, want mcp / claude-code / no ref", in.GetSource())
	}
	if in.GetProjectId() != "p1" || in.GetClientRequestId() != "c-1" || in.GetTitle() != "Add retry" {
		t.Errorf("args not mapped: %+v", in)
	}
	sc := res.StructuredContent.(map[string]any)
	if sc["created"] != true {
		t.Errorf("result = %v", sc)
	}

	// No identity or source field exists in the tool input: the closed schema rejects them.
	for _, field := range []string{"source_provider", "tenant_id", "user_id", "reporter_id", "source"} {
		_, err := ex.CallTool(context.Background(), alice, "request_create",
			json.RawMessage(fmt.Sprintf(`{"project_id":"p1","title":"x","%s":"jira"}`, field)))
		if err == nil || !strings.HasPrefix(errText(err), "INVALID_ARGUMENTS") {
			t.Errorf("field %s must be rejected, got %v", field, err)
		}
	}
	if len(fake.creates) != 1 {
		t.Errorf("rejected calls must not reach the service, creates = %d", len(fake.creates))
	}
}

func TestRequestCreate_UnknownClientNameFallsBack(t *testing.T) {
	fake := &fakeRequestService{}
	ex := newRequestFlowExec(t, fake, DefaultConfig())
	ex.WithGuards(Guards{SessionID: func(context.Context) string { return "s" }, ClientName: func(context.Context) string { return "" }})
	if _, err := ex.CallTool(context.Background(), alice, "request_create", json.RawMessage(`{"project_id":"p1","title":"x"}`)); err != nil {
		t.Fatal(err)
	}
	if got := fake.creates[0].GetSource().GetSite(); got != "unknown-mcp-client" {
		t.Errorf("site = %q", got)
	}
}

func TestWithRequestOrigin_OnlyRequestNamespaces(t *testing.T) {
	ex := newRequestFlowExec(t, &fakeRequestService{}, DefaultConfig())
	base := context.Background()
	for _, spec := range ex.catalog.Specs() {
		got := ex.withRequestOrigin(base, alice, spec)
		marked := got != base
		if marked != isRequestFlowNamespace(spec.Namespace) {
			t.Errorf("%s: origin marked=%v, request namespace=%v", spec.Name, marked, isRequestFlowNamespace(spec.Namespace))
		}
	}
}

func TestRequestCreate_HourlyLimit(t *testing.T) {
	fake := &fakeRequestService{}
	ex := newRequestFlowExec(t, fake, DefaultConfig())
	now := time.Unix(1_700_000_000, 0)
	ex.now = func() time.Time { return now }
	ex.createLimit = newRequestCreateLimiter(20, ex.nowFn)
	call := func(p mcpserver.Principal, tool, in string) error {
		_, err := ex.CallTool(context.Background(), p, tool, json.RawMessage(in))
		return err
	}
	for i := 0; i < 20; i++ {
		if err := call(alice, "request_create", `{"project_id":"p1","title":"x"}`); err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
	}
	err := call(alice, "request_create", `{"project_id":"p1","title":"x"}`)
	if err == nil || !strings.HasPrefix(errText(err), "REQUEST_RATE_LIMITED") {
		t.Fatalf("21st create = %v, want REQUEST_RATE_LIMITED", err)
	}
	// request_spawnChild spends the same budget; reads do not.
	if err := call(alice, "request_spawnChild", `{"id":"r1","reason":"escalation","title":"c"}`); err == nil || !strings.HasPrefix(errText(err), "REQUEST_RATE_LIMITED") {
		t.Errorf("spawnChild must share the budget, got %v", err)
	}
	fake.getResp = &requestv1.Request{Id: "r1"}
	if err := call(alice, "request_get", `{"id":"r1"}`); err != nil {
		t.Errorf("request_get must not be limited: %v", err)
	}
	// Another user has an independent bucket.
	bob := alice
	bob.UserID = "bob"
	if err := call(bob, "request_create", `{"project_id":"p1","title":"x"}`); err != nil {
		t.Errorf("other user limited: %v", err)
	}
	if len(fake.creates) != 21 { // 20 by alice + 1 by bob
		t.Errorf("service saw %d creates, want 21", len(fake.creates))
	}
	// 3 minutes refill one token (20 per hour).
	now = now.Add(3 * time.Minute)
	if err := call(alice, "request_create", `{"project_id":"p1","title":"x"}`); err != nil {
		t.Errorf("after refill: %v", err)
	}
}

func TestRequestCreate_LimitCanBeDisabled(t *testing.T) {
	cfg, err := DefaultConfig().ApplyEnv(func(k string) string {
		if k == "MCP_REQUEST_CREATE_PER_HOUR" {
			return "0"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	ex := newRequestFlowExec(t, &fakeRequestService{}, cfg)
	for i := 0; i < 30; i++ {
		if _, err := ex.CallTool(context.Background(), alice, "request_create", json.RawMessage(`{"project_id":"p1","title":"x"}`)); err != nil {
			t.Fatalf("call %d with the limiter off: %v", i+1, err)
		}
	}
}

func TestRequestServiceErrorsKeepTheirCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"flow disabled", status.Error(codes.FailedPrecondition, "REQUEST_FLOW_DISABLED: request flow is off"), "REQUEST_FLOW_DISABLED"},
		{"pending limit", status.Error(codes.ResourceExhausted, "REQUEST_PENDING_LIMIT: too many unconfirmed"), "REQUEST_PENDING_LIMIT"},
		{"unimplemented", status.Error(codes.Unimplemented, "unimplemented"), "REQUEST_NOT_IMPLEMENTED"},
	}
	for _, c := range cases {
		fake := &fakeRequestService{createFn: func(*requestv1.CreateRequestRequest) (*requestv1.CreateRequestResponse, error) { return nil, c.err }}
		ex := newRequestFlowExec(t, fake, DefaultConfig())
		_, err := ex.CallTool(context.Background(), alice, "request_create", json.RawMessage(`{"project_id":"p1","title":"x"}`))
		if err == nil || !strings.HasPrefix(errText(err), c.want) {
			t.Errorf("%s: got %v, want prefix %s", c.name, err, c.want)
		}
	}
}

func TestRequestReadToolsAreUntrustedAndRedacted(t *testing.T) {
	fake := &fakeRequestService{getResp: &requestv1.Request{Id: "r1", Title: "t", Body: "use key ghp_abcdefghijklmnopqrstuvwxyz0123456789 now"}}
	ex := newRequestFlowExec(t, fake, DefaultConfig())
	var wrapped []string
	ex.WithGuards(Guards{
		WrapUntrusted: func(source, text string) string { wrapped = append(wrapped, source); return "[framed]" + text },
		SessionID:     func(context.Context) string { return "s" }, ClientName: func(context.Context) string { return "c" },
	})
	for _, tool := range []string{"request_get", "request_list", "request_typeHistory", "solution_list", "backlog_requests"} {
		fake.solResp = &requestv1.ListSolutionsResponse{}
		in := `{"id":"r1"}`
		switch tool {
		case "request_list", "backlog_requests":
			in = `{}`
		case "solution_list":
			in = `{"request_id":"r1"}`
		}
		if tool == "request_typeHistory" || tool == "backlog_requests" {
			// Not scripted on the fake: their RPCs would panic, so assert the flag on the spec instead.
			spec, _ := ex.catalog.Lookup(tool)
			if spec == nil || !spec.Untrusted {
				t.Errorf("%s must be flagged untrusted", tool)
			}
			continue
		}
		wrapped = nil
		res, err := ex.CallTool(context.Background(), alice, tool, json.RawMessage(in))
		if err != nil {
			t.Fatalf("%s: %v", tool, err)
		}
		if len(wrapped) != 1 || wrapped[0] != tool {
			t.Errorf("%s: output not framed as data (wrapped=%v)", tool, wrapped)
		}
		txt := res.Content[0].(*mcp.TextContent).Text
		if !strings.HasPrefix(txt, "[framed]") || strings.Contains(txt, "ghp_") {
			t.Errorf("%s: text not framed or leaks a key: %s", tool, txt)
		}
		if sc := res.StructuredContent.(map[string]any); sc["untrusted"] != true {
			t.Errorf("%s: untrusted flag missing: %v", tool, sc)
		}
	}
}

func TestRequestListToolWrapsItems(t *testing.T) {
	ex := newRequestFlowExec(t, &fakeRequestService{}, DefaultConfig())
	res, err := ex.CallTool(context.Background(), alice, "request_list", json.RawMessage(`{"project_id":"p1","status":["new"]}`))
	if err != nil {
		t.Fatal(err)
	}
	sc := res.StructuredContent.(map[string]any)
	items, ok := sc["items"].([]any)
	if !ok || len(items) != 1 || sc["nextPageToken"] != "n" {
		t.Fatalf("list envelope = %v", sc)
	}
	if _, stray := sc["requests"]; stray {
		t.Errorf("raw key left behind: %v", sc)
	}
}

func TestSolutionListToolKeepsOptionKeysSnakeCase(t *testing.T) {
	fake := &fakeRequestService{solResp: &requestv1.ListSolutionsResponse{Solutions: []*requestv1.Solution{{
		Id: "s1", RequestId: "r1", Kind: requestv1.SolutionKind_SOLUTION_KIND_SOLUTION, Status: requestv1.SolutionStatus_SOLUTION_STATUS_PROPOSED,
		OptionsJson: `{"options":[{"id":"opt-0","risk_level":"low"}],"open_questions":[]}`, ChosenOption: -1,
	}}}}
	ex := newRequestFlowExec(t, fake, DefaultConfig())
	res, err := ex.CallTool(context.Background(), alice, "solution_list", json.RawMessage(`{"request_id":"r1"}`))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(res.StructuredContent)
	if !strings.Contains(string(b), `"risk_level"`) || !strings.Contains(string(b), `"open_questions"`) {
		t.Errorf("AI-generated option keys were rewritten (CONTRACT C13): %s", b)
	}
}
