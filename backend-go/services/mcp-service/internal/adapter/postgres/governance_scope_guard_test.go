package postgres

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// Same guard as tenant_scope_guard_test.go for the governance repository
// files: a method outside withTenantTx/withRelayTx would silently see nothing
// under FORCE RLS (or, worse, be tempted to bypass it).
func TestEveryGovernanceRepositoryMethodRunsInsideAScopedTx(t *testing.T) {
	for _, f := range []string{"policy_repository.go", "approval_repository.go", "tool_call_repository.go", "kill_switch_repository.go"} {
		file, err := parser.ParseFile(token.NewFileSet(), f, nil, 0)
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
				if sel, ok := n.(*ast.SelectorExpr); ok && (sel.Sel.Name == "withTenantTx" || sel.Sel.Name == "withRelayTx" || sel.Sel.Name == "findOpenApproval") {
					scoped = true
				}
				return true
			})
			if !scoped {
				t.Errorf("%s: Repository.%s does not use withTenantTx/withRelayTx", f, fn.Name.Name)
			}
			checked++
		}
		if checked == 0 {
			t.Fatalf("%s: no methods inspected", f)
		}
	}
}

// Every governance table must carry a tenant policy; this is what keeps a
// forgotten WHERE tenant_id from becoming a cross-tenant read.
func TestMigrationsEnableForcedRLSOnEveryGovernanceTable(t *testing.T) {
	for _, tc := range []struct{ file, table string }{
		{"0003_tool_policies.up.sql", "tool_policies"}, {"0003_tool_policies.up.sql", "tool_policy_revisions"},
		{"0004_approvals_audit_killswitch.up.sql", "approvals"}, {"0004_approvals_audit_killswitch.up.sql", "kill_switches"},
		{"0004_approvals_audit_killswitch.up.sql", "tool_calls"}, {"0004_approvals_audit_killswitch.up.sql", "taint"},
	} {
		sql := readMigrationFile(t, tc.file)
		if !containsAll(sql, "'"+tc.table+"'", "FORCE ROW LEVEL SECURITY", "tenant_isolation") {
			t.Errorf("%s: %s lacks forced RLS wiring", tc.file, tc.table)
		}
	}
}
