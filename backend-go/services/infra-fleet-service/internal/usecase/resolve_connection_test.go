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
	err            error
	calls          []string // connectionIDs the port was called with, for assertions
}

func (f *fakeConnectionResolver) ResolveConnection(ctx context.Context, tenantID, connectionID string) (bool, domain.DevServer, error) {
	f.calls = append(f.calls, connectionID)
	if f.err != nil {
		return false, domain.DevServer{}, f.err
	}
	ds, found := f.byConnectionID[connectionID]
	return found, ds, nil
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}

func TestResolveConnection_RequiresTenantContext(t *testing.T) {
	uc := NewResolveConnection(&fakeConnectionResolver{})
	_, err := uc.Execute(context.Background(), "conn-1")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestResolveConnection_EmptyConnectionID_ShortCircuitsToLocal(t *testing.T) {
	resolver := &fakeConnectionResolver{}
	uc := NewResolveConnection(resolver)

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, "")
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
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelaySSH)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	uc := NewResolveConnection(resolver)

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, "conn-1")
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

// Branch 2: not found — no dev server owns this connectionId, meaning
// "execute locally". This must NOT be an error.
func TestResolveConnection_NotFound_ReturnsNotConnectedWithoutError(t *testing.T) {
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{}}
	uc := NewResolveConnection(resolver)

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, "unknown-conn")
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
	_, err := uc.Execute(ctx, "conn-1")
	if err == nil {
		t.Fatal("expected error to propagate from resolver failure")
	}
}
