//go:build integration

package mysql

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func TestMySQL_SchemaContract_InformationSchema(t *testing.T) {
	f := newMigratedMySQL(t)
	for table, want := range contracttest.ExpectedColumns() {
		rows, err := f.admin.Query(`SELECT column_name, is_nullable = 'YES' FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND extra NOT LIKE '%STORED GENERATED%' AND extra NOT LIKE '%VIRTUAL GENERATED%'`, table) // generated keys (open_key, live_key) are index plumbing, not part of the contract
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]bool{}
		for rows.Next() {
			var name string
			var nullable bool
			if err := rows.Scan(&name, &nullable); err != nil {
				t.Fatal(err)
			}
			got[name] = nullable
		}
		_ = rows.Close()
		if len(got) != len(want) {
			t.Errorf("%s: %d columns in DB, want %d (%v)", table, len(got), len(want), got)
		}
		for _, c := range want {
			nullable, ok := got[c.Name]
			if !ok || nullable != c.Nullable {
				t.Errorf("%s.%s: present=%v nullable=%v, want nullable=%v", table, c.Name, ok, nullable, c.Nullable)
			}
		}
	}
}
