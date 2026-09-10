package main

import (
	"strings"
	"testing"
)

// TestToMySQLDriverDSN_ConvertsURLFormat covers the plain-URL DSN shape
// (host NOT already wrapped as "tcp(...)"). NOTE: deviates from this
// task's original expected value of an exact
// "user:pass@tcp(host:3306)/usage" match — this implementation always
// appends "?parseTime=true" when absent (required for TIMESTAMP columns to
// scan into time.Time/sql.NullTime, discovered implementing
// internal/adapter/mysql.Repository.ListSessions in TASK-BE-DB-005), so
// this test checks a prefix + substring instead of an exact string.
func TestToMySQLDriverDSN_ConvertsURLFormat(t *testing.T) {
	got, err := toMySQLDriverDSN("mysql://user:pass@host:3306/usage")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	const wantPrefix = "user:pass@tcp(host:3306)/usage"
	if !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("toMySQLDriverDSN() = %q, want prefix %q", got, wantPrefix)
	}
	if !strings.Contains(got, "parseTime=true") {
		t.Errorf("toMySQLDriverDSN() = %q, want it to contain parseTime=true", got)
	}
}

// TestToMySQLDriverDSN_ConvertsAlreadyWrappedTCPFormat covers the DSN shape
// this service's own DATABASE_DSN convention actually uses throughout
// specs/backend-go/crs/v4/multi-database (host pre-wrapped as
// "tcp(host:port)", e.g. common/testutil.StartMySQL's return value and
// this task's own Verify section example) — net/url.Parse cannot parse
// this shape at all (confirmed empirically: it rejects the literal
// parentheses with "invalid port"), so it's handled by a scheme-prefix
// strip instead of URL parsing.
func TestToMySQLDriverDSN_ConvertsAlreadyWrappedTCPFormat(t *testing.T) {
	got, err := toMySQLDriverDSN("mysql://root:orca@tcp(localhost:3307)/usage")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	const wantPrefix = "root:orca@tcp(localhost:3307)/usage"
	if !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("toMySQLDriverDSN() = %q, want prefix %q", got, wantPrefix)
	}
}

func TestToMySQLDriverDSN_TiDBSchemeConvertsSameAsMySQL(t *testing.T) {
	gotMySQL, err := toMySQLDriverDSN("mysql://user:pass@host:4000/usage")
	if err != nil {
		t.Fatalf("unexpected error (mysql://): %v", err)
	}
	gotTiDB, err := toMySQLDriverDSN("tidb://user:pass@host:4000/usage")
	if err != nil {
		t.Fatalf("unexpected error (tidb://): %v", err)
	}
	if gotMySQL != gotTiDB {
		t.Errorf("tidb:// and mysql:// produced different driver DSNs: %q vs %q", gotTiDB, gotMySQL)
	}
}

func TestToMySQLDriverDSN_RejectsUnparsableDSN(t *testing.T) {
	for _, dsn := range []string{"", "not-a-dsn", "postgres://user:pass@host:5432/db"} {
		if _, err := toMySQLDriverDSN(dsn); err == nil {
			t.Errorf("toMySQLDriverDSN(%q): expected error, got nil", dsn)
		}
	}
}
