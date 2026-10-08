//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func (f *pgFixture) rolloutEnv() contracttest.RolloutEnv {
	base := New(f.app)
	return contracttest.RolloutEnv{
		ApprovalEnv: f.approvalEnv(),
		Flow:        NewFlowSettingsRepository(base),
		Finder:      NewSourceLookup(base),
		Samples:     NewMetricsSamples(base),
		AgeRequest: func(t *testing.T, id string, age time.Duration) {
			t.Helper()
			if _, err := f.admin.Exec(context.Background(), `UPDATE request.requests SET updated_at = now() - make_interval(secs => $2) WHERE id = $1`, id, age.Seconds()); err != nil {
				t.Fatal(err)
			}
		},
	}
}

func TestPostgres_RolloutContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunRolloutContract(t, func(*testing.T) contracttest.RolloutEnv { return f.rolloutEnv() })
}

// tenant_settings holds the kill switch, so a connection without a tenant must see nothing of it.
func TestPostgres_TenantSettingsRLS(t *testing.T) {
	f := newMigratedPostgres(t)
	ctx := context.Background()
	env := f.rolloutEnv()
	tenantID := "11111111-1111-4111-8111-111111111111"
	if err := env.Flow.Upsert(contracttest.CtxForTenant(tenantID), true, "admin"); err != nil {
		t.Fatal(err)
	}
	var visible int
	if err := f.app.QueryRow(ctx, `SELECT count(*) FROM request.tenant_settings`).Scan(&visible); err != nil || visible != 0 {
		t.Fatalf("a bare connection saw %d tenant_settings rows (err %v), FORCE RLS must hide them", visible, err)
	}
	if _, err := f.app.Exec(ctx, `INSERT INTO request.tenant_settings (tenant_id, request_flow_enabled) VALUES ($1, true)`, tenantID); err == nil {
		t.Fatal("WITH CHECK must reject a write that carries no tenant scope")
	}
	var total int
	if err := f.admin.QueryRow(ctx, `SELECT count(*) FROM request.tenant_settings`).Scan(&total); err != nil || total != 1 {
		t.Fatalf("superuser sees %d rows, want 1 (%v)", total, err)
	}
}

// The gauge sampler reads requests across tenants through the relay switch, and must not be able to change them.
func TestPostgres_RequestsRelayScanIsReadOnly(t *testing.T) {
	f := newMigratedPostgres(t)
	ctx := context.Background()
	if err := insertRequestRow(f.admin, map[string]any{"status": "classifying"}); err != nil {
		t.Fatal(err)
	}
	var bare int
	if err := f.app.QueryRow(ctx, `SELECT count(*) FROM request.requests`).Scan(&bare); err != nil || bare != 0 {
		t.Fatalf("bare connection saw %d requests (%v)", bare, err)
	}
	tx, err := f.app.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.relay', 'on', true)`); err != nil {
		t.Fatal(err)
	}
	var seen int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM request.requests WHERE status = 'classifying'`).Scan(&seen); err != nil || seen != 1 {
		t.Fatalf("relay scan saw %d, want 1 (%v)", seen, err)
	}
	tag, err := tx.Exec(ctx, `UPDATE request.requests SET title = 'tampered'`)
	if err != nil || tag.RowsAffected() != 0 {
		t.Fatalf("relay mode must not write: %v %v", tag, err)
	}
}
