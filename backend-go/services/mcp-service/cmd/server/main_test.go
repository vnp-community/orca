package main

import (
	"strings"
	"testing"
)

func TestRequirePostgres(t *testing.T) {
	for _, dsn := range []string{"postgres://u:p@h:5432/mcp", "postgresql://u:p@h/mcp?sslmode=disable"} {
		if err := requirePostgres(dsn); err != nil {
			t.Errorf("%s: unexpected error %v", dsn, err)
		}
	}
	for _, dsn := range []string{"mysql://u:p@h:3306/mcp", "tidb://u:p@h:4000/mcp"} {
		err := requirePostgres(dsn)
		if err == nil || !strings.Contains(err.Error(), "PostgreSQL only") {
			t.Errorf("%s: want PostgreSQL-only error, got %v", dsn, err)
		}
	}
	if requirePostgres("sqlite://x") == nil || requirePostgres("") == nil {
		t.Error("unknown/empty DSN must fail")
	}
}
