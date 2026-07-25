# TASK-DB-026: Cập nhật `src/server/index.ts` — pass ORCA_DB_URL vào bootstrap ✅ DONE

**Source:** SOL-DB-004  
**Phase:** 4 | **Effort:** XS (< 20 min) | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-DB-024

---

## Objective

`src/server/index.ts` là entry point của server mode. Cần đảm bảo `ORCA_DB_URL` env var được log khi server khởi động, và truyền `database: null` khi muốn dùng default SQLite.

---

## Context cần đọc TRƯỚC

```bash
cat src/server/index.ts
```

---

## Modification

Trong `src/server/index.ts`, KHÔNG cần truyền `database` option nếu đã dùng `loadDatabaseConfig()` trong `server-bootstrap.ts`. Nhưng cần đảm bảo ORCA_DB_URL được log ở startup:

```typescript
// Thêm vào startup log (trước khi gọi initializeOrcaServices):
const dbUrl = process.env['ORCA_DB_URL']
const dbDialect = process.env['ORCA_DB_DIALECT']

if (dbUrl) {
  // Mask password for logging
  const { formatDsn, parseDsn } = await import('../main/db/dsn-parser')
  try {
    const config = parseDsn(dbUrl)
    console.log(`[Server] Database: ${formatDsn(config)}`)  // password masked
  } catch {
    console.log(`[Server] Database: ORCA_DB_URL is set (invalid DSN — will error on connect)`)
  }
} else if (dbDialect) {
  console.log(`[Server] Database dialect: ${dbDialect}`)
} else {
  console.log('[Server] Database: SQLite (default)')
}
```

---

## Verification

```bash
# Verify env var is logged at startup
ORCA_DB_URL=mysql://user:secret@localhost/orca timeout 3 node out/server/index.js 2>&1 | grep -i database
# Expected: shows masked URL (no 'secret' in output)
```

---

## Done criteria

- [x] `src/server/index.ts` logs DB info at startup
- [x] Password is masked in log output
- [x] No new TypeScript errors
