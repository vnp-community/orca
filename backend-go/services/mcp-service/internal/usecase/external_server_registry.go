package usecase

import (
	"bytes"
	"context"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// maxSecretBytes bounds a secret value; larger input is rejected by field name only.
const maxSecretBytes = 8 << 10

// ExternalServerRegistry implements the six mcp.externalServer.* operations.
// Authorization is evaluated here (defense in depth behind the gateway).
type ExternalServerRegistry struct {
	repo          ExternalServerRepository
	broker        SecretBroker // nil: secrets unavailable (MCP_UNAVAILABLE)
	prober        ToolProber   // nil: http probing unavailable
	policy        domain.ServerPolicy
	stdioFourEyes bool
	clock         Clock
}

type ExternalServerOptions struct {
	Policy        domain.ServerPolicy
	StdioFourEyes bool
}

func NewExternalServerRegistry(repo ExternalServerRepository, broker SecretBroker, prober ToolProber, opts ExternalServerOptions, clock Clock) *ExternalServerRegistry {
	return &ExternalServerRegistry{repo: repo, broker: broker, prober: prober, policy: opts.Policy, stdioFourEyes: opts.StdioFourEyes, clock: clock}
}

func (c callerIdentity) actor() domain.Actor { return domain.Actor{UserID: c.UserID, Role: c.Role} }

func (uc *ExternalServerRegistry) List(ctx context.Context, scope string) ([]domain.ExternalServer, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return nil, err
	}
	switch scope {
	case "", domain.ScopeTenant, domain.ScopeTeam, domain.ScopeUser:
	default:
		return nil, domain.ErrInvalidArgument("unknown scope filter")
	}
	f := ExternalServerFilter{Scope: scope}
	if !id.actor().IsAdmin() {
		f.OwnerUserID = id.UserID
	}
	out, err := uc.repo.ListExternalServers(ctx, id.TenantID, f)
	if err != nil {
		return nil, wrapRepoErr(err, "failed to list external servers")
	}
	return out, nil
}

// UpsertExternalServerInput is the client-controlled part: status, digests and
// hasSecret are never accepted from the caller.
type UpsertExternalServerInput struct {
	ID   string
	Spec domain.ServerSpec
}

func refsFromNames(kind string, names []string, existing []domain.SecretRef) []domain.SecretRef {
	out := make([]domain.SecretRef, 0, len(names))
	for _, n := range names {
		r := domain.SecretRef{Kind: kind, Name: n}
		for _, e := range existing {
			if e.Name == n {
				r = e
			}
		}
		out = append(out, r)
	}
	return out
}

func (uc *ExternalServerRegistry) Upsert(ctx context.Context, in UpsertExternalServerInput) (domain.ExternalServer, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return domain.ExternalServer{}, err
	}
	now := uc.clock.Now()
	sp := in.Spec
	if in.ID == "" {
		return uc.create(ctx, id, sp, now)
	}
	if _, err := uuid.Parse(in.ID); err != nil {
		return domain.ExternalServer{}, domain.ErrNotFound()
	}
	cur, err := uc.repo.GetExternalServer(ctx, id.TenantID, in.ID)
	if err != nil {
		return domain.ExternalServer{}, wrapRepoErr(err, "failed to load external server")
	}
	if err := id.actor().Authorize(domain.ActionWrite, cur.Scope, cur.ScopeID); err != nil {
		return domain.ExternalServer{}, err
	}
	// Scope and scope id are immutable: moving a server between scopes would
	// silently widen who can use it.
	sp.Scope, sp.ScopeID = cur.Scope, cur.ScopeID
	if err := domain.ValidateSpec(sp, uc.policy); err != nil {
		return domain.ExternalServer{}, err
	}
	next := cur
	next.Name, next.Transport, next.URL, next.Command, next.Args = sp.Name, sp.Transport, sp.URL, sp.Command, nonNil(sp.Args)
	next.EnvRefs = refsFromNames(domain.SecretKindEnv, sp.EnvNames, cur.EnvRefs)
	next.HeaderRefs = refsFromNames(domain.SecretKindHeader, sp.HeaderNames, cur.HeaderRefs)
	next.SpecDigest = domain.ComputeSpecDigest(sp.Transport, sp.URL, sp.Command, next.Args, sp.EnvNames, sp.HeaderNames)
	if next.SpecDigest != cur.SpecDigest {
		// A changed launch spec or endpoint needs a fresh review; the approved
		// digest stays as history. Dropping the probe forces a re-probe.
		next.Status = domain.StatusPendingReview
		next.LastProbeDigest, next.LastProbeAt, next.Health = "", nil, nil
	}
	removed := removedRefs(cur, next)
	if err := uc.revokeRefs(ctx, id.TenantID, cur.ID, removed); err != nil {
		return domain.ExternalServer{}, err
	}
	evs, err := uc.upsertEvents(id, next, domain.SubjectExternalServerUpdated, "update", now)
	if err != nil {
		return domain.ExternalServer{}, domain.ErrInternal("failed to build events", err)
	}
	if err := uc.repo.UpdateExternalServer(ctx, next, evs); err != nil {
		return domain.ExternalServer{}, wrapRepoErr(err, "failed to update external server")
	}
	out, err := uc.repo.GetExternalServer(ctx, id.TenantID, next.ID)
	if err != nil {
		return domain.ExternalServer{}, wrapRepoErr(err, "failed to reload external server")
	}
	return out, nil
}

func nonNil(a []string) []string {
	if a == nil {
		return []string{}
	}
	return a
}

func (uc *ExternalServerRegistry) create(ctx context.Context, id callerIdentity, sp domain.ServerSpec, now time.Time) (domain.ExternalServer, error) {
	if err := id.actor().AuthorizeCreate(sp.Scope); err != nil {
		return domain.ExternalServer{}, err
	}
	switch sp.Scope {
	case domain.ScopeUser:
		sp.ScopeID = id.UserID // assigned by the server, never taken from the client
	case domain.ScopeTenant:
		sp.ScopeID = id.TenantID
	case domain.ScopeTeam:
		if _, err := uuid.Parse(sp.ScopeID); err != nil {
			return domain.ExternalServer{}, domain.ErrServerInvalid("team scope requires a team id")
		}
	}
	if err := domain.ValidateSpec(sp, uc.policy); err != nil {
		return domain.ExternalServer{}, err
	}
	s := domain.ExternalServer{
		ID: uuid.NewString(), TenantID: id.TenantID, Scope: sp.Scope, ScopeID: sp.ScopeID, Name: sp.Name,
		Transport: sp.Transport, URL: sp.URL, Command: sp.Command, Args: nonNil(sp.Args),
		EnvRefs:    refsFromNames(domain.SecretKindEnv, sp.EnvNames, nil),
		HeaderRefs: refsFromNames(domain.SecretKindHeader, sp.HeaderNames, nil),
		Status:     domain.StatusPendingReview, CreatedBy: id.UserID, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	s.SpecDigest = domain.ComputeSpecDigest(s.Transport, s.URL, s.Command, s.Args, sp.EnvNames, sp.HeaderNames)
	evs, err := uc.upsertEvents(id, s, domain.SubjectExternalServerCreated, "create", now)
	if err != nil {
		return domain.ExternalServer{}, domain.ErrInternal("failed to build events", err)
	}
	if err := uc.repo.CreateExternalServer(ctx, s, evs); err != nil {
		return domain.ExternalServer{}, wrapRepoErr(err, "failed to create external server")
	}
	return s, nil
}

func removedRefs(cur, next domain.ExternalServer) []domain.SecretRef {
	keep := map[string]bool{}
	for _, r := range append(append([]domain.SecretRef{}, next.EnvRefs...), next.HeaderRefs...) {
		keep[r.Kind+"\x00"+r.Name] = true
	}
	var out []domain.SecretRef
	for _, r := range append(append([]domain.SecretRef{}, cur.EnvRefs...), cur.HeaderRefs...) {
		if !keep[r.Kind+"\x00"+r.Name] && r.HasSecret() {
			out = append(out, r)
		}
	}
	return out
}

// revokeRefs deletes the broker secrets before the registry row changes, so a
// broker outage leaves everything retryable instead of orphaning a secret.
func (uc *ExternalServerRegistry) revokeRefs(ctx context.Context, tenantID, serverID string, refs []domain.SecretRef) error {
	for _, r := range refs {
		if uc.broker == nil {
			return domain.ErrUnavailable("secret store is not configured", nil)
		}
		if err := uc.broker.Delete(ctx, tenantID, r.BrokerOwnerID); err != nil {
			return domain.ErrUnavailable("secret store unavailable", err)
		}
	}
	return nil
}

func (uc *ExternalServerRegistry) upsertEvents(id callerIdentity, s domain.ExternalServer, subject, op string, now time.Time) ([]domain.OutboxRecord, error) {
	ev, err := domain.NewOutboxEvent(uuid.NewString(), subject, id.TenantID, now, map[string]any{
		"server_id": s.ID, "name": s.Name, "scope": s.Scope, "transport": s.Transport, "status": s.Status, "actor_id": id.UserID,
	})
	if err != nil {
		return nil, err
	}
	au, err := domain.NewAdminAuditEvent(uuid.NewString(), uuid.NewString(), id.TenantID, id.UserID, domain.AuditActionExternalServerUpsert,
		"mcp_external_server", s.ID, "allowed", now, map[string]any{"op": op, "name": s.Name, "scope": s.Scope, "transport": s.Transport, "status": s.Status})
	if err != nil {
		return nil, err
	}
	return []domain.OutboxRecord{ev, au}, nil
}

// SetSecretInput.Value is plaintext received once over TLS (decision D1). It is
// handed to the broker and dropped; it must never reach a log, event or error.
type SetSecretInput struct {
	ServerID, Kind, Name string
	Value                domain.SecretValue
}

func (uc *ExternalServerRegistry) SetSecret(ctx context.Context, in SetSecretInput) error {
	defer in.Value.Zero()
	id, err := userCaller(ctx)
	if err != nil {
		return err
	}
	if _, err := uuid.Parse(in.ServerID); err != nil {
		return domain.ErrNotFound()
	}
	s, err := uc.repo.GetExternalServer(ctx, id.TenantID, in.ServerID)
	if err != nil {
		return wrapRepoErr(err, "failed to load external server")
	}
	if err := id.actor().Authorize(domain.ActionWrite, s.Scope, s.ScopeID); err != nil {
		return err
	}
	if in.Kind != domain.SecretKindEnv && in.Kind != domain.SecretKindHeader {
		return domain.ErrInvalidArgument("kind must be env or header")
	}
	if _, ok := s.Ref(in.Kind, in.Name); !ok {
		return domain.ErrInvalidArgument("name is not declared in the server's references")
	}
	n := in.Value.Len()
	if n == 0 {
		return domain.ErrInvalidArgument("value is required")
	}
	if n > maxSecretBytes || !utf8.Valid(in.Value.Reveal()) || bytes.ContainsAny(in.Value.Reveal(), "\r\n\x00") && in.Kind == domain.SecretKindHeader {
		return domain.ErrInvalidArgument("value is not acceptable")
	}
	if uc.broker == nil {
		return domain.ErrUnavailable("secret store is not configured", nil)
	}
	owner := domain.BrokerOwner(s.ID, in.Kind, in.Name)
	if err := uc.broker.Put(ctx, id.TenantID, owner, in.Value); err != nil {
		return domain.ErrUnavailable("secret store unavailable", err)
	}
	now := uc.clock.Now()
	ref := domain.SecretRef{Kind: in.Kind, Name: in.Name, BrokerOwnerID: owner, SetBy: id.UserID, SetAt: &now}
	ev, err := domain.NewOutboxEvent(uuid.NewString(), domain.SubjectExternalServerSecretSet, id.TenantID, now, map[string]any{
		"server_id": s.ID, "kind": in.Kind, "name": in.Name, "actor_id": id.UserID,
	})
	if err != nil {
		return domain.ErrInternal("failed to build events", err)
	}
	au, err := domain.NewAdminAuditEvent(uuid.NewString(), uuid.NewString(), id.TenantID, id.UserID, domain.AuditActionExternalServerSecretSet,
		"mcp_external_server", s.ID, "allowed", now, map[string]any{"kind": in.Kind, "name": in.Name})
	if err != nil {
		return domain.ErrInternal("failed to build events", err)
	}
	if err := uc.repo.SetSecretRef(ctx, id.TenantID, s.ID, ref, []domain.OutboxRecord{ev, au}); err != nil {
		return wrapRepoErr(err, "failed to record secret reference")
	}
	return nil
}

// ProbeOutput is the CONTRACT probe result.
type ProbeOutput struct {
	Transport     string
	Tools         []domain.ToolInfo
	Digest        string
	ApprovedTools []domain.ToolInfo
	ToolsChanged  bool
}

func (uc *ExternalServerRegistry) Probe(ctx context.Context, serverID string) (ProbeOutput, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return ProbeOutput{}, err
	}
	if _, err := uuid.Parse(serverID); err != nil {
		return ProbeOutput{}, domain.ErrNotFound()
	}
	s, err := uc.repo.GetExternalServer(ctx, id.TenantID, serverID)
	if err != nil {
		return ProbeOutput{}, wrapRepoErr(err, "failed to load external server")
	}
	if err := id.actor().Authorize(domain.ActionProbe, s.Scope, s.ScopeID); err != nil {
		return ProbeOutput{}, err
	}
	now := uc.clock.Now()
	tools, digest, perr := uc.observe(ctx, s)
	if perr != nil {
		uc.recordUnhealthy(ctx, s, perr, now)
		return ProbeOutput{}, perr
	}
	evs := []domain.OutboxRecord{}
	changed := s.ApprovedDigest != "" && digest != s.ApprovedDigest
	if changed && s.LastProbeDigest != digest {
		ev, err := domain.NewOutboxEvent(uuid.NewString(), domain.SubjectExternalServerToolsChanged, id.TenantID, now,
			map[string]any{"server_id": s.ID, "name": s.Name, "digest": digest, "approved_digest": s.ApprovedDigest})
		if err != nil {
			return ProbeOutput{}, domain.ErrInternal("failed to build events", err)
		}
		evs = append(evs, ev)
	}
	au, err := domain.NewAdminAuditEvent(uuid.NewString(), uuid.NewString(), id.TenantID, id.UserID, domain.AuditActionExternalServerProbe,
		"mcp_external_server", s.ID, "allowed", now, map[string]any{"digest": digest, "tool_count": len(tools), "tools_changed": changed})
	if err != nil {
		return ProbeOutput{}, domain.ErrInternal("failed to build events", err)
	}
	evs = append(evs, au)
	if err := uc.repo.RecordProbe(ctx, id.TenantID, s.ID, ProbeRecord{Digest: digest, Tools: tools, At: now, Source: "probe"}, evs); err != nil {
		return ProbeOutput{}, wrapRepoErr(err, "failed to record probe")
	}
	return ProbeOutput{Transport: s.Transport, Tools: tools, Digest: digest, ApprovedTools: s.ApprovedTools, ToolsChanged: changed}, nil
}

// observe returns the tools and digest of a server. stdio servers are never
// executed by this service: their "digest" is the launch spec digest.
func (uc *ExternalServerRegistry) observe(ctx context.Context, s domain.ExternalServer) ([]domain.ToolInfo, string, error) {
	if s.Transport == domain.TransportStdio {
		return []domain.ToolInfo{}, s.SpecDigest, nil
	}
	if uc.prober == nil {
		return nil, "", domain.ErrUnavailable("prober is not configured", nil)
	}
	target := ProbeTarget{URL: s.URL, Headers: map[string]domain.SecretValue{}}
	for _, r := range s.HeaderRefs {
		if !r.HasSecret() {
			continue
		}
		if uc.broker == nil {
			return nil, "", domain.ErrUnavailable("secret store is not configured", nil)
		}
		v, err := uc.broker.Get(ctx, s.TenantID, r.BrokerOwnerID)
		if err != nil {
			return nil, "", domain.ErrUnavailable("secret store unavailable", err)
		}
		target.Headers[r.Name] = v
	}
	defer func() {
		for _, v := range target.Headers {
			v.Zero()
		}
	}()
	res, err := uc.prober.ListTools(ctx, target)
	if err != nil {
		return nil, "", classifyProbeError(err)
	}
	return res.Tools, domain.ComputeToolsDigest(res.Tools), nil
}

func classifyProbeError(err error) error {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return err
	}
	var pf domain.ErrProbeFailed
	reason := "unreachable"
	if errors.As(err, &pf) {
		reason = pf.Reason
	}
	return domain.ErrUnavailable("external server probe failed: "+reason, err)
}

func (uc *ExternalServerRegistry) recordUnhealthy(ctx context.Context, s domain.ExternalServer, perr error, now time.Time) {
	var ae *apperrors.AppError
	msg := "probe failed"
	if errors.As(perr, &ae) {
		msg = ae.Message
	}
	// Best effort: the probe error itself is what the caller gets.
	_ = uc.repo.RecordHealth(ctx, s.TenantID, s.ID, domain.Health{OK: false, CheckedAt: now, Error: msg}, nil)
}

type ReviewInput struct {
	ServerID, Decision, ToolsDigest string
}

func (uc *ExternalServerRegistry) Review(ctx context.Context, in ReviewInput) (domain.ExternalServer, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return domain.ExternalServer{}, err
	}
	if _, err := uuid.Parse(in.ServerID); err != nil {
		return domain.ExternalServer{}, domain.ErrNotFound()
	}
	if in.Decision != domain.DecisionApprove && in.Decision != domain.DecisionReject {
		return domain.ExternalServer{}, domain.ErrInvalidArgument("decision must be approve or reject")
	}
	s, err := uc.repo.GetExternalServer(ctx, id.TenantID, in.ServerID)
	if err != nil {
		return domain.ExternalServer{}, wrapRepoErr(err, "failed to load external server")
	}
	if err := id.actor().Authorize(domain.ActionReview, s.Scope, s.ScopeID); err != nil {
		return domain.ExternalServer{}, err
	}
	approve := in.Decision == domain.DecisionApprove
	if approve && s.Transport == domain.TransportStdio {
		if !uc.policy.StdioEnabled {
			return domain.ExternalServer{}, domain.ErrStdioNotAllowed("stdio servers are disabled on this deployment")
		}
		if uc.stdioFourEyes && s.CreatedBy == id.UserID {
			return domain.ExternalServer{}, domain.ErrStdioNotAllowed("a different administrator must review this stdio server")
		}
	}
	if approve && (in.ToolsDigest == "" || s.LastProbeDigest == "" || in.ToolsDigest != s.LastProbeDigest) {
		return domain.ExternalServer{}, domain.ErrDigestMismatch()
	}
	now := uc.clock.Now()
	ev, err := domain.NewOutboxEvent(uuid.NewString(), domain.SubjectExternalServerReviewed, id.TenantID, now, map[string]any{
		"server_id": s.ID, "name": s.Name, "decision": in.Decision, "digest": in.ToolsDigest, "actor_id": id.UserID,
	})
	if err != nil {
		return domain.ExternalServer{}, domain.ErrInternal("failed to build events", err)
	}
	outcome := "allowed"
	if !approve {
		outcome = "denied"
	}
	au, err := domain.NewAdminAuditEvent(uuid.NewString(), uuid.NewString(), id.TenantID, id.UserID, domain.AuditActionExternalServerReview,
		"mcp_external_server", s.ID, outcome, now, map[string]any{"decision": in.Decision, "digest": in.ToolsDigest, "name": s.Name})
	if err != nil {
		return domain.ExternalServer{}, domain.ErrInternal("failed to build events", err)
	}
	out, err := uc.repo.ApplyReview(ctx, ReviewRecord{
		TenantID: id.TenantID, ServerID: s.ID, ReviewerID: id.UserID, Approve: approve, ExpectedDigest: in.ToolsDigest, At: now,
	}, []domain.OutboxRecord{ev, au})
	if err != nil {
		return domain.ExternalServer{}, wrapRepoErr(err, "failed to record review")
	}
	return out, nil
}

func (uc *ExternalServerRegistry) Delete(ctx context.Context, serverID string) error {
	id, err := userCaller(ctx)
	if err != nil {
		return err
	}
	if _, err := uuid.Parse(serverID); err != nil {
		return domain.ErrNotFound()
	}
	s, err := uc.repo.GetExternalServer(ctx, id.TenantID, serverID)
	if err != nil {
		return wrapRepoErr(err, "failed to load external server")
	}
	if err := id.actor().Authorize(domain.ActionWrite, s.Scope, s.ScopeID); err != nil {
		return err
	}
	var held []domain.SecretRef
	for _, r := range append(append([]domain.SecretRef{}, s.EnvRefs...), s.HeaderRefs...) {
		if r.HasSecret() {
			held = append(held, r)
		}
	}
	if err := uc.revokeRefs(ctx, id.TenantID, s.ID, held); err != nil {
		return err
	}
	now := uc.clock.Now()
	ev, err := domain.NewOutboxEvent(uuid.NewString(), domain.SubjectExternalServerDeleted, id.TenantID, now,
		map[string]any{"server_id": s.ID, "name": s.Name, "scope": s.Scope, "actor_id": id.UserID})
	if err != nil {
		return domain.ErrInternal("failed to build events", err)
	}
	au, err := domain.NewAdminAuditEvent(uuid.NewString(), uuid.NewString(), id.TenantID, id.UserID, domain.AuditActionExternalServerDelete,
		"mcp_external_server", s.ID, "allowed", now, map[string]any{"name": s.Name, "scope": s.Scope})
	if err != nil {
		return domain.ErrInternal("failed to build events", err)
	}
	if err := uc.repo.DeleteExternalServer(ctx, id.TenantID, s.ID, []domain.OutboxRecord{ev, au}); err != nil {
		return wrapRepoErr(err, "failed to delete external server")
	}
	return nil
}

func asAppErr(err error) *apperrors.AppError {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae
	}
	return nil
}
