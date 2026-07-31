# TASK-003: Update migrations/index.ts

**Phase:** 1 — Foundation  
**Solution ref:** [SOL-V5-000](../solutions/SOL-V5-000-server-bootstrap-changes.md) §4  
**Prerequisite:** TASK-001 (5 migration files phải tồn tại)  
**Status:** ✅ DONE — 2026-07-28

> **Kết quả:** 5 imports + 5 entries added. `ALL_MIGRATIONS` now has 10 migrations. 167/167 tests pass.


---

## Mô tả

Cập nhật `src/main/db/migrations/index.ts` để register 5 migration mới (0006–0010) vào `ALL_MIGRATIONS` array. Đây là **file modify duy nhất** trong TASK-001~003.

---

## File cần đọc trước

```bash
cat src/main/db/migrations/index.ts
```

Expected current content:
```typescript
import { migration0001InitialSchema } from './0001_initial_schema'
import { migration0002AddAutomations } from './0002_add_automations'
import { migration0003AddWorkspaceSessions } from './0003_add_workspace_sessions'
import { migration0004OrcaAppTables } from './0004_orca_app_tables'
import { migration0005AddAuthSchema } from './0005_add_auth_schema'
import type { Migration } from './types'

export const ALL_MIGRATIONS: Migration[] = [
  migration0001InitialSchema,
  migration0002AddAutomations,
  migration0003AddWorkspaceSessions,
  migration0004OrcaAppTables,
  migration0005AddAuthSchema,
]
```

---

## Thay đổi cần thực hiện

Thêm 5 imports mới và 5 entries vào array:

```typescript
import { migration0001InitialSchema } from './0001_initial_schema'
import { migration0002AddAutomations } from './0002_add_automations'
import { migration0003AddWorkspaceSessions } from './0003_add_workspace_sessions'
import { migration0004OrcaAppTables } from './0004_orca_app_tables'
import { migration0005AddAuthSchema } from './0005_add_auth_schema'
// [NEW v5.0]
import { migration0006CompanyDept } from './0006_company_dept'
import { migration0007Projects } from './0007_projects'
import { migration0008AiProviders } from './0008_ai_providers'
import { migration0009Workflows } from './0009_workflows'
import { migration0010Tasks } from './0010_tasks'
import type { Migration } from './types'

export const ALL_MIGRATIONS: Migration[] = [
  migration0001InitialSchema,
  migration0002AddAutomations,
  migration0003AddWorkspaceSessions,
  migration0004OrcaAppTables,
  migration0005AddAuthSchema,
  migration0006CompanyDept,   // [NEW v5.0]
  migration0007Projects,      // [NEW v5.0]
  migration0008AiProviders,   // [NEW v5.0]
  migration0009Workflows,     // [NEW v5.0]
  migration0010Tasks,         // [NEW v5.0]
]
```

---

## Verification

```bash
# TypeScript check
pnpm tsc --noEmit

# Confirm migrations register
grep -c 'migration' src/main/db/migrations/index.ts
# Expected: 10 lines with "migration"
```

## Acceptance Criteria

- [x] 5 import statements mới thêm vào
- [x] 5 entries mới trong `ALL_MIGRATIONS` array
- [x] Thứ tự migrations đúng (0001 → 0010)
- [x] Không TypeScript errors
