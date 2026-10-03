package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

// Dispatcher is the slice of *wscompat.Registry the executor needs.
type Dispatcher interface {
	Dispatch(ctx context.Context, id wscompat.Identity, channel string, args []json.RawMessage) (any, error)
	DispatchStreamChannel(ctx context.Context, id wscompat.Identity, channel string, args []json.RawMessage) (ack any, events <-chan wscompat.PushEvent, ok bool, err error)
}

// Executor implements mcpserver.ToolExecutor.
type Executor struct {
	catalog *Catalog
	disp    Dispatcher
	gate    mcpserver.PolicyGate
	session *ToolSession
	cfg     Config
	log     *slog.Logger

	guards Guards

	// sessions owns the PTYs of terminal/agent tools per MCP session (BE-009).
	sessions *ToolSessions

	cacheMu sync.Mutex
	cache   map[string]cachedResult
	now     func() time.Time

	// scmLimit spends the per-tenant provider quota (github, gitlab, linear).
	scmLimit *scmLimiter
}

// Guards are the governance hooks the composition root plugs in (BE-MCP-SOL-012/013).
// All optional; nil hooks are skipped.
type Guards struct {
	// WatchKill returns a context cancelled shortly after the principal is
	// kill-switched, so a running tool is stopped (mcppolicy.Gate.WatchKill).
	WatchKill func(ctx context.Context, p mcpserver.Principal) (context.Context, context.CancelFunc)
	// WrapUntrusted frames third-party text in tool output as data, not
	// instructions (mcppolicy.WrapUntrusted).
	WrapUntrusted func(source, text string) string
	// NeverDispatch is the last-resort fuse: channels it matches are refused
	// even when the gate allowed them (mcppolicy.NeverDispatch).
	NeverDispatch func(channel string) bool
	// SessionID / ClientName read the verified MCP session facts the transport
	// put on the request context (mcppolicy.SessionIDFromContext / ClientNameFromContext).
	// Terminal and agent tools need them to own and later stop their PTYs.
	SessionID  func(ctx context.Context) string
	ClientName func(ctx context.Context) string
}

// WithGuards installs the governance hooks and returns the executor.
func (e *Executor) WithGuards(g Guards) *Executor {
	e.guards = g
	return e
}

type cachedResult struct {
	obj map[string]any
	exp time.Time
}

const maxCachedResults = 1024

// NewExecutor wires the pipeline. gate nil = fail-closed default; session nil
// = a fresh background session.
func NewExecutor(c *Catalog, d Dispatcher, gate mcpserver.PolicyGate, s *ToolSession, cfg Config, log *slog.Logger) *Executor {
	if gate == nil {
		gate = mcpserver.FailClosedGate{}
	}
	if s == nil {
		s = NewToolSession(context.Background())
	}
	if log == nil {
		log = slog.Default()
	}
	e := &Executor{catalog: c, disp: d, gate: gate, session: s, cfg: cfg.withDefaults(), log: log,
		cache: map[string]cachedResult{}, now: time.Now}
	e.scmLimit = newSCMLimiter(e.cfg.SCMRatePerMin, e.nowFn)
	e.sessions = newToolSessions(s, d, DefaultPtyToolsConfig(), log, e.nowFn, func() Guards { return e.guards })
	return e
}

func (e *Executor) nowFn() time.Time { return e.now() }

// WithPtyTools sets the terminal/agent tool limits and worktree resolver. Call
// it before the executor serves requests.
func (e *Executor) WithPtyTools(cfg PtyToolsConfig) *Executor {
	e.sessions.Close()
	e.sessions = newToolSessions(e.session, e.disp, cfg, e.log, e.nowFn, func() Guards { return e.guards })
	return e
}

// CloseSession stops every terminal and agent the given MCP session (row id)
// started; wire it to the session-closed signal. Idempotent.
func (e *Executor) CloseSession(mcpSessionID, reason string) {
	e.sessions.CloseSession(mcpSessionID, reason)
}

// Close stops all PTYs and background goroutines (process shutdown).
func (e *Executor) Close() { e.sessions.Close() }

func toolErr(code, msg string) error { return fmt.Errorf("%s: %s", code, msg) }

// CallTool runs: lookup -> validate -> scope -> args -> PolicyGate.Decide ->
// (approval) -> dispatch -> normalize/redact/truncate -> PolicyGate.Complete.
func (e *Executor) CallTool(ctx context.Context, p mcpserver.Principal, name string, args json.RawMessage) (*mcp.CallToolResult, error) {
	spec, ok := e.catalog.Lookup(name)
	if !ok {
		return nil, mcpserver.ErrUnknownTool
	}
	input, err := validateInput(spec, args)
	if err != nil {
		return nil, err
	}
	if !hasScope(p, spec.RequiredScope()) {
		return nil, toolErr("MCP_SCOPE_NOT_ALLOWED", "the token lacks the scope required by this tool")
	}
	id := wscompat.Identity{TenantID: p.TenantID, UserID: p.UserID, Role: p.Role}
	chanArgs, err := spec.buildArgs(input, id, e.cfg)
	if err != nil {
		return nil, toolErr("INVALID_ARGUMENTS", shorten(err.Error()))
	}
	meta := spec.Meta()
	if err := e.authorize(ctx, p, meta, input); err != nil {
		return nil, err
	}
	start := e.now()
	if e.guards.NeverDispatch != nil && neverDispatched(e.guards.NeverDispatch, spec) {
		e.gate.Complete(context.WithoutCancel(ctx), p, meta, "error", 0)
		return nil, toolErr("MCP_POLICY_DENIED", "this tool call is not permitted by policy")
	}
	obj, err := e.run(ctx, p, spec, id, chanArgs, input)
	outcome := "ok"
	if err != nil {
		outcome = "error"
	}
	// Detached ctx: audit must land even if the client cancelled the call.
	e.gate.Complete(context.WithoutCancel(ctx), p, meta, outcome, e.now().Sub(start))
	if err != nil {
		return nil, mapExecError(err)
	}
	res := resultOf(obj)
	if meta.UntrustedOutput && e.guards.WrapUntrusted != nil {
		// Only the text block an LLM reads is framed; structuredContent keeps
		// the schema-validated data for programmatic consumers.
		if tc, ok := res.Content[0].(*mcp.TextContent); ok {
			tc.Text = e.guards.WrapUntrusted(spec.Name, tc.Text)
		}
	}
	return res, nil
}

func validateInput(spec *ToolSpec, args json.RawMessage) (json.RawMessage, error) {
	if len(strings.TrimSpace(string(args))) == 0 || string(args) == "null" {
		args = json.RawMessage(`{}`)
	}
	var inst any
	if err := json.Unmarshal(args, &inst); err != nil {
		return nil, toolErr("INVALID_ARGUMENTS", "arguments must be a JSON object")
	}
	if _, isObj := inst.(map[string]any); !isObj {
		return nil, toolErr("INVALID_ARGUMENTS", "arguments must be a JSON object")
	}
	if err := spec.resolved.Validate(inst); err != nil {
		return nil, toolErr("INVALID_ARGUMENTS", shorten(err.Error()))
	}
	return args, nil
}

func (s *ToolSpec) buildArgs(in json.RawMessage, id wscompat.Identity, cfg Config) ([]json.RawMessage, error) {
	if s.ArgsOverride != nil {
		return s.ArgsOverride(in, id)
	}
	return defaultArgs(s.Fields, s.Consts, in)
}

// authorize applies the PolicyGate; every failure path denies.
func (e *Executor) authorize(ctx context.Context, p mcpserver.Principal, meta mcpserver.ToolMeta, input json.RawMessage) error {
	d, err := e.gate.Decide(ctx, p, meta, input)
	if err != nil {
		e.log.WarnContext(ctx, "mcp policy gate failed; denying", slog.String("tool", meta.Name), slog.Any("error", err))
		return toolErr("MCP_UNAVAILABLE", "policy decision unavailable, retry later")
	}
	switch d.Outcome {
	case mcpserver.OutcomeAllow:
		return nil
	case mcpserver.OutcomeRequireApproval:
		if d.ApprovalID == "" {
			return toolErr("MCP_POLICY_DENIED", "approval is required but could not be requested")
		}
		approved, err := e.awaitApproval(ctx, p, d)
		if err != nil {
			e.log.WarnContext(ctx, "mcp approval wait failed; denying", slog.String("tool", meta.Name), slog.Any("error", err))
			return toolErr("MCP_UNAVAILABLE", "approval could not be completed")
		}
		if !approved {
			return toolErr("MCP_APPROVAL_DENIED", "the user did not approve this call")
		}
		return nil
	default: // "deny" and any unknown outcome
		msg := "this tool call is not permitted by policy"
		if m := strings.TrimSpace(d.Message); m != "" {
			msg = shorten(m)
		}
		return toolErr("MCP_POLICY_DENIED", msg)
	}
}

// awaitApproval waits for the owner's decision on ANY channel. When mcp-service
// marked the approval elicitation-eligible and the client supports it, the
// question is also asked in-band; the answer is only a hint to mcp-service,
// which re-checks the risk before honoring an approve.
func (e *Executor) awaitApproval(ctx context.Context, p mcpserver.Principal, d mcpserver.GateDecision) (bool, error) {
	elicit, canAsk := mcpserver.ElicitorFromContext(ctx)
	decider, canDecide := e.gate.(mcpserver.ElicitationDecider)
	if !canAsk || !canDecide || !d.ElicitationEligible || d.ApprovalPrompt == "" {
		return e.gate.AwaitApproval(ctx, p, d.ApprovalID)
	}
	ectx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		dec, err := elicit(ectx, d.ApprovalPrompt)
		if err != nil {
			e.log.DebugContext(ctx, "mcp elicitation not answered; waiting for the out-of-band approval", slog.Any("error", err))
			return
		}
		approve := dec.Action == "accept" && dec.Approve
		if dec.Action != "accept" && dec.Action != "decline" && dec.Action != "cancel" {
			return
		}
		if err := decider.DecideByElicitation(context.WithoutCancel(ctx), p, d.ApprovalID, approve); err != nil {
			e.log.WarnContext(ctx, "mcp elicitation decision not recorded", slog.Any("error", err))
		}
	}()
	approved, err := e.gate.AwaitApproval(ctx, p, d.ApprovalID)
	cancel()
	<-done
	return approved, err
}

func (e *Executor) run(ctx context.Context, p mcpserver.Principal, spec *ToolSpec, id wscompat.Identity, args []json.RawMessage, input json.RawMessage) (map[string]any, error) {
	if e.guards.WatchKill != nil {
		parent := ctx
		kctx, kcancel := e.guards.WatchKill(ctx, p)
		defer kcancel()
		ctx = kctx
		obj, err := e.runGuarded(ctx, p, spec, id, args, input)
		if err != nil && kctx.Err() != nil && parent.Err() == nil && e.session.Context().Err() == nil {
			return nil, toolErr("MCP_KILL_SWITCH_ACTIVE", "MCP access was suspended by an administrator")
		}
		return obj, err
	}
	return e.runGuarded(ctx, p, spec, id, args, input)
}

func (e *Executor) runGuarded(ctx context.Context, p mcpserver.Principal, spec *ToolSpec, id wscompat.Identity, args []json.RawMessage, input json.RawMessage) (map[string]any, error) {
	key := ""
	if spec.CacheTTL > 0 {
		key = cacheKey(p, spec.Name, input)
		if obj, ok := e.cacheGet(key); ok {
			return obj, nil
		}
	}
	if err := spec.guardInputPath(input, e.cfg.SensitivePathExtra); err != nil {
		return nil, err
	}
	// After the cache check: a cached answer never touches the provider.
	if ok, wait := e.scmLimit.allow(p.TenantID, rateGroup(spec.Namespace)); !ok {
		return nil, &ToolError{"RATE_LIMITED", fmt.Sprintf("too many %s requests for this tenant; retry in %ds", spec.Namespace, int(wait.Seconds())+1)}
	}
	ctx, cancel := context.WithTimeout(ctx, e.cfg.ToolTimeout)
	defer cancel()
	stop := context.AfterFunc(e.session.Context(), cancel)
	defer stop()

	var res any
	var err error
	if spec.Kind == KindComposite {
		if spec.Compose == nil {
			return nil, errors.New("MCP_INTERNAL: composite tool without implementation")
		}
		res, err = spec.Compose(ctx, e.compositeEnv(ctx, p, id), input)
	} else {
		res, err = e.dispatch(ctx, id, spec.Channel, args)
	}
	if err != nil {
		return nil, err
	}
	obj, err := normalizeResult(res, spec)
	if err != nil {
		return nil, err
	}
	spec.filterSensitiveItems(obj, input, e.cfg.SensitivePathExtra)
	if piiApplies(e.cfg.PIIMask, spec) {
		obj = maskPII(obj).(map[string]any)
	}
	if key != "" {
		e.cachePut(key, obj, time.Duration(spec.CacheTTL)*time.Second)
	}
	return obj, nil
}

// dispatch uses the unary path, or for streamChannel channels takes only the
// ack and cancels the event stream (pack 1/2 never consume pushes).
func (e *Executor) dispatch(ctx context.Context, id wscompat.Identity, channel string, args []json.RawMessage) (any, error) {
	sctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ack, _, ok, err := e.disp.DispatchStreamChannel(sctx, id, channel, args)
	if ok {
		return ack, err
	}
	return e.disp.Dispatch(ctx, id, channel, args)
}

func resultOf(obj map[string]any) *mcp.CallToolResult {
	b, _ := json.Marshal(obj)
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}, StructuredContent: obj}
}

func cacheKey(p mcpserver.Principal, tool string, input json.RawMessage) string {
	var canon any
	_ = json.Unmarshal(input, &canon)
	b, _ := json.Marshal(canon) // map keys are sorted -> canonical
	sum := sha256.Sum256([]byte(p.TenantID + "\x00" + p.UserID + "\x00" + tool + "\x00" + string(b)))
	return hex.EncodeToString(sum[:])
}

func (e *Executor) cacheGet(key string) (map[string]any, bool) {
	e.cacheMu.Lock()
	defer e.cacheMu.Unlock()
	c, ok := e.cache[key]
	if !ok || e.now().After(c.exp) {
		delete(e.cache, key)
		return nil, false
	}
	return c.obj, true
}

func (e *Executor) cachePut(key string, obj map[string]any, ttl time.Duration) {
	e.cacheMu.Lock()
	defer e.cacheMu.Unlock()
	if len(e.cache) >= maxCachedResults {
		for k, v := range e.cache { // evict expired, else one arbitrary entry
			if e.now().After(v.exp) {
				delete(e.cache, k)
			}
		}
		if len(e.cache) >= maxCachedResults {
			for k := range e.cache {
				delete(e.cache, k)
				break
			}
		}
	}
	e.cache[key] = cachedResult{obj: obj, exp: e.now().Add(ttl)}
}

var codedMessage = regexp.MustCompile(`^([A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+):`)

// mapExecError never leaks internals: only a CODE survives from wscompat's
// "CODE: message" errors (messages may embed arguments or hosts).
func mapExecError(err error) error {
	var te *ToolError
	if errors.As(err, &te) {
		return toolErr(te.Code, te.Msg)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return toolErr("MCP_TIMEOUT", "the tool timed out; retry with a narrower request")
	}
	if errors.Is(err, context.Canceled) {
		return toolErr("MCP_TIMEOUT", "the call was cancelled")
	}
	st, isStatus := status.FromError(err)
	if isStatus {
		switch st.Code() {
		case codes.InvalidArgument:
			if m := codedMessage.FindStringSubmatch(st.Message()); m != nil {
				return toolErr(m[1], "invalid request")
			}
			return toolErr("MCP_INVALID_ARGUMENT", "invalid argument")
		case codes.NotFound, codes.PermissionDenied:
			return toolErr("MCP_NOT_FOUND", "not found or not permitted")
		case codes.Unavailable, codes.DeadlineExceeded:
			return toolErr("MCP_UNAVAILABLE", "temporarily unavailable, retry")
		}
		if m := codedMessage.FindStringSubmatch(st.Message()); m != nil {
			return toolErr(m[1], "request failed")
		}
		if st.Code() != codes.Unknown {
			return toolErr("MCP_INTERNAL", "internal error")
		}
	}
	if m := codedMessage.FindStringSubmatch(err.Error()); m != nil {
		return toolErr(m[1], "request failed")
	}
	if strings.HasPrefix(err.Error(), "INVALID_ARGUMENTS") || strings.HasPrefix(err.Error(), "MCP_") {
		return err
	}
	return toolErr("MCP_INTERNAL", "internal error")
}

func shorten(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// neverDispatched also covers the channels a composite tool dispatches.
func neverDispatched(fuse func(string) bool, spec *ToolSpec) bool {
	if fuse(spec.Channel) {
		return true
	}
	for _, c := range spec.UsesChannels {
		if fuse(c) {
			return true
		}
	}
	return false
}
