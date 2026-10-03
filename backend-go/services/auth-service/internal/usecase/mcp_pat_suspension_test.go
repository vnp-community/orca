package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

type fakeSuspensions struct {
	set     map[string]string
	failErr error
}

func (f *fakeSuspensions) SetMcpPatSuspension(_ context.Context, tenantID string, suspended bool, reason string, _ time.Time) error {
	if suspended {
		f.set[tenantID] = reason
	} else {
		delete(f.set, tenantID)
	}
	return nil
}

func (f *fakeSuspensions) IsMcpPatSuspended(_ context.Context, tenantID string) (bool, error) {
	if f.failErr != nil {
		return false, f.failErr
	}
	_, ok := f.set[tenantID]
	return ok, nil
}

func TestPatSuspension_KillSwitchOnThenOff(t *testing.T) {
	h := newMcpHarness(t, domain.RoleUser)
	sus := &fakeSuspensions{set: map[string]string{}}
	h.res = h.res.WithPatSuspension(sus)
	setter := NewSetMcpPatSuspension(sus, h.audit, h.clock)
	out, _ := h.create([]string{"orca:read"}, 1)
	jti := out.Token.JTI

	if r := h.resolve(jti); !r.Active {
		t.Fatalf("before kill switch: %+v", r)
	}
	if err := setter.Execute(context.Background(), h.tenant, h.user.ID, true, "incident"); err != nil {
		t.Fatal(err)
	}
	if r := h.resolve(jti); r.Active || r.InactiveReason != McpInactiveSuspended {
		t.Fatalf("suspended: %+v", r)
	}
	// Idempotent: a retried cleanup changes nothing.
	if err := setter.Execute(context.Background(), h.tenant, h.user.ID, true, "incident"); err != nil {
		t.Fatal(err)
	}
	// A PAT created while suspended is suspended too.
	late, _ := h.create([]string{"orca:read"}, 1)
	if r := h.resolve(late.Token.JTI); r.Active || r.InactiveReason != McpInactiveSuspended {
		t.Fatalf("token minted during suspension: %+v", r)
	}
	if err := setter.Execute(context.Background(), h.tenant, h.user.ID, false, "all clear"); err != nil {
		t.Fatal(err)
	}
	// PAT works again once the kill switch is off ...
	if r := h.resolve(jti); !r.Active || r.Role != "user" {
		t.Fatalf("after kill switch off: %+v", r)
	}
	// ... but a PAT revoked in the meantime stays revoked.
	if err := setter.Execute(context.Background(), h.tenant, h.user.ID, true, "again"); err != nil {
		t.Fatal(err)
	}
	if err := h.revoke.Execute(context.Background(), h.tenant, h.user.ID, jti); err != nil {
		t.Fatal(err)
	}
	_ = setter.Execute(context.Background(), h.tenant, h.user.ID, false, "clear")
	if r := h.resolve(jti); r.Active || r.InactiveReason != McpInactiveRevoked {
		t.Fatalf("revoked token must not come back: %+v", r)
	}
	var actions []string
	for _, e := range h.audit.entries {
		actions = append(actions, e.Action)
	}
	if !contains(actions, "mcp_token.suspended") || !contains(actions, "mcp_token.resumed") {
		t.Fatalf("audit actions = %v", actions)
	}
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func TestPatSuspension_OtherTenantUnaffected(t *testing.T) {
	h := newMcpHarness(t, domain.RoleUser)
	sus := &fakeSuspensions{set: map[string]string{uuid.NewString(): "other tenant"}}
	h.res = h.res.WithPatSuspension(sus)
	out, _ := h.create([]string{"orca:read"}, 1)
	if r := h.resolve(out.Token.JTI); !r.Active {
		t.Fatalf("another tenant's suspension leaked: %+v", r)
	}
}

func TestPatSuspension_LookupFailureFailsClosed(t *testing.T) {
	h := newMcpHarness(t, domain.RoleUser)
	h.res = h.res.WithPatSuspension(&fakeSuspensions{set: map[string]string{}, failErr: errors.New("db down")})
	out, _ := h.create([]string{"orca:read"}, 1)
	if _, err := h.res.Execute(context.Background(), ResolveMcpPrincipalInput{JTI: out.Token.JTI, UserID: h.user.ID, TenantID: h.tenant, TokenUse: "mcp_pat"}); err == nil {
		t.Fatal("a failing suspension lookup must surface as an error (gateway fails closed)")
	}
}

func TestPatSuspension_RequiresTenant(t *testing.T) {
	h := newMcpHarness(t, domain.RoleUser)
	if err := NewSetMcpPatSuspension(&fakeSuspensions{set: map[string]string{}}, h.audit, h.clock).Execute(context.Background(), "", "", true, "x"); err == nil {
		t.Fatal("empty tenant must be rejected")
	}
}
