//go:build integration

package mysql

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

// insertRawRequest writes a minimal valid row with the given column overrides, bypassing the repository.
func insertRawRequest(db *sql.DB, overrides map[string]any) error {
	cols := []string{"id", "tenant_id", "number", "title", "body", "source_provider", "source_url", "status", "urgency", "classification_reason", "return_reason", "reporter_id"}
	vals := map[string]any{
		"id": uuid.NewString(), "tenant_id": uuid.NewString(), "number": 1, "title": "t", "body": "", "source_provider": "manual",
		"source_url": "", "status": "new", "urgency": "normal", "classification_reason": "", "return_reason": "", "reporter_id": uuid.NewString(),
	}
	for k, v := range overrides {
		if _, ok := vals[k]; !ok {
			cols = append(cols, k)
		}
		vals[k] = v
	}
	args := make([]any, len(cols))
	for i, c := range cols {
		args[i] = vals[c]
	}
	_, err := db.Exec("INSERT INTO requests ("+strings.Join(cols, ", ")+") VALUES ("+strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",")+")", args...)
	return err
}

// A Request created before 0099 reads back with an empty ref, a default MySQL server (strict) rejects 256 characters, and
// 0099 can be reversed and re-applied on its own without touching the row.
func TestMySQL_Migration_BaseRef(t *testing.T) {
	_, admin := startMySQL(t)
	before, target := contracttest.MigrationUpScriptsSplit(t, "mysql", "0099")
	applyScripts(t, admin, before)
	oldID := uuid.NewString()
	if err := insertRawRequest(admin, map[string]any{"id": oldID}); err != nil {
		t.Fatal(err)
	}
	applyScripts(t, admin, []string{target})

	var ref string
	if err := admin.QueryRow(`SELECT base_ref FROM requests WHERE id = ?`, oldID).Scan(&ref); err != nil || ref != "" {
		t.Fatalf("a Request that predates 0099 must read back empty: %q %v", ref, err)
	}
	var mode string
	if err := admin.QueryRow(`SELECT @@SESSION.sql_mode`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mode, "STRICT_TRANS_TABLES") && !strings.Contains(mode, "STRICT_ALL_TABLES") {
		t.Fatalf("the test server must run in strict mode like the default deployment, sql_mode=%q", mode)
	}
	if err := insertRawRequest(admin, map[string]any{"base_ref": strings.Repeat("a", 255)}); err != nil {
		t.Fatalf("255 characters must be accepted: %v", err)
	}
	if err := insertRawRequest(admin, map[string]any{"base_ref": strings.Repeat("a", 256)}); err == nil {
		t.Fatal("256 characters must be rejected under strict mode, not truncated")
	}
	if err := insertRawRequest(admin, map[string]any{"base_ref": nil}); err == nil {
		t.Fatal("NULL must be rejected: empty is the only way to say default")
	}

	down := contracttest.MigrationScripts(t, "mysql", "down")[contracttest.MigrationScriptIndex(t, "mysql", "down", "0099_")]
	applyScripts(t, admin, []string{down})
	var cols int
	if err := admin.QueryRow(`SELECT count(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name='requests' AND column_name='base_ref'`).Scan(&cols); err != nil || cols != 0 {
		t.Fatalf("down must drop the column: %d %v", cols, err)
	}
	var n int
	if err := admin.QueryRow(`SELECT count(*) FROM requests WHERE id = ?`, oldID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("down must keep the rows: %d %v", n, err)
	}
	applyScripts(t, admin, []string{target})
	if err := admin.QueryRow(`SELECT base_ref FROM requests WHERE id = ?`, oldID).Scan(&ref); err != nil || ref != "" {
		t.Fatalf("re-applied 0099 must default existing rows to empty: %q %v", ref, err)
	}
}

// A server in non-strict mode truncates an overlong VARCHAR silently; the repository must refuse the write itself so a
// person never ends up with a different ref from the one they chose.
func TestMySQL_BaseRef_NonStrictServerDoesNotTruncate(t *testing.T) {
	f := newMigratedMySQL(t)
	var name string
	if err := f.admin.QueryRow(`SELECT DATABASE()`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	loose, err := sql.Open("mysql", fmt.Sprintf("root:orca@tcp(%s)/%s?%s&sql_mode=%%27%%27", sharedMySQLAddr(t), name, sessionParams))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = loose.Close() })
	var mode string
	if err := loose.QueryRow(`SELECT @@SESSION.sql_mode`).Scan(&mode); err != nil || mode != "" {
		t.Fatalf("expected a non-strict session, sql_mode=%q err=%v", mode, err)
	}

	// Proof that the server itself would have truncated.
	rawID := uuid.NewString()
	if err := insertRawRequest(loose, map[string]any{"id": rawID, "base_ref": strings.Repeat("a", 300)}); err != nil {
		t.Fatalf("a non-strict server accepts the overlong value: %v", err)
	}
	var stored string
	if err := loose.QueryRow(`SELECT base_ref FROM requests WHERE id = ?`, rawID).Scan(&stored); err != nil || len(stored) != 255 {
		t.Fatalf("a non-strict server truncates to 255: %d %v", len(stored), err)
	}

	repo := NewRequestRepository(New(loose))
	ctx := contracttest.CtxForTenant(uuid.NewString())
	_, err = repo.SetBaseRef(ctx, uuid.NewString(), strings.Repeat("b", 300), 1)
	if err == nil || !strings.Contains(err.Error(), "REQUEST_BASE_REF_INVALID") {
		t.Fatalf("the repository must reject an overlong ref itself, got %v", err)
	}
}

func TestMySQL_BaseRefLockContract(t *testing.T) {
	f := newMigratedMySQL(t)
	base := New(f.db)
	contracttest.RunBaseRefLockContract(t, contracttest.BaseRefLockDeps{
		LifecycleEnv: f.lifecycleEnv(), Locker: NewRequestLocker(base), Presence: NewWorktreePresence(base), Changes: NewOpenSpecChangeRepository(base),
	})
}
