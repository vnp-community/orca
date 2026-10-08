package contracttest

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// MigrationScripts returns the migration SQL for a dialect directory ("postgres" or "mysql"),
// up scripts in ascending order or down scripts in descending order.
func MigrationScripts(t *testing.T, dialectDir, direction string) []string {
	t.Helper()
	dir := findMigrationsDir(t, dialectDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "."+direction+".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if direction == "down" {
		sort.Sort(sort.Reverse(sort.StringSlice(names)))
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, string(b))
	}
	if len(out) == 0 {
		t.Fatalf("no %s migrations found in %s", direction, dir)
	}
	return out
}

// findMigrationsDir walks up from the test's working directory so callers in any package can use it.
func findMigrationsDir(t *testing.T, dialectDir string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "migrations", dialectDir)
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("migrations/%s not found above the working directory", dialectDir)
	return ""
}

// MigrationUpScriptsSplit returns the up scripts below number (e.g. "0020") and the script of
// exactly that number, so a test can seed rows between them to prove a backfill.
func MigrationUpScriptsSplit(t *testing.T, dialectDir, number string) (before []string, target string) {
	t.Helper()
	dir := findMigrationsDir(t, dialectDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case strings.HasPrefix(n, number+"_"):
			target = string(b)
		case n < number:
			before = append(before, string(b))
		}
	}
	if target == "" {
		t.Fatalf("no up migration numbered %s in %s", number, dir)
	}
	return before, target
}

// MigrationScriptIndex is the position of the script whose file name starts with prefix (for example "0060_")
// in the slice MigrationScripts returns, so tests do not count from the end as migrations are added.
func MigrationScriptIndex(t *testing.T, dialectDir, direction, prefix string) int {
	t.Helper()
	entries, err := os.ReadDir(findMigrationsDir(t, dialectDir))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "."+direction+".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if direction == "down" {
		sort.Sort(sort.Reverse(sort.StringSlice(names)))
	}
	for i, n := range names {
		if strings.HasPrefix(n, prefix) {
			return i
		}
	}
	t.Fatalf("no %s migration starting with %s", direction, prefix)
	return -1
}
