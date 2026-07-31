# TASK-007: ProfileService CRUD

**Phase:** 2 — Profile Hierarchy  
**Solution ref:** [SOL-V5-001](../solutions/SOL-V5-001-profile-hierarchy.md) §2.2  
**Prerequisite:** TASK-001 (migration 0006), TASK-006 (OrcaProfile types)  
**Status:** ✅ DONE — 2026-07-28

---

## Mô tả

Implement `ProfileService` — CRUD service cho company, dept, user profiles. Sử dụng `IConnectionPool.query()` theo đúng pattern của `auth-session-store.ts`.

---

## File cần tạo: `src/main/profile/ProfileService.ts`

Implement đầy đủ theo [SOL-V5-001 §2.2](../solutions/SOL-V5-001-profile-hierarchy.md):

**Public API:**
- `getCompanyProfile(companyId)` → `OrcaProfile | null`
- `setCompanyProfile(companyId, profile, updatedBy)` → `void`
- `getDeptProfile(deptId)` → `OrcaProfile | null`
- `setDeptProfile(deptId, profile, updatedBy)` → `void`
- `getUserProfile(userId)` → `OrcaProfile | null`
- `setUserProfile(userId, profile)` → `void`
- `getCompanyProfileForUser(userId)` → `OrcaProfile | null` *(JOIN query)*
- `getDeptProfileForUser(userId)` → `OrcaProfile | null`
- `createCompany(name, adminUserId)` → `string` (id)
- `createDepartment(companyId, name, parentDeptId?)` → `string` (id)
- `setUserDepartment(userId, deptId)` → `void`

**Pattern** (follow auth-session-store.ts):
```typescript
import type { IConnectionPool } from '../db/pool'
import type { OrcaProfile } from './OrcaProfile'
import { randomUUID } from 'node:crypto'

export class ProfileService {
  constructor(private readonly pool: IConnectionPool) {}

  async getCompanyProfile(companyId: string): Promise<OrcaProfile | null> {
    const rows = await this.pool.query<{ profileJson: string }>(
      'SELECT profile_json as profileJson FROM orca_companies WHERE id = ?',
      [companyId]
    )
    return rows[0] ? JSON.parse(rows[0].profileJson) : null
  }

  // ... implement all methods
}
```

**Key constraints:**
- Sử dụng `JSON.parse` / `JSON.stringify` cho `profile_json` column
- `getCompanyProfileForUser` cần JOIN: `orca_users → orca_departments → orca_companies`
- `setUserProfile` dùng `INSERT ... ON CONFLICT(user_id) DO UPDATE` (upsert)
- `randomUUID()` từ `node:crypto` cho IDs

---

## Verification

```bash
pnpm tsc --noEmit
```

## Acceptance Criteria

- [x] `ProfileService` class export
- [x] Tất cả 11 methods implement
- [x] `setUserProfile` sử dụng upsert (ON CONFLICT)
- [x] `getCompanyProfileForUser` dùng JOIN
- [x] Không hardcode SQL string dài — viết readable
- [x] Không TypeScript errors
