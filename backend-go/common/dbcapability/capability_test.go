package dbcapability

import "testing"

func TestDetectDialectFromDSN_Postgres(t *testing.T) {
	for _, dsn := range []string{
		"postgres://user:pass@host:5432/db",
		"postgresql://user:pass@host:5432/db",
	} {
		caps, err := DetectDialectFromDSN(dsn)
		if err != nil {
			t.Fatalf("DetectDialectFromDSN(%q): unexpected error: %v", dsn, err)
		}
		if caps.Dialect != DialectPostgres {
			t.Errorf("DetectDialectFromDSN(%q) = %q, want %q", dsn, caps.Dialect, DialectPostgres)
		}
	}
}

func TestDetectDialectFromDSN_MySQL(t *testing.T) {
	caps, err := DetectDialectFromDSN("mysql://user:pass@host:3306/db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if caps.Dialect != DialectMySQL {
		t.Errorf("got %q, want %q", caps.Dialect, DialectMySQL)
	}
}

func TestDetectDialectFromDSN_TiDBAliasesMySQL(t *testing.T) {
	caps, err := DetectDialectFromDSN("tidb://user:pass@host:4000/db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if caps.Dialect != DialectMySQL {
		t.Errorf("got %q, want %q (tidb must alias mysql, not be a third dialect)", caps.Dialect, DialectMySQL)
	}
	if caps != mysqlCaps {
		t.Errorf("tidb:// Capabilities differ from mysqlCaps: got %+v, want %+v", caps, mysqlCaps)
	}
}

func TestDetectDialectFromDSN_UnrecognizedSchemeReturnsError(t *testing.T) {
	for _, dsn := range []string{"", "sqlite://data.db", "not-a-dsn"} {
		if _, err := DetectDialectFromDSN(dsn); err == nil {
			t.Errorf("DetectDialectFromDSN(%q): expected error, got nil", dsn)
		}
	}
}

func TestCapabilities_MySQLHasNoRLSOrJSONB(t *testing.T) {
	if mysqlCaps.SupportsRLS {
		t.Error("mysqlCaps.SupportsRLS should be false — MySQL has no RLS equivalent")
	}
	if mysqlCaps.SupportsJSONB {
		t.Error("mysqlCaps.SupportsJSONB should be false — MySQL has no JSONB operators")
	}
}
