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

func tableCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM information_schema.tables WHERE table_schema = DATABASE()`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func hasTable(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`, name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func TestMySQL_Migration_UpDownUp(t *testing.T) {
	_, admin := startMySQL(t)
	ups := contracttest.MigrationScripts(t, "mysql", "up")
	downs := contracttest.MigrationScripts(t, "mysql", "down")
	if len(ups) != len(downs) {
		t.Fatalf("%d up vs %d down scripts", len(ups), len(downs))
	}
	applyScripts(t, admin, ups)
	full := tableCount(t, admin)

	applyScripts(t, admin, downs[:len(downs)-1])
	if !hasTable(t, admin, "outbox_events") || !hasTable(t, admin, "processed_events") || hasTable(t, admin, "requests") || hasTable(t, admin, "approvals") {
		t.Fatal("down of later migrations must keep 0001 tables and drop the rest")
	}
	applyScripts(t, admin, ups[1:])
	if tableCount(t, admin) != full {
		t.Fatal("re-applying up changed the table count")
	}
	applyScripts(t, admin, downs)
	if n := tableCount(t, admin); n != 0 {
		t.Fatalf("full down left %d tables", n)
	}
	applyScripts(t, admin, ups)
	if tableCount(t, admin) != full {
		t.Fatal("up after full down changed the table count")
	}
}

func TestMySQL_Migration_ChecksRejectInvalidValues(t *testing.T) {
	var minor string
	f := newMigratedMySQL(t)
	if err := f.admin.QueryRow(`SELECT VERSION()`).Scan(&minor); err != nil {
		t.Fatal(err)
	}
	// CHECK is enforced from 8.0.16; older servers accept anything silently.
	var major, mid, patch int
	_, _ = fmt.Sscanf(strings.SplitN(minor, "-", 2)[0], "%d.%d.%d", &major, &mid, &patch)
	if major < 8 || (major == 8 && mid == 0 && patch < 16) {
		t.Skipf("MySQL %s does not enforce CHECK constraints", minor)
	}

	insert := func(overrides map[string]any) error {
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
		_, err := f.admin.Exec("INSERT INTO requests ("+strings.Join(cols, ", ")+") VALUES ("+strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",")+")", args...)
		return err
	}
	if err := insert(nil); err != nil {
		t.Fatalf("baseline row must be valid: %v", err)
	}
	cases := []struct {
		name      string
		overrides map[string]any
	}{
		{"type", map[string]any{"type": "foo"}},
		{"status", map[string]any{"status": "bar"}},
		{"confidence high", map[string]any{"confidence": 1.5}},
		{"urgency", map[string]any{"urgency": "x"}},
		{"size", map[string]any{"size": "XL"}},
		{"type_source", map[string]any{"type_source": "robot"}},
		{"returned_from_stage", map[string]any{"returned_from_stage": "z", "status": "request_backlog"}},
		{"source_provider", map[string]any{"source_provider": "svn"}},
		{"backlog without stage", map[string]any{"status": "request_backlog"}},
		{"stage without backlog", map[string]any{"returned_from_stage": "plan"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := insert(c.overrides); err == nil {
				t.Fatalf("%v was accepted", c.overrides)
			}
		})
	}
	tenantID, id := uuid.NewString(), uuid.NewString()
	for name, stmt := range map[string]string{
		"self link":        `INSERT INTO request_links (tenant_id, parent_request_id, child_request_id, reason) VALUES (?, ?, ?, 'blocks')`,
		"bad link reason":  `INSERT INTO request_links (tenant_id, parent_request_id, child_request_id, reason) VALUES (?, ?, ?, 'bogus')`,
		"empty source_ref": `INSERT INTO request_idempotency (tenant_id, source_provider, source_site, source_ref, request_id) VALUES (?, 'manual', ?, '', ?)`,
	} {
		child := id
		if name == "bad link reason" {
			child = uuid.NewString()
		}
		if _, err := f.admin.Exec(stmt, tenantID, id, child); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
