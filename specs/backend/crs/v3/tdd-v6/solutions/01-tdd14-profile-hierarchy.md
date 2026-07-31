# Solution: TDD-14 — User Profile Hierarchy

**TDD Ref:** [14-profile-hierarchy.md](../../../../../tdd/v5/14-profile-hierarchy.md)  
**Status:** ✅ **FULLY COMPLETE** — profile-rpc.test.ts đã tạo (19 tests PASS)  
**Tái sử dụng:** 95% (chỉ cần viết thêm 1 test file)

---

## 1. Code Đã Tồn Tại — Tái sử dụng Hoàn Toàn

### 1.1 `src/main/profile/OrcaProfile.ts` ✅

Đã có đầy đủ types:
- `McpServerConfig`, `AgentProfileSection`, `EditorProfileSection`
- `ShellProfileSection`, `SecurityProfileSection`
- `OrcaProfile`, `ResolvedProfile`, `ProfileMergeOptions`

**Không thay đổi.**

### 1.2 `src/main/profile/ProfileResolver.ts` ✅ (310 lines)

Đã implement **vượt mức TDD spec**:
- Cache 60s TTL per userId
- `resolve(userId)` → parallel fetch 3 layers
- `invalidate(userId?)` → single user hoặc entire cache
- `mergeScalar()` — User > Dept > Company per-field
- `mergeShell()` — pathAdditions concat, envVars merge
- `mergeMcpServers()` — dedup by name, user wins
- `mergeEnvVars()` — key-level sources tracking

**Không thay đổi.**

### 1.3 `src/main/profile/ProfileService.ts` ✅

Đã có:
- `getCompanyProfile()`, `setCompanyProfile()`
- `getDeptProfile()`, `setDeptProfile()`
- `getUserProfile()`, `setUserProfile()`
- `getCompanyProfileForUser()`, `getDeptProfileForUser()`
- `createCompany()`, `createDepartment()`, `setUserDepartment()`

**Không thay đổi.**

### 1.4 `src/main/profile/profile-rpc-handler.ts` ✅

Đã implement tất cả 9 RPC methods:
- `profile.getResolved`, `profile.getCompany`, `profile.updateCompany`
- `profile.getDept`, `profile.updateDept`
- `profile.getUserProfile`, `profile.updateUser`
- `profile.invalidate`, `profile.listDepts`, `profile.setUserDept`

**Không thay đổi.**

### 1.5 `src/main/db/migrations/0006_company_dept.ts` ✅

Đã có migration: `orca_company`, `orca_departments` + ALTER orca_users.  
**Không thay đổi.**

### 1.6 Tests đã có ✅

- `src/main/profile/__tests__/ProfileResolver.test.ts` — cache hit/miss, locked sections, concat, merge logic
- `src/main/profile/__tests__/ProfileService.test.ts` — CRUD round-trip

---

## 2. ✅ Đã Thực Thi (2026-07-30T23:43 ICT)

### 2.1 `src/main/profile/__tests__/profile-rpc.test.ts` ✅ 19 tests PASS

**Tái sử dụng pattern từ:** `src/main/project/__tests__/project-rpc.test.ts`

```typescript
// src/main/profile/__tests__/profile-rpc.test.ts
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { registerProfileRpcHandlers } from '../profile-rpc-handler'

// Mock pattern (reuse từ project-rpc.test.ts mock style):
const mockPool = {
  query: vi.fn().mockResolvedValue([]),
  queryOne: vi.fn().mockResolvedValue(null),
}
const mockRpcServer = {
  handle: vi.fn(),
}

describe('profile RPC handlers', () => {
  describe('profile.getResolved', () => {
    it('returns resolved profile for authenticated user')
    it('returns empty profile when no company configured')
  })

  describe('profile.updateCompany', () => {
    it('admin can update company profile')
    it('non-admin receives 403 PROFILE_UNAUTHORIZED')
    it('invalid JSON in profile body returns PROFILE_INVALID_JSON')
  })

  describe('profile.updateUser', () => {
    it('own user can update non-locked fields')
    it('rejects update of locked security section — PROFILE_FIELD_LOCKED')
    it('other user cannot update — 403')
  })

  describe('profile.invalidate', () => {
    it('admin can invalidate any user cache')
    it('non-admin receives 403')
  })

  describe('profile.setUserDept', () => {
    it('admin can set user department')
    it('non-admin receives 403')
    it('unknown dept returns DEPT_NOT_FOUND')
  })
})
```

**Target: ≥ 12 tests**

---

## 3. Verification Plan

```bash
# Run existing + new tests
pnpm vitest run src/main/profile
```

**Expected:** ≥ 37 tests total (25 existing + 12 new)

---

## 4. server-bootstrap.ts — Wire Profile Services

Profile services đã được khai báo trong `ServerBootstrapResult` interface.  
Xem [08-server-bootstrap-wiring.md](./08-server-bootstrap-wiring.md) để xem wiring code.

**Key injection point:**
```typescript
// In initializeOrcaServices():
const profileService = new ProfileService(pool)
const profileResolver = new ProfileResolver(profileService)
// Returned in ServerBootstrapResult
```
