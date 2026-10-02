package mcpserver

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// sessionHook runs the session-lifecycle side effects of protocol methods
// (BE-MCP-SOL-004). A non-nil error is returned to the client as-is.
func (e *engine) sessionHook(ctx context.Context, method string, req mcp.Request) error {
	if e.host == nil {
		return nil
	}
	ls := e.host.local(req.GetSession().ID())
	switch method {
	case "initialize":
		return e.host.persistInitialize(ctx, ls, req)
	case "notifications/initialized":
		if ls != nil {
			ls.mu.Lock()
			ls.ready = true
			ls.mu.Unlock()
			e.host.touch(ctx, ls.principal(), ls, true)
		}
	case "notifications/cancelled":
		// Cancellation of a request running on ANOTHER replica: the SDK only
		// cancels requests of its own connection, so broadcast unless this
		// message itself came from a peer.
		if ls == nil || isInjected(req) {
			return nil
		}
		if p, ok := req.GetParams().(*mcp.CancelledParams); ok && p != nil {
			if rid, err := json.Marshal(p.RequestID); err == nil {
				e.host.signal(ls.row(), sessionSignal{T: "cancel", RID: rid})
			}
		}
	case "tools/call":
		if ls != nil {
			ls.mu.Lock()
			ls.calls++
			ls.mu.Unlock()
		}
	}
	return nil
}

func isInjected(req mcp.Request) bool {
	x := req.GetExtra()
	return x != nil && x.Header.Get(injectedHeader) != ""
}

func (s *sessionHost) persistInitialize(ctx context.Context, ls *localSession, req mcp.Request) error {
	p, ok := principalOf(req)
	ip, _ := req.GetParams().(*mcp.InitializeParams)
	if ls == nil || !ok || ip == nil {
		return &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
	}
	ns := NewSession{ProtocolVersion: Negotiate(ip.ProtocolVersion)}
	if ip.ClientInfo != nil {
		ns.ClientName, ns.ClientVersion = ip.ClientInfo.Name, ip.ClientInfo.Version
	}
	if ip.Capabilities != nil {
		ns.Capabilities, _ = json.Marshal(ip.Capabilities)
	}
	sctx, cancel := storeCtx(ctx)
	defer cancel()
	rec, err := s.store.Create(sctx, p, SecretHash(ls.secret), ns)
	if err != nil {
		s.log.ErrorContext(ctx, "mcp session not persisted", slog.Any("error", err))
		return &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
	}
	s.setRow(ls, rec.ID)
	if s.srec != nil {
		s.srec.SessionOpened()
	}
	return nil
}

// toolContext adds the verified facts the executor and the policy gate need:
// session row id, client name, token depth/root (via Deps.RequestContext),
// the elicitation hook when the client supports it, and progress reporting.
func (e *engine) toolContext(ctx context.Context, req mcp.Request, p Principal, params *mcp.CallToolParamsRaw) context.Context {
	ss, _ := req.GetSession().(*mcp.ServerSession)
	info := RequestInfo{Depth: p.Depth, Root: p.Root}
	if e.host != nil {
		if ls := e.host.local(req.GetSession().ID()); ls != nil {
			info.SessionID = ls.row()
		}
	}
	var ip *mcp.InitializeParams
	if ss != nil {
		ip = ss.InitializeParams()
	}
	if ip != nil && ip.ClientInfo != nil {
		info.ClientName = ip.ClientInfo.Name
	}
	if e.host != nil && e.host.d.RequestContext != nil {
		ctx = e.host.d.RequestContext(ctx, p, info)
	}
	if ss != nil && ip != nil && ip.Capabilities != nil && ip.Capabilities.Elicitation != nil {
		ctx = WithElicitor(ctx, func(ectx context.Context, msg string) (ElicitDecision, error) {
			res, err := ss.Elicit(ectx, &mcp.ElicitParams{Mode: "form", Message: msg, RequestedSchema: approvalSchema})
			if err != nil {
				return ElicitDecision{}, err
			}
			ok, _ := res.Content["approve"].(bool)
			return ElicitDecision{Action: res.Action, Approve: ok}, nil
		})
	}
	if tok := params.GetProgressToken(); tok != nil && ss != nil {
		ctx = WithProgress(ctx, func(progress, total float64, msg string) {
			_ = ss.NotifyProgress(ctx, &mcp.ProgressNotificationParams{ProgressToken: tok, Progress: progress, Total: total, Message: msg})
		})
	}
	return ctx
}

// approvalSchema is the fixed, non-sensitive form of an approval elicitation.
var approvalSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"approve": map[string]any{"type": "boolean", "title": "Approve this action"},
	},
	"required": []string{"approve"},
}

// trackRequest makes the request context die with the session: the SDK's
// Close waits for in-flight handlers instead of cancelling them, so DELETE,
// idle expiry and kill signals must cancel running tools themselves.
func (e *engine) trackRequest(ctx context.Context, req mcp.Request) (context.Context, func()) {
	if e.host == nil {
		return ctx, func() {}
	}
	ls := e.host.local(req.GetSession().ID())
	if ls == nil {
		return ctx, func() {}
	}
	return ls.track(ctx)
}
