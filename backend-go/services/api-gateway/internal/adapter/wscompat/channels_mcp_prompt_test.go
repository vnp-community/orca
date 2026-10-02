package wscompat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

type promptClient struct {
	mcpv1.McpServiceClient
	err       error
	upsertReq *mcpv1.UpsertPromptRequest
	deleteReq *mcpv1.DeletePromptRequest
	calls     int
}

func (f *promptClient) ListPrompts(context.Context, *mcpv1.ListPromptsRequest, ...grpc.CallOption) (*mcpv1.ListPromptsResponse, error) {
	f.calls++
	return &mcpv1.ListPromptsResponse{Prompts: []*mcpv1.CustomPrompt{{
		Id: "c1", Name: "team_standup", Description: "d", Version: 3, Template: "Hi {{team}}", UpdatedAt: timestamppb.New(time.Date(2026, 10, 2, 1, 2, 3, 0, time.UTC)),
		Arguments: []*mcpv1.PromptArgument{{Name: "team", Description: "Team", Required: true}},
	}}}, f.err
}

func (f *promptClient) UpsertPrompt(_ context.Context, in *mcpv1.UpsertPromptRequest, _ ...grpc.CallOption) (*mcpv1.CustomPrompt, error) {
	f.calls++
	f.upsertReq = in
	if f.err != nil {
		return nil, f.err
	}
	out := in.GetPrompt()
	return &mcpv1.CustomPrompt{Id: "c-new", Name: out.GetName(), Version: out.GetVersion() + 1, Template: out.GetTemplate(), Arguments: out.GetArguments()}, nil
}

func (f *promptClient) DeletePrompt(_ context.Context, in *mcpv1.DeletePromptRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.calls++
	f.deleteReq = in
	return &emptypb.Empty{}, f.err
}

func TestMcpPrompt_AdminOnlyDisabledAndSingleObjectArg(t *testing.T) {
	c := &promptClient{}
	for _, ch := range []string{"mcp.admin.prompt.list", "mcp.admin.prompt.upsert", "mcp.admin.prompt.delete"} {
		for _, role := range []string{"user", ""} {
			if _, err := callMcp(t, McpChannelDeps{Enabled: true, Client: c}, role, ch, map[string]any{}); err == nil || !strings.HasPrefix(err.Error(), "MCP_NOT_ADMIN: ") {
				t.Errorf("%s as %q: %v", ch, role, err)
			}
		}
		if _, err := callMcp(t, McpChannelDeps{Enabled: false, Client: c}, "admin", ch, map[string]any{}); err == nil || !strings.HasPrefix(err.Error(), "MCP_DISABLED: ") {
			t.Errorf("%s disabled: %v", ch, err)
		}
		if _, err := callMcp(t, McpChannelDeps{Enabled: true}, "admin", ch, map[string]any{"promptId": "p"}); err == nil || !strings.HasPrefix(err.Error(), "MCP_UNAVAILABLE: ") {
			t.Errorf("%s without client: %v", ch, err)
		}
	}
	if c.calls != 0 {
		t.Fatalf("rejected calls reached mcp-service: %d", c.calls)
	}
	// upsert / delete need their one object argument.
	d := McpChannelDeps{Enabled: true, Client: c}
	for _, ch := range []string{"mcp.admin.prompt.upsert", "mcp.admin.prompt.delete"} {
		if _, err := callMcp(t, d, "admin", ch, nil); err == nil || !strings.HasPrefix(err.Error(), "MCP_INVALID_ARGUMENT: ") {
			t.Errorf("%s without args: %v", ch, err)
		}
	}
}

func TestMcpPromptList_BuiltinsFirstThenCustom(t *testing.T) {
	d := McpChannelDeps{Enabled: true, Client: &promptClient{}}
	got, err := callMcp(t, d, "admin", "mcp.admin.prompt.list", nil)
	if err != nil {
		t.Fatal(err)
	}
	var list []McpPrompt
	if err := json.Unmarshal([]byte(got), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 6 {
		t.Fatalf("len = %d", len(list))
	}
	for i, p := range list[:5] {
		if !p.Builtin || p.ID != "builtin:"+p.Name || p.Version < 1 || p.Template == "" || p.UpdatedAt == "" || len(p.Arguments) == 0 {
			t.Errorf("builtin %d: %+v", i, p)
		}
	}
	c := list[5]
	if c.Builtin || c.ID != "c1" || c.Name != "team_standup" || c.Version != 3 || c.UpdatedAt != "2026-10-02T01:02:03Z" || len(c.Arguments) != 1 || !c.Arguments[0].Required {
		t.Fatalf("custom: %+v", c)
	}
	// CONTRACT camelCase keys.
	for _, k := range []string{`"id"`, `"name"`, `"description"`, `"version"`, `"updatedAt"`, `"arguments"`, `"template"`, `"builtin"`} {
		if !strings.Contains(got, k) {
			t.Errorf("missing key %s", k)
		}
	}
}

func TestMcpPromptUpsertDelete_BuiltinReadonlyAndErrors(t *testing.T) {
	c := &promptClient{}
	d := McpChannelDeps{Enabled: true, Client: c}
	for _, arg := range []any{
		map[string]any{"id": "builtin:plan_task", "name": "plan_task", "template": "x"},
		map[string]any{"builtin": true, "name": "other", "template": "x"},
	} {
		if _, err := callMcp(t, d, "admin", "mcp.admin.prompt.upsert", arg); err == nil || !strings.HasPrefix(err.Error(), "MCP_PROMPT_BUILTIN_READONLY: ") {
			t.Errorf("%v: %v", arg, err)
		}
	}
	if _, err := callMcp(t, d, "admin", "mcp.admin.prompt.delete", map[string]any{"promptId": "builtin:plan_task"}); err == nil || !strings.HasPrefix(err.Error(), "MCP_PROMPT_BUILTIN_READONLY: ") {
		t.Errorf("delete builtin: %v", err)
	}
	if c.calls != 0 {
		t.Fatal("built-in changes must never reach mcp-service")
	}

	// Upsert forwards the contract fields (not updatedAt/builtin) and returns the new McpPrompt.
	got, err := callMcp(t, d, "admin", "mcp.admin.prompt.upsert", map[string]any{
		"id": "c1", "name": "team_standup", "version": 3, "template": "Hi {{team}}", "updatedAt": "2020-01-01T00:00:00Z",
		"arguments": []map[string]any{{"name": "team", "description": "Team", "required": true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	r := c.upsertReq.GetPrompt()
	if r.GetId() != "c1" || r.GetVersion() != 3 || r.GetName() != "team_standup" || len(r.GetArguments()) != 1 || r.GetUpdatedAt() != nil {
		t.Fatalf("request: %+v", r)
	}
	if !strings.Contains(got, `"version":4`) || !strings.Contains(got, `"builtin":false`) {
		t.Fatalf("%s", got)
	}
	if out, err := callMcp(t, d, "admin", "mcp.admin.prompt.delete", map[string]any{"promptId": "c1"}); err != nil || out != `{"ok":true}` || c.deleteReq.GetPromptId() != "c1" {
		t.Fatalf("%s %v", out, err)
	}

	// mcp-service error codes pass through in the "<CODE>: <message>" format.
	for code, grpcCode := range map[string]codes.Code{
		"MCP_PROMPT_INVALID: template: uses undeclared variable {{x}}": codes.InvalidArgument,
		"MCP_PROMPT_NAME_CONFLICT: a prompt named a already exists":    codes.AlreadyExists,
		"MCP_PROMPT_VERSION_CONFLICT: prompt was changed (current=4)":  codes.AlreadyExists,
		"MCP_NOT_FOUND: not found":                                     codes.NotFound,
	} {
		c.err = status.Error(grpcCode, code)
		_, err := callMcp(t, d, "admin", "mcp.admin.prompt.upsert", map[string]any{"name": "abc", "template": "x"})
		if err == nil || err.Error() != code {
			t.Errorf("want %q, got %v", code, err)
		}
	}
	// Anything else is generic: no internals leak.
	c.err = status.Error(codes.Internal, "pq: password authentication failed for 10.0.0.9")
	if _, err := callMcp(t, d, "admin", "mcp.admin.prompt.delete", map[string]any{"promptId": "c1"}); err == nil || strings.Contains(err.Error(), "10.0.0.9") || !strings.HasPrefix(err.Error(), "MCP_INTERNAL: ") {
		t.Fatalf("%v", err)
	}
}
