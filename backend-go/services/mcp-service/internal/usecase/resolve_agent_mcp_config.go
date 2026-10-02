package usecase

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// RenderedAgentConfig is what a renderer produces for one agent CLI. Files and
// Args never contain secrets; Env only non-secret values (e.g. inline JSON
// built from placeholders).
type RenderedAgentConfig struct {
	Files     []AgentConfigFile
	ExtraArgs []string
	Env       []AgentEnv
	// Unverified marks renderers whose CLI config format is not yet confirmed
	// against the shipped CLI version.
	Unverified bool
}

type AgentConfigFile struct{ Path, Content, Mode string }
type AgentEnv struct {
	Name  string
	Value domain.SecretValue
}

// AgentConfigRenderer turns neutral server entries into one CLI's config.
type AgentConfigRenderer interface {
	Supports(agentKind string) bool
	Render(agentKind, hostOS string, entries []domain.AgentServerEntry) (RenderedAgentConfig, error)
}

// EgressChecker re-validates, at spawn time, that a host still resolves only
// to public addresses (narrows the rebinding window; residual risk R3).
type EgressChecker interface {
	CheckHost(ctx context.Context, host string) error
}

type AgentConfigOptions struct {
	Enabled               bool // MCP_AGENT_CONFIG_ENABLED
	MaxDepth              int  // MCP_MAX_AGENT_DEPTH
	TokenTTL              time.Duration
	TokenScopes           []string
	OrcaMcpURL            string // empty: the "orca" server is never granted
	StdioEnabled          bool
	StdioAllowUnsandboxed bool
	URLPolicy             domain.ExternalURLPolicy
}

type ResolveAgentMcpConfigInput struct {
	UserID, ProjectID, AgentKind, HostOS, HostKind string
	ParentDepth                                    int
	ParentSessionID                                string
}

type AgentSecretEnv struct {
	Name  string
	Value domain.SecretValue
}

// ResolveAgentMcpConfigOutput.Env holds secrets: child process env only.
type ResolveAgentMcpConfigOutput struct {
	Files     []AgentConfigFile
	ExtraArgs []string
	Env       []AgentSecretEnv
	Warnings  []domain.AgentConfigWarning
	Depth     int
}

// ResolveAgentMcpConfig builds the MCP configuration for an agent Orca spawns.
// The user's profile only supplies server NAMES; command/url/env always come
// from the reviewed registry entry (the profile merge in tenant-service is
// untouched and its inline entries are ignored).
type ResolveAgentMcpConfig struct {
	repo     ExternalServerRepository
	broker   SecretBroker
	profile  ProfileMcpReader
	settings TenantSettingsRepository
	defaults Defaults
	tokens   AgentTokenIssuer
	render   AgentConfigRenderer
	egress   EgressChecker
	outbox   OutboxWriter
	opts     AgentConfigOptions
	clock    Clock
}

type ResolveAgentMcpConfigDeps struct {
	Repo     ExternalServerRepository
	Broker   SecretBroker
	Profile  ProfileMcpReader
	Settings TenantSettingsRepository
	Defaults Defaults
	Tokens   AgentTokenIssuer // optional
	Render   AgentConfigRenderer
	Egress   EgressChecker // optional
	Outbox   OutboxWriter
}

func NewResolveAgentMcpConfig(d ResolveAgentMcpConfigDeps, opts AgentConfigOptions, clock Clock) *ResolveAgentMcpConfig {
	return &ResolveAgentMcpConfig{repo: d.Repo, broker: d.Broker, profile: d.Profile, settings: d.Settings, defaults: d.Defaults,
		tokens: d.Tokens, render: d.Render, egress: d.Egress, outbox: d.Outbox, opts: opts, clock: clock}
}

func (uc *ResolveAgentMcpConfig) Execute(ctx context.Context, in ResolveAgentMcpConfigInput) (ResolveAgentMcpConfigOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ResolveAgentMcpConfigOutput{}, domain.ErrNoTenant(err)
	}
	if in.UserID == "" {
		return ResolveAgentMcpConfigOutput{}, domain.ErrInvalidArgument("user_id is required")
	}
	out := ResolveAgentMcpConfigOutput{Depth: in.ParentDepth + 1}
	if !uc.opts.Enabled {
		return out, nil
	}
	st, err := uc.settings.GetOrCreateTenantSettings(ctx, uc.defaults.For(tenantID))
	if err != nil {
		return out, wrapRepoErr(err, "failed to load tenant settings")
	}
	if st.KillSwitch.Active {
		return ResolveAgentMcpConfigOutput{Depth: out.Depth}, domain.ErrKillSwitchActive()
	}
	if !st.Enabled {
		return out, nil
	}
	if !uc.render.Supports(in.AgentKind) {
		out.Warnings = append(out.Warnings, domain.AgentConfigWarning{Reason: domain.WarnAgentUnsupported})
		return out, nil
	}

	names, err := uc.profile.ServerNames(ctx, in.UserID)
	if err != nil {
		return out, domain.ErrUnavailable("profile service unavailable", err)
	}
	teams, err := uc.profile.TeamIDs(ctx, in.UserID)
	if err != nil {
		return out, domain.ErrUnavailable("profile service unavailable", err)
	}
	b := &agentBuild{uc: uc, tenantID: tenantID, in: in, out: &out}
	if err := b.addRegistryServers(ctx, names, teams); err != nil {
		return out, err
	}
	b.addOrcaServer(ctx)
	if len(b.entries) == 0 {
		return out, nil
	}
	r, err := uc.render.Render(in.AgentKind, in.HostOS, b.entries)
	if err != nil {
		return out, domain.ErrInternal("failed to render agent config", err)
	}
	out.Files, out.ExtraArgs = r.Files, r.ExtraArgs
	for _, e := range r.Env {
		out.Env = append(out.Env, AgentSecretEnv{Name: e.Name, Value: e.Value})
	}
	out.Env = append(out.Env, b.env...)
	if r.Unverified {
		out.Warnings = append(out.Warnings, domain.AgentConfigWarning{
			Reason: domain.WarnConfigUnverified + ": " + in.AgentKind + " (chưa xác minh với phiên bản CLI đang ship)"})
	}
	uc.emitResolved(ctx, tenantID, in, b.entries, out)
	return out, nil
}

type agentBuild struct {
	uc       *ResolveAgentMcpConfig
	tenantID string
	in       ResolveAgentMcpConfigInput
	out      *ResolveAgentMcpConfigOutput
	entries  []domain.AgentServerEntry
	env      []AgentSecretEnv
	n        int
}

func (b *agentBuild) warn(name, reason string) {
	b.out.Warnings = append(b.out.Warnings, domain.AgentConfigWarning{ServerName: name, Reason: reason})
}

func (b *agentBuild) nextVar() string {
	b.n++
	return fmt.Sprintf("ORCA_MCP_%d", b.n)
}

// priority: user-owned beats team beats tenant; foreign user/team entries are not candidates.
func priority(s domain.ExternalServer, userID string, teams map[string]bool) int {
	switch {
	case s.Scope == domain.ScopeUser && s.ScopeID == userID:
		return 3
	case s.Scope == domain.ScopeTeam && teams[s.ScopeID]:
		return 2
	case s.Scope == domain.ScopeTenant:
		return 1
	}
	return 0
}

func skipReason(s domain.ExternalServer) string {
	switch {
	case s.Status == domain.StatusDisabled:
		return domain.WarnDisabled
	case s.Status == domain.StatusPendingReview:
		return domain.WarnPendingReview
	case s.ToolsChanged():
		return domain.WarnToolsChanged
	}
	return ""
}

func (b *agentBuild) addRegistryServers(ctx context.Context, names, teamIDs []string) error {
	uniq := map[string]bool{}
	var wanted []string
	for _, n := range names {
		if n != "" && !uniq[n] {
			uniq[n] = true
			wanted = append(wanted, n)
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	teams := map[string]bool{}
	for _, t := range teamIDs {
		teams[t] = true
	}
	all, err := b.uc.repo.ListServersByName(ctx, b.tenantID, wanted)
	if err != nil {
		return wrapRepoErr(err, "failed to load external servers")
	}
	byName := map[string][]domain.ExternalServer{}
	for _, s := range all {
		if priority(s, b.in.UserID, teams) > 0 {
			byName[s.Name] = append(byName[s.Name], s)
		}
	}
	for _, name := range wanted {
		cands := byName[name]
		if len(cands) == 0 {
			b.warn(name, domain.WarnNotInRegistry)
			continue
		}
		sort.SliceStable(cands, func(i, j int) bool {
			return priority(cands[i], b.in.UserID, teams) > priority(cands[j], b.in.UserID, teams)
		})
		var chosen *domain.ExternalServer
		for i := range cands {
			if skipReason(cands[i]) == "" {
				chosen = &cands[i]
				break
			}
		}
		if chosen == nil {
			b.warn(name, skipReason(cands[0]))
			continue
		}
		b.addServer(ctx, *chosen)
	}
	return nil
}

func (b *agentBuild) addServer(ctx context.Context, s domain.ExternalServer) {
	e := domain.AgentServerEntry{Name: s.Name, Transport: s.Transport}
	var env []AgentSecretEnv
	collect := func(refs []domain.SecretRef) ([]domain.EnvBinding, bool) {
		var bs []domain.EnvBinding
		for _, r := range refs {
			if !r.HasSecret() || b.uc.broker == nil {
				return nil, false
			}
			v, err := b.uc.broker.Get(ctx, b.tenantID, r.BrokerOwnerID)
			if err != nil {
				return nil, false
			}
			name := fmt.Sprintf("ORCA_MCP_%d", b.n+len(env)+1)
			env = append(env, AgentSecretEnv{Name: name, Value: v})
			bs = append(bs, domain.EnvBinding{Name: r.Name, Var: name})
		}
		return bs, true
	}
	var ok bool
	switch s.Transport {
	case domain.TransportHTTP:
		if _, err := domain.ValidateExternalURL(s.URL, b.uc.opts.URLPolicy); err != nil {
			b.warn(s.Name, domain.WarnSSRFBlocked)
			return
		}
		if b.uc.egress != nil && !b.uc.opts.URLPolicy.HTTPAllowed(hostPortOf(s.URL)) {
			if err := b.uc.egress.CheckHost(ctx, hostOf(s.URL)); err != nil {
				b.warn(s.Name, domain.WarnSSRFBlocked)
				return
			}
		}
		e.URL = s.URL
		if e.Headers, ok = collect(s.HeaderRefs); !ok {
			b.warn(s.Name, domain.WarnSecretUnresolvable)
			return
		}
	case domain.TransportStdio:
		// No launcher/sandbox exists on hosts yet (D3): only an explicit
		// operator opt-in lets stdio servers reach an agent.
		if !b.uc.opts.StdioEnabled || !b.uc.opts.StdioAllowUnsandboxed {
			b.warn(s.Name, domain.WarnStdioUnsupported)
			return
		}
		e.Command, e.Args = "orca-mcp-launch", append([]string{"--", s.Command}, s.Args...)
		if e.Env, ok = collect(s.EnvRefs); !ok {
			b.warn(s.Name, domain.WarnSecretUnresolvable)
			return
		}
	default:
		b.warn(s.Name, domain.WarnStdioUnsupported)
		return
	}
	b.n += len(env)
	b.env = append(b.env, env...)
	b.entries = append(b.entries, e)
}

func (b *agentBuild) addOrcaServer(ctx context.Context) {
	if b.uc.opts.OrcaMcpURL == "" || b.uc.tokens == nil {
		return
	}
	if b.in.ParentDepth+1 > b.uc.opts.MaxDepth {
		b.warn("orca", domain.WarnDepthExceeded)
		return
	}
	tok, err := b.uc.tokens.Issue(ctx, b.in.UserID, "agent:"+b.in.AgentKind, b.uc.opts.TokenScopes, b.uc.opts.TokenTTL)
	if err != nil {
		b.warn("orca", domain.WarnOrcaTokenUnavailable)
		return
	}
	// The token goes only to the Orca server entry, never to external servers
	// (no token passthrough).
	b.env = append(b.env,
		AgentSecretEnv{Name: "ORCA_MCP_TOKEN", Value: tok},
		AgentSecretEnv{Name: "ORCA_MCP_DEPTH", Value: domain.NewSecretValue(fmt.Sprint(b.out.Depth))})
	if b.in.ParentSessionID != "" {
		b.env = append(b.env, AgentSecretEnv{Name: "ORCA_MCP_PARENT_SESSION", Value: domain.NewSecretValue(b.in.ParentSessionID)})
	}
	b.entries = append(b.entries, domain.AgentServerEntry{Name: "orca", Transport: domain.TransportHTTP, URL: b.uc.opts.OrcaMcpURL, BearerVar: "ORCA_MCP_TOKEN"})
}

func (uc *ResolveAgentMcpConfig) emitResolved(ctx context.Context, tenantID string, in ResolveAgentMcpConfigInput, entries []domain.AgentServerEntry, out ResolveAgentMcpConfigOutput) {
	if uc.outbox == nil {
		return
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
	}
	ev, err := domain.NewOutboxEvent(uuid.NewString(), domain.SubjectAgentConfigResolved, tenantID, uc.clock.Now(), map[string]any{
		"user_id": in.UserID, "agent_kind": in.AgentKind, "servers": names, "count": len(entries), "skipped": len(out.Warnings), "mcp_depth": out.Depth,
	})
	if err == nil {
		_ = uc.outbox.EnqueueOutbox(ctx, tenantID, ev) // audit is best effort; spawn must not fail on it
	}
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func hostPortOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	port := u.Port()
	if port == "" {
		port = map[string]string{"https": "443", "http": "80"}[u.Scheme]
	}
	return net.JoinHostPort(u.Hostname(), port)
}
