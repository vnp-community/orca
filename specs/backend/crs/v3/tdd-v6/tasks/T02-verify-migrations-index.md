# T02 — Verify Migrations Index Exports 0006-0010

**Phase:** 1 (Quick Win)  
**Effort:** ~5 min  
**Depends on:** —  
**Solution ref:** [08-server-bootstrap-wiring.md](../solutions/08-server-bootstrap-wiring.md)  
**TDD ref:** TDD-14, TDD-15, TDD-16, TDD-17, TDD-18

---

## Mục tiêu

Xác nhận file `src/main/db/migrations/index.ts` export đủ **tất cả 10 migrations** (0001→0010) trong array `ALL_MIGRATIONS`.

---

## Context

Migration runner trong server-bootstrap tự động áp dụng tất cả migrations khi server start:

```typescript
const { ALL_MIGRATIONS } = await import('./db/migrations')
const runner = new MigrationRunner(db, ALL_MIGRATIONS)
await runner.migrate()
```

Nếu migrations 0006-0010 thiếu trong `ALL_MIGRATIONS`, các bảng v5.0 sẽ không được tạo → tất cả services bị lỗi.

---

## Files Cần Đọc

1. `src/main/db/migrations/index.ts` — kiểm tra ALL_MIGRATIONS
2. `src/main/db/migrations/0006_company_dept.ts` — verify tồn tại
3. `src/main/db/migrations/0007_projects.ts` — verify tồn tại
4. `src/main/db/migrations/0008_ai_providers.ts` — verify tồn tại
5. `src/main/db/migrations/0009_workflows.ts` — verify tồn tại
6. `src/main/db/migrations/0010_tasks.ts` — verify tồn tại

---

## Files Cần Sửa (nếu cần)

### `src/main/db/migrations/index.ts`

**Expected content:**

```typescript
import { migration0001 } from './0001_initial_schema'
import { migration0002 } from './0002_add_automations'
import { migration0003 } from './0003_add_workspace_sessions'
import { migration0004 } from './0004_orca_app_tables'
import { migration0005 } from './0005_add_auth_schema'
import { migration0006 } from './0006_company_dept'
import { migration0007 } from './0007_projects'
import { migration0008 } from './0008_ai_providers'
import { migration0009 } from './0009_workflows'
import { migration0010 } from './0010_tasks'

export const ALL_MIGRATIONS = [
  migration0001,
  migration0002,
  migration0003,
  migration0004,
  migration0005,
  migration0006,
  migration0007,
  migration0008,
  migration0009,
  migration0010,
]
```

Nếu thiếu migration nào → thêm import và entry vào array.

---

## Verification Steps

```bash
# 1. Check index file content
cat src/main/db/migrations/index.ts

# 2. Check all migration files exist
ls src/main/db/migrations/*.ts | grep -v "__tests__\|runner\|types\|index"

# 3. TypeScript check
pnpm tsc --noEmit
```

---

## Acceptance Criteria

- [x] `ALL_MIGRATIONS` array có đúng 10 entries (0001 → 0010) ✅
- [x] Tất cả 10 migration files tồn tại trong `src/main/db/migrations/` ✅
- [x] `pnpm tsc --noEmit` → 0 errors liên quan migrations ✅
