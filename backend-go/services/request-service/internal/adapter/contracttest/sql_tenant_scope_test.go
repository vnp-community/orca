package contracttest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSource(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "repo.go"), []byte("package x\n"+body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// The scan goes red when someone adds a statement with no tenant bound.
func TestScanTenantScope_FlagsUnscopedStatements(t *testing.T) {
	cases := map[string]string{
		"select":                     "func A() { _ = `SELECT id FROM requests WHERE id = ?` }",
		"update":                     "func A() { _ = `UPDATE requests SET title = ? WHERE id = ?` }",
		"delete":                     "func A() { _ = \"DELETE FROM requests WHERE id = ?\" }",
		"insert":                     "func A() { _ = `INSERT INTO requests (id, title) VALUES (?, ?)` }",
		"concat":                     "func A(c string) { _ = `SELECT ` + c + ` FROM requests WHERE id = ?` }",
		"tenant only in select list": "func A() { _ = `SELECT tenant_id, id FROM requests WHERE id = ?` }",
	}
	for name, src := range cases {
		problems, scanned, err := ScanTenantScope(writeSource(t, src), nil)
		if err != nil || scanned != 1 || len(problems) != 1 || !strings.Contains(problems[0], "repo.go:A") {
			t.Errorf("%s: scanned=%d problems=%v err=%v", name, scanned, problems, err)
		}
	}
}

func TestScanTenantScope_AcceptsScopedStatements(t *testing.T) {
	src := "func A(c string) {\n" +
		"_ = `SELECT id FROM requests WHERE tenant_id = ? AND id = ?`\n" +
		"_ = `UPDATE requests SET title = ? WHERE id = ? AND tenant_id = ?`\n" +
		"_ = `DELETE FROM requests WHERE tenant_id = $1`\n" +
		"_ = `INSERT INTO requests (id, tenant_id, title) VALUES (?, ?, ?)`\n" +
		"_ = `SELECT ` + c + ` FROM requests WHERE tenant_id = ? AND ` + c\n" +
		"_ = `SELECT a.id FROM a JOIN b ON a.tenant_id = b.tenant_id`\n}"
	problems, scanned, err := ScanTenantScope(writeSource(t, src), nil)
	if err != nil || scanned != 6 || len(problems) != 0 {
		t.Errorf("scanned=%d problems=%v err=%v", scanned, problems, err)
	}
}

func TestScanTenantScope_AllowListNeedsReasonAndMustNotBeStale(t *testing.T) {
	dir := writeSource(t, "func Relay() { _ = `SELECT id FROM outbox WHERE published_at IS NULL` }\nfunc Scoped() { _ = `SELECT id FROM t WHERE tenant_id = ?` }")
	if p, _, _ := ScanTenantScope(dir, map[string]string{"repo.go:Relay": "relay"}); len(p) != 0 {
		t.Errorf("allowed relay should pass: %v", p)
	}
	if p, _, _ := ScanTenantScope(dir, map[string]string{"repo.go:Relay": ""}); len(p) != 1 || !strings.Contains(p[0], "no reason") {
		t.Errorf("empty reason: %v", p)
	}
	if p, _, _ := ScanTenantScope(dir, map[string]string{"repo.go:Relay": "relay", "repo.go:Scoped": "old exemption"}); len(p) != 1 || !strings.Contains(p[0], "stale") {
		t.Errorf("a function that is now scoped must leave the allow-list: %v", p)
	}
}
