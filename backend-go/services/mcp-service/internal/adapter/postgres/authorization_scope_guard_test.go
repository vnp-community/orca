package postgres

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// Same guard as tenant_scope_guard_test.go, for the consent/grant repository
// file: a method that skips withTenantTx/withRelayTx would silently return
// nothing under FORCE RLS.
func TestEveryAuthorizationRepositoryMethodRunsInsideAScopedTx(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "authorization_repository.go", nil, 0)
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
			return true
		})
		if !scoped {
			t.Errorf("Repository.%s does not use withTenantTx/withRelayTx", fn.Name.Name)
		}
		checked++
	}
	if checked < 10 {
		t.Fatalf("inspected only %d methods", checked)
	}
}
