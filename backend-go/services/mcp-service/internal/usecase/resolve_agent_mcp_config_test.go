package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

type fakeProfile struct {
	names []string
	teams []string
	err   error
}

func (f fakeProfile) ServerNames(context.Context, string) ([]string, error) { return f.names, f.err }
func (f fakeProfile) TeamIDs(context.Context, string) ([]string, error)     { return f.teams, f.err }

type fakeTokens struct {
	err   error
	calls int
}

func (f *fakeTokens) Issue(context.Context, string, string, []string, time.Duration) (domain.SecretValue, error) {
	f.calls++
	return domain.NewSecretValue("omp_ORCA_TOKEN"), f.err
}

// entryRenderer prints neutral entries; the real per-CLI formats are tested in
// adapter/agentconfig (importing it here would be an import cycle).
type entryRenderer struct{}

func (entryRenderer) Supports(k string) bool {
	return k == "claude" || k == "codex" || k == "gemini" || k == "opencode"
}

func (entryRenderer) Render(_, _ string, es []domain.AgentServerEntry) (RenderedAgentConfig, error) {
	var b strings.Builder
	for _, e := range es {
		fmt.Fprintf(&b, "%q url=%s cmd=%s args=%v", e.Name, e.URL, e.Command, e.Args)
		for _, h := range e.Headers {
			fmt.Fprintf(&b, " hdr:%s=${%s}", h.Name, h.Var)
		}
		if e.BearerVar != "" {
			fmt.Fprintf(&b, " bearer=${%s}", e.BearerVar)
		}
		b.WriteString("\n")
	}
	return RenderedAgentConfig{Files: []AgentConfigFile{{Path: "cfg", Content: b.String(), Mode: "0600"}}, Unverified: true}, nil
}

type blockHosts map[string]bool

func (b blockHosts) CheckHost(_ context.Context, h string) error {
	if b[h] {
		return domain.ErrSSRFBlocked("destination address is not allowed")
	}
	return nil
}

type resolveEnv struct {
	repo     *memRepo
	broker   *memBroker
	tokens   *fakeTokens
	settings *fakeRepo
	opts     AgentConfigOptions
	profile  fakeProfile
	egress   EgressChecker
}

func newResolveEnv() *resolveEnv {
	return &resolveEnv{repo: newMemRepo(), broker: newMemBroker(), tokens: &fakeTokens{}, settings: newFakeRepo(),
		opts: AgentConfigOptions{Enabled: true, MaxDepth: 2, TokenTTL: time.Hour, TokenScopes: []string{"orca:read"}, OrcaMcpURL: "https://orca.example.com/mcp"}}
}

func (e *resolveEnv) uc() *ResolveAgentMcpConfig {
	return NewResolveAgentMcpConfig(ResolveAgentMcpConfigDeps{
		Repo: e.repo, Broker: e.broker, Profile: e.profile, Settings: e.settings, Defaults: Defaults{TenantEnabled: true, MaxTokenDays: 90},
		Tokens: e.tokens, Render: entryRenderer{}, Egress: e.egress, Outbox: e.repo,
	}, e.opts, extClock{time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)})
}

var nextID = 0

func (e *resolveEnv) add(scope, scopeID, name, status string, mut func(*domain.ExternalServer)) domain.ExternalServer {
	nextID++
	s := domain.ExternalServer{
		ID: "00000000-0000-4000-8000-" + strings.Repeat("0", 11) + string(rune('a'+nextID)), TenantID: tenA, Scope: scope, ScopeID: scopeID, Name: name,
		Transport: domain.TransportHTTP, URL: "https://" + scope + "." + name + ".example.com/mcp", Status: status, CreatedBy: adm,
		LastProbeDigest: "d1", ApprovedDigest: "d1",
	}
	if mut != nil {
		mut(&s)
	}
	e.repo.put(s)
	return s
}

func run(t *testing.T, e *resolveEnv, in ResolveAgentMcpConfigInput) ResolveAgentMcpConfigOutput {
	t.Helper()
	if in.UserID == "" {
		in.UserID = usr
	}
	if in.AgentKind == "" {
		in.AgentKind = "claude"
	}
	out, err := e.uc().Execute(ctxAs(tenA, "", ""), in)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func warned(out ResolveAgentMcpConfigOutput, name, reason string) bool {
	for _, w := range out.Warnings {
		if w.ServerName == name && w.Reason == reason {
			return true
		}
	}
	return false
}

func fileText(out ResolveAgentMcpConfigOutput) string {
	var b strings.Builder
	for _, f := range out.Files {
		b.WriteString(f.Content)
	}
	b.WriteString(strings.Join(out.ExtraArgs, " "))
	return b.String()
}

func TestResolve_SkipsPendingDisabledChangedAndUnknown(t *testing.T) {
	e := newResolveEnv()
	e.profile = fakeProfile{names: []string{"ok", "pend", "dis", "chg", "ghost"}}
	e.add(domain.ScopeTenant, tenA, "ok", domain.StatusApproved, nil)
	e.add(domain.ScopeTenant, tenA, "pend", domain.StatusPendingReview, nil)
	e.add(domain.ScopeTenant, tenA, "dis", domain.StatusDisabled, nil)
	e.add(domain.ScopeTenant, tenA, "chg", domain.StatusApproved, func(s *domain.ExternalServer) { s.LastProbeDigest = "d2" })
	out := run(t, e, ResolveAgentMcpConfigInput{})
	txt := fileText(out)
	if !strings.Contains(txt, `"ok"`) || strings.Contains(txt, `"pend"`) || strings.Contains(txt, `"dis"`) || strings.Contains(txt, `"chg"`) {
		t.Fatalf("config: %s", txt)
	}
	for name, reason := range map[string]string{"pend": domain.WarnPendingReview, "dis": domain.WarnDisabled, "chg": domain.WarnToolsChanged, "ghost": domain.WarnNotInRegistry} {
		if !warned(out, name, reason) {
			t.Errorf("missing warning %s/%s: %+v", name, reason, out.Warnings)
		}
	}
}

func TestResolve_UserBeatsTeamBeatsTenant_AndForeignEntriesIgnored(t *testing.T) {
	e := newResolveEnv()
	e.profile = fakeProfile{names: []string{"x", "y"}, teams: []string{team1}}
	e.add(domain.ScopeTenant, tenA, "x", domain.StatusApproved, nil)
	e.add(domain.ScopeTeam, team1, "x", domain.StatusApproved, nil)
	e.add(domain.ScopeUser, usr, "x", domain.StatusApproved, nil)
	e.add(domain.ScopeUser, usr2, "y", domain.StatusApproved, nil)                                   // someone else's private server
	e.add(domain.ScopeTeam, "99999999-0000-4000-8000-000000000009", "y", domain.StatusApproved, nil) // a team the user is not in
	out := run(t, e, ResolveAgentMcpConfigInput{})
	txt := fileText(out)
	if !strings.Contains(txt, "user.x.example.com") || strings.Contains(txt, "team.x.") || strings.Contains(txt, "tenant.x.") {
		t.Fatalf("user entry must win: %s", txt)
	}
	if strings.Contains(txt, ".y.example.com") || !warned(out, "y", domain.WarnNotInRegistry) {
		t.Fatalf("foreign entries must be invisible: %s %+v", txt, out.Warnings)
	}
	// team beats tenant when no user entry exists
	e2 := newResolveEnv()
	e2.profile = fakeProfile{names: []string{"x"}, teams: []string{team1}}
	e2.add(domain.ScopeTenant, tenA, "x", domain.StatusApproved, nil)
	e2.add(domain.ScopeTeam, team1, "x", domain.StatusApproved, nil)
	if txt := fileText(run(t, e2, ResolveAgentMcpConfigInput{})); !strings.Contains(txt, "team.x.example.com") {
		t.Fatalf("team must beat tenant: %s", txt)
	}
	// a pending user override does not silently block the approved tenant entry
	e3 := newResolveEnv()
	e3.profile = fakeProfile{names: []string{"x"}}
	e3.add(domain.ScopeTenant, tenA, "x", domain.StatusApproved, nil)
	e3.add(domain.ScopeUser, usr, "x", domain.StatusPendingReview, nil)
	if txt := fileText(run(t, e3, ResolveAgentMcpConfigInput{})); !strings.Contains(txt, "tenant.x.example.com") || strings.Contains(txt, "user.x.") {
		t.Fatalf("fallback to usable entry: %s", txt)
	}
}

func TestResolve_IgnoresInlineProfileCommand(t *testing.T) {
	// The profile reader only exposes names (tenant-service's inline command/env
	// never reach this usecase); a name that is not in the registry yields nothing.
	e := newResolveEnv()
	e.profile = fakeProfile{names: []string{"inline-evil"}}
	out := run(t, e, ResolveAgentMcpConfigInput{})
	if strings.Contains(fileText(out), "inline-evil") && !strings.Contains(fileText(out), "orca") {
		t.Fatal("unregistered names must not be granted")
	}
	if !warned(out, "inline-evil", domain.WarnNotInRegistry) {
		t.Fatalf("%+v", out.Warnings)
	}
}

func TestResolve_SecretOnlyInEnv(t *testing.T) {
	const secret = "hdr-SECRET-VALUE-123"
	e := newResolveEnv()
	e.profile = fakeProfile{names: []string{"api"}}
	s := e.add(domain.ScopeTenant, tenA, "api", domain.StatusApproved, func(s *domain.ExternalServer) {
		owner := domain.BrokerOwner(s.ID, "header", "X-Api-Key")
		s.HeaderRefs = []domain.SecretRef{{Kind: "header", Name: "X-Api-Key", BrokerOwnerID: owner}}
	})
	e.broker.secrets[s.HeaderRefs[0].BrokerOwnerID] = secret
	for _, kind := range []string{"claude"} {
		out := run(t, e, ResolveAgentMcpConfigInput{AgentKind: kind})
		all := fileText(out)
		for _, ev := range out.Env {
			if string(ev.Value.Reveal()) == secret {
				continue
			}
			all += ev.Name + "=" + string(ev.Value.Reveal())
		}
		if strings.Contains(all, secret) {
			t.Errorf("%s: secret value leaked into files/args/non-secret env: %s", kind, all)
		}
		found := false
		for _, ev := range out.Env {
			found = found || string(ev.Value.Reveal()) == secret
		}
		if !found {
			t.Errorf("%s: secret must be delivered via env", kind)
		}
		if !warned(out, "", domain.WarnConfigUnverified+": "+kind+" (chưa xác minh với phiên bản CLI đang ship)") {
			t.Errorf("%s: missing unverified-format marker: %+v", kind, out.Warnings)
		}
	}
}

func TestResolve_UnresolvableOrUnsetSecretFailsClosed(t *testing.T) {
	e := newResolveEnv()
	e.profile = fakeProfile{names: []string{"unset", "gone"}}
	e.add(domain.ScopeTenant, tenA, "unset", domain.StatusApproved, func(s *domain.ExternalServer) {
		s.HeaderRefs = []domain.SecretRef{{Kind: "header", Name: "K"}}
	})
	e.add(domain.ScopeTenant, tenA, "gone", domain.StatusApproved, func(s *domain.ExternalServer) {
		s.HeaderRefs = []domain.SecretRef{{Kind: "header", Name: "K", BrokerOwnerID: "mcp:missing"}}
	})
	out := run(t, e, ResolveAgentMcpConfigInput{})
	if !warned(out, "unset", domain.WarnSecretUnresolvable) || !warned(out, "gone", domain.WarnSecretUnresolvable) {
		t.Fatalf("%+v", out.Warnings)
	}
}

func TestResolve_DepthExceededOmitsOrcaButKeepsExternal(t *testing.T) {
	e := newResolveEnv()
	e.profile = fakeProfile{names: []string{"ext"}}
	e.add(domain.ScopeTenant, tenA, "ext", domain.StatusApproved, nil)

	ok := run(t, e, ResolveAgentMcpConfigInput{ParentDepth: 1}) // 1+1 == max 2
	if ok.Depth != 2 || !strings.Contains(fileText(ok), `"orca"`) {
		t.Fatalf("depth 2 allowed: %d %s", ok.Depth, fileText(ok))
	}
	deep := run(t, e, ResolveAgentMcpConfigInput{ParentDepth: 2})
	if deep.Depth != 3 || strings.Contains(fileText(deep), `"orca"`) || !strings.Contains(fileText(deep), `"ext"`) || !warned(deep, "orca", domain.WarnDepthExceeded) {
		t.Fatalf("depth 3: %d %s %+v", deep.Depth, fileText(deep), deep.Warnings)
	}
	zero := run(t, e, ResolveAgentMcpConfigInput{})
	var depthEnv string
	for _, ev := range zero.Env {
		if ev.Name == "ORCA_MCP_DEPTH" {
			depthEnv = string(ev.Value.Reveal())
		}
	}
	if zero.Depth != 1 || depthEnv != "1" {
		t.Fatalf("depth env: %d %q", zero.Depth, depthEnv)
	}
}

func TestResolve_OrcaTokenNeverGoesToExternalServers(t *testing.T) {
	e := newResolveEnv()
	e.profile = fakeProfile{names: []string{"ext"}}
	e.add(domain.ScopeTenant, tenA, "ext", domain.StatusApproved, nil)
	out := run(t, e, ResolveAgentMcpConfigInput{})
	txt := fileText(out)
	if strings.Contains(txt, "omp_ORCA_TOKEN") {
		t.Fatal("token value must never be rendered")
	}
	// exactly one server entry references the token variable: orca
	if strings.Count(txt, "bearer=") != 1 {
		t.Fatalf("token var must appear only on the orca entry: %s", txt)
	}
}

func TestResolve_TokenFailureDegradesToWarning(t *testing.T) {
	e := newResolveEnv()
	e.tokens.err = errors.New("auth down")
	e.profile = fakeProfile{names: []string{"ext"}}
	e.add(domain.ScopeTenant, tenA, "ext", domain.StatusApproved, nil)
	out := run(t, e, ResolveAgentMcpConfigInput{})
	if !warned(out, "orca", domain.WarnOrcaTokenUnavailable) || !strings.Contains(fileText(out), `"ext"`) {
		t.Fatalf("%+v %s", out.Warnings, fileText(out))
	}
}

func TestResolve_KillSwitchDisabledAndUnsupportedAgent(t *testing.T) {
	e := newResolveEnv()
	e.profile = fakeProfile{names: []string{"ext"}}
	e.add(domain.ScopeTenant, tenA, "ext", domain.StatusApproved, nil)

	s := Defaults{TenantEnabled: true, MaxTokenDays: 90}.For(tenA)
	s.KillSwitch.Active = true
	e.settings.rows[tenA] = s
	out, err := e.uc().Execute(ctxAs(tenA, "", ""), ResolveAgentMcpConfigInput{UserID: usr, AgentKind: "claude"})
	if code(err) != domain.CodeKillSwitchActive || len(out.Files)+len(out.Env)+len(out.ExtraArgs) != 0 {
		t.Fatalf("kill switch: %v %+v", err, out)
	}
	s.KillSwitch.Active, s.Enabled = false, false
	e.settings.rows[tenA] = s
	if out := run(t, e, ResolveAgentMcpConfigInput{}); len(out.Files) != 0 {
		t.Fatal("tenant disabled -> nothing granted")
	}
	e.settings.rows[tenA] = Defaults{TenantEnabled: true, MaxTokenDays: 90}.For(tenA)
	if out := run(t, e, ResolveAgentMcpConfigInput{AgentKind: "ollama"}); len(out.Files) != 0 || !warned(out, "", domain.WarnAgentUnsupported) {
		t.Fatalf("unsupported agent: %+v", out)
	}
	e.opts.Enabled = false
	if out := run(t, e, ResolveAgentMcpConfigInput{}); len(out.Files)+len(out.Env) != 0 {
		t.Fatal("flag off -> nothing granted")
	}
}

func TestResolve_StdioNeedsOptInAndIsWrappedInLauncher(t *testing.T) {
	e := newResolveEnv()
	e.profile = fakeProfile{names: []string{"local"}}
	e.add(domain.ScopeTenant, tenA, "local", domain.StatusApproved, func(s *domain.ExternalServer) {
		s.Transport, s.URL, s.Command, s.Args = domain.TransportStdio, "", "npx", []string{"-y", "pkg@1.2.3"}
	})
	if out := run(t, e, ResolveAgentMcpConfigInput{}); !warned(out, "local", domain.WarnStdioUnsupported) {
		t.Fatalf("default: %+v", out.Warnings)
	}
	e.opts.StdioEnabled, e.opts.StdioAllowUnsandboxed = true, true
	txt := fileText(run(t, e, ResolveAgentMcpConfigInput{}))
	if !strings.Contains(txt, "orca-mcp-launch") || !strings.Contains(txt, "pkg@1.2.3") {
		t.Fatalf("%s", txt)
	}
}

func TestResolve_SSRFRevalidatedAtSpawn(t *testing.T) {
	e := newResolveEnv()
	e.profile = fakeProfile{names: []string{"rebound", "bad", "fine"}}
	e.add(domain.ScopeTenant, tenA, "rebound", domain.StatusApproved, func(s *domain.ExternalServer) { s.URL = "https://rebound.example.com/mcp" })
	e.add(domain.ScopeTenant, tenA, "bad", domain.StatusApproved, func(s *domain.ExternalServer) { s.URL = "https://169.254.169.254/x" })
	e.add(domain.ScopeTenant, tenA, "fine", domain.StatusApproved, func(s *domain.ExternalServer) { s.URL = "https://fine.example.com/mcp" })
	e.egress = blockHosts{"rebound.example.com": true}
	out := run(t, e, ResolveAgentMcpConfigInput{})
	if !warned(out, "rebound", domain.WarnSSRFBlocked) || !warned(out, "bad", domain.WarnSSRFBlocked) {
		t.Fatalf("%+v", out.Warnings)
	}
	if !strings.Contains(fileText(out), `"fine"`) {
		t.Fatal("healthy server must still be granted")
	}
}

func TestResolve_EventHasNoSecretsAndProfileFailureIsUnavailable(t *testing.T) {
	e := newResolveEnv()
	e.profile = fakeProfile{names: []string{"api"}}
	s := e.add(domain.ScopeTenant, tenA, "api", domain.StatusApproved, func(s *domain.ExternalServer) {
		s.HeaderRefs = []domain.SecretRef{{Kind: "header", Name: "K", BrokerOwnerID: "o"}}
	})
	_ = s
	e.broker.secrets["o"] = "TOP-SECRET-XYZ"
	run(t, e, ResolveAgentMcpConfigInput{})
	var n int
	for _, ev := range e.repo.events {
		if ev.Subject == domain.SubjectAgentConfigResolved {
			n++
			if strings.Contains(string(ev.PayloadJSON), "TOP-SECRET") || strings.Contains(string(ev.PayloadJSON), "omp_") {
				t.Fatalf("event leaks secret: %s", ev.PayloadJSON)
			}
		}
	}
	if n != 1 {
		t.Fatalf("want one agentconfig.resolved event, got %d", n)
	}
	e.profile = fakeProfile{err: errors.New("down")}
	if _, err := e.uc().Execute(ctxAs(tenA, "", ""), ResolveAgentMcpConfigInput{UserID: usr, AgentKind: "claude"}); code(err) != domain.CodeUnavailable {
		t.Fatalf("%v", err)
	}
}
