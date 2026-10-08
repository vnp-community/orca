//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func (f *pgFixture) approvalEnv() contracttest.ApprovalEnv {
	base := New(f.app)
	return contracttest.ApprovalEnv{
		LifecycleEnv: f.lifecycleEnv(),
		Approvals:    NewApprovalRepository(base),
		Approvers:    NewApproverRepository(base),
		Policies:     NewPolicyRepository(base),
		Locker:       NewRequestLocker(base),
	}
}

func TestPostgres_ApprovalRepositoryContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunApprovalRepositoryContract(t, func(*testing.T) contracttest.ApprovalEnv { return f.approvalEnv() })
}

// The sweeper's cross-tenant read is the only RLS exception: it must read, never write.
func TestPostgres_ApprovalSweepPolicyIsReadOnly(t *testing.T) {
	f := newMigratedPostgres(t)
	env := f.approvalEnv()
	ctx := context.Background()
	a, _ := contracttestSeed(t, env)

	var visibleWithoutScope int
	if err := f.app.QueryRow(ctx, `SELECT count(*) FROM request.approvals`).Scan(&visibleWithoutScope); err != nil || visibleWithoutScope != 0 {
		t.Fatalf("a bare connection must see no rows under FORCE RLS: %d %v", visibleWithoutScope, err)
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
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM request.approvals WHERE id = $1`, a).Scan(&seen); err != nil || seen != 1 {
		t.Fatalf("relay scan must see the row: %d %v", seen, err)
	}
	tag, err := tx.Exec(ctx, `UPDATE request.approvals SET comment = 'tampered' WHERE id = $1`, a)
	if err != nil || tag.RowsAffected() != 0 {
		t.Fatalf("relay mode must not allow writes: %v %v", tag, err)
	}
}
