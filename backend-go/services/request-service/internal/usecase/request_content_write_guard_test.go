package usecase

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// Content columns of a Request change only through AppendRequestRevision (CR-REQ-027 section 2.4), the way
// status_write_guard_test.go keeps TransitionRequest the only writer of status. The check is syntactic (no type
// information): the column fields have unique names, and Title and Body are matched only on the variable
// names a domain.Request is held in, so a CreateRequestInput.Title assignment does not trip it.
var (
	uniqueContentFields = map[string]bool{
		"AcceptanceCriteriaJSON": true, "TypeFieldsJSON": true, "ContentRevision": true, "ContentDigest": true, "ContentSchemaVersion": true,
	}
	sharedContentFields = map[string]bool{"Title": true, "Body": true}
	requestVariables    = map[string]bool{"r": true, "req": true, "request": true, "updated": true, "next": true, "got": true, "cur": true, "stored": true}
	contentWriters      = map[string]bool{"append_request_revision.go": true}
)

func scanContentWrites(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	fset := token.NewFileSet()
	for _, f := range files {
		base := filepath.Base(f)
		if strings.HasSuffix(base, "_test.go") || contentWriters[base] {
			continue
		}
		parsed, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		report := func(pos token.Pos, what string) { out = append(out, fset.Position(pos).String()+" "+what) }
		checkLHS := func(e ast.Expr) {
			sel, ok := e.(*ast.SelectorExpr)
			if !ok {
				return
			}
			switch {
			case uniqueContentFields[sel.Sel.Name]:
				report(sel.Pos(), "assigns ."+sel.Sel.Name)
			case sharedContentFields[sel.Sel.Name]:
				if id, ok := sel.X.(*ast.Ident); ok && requestVariables[id.Name] {
					report(sel.Pos(), "assigns "+id.Name+"."+sel.Sel.Name)
				}
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
							if id, ok := kv.Key.(*ast.Ident); ok && (uniqueContentFields[id.Name] || sharedContentFields[id.Name]) {
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

func TestContentWriteGuard(t *testing.T) {
	for _, v := range scanContentWrites(t, ".") {
		t.Errorf("request content written outside append_request_revision.go: %s", v)
	}
}

func TestContentWriteGuard_CatchesAViolation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.go"), "package usecase\nfunc f(r R) { r.Title = \"x\"; r.ContentRevision++; _ = domain.Request{Body: \"y\"} }\n")
	writeFile(t, filepath.Join(dir, "fine.go"), "package usecase\nfunc g(in In) { in.Title = \"x\" }\n")
	got := scanContentWrites(t, dir)
	if len(got) != 3 {
		t.Fatalf("want 3 violations in bad.go and none in fine.go, got %v", got)
	}
}
