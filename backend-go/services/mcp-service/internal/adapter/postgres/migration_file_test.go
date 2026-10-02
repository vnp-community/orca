package postgres

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readMigrationFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "postgres", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
