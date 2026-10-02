package postgres

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// A repository method that forgets withTenantTx/withRelayTx would silently
// return empty results under FORCE RLS; this catches it at unit-test time.
func TestEveryRepositoryMethodRunsInsideAScopedTx(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "repository.go", nil, 0)
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
	if checked == 0 {
		t.Fatal("no methods inspected")
	}
}
