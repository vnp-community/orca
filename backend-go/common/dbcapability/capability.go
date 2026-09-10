// Package dbcapability describes what each supported SQL dialect can and
// cannot do, so callers (migration selection, repository adapter factory)
// branch on capability rather than repeating dialect string comparisons —
// see specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-001.
package dbcapability

import (
	"fmt"
	"strings"
)

// Dialect identifies which SQL engine a service's DATABASE_DSN points at.
type Dialect string

const (
	DialectPostgres Dialect = "postgres"
	// DialectMySQL covers both MySQL and TiDB — TiDB speaks the MySQL wire
	// protocol (see docs/adrs/v2/ADR-021-unified-postgres-microservices-platform.md
	// line 40), so one driver/capability set serves both.
	DialectMySQL Dialect = "mysql"
)

// Capabilities describes what a dialect supports.
type Capabilities struct {
	Dialect Dialect
	// PlaceholderStyle documents which SQL placeholder syntax this
	// dialect's driver expects — informational only in this package;
	// each adapter package (postgres/pgx vs mysql/database/sql) writes
	// its own SQL literally, this field is for logging/diagnostics and
	// for any future shared query-building helper.
	PlaceholderStyle string
	// SupportsRLS is false for MySQL — a service running on a dialect
	// where this is false MUST enforce tenant isolation entirely at the
	// application layer (no RLS backstop). See TASK-BE-DB-003.
	SupportsRLS bool
	// SupportsJSONB is false for MySQL — use a plain JSON column and lose
	// JSONB operators (->, @>) where a query relied on them.
	SupportsJSONB     bool
	SupportsReturning bool
}

var postgresCaps = Capabilities{
	Dialect: DialectPostgres, PlaceholderStyle: "$N",
	SupportsRLS: true, SupportsJSONB: true, SupportsReturning: true,
}

var mysqlCaps = Capabilities{
	Dialect: DialectMySQL, PlaceholderStyle: "?",
	SupportsRLS: false, SupportsJSONB: false, SupportsReturning: false,
}

// DetectDialectFromDSN reads the DSN's scheme. No new env var is
// introduced — this works directly on the DATABASE_DSN string every
// service already gets from common/secrets.DatabaseCredentialsFromFile
// (see backend-go/common/config/config.go:27 and
// backend-go/common/secrets — that function is already dialect-agnostic,
// it only returns a raw DSN string).
func DetectDialectFromDSN(dsn string) (Capabilities, error) {
	switch {
	case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
		return postgresCaps, nil
	case strings.HasPrefix(dsn, "mysql://"), strings.HasPrefix(dsn, "tidb://"):
		return mysqlCaps, nil
	default:
		return Capabilities{}, fmt.Errorf("dbcapability: unrecognized DSN scheme in %q", dsn)
	}
}
