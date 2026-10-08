package usecase

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// guardedFields may only be assigned in transition_request.go (and tests): that is how one use case stays the sole writer of status.
var guardedFields = map[string]bool{"Status": true, "ReturnedFromStage": true, "ReturnReason": true, "ReturnedCategory": true}

const statusWriter = "transition_request.go"

func TestOnlyTransitionRequestWritesStatus(t *testing.T) {
	violations := scanStatusWrites(t, ".")
	for _, v := range violations {
		t.Errorf("status field written outside %s: %s", statusWriter, v)
	}
}

func scanStatusWrites(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	fset := token.NewFileSet()
	for _, f := range files {
		base := filepath.Base(f)
		if strings.HasSuffix(base, "_test.go") || base == statusWriter {
			continue
		}
		parsed, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		report := func(pos token.Pos, what string) {
			out = append(out, fset.Position(pos).String()+" "+what)
		}
		checkLHS := func(e ast.Expr) {
			if sel, ok := e.(*ast.SelectorExpr); ok && guardedFields[sel.Sel.Name] {
				report(sel.Pos(), "assigns ."+sel.Sel.Name)
			}
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				for _, l := range x.Lhs {
					checkLHS(l)
				}
			case *ast.IncDecStmt:
				checkLHS(x.X)
			case *ast.CompositeLit:
				if isDomainRequest(x.Type) {
					for _, el := range x.Elts {
						if kv, ok := el.(*ast.KeyValueExpr); ok {
							if id, ok := kv.Key.(*ast.Ident); ok && guardedFields[id.Name] {
								report(id.Pos(), "sets "+id.Name+" in a domain.Request literal")
							}
						}
					}
				}
			}
			return true
		})
	}
	return out
}

func isDomainRequest(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Request" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "domain"
}

// The guard must actually catch a violation, or a silent parser regression would pass it.
func TestStatusWriteGuard_DetectsViolation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "zz_bad.go"), "package usecase\nimport \"x/domain\"\nfunc f(r *domain.Request) { r.Status = \"new\"; _ = domain.Request{ReturnReason: \"x\"} }\n")
	writeFile(t, filepath.Join(dir, "transition_request.go"), "package usecase\nfunc g(r *R) { r.Status = \"ok\" }\n")
	got := scanStatusWrites(t, dir)
	if len(got) != 2 {
		t.Fatalf("want 2 violations (assignment + literal), got %v", got)
	}
}
