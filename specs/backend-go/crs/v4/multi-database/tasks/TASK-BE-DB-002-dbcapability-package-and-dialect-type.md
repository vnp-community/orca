# TASK-BE-DB-002: Package `backend-go/common/dbcapability`

**Solution:** BE-DB-SOL-001 §2 | **CR:** CR-DB-002
**Service:** `backend-go/common` (package mới, dùng chung — chưa consumer nào import ở task này)
**Depends on:** Không
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** Tạo đúng 2 file mới
> `backend-go/common/dbcapability/capability.go` và `capability_test.go`,
> nội dung khớp 1:1 với mẫu trong task doc (không cần điều chỉnh gì —
> package hoàn toàn mới, không có code thật nào từng tồn tại để lệch so
> với). `common/go.mod` **không cần sửa** — xác nhận package chỉ dùng
> `strings`/`fmt` stdlib, đúng như task đã dự đoán, không thêm dependency
> nào. Build/test đã chạy thật: `cd backend-go/common && go build ./...`
> sạch; `go test ./dbcapability/... -v` — **5/5 test PASS** (
> `TestDetectDialectFromDSN_Postgres`, `TestDetectDialectFromDSN_MySQL`,
> `TestDetectDialectFromDSN_TiDBAliasesMySQL`,
> `TestDetectDialectFromDSN_UnrecognizedSchemeReturnsError`,
> `TestCapabilities_MySQLHasNoRLSOrJSONB`); `gofmt -l dbcapability/*.go`
> không in gì (sạch). Không có consumer nào import package này ở task
> này (đúng phạm vi) nên không cần `impact()` — package mới hoàn toàn,
> không có symbol cũ bị sửa.

---

## Mục tiêu

Tạo package `Capabilities`/`Dialect` dùng chung theo đúng thiết kế
BE-DB-SOL-001 §2 — nền tảng cho mọi task sau. Task này **chỉ tạo
package**, chưa wire vào `usage-service` (đó là TASK-BE-DB-006).

## Files cần sửa

1. `backend-go/common/dbcapability/capability.go` (MỚI)
2. `backend-go/common/dbcapability/capability_test.go` (MỚI)
3. `backend-go/common/go.mod` — xác nhận không cần thêm dependency mới (package chỉ dùng `strings`/`fmt` stdlib)

## Nội dung `capability.go`

```go
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
	SupportsJSONB bool
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
```

## Test cases cần cover (`capability_test.go`)

- `TestDetectDialectFromDSN_Postgres` — `postgres://...` và `postgresql://...` cả hai trả về `DialectPostgres`.
- `TestDetectDialectFromDSN_MySQL` — `mysql://...` trả về `DialectMySQL`.
- `TestDetectDialectFromDSN_TiDBAliasesMySQL` — `tidb://...` trả về `DialectMySQL` (cùng `Capabilities`, không phải dialect thứ 3).
- `TestDetectDialectFromDSN_UnrecognizedSchemeReturnsError` — DSN không có scheme hợp lệ (vd. chuỗi rỗng, hoặc `sqlite://...` — không hỗ trợ) trả về lỗi, không panic.
- `TestCapabilities_MySQLHasNoRLSOrJSONB` — xác nhận `mysqlCaps.SupportsRLS == false && mysqlCaps.SupportsJSONB == false` (chốt hợp đồng cho TASK-BE-DB-003/004 dựa vào).

## Verify

```bash
cd backend-go/common && go build ./... && go test ./dbcapability/...
gofmt -l dbcapability/*.go
```

## gitnexus

Package hoàn toàn mới, chưa có consumer nào — không cần chạy `impact()`
trước khi TẠO (không có symbol cũ nào bị sửa). Khi task sau (TASK-BE-DB-006)
wire `DetectDialectFromDSN`/`Capabilities` vào `usage-service/cmd/server/main.go`,
task đó phải tự chạy `impact()` cho các symbol nó thực sự sửa ở đó (không
lặp lại ở đây vì task này không đụng tới `usage-service`).
