package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

type govClient struct {
	mcpv1.McpServiceClient
	err          error
	upsertReq    *mcpv1.UpsertToolPolicyRequest
	updateReq    *mcpv1.UpdateTenantSettingsRequest
	decideReq    *mcpv1.DecideApprovalRequest
	killReq      *mcpv1.SetKillSwitchRequest
	explainReq   *mcpv1.ExplainPolicyRequest
	listApproval *mcpv1.ListApprovalsRequest
	md           metadata.MD
	stream       *fakeEventStream
}

func (f *govClient) rec(ctx context.Context) { f.md, _ = metadata.FromOutgoingContext(ctx) }

func (f *govClient) GetTenantSettings(ctx context.Context, _ *emptypb.Empty, _ ...grpc.CallOption) (*mcpv1.TenantSettings, error) {
	f.rec(ctx)
	return &mcpv1.TenantSettings{Enabled: true, MaxTokenDays: 30, ApprovalTtlSeconds: 120,
		KillSwitch: &mcpv1.KillSwitch{Active: true, Reason: "incident", At: timestamppb.New(time.Date(2026, 10, 1, 1, 2, 3, 0, time.UTC))}}, f.err
}
func (f *govClient) UpdateTenantSettings(ctx context.Context, in *mcpv1.UpdateTenantSettingsRequest, _ ...grpc.CallOption) (*mcpv1.TenantSettings, error) {
	f.rec(ctx)
	f.updateReq = in
	return &mcpv1.TenantSettings{Enabled: true, MaxTokenDays: in.GetMaxTokenDays()}, f.err
}
func (f *govClient) ListToolPolicies(ctx context.Context, _ *emptypb.Empty, _ ...grpc.CallOption) (*mcpv1.ListToolPoliciesResponse, error) {
	f.rec(ctx)
	return &mcpv1.ListToolPoliciesResponse{Policies: []*mcpv1.ToolPolicy{{Id: "p1", Version: 2, Decision: "deny", UpdatedBy: "u9",
		UpdatedAt: timestamppb.New(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)), Match: &mcpv1.ToolPolicyMatch{Tool: "task_list", Roles: []string{"user"}}}}}, f.err
}
func (f *govClient) UpsertToolPolicy(ctx context.Context, in *mcpv1.UpsertToolPolicyRequest, _ ...grpc.CallOption) (*mcpv1.ToolPolicy, error) {
	f.rec(ctx)
	f.upsertReq = in
	if f.err != nil {
		return nil, f.err
	}
	return &mcpv1.ToolPolicy{Id: "p-new", Version: in.GetPolicy().GetVersion() + 1, Decision: in.GetPolicy().GetDecision(), Match: in.GetPolicy().GetMatch()}, nil
}
func (f *govClient) DeleteToolPolicy(ctx context.Context, _ *mcpv1.DeleteToolPolicyRequest, _ ...grpc.CallOption) (*mcpv1.DeleteToolPolicyResponse, error) {
	f.rec(ctx)
	return &mcpv1.DeleteToolPolicyResponse{}, f.err
}
func (f *govClient) ExplainPolicy(ctx context.Context, in *mcpv1.ExplainPolicyRequest, _ ...grpc.CallOption) (*mcpv1.PolicyDecision, error) {
	f.rec(ctx)
	f.explainReq = in
	return &mcpv1.PolicyDecision{Decision: "require_approval", Reasons: []string{"risk_default:exec=require_approval"}}, f.err
}
func (f *govClient) SetKillSwitch(ctx context.Context, in *mcpv1.SetKillSwitchRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.rec(ctx)
	f.killReq = in
	return &emptypb.Empty{}, f.err
}
func (f *govClient) ListKillSwitches(ctx context.Context, _ *emptypb.Empty, _ ...grpc.CallOption) (*mcpv1.ListKillSwitchesResponse, error) {
	f.rec(ctx)
	return &mcpv1.ListKillSwitchesResponse{Entries: []*mcpv1.KillSwitchEntry{{Scope: "client", TargetId: "c1", Active: true, Reason: "r", SetBy: "u9", At: timestamppb.New(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))}}}, f.err
}
func (f *govClient) ListApprovals(ctx context.Context, in *mcpv1.ListApprovalsRequest, _ ...grpc.CallOption) (*mcpv1.ListApprovalsResponse, error) {
	f.rec(ctx)
	f.listApproval = in
	return &mcpv1.ListApprovalsResponse{NextCursor: "next", Approvals: []*mcpv1.Approval{sampleApproval()}}, f.err
}
func (f *govClient) DecideApproval(ctx context.Context, in *mcpv1.DecideApprovalRequest, _ ...grpc.CallOption) (*mcpv1.Approval, error) {
	f.rec(ctx)
	f.decideReq = in
	if f.err != nil {
		return nil, f.err
	}
	a := sampleApproval()
	a.Status, a.DecidedVia = "approved", in.GetVia()
	return a, nil
}

func sampleApproval() *mcpv1.Approval {
	return &mcpv1.Approval{Id: "a1", CreatedAt: timestamppb.New(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)), ExpiresAt: timestamppb.New(time.Date(2026, 10, 1, 0, 10, 0, 0, time.UTC)),
		Status: "pending", ToolName: "terminal_send", ToolTitle: "Send input", Risk: "exec", ClientName: "Agent", SessionId: "s1",
		ArgsPreview: "{}", ArgsRedacted: true, ParamsHash: "sha256:abc", Reasons: []string{"x"}}
}

func grpcErr(code codes.Code, msg string) error { return status.Error(code, msg) }

func TestMcpGovernance_AdminOnlyAndDisabled(t *testing.T) {
	adminChannels := []string{"mcp.admin.settings.get", "mcp.admin.settings.set", "mcp.admin.policy.list", "mcp.admin.policy.upsert", "mcp.admin.policy.delete",
		"mcp.admin.policy.explain", "mcp.admin.killswitch.set", "mcp.admin.killswitch.list", "mcp.admin.audit.query"}
	c := &govClient{}
	for _, ch := range adminChannels {
		for _, role := range []string{"user", ""} {
			if _, err := callMcp(t, McpChannelDeps{Enabled: true, Client: c}, role, ch, map[string]any{}); err == nil || !strings.HasPrefix(err.Error(), "MCP_NOT_ADMIN: ") {
				t.Errorf("%s as %q: %v", ch, role, err)
			}
		}
		if _, err := callMcp(t, McpChannelDeps{Enabled: false, Client: c}, "admin", ch, map[string]any{}); err == nil || !strings.HasPrefix(err.Error(), "MCP_DISABLED: ") {
			t.Errorf("%s disabled: %v", ch, err)
		}
	}
	for _, ch := range []string{"mcp.approval.list", "mcp.approval.decide"} {
		if _, err := callMcp(t, McpChannelDeps{Enabled: false, Client: c}, "user", ch, map[string]any{}); err == nil || !strings.HasPrefix(err.Error(), "MCP_DISABLED: ") {
			t.Errorf("%s disabled: %v", ch, err)
		}
	}
}

func TestMcpSettings_GetSetAndRejectKillSwitchPatch(t *testing.T) {
	c := &govClient{}
	d := McpChannelDeps{Enabled: true, Client: c}
	got, err := callMcp(t, d, "admin", "mcp.admin.settings.get", nil)
	want := `{"enabled":true,"dcrEnabled":false,"maxTokenDays":30,"approvalTtlSeconds":120,"killSwitch":{"active":true,"reason":"incident","at":"2026-10-01T01:02:03Z"}}`
	if err != nil || got != want {
		t.Fatalf("got %s err %v\nwant %s", got, err, want)
	}
	if _, err := callMcp(t, d, "admin", "mcp.admin.settings.set", map[string]any{"maxTokenDays": 14, "enabled": false}); err != nil {
		t.Fatal(err)
	}
	if c.updateReq.GetMaxTokenDays() != 14 || c.updateReq.Enabled == nil || *c.updateReq.Enabled || c.updateReq.DcrEnabled != nil || c.updateReq.ApprovalTtlSeconds != nil {
		t.Fatalf("partial patch must only carry the given fields: %+v", c.updateReq)
	}
	for name, arg := range map[string]any{
		"killSwitch":  map[string]any{"enabled": true, "killSwitch": map[string]any{"active": false}},
		"unknown":     map[string]any{"role": "admin"},
		"empty patch": map[string]any{},
	} {
		if _, err := callMcp(t, d, "admin", "mcp.admin.settings.set", arg); err == nil || !strings.HasPrefix(err.Error(), "MCP_INVALID_ARGUMENT: ") {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestMcpPolicy_ListUpsertDeleteExplainAndErrorShaping(t *testing.T) {
	views := fakeToolLister{views: []McpToolView{
		{Name: "task_create", Channel: "task.create", Namespace: "task", Risk: "write_reversible", RequiredScope: "orca:write"},
		{Name: "terminal_send", Channel: "terminal.send", Namespace: "terminal", Risk: "exec", RequiredScope: "orca:exec"},
		{Name: "credentials_get", Channel: "credentials.get", Namespace: "credentials", Risk: "admin", HardDenied: true},
		{Name: "agent_start", Channel: "agent.start", Namespace: "agent", Risk: "exec", RequiredScope: "orca:exec", Annotations: McpToolAnnotations{OpenWorld: true}},
	}}
	c := &govClient{}
	d := McpChannelDeps{Enabled: true, Client: c, ToolCatalog: views}
	got, err := callMcp(t, d, "admin", "mcp.admin.policy.list", nil)
	if err != nil || got != `[{"id":"p1","version":2,"updatedAt":"2026-10-01T00:00:00Z","updatedBy":"u9","match":{"tool":"task_list","roles":["user"]},"decision":"deny"}]` {
		t.Fatalf("%s %v", got, err)
	}
	// Namespace-wide policy: the gateway hands mcp-service every tool it reaches.
	if _, err := callMcp(t, d, "admin", "mcp.admin.policy.upsert", map[string]any{"decision": "require_approval", "version": 0, "match": map[string]any{"namespace": "task"}}); err != nil {
		t.Fatal(err)
	}
	if len(c.upsertReq.TouchedTools) != 1 || c.upsertReq.TouchedTools[0].Name != "task_create" {
		t.Fatalf("touched: %+v", c.upsertReq.TouchedTools)
	}
	// Client-wide policy reaches the entire catalog, hard-denied entries included.
	_, _ = callMcp(t, d, "admin", "mcp.admin.policy.upsert", map[string]any{"decision": "allow", "match": map[string]any{"clientId": "c1"}})
	names := map[string]bool{}
	for _, tl := range c.upsertReq.TouchedTools {
		names[tl.Name] = true
	}
	if len(names) != 4 || !names["credentials_get"] {
		t.Fatalf("client-wide policy must touch hard-denied tools too: %v", names)
	}
	var spawn *mcpv1.ToolRef
	for _, tl := range c.upsertReq.TouchedTools {
		if tl.Name == "agent_start" {
			spawn = tl
		}
	}
	if spawn == nil || !spawn.SpawnsProcess || !spawn.OpenWorld {
		t.Fatalf("derived tool facts: %+v", spawn)
	}
	// Server-side rejection codes pass through verbatim.
	c.err = grpcErr(codes.FailedPrecondition, "MCP_POLICY_HARD_DENY: policy would loosen hard-denied tools: credentials_get")
	if _, err := callMcp(t, d, "admin", "mcp.admin.policy.upsert", map[string]any{"decision": "allow", "match": map[string]any{"tool": "credentials_get"}}); err == nil || !strings.HasPrefix(err.Error(), "MCP_POLICY_HARD_DENY: ") {
		t.Fatalf("%v", err)
	}
	c.err = grpcErr(codes.AlreadyExists, "MCP_POLICY_VERSION_CONFLICT: policy was changed by someone else; reload and retry (current=4)")
	if _, err := callMcp(t, d, "admin", "mcp.admin.policy.upsert", map[string]any{"id": "p1", "version": 3, "decision": "deny", "match": map[string]any{"tool": "x"}}); err == nil || !strings.HasPrefix(err.Error(), "MCP_POLICY_VERSION_CONFLICT: ") {
		t.Fatalf("%v", err)
	}
	c.err = errors.New("pq: connection refused to 10.0.0.5")
	if _, err := callMcp(t, d, "admin", "mcp.admin.policy.delete", map[string]any{"policyId": "p1"}); err == nil || strings.Contains(err.Error(), "10.0.0.5") || !strings.HasPrefix(err.Error(), "MCP_INTERNAL: ") {
		t.Fatalf("internal details must not leak: %v", err)
	}
	c.err = nil
	if got, err := callMcp(t, d, "admin", "mcp.admin.policy.delete", map[string]any{"policyId": "p1"}); err != nil || got != `{"ok":true}` {
		t.Fatalf("%s %v", got, err)
	}
	got, err = callMcp(t, d, "admin", "mcp.admin.policy.explain", map[string]any{"tool": "terminal_send", "userId": "u7", "clientId": "c1"})
	if err != nil || got != `{"decision":"require_approval","reasons":["risk_default:exec=require_approval"]}` || c.explainReq.ToolRef.Channel != "terminal.send" {
		t.Fatalf("%s %v", got, err)
	}
	if _, err := callMcp(t, d, "admin", "mcp.admin.policy.explain", map[string]any{"tool": "nope"}); err == nil || !strings.HasPrefix(err.Error(), "MCP_NOT_FOUND: ") {
		t.Fatalf("unknown tool: %v", err)
	}
	if _, err := callMcp(t, McpChannelDeps{Enabled: true, Client: c}, "admin", "mcp.admin.policy.explain", map[string]any{"tool": "x"}); err == nil || !strings.HasPrefix(err.Error(), "MCP_UNAVAILABLE: ") {
		t.Fatalf("no catalog: %v", err)
	}
}

func TestMcpKillSwitchChannels(t *testing.T) {
	c := &govClient{}
	d := McpChannelDeps{Enabled: true, Client: c}
	if _, err := callMcp(t, d, "admin", "mcp.admin.killswitch.set", map[string]any{"scope": "tenant", "reason": "incident"}); err == nil || !strings.HasPrefix(err.Error(), "MCP_INVALID_ARGUMENT: ") {
		t.Fatalf("active is required: %v", err)
	}
	if got, err := callMcp(t, d, "admin", "mcp.admin.killswitch.set", map[string]any{"scope": "client", "targetId": "c1", "active": false, "reason": "all clear"}); err != nil || got != `{"ok":true}` {
		t.Fatalf("%s %v", got, err)
	}
	if c.killReq.Scope != "client" || c.killReq.TargetId != "c1" || c.killReq.Active || c.killReq.Reason != "all clear" {
		t.Fatalf("%+v", c.killReq)
	}
	got, err := callMcp(t, d, "admin", "mcp.admin.killswitch.list", nil)
	if err != nil || got != `[{"scope":"client","targetId":"c1","active":true,"reason":"r","at":"2026-10-01T00:00:00Z","by":"u9"}]` {
		t.Fatalf("%s %v", got, err)
	}
	c.err = grpcErr(codes.PermissionDenied, "MCP_NOT_ADMIN: administrator role required")
	if _, err := callMcp(t, d, "admin", "mcp.admin.killswitch.set", map[string]any{"scope": "tenant", "active": true, "reason": "incident"}); err == nil || !strings.HasPrefix(err.Error(), "MCP_NOT_ADMIN: ") {
		t.Fatalf("%v", err)
	}
}

func TestMcpApprovalChannels(t *testing.T) {
	c := &govClient{}
	d := McpChannelDeps{Enabled: true, Client: c}
	got, err := callMcp(t, d, "user", "mcp.approval.list", map[string]any{"status": "pending", "limit": 10})
	if err != nil || !strings.Contains(got, `"nextCursor":"next"`) || !strings.Contains(got, `"argsPreview":{"text":"{}","redacted":true}`) || !strings.Contains(got, `"tool":{"name":"terminal_send","title":"Send input","risk":"exec"}`) {
		t.Fatalf("%s %v", got, err)
	}
	if c.listApproval.GetStatus() != "pending" || c.listApproval.GetLimit() != 10 {
		t.Fatalf("%+v", c.listApproval)
	}
	if _, err := callMcp(t, d, "user", "mcp.approval.list", map[string]any{"status": "weird"}); err == nil {
		t.Fatal("bad status")
	}
	// Browser session => web; identity (and thus ownership) comes from the session only.
	r := NewRegistry()
	RegisterMcpChannels(r, d)
	arg, _ := json.Marshal(map[string]any{"approvalId": "a1", "decision": "approve", "paramsHash": "sha256:abc", "note": "ok", "via": "elicitation", "userId": "someone-else"})
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1", Role: "user"}, "mcp.approval.decide", []json.RawMessage{arg}); err != nil {
		t.Fatal(err)
	}
	if c.decideReq.Via != "web" || c.decideReq.ParamsHash != "sha256:abc" || c.md.Get("x-orca-user-id")[0] != "u1" {
		t.Fatalf("req=%+v md=%v", c.decideReq, c.md)
	}
	// Paired mobile device => mobile.
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1", Role: "user", DeviceID: "dev-1"}, "mcp.approval.decide", []json.RawMessage{arg}); err != nil {
		t.Fatal(err)
	}
	if c.decideReq.Via != "mobile" {
		t.Fatalf("via must be derived from the session: %s", c.decideReq.Via)
	}
	// An identity without a user can never decide.
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "mcp.approval.decide", []json.RawMessage{arg}); err == nil || !strings.HasPrefix(err.Error(), "MCP_NOT_FOUND: ") {
		t.Fatalf("%v", err)
	}
	for code, want := range map[string]string{
		"MCP_APPROVAL_EXPIRED":         "MCP_APPROVAL_EXPIRED: ",
		"MCP_APPROVAL_ALREADY_DECIDED": "MCP_APPROVAL_ALREADY_DECIDED: ",
		"MCP_APPROVAL_HASH_MISMATCH":   "MCP_APPROVAL_HASH_MISMATCH: ",
		"MCP_NOT_FOUND":                "MCP_NOT_FOUND: ",
		"MCP_KILL_SWITCH_ACTIVE":       "MCP_KILL_SWITCH_ACTIVE: ",
	} {
		c.err = grpcErr(codes.FailedPrecondition, code+": detail")
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1", Role: "user"}, "mcp.approval.decide", []json.RawMessage{arg})
		if err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("%s: %v", code, err)
		}
	}
}

type fakeAudit struct {
	req  *authv1.QueryAuditLogRequest
	resp *authv1.QueryAuditLogResponse
	err  error
}

func (f *fakeAudit) QueryAuditLog(_ context.Context, in *authv1.QueryAuditLogRequest, _ ...grpc.CallOption) (*authv1.QueryAuditLogResponse, error) {
	f.req = in
	return f.resp, f.err
}

func TestMcpAuditQuery(t *testing.T) {
	good := &authv1.AuditEntry{Id: "e1", ActorId: "11111111-1111-1111-1111-111111111111", TargetId: "terminal_send", OccurredAt: timestamppb.New(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)),
		MetadataJson: `{"decision":"approved","risk":"exec","client_name":"Agent","mcp_session_id":"s1","args_summary":"{}","result":"ok","duration_ms":12,"approver":"u2"}`}
	denied := &authv1.AuditEntry{Id: "e2", ActorId: "11111111-1111-1111-1111-111111111111", TargetId: "x", MetadataJson: `{"decision":"deny","risk":"read"}`}
	bad := []*authv1.AuditEntry{{Id: "e3", ActorId: "u", TargetId: "x", MetadataJson: `{"decision":"weird","risk":"read"}`}, {Id: "e4", MetadataJson: `not json`}, {Id: "e5", ActorId: "u", MetadataJson: `{"decision":"allow","risk":"read"}`}}
	a := &fakeAudit{resp: &authv1.QueryAuditLogResponse{Entries: append([]*authv1.AuditEntry{good, denied}, bad...), NextPageToken: "tok"}}
	d := McpChannelDeps{Enabled: true, Client: &govClient{}, Audit: a}
	got, err := callMcp(t, d, "admin", "mcp.admin.audit.query", map[string]any{"from": "2026-09-01T00:00:00Z", "to": "2026-10-02T00:00:00Z", "userId": "11111111-1111-1111-1111-111111111111", "tool": "terminal_send", "decision": "approved", "cursor": "c0", "limit": 500})
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Entries    []map[string]any `json:"entries"`
		NextCursor string           `json:"nextCursor"`
	}
	_ = json.Unmarshal([]byte(got), &out)
	if len(out.Entries) != 2 || out.NextCursor != "tok" || out.Entries[0]["actorType"] != "agent" || out.Entries[0]["result"] != "ok" || out.Entries[0]["durationMs"] != float64(12) || out.Entries[1]["result"] != nil {
		t.Fatalf("rows with missing/unknown fields must be skipped, not crash: %s", got)
	}
	r := a.req
	if r.ActorType != "agent" || r.Action != "mcp.tool_call" || r.TargetId != "terminal_send" || r.ActorId != "11111111-1111-1111-1111-111111111111" || r.PageToken != "c0" ||
		r.PageSize != 200 || r.Order != authv1.AuditOrder_AUDIT_ORDER_TIME_DESC || len(r.MetadataFilters) != 1 || r.MetadataFilters[0].Key != "decision" || r.TenantId != "t1" {
		t.Fatalf("request: %+v", r)
	}
	for name, arg := range map[string]any{
		"bad from": map[string]any{"from": "yesterday"}, "bad to": map[string]any{"to": "x"}, "from after to": map[string]any{"from": "2026-10-02T00:00:00Z", "to": "2026-10-01T00:00:00Z"},
		"bad user": map[string]any{"userId": "not-a-uuid"}, "bad decision": map[string]any{"decision": "maybe"},
	} {
		if _, err := callMcp(t, d, "admin", "mcp.admin.audit.query", arg); err == nil || !strings.HasPrefix(err.Error(), "MCP_INVALID_ARGUMENT: ") {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := callMcp(t, McpChannelDeps{Enabled: true, Client: &govClient{}}, "admin", "mcp.admin.audit.query", nil); err == nil || !strings.HasPrefix(err.Error(), "MCP_UNAVAILABLE: ") {
		t.Fatalf("%v", err)
	}
}
