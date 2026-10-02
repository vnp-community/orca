package postgres

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// Same guard as tenant_scope_guard_test.go for the external server registry
// repository: a method that skips withTenantTx/withRelayTx would silently see
// nothing under FORCE RLS.
func TestEveryExternalServerRepositoryMethodRunsInsideAScopedTx(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "external_server_repository.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || !fn.Name.IsExported() {
			continue
		}
		scoped := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && (sel.Sel.Name == "withTenantTx" || sel.Sel.Name == "withRelayTx") {
				scoped = true
			}
			// queryServers is the shared scoped helper.
			if id, ok := n.(*ast.Ident); ok && id.Name == "queryServers" {
				scoped = true
			}
			return true
		})
		if !scoped {
			t.Errorf("Repository.%s does not use a scoped tx", fn.Name.Name)
		}
		checked++
	}
	if checked < 10 {
		t.Fatalf("inspected only %d methods", checked)
	}
}

func TestExternalServersMigrationHasNoSecretValueColumn(t *testing.T) {
	up := strings.ToLower(readMigrationFile(t, "0007_external_servers.up.sql"))
	for _, bad := range []string{"secret_value", "plaintext", "ciphertext", "password"} {
		if strings.Contains(up, bad) {
			t.Errorf("migration mentions %q", bad)
		}
	}
	if !containsAll(up, "force row level security", "tenant_isolation", "external_server_tools_history", "external_server_secret_refs") {
		t.Error("RLS/tables missing")
	}
}
