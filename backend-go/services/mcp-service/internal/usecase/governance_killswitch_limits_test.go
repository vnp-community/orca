package usecase_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
	tt "github.com/stablyai/orca-go/services/mcp-service/internal/usecase/usecasetest"
)

func TestKillSwitchAdminRules(t *testing.T) {
	h := tt.NewHarness(true)
	user, adm := tt.Ctx(tenantA, userU, "user"), tt.Ctx(tenantA, userAdm, "admin")
	if err := h.KillAdmin.Set(user, usecase.SetKillSwitchInput{Scope: "tenant", Reason: "nope", Active: true}); code(err) != domain.CodeNotAdmin {
		t.Fatalf("non-admin: %v", err)
	}
	if _, err := h.KillAdmin.List(user); code(err) != domain.CodeNotAdmin {
		t.Fatalf("non-admin list: %v", err)
	}
	for name, in := range map[string]usecase.SetKillSwitchInput{
		"short reason":   {Scope: "tenant", Reason: "x", Active: true},
		"bad scope":      {Scope: "planet", Reason: "reason", Active: true},
		"client no id":   {Scope: "client", Reason: "reason", Active: true},
		"unknown grant":  {Scope: "grant", TargetID: "dddddddd-dddd-dddd-dddd-dddddddddddd", Reason: "reason", Active: true},
		"unknown client": {Scope: "client", TargetID: "ghost", Reason: "reason", Active: true},
	} {
		if err := h.KillAdmin.Set(adm, in); err == nil {
			t.Errorf("%s must be rejected", name)
		}
	}
	// Reason is required even to switch it off.
	if err := h.KillAdmin.Set(adm, usecase.SetKillSwitchInput{Scope: "tenant", Reason: "", Active: false}); err == nil {
		t.Error("deactivation needs a reason")
	}
	if err := h.KillAdmin.Set(adm, usecase.SetKillSwitchInput{Scope: "tenant", Reason: "incident", Active: true}); err != nil {
		t.Fatal(err)
	}
	list, _ := h.KillAdmin.List(adm)
	if len(list) != 1 || list[0].Scope != "tenant" || list[0].SetBy != userAdm {
		t.Fatalf("%+v", list)
	}
	s, _ := h.Settings.Get(adm)
	if !s.KillSwitch.Active {
		t.Fatal("settings.get must report the tenant kill switch")
	}
	if len(h.Store.EventsOf(domain.SubjectKillSwitchChanged)) != 1 || len(h.Store.EventsOf(domain.SubjectAuditAppended)) != 1 {
		t.Fatal("expected one killswitch.changed and one audit event")
	}
}

func TestKillStateRPCAnswersPerScope(t *testing.T) {
	h := tt.NewHarness(true)
	adm := tt.Ctx(tenantA, userAdm, "admin")
	h.Clients.Set("c9", "allowed")
	_ = h.KillAdmin.Set(adm, usecase.SetKillSwitchInput{Scope: "client", TargetID: "c9", Reason: "bad client", Active: true})
	ctx := tt.Ctx(tenantA, userU, "user")
	if _, blocked, _ := h.KillState.Execute(ctx, "c9", "", ""); !blocked {
		t.Error("c9 must be blocked")
	}
	if _, blocked, _ := h.KillState.Execute(ctx, "c1", "", ""); blocked {
		t.Error("c1 must not be blocked")
	}
	if _, blocked, _ := h.KillState.Execute(tt.Ctx(tenantB, userU, "user"), "c9", "", ""); blocked {
		t.Error("another tenant is unaffected")
	}
}

// With the cache TTL in force a kill switch set elsewhere is seen within it.
func TestKillStateCacheBoundsPropagation(t *testing.T) {
	h := tt.NewHarness(true)
	cfg := usecase.DefaultGovernanceConfig()
	cfg.KillStateTTL = 5 * time.Second
	core := usecase.NewGovernanceCore(h.Store, h.Store, h.Store, h.Engine, h.Clients, h.Clock, usecase.Defaults{TenantEnabled: true, MaxTokenDays: 90}, cfg, nil)
	ctx := tt.Ctx(tenantA, userU, "user")
	if st, _ := core.KillState(ctx, tenantA); len(st.Entries) != 0 {
		t.Fatal("clear")
	}
	_ = h.KillAdmin.Set(tt.Ctx(tenantA, userAdm, "admin"), usecase.SetKillSwitchInput{Scope: "tenant", Reason: "incident", Active: true}) // written by "another replica"
	if st, _ := core.KillState(ctx, tenantA); len(st.Entries) != 0 {
		t.Fatal("cached view within TTL")
	}
	h.Clock.Advance(6 * time.Second)
	if st, _ := core.KillState(ctx, tenantA); len(st.Entries) != 1 {
		t.Fatal("must be visible after the TTL even if no event arrived")
	}
	core.InvalidateTenant(tenantA)
}

func TestRateLimitPerClassAndPerClient(t *testing.T) {
	h := tt.NewHarness(true)
	ctx := tt.Ctx(tenantA, userU, "user")
	h.Store.Policies[tenantA] = []domain.ToolPolicy{pol("p", "allow", domain.ToolPolicyMatch{Tool: "terminal_send"})}
	denied := ""
	for i := 0; i < h.Cfg.Rate.PerMinute["exec"]+1; i++ {
		out, err := h.Authorize.Execute(ctx, usecase.AuthorizeInput{Tool: tt.ToolExec, Ctx: tt.CC(), Args: []byte(fmt.Sprintf(`{"n":%d}`, i))})
		if err != nil {
			t.Fatal(err)
		}
		if out.Outcome == domain.DecisionDeny {
			denied = out.Reasons[0]
			if i != h.Cfg.Rate.PerMinute["exec"] {
				t.Fatalf("limited too early at %d", i)
			}
		}
	}
	if denied != domain.ReasonRateLimited {
		t.Fatalf("denied reason: %q", denied)
	}
	// Another client has its own budget; so does the next minute.
	h.Clients.Set("c2", "allowed")
	cc2 := tt.CC()
	cc2.ClientID = "c2"
	if out, _ := h.Authorize.Execute(ctx, usecase.AuthorizeInput{Tool: tt.ToolExec, Ctx: cc2, Args: []byte(`{"n":1}`)}); out.Outcome != domain.DecisionAllow {
		t.Fatalf("other client: %+v", out)
	}
	h.Clock.Advance(61 * time.Second)
	if out, _ := h.Authorize.Execute(ctx, usecase.AuthorizeInput{Tool: tt.ToolExec, Ctx: tt.CC(), Args: []byte(`{"n":500}`)}); out.Outcome != domain.DecisionAllow {
		t.Fatalf("window must slide: %+v", out)
	}
}

func TestRateLimitedDenyAuditIsSuppressedWithCount(t *testing.T) {
	h := tt.NewHarness(true)
	h.Cfg.Rate.PerMinute["read"] = 1
	core := usecase.NewGovernanceCore(h.Store, h.Store, h.Store, h.Engine, h.Clients, h.Clock, usecase.Defaults{TenantEnabled: true, MaxTokenDays: 90}, h.Cfg, nil)
	auth := usecase.NewAuthorizeToolCall(core, h.Store, h.Store, nil, h.Clock)
	ctx := tt.Ctx(tenantA, userU, "user")
	for i := 0; i < 6; i++ {
		_, _ = auth.Execute(ctx, usecase.AuthorizeInput{Tool: tt.ToolRead, Ctx: tt.CC(), Args: []byte(fmt.Sprintf(`{"n":%d}`, i))})
	}
	denyRows := 0
	for _, c := range h.Store.Calls {
		if c.Decision == domain.CallDeny {
			denyRows++
		}
	}
	if denyRows != 1 {
		t.Fatalf("flood denials must collapse to one audit row per 10s, got %d", denyRows)
	}
	h.Clock.Advance(11 * time.Second)
	_, _ = auth.Execute(ctx, usecase.AuthorizeInput{Tool: tt.ToolRead, Ctx: tt.CC(), Args: []byte(`{"n":99}`)})
	found := false
	for _, e := range h.Store.EventsOf(domain.SubjectAuditAppended) {
		found = found || (contains(string(e.PayloadJSON), `"suppressed_count":4`))
	}
	if !found {
		t.Fatal("next recorded row must carry the suppressed count")
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func TestCompleteRecordsTaintAndAuditOnce(t *testing.T) {
	h := tt.NewHarness(true)
	ctx := tt.Ctx(tenantA, userU, "user")
	reader := tt.ToolRead
	reader.ReadUntrusted = true
	out, _ := h.Authorize.Execute(ctx, usecase.AuthorizeInput{Tool: reader, Ctx: tt.CC(), Args: []byte(`{}`)})
	if out.Outcome != domain.DecisionAllow {
		t.Fatal(out)
	}
	if err := h.Complete.Execute(ctx, usecase.CompleteInput{CallID: out.CallID, Result: "error"}); err != nil {
		t.Fatal(err)
	}
	if tainted, _ := h.Store.IsTainted(ctx, tenantA, userU, "c1", h.Clock.Now()); tainted {
		t.Fatal("a failed read taints nothing")
	}
	out2, _ := h.Authorize.Execute(ctx, usecase.AuthorizeInput{Tool: reader, Ctx: tt.CC(), Args: []byte(`{"n":2}`)})
	_ = h.Complete.Execute(ctx, usecase.CompleteInput{CallID: out2.CallID, Result: "ok"})
	_ = h.Complete.Execute(ctx, usecase.CompleteInput{CallID: out2.CallID, Result: "ok"}) // idempotent
	if tainted, _ := h.Store.IsTainted(ctx, tenantA, userU, "c1", h.Clock.Now()); !tainted {
		t.Fatal("a successful untrusted read must taint (user, client)")
	}
	if tainted, _ := h.Store.IsTainted(ctx, tenantA, userU, "c2", h.Clock.Now()); tainted {
		t.Fatal("taint is per client")
	}
	h.Clock.Advance(31 * time.Minute)
	if tainted, _ := h.Store.IsTainted(ctx, tenantA, userU, "c1", h.Clock.Now()); tainted {
		t.Fatal("taint expires")
	}
	audit := 0
	for _, e := range h.Store.EventsOf(domain.SubjectAuditAppended) {
		if contains(string(e.PayloadJSON), out2.CallID) {
			audit++
		}
	}
	if audit != 1 {
		t.Fatalf("exactly one audit event per call, got %d", audit)
	}
	if err := h.Complete.Execute(ctx, usecase.CompleteInput{CallID: out2.CallID, Result: "maybe"}); code(err) != domain.CodeInvalidArgument {
		t.Fatalf("bad result: %v", err)
	}
	if err := h.Complete.Execute(tt.Ctx(tenantA, "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", "user"), usecase.CompleteInput{CallID: out2.CallID, Result: "ok"}); code(err) != domain.CodeNotFound {
		t.Fatalf("another user's call: %v", err)
	}
}

func TestReaperFinalizesInterruptedCalls(t *testing.T) {
	h := tt.NewHarness(true)
	ctx := tt.Ctx(tenantA, userU, "user")
	out, _ := h.Authorize.Execute(ctx, usecase.AuthorizeInput{Tool: tt.ToolRead, Ctx: tt.CC(), Args: []byte(`{}`)})
	if n, _ := h.Maint.ReapInterrupted(context.Background(), 10); n != 0 {
		t.Fatal("fresh call must not be reaped")
	}
	h.Clock.Advance(16 * time.Minute)
	if n, err := h.Maint.ReapInterrupted(context.Background(), 10); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if c := h.Store.Calls[out.CallID]; c.State != domain.CallStateDone || c.ReasonCode != domain.ReasonInterrupted || c.Result != domain.ResultError {
		t.Fatalf("%+v", c)
	}
}

func TestExplainPolicyIsAdminOnlyAndUsesSameDecision(t *testing.T) {
	h := tt.NewHarness(true)
	h.Store.Policies[tenantA] = []domain.ToolPolicy{pol("p1", "require_approval", domain.ToolPolicyMatch{Tool: "task_create"})}
	if _, err := h.Explain.Execute(tt.Ctx(tenantA, userU, "user"), usecase.ExplainInput{Tool: tt.ToolWrite}); code(err) != domain.CodeNotAdmin {
		t.Fatalf("%v", err)
	}
	adm := tt.Ctx(tenantA, userAdm, "admin")
	d, err := h.Explain.Execute(adm, usecase.ExplainInput{Tool: tt.ToolWrite})
	if err != nil || d.Decision != "require_approval" || d.Reasons[0] != "policy:p1:require_approval" {
		t.Fatalf("%+v %v", d, err)
	}
	d, _ = h.Explain.Execute(adm, usecase.ExplainInput{Tool: tt.ToolWrite, UserID: userU})
	if d.Reasons[len(d.Reasons)-1] != "role_unresolved" {
		t.Fatalf("%+v", d)
	}
	if _, err := h.Explain.Execute(adm, usecase.ExplainInput{}); code(err) != domain.CodeNotFound {
		t.Fatalf("unknown tool: %v", err)
	}
}
