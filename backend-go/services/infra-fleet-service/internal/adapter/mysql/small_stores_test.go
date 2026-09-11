//go:build integration

// Smoke-level integration coverage (create + read back, at minimum) for
// every remaining repository unit in this package not already covered by
// repository_test.go — see setupDB there for the shared MySQL container +
// migration bootstrap. Mirrors internal/adapter/postgres's equivalent
// per-file tests where a direct match exists.
package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// TestDevServerGroupStore_CreateAndList_DoesNotLeakAcrossTenants covers
// CR-DS-006 Phase 1's DevServerGroup persistence — tenant scoping and
// parent_group_id round-tripping through NULL (root groups), plus
// TASK-BE-DB-003's tenant-isolation-without-RLS pattern (dev_server_groups
// is RLS-bearing on Postgres, migrations/postgres/0030).
func TestDevServerGroupStore_CreateAndList_DoesNotLeakAcrossTenants(t *testing.T) {
	db := setupDB(t)
	store := NewDevServerGroupStore(db)
	ctx := context.Background()

	root, err := domain.NewDevServerGroup(uuid.NewString(), testTenant1, "Backend Team", "")
	if err != nil {
		t.Fatalf("building root group: %v", err)
	}
	if _, err := store.Create(ctx, root); err != nil {
		t.Fatalf("creating root group: %v", err)
	}
	child, err := domain.NewDevServerGroup(uuid.NewString(), testTenant1, "Backend Team - Staging", root.ID)
	if err != nil {
		t.Fatalf("building child group: %v", err)
	}
	if _, err := store.Create(ctx, child); err != nil {
		t.Fatalf("creating child group: %v", err)
	}
	other, _ := domain.NewDevServerGroup(uuid.NewString(), testTenant2, "Other Tenant Team", "")
	if _, err := store.Create(ctx, other); err != nil {
		t.Fatalf("creating other-tenant group: %v", err)
	}

	got, err := store.List(ctx, testTenant1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 groups for tenant1 (cross-tenant leak?), got %+v", got)
	}
	var foundRoot, foundChild bool
	for _, g := range got {
		if g.ID == root.ID {
			foundRoot = true
			if g.ParentGroupID != "" {
				t.Errorf("expected root group to have no parent, got %q", g.ParentGroupID)
			}
		}
		if g.ID == child.ID {
			foundChild = true
			if g.ParentGroupID != root.ID {
				t.Errorf("expected child group's parent to be %q, got %q", root.ID, g.ParentGroupID)
			}
		}
	}
	if !foundRoot || !foundChild {
		t.Errorf("expected both root and child groups in listing, got %+v", got)
	}
}

func TestDevServerGroupGrantStore_CreateListDelete(t *testing.T) {
	db := setupDB(t)
	groupStore := NewDevServerGroupStore(db)
	grantStore := NewDevServerGroupGrantStore(db)
	ctx := context.Background()

	group, err := domain.NewDevServerGroup(uuid.NewString(), testTenant1, "Backend Team", "")
	if err != nil {
		t.Fatalf("building group: %v", err)
	}
	if _, err := groupStore.Create(ctx, group); err != nil {
		t.Fatalf("creating group: %v", err)
	}

	grant, err := domain.NewDevServerGroupGrant(uuid.NewString(), testTenant1, group.ID, domain.GranteeKindDepartment, "dept-1")
	if err != nil {
		t.Fatalf("building grant: %v", err)
	}
	if _, err := grantStore.Create(ctx, grant); err != nil {
		t.Fatalf("creating grant: %v", err)
	}

	byGroup, err := grantStore.ListByGroup(ctx, testTenant1, group.ID)
	if err != nil {
		t.Fatalf("ListByGroup: %v", err)
	}
	if len(byGroup) != 1 || byGroup[0].GranteeID != "dept-1" {
		t.Fatalf("unexpected ListByGroup result: %+v", byGroup)
	}

	all, err := grantStore.ListAll(ctx, testTenant1)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("want 1 grant, got %d", len(all))
	}

	if err := grantStore.Delete(ctx, testTenant1, grant.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	afterDelete, err := grantStore.ListAll(ctx, testTenant1)
	if err != nil {
		t.Fatalf("ListAll after delete: %v", err)
	}
	if len(afterDelete) != 0 {
		t.Errorf("want 0 grants after delete, got %d", len(afterDelete))
	}
}

func TestDevServerAccessRequestStore_CreateGetListPendingUpdateStatus(t *testing.T) {
	db := setupDB(t)
	groupStore := NewDevServerGroupStore(db)
	reqStore := NewDevServerAccessRequestStore(db)
	ctx := context.Background()

	group, _ := domain.NewDevServerGroup(uuid.NewString(), testTenant1, "Backend Team", "")
	if _, err := groupStore.Create(ctx, group); err != nil {
		t.Fatalf("creating group: %v", err)
	}

	req, err := domain.NewDevServerAccessRequest(uuid.NewString(), testTenant1, uuid.NewString(), group.ID, "please", domain.GranteeKindDepartment, "dept-1", 1_700_000_000_000)
	if err != nil {
		t.Fatalf("building access request: %v", err)
	}
	if _, err := reqStore.Create(ctx, req); err != nil {
		t.Fatalf("creating access request: %v", err)
	}

	got, err := reqStore.Get(ctx, testTenant1, req.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Message != "please" || got.Status != domain.AccessRequestStatusPending {
		t.Errorf("unexpected round-tripped access request: %+v", got)
	}

	pending, err := reqStore.ListPending(ctx, testTenant1)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("want 1 pending request, got %d", len(pending))
	}

	updated, err := reqStore.UpdateStatus(ctx, testTenant1, req.ID, domain.AccessRequestStatusApproved)
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if updated.Status != domain.AccessRequestStatusApproved {
		t.Errorf("want Status=approved, got %q", updated.Status)
	}
}

func TestAgentTokenStore_InsertListRevoke(t *testing.T) {
	db := setupDB(t)
	repo := New(db)
	tokenStore := NewAgentTokenStore(db)
	ctx := context.Background()

	ds, _ := domain.NewDevServer(uuid.NewString(), testTenant1, "10.0.0.20", domain.ConnectionModeDirectWebSocket, "", nil)
	if _, err := repo.Register(ctx, ds); err != nil {
		t.Fatalf("register dev server: %v", err)
	}

	tok := domain.AgentToken{
		ID: uuid.NewString(), TenantID: testTenant1, DevServerID: ds.ID, Name: "laptop",
		TokenHash: "abc123", CreatedAt: time.Now().UTC(),
	}
	if err := tokenStore.Insert(ctx, tok); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	n, err := tokenStore.CountActive(ctx, testTenant1, ds.ID)
	if err != nil {
		t.Fatalf("CountActive: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 active token, got %d", n)
	}

	gotByHash, found, err := tokenStore.FindActiveByHash(ctx, "abc123")
	if err != nil || !found {
		t.Fatalf("FindActiveByHash: found=%v err=%v", found, err)
	}
	if gotByHash.ID != tok.ID {
		t.Errorf("want token %q, got %q", tok.ID, gotByHash.ID)
	}

	revoked, err := tokenStore.Revoke(ctx, testTenant1, tok.ID)
	if err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if revoked.RevokedAt == nil {
		t.Errorf("want RevokedAt to be set")
	}
	// Revoking an already-revoked token must fail — proves Revoke doesn't
	// rely on a RowsAffected() no-op-retry ambiguity (see that method's doc
	// comment): revoked_at genuinely transitions NULL -> non-NULL exactly
	// once, so a second call finds 0 matching rows for real.
	if _, err := tokenStore.Revoke(ctx, testTenant1, tok.ID); err == nil {
		t.Errorf("want an error revoking an already-revoked token")
	}
}

func TestFleetDefinitionStore_CreateGetListUpdate(t *testing.T) {
	db := setupDB(t)
	store := NewFleetDefinitionStore(db)
	ctx := context.Background()

	def, err := domain.NewFleetDefinition(
		uuid.NewString(), testTenant1, "my-fleet",
		[]domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role", Kind: domain.AgentKindDevServer}},
		&domain.ProvisionConfig{IaC: "terraform", WorkingDir: "/infra", VarsFile: "prod.tfvars"},
		uuid.NewString(),
	)
	if err != nil {
		t.Fatalf("NewFleetDefinition: %v", err)
	}

	created, err := store.Create(ctx, def)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Error("expected CreatedAt/UpdatedAt to be populated")
	}

	got, err := store.Get(ctx, testTenant1, def.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Servers) != 1 || got.Servers[0].Host != "h1" {
		t.Errorf("servers JSON did not round-trip: %+v", got.Servers)
	}
	if got.Provision == nil || got.Provision.IaC != "terraform" {
		t.Errorf("provision JSON did not round-trip: %+v", got.Provision)
	}

	list, err := store.List(ctx, testTenant1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 fleet definition, got %d", len(list))
	}

	got.Version = 2
	got.Servers = append(got.Servers, domain.FleetSpecServer{Host: "h2", UserName: "orca", VaultSSHRole: "role", Kind: domain.AgentKindDevServer})
	updated, err := store.Update(ctx, got)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(updated.Servers) != 2 {
		t.Errorf("want 2 servers after update, got %d", len(updated.Servers))
	}

	// A stale version must be rejected — proves the optimistic-locking
	// translation (repository.go's doc comment: UPDATE unconditionally,
	// then SELECT scoped to the NEW version) correctly reports conflict
	// instead of silently succeeding or silently no-op'ing.
	got.Version = 2 // stale again, since the real current version is now 2->3 territory
	got.Servers = got.Servers[:1]
	if _, err := store.Update(ctx, got); err != domain.ErrFleetDefinitionVersionConflict {
		t.Errorf("want ErrFleetDefinitionVersionConflict on stale version, got %v", err)
	}
}

func TestTerminalSessionStore_CreateGetListTouchClose(t *testing.T) {
	db := setupDB(t)
	store := NewTerminalSessionStore(db)
	ctx := context.Background()
	now := time.Now().UTC()

	session := domain.TerminalSession{
		PtyID: "pty-1", TenantID: testTenant1, Cwd: "/repo", CreatedAt: now, LastActiveAt: now,
	}
	if _, err := store.Create(ctx, session); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, got, err := store.Get(ctx, testTenant1, "pty-1")
	if err != nil || !found {
		t.Fatalf("Get: found=%v err=%v", found, err)
	}
	if got.Cwd != "/repo" {
		t.Errorf("want Cwd=/repo, got %q", got.Cwd)
	}

	if err := store.Touch(ctx, testTenant1, "pty-1", now.Add(time.Minute)); err != nil {
		t.Fatalf("Touch: %v", err)
	}

	open, err := store.List(ctx, testTenant1, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(open) != 1 {
		t.Fatalf("want 1 open session, got %d", len(open))
	}

	if err := store.Close(ctx, testTenant1, "pty-1", now.Add(2*time.Minute)); err != nil {
		t.Fatalf("Close: %v", err)
	}
	afterClose, err := store.List(ctx, testTenant1, "")
	if err != nil {
		t.Fatalf("List after close: %v", err)
	}
	if len(afterClose) != 0 {
		t.Errorf("want 0 open sessions after close, got %d", len(afterClose))
	}

	// Touch/Close on an unknown pty_id must fail (not-found), proving these
	// don't silently succeed on a genuinely missing row.
	if err := store.Touch(ctx, testTenant1, "no-such-pty", now); err == nil {
		t.Errorf("want an error touching an unknown terminal session")
	}
}

func TestTerminalScrollbackSnapshotStore_UpsertGetSumDelete(t *testing.T) {
	db := setupDB(t)
	store := NewTerminalScrollbackSnapshotStore(db)
	ctx := context.Background()
	worktreeID := uuid.NewString()
	now := time.Now().UTC()

	if err := store.Upsert(ctx, domain.TerminalScrollbackSnapshot{
		TenantID: testTenant1, WorktreeID: worktreeID, PaneKey: "pane-a",
		Cols: 80, Rows: 24, DataGzip: []byte("gzA"), UncompressedBytes: 1000, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert pane-a: %v", err)
	}
	if err := store.Upsert(ctx, domain.TerminalScrollbackSnapshot{
		TenantID: testTenant1, WorktreeID: worktreeID, PaneKey: "pane-b",
		Cols: 80, Rows: 24, DataGzip: []byte("gzB"), UncompressedBytes: 500, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert pane-b: %v", err)
	}

	found, got, err := store.Get(ctx, testTenant1, worktreeID, "pane-a")
	if err != nil || !found {
		t.Fatalf("Get: found=%v err=%v", found, err)
	}
	if string(got.DataGzip) != "gzA" || got.Rows != 24 {
		t.Errorf("unexpected round-tripped snapshot (rows column must survive its backtick-quoting): %+v", got)
	}

	sum, err := store.SumUncompressedBytes(ctx, testTenant1, worktreeID, "pane-a")
	if err != nil {
		t.Fatalf("SumUncompressedBytes: %v", err)
	}
	if sum != 500 {
		t.Errorf("want sum=500 (excluding pane-a), got %d", sum)
	}

	// Re-upsert pane-a with new data — exercises ON DUPLICATE KEY UPDATE's
	// path over the (tenant_id, worktree_id, pane_key) unique constraint.
	if err := store.Upsert(ctx, domain.TerminalScrollbackSnapshot{
		TenantID: testTenant1, WorktreeID: worktreeID, PaneKey: "pane-a",
		Cols: 100, Rows: 30, DataGzip: []byte("gzA2"), UncompressedBytes: 1200, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("re-upsert pane-a: %v", err)
	}
	_, reGot, err := store.Get(ctx, testTenant1, worktreeID, "pane-a")
	if err != nil {
		t.Fatalf("Get after re-upsert: %v", err)
	}
	if reGot.Cols != 100 || string(reGot.DataGzip) != "gzA2" {
		t.Errorf("want the re-upsert's new values, got %+v", reGot)
	}

	if err := store.DeleteByWorktree(ctx, testTenant1, worktreeID); err != nil {
		t.Fatalf("DeleteByWorktree: %v", err)
	}
	if found, _, err := store.Get(ctx, testTenant1, worktreeID, "pane-a"); err != nil || found {
		t.Errorf("want no snapshot after DeleteByWorktree, found=%v err=%v", found, err)
	}
}

func TestQueuedPromptStore_UpsertGetDeleteGetAndDelete(t *testing.T) {
	db := setupDB(t)
	sessionStore := NewTerminalSessionStore(db)
	promptStore := NewQueuedPromptStore(db)
	ctx := context.Background()
	now := time.Now().UTC()

	if _, err := sessionStore.Create(ctx, domain.TerminalSession{PtyID: "pty-q1", TenantID: testTenant1, CreatedAt: now, LastActiveAt: now}); err != nil {
		t.Fatalf("seeding terminal session: %v", err)
	}

	p, err := domain.NewQueuedPrompt("pty-q1", testTenant1, "hello agent", "", now)
	if err != nil {
		t.Fatalf("NewQueuedPrompt: %v", err)
	}
	if err := promptStore.Upsert(ctx, p); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, found, err := promptStore.Get(ctx, "pty-q1")
	if err != nil || !found {
		t.Fatalf("Get: found=%v err=%v", found, err)
	}
	if got.Prompt != "hello agent" {
		t.Errorf("want Prompt='hello agent', got %q", got.Prompt)
	}

	gotDel, deleted, err := promptStore.GetAndDelete(ctx, "pty-q1")
	if err != nil || !deleted {
		t.Fatalf("GetAndDelete: found=%v err=%v", deleted, err)
	}
	if gotDel.Prompt != "hello agent" {
		t.Errorf("GetAndDelete returned wrong prompt: %+v", gotDel)
	}
	if _, found, err := promptStore.Get(ctx, "pty-q1"); err != nil || found {
		t.Errorf("want no prompt left after GetAndDelete, found=%v err=%v", found, err)
	}
}

func TestEphemeralVmRuntimeStore_ListGetUpdateStatusAndProvisionResult(t *testing.T) {
	db := setupDB(t)
	store := NewEphemeralVmRuntimeStore(db)
	ctx := context.Background()

	// Runtime rows have no domain constructor exercised in the Postgres
	// suite either — built as a raw INSERT via the store itself isn't
	// exposed (no Create method on this port, per
	// list_ephemeral_vm_runtimes.go's interface: List/Get/GetByWorkspaceID/
	// UpdateStatus/UpdateProvisionResult/FindDevServerByEnvironmentID/
	// SetEnvironmentID only) — seed directly through *sql.DB instead,
	// mirroring how the domain layer's real caller (EphemeralVmRelay)
	// expects a pre-existing row from an out-of-band provisioning flow.
	runtimeID := uuid.NewString()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO ephemeral_vm_runtimes (id, tenant_id, repo_id, recipe_id, status)
		VALUES (?, ?, ?, ?, 'provisioning')
	`, runtimeID, testTenant1, uuid.NewString(), "recipe-1"); err != nil {
		t.Fatalf("seeding ephemeral vm runtime: %v", err)
	}

	got, err := store.Get(ctx, testTenant1, runtimeID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.RecipeID != "recipe-1" || got.Status != "provisioning" {
		t.Errorf("unexpected seeded runtime: %+v", got)
	}

	list, err := store.List(ctx, testTenant1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 runtime, got %d", len(list))
	}

	updated, err := store.UpdateStatus(ctx, testTenant1, runtimeID, "active", "workspace-1", "")
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if updated.Status != "active" || updated.WorkspaceID != "workspace-1" {
		t.Errorf("unexpected UpdateStatus result: %+v", updated)
	}

	byWorkspace, err := store.GetByWorkspaceID(ctx, testTenant1, "workspace-1")
	if err != nil {
		t.Fatalf("GetByWorkspaceID: %v", err)
	}
	if byWorkspace.ID != runtimeID {
		t.Errorf("want runtime %q, got %q", runtimeID, byWorkspace.ID)
	}

	provisioned, err := store.UpdateProvisionResult(ctx, testTenant1, runtimeID, "active", "ssh", "")
	if err != nil {
		t.Fatalf("UpdateProvisionResult: %v", err)
	}
	if provisioned.ConnectionType != "ssh" {
		t.Errorf("want ConnectionType=ssh, got %q", provisioned.ConnectionType)
	}

	envSet, err := store.SetEnvironmentID(ctx, testTenant1, runtimeID, "env-123")
	if err != nil {
		t.Fatalf("SetEnvironmentID: %v", err)
	}
	if envSet.EnvironmentID != "env-123" {
		t.Errorf("want EnvironmentID=env-123, got %q", envSet.EnvironmentID)
	}

	devServerID, found, err := store.FindDevServerByEnvironmentID(ctx, testTenant1, "env-123")
	if err != nil || !found || devServerID != "env-123" {
		t.Fatalf("FindDevServerByEnvironmentID: devServerID=%q found=%v err=%v", devServerID, found, err)
	}
}

func TestEphemeralVmSshTargetStore_UpsertGet(t *testing.T) {
	db := setupDB(t)
	runtimeID := uuid.NewString()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO ephemeral_vm_runtimes (id, tenant_id, repo_id, recipe_id, status)
		VALUES (?, ?, ?, ?, 'active')
	`, runtimeID, testTenant1, uuid.NewString(), "recipe-1"); err != nil {
		t.Fatalf("seeding ephemeral vm runtime: %v", err)
	}

	store := NewEphemeralVmSshTargetStore(db)
	ctx := context.Background()

	record := domain.EphemeralVmSshTargetRecord{
		TenantID: testTenant1, RuntimeID: runtimeID, Host: "10.1.1.1", Port: 22, Username: "root",
		IdentityFileVaultPath: "secret/vm-key",
	}
	saved, err := store.Upsert(ctx, record)
	if err != nil {
		t.Fatalf("Upsert (insert): %v", err)
	}
	if saved.ID == "" {
		t.Errorf("want a generated ID")
	}

	record.Host = "10.1.1.2"
	record.HostKeyFingerprint = "SHA256:abc"
	if _, err := store.Upsert(ctx, record); err != nil {
		t.Fatalf("Upsert (update): %v", err)
	}

	got, found, err := store.Get(ctx, testTenant1, runtimeID)
	if err != nil || !found {
		t.Fatalf("Get: found=%v err=%v", found, err)
	}
	if got.Host != "10.1.1.2" || got.HostKeyFingerprint != "SHA256:abc" {
		t.Errorf("want the second upsert's values, got %+v", got)
	}
}

func TestBrowserProfileStore_ListCreateDelete(t *testing.T) {
	db := setupDB(t)
	repo := New(db)
	store := NewBrowserProfileStore(db)
	ctx := context.Background()

	ds, _ := domain.NewDevServer(uuid.NewString(), testTenant1, "10.0.0.30", domain.ConnectionModeRelayWebSocket, "", nil)
	if _, err := repo.Register(ctx, ds); err != nil {
		t.Fatalf("register: %v", err)
	}

	profile := domain.BrowserProfile{
		ID: uuid.NewString(), TenantID: testTenant1, DevServerID: ds.ID, Name: "Default", SourceBrowser: "chrome",
	}
	created, err := store.Create(ctx, profile)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.SourceBrowser != "chrome" {
		t.Errorf("want SourceBrowser=chrome, got %q", created.SourceBrowser)
	}

	list, err := store.List(ctx, testTenant1, ds.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 profile, got %d", len(list))
	}

	if err := store.Delete(ctx, testTenant1, profile.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	afterDelete, err := store.List(ctx, testTenant1, ds.ID)
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(afterDelete) != 0 {
		t.Errorf("want 0 profiles after delete, got %d", len(afterDelete))
	}
}

func TestAgentSessionStore_CreateGetAndUniqueConstraint(t *testing.T) {
	db := setupDB(t)
	repo := New(db)
	terminalStore := NewTerminalSessionStore(db)
	agentStore := NewAgentSessionStore(db)
	ctx := context.Background()
	now := time.Now().UTC()
	worktreeID := uuid.NewString()
	userID := uuid.NewString()

	ds, _ := domain.NewDevServer(uuid.NewString(), testTenant1, "10.0.0.40", domain.ConnectionModeRelayWebSocket, "", nil)
	if _, err := repo.Register(ctx, ds); err != nil {
		t.Fatalf("register dev server: %v", err)
	}
	if _, err := terminalStore.Create(ctx, domain.TerminalSession{PtyID: "agent-pty-1", TenantID: testTenant1, CreatedAt: now, LastActiveAt: now}); err != nil {
		t.Fatalf("seed terminal session: %v", err)
	}

	session := domain.AgentSession{
		ID: uuid.NewString(), TenantID: testTenant1, PtyID: "agent-pty-1", WorktreeID: worktreeID, DevServerID: ds.ID,
		UserID: userID, ModelID: "claude", Status: domain.AgentStatusSpawning, StartedAt: now, LastActiveAt: now,
	}
	if _, err := agentStore.Create(ctx, session); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, got, err := agentStore.Get(ctx, testTenant1, session.ID)
	if err != nil || !found {
		t.Fatalf("Get: found=%v err=%v", found, err)
	}
	if got.ModelID != "claude" {
		t.Errorf("want ModelID=claude, got %q", got.ModelID)
	}

	foundByPty, gotByPty, err := agentStore.GetByPtyID(ctx, testTenant1, "agent-pty-1")
	if err != nil || !foundByPty || gotByPty.ID != session.ID {
		t.Fatalf("GetByPtyID: found=%v err=%v got=%+v", foundByPty, err, gotByPty)
	}

	// BR-AG-01: a second non-terminal session for the same (tenant,
	// worktree, user) must be rejected — proves migrations/mysql/0019's
	// generated-column translation of the Postgres partial unique index
	// actually enforces the invariant against a real MySQL 8 server, not
	// just in theory.
	if _, err := terminalStore.Create(ctx, domain.TerminalSession{PtyID: "agent-pty-2", TenantID: testTenant1, CreatedAt: now, LastActiveAt: now}); err != nil {
		t.Fatalf("seed second terminal session: %v", err)
	}
	dup := domain.AgentSession{
		ID: uuid.NewString(), TenantID: testTenant1, PtyID: "agent-pty-2", WorktreeID: worktreeID, DevServerID: ds.ID,
		UserID: userID, ModelID: "claude", Status: domain.AgentStatusRunning, StartedAt: now, LastActiveAt: now,
	}
	if _, err := agentStore.Create(ctx, dup); err != domain.ErrAgentAlreadyRunning {
		t.Fatalf("want ErrAgentAlreadyRunning for a second active session on the same worktree+user, got %v", err)
	}

	if err := agentStore.MarkStopped(ctx, testTenant1, session.ID, now.Add(time.Minute)); err != nil {
		t.Fatalf("MarkStopped: %v", err)
	}
	_, stopped, err := agentStore.Get(ctx, testTenant1, session.ID)
	if err != nil {
		t.Fatalf("Get after stop: %v", err)
	}
	if stopped.Status != domain.AgentStatusStopped {
		t.Errorf("want Status=stopped, got %q", stopped.Status)
	}

	// Now that the first session is stopped, the generated-column unique
	// key must free up — a fresh Create for the same worktree+user must
	// succeed.
	if _, err := agentStore.Create(ctx, dup); err != nil {
		t.Fatalf("want Create to succeed once the prior session is stopped, got %v", err)
	}
}

func TestAgentRateLimitedOutboxStore_EnqueueFetchMarkPublished(t *testing.T) {
	db := setupDB(t)
	store := NewAgentRateLimitedOutboxStore(db)
	ctx := context.Background()

	rec := outbox.Record{
		ID:      uuid.NewString(),
		Subject: "orca.infra.agent.rate_limited",
		Event: eventbus.Event{
			TenantID: testTenant1, OccurredAt: time.Now().UTC(), Payload: []byte(`{}`),
		},
	}
	if err := store.Enqueue(ctx, rec); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	unpublished, err := store.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished: %v", err)
	}
	if len(unpublished) != 1 || unpublished[0].ID != rec.ID {
		t.Fatalf("unexpected fetch result: %+v", unpublished)
	}

	if err := store.MarkPublished(ctx, []string{rec.ID}); err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}
	afterMark, err := store.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished after mark: %v", err)
	}
	if len(afterMark) != 0 {
		t.Errorf("want 0 unpublished after MarkPublished, got %d", len(afterMark))
	}
}
