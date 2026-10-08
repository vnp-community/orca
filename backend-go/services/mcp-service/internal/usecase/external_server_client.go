package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// Limits of external calls (BE-REQ-SOL-031 section E).
const (
	DefaultExternalMaxBytes = 64 << 10
	MaxExternalMaxBytes     = 1 << 20
	MaxExternalArgsBytes    = 64 << 10
	MaxExternalURIBytes     = 2048
)

// ExternalServerClient runs read-only calls on approved external MCP servers
// for internal callers (request-service). Every check fails closed.
type ExternalServerClient struct {
	repo   ExternalServerRepository
	broker SecretBroker // nil: header secrets unavailable
	caller ToolCaller   // nil: calls unavailable
	outbox OutboxWriter // nil: no audit (tests only)
	clock  Clock
}

func NewExternalServerClient(repo ExternalServerRepository, broker SecretBroker, caller ToolCaller, outbox OutboxWriter, clock Clock) *ExternalServerClient {
	return &ExternalServerClient{repo: repo, broker: broker, caller: caller, outbox: outbox, clock: clock}
}

type CallToolInput struct {
	ServerID, Tool string
	ArgumentsJSON  []byte
	MaxBytes       int
}

type CallToolOutput struct {
	Text      string
	IsError   bool
	Truncated bool
	SizeBytes int
	Digest    string
}

type ReadResourceInput struct {
	ServerID, URI string
	MaxBytes      int
}

type ReadResourceOutput struct {
	Text, MimeType string
	Truncated      bool
	SizeBytes      int
	Digest         string
}

func (uc *ExternalServerClient) CallTool(ctx context.Context, in CallToolInput) (CallToolOutput, error) {
	id, s, maxBytes, err := uc.begin(ctx, in.ServerID, in.MaxBytes)
	if err != nil {
		return CallToolOutput{}, err
	}
	meta := map[string]any{"tool": in.Tool, "server_id": s.ID}
	deny := func(reason string, err error) (CallToolOutput, error) {
		meta["reason"] = reason
		uc.audit(ctx, id, domain.AuditActionExternalCall, s.ID, "denied", meta)
		return CallToolOutput{}, err
	}
	if err := checkUsable(s); err != nil {
		return deny("not_usable", err)
	}
	if !approvedTool(s, in.Tool) {
		return deny("tool_not_approved", domain.ErrToolNotApproved())
	}
	if len(in.ArgumentsJSON) > MaxExternalArgsBytes || (len(in.ArgumentsJSON) > 0 && !json.Valid(in.ArgumentsJSON)) {
		return deny("bad_arguments", domain.ErrInvalidArgument("arguments_json must be valid JSON of at most 65536 bytes"))
	}
	if len(in.ArgumentsJSON) > 0 {
		if err := domain.ValidateEgressArgs(in.ArgumentsJSON, domain.SecretRedactor{}); err != nil {
			return deny("egress_secret", domain.ErrInvalidArgument("arguments_json carries a credential and was not sent"))
		}
	}
	target, err := uc.target(ctx, s)
	if err != nil {
		return deny("secret_unavailable", err)
	}
	defer zeroHeaders(target)
	res, err := uc.caller.CallTool(ctx, target, in.Tool, in.ArgumentsJSON, maxBytes)
	if err != nil {
		meta["reason"] = "upstream_failed"
		uc.audit(ctx, id, domain.AuditActionExternalCall, s.ID, "error", meta)
		return CallToolOutput{}, classifyProbeError(err)
	}
	text, _ := domain.SecretRedactor{}.Redact(res.Text)
	text = clipUTF8(text, maxBytes)
	meta["bytes"], meta["truncated"], meta["is_error"] = len(text), res.Truncated, res.IsError
	uc.audit(ctx, id, domain.AuditActionExternalCall, s.ID, "allowed", meta)
	return CallToolOutput{Text: text, IsError: res.IsError, Truncated: res.Truncated, SizeBytes: len(text), Digest: sha256Hex(text)}, nil
}

func (uc *ExternalServerClient) ReadResource(ctx context.Context, in ReadResourceInput) (ReadResourceOutput, error) {
	id, s, maxBytes, err := uc.begin(ctx, in.ServerID, in.MaxBytes)
	if err != nil {
		return ReadResourceOutput{}, err
	}
	// The raw uri may carry a token in its query: only its digest is audited.
	meta := map[string]any{"uri_digest": sha256Hex(in.URI), "server_id": s.ID}
	deny := func(reason string, err error) (ReadResourceOutput, error) {
		meta["reason"] = reason
		uc.audit(ctx, id, domain.AuditActionExternalRead, s.ID, "denied", meta)
		return ReadResourceOutput{}, err
	}
	if err := checkUsable(s); err != nil {
		return deny("not_usable", err)
	}
	if err := validateResourceURI(in.URI); err != nil {
		return deny("bad_uri", err)
	}
	target, err := uc.target(ctx, s)
	if err != nil {
		return deny("secret_unavailable", err)
	}
	defer zeroHeaders(target)
	res, err := uc.caller.ReadResource(ctx, target, in.URI, maxBytes)
	if err != nil {
		meta["reason"] = "upstream_failed"
		uc.audit(ctx, id, domain.AuditActionExternalRead, s.ID, "error", meta)
		return ReadResourceOutput{}, classifyProbeError(err)
	}
	text, _ := domain.SecretRedactor{}.Redact(res.Text)
	text = clipUTF8(text, maxBytes)
	meta["bytes"], meta["truncated"] = len(text), res.Truncated
	uc.audit(ctx, id, domain.AuditActionExternalRead, s.ID, "allowed", meta)
	return ReadResourceOutput{Text: text, MimeType: res.MimeType, Truncated: res.Truncated, SizeBytes: len(text), Digest: sha256Hex(text)}, nil
}

// begin does the checks that precede any audit: caller, ids, size, server load.
func (uc *ExternalServerClient) begin(ctx context.Context, serverID string, maxBytes int) (callerIdentity, domain.ExternalServer, int, error) {
	id, err := tenantCaller(ctx)
	if err != nil {
		return id, domain.ExternalServer{}, 0, err
	}
	if _, err := uuid.Parse(serverID); err != nil {
		return id, domain.ExternalServer{}, 0, domain.ErrNotFound()
	}
	if maxBytes < 0 {
		return id, domain.ExternalServer{}, 0, domain.ErrInvalidArgument("max_bytes must not be negative")
	}
	if maxBytes == 0 {
		maxBytes = DefaultExternalMaxBytes
	}
	if maxBytes > MaxExternalMaxBytes {
		maxBytes = MaxExternalMaxBytes
	}
	if uc.caller == nil {
		return id, domain.ExternalServer{}, 0, domain.ErrUnavailable("external caller is not configured", nil)
	}
	s, err := uc.repo.GetExternalServer(ctx, id.TenantID, serverID)
	if err != nil {
		return id, s, 0, wrapRepoErr(err, "failed to load external server")
	}
	return id, s, maxBytes, nil
}

// checkUsable: Usable() already covers pending, disabled and rug-pull; a call
// never re-probes, so it cannot refresh the digest itself.
func checkUsable(s domain.ExternalServer) error {
	if s.Transport != domain.TransportHTTP {
		return domain.ErrServerNotUsable("only http servers can be called")
	}
	if !s.Usable() {
		return domain.ErrServerNotUsable("server is not approved or its tools changed")
	}
	return nil
}

func approvedTool(s domain.ExternalServer, tool string) bool {
	for _, t := range s.ApprovedTools {
		if t.Name == tool {
			return true
		}
	}
	return false
}

func validateResourceURI(raw string) error {
	if raw == "" || len(raw) > MaxExternalURIBytes || !utf8.ValidString(raw) {
		return domain.ErrInvalidArgument("uri is empty, too long or not UTF-8")
	}
	for _, r := range raw {
		if r < 0x20 || r == 0x7f {
			return domain.ErrInvalidArgument("uri contains control characters")
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return domain.ErrInvalidArgument("uri must have a scheme")
	}
	if u.User != nil {
		return domain.ErrInvalidArgument("uri must not carry credentials")
	}
	if _, changed := (domain.SecretRedactor{}).Redact(raw); changed {
		return domain.ErrInvalidArgument("uri carries a credential and was not sent")
	}
	return nil
}

func (uc *ExternalServerClient) target(ctx context.Context, s domain.ExternalServer) (CallTarget, error) {
	t := CallTarget{URL: s.URL, Headers: map[string]domain.SecretValue{}}
	for _, r := range s.HeaderRefs {
		if !r.HasSecret() {
			continue
		}
		if uc.broker == nil {
			return t, domain.ErrUnavailable("secret store is not configured", nil)
		}
		v, err := uc.broker.Get(ctx, s.TenantID, r.BrokerOwnerID)
		if err != nil {
			zeroHeaders(t)
			return CallTarget{}, domain.ErrUnavailable("secret store unavailable", err)
		}
		t.Headers[r.Name] = v
	}
	return t, nil
}

func zeroHeaders(t CallTarget) {
	for _, v := range t.Headers {
		v.Zero()
	}
}

// audit is best effort: the call already happened and its result must not be
// lost; metadata never holds arguments or result content.
func (uc *ExternalServerClient) audit(ctx context.Context, id callerIdentity, action, serverID, outcome string, meta map[string]any) {
	if uc.outbox == nil {
		return
	}
	ev, err := domain.NewExternalCallAuditEvent(uuid.NewString(), uuid.NewString(), id.TenantID, id.UserID, tenant.ActorType(ctx), action, serverID, outcome, uc.clock.Now(), meta)
	if err != nil {
		return
	}
	_ = uc.outbox.EnqueueOutbox(ctx, id.TenantID, ev)
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// clipUTF8 re-applies the byte cap after redaction (the marker can be longer
// than the secret it replaced).
func clipUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
