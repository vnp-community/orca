package contracttest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var (
	stmtStart   = regexp.MustCompile(`(?is)^\s*(SELECT|UPDATE|DELETE\s+FROM|INSERT\s+INTO|WITH)\b`)
	insertStart = regexp.MustCompile(`(?is)^\s*INSERT\s+INTO\s+\S+\s*\(([^)]*)\)`)
	tenantBound = regexp.MustCompile(`(?i)\btenant_id\s*(=|IN\b)`)
)

// CheckTenantScope parses every non-test Go file of an adapter directory and requires each SQL statement literal
// to be tenant scoped: a WHERE/ON bound on tenant_id, or tenant_id in the INSERT column list. Statements that must
// run across tenants (relays, sweepers) are listed in allowed as "file.go:Function" with the reason. It is a lint of the
// source, not a proof: SQL built from non-literal pieces is invisible to it (the repo convention is literals).
func CheckTenantScope(t *testing.T, dir string, allowed map[string]string) {
	t.Helper()
	problems, scanned, err := ScanTenantScope(dir, allowed)
	if err != nil {
		t.Fatal(err)
	}
	if scanned == 0 {
		t.Fatalf("no SQL statements found under %s: the scan itself is broken", dir)
	}
	for _, p := range problems {
		t.Error(p)
	}
}

// ScanTenantScope returns the problems found and how many statements it looked at.
func ScanTenantScope(dir string, allowed map[string]string) (problems []string, scanned int, err error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, 0, err
	}
	used := map[string]bool{}
	var offenders []string
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, 0, err
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			return nil, 0, err
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			key := filepath.Base(path) + ":" + fn.Name.Name
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				text, pos, isString := statementText(n)
				if !isString {
					return true
				}
				// A concatenation is one statement; its literal pieces are not checked on their own.
				if !stmtStart.MatchString(text) {
					return false
				}
				scanned++
				if scoped(text) {
					return false
				}
				if _, ok := allowed[key]; ok {
					used[key] = true
					return false
				}
				offenders = append(offenders, "SQL without a tenant_id bound (scope it or allow-list it with a reason): "+key+" at "+fset.Position(pos).String())
				return false
			})
		}
	}
	sort.Strings(offenders)
	problems = append(problems, offenders...)
	keys := make([]string, 0, len(allowed))
	for k := range allowed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if allowed[key] == "" {
			problems = append(problems, "allow-list entry "+key+" has no reason")
		}
		if !used[key] {
			problems = append(problems, "stale allow-list entry "+key+": the function no longer has an unscoped statement")
		}
	}
	return problems, scanned, nil
}

func scoped(stmt string) bool {
	if m := insertStart.FindStringSubmatch(stmt); m != nil {
		return strings.Contains(strings.ToLower(m[1]), "tenant_id")
	}
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmt)), "INSERT") {
		return false
	}
	return tenantBound.MatchString(stmt)
}

// statementText returns the joined literal text of a string literal or of a `"a" + x + "b"` chain.
func statementText(n ast.Node) (string, token.Pos, bool) {
	switch x := n.(type) {
	case *ast.BasicLit:
		if x.Kind != token.STRING {
			return "", 0, false
		}
		v, err := strconv.Unquote(x.Value)
		return v, x.Pos(), err == nil
	case *ast.BinaryExpr:
		if x.Op != token.ADD {
			return "", 0, false
		}
		var sb strings.Builder
		found := false
		var walk func(e ast.Expr)
		walk = func(e ast.Expr) {
			switch y := e.(type) {
			case *ast.BinaryExpr:
				if y.Op == token.ADD {
					walk(y.X)
					walk(y.Y)
				}
			case *ast.BasicLit:
				if y.Kind == token.STRING {
					if v, err := strconv.Unquote(y.Value); err == nil {
						sb.WriteString(v)
						found = true
					}
				}
			}
		}
		walk(x)
		return sb.String(), x.Pos(), found
	}
	return "", 0, false
}
