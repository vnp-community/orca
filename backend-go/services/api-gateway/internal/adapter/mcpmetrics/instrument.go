package mcpmetrics

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

const tracerName = "orca/api-gateway/mcp"

func tracer() trace.Tracer { return otel.Tracer(tracerName) }

// callObs carries the policy facts of ONE tools/call from the gate decorator
// to the executor decorator (same request context), so tool_calls_total can
// carry decision without a cross-request lookup table.
type callObs struct {
	decision, risk, namespace string
	decided                   bool
}

type callObsKey struct{}

func obsFrom(ctx context.Context) *callObs {
	o, _ := ctx.Value(callObsKey{}).(*callObs)
	return o
}

var risks = []string{mcpserver.RiskRead, mcpserver.RiskWriteReversible, mcpserver.RiskExec, mcpserver.RiskDestructive, mcpserver.RiskAdmin}

// ---- PolicyGate ----

// InstrumentGate wraps a PolicyGate with the mcp.policy span, denial and
// approval counters. The optional capabilities of the inner gate
// (ToolPolicyView, ElicitationDecider) are preserved exactly: the executor
// and catalog type-assert them, and fabricating one would change behavior.
func InstrumentGate(inner mcpserver.PolicyGate, m *Metrics) mcpserver.PolicyGate {
	if inner == nil || m == nil {
		return inner
	}
	g := &gate{inner: inner, m: m}
	view, hasView := inner.(mcpserver.ToolPolicyView)
	dec, hasDec := inner.(mcpserver.ElicitationDecider)
	switch {
	case hasView && hasDec:
		return struct {
			*gate
			mcpserver.ToolPolicyView
			mcpserver.ElicitationDecider
		}{g, view, dec}
	case hasView:
		return struct {
			*gate
			mcpserver.ToolPolicyView
		}{g, view}
	case hasDec:
		return struct {
			*gate
			mcpserver.ElicitationDecider
		}{g, dec}
	}
	return g
}

type gate struct {
	inner mcpserver.PolicyGate
	m     *Metrics
}

func (g *gate) Decide(ctx context.Context, p mcpserver.Principal, meta mcpserver.ToolMeta, args json.RawMessage) (mcpserver.GateDecision, error) {
	ctx, span := tracer().Start(ctx, "mcp.policy", trace.WithAttributes(
		attribute.String("mcp.tool", meta.Name), attribute.String("mcp.risk", pick(meta.Risk, "unknown", risks...))))
	defer span.End()
	d, err := g.inner.Decide(ctx, p, meta, args)
	decision := "error"
	if err == nil {
		decision = pick(d.Outcome, mcpserver.OutcomeDeny, mcpserver.OutcomeAllow, mcpserver.OutcomeRequireApproval, mcpserver.OutcomeDeny)
	} else {
		span.SetStatus(codes.Error, "policy gate failed")
	}
	span.SetAttributes(attribute.String("mcp.decision", decision))
	if o := obsFrom(ctx); o != nil {
		o.decision, o.risk, o.namespace, o.decided = decision, pick(meta.Risk, "unknown", risks...), meta.Namespace, true
	}
	if decision == mcpserver.OutcomeDeny {
		g.m.policyDenials.WithLabelValues(denialReason(d.Reasons)).Inc()
	}
	return d, err
}

func (g *gate) AwaitApproval(ctx context.Context, p mcpserver.Principal, id string) (bool, error) {
	ok, err := g.inner.AwaitApproval(ctx, p, id)
	switch {
	case err != nil || ctx.Err() != nil:
		g.m.approvals.WithLabelValues("cancelled").Inc()
	case ok:
		g.m.approvals.WithLabelValues("approved").Inc()
	default:
		g.m.approvals.WithLabelValues("denied").Inc()
	}
	return ok, err
}

func (g *gate) Complete(ctx context.Context, p mcpserver.Principal, meta mcpserver.ToolMeta, result string, dur time.Duration) {
	g.inner.Complete(ctx, p, meta, result, dur)
}

// denialReason maps mcp-service reason codes to the closed label set.
func denialReason(reasons []string) string {
	for _, r := range reasons {
		switch {
		case strings.Contains(r, "kill"):
			return "kill_switch"
		case strings.Contains(r, "hard"):
			return "hard_deny"
		case strings.Contains(r, "scope"):
			return "scope"
		}
	}
	return "policy"
}

// ---- ToolExecutor ----

// InstrumentExecutor records tool_calls_total and tool_duration_seconds.
// known reports whether a name is in the catalog (tools.Catalog.Lookup); other
// names are labelled "unknown" so arbitrary client input never becomes a series.
func InstrumentExecutor(inner mcpserver.ToolExecutor, m *Metrics, known func(name string) bool) mcpserver.ToolExecutor {
	if inner == nil || m == nil {
		return inner
	}
	return &executor{inner: inner, m: m, known: known}
}

type executor struct {
	inner mcpserver.ToolExecutor
	m     *Metrics
	known func(string) bool
}

func (e *executor) CallTool(ctx context.Context, p mcpserver.Principal, name string, args json.RawMessage) (*mcp.CallToolResult, error) {
	obs := &callObs{decision: "none"}
	ctx = context.WithValue(ctx, callObsKey{}, obs)
	start := time.Now()
	res, err := e.inner.CallTool(ctx, p, name, args)
	tool := "unknown"
	if e.known != nil && e.known(name) {
		tool = name
	}
	result := classifyResult(ctx, res, err)
	e.m.toolCalls.WithLabelValues(tool, obs.decision, result).Inc()
	if obs.decided && (result == "ok" || result == "error") {
		e.m.toolDuration.WithLabelValues(obs.risk, namespaceLabel(obs.namespace)).Observe(time.Since(start).Seconds())
	}
	return res, err
}

func namespaceLabel(ns string) string {
	if ns == "" || len(ns) > 32 {
		return "unknown"
	}
	return ns
}

func classifyResult(ctx context.Context, res *mcp.CallToolResult, err error) string {
	switch {
	case err == nil && (res == nil || !res.IsError):
		return "ok"
	case err == nil:
		return "error"
	case errors.Is(err, mcpserver.ErrUnknownTool):
		return "unknown_tool"
	case ctx.Err() != nil:
		return "cancelled"
	}
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "MCP_POLICY_DENIED"), strings.HasPrefix(msg, "MCP_APPROVAL_DENIED"),
		strings.HasPrefix(msg, "MCP_SCOPE_NOT_ALLOWED"), strings.HasPrefix(msg, "MCP_KILL_SWITCH_ACTIVE"):
		return "denied"
	case strings.HasPrefix(msg, "INVALID_ARGUMENTS"):
		return "invalid"
	case strings.HasPrefix(msg, "MCP_UNAVAILABLE"):
		return "unavailable"
	}
	return "error"
}

// ---- Dispatcher (mcp.dispatch span) ----

// Dispatcher is the slice of *wscompat.Registry the tool executor needs
// (identical to tools.Dispatcher; redeclared to avoid importing tools).
type Dispatcher interface {
	Dispatch(ctx context.Context, id wscompat.Identity, channel string, args []json.RawMessage) (any, error)
	DispatchStreamChannel(ctx context.Context, id wscompat.Identity, channel string, args []json.RawMessage) (ack any, events <-chan wscompat.PushEvent, ok bool, err error)
}

// TraceDispatcher wraps the channel registry so every MCP-originated channel
// call runs in an mcp.dispatch span that is a child of the request span; the
// gRPC clients below it propagate the trace to the downstream service.
func TraceDispatcher(inner Dispatcher) Dispatcher { return tracedDispatcher{inner} }

type tracedDispatcher struct{ inner Dispatcher }

func (d tracedDispatcher) Dispatch(ctx context.Context, id wscompat.Identity, channel string, args []json.RawMessage) (any, error) {
	ctx, span := tracer().Start(ctx, "mcp.dispatch", trace.WithAttributes(attribute.String("mcp.channel", channel)))
	defer span.End()
	out, err := d.inner.Dispatch(ctx, id, channel, args)
	if err != nil {
		span.SetStatus(codes.Error, "dispatch failed")
	}
	return out, err
}

func (d tracedDispatcher) DispatchStreamChannel(ctx context.Context, id wscompat.Identity, channel string, args []json.RawMessage) (any, <-chan wscompat.PushEvent, bool, error) {
	ctx, span := tracer().Start(ctx, "mcp.dispatch", trace.WithAttributes(attribute.String("mcp.channel", channel), attribute.Bool("mcp.stream", true)))
	defer span.End()
	ack, ev, ok, err := d.inner.DispatchStreamChannel(ctx, id, channel, args)
	if err != nil {
		span.SetStatus(codes.Error, "dispatch failed")
	}
	return ack, ev, ok, err
}
