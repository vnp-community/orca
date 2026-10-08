//go:build integration

package mysql

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestOpenSpecChangeRepository_IsTenantScoped(t *testing.T) {
	f := newMigratedMySQL(t)
	repo := NewOpenSpecChangeRepository(New(f.db))
	tenantA, tenantB := uuid.NewString(), uuid.NewString()
	reqA, reqB := uuid.NewString(), uuid.NewString()
	for _, r := range []struct{ id, tenant string }{{reqA, tenantA}, {reqB, tenantB}} {
		if err := rawRequest(f, "analyzing", map[string]any{"id": r.id, "tenant_id": r.tenant}); err != nil {
			t.Fatal(err)
		}
	}
	ctxA := tenant.WithTenantID(context.Background(), tenantA)
	ctxB := tenant.WithTenantID(context.Background(), tenantB)

	change := domain.OpenSpecChange{ID: uuid.NewString(), RequestID: reqA, Status: "preparing", SyncState: "pending"}
	if _, err := repo.Upsert(ctxA, change); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := f.admin.QueryRow(`SELECT tenant_id FROM openspec_changes WHERE request_id = ?`, reqA).Scan(&stored); err != nil || stored != tenantA {
		t.Fatalf("change must be stored under the caller's tenant: tenant=%q err=%v", stored, err)
	}
	if _, found, err := repo.GetByRequest(ctxB, reqA); err != nil || found {
		t.Fatalf("another tenant must not read the change: found=%v err=%v", found, err)
	}
	if _, found, err := repo.GetByRequest(ctxA, reqA); err != nil || !found {
		t.Fatalf("owner must read its change: found=%v err=%v", found, err)
	}
}
