//go:build integration

// TASK-BE-DB-003 pattern (usage-service's pilot), mirrored per this
// rollout's auth-service task: proves application-layer tenant_id scoping
// alone is sufficient on a dialect with NO Row-Level-Security equivalent
// at all, for every table whose Postgres migration declares an RLS policy
// (users, sessions, audit_log, sso_group_role_mapping — see
// migrations/postgres/0001_init.up.sql, 0006_sso_group_role_mapping.up.sql;
// sso_identities also has an RLS policy but no tenant-scoped query method
// exists on SsoIdentityRepository to test — see
// migrations/mysql/0009_sso_identities.up.sql's comment).
package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

func TestUserRepository_ListUsers_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenant1 := uuid.NewString()
	tenant2 := uuid.NewString()
	createTestUser(t, repo, tenant1, "iso-user-1@example.com")
	createTestUser(t, repo, tenant2, "iso-user-2@example.com")

	list, _, err := repo.ListUsers(ctx, tenant1, "", 50)
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(list) != 1 || list[0].TenantID != tenant1 {
		t.Fatalf("expected exactly 1 user scoped to tenant1, got %+v", list)
	}
}

func TestSessionRepository_ListForTenant_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenant1 := uuid.NewString()
	tenant2 := uuid.NewString()
	user1 := createTestUser(t, repo, tenant1, "iso-session-1@example.com")
	user2 := createTestUser(t, repo, tenant2, "iso-session-2@example.com")

	now := time.Now().UTC().Truncate(time.Second)
	s1, err := domain.NewSession(domain.HashSessionToken("iso-token-1"), user1.ID, tenant1, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("building session1: %v", err)
	}
	if err := repo.CreateSession(ctx, s1); err != nil {
		t.Fatalf("create session1: %v", err)
	}
	s2, err := domain.NewSession(domain.HashSessionToken("iso-token-2"), user2.ID, tenant2, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("building session2: %v", err)
	}
	if err := repo.CreateSession(ctx, s2); err != nil {
		t.Fatalf("create session2: %v", err)
	}

	rows, _, err := repo.ListForTenant(ctx, tenant1, "", 50)
	if err != nil {
		t.Fatalf("list for tenant: %v", err)
	}
	if len(rows) != 1 || rows[0].Session.TenantID != tenant1 {
		t.Fatalf("expected exactly 1 session scoped to tenant1, got %+v", rows)
	}
}

func TestAuditRepository_Query_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenant1 := uuid.NewString()
	tenant2 := uuid.NewString()
	e1, _ := domain.NewAuditEntry(uuid.NewString(), tenant1, uuid.NewString(), "user.login", "", "user", "t1", nil, domain.OutcomeAllowed, "", time.Now())
	e2, _ := domain.NewAuditEntry(uuid.NewString(), tenant2, uuid.NewString(), "user.login", "", "user", "t2", nil, domain.OutcomeAllowed, "", time.Now())
	if err := repo.Append(ctx, e1); err != nil {
		t.Fatalf("append e1: %v", err)
	}
	if err := repo.Append(ctx, e2); err != nil {
		t.Fatalf("append e2: %v", err)
	}

	entries, _, err := repo.Query(ctx, usecase.AuditQueryFilter{TenantID: tenant1}, "", 50)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(entries) != 1 || entries[0].TenantID != tenant1 {
		t.Fatalf("expected exactly 1 entry scoped to tenant1, got %+v", entries)
	}
}

// --- SsoGroupRoleMappingRepository ---

func TestSsoGroupRoleMappingRepository_UpsertInsertsThenUpdates(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	tenantID := uuid.NewString()
	m, err := domain.NewSsoGroupRoleMapping(uuid.NewString(), tenantID, domain.SsoProviderOIDC, "orca-admins", domain.RoleUser, now)
	if err != nil {
		t.Fatalf("building mapping: %v", err)
	}
	inserted, err := repo.Upsert(ctx, m)
	if err != nil {
		t.Fatalf("insert upsert: %v", err)
	}
	if inserted.Role != domain.RoleUser {
		t.Fatalf("expected role user, got %q", inserted.Role)
	}

	// Same (tenant_id, provider, group_name), different id and role — MUST
	// update role and keep the PRE-EXISTING row's id, mirroring the
	// Postgres variant's `ON CONFLICT ... DO UPDATE SET role = EXCLUDED.role
	// RETURNING id, ...` (id is never part of the SET clause, so the
	// original row's id survives a conflict on either dialect).
	conflictAttempt := m
	conflictAttempt.ID = uuid.NewString()
	conflictAttempt.Role = domain.RoleAdmin
	updated, err := repo.Upsert(ctx, conflictAttempt)
	if err != nil {
		t.Fatalf("conflicting upsert: %v", err)
	}
	if updated.ID != inserted.ID {
		t.Errorf("expected the pre-existing row's id %q to survive the conflict, got %q", inserted.ID, updated.ID)
	}
	if updated.Role != domain.RoleAdmin {
		t.Errorf("expected role to be updated to admin, got %q", updated.Role)
	}

	list, err := repo.ListForProvider(ctx, tenantID, "")
	if err != nil || len(list) != 1 {
		t.Fatalf("expected exactly 1 mapping row (not 2), got %d, err %v", len(list), err)
	}
}

func TestSsoGroupRoleMappingRepository_ListForProvider_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	tenant1 := uuid.NewString()
	tenant2 := uuid.NewString()
	m1, err := domain.NewSsoGroupRoleMapping(uuid.NewString(), tenant1, domain.SsoProviderGitHub, "org:tenant1", domain.RoleAdmin, now)
	if err != nil {
		t.Fatalf("building mapping1: %v", err)
	}
	if _, err := repo.Upsert(ctx, m1); err != nil {
		t.Fatalf("upsert m1: %v", err)
	}
	m2, err := domain.NewSsoGroupRoleMapping(uuid.NewString(), tenant2, domain.SsoProviderGitHub, "org:tenant2", domain.RoleAdmin, now)
	if err != nil {
		t.Fatalf("building mapping2: %v", err)
	}
	if _, err := repo.Upsert(ctx, m2); err != nil {
		t.Fatalf("upsert m2: %v", err)
	}

	list, err := repo.ListForProvider(ctx, tenant1, "")
	if err != nil {
		t.Fatalf("list for provider: %v", err)
	}
	if len(list) != 1 || list[0].TenantID != tenant1 {
		t.Fatalf("expected exactly 1 mapping scoped to tenant1, got %+v", list)
	}

	// provider filter: "" means every provider (empty-means-no-filter
	// convention) — narrowing to a nonexistent provider must return none.
	none, err := repo.ListForProvider(ctx, tenant1, domain.SsoProviderOIDC)
	if err != nil {
		t.Fatalf("list for provider (narrowed): %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected 0 mappings for tenant1+oidc, got %+v", none)
	}
}
