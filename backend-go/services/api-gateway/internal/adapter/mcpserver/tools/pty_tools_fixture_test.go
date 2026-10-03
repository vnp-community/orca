package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

// fakeFleet is an in-memory infra-fleet: it spawns PTYs, serves AttachPty
// streams that end when their context is cancelled (like a real gRPC stream)
// and records what was killed or stopped. The real wscompat terminal/agent
// channel handlers run on top of it, so these tests exercise the same path as
// the UI (registry -> handler -> infra-fleet client).
type fakeFleet struct {
	infrafleetv1.InfraFleetServiceClient

	mu         sync.Mutex
	n          int
	ptys       map[string]*fakePty
	open       []*infrafleetv1.TerminalSession
	killed     []string
	killReason map[string]string // ptyID -> reason sent on KillTerminalSession
	stopped    []string          // terminal.stop + agent.stop ids
	agentStops []string
	agentKills []string
	spawnReqs  []*infrafleetv1.SpawnTerminalSessionRequest
	agentReqs  []*infrafleetv1.StartAgentSessionRequest
	agentReady bool
	exitOnStop bool // agent exits on StopAgentSession (polite stop works)
	listCalls  int
}

type fakePty struct {
	id     string
	out    chan *infrafleetv1.PtyServerFrame
	inputs [][]byte
}

func newFakeFleet() *fakeFleet { return &fakeFleet{ptys: map[string]*fakePty{}, agentReady: true} }

func (f *fakeFleet) newPty() *fakePty {
	f.n++
	p := &fakePty{id: fmt.Sprintf("pty-%d", f.n), out: make(chan *infrafleetv1.PtyServerFrame, 4096)}
	f.ptys[p.id] = p
	return p
}

func (f *fakeFleet) emit(ptyID string, data string) {
	f.mu.Lock()
	p := f.ptys[ptyID]
	f.mu.Unlock()
	p.out <- &infrafleetv1.PtyServerFrame{Frame: &infrafleetv1.PtyServerFrame_Out{Out: &infrafleetv1.PtyOutput{Data: []byte(data)}}}
}

func (f *fakeFleet) exit(ptyID string, code int32) {
	f.mu.Lock()
	p := f.ptys[ptyID]
	f.mu.Unlock()
	p.out <- &infrafleetv1.PtyServerFrame{Frame: &infrafleetv1.PtyServerFrame_Exited{Exited: &infrafleetv1.PtyExited{ExitCode: code}}}
}

func (f *fakeFleet) inputsOf(ptyID string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, b := range f.ptys[ptyID].inputs {
		out = append(out, string(b))
	}
	return out
}

func (f *fakeFleet) SpawnTerminalSession(_ context.Context, in *infrafleetv1.SpawnTerminalSessionRequest, _ ...grpc.CallOption) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.spawnReqs = append(f.spawnReqs, in)
	p := f.newPty()
	s := &infrafleetv1.TerminalSession{PtyId: p.id, ConnectionId: in.GetConnectionId(), Cwd: in.GetCwd(), Origin: in.GetOrigin()}
	f.open = append(f.open, s)
	return &infrafleetv1.SpawnTerminalSessionResponse{Session: s}, nil
}

func (f *fakeFleet) ListTerminalSessions(context.Context, *infrafleetv1.ListTerminalSessionsRequest, ...grpc.CallOption) (*infrafleetv1.ListTerminalSessionsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listCalls++
	return &infrafleetv1.ListTerminalSessionsResponse{Sessions: append([]*infrafleetv1.TerminalSession(nil), f.open...)}, nil
}

func (f *fakeFleet) KillTerminalSession(_ context.Context, in *infrafleetv1.KillTerminalSessionRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.mu.Lock()
	f.killed = append(f.killed, in.GetPtyId())
	if f.killReason == nil {
		f.killReason = map[string]string{}
	}
	f.killReason[in.GetPtyId()] = in.GetReason()
	for i, s := range f.open {
		if s.GetPtyId() == in.GetPtyId() {
			f.open = append(f.open[:i], f.open[i+1:]...)
			break
		}
	}
	f.mu.Unlock()
	return &emptypb.Empty{}, nil
}

func (f *fakeFleet) StopTerminalProcess(_ context.Context, in *infrafleetv1.StopTerminalProcessRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.mu.Lock()
	f.stopped = append(f.stopped, in.GetPtyId())
	f.mu.Unlock()
	return &emptypb.Empty{}, nil
}

func (f *fakeFleet) WaitTerminalSession(context.Context, *infrafleetv1.WaitTerminalSessionRequest, ...grpc.CallOption) (*infrafleetv1.WaitTerminalSessionResponse, error) {
	return &infrafleetv1.WaitTerminalSessionResponse{Exited: true, ExitCode: 7}, nil
}

func (f *fakeFleet) GetTerminalAgentStatus(context.Context, *infrafleetv1.GetTerminalAgentStatusRequest, ...grpc.CallOption) (*infrafleetv1.GetTerminalAgentStatusResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return &infrafleetv1.GetTerminalAgentStatusResponse{AgentRunning: true, AgentKind: "claude", ReadyForInput: f.agentReady}, nil
}

func (f *fakeFleet) StartAgentSession(_ context.Context, in *infrafleetv1.StartAgentSessionRequest, _ ...grpc.CallOption) (*infrafleetv1.AgentSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.agentReqs = append(f.agentReqs, in)
	p := f.newPty()
	return &infrafleetv1.AgentSession{Id: "agent-" + p.id, PtyId: p.id, Status: "spawning", UserId: in.GetUserId(), Origin: in.GetOrigin()}, nil
}

func (f *fakeFleet) ptyOfAgent(sessionID string) string { return sessionID[len("agent-"):] }

func (f *fakeFleet) StopAgentSession(_ context.Context, in *infrafleetv1.StopAgentSessionRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.mu.Lock()
	f.agentStops = append(f.agentStops, in.GetSessionId())
	exit := f.exitOnStop
	f.mu.Unlock()
	if exit {
		f.exit(f.ptyOfAgent(in.GetSessionId()), 0)
	}
	return &emptypb.Empty{}, nil
}

func (f *fakeFleet) KillAgentSession(_ context.Context, in *infrafleetv1.KillAgentSessionRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.mu.Lock()
	f.agentKills = append(f.agentKills, in.GetSessionId())
	f.mu.Unlock()
	f.exit(f.ptyOfAgent(in.GetSessionId()), 137)
	return &emptypb.Empty{}, nil
}

func (f *fakeFleet) AttachPty(ctx context.Context, _ ...grpc.CallOption) (grpc.BidiStreamingClient[infrafleetv1.PtyClientFrame, infrafleetv1.PtyServerFrame], error) {
	return &fakeAttach{ctx: ctx, f: f, attached: make(chan *fakePty, 1)}, nil
}

type fakeAttach struct {
	ctx      context.Context
	f        *fakeFleet
	attached chan *fakePty
	pty      *fakePty
}

func (s *fakeAttach) Send(fr *infrafleetv1.PtyClientFrame) error {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if a := fr.GetAttach(); a != nil {
		s.attached <- s.f.ptys[a.GetPtyId()]
	}
	if in := fr.GetInput(); in != nil && s.pty != nil {
		s.pty.inputs = append(s.pty.inputs, in.GetData())
	}
	return nil
}

func (s *fakeAttach) Recv() (*infrafleetv1.PtyServerFrame, error) {
	s.f.mu.Lock()
	p := s.pty
	s.f.mu.Unlock()
	if p == nil {
		select {
		case p = <-s.attached:
			s.f.mu.Lock()
			s.pty = p
			s.f.mu.Unlock()
		case <-s.ctx.Done():
			return nil, s.ctx.Err()
		}
	}
	select {
	case fr := <-p.out:
		return fr, nil
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

func (s *fakeAttach) Header() (metadata.MD, error) { return nil, nil }
func (s *fakeAttach) Trailer() metadata.MD         { return nil }
func (s *fakeAttach) CloseSend() error             { return nil }
func (s *fakeAttach) Context() context.Context     { return s.ctx }
func (s *fakeAttach) SendMsg(any) error            { return nil }
func (s *fakeAttach) RecvMsg(any) error            { return nil }

// ---- executor fixture ---------------------------------------------------------

type sessCtxKey struct{}

func withMCPSession(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessCtxKey{}, id)
}

type stubTargets struct{}

func (stubTargets) ResolveWorktreeTarget(_ context.Context, _ wscompat.Identity, wt string) (string, string, error) {
	if wt == "missing" {
		return "", "", fmt.Errorf("no such worktree")
	}
	return "conn-" + wt, "/repo/" + wt, nil
}

type ptyFixture struct {
	t     *testing.T
	fleet *fakeFleet
	ex    *Executor
	reg   *wscompat.Registry
	gate  mcpserver.PolicyGate
}

func newPtyFixture(t *testing.T, mutate func(*PtyToolsConfig), gate mcpserver.PolicyGate) *ptyFixture {
	t.Helper()
	fleet := newFakeFleet()
	reg := wscompat.NewRegistry()
	wscompat.RegisterProductionChannels(reg, wscompat.ChannelDeps{InfraFleet: fleet})
	cfg := DefaultConfig()
	cfg.Packs = allPacks
	if gate == nil {
		gate = &mcpservertest.FakeGate{}
	}
	cat, err := NewCatalog(AllSpecs(), cfg, gate, reg.Channels())
	if err != nil {
		t.Fatal(err)
	}
	pc := DefaultPtyToolsConfig()
	pc.Targets = stubTargets{}
	pc.AgentGrace = 40 * time.Millisecond
	pc.JanitorEvery = time.Hour
	if mutate != nil {
		mutate(&pc)
	}
	ex := NewExecutor(cat, reg, gate, nil, cfg, quiet).WithGuards(Guards{
		SessionID:  func(ctx context.Context) string { s, _ := ctx.Value(sessCtxKey{}).(string); return s },
		ClientName: func(context.Context) string { return "Test Client" },
	}).WithPtyTools(pc)
	t.Cleanup(ex.Close)
	return &ptyFixture{t: t, fleet: fleet, ex: ex, reg: reg, gate: gate}
}

func (f *ptyFixture) call(sess string, p mcpserver.Principal, tool, args string) (map[string]any, error) {
	f.t.Helper()
	res, err := f.ex.CallTool(withMCPSession(context.Background(), sess), p, tool, json.RawMessage(args))
	if err != nil {
		return nil, err
	}
	m, _ := res.StructuredContent.(map[string]any)
	if m == nil {
		f.t.Fatalf("%s: no structured content: %#v", tool, res)
	}
	return m, nil
}

func (f *ptyFixture) mustCall(sess string, tool, args string) map[string]any {
	f.t.Helper()
	m, err := f.call(sess, alice, tool, args)
	if err != nil {
		f.t.Fatalf("%s %s: %v", tool, args, err)
	}
	return m
}

func seqOf(t *testing.T, m map[string]any, key string) uint64 {
	t.Helper()
	n, ok := m[key].(json.Number)
	if !ok {
		t.Fatalf("%s missing or not a number in %v", key, m)
	}
	v, err := n.Int64()
	if err != nil {
		t.Fatal(err)
	}
	return uint64(v)
}

func eventually(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

var _ = mcp.CallToolResult{}
