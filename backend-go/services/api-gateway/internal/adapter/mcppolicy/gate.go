// Package mcppolicy adapts mcp-service's governance RPCs to the
// mcpserver.PolicyGate port. It contains no business rules: every decision
// (policy, approvals, kill switch, limits, audit) is made by mcp-service, and
// any failure here is a deny.
package mcppolicy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// Config bounds every call to mcp-service (default 5s: arch/08) and the wait
// for a human (default 50s: below the ~60s timeout of common MCP SDKs).
type Config struct {
	CallTimeout     time.Duration
	ApprovalMaxWait time.Duration
}

func (c Config) withDefaults() Config {
	if c.CallTimeout <= 0 {
		c.CallTimeout = 5 * time.Second
	}
	if c.ApprovalMaxWait <= 0 {
		c.ApprovalMaxWait = 50 * time.Second
	}
	return c
}

const (
	msgUnavailable = "Orca cannot authorize this action right now."
	maxPending     = 4096
	pendingTTL     = 15 * time.Minute
)

// spawningChannels start processes or agents; mcp-service denies them at
// recursion depth >= MCP_MAX_DEPTH. Matching is by channel, never by anything
// the client supplies.
var spawningChannels = map[string]bool{
	"agent.start": true, "agent.resume": true, "agent.switchAccount": true, "terminal.create": true,
}

type callKey struct{ tenant, user, client, tool string }

type pendingApproval struct {
	hash string // params_hash returned by AuthorizeToolCall, required to decide
	p    mcpserver.Principal
	meta mcpserver.ToolMeta
	args json.RawMessage
	at   time.Time
}

// Gate implements mcpserver.PolicyGate (and mcpserver.ToolPolicyView).
type Gate struct {
	c   mcpv1.McpServiceClient
	cfg Config
	log *slog.Logger
	now func() time.Time

	mu      sync.Mutex
	started map[callKey][]string       // call ids awaiting Complete, FIFO per key
	pending map[string]pendingApproval // approval id -> original call, for re-authorization after approval
}

func NewGate(c mcpv1.McpServiceClient, cfg Config, log *slog.Logger) *Gate {
	if log == nil {
		log = slog.Default()
	}
	return &Gate{c: c, cfg: cfg.withDefaults(), log: log, now: time.Now,
		started: map[callKey][]string{}, pending: map[string]pendingApproval{}}
}

func tokenKind(p mcpserver.Principal) string {
	if p.GrantID != "" || p.ClientID != "" {
		return "mcp_oauth"
	}
	return "mcp_pat"
}

func toolRef(m mcpserver.ToolMeta) *mcpv1.ToolRef {
	return &mcpv1.ToolRef{
		Name: m.Name, Channel: m.Channel, Namespace: m.Namespace, Risk: m.Risk, RequiredScope: m.RequiredScope, Title: m.Name,
		OpenWorld: m.OpenWorld, SpawnsProcess: spawningChannels[m.Channel], ReadUntrusted: m.UntrustedOutput,
	}
}

func callContext(ctx context.Context, p mcpserver.Principal) *mcpv1.CallContext {
	depth, root := DepthFromContext(ctx)
	return &mcpv1.CallContext{
		ClientId: p.ClientID, ClientName: ClientNameFromContext(ctx), Scopes: p.Scopes, TokenKind: tokenKind(p), McpSessionId: SessionIDFromContext(ctx),
		Depth: int32(depth), McpRoot: root, TokenId: p.TokenID, GrantId: p.GrantID,
	}
}

// rpcCtx attaches the principal's identity as gRPC metadata and bounds the call.
func (g *Gate) rpcCtx(ctx context.Context, p mcpserver.Principal, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: p.TenantID, UserID: p.UserID, Role: p.Role})
	return context.WithTimeout(ctx, timeout)
}

func denyDecision(msg string) mcpserver.GateDecision {
	return mcpserver.GateDecision{Outcome: mcpserver.OutcomeDeny, Message: msg}
}

func (g *Gate) Decide(ctx context.Context, p mcpserver.Principal, meta mcpserver.ToolMeta, args json.RawMessage) (mcpserver.GateDecision, error) {
	if g == nil || g.c == nil {
		return denyDecision(msgUnavailable), errors.New("mcppolicy: mcp-service client not configured")
	}
	d, err := g.authorize(ctx, p, meta, args)
	if err != nil {
		return denyDecision(msgUnavailable), err
	}
	switch d.Outcome {
	case mcpserver.OutcomeAllow:
		// An allow without a journal row would be an unaudited call: refuse.
		if d.CallId == "" {
			return denyDecision(msgUnavailable), errors.New("mcppolicy: allow without call id")
		}
		g.pushCall(p, meta, d.CallId)
		return mcpserver.GateDecision{Outcome: mcpserver.OutcomeAllow, Reasons: d.Reasons}, nil
	case mcpserver.OutcomeRequireApproval:
		if d.ApprovalId == "" {
			return denyDecision(msgUnavailable), errors.New("mcppolicy: approval outcome without approval id")
		}
		g.rememberPending(d.ApprovalId, d.ParamsHash, p, meta, args)
		return mcpserver.GateDecision{Outcome: mcpserver.OutcomeRequireApproval, Reasons: d.Reasons, ApprovalID: d.ApprovalId, Message: d.Message,
			ElicitationEligible: d.ElicitationEligible, ApprovalPrompt: d.ApprovalPrompt}, nil
	default: // deny and anything unknown
		return mcpserver.GateDecision{Outcome: mcpserver.OutcomeDeny, Reasons: d.Reasons, Message: d.Message}, nil
	}
}

func (g *Gate) authorize(ctx context.Context, p mcpserver.Principal, meta mcpserver.ToolMeta, args json.RawMessage) (*mcpv1.AuthorizeToolCallResponse, error) {
	rctx, cancel := g.rpcCtx(ctx, p, g.cfg.CallTimeout)
	defer cancel()
	resp, err := g.c.AuthorizeToolCall(rctx, &mcpv1.AuthorizeToolCallRequest{Tool: toolRef(meta), Ctx: callContext(ctx, p), Arguments: args})
	if err != nil {
		return nil, fmt.Errorf("mcppolicy: authorize %s: %w", meta.Name, err)
	}
	return resp, nil
}

// AwaitApproval waits for the owner's decision. On approval it re-runs the
// authorization with the SAME arguments, which atomically consumes the
// single-use approval and starts the journal row; only an allow from that
// second call lets the tool run. Anything else (pending, denied, expired, or a
// lost consume race) is "not approved".
func (g *Gate) AwaitApproval(ctx context.Context, p mcpserver.Principal, approvalID string) (bool, error) {
	if g == nil || g.c == nil {
		return false, errors.New("mcppolicy: mcp-service client not configured")
	}
	g.mu.Lock()
	pend, ok := g.pending[approvalID]
	g.mu.Unlock()
	if !ok || pend.p.TenantID != p.TenantID || pend.p.UserID != p.UserID {
		return false, nil
	}
	rctx, cancel := g.rpcCtx(ctx, p, g.cfg.ApprovalMaxWait+g.cfg.CallTimeout)
	defer cancel()
	w, err := g.c.WaitApproval(rctx, &mcpv1.WaitApprovalRequest{ApprovalId: approvalID, MaxWaitMs: int32(g.cfg.ApprovalMaxWait / time.Millisecond)})
	if err != nil {
		return false, fmt.Errorf("mcppolicy: wait approval: %w", err)
	}
	if w.GetStatus() == "pending" {
		return false, nil // keep the pending entry: the client's retry may find it approved
	}
	g.mu.Lock()
	delete(g.pending, approvalID)
	g.mu.Unlock()
	if w.GetStatus() != "approved" {
		return false, nil
	}
	d, err := g.authorize(ctx, pend.p, pend.meta, pend.args)
	if err != nil {
		return false, err
	}
	if d.Outcome != mcpserver.OutcomeAllow || d.CallId == "" {
		return false, nil
	}
	g.pushCall(pend.p, pend.meta, d.CallId)
	return true, nil
}

// DecideByElicitation records the in-band answer of the approval's owner.
// The approver is the principal of the very MCP session that was asked.
func (g *Gate) DecideByElicitation(ctx context.Context, p mcpserver.Principal, approvalID string, approve bool) error {
	if g == nil || g.c == nil {
		return errors.New("mcppolicy: mcp-service client not configured")
	}
	g.mu.Lock()
	pend, ok := g.pending[approvalID]
	g.mu.Unlock()
	if !ok || pend.p.TenantID != p.TenantID || pend.p.UserID != p.UserID {
		return errors.New("mcppolicy: unknown approval")
	}
	decision := "deny"
	if approve {
		decision = "approve"
	}
	rctx, cancel := g.rpcCtx(ctx, p, g.cfg.CallTimeout)
	defer cancel()
	_, err := g.c.DecideApproval(rctx, &mcpv1.DecideApprovalRequest{ApprovalId: approvalID, Decision: decision, ParamsHash: pend.hash, Via: "elicitation"})
	if err != nil {
		return fmt.Errorf("mcppolicy: decide approval by elicitation: %w", err)
	}
	return nil
}

// Complete finalizes the journal row of the matching admitted call. Failures
// are logged only: mcp-service's reaper finalizes calls that never complete.
func (g *Gate) Complete(ctx context.Context, p mcpserver.Principal, meta mcpserver.ToolMeta, result string, dur time.Duration) {
	if g == nil || g.c == nil {
		return
	}
	callID := g.popCall(p, meta)
	if callID == "" {
		return
	}
	if result != "ok" {
		result = "error"
	}
	rctx, cancel := g.rpcCtx(context.WithoutCancel(ctx), p, g.cfg.CallTimeout)
	defer cancel()
	if _, err := g.c.CompleteToolCall(rctx, &mcpv1.CompleteToolCallRequest{CallId: callID, Result: result, DurationMs: dur.Milliseconds()}); err != nil {
		g.log.WarnContext(ctx, "mcp tool completion not recorded; the reaper will finalize it", slog.String("tool", meta.Name), slog.Any("error", err))
	}
}

func keyOf(p mcpserver.Principal, m mcpserver.ToolMeta) callKey {
	return callKey{tenant: p.TenantID, user: p.UserID, client: p.ClientID, tool: m.Name}
}

func (g *Gate) pushCall(p mcpserver.Principal, m mcpserver.ToolMeta, callID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	k := keyOf(p, m)
	g.started[k] = append(g.started[k], callID)
}

func (g *Gate) popCall(p mcpserver.Principal, m mcpserver.ToolMeta) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	k := keyOf(p, m)
	q := g.started[k]
	if len(q) == 0 {
		return ""
	}
	id := q[0]
	if len(q) == 1 {
		delete(g.started, k)
	} else {
		g.started[k] = q[1:]
	}
	return id
}

func (g *Gate) rememberPending(id, hash string, p mcpserver.Principal, m mcpserver.ToolMeta, args json.RawMessage) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	if len(g.pending) >= maxPending {
		for k, v := range g.pending {
			if now.Sub(v.at) > pendingTTL {
				delete(g.pending, k)
			}
		}
		if len(g.pending) >= maxPending { // still full: drop the oldest-by-iteration, never grow unbounded
			for k := range g.pending {
				delete(g.pending, k)
				break
			}
		}
	}
	g.pending[id] = pendingApproval{hash: hash, p: p, meta: m, args: append(json.RawMessage(nil), args...), at: now}
}

// zeroUser stands in for "no particular user" when asking what the tenant
// default for a tool is (admin Tools tab, tools/list filtering).
const zeroUser = "00000000-0000-0000-0000-000000000000"

// EffectiveDecision implements mcpserver.ToolPolicyView: the tenant default
// for a tool (role user, no particular client), via the same policy path.
func (g *Gate) EffectiveDecision(ctx context.Context, tenantID string, meta mcpserver.ToolMeta) (mcpserver.EffectiveDecision, error) {
	if g == nil || g.c == nil {
		return mcpserver.EffectiveDecision{}, errors.New("mcppolicy: mcp-service client not configured")
	}
	p := mcpserver.Principal{TenantID: tenantID, UserID: zeroUser, Role: "user"}
	rctx, cancel := g.rpcCtx(ctx, p, g.cfg.CallTimeout)
	defer cancel()
	d, err := g.c.EvaluateToolCall(rctx, &mcpv1.EvaluateToolCallRequest{
		Tool: toolRef(meta), DryRun: true,
		Ctx: &mcpv1.CallContext{TokenKind: "mcp_pat", Scopes: []string{meta.RequiredScope}},
	})
	if err != nil {
		return mcpserver.EffectiveDecision{}, err
	}
	switch d.GetDecision() {
	case mcpserver.OutcomeAllow, mcpserver.OutcomeRequireApproval, mcpserver.OutcomeDeny:
	default:
		return mcpserver.EffectiveDecision{Decision: mcpserver.OutcomeDeny, Source: "default"}, nil
	}
	src := d.GetSource()
	switch src {
	case "default", "tenant_policy", "hard_deny", "kill_switch":
	default:
		src = "default"
	}
	return mcpserver.EffectiveDecision{Decision: d.GetDecision(), Source: src}, nil
}

var (
	_ mcpserver.ElicitationDecider = (*Gate)(nil)
	_ mcpserver.PolicyGate         = (*Gate)(nil)
	_ mcpserver.ToolPolicyView     = (*Gate)(nil)
)
