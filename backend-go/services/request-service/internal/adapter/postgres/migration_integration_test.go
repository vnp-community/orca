//go:build integration

package postgres

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func tableExists(t *testing.T, conn *pgx.Conn, table string) bool {
	t.Helper()
	var n int
	err := conn.QueryRow(context.Background(),
		`SELECT count(*) FROM information_schema.tables WHERE table_schema = 'request' AND table_name = $1`, table).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n == 1
}

// Every table in the request schema must have RLS enabled and forced, or the table owner bypasses it.
func assertAllTablesForceRLS(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	rows, err := conn.Query(context.Background(), `
		SELECT c.relname, c.relrowsecurity, c.relforcerowsecurity
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'request' AND c.relkind = 'r'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var name string
		var enabled, forced bool
		if err := rows.Scan(&name, &enabled, &forced); err != nil {
			t.Fatal(err)
		}
		count++
		if !enabled || !forced {
			t.Errorf("table %s: relrowsecurity=%v relforcerowsecurity=%v, want both true", name, enabled, forced)
		}
	}
	if count < 15 {
		t.Errorf("only %d tables found in schema request", count)
	}
}

func TestPostgres_Migration_UpDownUp(t *testing.T) {
	_, admin := startPostgres(t)
	ups := contracttest.MigrationScripts(t, "postgres", "up")
	downs := contracttest.MigrationScripts(t, "postgres", "down")
	if len(ups) != len(downs) {
		t.Fatalf("%d up vs %d down scripts", len(ups), len(downs))
	}

	applyScripts(t, admin, ups)
	assertAllTablesForceRLS(t, admin)

	// Down of everything above 0001 (downs are descending, 0001 is last) leaves 0001 intact.
	applyScripts(t, admin, downs[:len(downs)-1])
	if !tableExists(t, admin, "outbox_events") || !tableExists(t, admin, "processed_events") {
		t.Fatal("0001 tables must survive the down of later migrations")
	}
	if tableExists(t, admin, "requests") || tableExists(t, admin, "approvals") {
		t.Fatal("later tables must be gone after their down migrations")
	}

	applyScripts(t, admin, ups[1:])
	assertAllTablesForceRLS(t, admin)

	applyScripts(t, admin, downs)
	var n int
	if err := admin.QueryRow(context.Background(), `SELECT count(*) FROM information_schema.schemata WHERE schema_name = 'request'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("schema should be gone after full down: n=%d err=%v", n, err)
	}
	applyScripts(t, admin, ups)
	assertAllTablesForceRLS(t, admin)
}

// insertRequestRow inserts a valid row with the given column overrides, using a fresh tenant
// so the (tenant_id, number) key never collides between probes.
func insertRequestRow(conn *pgx.Conn, overrides map[string]any) error {
	cols := []string{"id", "tenant_id", "number", "title", "source_provider", "status", "urgency", "reporter_id"}
	vals := map[string]any{
		"id": uuid.NewString(), "tenant_id": uuid.NewString(), "number": int64(1), "title": "t",
		"source_provider": "manual", "status": "new", "urgency": "normal", "reporter_id": uuid.NewString(),
	}
	for k, v := range overrides {
		if _, ok := vals[k]; !ok {
			cols = append(cols, k)
		}
		vals[k] = v
	}
	args := make([]any, len(cols))
	placeholders := make([]string, len(cols))
	for i, c := range cols {
		args[i] = vals[c]
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	_, err := conn.Exec(context.Background(),
		"INSERT INTO request.requests ("+strings.Join(cols, ", ")+") VALUES ("+strings.Join(placeholders, ", ")+")", args...)
	return err
}

func TestPostgres_Migration_ChecksRejectInvalidValues(t *testing.T) {
	f := newMigratedPostgres(t)
	ctx := context.Background()

	if err := insertRequestRow(f.admin, nil); err != nil {
		t.Fatalf("baseline row must be valid: %v", err)
	}
	cases := []struct {
		name      string
		overrides map[string]any
	}{
		{"type", map[string]any{"type": "foo"}},
		{"status", map[string]any{"status": "bar"}},
		{"confidence high", map[string]any{"confidence": 1.5}},
		{"confidence low", map[string]any{"confidence": -0.1}},
		{"urgency", map[string]any{"urgency": "x"}},
		{"size", map[string]any{"size": "XL"}},
		{"type_source", map[string]any{"type_source": "robot"}},
		{"returned_from_stage", map[string]any{"returned_from_stage": "z", "status": "request_backlog"}},
		{"source_provider", map[string]any{"source_provider": "svn"}},
		{"solution_engine", map[string]any{"solution_engine": "magic"}},
		{"backlog without stage", map[string]any{"status": "request_backlog"}},
		{"stage without backlog", map[string]any{"returned_from_stage": "plan"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := insertRequestRow(f.admin, c.overrides); err == nil {
				t.Fatalf("%v was accepted", c.overrides)
			}
		})
	}

	tenantID, id := uuid.NewString(), uuid.NewString()
	for name, stmt := range map[string]string{
		"self link":        `INSERT INTO request.request_links (tenant_id, parent_request_id, child_request_id, reason) VALUES ($1, $2, $2, 'blocks')`,
		"empty source_ref": `INSERT INTO request.request_idempotency (tenant_id, source_provider, source_site, source_ref, request_id) VALUES ($1, 'manual', '', '', $2)`,
		"bad link reason":  `INSERT INTO request.request_links (tenant_id, parent_request_id, child_request_id, reason) VALUES ($1, $2, gen_random_uuid(), 'bogus')`,
		"bad actor kind":   `INSERT INTO request.request_type_history (tenant_id, request_id, to_type, actor_id, actor_kind) VALUES ($1, $2, 'bug', gen_random_uuid(), 'robot')`,
	} {
		if _, err := f.admin.Exec(ctx, stmt, tenantID, id); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
