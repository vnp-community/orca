package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
	tt "github.com/stablyai/orca-go/services/mcp-service/internal/usecase/usecasetest"
)

const (
	tenantA = "11111111-1111-1111-1111-111111111111"
	tenantB = "22222222-2222-2222-2222-222222222222"
	userU   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	userAdm = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

func pol(id, decision string, m domain.ToolPolicyMatch) domain.ToolPolicy {
	return domain.ToolPolicy{ID: id, Version: 1, Match: m, Decision: decision}
}

type matrixCase struct {
	name     string
	tool     domain.ToolRef
	mutate   func(*tt.Harness, *domain.CallContext)
	role     string
	policies []domain.ToolPolicy
	want     string
}

func scopesOnly(s ...string) func(*tt.Harness, *domain.CallContext) {
	return func(_ *tt.Harness, c *domain.CallContext) { c.Scopes = s }
}

func withTool(t domain.ToolRef, f func(*domain.ToolRef)) domain.ToolRef { f(&t); return t }

func matrix() []matrixCase {
	userRole, adminRole := "user", "admin"
	cases := []matrixCase{
		{"read allowed", tt.ToolRead, nil, userRole, nil, "allow"},
		{"write allowed", tt.ToolWrite, nil, userRole, nil, "allow"},
		{"exec needs approval", tt.ToolExec, nil, userRole, nil, "require_approval"},
		{"destructive needs approval", tt.ToolDestructive, nil, userRole, nil, "require_approval"},
		{"admin risk denied for user", tt.ToolAdmin, nil, userRole, nil, "deny"},
		{"admin risk denied for admin by default", tt.ToolAdmin, nil, adminRole, nil, "deny"},
		{"admin risk allowed for admin with exact policy", tt.ToolAdmin, nil, adminRole, []domain.ToolPolicy{pol("p", "allow", domain.ToolPolicyMatch{Tool: "tenant_update"})}, "allow"},
		{"admin risk still denied for user with allow policy", tt.ToolAdmin, nil, userRole, []domain.ToolPolicy{pol("p", "allow", domain.ToolPolicyMatch{Tool: "tenant_update"})}, "deny"},
		{"unknown risk denied", withTool(tt.ToolRead, func(t *domain.ToolRef) { t.Risk = "mystery" }), nil, userRole, nil, "deny"},
		{"missing risk denied", withTool(tt.ToolRead, func(t *domain.ToolRef) { t.Risk = "" }), nil, userRole, nil, "deny"},
		{"missing channel denied", withTool(tt.ToolRead, func(t *domain.ToolRef) { t.Channel = "" }), nil, userRole, nil, "deny"},
		{"missing scope denied", tt.ToolWrite, scopesOnly("orca:read"), userRole, nil, "deny"},
		{"no scopes denied", tt.ToolRead, scopesOnly(), userRole, nil, "deny"},
		{"exec scope missing denied", tt.ToolExec, scopesOnly("orca:read", "orca:write"), userRole, nil, "deny"},
		{"hard deny credentials", tt.ToolHardDenied, nil, userRole, nil, "deny"},
		{"hard deny credentials even with allow policy", tt.ToolHardDenied, nil, adminRole, []domain.ToolPolicy{pol("p", "allow", domain.ToolPolicyMatch{Namespace: "credentials"})}, "deny"},
		{"hard deny mcp.approval.decide", withTool(tt.ToolRead, func(t *domain.ToolRef) {
			t.Name, t.Channel, t.Namespace = "mcp_approval_decide", "mcp.approval.decide", "mcp"
		}), nil, adminRole, nil, "deny"},
		{"hard deny admin.createUser", withTool(tt.ToolWrite, func(t *domain.ToolRef) {
			t.Name, t.Channel, t.Namespace = "admin_createUser", "admin.createUser", "admin"
		}), nil, adminRole, nil, "deny"},
		{"hard deny startAuthLogin", withTool(tt.ToolWrite, func(t *domain.ToolRef) { t.Name, t.Channel = "github_startAuthLogin", "github.startAuthLogin" }), nil, userRole, nil, "deny"},
		{"hard deny devServer.agentTokens.create", withTool(tt.ToolWrite, func(t *domain.ToolRef) { t.Channel = "devServer.agentTokens.create" }), nil, adminRole, nil, "deny"},
		{"policy deny read", tt.ToolRead, nil, userRole, []domain.ToolPolicy{pol("p", "deny", domain.ToolPolicyMatch{Tool: "task_list"})}, "deny"},
		{"policy approval for write", tt.ToolWrite, nil, userRole, []domain.ToolPolicy{pol("p", "require_approval", domain.ToolPolicyMatch{Namespace: "task"})}, "require_approval"},
		{"policy deny beats allow", tt.ToolWrite, nil, userRole, []domain.ToolPolicy{pol("a", "allow", domain.ToolPolicyMatch{Tool: "task_create"}), pol("d", "deny", domain.ToolPolicyMatch{Namespace: "task"})}, "deny"},
		{"policy approval beats allow", tt.ToolWrite, nil, userRole, []domain.ToolPolicy{pol("a", "allow", domain.ToolPolicyMatch{Tool: "task_create"}), pol("r", "require_approval", domain.ToolPolicyMatch{Risk: "write_reversible"})}, "require_approval"},
		{"policy for other client ignored", tt.ToolRead, nil, userRole, []domain.ToolPolicy{pol("p", "deny", domain.ToolPolicyMatch{ClientID: "other"})}, "allow"},
		{"policy for this client applies", tt.ToolRead, nil, userRole, []domain.ToolPolicy{pol("p", "deny", domain.ToolPolicyMatch{ClientID: "c1"})}, "deny"},
		{"policy roles admin ignored for user", tt.ToolRead, nil, userRole, []domain.ToolPolicy{pol("p", "deny", domain.ToolPolicyMatch{Roles: []string{"admin"}})}, "allow"},
		{"policy roles user applies", tt.ToolRead, nil, userRole, []domain.ToolPolicy{pol("p", "deny", domain.ToolPolicyMatch{Roles: []string{"user"}})}, "deny"},
		{"wildcard exec allow clamped", tt.ToolExec, nil, userRole, []domain.ToolPolicy{pol("p", "allow", domain.ToolPolicyMatch{Risk: "exec"})}, "require_approval"},
		{"namespace destructive allow clamped", tt.ToolDestructive, nil, userRole, []domain.ToolPolicy{pol("p", "allow", domain.ToolPolicyMatch{Namespace: "worktree"})}, "require_approval"},
		{"exact exec allow honored", tt.ToolExec, nil, userRole, []domain.ToolPolicy{pol("p", "allow", domain.ToolPolicyMatch{Tool: "terminal_send"})}, "allow"},
		{"exec deny policy", tt.ToolExec, nil, userRole, []domain.ToolPolicy{pol("p", "deny", domain.ToolPolicyMatch{Tool: "terminal_send"})}, "deny"},
		{"client blocked", tt.ToolRead, func(h *tt.Harness, _ *domain.CallContext) { h.Clients.Set("c1", "blocked") }, userRole, nil, "deny"},
		{"client pending", tt.ToolRead, func(h *tt.Harness, _ *domain.CallContext) { h.Clients.Set("c1", "pending") }, userRole, nil, "deny"},
		{"unknown client denied", tt.ToolRead, func(_ *tt.Harness, c *domain.CallContext) { c.ClientID = "ghost" }, userRole, nil, "deny"},
		{"PAT bypasses client registry", tt.ToolRead, func(_ *tt.Harness, c *domain.CallContext) { c.ClientID, c.TokenKind = "", "mcp_pat" }, userRole, nil, "allow"},
		{"tenant disabled", tt.ToolRead, func(h *tt.Harness, _ *domain.CallContext) {
			f := false
			_, _ = h.Settings.Set(tt.Ctx(tenantA, userAdm, "admin"), usecase.SettingsPatch{Enabled: &f})
		}, userRole, nil, "deny"},
		{"kill switch tenant", tt.ToolRead, func(h *tt.Harness, _ *domain.CallContext) {
			_ = h.KillAdmin.Set(tt.Ctx(tenantA, userAdm, "admin"), usecase.SetKillSwitchInput{Scope: "tenant", Reason: "incident", Active: true})
		}, userRole, nil, "deny"},
		{"kill switch client", tt.ToolRead, func(h *tt.Harness, _ *domain.CallContext) {
			_ = h.KillAdmin.Set(tt.Ctx(tenantA, userAdm, "admin"), usecase.SetKillSwitchInput{Scope: "client", TargetID: "c1", Reason: "incident", Active: true})
		}, userRole, nil, "deny"},
		{"kill switch other client no effect", tt.ToolRead, func(h *tt.Harness, _ *domain.CallContext) {
			h.Clients.Set("c2", "allowed")
			_ = h.KillAdmin.Set(tt.Ctx(tenantA, userAdm, "admin"), usecase.SetKillSwitchInput{Scope: "client", TargetID: "c2", Reason: "incident", Active: true})
		}, userRole, nil, "allow"},
		{"kill switch session", tt.ToolRead, func(h *tt.Harness, _ *domain.CallContext) {
			_ = h.KillAdmin.Set(tt.Ctx(tenantA, userAdm, "admin"), usecase.SetKillSwitchInput{Scope: "session", TargetID: "sess-1", Reason: "incident", Active: true})
		}, userRole, nil, "deny"},
		{"kill switch released", tt.ToolRead, func(h *tt.Harness, _ *domain.CallContext) {
			c := tt.Ctx(tenantA, userAdm, "admin")
			_ = h.KillAdmin.Set(c, usecase.SetKillSwitchInput{Scope: "tenant", Reason: "incident", Active: true})
			_ = h.KillAdmin.Set(c, usecase.SetKillSwitchInput{Scope: "tenant", Reason: "all clear", Active: false})
		}, userRole, nil, "allow"},
		{"taint escalates open-world write", withTool(tt.ToolWrite, func(t *domain.ToolRef) { t.OpenWorld = true }), func(h *tt.Harness, _ *domain.CallContext) {
			h.Store.Taint[tenantA+"|"+userU+"|c1"] = h.Clock.Now().Add(1e9 * 60)
		}, userRole, nil, "require_approval"},
		{"taint ignored for closed-world", tt.ToolWrite, func(h *tt.Harness, _ *domain.CallContext) {
			h.Store.Taint[tenantA+"|"+userU+"|c1"] = h.Clock.Now().Add(1e9 * 60)
		}, userRole, nil, "allow"},
		{"expired taint ignored", withTool(tt.ToolWrite, func(t *domain.ToolRef) { t.OpenWorld = true }), func(h *tt.Harness, _ *domain.CallContext) {
			h.Store.Taint[tenantA+"|"+userU+"|c1"] = h.Clock.Now().Add(-1e9)
		}, userRole, nil, "allow"},
		{"open-world clean session allowed", withTool(tt.ToolWrite, func(t *domain.ToolRef) { t.OpenWorld = true }), nil, userRole, nil, "allow"},
		{"spawn at max depth denied", withTool(tt.ToolExec, func(t *domain.ToolRef) { t.SpawnsProcess = true }), func(_ *tt.Harness, c *domain.CallContext) { c.Depth = 1 }, userRole, nil, "deny"},
		{"spawn below max depth needs approval", withTool(tt.ToolExec, func(t *domain.ToolRef) { t.SpawnsProcess = true }), nil, userRole, nil, "require_approval"},
		{"non-spawning at high depth unaffected", tt.ToolRead, func(_ *tt.Harness, c *domain.CallContext) { c.Depth = 9 }, userRole, nil, "allow"},
	}
	return cases
}

func TestDecisionMatrix(t *testing.T) {
	cases := matrix()
	if len(cases) < 40 {
		t.Fatalf("matrix has %d cases, want >= 40", len(cases))
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := tt.NewHarness(true)
			cc := tt.CC()
			if len(c.policies) > 0 {
				h.Store.Policies[tenantA] = c.policies
			}
			if c.mutate != nil {
				c.mutate(h, &cc)
			}
			got, err := h.Evaluate.Execute(tt.Ctx(tenantA, userU, c.role), c.tool, cc)
			if err != nil {
				t.Fatal(err)
			}
			if got.Decision != c.want {
				t.Fatalf("decision = %s (source %s, reasons %v), want %s", got.Decision, got.Source, got.Reasons, c.want)
			}
		})
	}
}

// Any failure of the policy path must end in deny, never allow.
func TestEngineFailuresDeny(t *testing.T) {
	h := tt.NewHarnessWithEngine(true, tt.NewClock(), failingEngine{})
	got, err := h.Evaluate.Execute(tt.Ctx(tenantA, userU, "user"), tt.ToolRead, tt.CC())
	if err != nil || got.Decision != "deny" {
		t.Fatalf("engine error must deny, got %+v err=%v", got, err)
	}
	out, err := h.Authorize.Execute(tt.Ctx(tenantA, userU, "user"), usecase.AuthorizeInput{Tool: tt.ToolRead, Ctx: tt.CC()})
	if err != nil || out.Outcome != "deny" || out.CallID != "" {
		t.Fatalf("authorize must deny when engine fails: %+v %v", out, err)
	}
}

type failingEngine struct{}

func (failingEngine) Evaluate(context.Context, domain.PolicyInput) domain.PolicyDecision {
	return domain.DenyUnavailable()
}
func (failingEngine) HardDenySets(context.Context) ([]string, []string, error) {
	return nil, nil, context.DeadlineExceeded
}
func (failingEngine) IsHardDenied(context.Context, string) (bool, error) {
	return false, context.DeadlineExceeded
}

// D6: the tenant default flag changes `enabled` and nothing else.
func TestTenantDefaultEnabledTrue_DoesNotChangeRiskDefaults(t *testing.T) {
	tools := []domain.ToolRef{tt.ToolRead, tt.ToolWrite, tt.ToolExec, tt.ToolDestructive, tt.ToolAdmin, tt.ToolHardDenied}
	results := map[bool][]string{}
	for _, def := range []bool{true, false} {
		h := tt.NewHarness(def)
		if def == false {
			tr := true // enable explicitly so only the default differs
			_, _ = h.Settings.Set(tt.Ctx(tenantA, userAdm, "admin"), usecase.SettingsPatch{Enabled: &tr})
		}
		for _, tool := range tools {
			d, err := h.Evaluate.Execute(tt.Ctx(tenantA, userAdm, "admin"), tool, tt.CC())
			if err != nil {
				t.Fatal(err)
			}
			results[def] = append(results[def], d.Decision)
		}
		snap, _ := h.Core.KillState(tt.Ctx(tenantA, userU, "user"), tenantA)
		if len(snap.Entries) != 0 {
			t.Fatal("kill switch must start clear")
		}
	}
	want := []string{"allow", "allow", "require_approval", "require_approval", "deny", "deny"}
	for def, got := range results {
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("default=%v tool %s: got %s want %s", def, tools[i].Name, got[i], want[i])
			}
		}
	}
	// `enabled` is the only difference between a fresh true and a fresh false tenant.
	on, off := tt.NewHarness(true), tt.NewHarness(false)
	a, _ := on.Settings.Get(tt.Ctx(tenantA, userAdm, "admin"))
	b, _ := off.Settings.Get(tt.Ctx(tenantA, userAdm, "admin"))
	if !a.Enabled || b.Enabled {
		t.Fatalf("enabled must follow the default: %v %v", a.Enabled, b.Enabled)
	}
	a.Enabled, b.Enabled = false, false
	a.UpdatedAt, b.UpdatedAt = time.Time{}, time.Time{}
	if a != b {
		t.Fatalf("settings differ beyond enabled: %+v vs %+v", a, b)
	}
}

func TestTenantSettings_LazyCreateUsesConfigDefault(t *testing.T) {
	for _, def := range []bool{true, false} {
		h := tt.NewHarness(def)
		s, err := h.Settings.Get(tt.Ctx(tenantA, userAdm, "admin"))
		if err != nil || s.Enabled != def {
			t.Fatalf("default=%v got %+v err=%v", def, s, err)
		}
	}
}

func TestFilterToolsHidesDeny_CallStillBlocked(t *testing.T) {
	h := tt.NewHarness(true)
	h.Store.Policies[tenantA] = []domain.ToolPolicy{pol("p", "deny", domain.ToolPolicyMatch{Tool: "task_list"})}
	ctx := tt.Ctx(tenantA, userU, "user")
	ds, _, err := h.Filter.Execute(ctx, []domain.ToolRef{tt.ToolRead, tt.ToolWrite, tt.ToolExec, tt.ToolHardDenied}, tt.CC())
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, d := range ds {
		got[d.Name] = d.Decision.Decision
	}
	if got["task_list"] != "deny" || got["credentials_get"] != "deny" || got["task_create"] != "allow" || got["terminal_send"] != "require_approval" {
		t.Fatalf("filter decisions: %v", got)
	}
	out, err := h.Authorize.Execute(ctx, usecase.AuthorizeInput{Tool: tt.ToolRead, Ctx: tt.CC()})
	if err != nil || out.Outcome != "deny" || out.CallID != "" {
		t.Fatalf("direct call to hidden tool must still be blocked: %+v %v", out, err)
	}
}
