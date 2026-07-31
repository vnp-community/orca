# SOL-FE-V5-01: Profile Hierarchy UI

**TDD Ref:** [TDD-FE-11](../../../tdd/11-profile-ui.md)  
**Feature:** F33 | **ADR:** ADR-007 | **HLD:** C3.10, C4.7  
**Status:** ✅ DONE — Implemented via TASK-V5-04, TASK-V5-05  
**Additive-only:** ✅ Không sửa App.tsx, main.tsx

---

## 1. Files Cần Tạo

| File | Loại | Mô tả |
|------|------|-------|
| `src/renderer/src/store/slices/profile.ts` | Zustand Slice | Quản lý profile state |
| `src/renderer/src/components/profile/ProfileEditor.tsx` | Component | Editor 3-tab với source badges |
| `src/renderer/src/components/profile/ProfileSourceBadge.tsx` | Component | Badge chỉ nguồn gốc field |
| `src/renderer/src/components/profile/ProfileFieldRow.tsx` | Component | Wrapper field + badge |
| `src/renderer/src/components/profile/ProfileSection.tsx` | Component | Collapsible section |
| `src/renderer/src/components/profile/CompanyProfileAdmin.tsx` | Component | Admin UI cho Company/Dept |
| `src/renderer/src/components/profile/DeptProfileAdmin.tsx` | Component | Dept-level profile editor |
| `src/renderer/src/components/profile/ModelSelector.tsx` | Component | AI model picker |
| `src/renderer/src/components/profile/McpServerList.tsx` | Component | MCP server list + add |
| `src/renderer/src/components/profile/EnvVarsEditor.tsx` | Component | KEY=VALUE editor |
| `src/renderer/src/hooks/useProfile.ts` | Hook | Fetch + save profile actions |

---

## 2. Files Cần Sửa (Additive)

| File | Thay đổi |
|------|---------|
| `src/renderer/src/store/index.ts` | Register `createProfileSlice` |
| `src/renderer/src/components/admin/AdminApp.tsx` | Thêm route `/admin/profile` → `CompanyProfileAdmin` |
| `src/renderer/src/components/admin/AdminDashboard.tsx` | Thêm link tới `/admin/profile` |

---

## 3. Giải pháp Chi tiết

### 3.1 Profile Slice

```typescript
// src/renderer/src/store/slices/profile.ts

import type { OrcaProfile, ResolvedProfile, Department } from '@shared/profile-types'

export type ProfileSliceState = {
  userProfile:      OrcaProfile | null
  resolvedProfile:  ResolvedProfile | null  // includes _sources map
  companyProfile:   OrcaProfile | null
  depts:            Department[]
  profileIsLoading: boolean
}

export type ProfileSliceActions = {
  setUserProfile(p: OrcaProfile): void
  setResolved(p: ResolvedProfile): void
  setCompanyProfile(p: OrcaProfile): void
  setDepts(depts: Department[]): void
  setProfileLoading(v: boolean): void
}

export type ProfileSlice = ProfileSliceState & ProfileSliceActions

export function createProfileSlice(set): ProfileSlice {
  return {
    userProfile:      null,
    resolvedProfile:  null,
    companyProfile:   null,
    depts:            [],
    profileIsLoading: false,

    setUserProfile:    (p)     => set(s => { s.userProfile = p }),
    setResolved:       (p)     => set(s => { s.resolvedProfile = p }),
    setCompanyProfile: (p)     => set(s => { s.companyProfile = p }),
    setDepts:          (depts) => set(s => { s.depts = depts }),
    setProfileLoading: (v)     => set(s => { s.profileIsLoading = v }),
  }
}
```

### 3.2 ResolvedProfile (Shared Types)

```typescript
// src/shared/profile-types.ts (thêm vào shared)

export type ProfileSource = 'company' | 'dept' | 'user' | 'concat'

export type ResolvedProfile = OrcaProfile & {
  _sources: Record<string, ProfileSource>  // field path → source
}

export type OrcaProfile = {
  agent?: {
    preferredModel?:       string
    trustPreset?:          'strict' | 'standard' | 'relaxed' | 'custom'
    customInstructions?:   string
  }
  mcp?: {
    servers?: McpServerConfig[]
  }
  shell?: {
    defaultShell?:    string
    pathAdditions?:   string[]
    envVars?:         Record<string, string>
  }
  security?: {
    approvedModels?:    string[]    // glob patterns e.g. 'claude-*'
    disallowedCmds?:   string[]
  }
}

export type Department = {
  id:       string
  name:     string
  parentId: string | null  // company id
}
```

### 3.3 useProfile Hook

```typescript
// src/renderer/src/hooks/useProfile.ts

import { useCallback, useEffect } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc as rpc } from '../runtime/runtime-rpc-client'
import type { OrcaProfile, ResolvedProfile } from '@shared/profile-types'
import toast from 'react-hot-toast'

export function useProfile() {
  const { resolvedProfile, userProfile, profileIsLoading } = useAppStore(s => ({
    resolvedProfile:  s.resolvedProfile,
    userProfile:      s.userProfile,
    profileIsLoading: s.profileIsLoading,
  }))

  // Fetch on mount
  useEffect(() => {
    const store = useAppStore.getState()
    store.setProfileLoading(true)
    Promise.all([
      rpc('profile.getResolved', {}),
      rpc('profile.getUser', {}),
    ]).then(([resolved, user]) => {
      store.setResolved(resolved as ResolvedProfile)
      store.setUserProfile(user as OrcaProfile)
    }).finally(() => {
      store.setProfileLoading(false)
    })
  }, [])

  return { resolvedProfile, userProfile, profileIsLoading }
}

export function useProfileActions() {
  const saveProfile = useCallback(async (
    scope: 'user' | 'dept' | 'company',
    profile: OrcaProfile,
    scopeId?: string
  ) => {
    try {
      if (scope === 'user') {
        await rpc('profile.updateUser', { profile })
        const resolved = await rpc('profile.getResolved', {})
        useAppStore.getState().setResolved(resolved as ResolvedProfile)
        useAppStore.getState().setUserProfile(profile)
        toast.success('Profile saved')
      } else if (scope === 'company') {
        await rpc('profile.updateCompany', { profile })
        toast.success('Company profile updated')
      } else if (scope === 'dept' && scopeId) {
        await rpc('profile.updateDept', { deptId: scopeId, profile })
        toast.success('Department profile updated')
      }
    } catch (err: any) {
      toast.error(err.message ?? 'Failed to save profile')
    }
  }, [])

  return { saveProfile }
}
```

### 3.4 ProfileEditor Component

```typescript
// src/renderer/src/components/profile/ProfileEditor.tsx
// Xem chi tiết trong TDD-FE-11 section 2

// Điểm khác biệt so với TDD:
// - Dùng callRuntimeRpc thay vì rpc.call (nhất quán với codebase)
// - Toast qua react-hot-toast (đã có trong codebase)
// - Tabs dùng shadcn/ui Tabs (đã có)
```

### 3.5 Mount vào Admin SPA

```typescript
// src/renderer/src/components/admin/AdminApp.tsx — MODIFY (additive)
// Thêm route:
import { CompanyProfileAdmin } from '../profile/CompanyProfileAdmin'

<Route path="/profile" element={<CompanyProfileAdmin />} />

// src/renderer/src/components/admin/AdminDashboard.tsx — MODIFY
// Thêm nav link: "Company Profile" → /admin/profile
```

---

## 4. RPC Methods Cần Backend Expose

| Method | Params | Return | Backend TDD |
|--------|--------|--------|-------------|
| `profile.getResolved` | `{}` | `ResolvedProfile` | TDD-BE-14 |
| `profile.getUser` | `{}` | `OrcaProfile` | TDD-BE-14 |
| `profile.updateUser` | `{ profile }` | `void` | TDD-BE-14 |
| `profile.getCompany` | `{}` | `OrcaProfile` | TDD-BE-14 |
| `profile.updateCompany` | `{ profile }` | `void` | TDD-BE-14 |
| `profile.listDepts` | `{}` | `Department[]` | TDD-BE-14 |
| `profile.getDept` | `{ deptId }` | `OrcaProfile` | TDD-BE-14 |
| `profile.updateDept` | `{ deptId, profile }` | `void` | TDD-BE-14 |

---

## 5. Test Plan

```
src/renderer/src/components/profile/__tests__/
├── ProfileEditor.test.tsx         (10 tests)
│   ├── renders 'My Settings' tab for user scope
│   ├── renders 'Effective Settings' tab
│   ├── security section locked when scope !== 'company'
│   ├── calls saveProfile on 'Save Changes' click
│   ├── shows loading state while fetching
│   ├── ModelSelector shows filtered models when approvedModels set
│   ├── ProfileSourceBadge shows 'Company' for company-sourced field
│   ├── company scope: security section is editable
│   ├── dept scope: no 'Effective Settings' tab
│   └── EnvVarsEditor: add/remove works
├── ProfileSourceBadge.test.tsx    (4 tests)
│   ├── company → purple badge + building icon
│   ├── dept → blue badge
│   ├── user → green badge
│   └── locked=true → 🔒 'Company Only'
├── ModelSelector.test.tsx         (4 tests)
│   ├── shows all models when approvedModels empty
│   ├── filters to matching models when approvedModels set
│   ├── calls onChange with selected model id
│   └── disabled prop disables select
└── hooks/__tests__/useProfile.test.ts  (7 tests)
    ├── fetches resolvedProfile on mount
    ├── fetches userProfile on mount
    ├── setProfileLoading true during fetch
    ├── saveProfile(user) calls profile.updateUser
    ├── saveProfile(user) refetches resolvedProfile
    ├── saveProfile(company) calls profile.updateCompany
    └── saveProfile(dept, ..., deptId) calls profile.updateDept
```

**Target:** ≥ 25 tests

---

## 6. Constraints & Risks

| Concern | Giải pháp |
|---------|---------|
| `_sources` key conflicts với Zustand immer | Dùng `Object.keys()` thay vì spread |
| Circular dept hierarchy | Server validate, FE chỉ display |
| Security fields editable khi scope=company | Guard: `readOnly={scope !== 'company'}` |
| Concurrent profile saves | Debounce + optimistic update |
