package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

// Terminal, agent and workflow-run tools (BE-MCP-SOL-009). They are Composite
// tools: each runs over the registry channels the UI uses, on a per-MCP-session
// ToolSession that owns the attached PTY streams, so a PTY can only be driven by
// the MCP session that started it and is stopped when that session ends.
//
// All of them carry risk=exec: the PolicyGate asks approval per call and sees
// the verbatim arguments (the command text of terminal_send, the prompt of
// agent_send) because the executor hands it the validated input.

// ptySubmitKey is what "Enter" sends: CR works on cooked ttys (ICRNL turns it
// into LF), raw-mode TUIs such as agent CLIs and Windows ConPTY alike.
const ptySubmitKey = "\r"

func composite(name, ns string, pack int, risk Risk, desc string, uses []string, fields []Field, fn ComposeFunc) *ToolSpec {
	s := &ToolSpec{
		Name: name, NameReason: "composite tool (BE-MCP-SOL-009)", UsesChannels: uses, Kind: KindComposite, Namespace: ns,
		Pack: pack, Title: titleOf(strings.ReplaceAll(name, "_", ".")), Description: desc, Risk: risk, Fields: fields,
		Compose: fn, KeepKeys: true,
		ArgsOverride: func(in json.RawMessage, _ wscompat.Identity) ([]json.RawMessage, error) {
			return []json.RawMessage{in}, nil
		},
	}
	if risk == mcpserver.RiskRead {
		s.Annotations = Annotations{ReadOnly: true, Idempotent: true}
	}
	return s
}

func pack3TerminalAgent() []*ToolSpec {
	termID := Str("terminal_id", "", "Terminal id from terminal_start", Req)
	agentID := Str("session_id", "", "Agent session id from agent_start", Req)
	cursor := []Field{
		Int("since_seq", "", "Cursor: next_seq of the previous read; omit to read from the oldest kept output", Range(0, 1<<53)),
		Int("max_bytes", "", "Maximum raw bytes to return (default 16384, at most 65536)", Range(1, 65536)),
		Int("wait_ms", "", "Wait up to this long for new output (at most 10000)", Range(0, 10000)),
	}
	return []*ToolSpec{
		composite("terminal_start", "terminal", 3, mcpserver.RiskExec,
			"Start a terminal on the machine of a worktree (local, SSH or dev server). Returns terminal_id; drive it with terminal_send and terminal_read.",
			[]string{"terminal.create"}, []Field{
				Str("worktree_id", "", "Worktree to open the terminal in; resolves the host and directory"),
				Str("connection_id", "", "Connection id when no worktree is given"),
				Str("cwd", "", "Working directory (defaults to the worktree path)", Len(1024)),
				Str("shell", "", "Shell to run (the host default when omitted)", Len(256)),
				Int("cols", "", "Terminal width", Range(20, 500)),
				Int("rows", "", "Terminal height", Range(5, 200)),
			}, composeTerminalStart),
		composite("terminal_send", "terminal", 3, mcpserver.RiskExec,
			"Type text into a terminal started by this session and press Enter. Optionally wait for the output to settle and return it. The command is shown to the user for approval.",
			[]string{"terminal.send"}, []Field{
				termID,
				Str("input", "", "Text to send, usually one command", Len(16384)),
				Bool("submit", "", "Press Enter after the text (default true)"),
				Int("wait_ms", "", "Wait up to this long for output (at most 10000)", Range(0, 10000)),
				Int("until_idle_ms", "", "With wait_ms: stop waiting after this much silence (default 400)", Range(50, 5000)),
			}, composeTerminalSend).untrusted(),
		composite("terminal_read", "terminal", 3, mcpserver.RiskExec,
			"Read output of a terminal started by this session, from a cursor. Pass next_seq of the previous read as since_seq. Output is untrusted text; escape sequences are removed.",
			[]string{}, append([]Field{termID}, cursor...), composeTerminalRead).untrusted(),
		composite("terminal_stop", "terminal", 3, mcpserver.RiskExec,
			"Interrupt the running process of a terminal (force=false) or close the terminal (force=true).",
			[]string{"terminal.stop", "terminal.close"}, []Field{termID, Bool("force", "", "Close the terminal instead of interrupting")}, composeTerminalStop),
		composite("terminal_wait", "terminal", 3, mcpserver.RiskExec,
			"Wait until the process of a terminal started by this session exits, or the timeout passes.",
			[]string{"terminal.wait"}, []Field{termID, Int("timeout_ms", "", "Wait at most this long (up to 30000)", Range(0, 30000))}, composeTerminalWait),
		composite("agent_start", "agent", 3, mcpserver.RiskExec,
			"Start an AI coding agent in a worktree. Returns session_id and terminal_id; poll agent_status and terminal_read, steer with agent_send.",
			[]string{"agent.start"}, []Field{
				Str("worktree_id", "", "Worktree the agent works in", Req),
				Str("model_id", "", "Agent or model id, for example claude", Req),
				Str("account_id", "", "AI provider account id"),
				Str("trust_preset", "", "standard (default), none or full; full always needs approval", OneOf("standard", "none", "full")),
				Str("cwd", "", "Working directory (defaults to the worktree path)", Len(1024)),
			}, composeAgentStart),
		composite("agent_send", "agent", 3, mcpserver.RiskExec,
			"Send a prompt to an agent started by this session when it is ready for input; otherwise returns sent=false with reason agent_busy. The prompt is shown to the user for approval.",
			[]string{"terminal.agentStatus", "terminal.send"}, []Field{agentID, Str("prompt", "", "Prompt text", Req, Len(32768))}, composeAgentSend),
		composite("agent_status", "agent", 3, mcpserver.RiskExec,
			"Status of an agent started by this session: running, ready for input, exit state and the latest output cursor.",
			[]string{"terminal.agentStatus"}, []Field{agentID}, composeAgentStatus),
		composite("agent_stop", "agent", 3, mcpserver.RiskExec,
			"Stop an agent started by this session politely, or kill it with force=true.",
			[]string{"agent.stop", "agent.kill"}, []Field{agentID, Bool("force", "", "Kill instead of stopping politely")}, composeAgentStop),
		composite("workflow_run", "workflow", 3, mcpserver.RiskExec,
			"Start a workflow template; it runs in the background. Calling again with the same arguments in the same session does not start a second run unless request_id differs.",
			[]string{"workflow.execute"}, []Field{
				Str("template_id", "", "Workflow template id", Req),
				Str("project_id", "", "Project id"),
				Str("request_id", "", "Idempotency key; defaults to a hash of the session and arguments", Len(128)),
			}, composeWorkflowRun),
	}
}

func pack1WorkflowRunStatus() []*ToolSpec {
	return []*ToolSpec{composite("workflow_run_status", "workflow", 1, mcpserver.RiskRead,
		"Status of a workflow execution started with workflow_run.",
		[]string{"workflow.getExecution"}, []Field{Str("execution_id", "", "Execution id from workflow_run", Req)}, composeWorkflowStatus)}
}

// ---- helpers --------------------------------------------------------------

func (env *CompositeEnv) owned(ts *ToolSession, id string) (*managedPty, error) {
	p, ok := ts.pty.find(id)
	if !ok || p.kind != kindTerminal {
		return nil, &ToolError{"MCP_NOT_FOUND", "terminal not found in this MCP session"}
	}
	p.touch(env.e.now())
	return p, nil
}

func (env *CompositeEnv) ownedAgent(ts *ToolSession, id string) (*managedPty, error) {
	p, ok := ts.pty.findAgent(id)
	if !ok {
		return nil, &ToolError{"MCP_NOT_FOUND", "agent session not found in this MCP session"}
	}
	p.touch(env.e.now())
	return p, nil
}

func (env *CompositeEnv) rateLimit(ts *ToolSession) error {
	if ok, wait := ts.pty.limiter.allow(); !ok {
		return &ToolError{"RATE_LIMITED", fmt.Sprintf("too many reads; retry_after_ms=%d", wait.Milliseconds()+1)}
	}
	return nil
}

// resolveTarget finds where to run: explicit connection or the worktree's host.
// It never falls back to a local PTY (infra-fleet refuses host-local ones).
func (env *CompositeEnv) resolveTarget(ctx context.Context, worktreeID, connectionID, cwd string) (conn, dir string, err error) {
	conn, dir = connectionID, cwd
	if worktreeID != "" {
		t := env.e.sessions.cfg.Targets
		if t == nil {
			if conn == "" {
				return "", "", &ToolError{"INVALID_ARGUMENTS", "worktree resolution is unavailable; pass connection_id and cwd"}
			}
		} else {
			c, d, rerr := t.ResolveWorktreeTarget(ctx, env.Identity, worktreeID)
			if rerr != nil {
				return "", "", &ToolError{"MCP_NOT_FOUND", "worktree not found or its host is not reachable"}
			}
			if conn == "" {
				conn = c
			}
			if dir == "" {
				dir = d
			}
		}
	}
	if conn == "" {
		return "", "", &ToolError{"INVALID_ARGUMENTS", "no host: pass a worktree_id bound to a host or a connection_id"}
	}
	return conn, dir, nil
}

// outputResult reads a ring from a cursor and shapes the tool result.
func (env *CompositeEnv) outputResult(ctx context.Context, p *managedPty, since *uint64, maxBytes, waitMs int) (map[string]any, error) {
	cfg := env.e.sessions.cfg
	if maxBytes <= 0 {
		maxBytes = cfg.ReadMaxBytes
	}
	maxBytes = min(maxBytes, cfg.ReadHardMaxBytes)
	rr, err := p.ring.Read(since, maxBytes)
	if err != nil {
		return nil, &ToolError{"INVALID_CURSOR", "since_seq is beyond the output produced so far; read again without since_seq"}
	}
	if len(rr.Data) == 0 && !rr.Exited && !rr.Detached && waitMs > 0 {
		p.ring.Wait(ctx.Done(), rr.Next, time.Duration(waitMs)*time.Millisecond)
		if rr, err = p.ring.Read(since, maxBytes); err != nil {
			return nil, &ToolError{"INVALID_CURSOR", "since_seq is beyond the output produced so far"}
		}
	}
	over := rr.Exited || rr.Detached
	text, consumed := sanitizeTerminal(rr.Data, over && rr.Next >= rr.Tail)
	next := rr.From + uint64(consumed)
	out := map[string]any{
		"terminal_id": p.ptyID, "text": text, "next_seq": next, "dropped_bytes": rr.Dropped,
		"truncated": rr.Dropped > 0 || next < rr.Tail, "has_more": next < rr.Tail, "exited": rr.Exited,
	}
	if rr.Exited {
		out["exit_code"] = rr.ExitCode
	}
	if rr.Detached && !rr.Exited {
		out["detached"] = true
	}
	return out, nil
}

// ---- terminal tools ---------------------------------------------------------

func composeTerminalStart(ctx context.Context, env *CompositeEnv, in json.RawMessage) (any, error) {
	a, err := decodeInput[struct {
		WorktreeID   string `json:"worktree_id"`
		ConnectionID string `json:"connection_id"`
		Cwd          string `json:"cwd"`
		Shell        string `json:"shell"`
		Cols         int    `json:"cols"`
		Rows         int    `json:"rows"`
	}](in)
	if err != nil {
		return nil, err
	}
	ts, err := env.Session(ctx)
	if err != nil {
		return nil, err
	}
	conn, cwd, err := env.resolveTarget(ctx, a.WorktreeID, a.ConnectionID, a.Cwd)
	if err != nil {
		return nil, err
	}
	release, err := env.e.sessions.reserve(ctx, ts, kindTerminal)
	if err != nil {
		return nil, err
	}
	defer release()
	if a.Cols == 0 {
		a.Cols = 120
	}
	if a.Rows == 0 {
		a.Rows = 30
	}
	ack, events, err := env.DispatchStream(ctx, ts, "terminal.create", map[string]any{
		"connectionId": conn, "cwd": cwd, "shell": a.Shell, "cols": a.Cols, "rows": a.Rows,
	})
	if err != nil {
		return nil, err
	}
	term, _ := asMap(ack)["terminal"].(map[string]any)
	ptyID, _ := term["ptyId"].(string)
	if ptyID == "" {
		return nil, &ToolError{"MCP_INTERNAL", "terminal was created without an id"}
	}
	now := env.e.now()
	pt := &managedPty{kind: kindTerminal, ptyID: ptyID, ring: newPtyOutputRing(env.e.sessions.cfg.RingBytes, now), created: now}
	if err := env.e.sessions.register(ts, pt, events); err != nil {
		return nil, err
	}
	out := map[string]any{"terminal_id": ptyID, "cwd": term["cwd"]}
	if o, ok := term["origin"]; ok {
		out["origin"] = o
	}
	return out, nil
}

func composeTerminalSend(ctx context.Context, env *CompositeEnv, in json.RawMessage) (any, error) {
	a, err := decodeInput[struct {
		TerminalID  string `json:"terminal_id"`
		Input       string `json:"input"`
		Submit      *bool  `json:"submit"`
		WaitMs      int    `json:"wait_ms"`
		UntilIdleMs int    `json:"until_idle_ms"`
	}](in)
	if err != nil {
		return nil, err
	}
	ts, err := env.Session(ctx)
	if err != nil {
		return nil, err
	}
	p, err := env.owned(ts, a.TerminalID)
	if err != nil {
		return nil, err
	}
	if exited, _, detached, _ := p.ring.State(); exited || detached {
		return nil, &ToolError{"TERMINAL_EXITED", "the terminal process has exited; start a new terminal"}
	}
	text := a.Input
	if a.Submit == nil || *a.Submit {
		text += ptySubmitKey
	}
	before := p.ring.Tail()
	if _, err := env.Dispatch(ctx, ts, "terminal.send", map[string]any{"terminal": p.ptyID, "text": text}); err != nil {
		return nil, err
	}
	if a.WaitMs <= 0 {
		return map[string]any{"terminal_id": p.ptyID, "sent": true, "next_seq": before}, nil
	}
	idle := time.Duration(a.UntilIdleMs) * time.Millisecond
	if idle <= 0 {
		idle = 400 * time.Millisecond
	}
	p.ring.WaitIdle(ctx.Done(), idle, time.Duration(a.WaitMs)*time.Millisecond, env.e.now)
	out, err := env.outputResult(ctx, p, &before, 0, 0)
	if err != nil {
		return nil, err
	}
	out["sent"] = true
	return out, nil
}

func composeTerminalRead(ctx context.Context, env *CompositeEnv, in json.RawMessage) (any, error) {
	a, err := decodeInput[struct {
		TerminalID string  `json:"terminal_id"`
		SinceSeq   *uint64 `json:"since_seq"`
		MaxBytes   int     `json:"max_bytes"`
		WaitMs     int     `json:"wait_ms"`
	}](in)
	if err != nil {
		return nil, err
	}
	ts, err := env.Session(ctx)
	if err != nil {
		return nil, err
	}
	p, err := env.owned(ts, a.TerminalID)
	if err != nil {
		return nil, err
	}
	if err := env.rateLimit(ts); err != nil {
		return nil, err
	}
	return env.outputResult(ctx, p, a.SinceSeq, a.MaxBytes, a.WaitMs)
}

func composeTerminalStop(ctx context.Context, env *CompositeEnv, in json.RawMessage) (any, error) {
	a, err := decodeInput[struct {
		TerminalID string `json:"terminal_id"`
		Force      bool   `json:"force"`
	}](in)
	if err != nil {
		return nil, err
	}
	ts, err := env.Session(ctx)
	if err != nil {
		return nil, err
	}
	p, err := env.owned(ts, a.TerminalID)
	if err != nil {
		return nil, err
	}
	if !a.Force {
		if _, err := env.Dispatch(ctx, ts, "terminal.stop", map[string]any{"terminal": p.ptyID}); err != nil {
			return nil, err
		}
		return map[string]any{"terminal_id": p.ptyID, "stopped": true, "forced": false}, nil
	}
	env.e.sessions.stopPty(ctx, ts, p)
	return map[string]any{"terminal_id": p.ptyID, "stopped": true, "forced": true}, nil
}

func composeTerminalWait(ctx context.Context, env *CompositeEnv, in json.RawMessage) (any, error) {
	a, err := decodeInput[struct {
		TerminalID string `json:"terminal_id"`
		TimeoutMs  int    `json:"timeout_ms"`
	}](in)
	if err != nil {
		return nil, err
	}
	ts, err := env.Session(ctx)
	if err != nil {
		return nil, err
	}
	p, err := env.owned(ts, a.TerminalID)
	if err != nil {
		return nil, err
	}
	res, err := env.Dispatch(ctx, ts, "terminal.wait", map[string]any{"terminal": p.ptyID, "timeoutMs": a.TimeoutMs})
	if err != nil {
		return nil, err
	}
	m := asMap(res)
	return map[string]any{"terminal_id": p.ptyID, "exited": m["exited"], "exit_code": m["exitCode"], "timed_out": m["timedOut"]}, nil
}

// ---- agent tools --------------------------------------------------------------

func composeAgentStart(ctx context.Context, env *CompositeEnv, in json.RawMessage) (any, error) {
	a, err := decodeInput[struct {
		WorktreeID  string `json:"worktree_id"`
		ModelID     string `json:"model_id"`
		AccountID   string `json:"account_id"`
		TrustPreset string `json:"trust_preset"`
		Cwd         string `json:"cwd"`
	}](in)
	if err != nil {
		return nil, err
	}
	ts, err := env.Session(ctx)
	if err != nil {
		return nil, err
	}
	conn, cwd, err := env.resolveTarget(ctx, a.WorktreeID, "", a.Cwd)
	if err != nil {
		return nil, err
	}
	release, err := env.e.sessions.reserve(ctx, ts, kindAgent)
	if err != nil {
		return nil, err
	}
	defer release()
	if a.TrustPreset == "" {
		a.TrustPreset = "standard"
	}
	// userId is the verified caller, never an input: the agent runs as them.
	ack, events, err := env.DispatchStream(ctx, ts, "agent.start", map[string]any{
		"connectionId": conn, "worktreeId": a.WorktreeID, "userId": env.Identity.UserID, "cwd": cwd,
		"modelId": a.ModelID, "accountId": a.AccountID, "trustPreset": a.TrustPreset, "cols": 120, "rows": 30,
	})
	if err != nil {
		return nil, err
	}
	m := asMap(ack)
	sessionID, _ := m["id"].(string)
	ptyID, _ := m["ptyId"].(string)
	if sessionID == "" || ptyID == "" {
		return nil, &ToolError{"MCP_INTERNAL", "agent was started without an id"}
	}
	now := env.e.now()
	pt := &managedPty{kind: kindAgent, ptyID: ptyID, sessionID: sessionID, ring: newPtyOutputRing(env.e.sessions.cfg.RingBytes, now), created: now}
	if err := env.e.sessions.register(ts, pt, events); err != nil {
		return nil, err
	}
	out := map[string]any{"session_id": sessionID, "terminal_id": ptyID, "status": m["status"]}
	if o, ok := m["origin"]; ok {
		out["origin"] = o
	}
	return out, nil
}

func (env *CompositeEnv) agentStatus(ctx context.Context, ts *ToolSession, p *managedPty) (map[string]any, error) {
	res, err := env.Dispatch(ctx, ts, "terminal.agentStatus", map[string]any{"terminal": p.ptyID})
	if err != nil {
		return nil, err
	}
	return asMap(res), nil
}

func composeAgentSend(ctx context.Context, env *CompositeEnv, in json.RawMessage) (any, error) {
	a, err := decodeInput[struct {
		SessionID string `json:"session_id"`
		Prompt    string `json:"prompt"`
	}](in)
	if err != nil {
		return nil, err
	}
	ts, err := env.Session(ctx)
	if err != nil {
		return nil, err
	}
	p, err := env.ownedAgent(ts, a.SessionID)
	if err != nil {
		return nil, err
	}
	if exited, _, detached, _ := p.ring.State(); exited || detached {
		return nil, &ToolError{"AGENT_EXITED", "the agent has exited"}
	}
	st, err := env.agentStatus(ctx, ts, p)
	if err != nil {
		return nil, err
	}
	// No queueing: DispatchPrompt exists only behind the excluded mobile channel.
	if ready, _ := st["readyForInput"].(bool); !ready {
		return map[string]any{"session_id": a.SessionID, "sent": false, "reason": "agent_busy"}, nil
	}
	before := p.ring.Tail()
	if _, err := env.Dispatch(ctx, ts, "terminal.send", map[string]any{"terminal": p.ptyID, "text": a.Prompt + ptySubmitKey}); err != nil {
		return nil, err
	}
	return map[string]any{"session_id": a.SessionID, "sent": true, "output_from_seq": before}, nil
}

func composeAgentStatus(ctx context.Context, env *CompositeEnv, in json.RawMessage) (any, error) {
	a, err := decodeInput[struct {
		SessionID string `json:"session_id"`
	}](in)
	if err != nil {
		return nil, err
	}
	ts, err := env.Session(ctx)
	if err != nil {
		return nil, err
	}
	p, err := env.ownedAgent(ts, a.SessionID)
	if err != nil {
		return nil, err
	}
	if err := env.rateLimit(ts); err != nil {
		return nil, err
	}
	exited, code, detached, _ := p.ring.State()
	out := map[string]any{"session_id": a.SessionID, "terminal_id": p.ptyID, "exited": exited, "last_output_seq": p.ring.Tail()}
	if exited {
		out["exit_code"] = code
	}
	if detached && !exited {
		out["detached"] = true
	}
	if !exited && !detached {
		st, err := env.agentStatus(ctx, ts, p)
		if err != nil {
			return nil, err
		}
		out["agent_running"], out["agent_kind"], out["ready_for_input"] = st["agentRunning"], st["agentKind"], st["readyForInput"]
	}
	return out, nil
}

func composeAgentStop(ctx context.Context, env *CompositeEnv, in json.RawMessage) (any, error) {
	a, err := decodeInput[struct {
		SessionID string `json:"session_id"`
		Force     bool   `json:"force"`
	}](in)
	if err != nil {
		return nil, err
	}
	ts, err := env.Session(ctx)
	if err != nil {
		return nil, err
	}
	p, err := env.ownedAgent(ts, a.SessionID)
	if err != nil {
		return nil, err
	}
	if a.Force {
		if _, err := env.Dispatch(ctx, ts, "agent.kill", map[string]any{"sessionId": a.SessionID, "signal": "SIGKILL"}); err != nil {
			return nil, err
		}
		ts.pty.streams.Detach(p.ptyID)
		ts.pty.forget(p)
		return map[string]any{"session_id": a.SessionID, "stopped": true, "forced": true}, nil
	}
	if _, err := env.Dispatch(ctx, ts, "agent.stop", map[string]any{"sessionId": a.SessionID}); err != nil {
		return nil, err
	}
	return map[string]any{"session_id": a.SessionID, "stopped": true, "forced": false}, nil
}

// ---- workflow tools ---------------------------------------------------------

func composeWorkflowRun(ctx context.Context, env *CompositeEnv, in json.RawMessage) (any, error) {
	a, err := decodeInput[struct {
		TemplateID string `json:"template_id"`
		ProjectID  string `json:"project_id"`
		RequestID  string `json:"request_id"`
	}](in)
	if err != nil {
		return nil, err
	}
	rid := a.RequestID
	if rid == "" {
		// A retried identical call must not start a second run.
		sum := sha256.Sum256([]byte(env.Principal.TenantID + "\x00" + env.Principal.UserID + "\x00" + env.SessionID + "\x00" + a.TemplateID + "\x00" + a.ProjectID))
		rid = hex.EncodeToString(sum[:16])
	}
	return env.DispatchPlain(ctx, "workflow.execute", map[string]any{"templateId": a.TemplateID, "projectId": a.ProjectID, "requestId": rid})
}

func composeWorkflowStatus(ctx context.Context, env *CompositeEnv, in json.RawMessage) (any, error) {
	a, err := decodeInput[struct {
		ExecutionID string `json:"execution_id"`
	}](in)
	if err != nil {
		return nil, err
	}
	return env.DispatchPlain(ctx, "workflow.getExecution", map[string]any{"executionId": a.ExecutionID})
}
