package wscompat

import (
	"context"
	"encoding/json"
	"testing"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

var mcpOrigin = ToolOrigin{ClientName: "Claude Code", MCPSessionID: "sess-1", UserID: "user-1"}

func TestTerminalCreate_Origin(t *testing.T) {
	var got *infrafleetv1.SessionOrigin
	fake := &fakeTerminalInfraFleetClient{
		spawnFunc: func(in *infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
			got = in.GetOrigin()
			return &infrafleetv1.SpawnTerminalSessionResponse{Session: &infrafleetv1.TerminalSession{PtyId: "pty-1", Origin: in.GetOrigin()}}, nil
		},
	}
	r := NewRegistry()
	registerTerminalChannels(r, fake)
	id := Identity{TenantID: "tenant-1", UserID: "user-1"}

	// UI call: no origin on the request and none in the JSON.
	ack, _, _, err := r.DispatchStreamChannel(newTerminalTestCtx(), id, "terminal.create", argsJSON(t, terminalCreateArgs{ConnectionID: "c"}))
	if err != nil || got != nil {
		t.Fatalf("UI create: err=%v origin=%v", err, got)
	}
	b, _ := json.Marshal(ack)
	if json.Valid(b) && contains(string(b), `"origin"`) {
		t.Fatalf("UI-created terminal must omit origin: %s", b)
	}

	// MCP call: origin from ctx only (args cannot carry it).
	ctx := WithToolOrigin(newTerminalTestCtx(), mcpOrigin)
	ack, _, _, err = r.DispatchStreamChannel(ctx, id, "terminal.create", argsJSON(t, terminalCreateArgs{ConnectionID: "c"}))
	if err != nil || got.GetType() != "mcp" || got.GetMcpSessionId() != "sess-1" || got.GetClientName() != "Claude Code" {
		t.Fatalf("MCP create: err=%v origin=%v", err, got)
	}
	b, _ = json.Marshal(ack)
	var out struct {
		Terminal struct {
			Origin *sessionOriginView `json:"origin"`
		} `json:"terminal"`
	}
	if err := json.Unmarshal(b, &out); err != nil || out.Terminal.Origin == nil || out.Terminal.Origin.McpSessionID != "sess-1" || out.Terminal.Origin.UserID != "user-1" {
		t.Fatalf("ack origin = %s", b)
	}
}

func TestTerminalList_IncludesOriginAdditively(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{
		listFunc: func(*infrafleetv1.ListTerminalSessionsRequest) (*infrafleetv1.ListTerminalSessionsResponse, error) {
			return &infrafleetv1.ListTerminalSessionsResponse{Sessions: []*infrafleetv1.TerminalSession{
				{PtyId: "a", Origin: &infrafleetv1.SessionOrigin{Type: "mcp", McpSessionId: "s"}}, {PtyId: "b"},
			}}, nil
		},
	}
	r := NewRegistry()
	registerTerminalChannels(r, fake)
	res, err := r.Dispatch(context.Background(), Identity{TenantID: "t"}, "terminal.list", nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(res)
	var arr []map[string]any
	_ = json.Unmarshal(b, &arr)
	if _, ok := arr[0]["origin"]; !ok {
		t.Errorf("mcp terminal lacks origin: %s", b)
	}
	if _, ok := arr[1]["origin"]; ok {
		t.Errorf("UI terminal must not carry origin: %s", b)
	}
	if arr[1]["ptyId"] != "b" {
		t.Errorf("existing shape changed: %s", b)
	}
}

func TestAgentStart_Origin(t *testing.T) {
	var got *infrafleetv1.SessionOrigin
	fake := &fakeAgentInfraFleetClient{startFunc: func(in *infrafleetv1.StartAgentSessionRequest) (*infrafleetv1.AgentSession, error) {
		got = in.GetOrigin()
		return &infrafleetv1.AgentSession{Id: "a1", PtyId: "pty-a", Origin: in.GetOrigin()}, nil
	}}
	r := NewRegistry()
	registerAgentChannels(r, fake, nil)
	id := Identity{TenantID: "t", UserID: "user-1"}
	ack, _, _, err := r.DispatchStreamChannel(newTerminalTestCtx(), id, "agent.start", argsJSON(t, agentStartArgs{WorktreeID: "w"}))
	if err != nil || got != nil || ack.(agentSessionView).Origin != nil {
		t.Fatalf("UI start: err=%v origin=%v ack=%+v", err, got, ack)
	}
	ack, _, _, err = r.DispatchStreamChannel(WithToolOrigin(newTerminalTestCtx(), mcpOrigin), id, "agent.start", argsJSON(t, agentStartArgs{WorktreeID: "w"}))
	if err != nil || got.GetMcpSessionId() != "sess-1" {
		t.Fatalf("MCP start: err=%v origin=%v", err, got)
	}
	if v := ack.(agentSessionView); v.Origin == nil || v.Origin.Type != "mcp" {
		t.Fatalf("ack origin = %+v", v.Origin)
	}
}

func TestToolStreamScope_ServesTerminalSendAndClosesStreams(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{spawnFunc: func(*infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
		return &infrafleetv1.SpawnTerminalSessionResponse{Session: &infrafleetv1.TerminalSession{PtyId: "pty-1"}}, nil
	}}
	r := NewRegistry()
	registerTerminalChannels(r, fake)
	scope := NewToolStreamScope()
	ctx := scope.Context(context.Background())
	id := Identity{TenantID: "t", UserID: "u"}
	_, events, _, err := r.DispatchStreamChannel(ctx, id, "terminal.create", argsJSON(t, terminalCreateArgs{ConnectionID: "c"}))
	if err != nil {
		t.Fatal(err)
	}
	if !scope.Attached("pty-1") {
		t.Fatal("stream not attached in the tool scope")
	}
	if _, err := r.Dispatch(ctx, id, "terminal.send", argsJSON(t, terminalSendArgs{PtyID: "pty-1", Data: "ls\n"})); err != nil {
		t.Fatalf("terminal.send through the scope: %v", err)
	}
	scope.Close()
	if scope.Attached("pty-1") {
		t.Fatal("Close must forget every stream")
	}
	// The fake stream ignores ctx cancellation (a real gRPC stream returns an
	// error on cancel); emulate that so the drain goroutine can end.
	fake.getLastStream().err <- context.Canceled
	for range events {
	}
}

func contains(s, sub string) bool { return len(sub) > 0 && indexOf(s, sub) >= 0 }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
