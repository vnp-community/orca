# TASK-DB-007: Backward Compat Test cho `src/main/sqlite/sync-database.ts` ✅ DONE

**Source:** SOL-DB-001 §4.4  
**Phase:** 1 | **Effort:** XS (< 30 min) | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-DB-006

---

## Objective

Cập nhật `src/main/sqlite/sync-database.ts` để re-export `SqliteAdapter` dưới tên `SyncDatabase` — đảm bảo tất cả code hiện tại dùng `import SyncDatabase from '../sqlite/sync-database'` vẫn hoạt động không đổi.

---

## Context cần đọc

- `src/main/sqlite/sync-database.ts` — xem nội dung hiện tại trước khi sửa
- `src/main/db/sqlite/sqlite-adapter.ts` (TASK-DB-006)
- SOL-DB-001 §4.4

---

## Files to modify

### `src/main/sqlite/sync-database.ts`

**Trước khi sửa** — đọc file hiện tại để hiểu exports nào đang được dùng.

**Sau khi sửa** — thêm re-export section ở cuối file (KHÔNG xóa nội dung hiện tại nếu cần thiết, hoặc replace nếu file chỉ là wrapper):

```typescript
/**
 * SyncDatabase — Backward Compatibility Shim
 *
 * This module re-exports SqliteAdapter under the SyncDatabase name so that
 * existing imports continue to work without modification.
 *
 * MIGRATION NOTE: New code should import from 'src/main/db/sqlite/sqlite-adapter'
 * and use SqliteAdapter directly.
 *
 * WHY: src/main/sqlite/sync-database.ts was the original home for the
 * SQLite wrapper. SqliteAdapter (in src/main/db/) is the new implementation
 * that implements the IDatabase interface. This shim bridges the two.
 */

// Re-export SqliteAdapter as the default export (SyncDatabase was a default export)
export { SqliteAdapter as default } from '../db/sqlite/sqlite-adapter'

// Re-export commonly used types for callers that do:
//   import SyncDatabase, { type SqliteStatement } from '../sqlite/sync-database'
export type { ISyncDatabase as SyncDatabaseInterface } from '../db/types'
```

**QUAN TRỌNG:** Nếu `src/main/sqlite/sync-database.ts` có nội dung phức tạp hơn (không chỉ là re-export), cần:
1. Đọc file hiện tại trước
2. Chỉ thay đổi phần default export để point tới `SqliteAdapter`
3. Giữ nguyên bất kỳ types hay utilities đặc thù nào

---

## Files to create (test)

### `src/main/sqlite/__tests__/sync-database-compat.test.ts`

```typescript
/**
 * Backward compatibility test for sync-database.ts shim.
 * Verifies that existing import patterns still work after migration.
 */
import { describe, it, expect } from 'vitest'
import SyncDatabase from '../sync-database'

describe('SyncDatabase backward compat shim', () => {
  it('SyncDatabase is a constructor (class)', () => {
    expect(typeof SyncDatabase).toBe('function')
  })

  it('new SyncDatabase(":memory:") creates database', () => {
    const db = new SyncDatabase(':memory:')
    expect(db).toBeDefined()
    db.close()
  })

  it('db.exec() works', () => {
    const db = new SyncDatabase(':memory:')
    expect(() => db.exec('CREATE TABLE t (id INTEGER)')).not.toThrow()
    db.close()
  })

  it('db.prepare() returns IStatement', () => {
    const db = new SyncDatabase(':memory:')
    db.exec('CREATE TABLE t (id INTEGER)')
    const stmt = db.prepare('SELECT * FROM t')
    expect(typeof stmt.all).toBe('function')
    expect(stmt.all()).toEqual([])
    db.close()
  })

  it('db.pragma() is callable', () => {
    const db = new SyncDatabase(':memory:')
    expect(() => db.pragma('journal_mode')).not.toThrow()
    db.close()
  })

  it('db.close() works', () => {
    const db = new SyncDatabase(':memory:')
    expect(() => db.close()).not.toThrow()
  })

  it('INSERT + SELECT roundtrip', () => {
    const db = new SyncDatabase(':memory:')
    db.exec('CREATE TABLE t (id INTEGER, val TEXT)')
    db.prepare('INSERT INTO t VALUES (?, ?)').run(1, 'hello')
    const row = db.prepare('SELECT * FROM t WHERE id = ?').get(1)
    expect(row).toMatchObject({ id: 1, val: 'hello' })
    db.close()
  })
})
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

# Run compat shim tests
pnpm vitest run src/main/sqlite/__tests__/sync-database-compat.test.ts

# Verify no regression on all sqlite-related tests
pnpm vitest run src/main/sqlite/

# TypeScript compile
npx tsc --noEmit 2>&1 | grep "sqlite" | head -20

# Check no broken imports in codebase
grep -r "from.*sqlite/sync-database" src/main/ --include="*.ts" | head -20
# All above imports should still work
```

Expected:
- 7/7 compat shim tests pass
- No new TypeScript errors
- Existing code using `SyncDatabase` not broken

---

## Done criteria

- [ ] `src/main/sqlite/sync-database.ts` re-exports `SqliteAdapter` as default
- [ ] `import SyncDatabase from '../sqlite/sync-database'` còn work
- [ ] `new SyncDatabase(':memory:')` constructor works
- [ ] `db.exec()`, `db.prepare()`, `db.pragma()`, `db.close()` còn work
- [ ] `src/main/sqlite/__tests__/sync-database-compat.test.ts` pass 7 tests
- [ ] Không có TypeScript error mới
