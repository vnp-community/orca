//go:build integration

package postgres

import (
	"context"
	"sort"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func TestPostgres_SchemaContract_InformationSchema(t *testing.T) {
	f := newMigratedPostgres(t)
	for table, want := range contracttest.ExpectedColumns() {
		rows, err := f.admin.Query(context.Background(),
			`SELECT column_name, is_nullable = 'YES' FROM information_schema.columns WHERE table_schema = 'request' AND table_name = $1`, table)
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
		rows.Close()
		if len(got) != len(want) {
			names := make([]string, 0, len(got))
			for n := range got {
				names = append(names, n)
			}
			sort.Strings(names)
			t.Errorf("%s: columns %v, want %d columns", table, names, len(want))
		}
		for _, c := range want {
			nullable, ok := got[c.Name]
			if !ok || nullable != c.Nullable {
				t.Errorf("%s.%s: present=%v nullable=%v, want nullable=%v", table, c.Name, ok, nullable, c.Nullable)
			}
		}
	}
}
