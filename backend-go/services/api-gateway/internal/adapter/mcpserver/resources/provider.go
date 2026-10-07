package resources

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcppolicy"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/tools"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

// Dispatcher is the slice of *wscompat.Registry reads need.
type Dispatcher interface {
	Dispatch(ctx context.Context, id wscompat.Identity, channel string, args []json.RawMessage) (any, error)
}

// Provider implements mcpserver.ResourceProvider.
type Provider struct {
	disp Dispatcher
	gate mcpserver.PolicyGate
	view mcpserver.ToolPolicyView // optional capability of gate
	cfg  Config
	log  *slog.Logger
	subs *subscriptions
	now  func() time.Time
}

var _ mcpserver.ResourceProvider = (*Provider)(nil)

// NewProvider wires the provider. gate nil = fail-closed default. events nil
// = subscribe is not offered (and not declared).
func NewProvider(d Dispatcher, gate mcpserver.PolicyGate, events TaskEventSource, cfg Config, log *slog.Logger) *Provider {
	if gate == nil {
		gate = mcpserver.FailClosedGate{}
	}
	if log == nil {
		log = slog.Default()
	}
	cfg = cfg.withDefaults()
	p := &Provider{disp: d, gate: gate, cfg: cfg, log: log, now: time.Now}
	p.view, _ = gate.(mcpserver.ToolPolicyView)
	p.subs = newSubscriptions(events, cfg, p.stillAllowed, log)
	return p
}

func (r *Provider) Bind(n mcpserver.ResourceNotifier) { r.subs.bind(n) }
func (r *Provider) Subscribable() bool                { return r.subs.src != nil }

func hasScope(p mcpserver.Principal, scope string) bool {
	for _, s := range p.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

func unavailable() error {
	return &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "resource temporarily unavailable"}
}

// authorize is the tool pipeline's gate step for a read: every failure path
// denies, and every denial is reported as ErrResourceNotFound.
func (r *Provider) authorize(ctx context.Context, p mcpserver.Principal, pl plan, uri string) error {
	if !hasScope(p, pl.meta.RequiredScope) {
		return mcpserver.ErrResourceNotFound
	}
	for _, c := range pl.calls {
		if mcppolicy.NeverDispatch(c.channel) {
			return mcpserver.ErrResourceNotFound
		}
	}
	args, _ := json.Marshal(map[string]string{"uri": uriWithoutQuery(uri)})
	d, err := r.gate.Decide(ctx, p, pl.meta, args)
	if err != nil {
		r.log.WarnContext(ctx, "mcp policy gate failed; denying resource", slog.String("resource", pl.meta.Name), slog.Any("error", err))
		return unavailable()
	}
	switch d.Outcome {
	case mcpserver.OutcomeAllow:
		return nil
	case mcpserver.OutcomeRequireApproval:
		if d.ApprovalID == "" {
			return mcpserver.ErrResourceNotFound
		}
		ok, err := r.gate.AwaitApproval(ctx, p, d.ApprovalID)
		if err != nil {
			return unavailable()
		}
		if !ok {
			return mcpserver.ErrResourceNotFound
		}
		return nil
	default:
		return mcpserver.ErrResourceNotFound
	}
}

func uriWithoutQuery(uri string) string {
	u, _, _ := strings.Cut(uri, "?")
	return u
}

func identityOf(p mcpserver.Principal) wscompat.Identity {
	return wscompat.Identity{TenantID: p.TenantID, UserID: p.UserID, Role: p.Role}
}

// ReadResource: parse -> plan -> gate -> dispatch -> redact/truncate/label.
func (r *Provider) ReadResource(ctx context.Context, p mcpserver.Principal, sessionID, uri string) (*mcp.ReadResourceResult, error) {
	ref, err := Parse(uri)
	if err != nil {
		return nil, mcpserver.ErrResourceNotFound
	}
	return r.read(ctx, p, sessionID, ref)
}

func (r *Provider) read(ctx context.Context, p mcpserver.Principal, sessionID string, ref Ref) (*mcp.ReadResourceResult, error) {
	pl, err := r.planFor(ref)
	if err != nil {
		return nil, mcpserver.ErrResourceNotFound
	}
	ctx = mcppolicy.WithSessionID(ctx, sessionID)
	if err := r.authorize(ctx, p, pl, ref.RawURI); err != nil {
		return nil, err
	}
	start := r.now()
	contents, err := r.fetch(ctx, p, ref, pl)
	outcome := "ok"
	if err != nil {
		outcome = "error"
	}
	// Detached ctx: the audit row must land even if the client cancelled.
	r.gate.Complete(context.WithoutCancel(ctx), p, pl.meta, outcome, r.now().Sub(start))
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{contents}}, nil
}

func (r *Provider) fetch(ctx context.Context, p mcpserver.Principal, ref Ref, pl plan) (*mcp.ResourceContents, error) {
	id := identityOf(p)
	objs := make([]map[string]any, 0, len(pl.calls))
	for _, c := range pl.calls {
		raw, _ := json.Marshal(c.args)
		cctx, cancel := context.WithTimeout(ctx, r.cfg.CallTimeout)
		cctx = tenant.WithActorType(cctx, tenant.ActorAgent)
		res, err := r.disp.Dispatch(cctx, id, c.channel, []json.RawMessage{raw})
		cancel()
		if err != nil {
			return nil, r.mapDispatchError(ctx, err)
		}
		var obj map[string]any
		switch {
		case c.preview:
			obj, err = tools.NormalizeFilePreview(res, 4*r.cfg.MaxBytes)
		case pl.text:
			obj, err = tools.NormalizeChannelResult(res, pl.untrusted, 4*r.cfg.MaxBytes)
		default:
			obj, err = tools.NormalizeChannelResult(res, pl.untrusted, r.cfg.MaxBytes)
		}
		if err != nil {
			return nil, unavailable()
		}
		objs = append(objs, obj)
	}
	rc := &mcp.ResourceContents{URI: ref.RawURI, MIMEType: "application/json"}
	var text string
	truncated := false
	if pl.text {
		t, tr, err := textOf(pl.calls[0], objs[0])
		if errors.Is(err, errBinary) {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "binary resources are not supported"}
		} else if err != nil {
			return nil, unavailable()
		}
		text, truncated = t, tr
		rc.MIMEType = pl.mime
		if t2, cut := cutText(text, r.cfg.MaxBytes, truncationHint(ref)); cut {
			text, truncated = t2, true
		}
	} else {
		doc := any(objs[0])
		if len(objs) > 1 {
			doc = combine(ref.Kind, objs)
		}
		b, err := json.Marshal(doc)
		if err != nil {
			return nil, unavailable()
		}
		text = string(b)
		if t2, cut := cutText(text, r.cfg.MaxBytes, ""); cut { // combined documents can exceed the cap
			text, truncated = t2, true
		}
	}
	meta := mcp.Meta{}
	if truncated {
		meta["truncated"] = true
	}
	if pl.untrusted {
		meta["orca/untrusted"] = true
		text = mcppolicy.WrapUntrusted(string(ref.Kind), text)
	}
	rc.Text = text
	if len(meta) > 0 {
		rc.Meta = meta
	}
	return rc, nil
}

func combine(k Kind, objs []map[string]any) map[string]any {
	switch k {
	case KindProject:
		return map[string]any{"project": objs[0], "worktrees": objs[1]}
	case KindTask:
		return map[string]any{"task": objs[0], "dependencies": objs[1], "comments": objs[2]}
	}
	return map[string]any{"parts": objs}
}

func truncationHint(ref Ref) string {
	if ref.Kind == KindWorktreeFile {
		return "\n[truncated - read more with ?offset=<n>&length=<m>]"
	}
	return "\n[truncated]"
}

// cutText shortens s to max bytes on a line (else rune) boundary.
func cutText(s string, max int, hint string) (string, bool) {
	if len(s) <= max {
		return s, false
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	if i := strings.LastIndexByte(s[:cut], '\n'); i > cut/2 {
		cut = i
	}
	return s[:cut] + hint, true
}

// mapDispatchError never leaks downstream detail. Downstream NotFound /
// PermissionDenied (cross-tenant included) are the same not-found.
func (r *Provider) mapDispatchError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	switch mapped := tools.MapExecError(err); {
	case isCode(mapped, "MCP_NOT_FOUND"), isCode(mapped, "MCP_INVALID_ARGUMENT"), isCode(mapped, "INVALID_ARGUMENTS"):
		return mcpserver.ErrResourceNotFound
	default:
		r.log.WarnContext(ctx, "mcp resource read failed", slog.Any("error", mapped))
		return unavailable()
	}
}

func isCode(err error, code string) bool { return strings.HasPrefix(err.Error(), code+":") }

// stillAllowed re-checks the cheap, side-effect-free part of a subscriber's
// right to a resource before a notification is sent.
func (r *Provider) stillAllowed(ctx context.Context, s *subscription) bool {
	if !s.p.ExpiresAt.IsZero() && !r.now().Before(s.p.ExpiresAt) {
		return false
	}
	if !hasScope(s.p, "orca:read") {
		return false
	}
	if r.view == nil {
		return true
	}
	pl, err := r.planFor(s.ref)
	if err != nil {
		return false
	}
	d, err := r.view.EffectiveDecision(ctx, s.p.TenantID, pl.meta)
	return err == nil && d.Decision != mcpserver.OutcomeDeny
}

// Subscribe authorizes with a real read (same gate, same not-found answer),
// then arms the event source.
func (r *Provider) Subscribe(ctx context.Context, p mcpserver.Principal, sessionID, uri string) error {
	ref, err := Parse(uri)
	if err != nil {
		return mcpserver.ErrResourceNotFound
	}
	if _, err := r.read(ctx, p, sessionID, ref); err != nil {
		return err
	}
	if !subscribable(ref.Kind) || !r.Subscribable() {
		return mcpserver.ErrSubscriptionUnsupported
	}
	return r.subs.add(&subscription{p: p, sessionID: sessionID, ref: ref})
}

func (r *Provider) Unsubscribe(sessionID, uri string) { r.subs.remove(sessionID, uri) }
func (r *Provider) EndSession(sessionID string)       { r.subs.endSession(sessionID) }

// Close stops timers and the event source; call on shutdown.
func (r *Provider) Close() { r.subs.close() }
