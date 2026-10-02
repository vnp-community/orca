package usecase_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
	tt "github.com/stablyai/orca-go/services/mcp-service/internal/usecase/usecasetest"
)

func code(err error) string {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	if err == nil {
		return ""
	}
	return "UNTYPED:" + err.Error()
}

func TestUpsertRejectsHardDenyTouch(t *testing.T) {
	credChan := domain.ToolRef{Name: "credentials_get", Channel: "credentials.get", Namespace: "credentials", Risk: "read"}
	cases := []struct {
		name    string
		p       domain.ToolPolicy
		touched []domain.ToolRef
		want    string
	}{
		{"allow exact hard-denied tool", pol("", "allow", domain.ToolPolicyMatch{Tool: "credentials_get"}), nil, domain.CodePolicyHardDeny},
		{"require_approval exact hard-denied tool", pol("", "require_approval", domain.ToolPolicyMatch{Tool: "credentials_get"}), nil, domain.CodePolicyHardDeny},
		{"allow prefix tool mcp_approval_decide", pol("", "allow", domain.ToolPolicyMatch{Tool: "mcp_approval_decide"}), nil, domain.CodePolicyHardDeny},
		{"allow namespace credentials", pol("", "allow", domain.ToolPolicyMatch{Namespace: "credentials"}), nil, domain.CodePolicyHardDeny},
		{"approval namespace mcp", pol("", "require_approval", domain.ToolPolicyMatch{Namespace: "mcp"}), nil, domain.CodePolicyHardDeny},
		{"allow namespace containing hard-denied channel", pol("", "allow", domain.ToolPolicyMatch{Namespace: "team"}), nil, domain.CodePolicyHardDeny},
		{"allow hard-denied channel list entry", pol("", "allow", domain.ToolPolicyMatch{Tool: "admin_createUser"}), nil, domain.CodePolicyHardDeny},
		{"touched tool is hard-denied", pol("", "allow", domain.ToolPolicyMatch{Risk: "read"}), []domain.ToolRef{credChan}, domain.CodePolicyHardDeny},
		{"client-only policy touches whole catalog", pol("", "require_approval", domain.ToolPolicyMatch{ClientID: "c1"}), nil, domain.CodePolicyHardDeny},
		{"roles-only allow touches whole catalog", pol("", "allow", domain.ToolPolicyMatch{Roles: []string{"user"}}), nil, domain.CodePolicyHardDeny},
		{"deny on hard-denied is fine", pol("", "deny", domain.ToolPolicyMatch{Tool: "credentials_get"}), nil, ""},
		{"allow ordinary tool ok", pol("", "require_approval", domain.ToolPolicyMatch{Tool: "task_create"}), nil, ""},
		{"wildcard exec allow rejected", pol("", "allow", domain.ToolPolicyMatch{Risk: "exec"}), nil, domain.CodeInvalidArgument},
		{"namespace allow touching exec rejected", pol("", "allow", domain.ToolPolicyMatch{Namespace: "terminal"}), []domain.ToolRef{tt.ToolExec}, domain.CodeInvalidArgument},
		{"exact exec allow accepted", pol("", "allow", domain.ToolPolicyMatch{Tool: "terminal_send"}), []domain.ToolRef{tt.ToolExec}, ""},
		{"empty match rejected", pol("", "deny", domain.ToolPolicyMatch{}), nil, domain.CodeInvalidArgument},
		{"bad decision rejected", pol("", "maybe", domain.ToolPolicyMatch{Tool: "x"}), nil, domain.CodeInvalidArgument},
		{"bad risk rejected", pol("", "deny", domain.ToolPolicyMatch{Risk: "weird"}), nil, domain.CodeInvalidArgument},
		{"bad role rejected", pol("", "deny", domain.ToolPolicyMatch{Roles: []string{"root"}}), nil, domain.CodeInvalidArgument},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := tt.NewHarness(true)
			_, err := h.Policies.Upsert(tt.Ctx(tenantA, userAdm, "admin"), usecase.UpsertInput{Policy: c.p, TouchedTools: c.touched})
			if got := code(err); got != c.want {
				t.Fatalf("code = %q (%v), want %q", got, err, c.want)
			}
			if c.want != "" && len(h.Store.Policies[tenantA]) != 0 {
				t.Fatal("rejected policy must not be stored")
			}
		})
	}
}

func TestPolicyAdminRequiresAdmin(t *testing.T) {
	h := tt.NewHarness(true)
	ctx := tt.Ctx(tenantA, userU, "user")
	if _, err := h.Policies.Upsert(ctx, usecase.UpsertInput{Policy: pol("", "deny", domain.ToolPolicyMatch{Tool: "x"})}); code(err) != domain.CodeNotAdmin {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := h.Policies.List(ctx); code(err) != domain.CodeNotAdmin {
		t.Fatalf("list: %v", err)
	}
	if err := h.Policies.Delete(ctx, "cccccccc-cccc-cccc-cccc-cccccccccccc"); code(err) != domain.CodeNotAdmin {
		t.Fatalf("delete: %v", err)
	}
	if _, err := h.Settings.Get(tt.Ctx(tenantA, userU, "")); code(err) != domain.CodeNotAdmin {
		t.Fatalf("empty role must fail closed: %v", err)
	}
}

func TestUpsertLifecycleEpochEventsAndRevisions(t *testing.T) {
	h := tt.NewHarness(true)
	ctx := tt.Ctx(tenantA, userAdm, "admin")
	created, err := h.Policies.Upsert(ctx, usecase.UpsertInput{Policy: pol("", "deny", domain.ToolPolicyMatch{Tool: "task_list"})})
	if err != nil || created.Version != 1 || created.ID == "" {
		t.Fatalf("create: %+v %v", created, err)
	}
	created.Decision = "require_approval"
	updated, err := h.Policies.Upsert(ctx, usecase.UpsertInput{Policy: created})
	if err != nil || updated.Version != 2 {
		t.Fatalf("update: %+v %v", updated, err)
	}
	if err := h.Policies.Delete(ctx, updated.ID); err != nil {
		t.Fatal(err)
	}
	if h.Store.Epoch[tenantA] != 3 {
		t.Fatalf("epoch = %d, want 3", h.Store.Epoch[tenantA])
	}
	if len(h.Store.Revisions) != 3 || len(h.Store.EventsOf(domain.SubjectPolicyChanged)) != 3 || len(h.Store.EventsOf(domain.SubjectAuditAppended)) != 3 {
		t.Fatalf("revisions=%v policyEvents=%d audit=%d", h.Store.Revisions, len(h.Store.EventsOf(domain.SubjectPolicyChanged)), len(h.Store.EventsOf(domain.SubjectAuditAppended)))
	}
	if err := h.Policies.Delete(ctx, updated.ID); code(err) != domain.CodeNotFound {
		t.Fatalf("second delete: %v", err)
	}
}

func TestUpsertVersionConflict(t *testing.T) {
	h := tt.NewHarness(true)
	ctx := tt.Ctx(tenantA, userAdm, "admin")
	p, _ := h.Policies.Upsert(ctx, usecase.UpsertInput{Policy: pol("", "deny", domain.ToolPolicyMatch{Tool: "task_list"})})
	var wg sync.WaitGroup
	var mu sync.Mutex
	var ok, conflict int
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q := p
			q.Decision = "require_approval"
			_, err := h.Policies.Upsert(ctx, usecase.UpsertInput{Policy: q})
			mu.Lock()
			defer mu.Unlock()
			switch code(err) {
			case "":
				ok++
			case domain.CodePolicyVersionConflict:
				conflict++
			default:
				t.Errorf("unexpected: %v", err)
			}
		}()
	}
	wg.Wait()
	if ok != 1 || conflict != 7 {
		t.Fatalf("ok=%d conflict=%d, want exactly one writer to win", ok, conflict)
	}
}

func TestSettingsBoundsAndNoKillSwitchPatch(t *testing.T) {
	h := tt.NewHarness(true)
	ctx := tt.Ctx(tenantA, userAdm, "admin")
	bad := []usecase.SettingsPatch{}
	for _, d := range []int{0, 91, -1} {
		d := d
		bad = append(bad, usecase.SettingsPatch{MaxTokenDays: &d})
	}
	for _, s := range []int{29, 901} {
		s := s
		bad = append(bad, usecase.SettingsPatch{ApprovalTTLSeconds: &s})
	}
	bad = append(bad, usecase.SettingsPatch{})
	for i, p := range bad {
		if _, err := h.Settings.Set(ctx, p); code(err) != domain.CodeInvalidArgument {
			t.Errorf("patch %d: %v", i, err)
		}
	}
	d, s := 30, 120
	f := false
	out, err := h.Settings.Set(ctx, usecase.SettingsPatch{MaxTokenDays: &d, ApprovalTTLSeconds: &s, DCREnabled: &f})
	if err != nil || out.MaxTokenDays != 30 || out.ApprovalTTLSeconds != 120 || !out.Enabled {
		t.Fatalf("%+v %v", out, err)
	}
}
