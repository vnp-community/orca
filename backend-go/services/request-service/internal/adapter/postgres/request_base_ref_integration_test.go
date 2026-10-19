//go:build integration

package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

// A Request created before 0099 reads back with an empty ref, the length CHECK rejects 256 characters, and 0099 can be
// reversed and re-applied on its own without touching the row.
func TestPostgres_Migration_BaseRef(t *testing.T) {
	_, admin := startPostgres(t)
	ctx := context.Background()
	before, target := contracttest.MigrationUpScriptsSplit(t, "postgres", "0099")
	applyScripts(t, admin, before)
	oldID := uuid.NewString()
	if err := insertRequestRow(admin, map[string]any{"id": oldID}); err != nil {
		t.Fatal(err)
	}
	applyScripts(t, admin, []string{target})
	assertAllTablesForceRLS(t, admin)

	var ref string
	if err := admin.QueryRow(ctx, `SELECT base_ref FROM request.requests WHERE id = $1`, oldID).Scan(&ref); err != nil || ref != "" {
		t.Fatalf("a Request that predates 0099 must read back empty: %q %v", ref, err)
	}
	if err := insertRequestRow(admin, map[string]any{"base_ref": strings.Repeat("a", 255)}); err != nil {
		t.Fatalf("255 characters must be accepted: %v", err)
	}
	if err := insertRequestRow(admin, map[string]any{"base_ref": strings.Repeat("a", 256)}); err == nil {
		t.Fatal("256 characters must be rejected by the CHECK")
	}
	if err := insertRequestRow(admin, map[string]any{"base_ref": nil}); err == nil {
		t.Fatal("NULL must be rejected: empty is the only way to say default")
	}

	downs := contracttest.MigrationScripts(t, "postgres", "down")
	applyScripts(t, admin, downs[contracttest.MigrationScriptIndex(t, "postgres", "down", "0099_"):contracttest.MigrationScriptIndex(t, "postgres", "down", "0099_")+1])
	var cols int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema='request' AND table_name='requests' AND column_name='base_ref'`).Scan(&cols); err != nil || cols != 0 {
		t.Fatalf("down must drop the column: %d %v", cols, err)
	}
	var n int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM request.requests WHERE id = $1`, oldID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("down must keep the rows: %d %v", n, err)
	}
	applyScripts(t, admin, []string{target})
	if err := admin.QueryRow(ctx, `SELECT base_ref FROM request.requests WHERE id = $1`, oldID).Scan(&ref); err != nil || ref != "" {
		t.Fatalf("re-applied 0099 must default existing rows to empty: %q %v", ref, err)
	}
}

func TestPostgres_BaseRefLockContract(t *testing.T) {
	f := newMigratedPostgres(t)
	base := New(f.app)
	contracttest.RunBaseRefLockContract(t, contracttest.BaseRefLockDeps{
		LifecycleEnv: f.lifecycleEnv(), Locker: NewRequestLocker(base), Presence: NewWorktreePresence(base), Changes: NewOpenSpecChangeRepository(base),
	})
}
