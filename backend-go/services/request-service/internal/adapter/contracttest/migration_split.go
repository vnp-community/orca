package contracttest

import (
	"strings"
	"testing"
)

// SplitMigrationsAt splits ordered up scripts at the first one that contains marker, so a test can load data written
// before a migration and read it after. The script holding marker starts the second part.
func SplitMigrationsAt(t *testing.T, scripts []string, marker string) (before, from []string) {
	t.Helper()
	for i, s := range scripts {
		if strings.Contains(s, marker) {
			return scripts[:i], scripts[i:]
		}
	}
	t.Fatalf("no migration script contains %q", marker)
	return nil, nil
}
