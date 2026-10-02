package usecase

import (
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// grantFor approves a grant for (tenant, user) on client and returns its id.
func (h *authzHarness) grantFor(tenantID, userID, role, client string) string {
	h.t.Helper()
	ctx := as(tenantID, userID, role)
	h.settings.rows[tenantID] = domain.TenantSettings{TenantID: tenantID, Enabled: true, DCREnabled: true, MaxTokenDays: 90, ApprovalTTLSeconds: 600}
	in := h.in("")
	in.ClientID = client
	r, err := h.create.Execute(ctx, in)
	if err != nil || r.RequestID == "" {
		h.t.Fatalf("create: %+v %v", r, err)
	}
	h.approve(ctx, r.RequestID, "orca:read")
	g, err := h.repo.GetActiveGrant(ctx, tenantID, userID, client)
	if err != nil {
		h.t.Fatal(err)
	}
	return g.ID
}

func TestListGrants_SelfOnly(t *testing.T) {
	h := newAuthzHarness(t)
	h.grantFor(tenantA, userAlice, "user", "app-1")
	h.grantFor(tenantA, userBob, "user", "app-1")

	got, err := h.list.Execute(as(tenantA, userAlice, "user"), ListGrantsInput{})
	if err != nil || len(got) != 1 || got[0].Grant.UserID != userAlice {
		t.Fatalf("got %+v err %v", got, err)
	}
	// A user-supplied filter or AllUsers flag from a non-admin must not widen the view.
	_, err = h.list.Execute(as(tenantA, userAlice, "user"), ListGrantsInput{AllUsers: true})
	wantErr(t, err, domain.CodeNotAdmin)
	got, _ = h.list.Execute(as(tenantA, userAlice, "user"), ListGrantsInput{UserID: userBob})
	if len(got) != 1 || got[0].Grant.UserID != userAlice {
		t.Fatalf("filter widened a self listing: %+v", got)
	}
}

func TestListGrants_AdminSeesTenantOnlyWithNames(t *testing.T) {
	h := newAuthzHarness(t)
	h.grantFor(tenantA, userAlice, "user", "app-1")
	h.grantFor(tenantA, userBob, "user", "app-1")
	h.grantFor(tenantB, userAlice, "user", "app-1")
	h.as.names = map[string]string{userAlice: "Alice"}

	got, err := h.list.Execute(as(tenantA, userBob, "admin"), ListGrantsInput{AllUsers: true})
	if err != nil || len(got) != 2 {
		t.Fatalf("admin list = %+v err %v (must exclude other tenants)", got, err)
	}
	for _, g := range got {
		if g.Grant.UserID == userAlice && g.UserName != "Alice" {
			t.Fatalf("name missing: %+v", g)
		}
	}
	got, _ = h.list.Execute(as(tenantA, userBob, "admin"), ListGrantsInput{AllUsers: true, UserID: userAlice})
	if len(got) != 1 || got[0].Grant.UserID != userAlice {
		t.Fatalf("filter failed: %+v", got)
	}
}

func TestRevokeGrant_OwnerOnly(t *testing.T) {
	h := newAuthzHarness(t)
	id := h.grantFor(tenantA, userAlice, "user", "app-1")

	wantErr(t, h.revoke.Execute(as(tenantA, userBob, "user"), RevokeGrantInput{GrantID: id}), domain.CodeNotFound)
	wantErr(t, h.revoke.Execute(as(tenantB, userAlice, "user"), RevokeGrantInput{GrantID: id}), domain.CodeNotFound)
	wantErr(t, h.revoke.Execute(as(tenantA, userAlice, "user"), RevokeGrantInput{GrantID: "garbage"}), domain.CodeNotFound)
	// Requesting admin mode without the role is refused outright, not downgraded.
	wantErr(t, h.revoke.Execute(as(tenantA, userBob, "user"), RevokeGrantInput{GrantID: id, Admin: true}), domain.CodeNotAdmin)
	if len(h.as.revoked) != 0 {
		t.Fatal("auth-service must not hear about rejected revocations")
	}

	if err := h.revoke.Execute(as(tenantA, userAlice, "user"), RevokeGrantInput{GrantID: id}); err != nil {
		t.Fatal(err)
	}
	if len(h.as.revoked) != 1 || h.as.revoked[0] != id+":user_revoked" {
		t.Fatalf("revoked = %v", h.as.revoked)
	}
	if h.repo.grants[id].RevocationPropagatedAt == nil {
		t.Fatal("successful propagation must be recorded")
	}
	last := h.repo.events[len(h.repo.events)-1]
	if last.Subject != domain.SubjectGrantRevoked {
		t.Fatalf("event = %s", last.Subject)
	}
}

func TestRevokeGrant_AdminCanRevokeOthersWithAdminReason(t *testing.T) {
	h := newAuthzHarness(t)
	id := h.grantFor(tenantA, userAlice, "user", "app-1")
	if err := h.revoke.Execute(as(tenantA, userBob, "admin"), RevokeGrantInput{GrantID: id, Admin: true}); err != nil {
		t.Fatal(err)
	}
	if h.as.revoked[0] != id+":admin_revoked" {
		t.Fatalf("revoked = %v", h.as.revoked)
	}
	if h.repo.grants[id].RevokedBy != userBob {
		t.Fatalf("revoked_by = %q", h.repo.grants[id].RevokedBy)
	}
	// An admin from another tenant still can't reach it.
	id2 := h.grantFor(tenantB, userAlice, "user", "app-1")
	wantErr(t, h.revoke.Execute(as(tenantA, userBob, "admin"), RevokeGrantInput{GrantID: id2, Admin: true}), domain.CodeNotFound)
}

func TestRevokeGrant_PropagationFailureIsRetriedByReconciler(t *testing.T) {
	h := newAuthzHarness(t)
	id := h.grantFor(tenantA, userAlice, "user", "app-1")
	ctx := as(tenantA, userAlice, "user")

	h.as.revokeErr = appErr(apperrors.KindInternal, "MCP_UNAVAILABLE")
	if err := h.revoke.Execute(ctx, RevokeGrantInput{GrantID: id}); err != nil {
		t.Fatalf("user must get OK once the revocation is durably recorded: %v", err)
	}
	g := h.repo.grants[id]
	if g.Status != domain.GrantRevoked || g.RevocationPropagatedAt != nil {
		t.Fatalf("grant = %+v", g)
	}
	if n, _ := h.recon.Execute(ctx, 10); n != 0 {
		t.Fatal("reconciler must not report success while auth-service is still failing")
	}

	h.as.revokeErr = nil
	n, err := h.recon.Execute(ctx, 10)
	if err != nil || n != 1 || h.repo.grants[id].RevocationPropagatedAt == nil {
		t.Fatalf("reconcile n=%d err=%v grant=%+v", n, err, h.repo.grants[id])
	}
	if len(h.as.revoked) != 1 {
		t.Fatalf("revoked = %v", h.as.revoked)
	}
	if n, _ := h.recon.Execute(ctx, 10); n != 0 {
		t.Fatal("already-propagated grants must not be re-sent")
	}
}

func TestRevokeGrant_IsIdempotent(t *testing.T) {
	h := newAuthzHarness(t)
	id := h.grantFor(tenantA, userAlice, "user", "app-1")
	ctx := as(tenantA, userAlice, "user")
	for i := 0; i < 2; i++ {
		if err := h.revoke.Execute(ctx, RevokeGrantInput{GrantID: id}); err != nil {
			t.Fatalf("revoke #%d: %v", i, err)
		}
	}
	revokedEvents := 0
	for _, e := range h.repo.events {
		if e.Subject == domain.SubjectGrantRevoked {
			revokedEvents++
		}
	}
	if revokedEvents != 1 {
		t.Fatalf("%d revoked events, want 1", revokedEvents)
	}
}

func TestRevokedGrantNoLongerAutoApproves(t *testing.T) {
	h := newAuthzHarness(t)
	id := h.grantFor(tenantA, userAlice, "user", "app-1")
	ctx := as(tenantA, userAlice, "user")
	if err := h.revoke.Execute(ctx, RevokeGrantInput{GrantID: id}); err != nil {
		t.Fatal(err)
	}
	h.as.info.Scopes = []string{"orca:read"}
	in := h.in("")
	in.Scope = "orca:read"
	out, err := h.create.Execute(ctx, in)
	if err != nil || out.RequestID == "" {
		t.Fatalf("a revoked grant must require fresh consent: %+v %v", out, err)
	}
}

// --- admin client management ---

func TestListOAuthClients_AdminOnlyWithGrantCounts(t *testing.T) {
	h := newAuthzHarness(t)
	h.grantFor(tenantA, userAlice, "user", "app-1")
	h.grantFor(tenantA, userBob, "user", "app-1")
	h.as.clients = []OAuthClientView{{ClientID: "app-1", Name: "Cool App", Status: "allowed"}, {ClientID: "app-2", Status: "pending"}}

	_, err := h.clients.Execute(as(tenantA, userAlice, "user"))
	wantErr(t, err, domain.CodeNotAdmin)
	_, err = h.clients.Execute(as(tenantA, userAlice, ""))
	wantErr(t, err, domain.CodeNotAdmin)

	got, err := h.clients.Execute(as(tenantA, userAlice, "admin"))
	if err != nil || len(got) != 2 || got[0].ActiveGrants != 2 || got[1].ActiveGrants != 0 {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestSetOAuthClientStatus(t *testing.T) {
	h := newAuthzHarness(t)
	h.grantFor(tenantA, userAlice, "user", "app-1")

	_, err := h.setSt.Execute(as(tenantA, userAlice, "user"), "app-1", "blocked")
	wantErr(t, err, domain.CodeNotAdmin)
	for _, bad := range []string{"pending", "", "ALLOWED"} {
		_, err = h.setSt.Execute(as(tenantA, userAlice, "admin"), "app-1", bad)
		wantErr(t, err, domain.CodeInvalidArgument)
	}
	if len(h.outbox.records) != 0 {
		t.Fatal("rejected changes must not publish events")
	}

	it, err := h.setSt.Execute(as(tenantA, userAlice, "admin"), "app-1", "blocked")
	if err != nil || it.Status != "blocked" || it.ActiveGrants != 1 {
		t.Fatalf("item = %+v err = %v", it, err)
	}
	if len(h.outbox.records) != 1 || h.outbox.records[0].Subject != domain.SubjectClientStatusChange {
		t.Fatalf("events = %+v", h.outbox.records)
	}

	h.as.setErr = appErr(apperrors.KindNotFound, "OAUTH_CLIENT_NOT_FOUND")
	_, err = h.setSt.Execute(as(tenantA, userAlice, "admin"), "ghost", "blocked")
	wantErr(t, err, domain.CodeNotFound)
	h.as.setErr = errors.New("boom")
	if _, err = h.setSt.Execute(as(tenantA, userAlice, "admin"), "app-1", "allowed"); err == nil {
		t.Fatal("other errors must propagate")
	}
}
