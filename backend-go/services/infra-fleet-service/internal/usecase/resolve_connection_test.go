package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeConnectionResolver is an in-memory ConnectionResolver — the "test
// against fakes, not a real database" pattern from
// specs/backend-go/standards/testing-strategy.md's unit-test section.
type fakeConnectionResolver struct {
	byConnectionID map[string]domain.DevServer
	connByID       map[string]domain.Connection // optional per-connection metadata (RepoPath/WorktreeID)

	// byDevServerID/byWorktreeID back ResolveConnectionByDevServer/
	// ResolveConnectionByWorktree (TASK-025) — keyed the same way as
	// byConnectionID/connByID but by the reverse-lookup key instead.
	byDevServerID   map[string]domain.DevServer
	connByDevServer map[string]domain.Connection
	byWorktreeID    map[string]domain.DevServer
	connByWorktree  map[string]domain.Connection

	err   error
	calls []string // connectionIDs the port was called with, for assertions
}

func (f *fakeConnectionResolver) ResolveConnection(ctx context.Context, tenantID, connectionID string) (bool, domain.DevServer, domain.Connection, error) {
	f.calls = append(f.calls, connectionID)
	if f.err != nil {
		return false, domain.DevServer{}, domain.Connection{}, f.err
	}
	ds, found := f.byConnectionID[connectionID]
	if !found {
		return false, domain.DevServer{}, domain.Connection{}, nil
	}
	return true, ds, f.connByID[connectionID], nil
}

func (f *fakeConnectionResolver) ResolveConnectionByDevServer(ctx context.Context, tenantID, devServerID string) (bool, domain.DevServer, domain.Connection, error) {
	f.calls = append(f.calls, devServerID)
	if f.err != nil {
		return false, domain.DevServer{}, domain.Connection{}, f.err
	}
	ds, found := f.byDevServerID[devServerID]
	if !found {
		return false, domain.DevServer{}, domain.Connection{}, nil
	}
	return true, ds, f.connByDevServer[devServerID], nil
}

func (f *fakeConnectionResolver) ResolveConnectionByWorktree(ctx context.Context, tenantID, worktreeID string) (bool, domain.DevServer, domain.Connection, error) {
	f.calls = append(f.calls, worktreeID)
	if f.err != nil {
		return false, domain.DevServer{}, domain.Connection{}, f.err
	}
	ds, found := f.byWorktreeID[worktreeID]
	if !found {
		return false, domain.DevServer{}, domain.Connection{}, nil
	}
	return true, ds, f.connByWorktree[worktreeID], nil
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}

func TestResolveConnection_RequiresTenantContext(t *testing.T) {
	uc := NewResolveConnection(&fakeConnectionResolver{})
	_, err := uc.Execute(context.Background(), ResolveConnectionInput{ConnectionID: "conn-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestResolveConnection_EmptyConnectionID_ShortCircuitsToLocal(t *testing.T) {
	resolver := &fakeConnectionResolver{}
	uc := NewResolveConnection(resolver)

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, ResolveConnectionInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Connected {
		t.Error("expected Connected=false for an empty connectionId")
	}
	if len(resolver.calls) != 0 {
		t.Errorf("expected no repository round-trip for an empty connectionId, got %d calls", len(resolver.calls))
	}
}

// Branch 1: found — the connectionId resolves to a live dev server.
func TestResolveConnection_Found_ReturnsConnectedAndDevServer(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelaySSH, "ssht1")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	uc := NewResolveConnection(resolver)

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, ResolveConnectionInput{ConnectionID: "conn-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Connected {
		t.Fatal("expected Connected=true for a resolvable connectionId")
	}
	if out.DevServer != ds {
		t.Errorf("expected resolved dev server %+v, got %+v", ds, out.DevServer)
	}
}

// fakeHandshakeInfoProvider is an in-memory usecase.HandshakeInfoProvider
// for TASK-INT-03-02's node_version enrichment tests.
type fakeHandshakeInfoProvider struct {
	byDevServer map[string]string // devServerID -> nodeVersion
}

func (f fakeHandshakeInfoProvider) NodeVersionFor(devServerID string) (string, bool) {
	v, ok := f.byDevServer[devServerID]
	return v, ok
}

// TestResolveConnection_Sessions_EnrichesNodeVersion covers TASK-INT-03-02:
// a connected resolution with Sessions set gets NodeVersion populated from
// the live session.
func TestResolveConnection_Sessions_EnrichesNodeVersion(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	uc := NewResolveConnection(resolver)
	uc.Sessions = fakeHandshakeInfoProvider{byDevServer: map[string]string{"ds1": "20.11.0"}}

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, ResolveConnectionInput{ConnectionID: "conn-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.NodeVersion != "20.11.0" {
		t.Errorf("NodeVersion = %q, want 20.11.0", out.NodeVersion)
	}
}

// TestResolveConnection_Sessions_NilLeavesNodeVersionEmpty covers the
// default (nil Sessions) case: NodeVersion stays empty, never an error.
func TestResolveConnection_Sessions_NilLeavesNodeVersionEmpty(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	uc := NewResolveConnection(resolver)

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, ResolveConnectionInput{ConnectionID: "conn-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.NodeVersion != "" {
		t.Errorf("NodeVersion = %q, want empty when Sessions is nil", out.NodeVersion)
	}
}

// TestResolveConnection_Sessions_MissLeavesNodeVersionEmpty covers a
// Sessions miss (no live session, or a session that predates the field) —
// still not an error.
func TestResolveConnection_Sessions_MissLeavesNodeVersionEmpty(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	uc := NewResolveConnection(resolver)
	uc.Sessions = fakeHandshakeInfoProvider{byDevServer: map[string]string{}}

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, ResolveConnectionInput{ConnectionID: "conn-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.NodeVersion != "" {
		t.Errorf("NodeVersion = %q, want empty on a Sessions miss", out.NodeVersion)
	}
}

// Branch 2: not found — no dev server owns this connectionId, meaning
// "execute locally". This must NOT be an error.
func TestResolveConnection_NotFound_ReturnsNotConnectedWithoutError(t *testing.T) {
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{}}
	uc := NewResolveConnection(resolver)

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, ResolveConnectionInput{ConnectionID: "unknown-conn"})
	if err != nil {
		t.Fatalf("expected no error for an unresolved connectionId, got %v", err)
	}
	if out.Connected {
		t.Error("expected Connected=false when no dev server owns the connectionId")
	}
	if !out.DevServer.IsZero() {
		t.Errorf("expected zero-value DevServer for an unresolved connectionId, got %+v", out.DevServer)
	}
}

func TestResolveConnection_RepositoryFailurePropagates(t *testing.T) {
	resolver := &fakeConnectionResolver{err: errors.New("db unavailable")}
	uc := NewResolveConnection(resolver)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, ResolveConnectionInput{ConnectionID: "conn-1"})
	if err == nil {
		t.Fatal("expected error to propagate from resolver failure")
	}
}

// TASK-025/TASK-030: ResolveConnectionInput{DevServerID: ...} must resolve
// to the same output a by-ConnectionID resolve of the same live connection
// would.
func TestResolveConnection_ByDevServerID_MatchesByConnectionID(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelaySSH, "ssht1")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	conn := domain.Connection{ID: "conn-1", TenantID: "tenant-1", DevServerID: "ds1", RepoPath: "/repo", WorktreeID: "wt-1"}

	byConnID := &fakeConnectionResolver{
		byConnectionID: map[string]domain.DevServer{"conn-1": ds},
		connByID:       map[string]domain.Connection{"conn-1": conn},
	}
	byDevServer := &fakeConnectionResolver{
		byDevServerID:   map[string]domain.DevServer{"ds1": ds},
		connByDevServer: map[string]domain.Connection{"ds1": conn},
	}

	ctx := withTenant(context.Background(), "tenant-1")
	wantOut, err := NewResolveConnection(byConnID).Execute(ctx, ResolveConnectionInput{ConnectionID: "conn-1"})
	if err != nil {
		t.Fatalf("unexpected error resolving by connection id: %v", err)
	}
	gotOut, err := NewResolveConnection(byDevServer).Execute(ctx, ResolveConnectionInput{DevServerID: "ds1"})
	if err != nil {
		t.Fatalf("unexpected error resolving by dev server id: %v", err)
	}
	if gotOut != wantOut {
		t.Errorf("resolving by dev_server_id = %+v, want %+v (same as by connection_id)", gotOut, wantOut)
	}
	if gotOut.ConnectionID != "conn-1" {
		t.Errorf("ConnectionID = %q, want conn-1", gotOut.ConnectionID)
	}
}

// TASK-025/TASK-030: same as above, but keyed by WorktreeID.
func TestResolveConnection_ByWorktreeID_MatchesByConnectionID(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelaySSH, "ssht1")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	conn := domain.Connection{ID: "conn-1", TenantID: "tenant-1", DevServerID: "ds1", RepoPath: "/repo", WorktreeID: "wt-1"}

	byConnID := &fakeConnectionResolver{
		byConnectionID: map[string]domain.DevServer{"conn-1": ds},
		connByID:       map[string]domain.Connection{"conn-1": conn},
	}
	byWorktree := &fakeConnectionResolver{
		byWorktreeID:   map[string]domain.DevServer{"wt-1": ds},
		connByWorktree: map[string]domain.Connection{"wt-1": conn},
	}

	ctx := withTenant(context.Background(), "tenant-1")
	wantOut, err := NewResolveConnection(byConnID).Execute(ctx, ResolveConnectionInput{ConnectionID: "conn-1"})
	if err != nil {
		t.Fatalf("unexpected error resolving by connection id: %v", err)
	}
	gotOut, err := NewResolveConnection(byWorktree).Execute(ctx, ResolveConnectionInput{WorktreeID: "wt-1"})
	if err != nil {
		t.Fatalf("unexpected error resolving by worktree id: %v", err)
	}
	if gotOut != wantOut {
		t.Errorf("resolving by worktree_id = %+v, want %+v (same as by connection_id)", gotOut, wantOut)
	}
	if gotOut.ConnectionID != "conn-1" {
		t.Errorf("ConnectionID = %q, want conn-1", gotOut.ConnectionID)
	}
}
