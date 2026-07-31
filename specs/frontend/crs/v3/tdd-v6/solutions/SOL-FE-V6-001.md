# SOL-FE-V6-001: Profile Hierarchy UI (TDD-FE-11)

**Solution ID:** SOL-FE-V6-001
**TDD Ref:** [TDD-FE-11](../../../../tdd/v5/11-profile-ui.md)
**Feature:** F33 | **ADR:** ADR-007 | **HLD Ref:** C3.10, C4.7
**Date:** 2026-07-30
**Status:** ✅ COMPLETED — 2026-07-30

---

## 1. Phan tich code hien co

### 1.1 Da ton tai (KHONG viet lai)

| File | Size | Nhan xet |
|------|------|---------|
| `components/profile/ProfileEditor.tsx` | 2982 bytes | Co san — can kiem tra logic |
| `components/profile/ProfileSourceBadge.tsx` | 1494 bytes | Co san — day du theo TDD |
| `components/profile/ProfileFieldRow.tsx` | 747 bytes | Co san — day du |
| `components/profile/ModelSelector.tsx` | 1741 bytes | Co san — can verify approved models filter |
| `components/profile/CompanyProfileAdmin.tsx` | 1927 bytes | Co san — can verify dept selector |
| `hooks/useProfile.ts` | 2646 bytes | Co san — can verify saveProfile scope handling |
| `store/slices/profile-slice.ts` | 2047 bytes | Co san — day du slices |

### 1.2 Chua ton tai (CAN TAO MOI)

| File | TDD Ref | Do uu tien |
|------|---------|-----------|
| `components/profile/DeptProfileAdmin.tsx` | Section 5 | MEDIUM |
| `components/profile/__tests__/ProfileEditor.test.tsx` | Section 7 | HIGH |
| `components/profile/__tests__/ProfileSourceBadge.test.tsx` | Section 7 | HIGH |
| `components/profile/__tests__/ModelSelector.test.tsx` | Section 7 | HIGH |
| `hooks/__tests__/useProfile.test.ts` | Section 7 | HIGH |

---

## 2. Giai phap — Nhung gi can bo sung

### 2.1 DeptProfileAdmin.tsx — File moi

**Ly do can tao moi:** File nay chua co trong codebase, can implement cho Admin SPA `/admin/profile` tab departments.

**Tai su dung:**
- Import `ProfileEditor` tu `./ProfileEditor` (da co)
- Dung `shadcn/ui` components: `Tabs`, `Button`, `Badge`
- Call RPC: `profile.listDepts`, `profile.createDept` — qua `callRuntimeRpc`

```typescript
// src/renderer/src/components/profile/DeptProfileAdmin.tsx [NEW]
import { useState, useEffect } from 'react'
import { ProfileEditor } from './ProfileEditor'
import { callRuntimeRpc, getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'
import { useAppStore } from '@/store'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import type { Department } from '@/types/profile-types'

export function DeptProfileAdmin() {
  const [depts, setDepts] = useState<Department[]>([])
  const [activeDeptId, setActiveDeptId] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<Department[]>(target, 'profile.listDepts', {})
      .then(d => setDepts(d))
      .catch(() => setDepts([]))
      .finally(() => setIsLoading(false))
  }, [])

  if (isLoading) return <div className="p-4 text-sm text-muted-foreground">Loading departments...</div>

  return (
    <div className="dept-profile-admin space-y-4 p-4">
      {/* Department selector */}
      <div className="flex flex-wrap gap-2">
        {depts.map(dept => (
          <Badge
            key={dept.id}
            variant={activeDeptId === dept.id ? 'default' : 'outline'}
            className="cursor-pointer"
            onClick={() => setActiveDeptId(dept.id)}
          >
            {dept.name}
          </Badge>
        ))}
      </div>

      {/* Department profile editor */}
      {activeDeptId ? (
        <ProfileEditor scope="dept" scopeId={activeDeptId} />
      ) : (
        <p className="text-sm text-muted-foreground">
          Select a department above to edit its profile settings.
        </p>
      )}
    </div>
  )
}
```

### 2.2 ProfileEditor.tsx — Kiem tra va bo sung

**Gap can kiem tra:**

1. **Security section locking** — Can dam bao `scope !== 'company'` thi render `readOnly={true}` cho security fields
2. **Tabs "Effective Settings"** — Chi hien thi khi `scope === 'user'`
3. **ModelSelector integration** — Phai dung `resolvedProfile?.security?.approvedModels` de filter

**Neu ProfileEditor.tsx chua day du, bo sung:**

```typescript
// Kiem tra ProfileEditor — Phan security section
<ProfileSection title="Security" locked={scope !== 'company'}>
  {/* Locked khi khong phai company scope */}
  {scope !== 'company' && (
    <p className="text-xs text-muted-foreground flex items-center gap-1">
      <Lock size={10} /> Only Company admins can modify security settings
    </p>
  )}
  <SecurityProfileForm
    profile={displayProfile?.security}
    readOnly={scope !== 'company' || readOnly}
  />
</ProfileSection>
```

### 2.3 useProfile.ts — Kiem tra saveProfile

**Gap can kiem tra — RPC method mapping:**

```typescript
// Phan can verify trong useProfile.ts:
export function useProfileActions() {
  const saveProfile = useCallback(async (scope: string, profile: OrcaProfile) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    
    if (scope === 'user') {
      await callRuntimeRpc(target, 'profile.updateUser', { profile })
      // Re-fetch resolved (cache invalidated server-side)
      const resolved = await callRuntimeRpc<ResolvedProfile>(target, 'profile.getResolved', {})
      useAppStore.getState().setResolved(resolved)
      toast.success('Profile saved')
    } else if (scope === 'company') {
      await callRuntimeRpc(target, 'profile.updateCompany', { profile })
      toast.success('Company profile updated')
    } else if (scope === 'dept') {
      // Gap: scopeId phai duoc truyen vao
      await callRuntimeRpc(target, 'profile.updateDept', { profile })
      toast.success('Department profile updated')
    }
  }, [])

  return { saveProfile }
}
```

---

## 3. Integration Points

### 3.1 Admin SPA Route Addition

**File can sua:** `src/renderer/src/components/admin/AdminApp.tsx`

```typescript
// Them route /admin/profile vao AdminApp.tsx (additive)
import { CompanyProfileAdmin } from '../profile/CompanyProfileAdmin'

// Trong Router:
<Route path="/profile" element={<CompanyProfileAdmin />} />
```

**Admin sidebar navigation:** Them "Profile Settings" link vao admin nav.

### 3.2 User Settings Integration

**File can xem xet:** `src/renderer/src/components/settings/`

Neu co tab "General" trong Settings, co the inject `ProfileEditor scope="user"` vao do thay vi tao trang rieng.

---

## 4. Type Verification

**File can kiem tra:** `src/renderer/src/types/profile-types.ts`

Dam bao co day du cac types sau:

```typescript
// Can co trong profile-types.ts:
interface OrcaProfile {
  agent?: { preferredModel?, trustPreset?, customInstructions?, approvedModels? }
  editor?: { theme?, fontSize?, fontFamily?, keybindings? }
  shell?: { defaultShell?, pathAdditions?, envVars?, startupCommands? }
  integrations?: { githubOrg?, linearWorkspace?, prTemplate? }
  fleet?: { allowedServerTags?, defaultConnectionType? }
  security?: { require2FA?, sessionTimeoutHours?, allowedIpRanges? }
}

interface ResolvedProfile extends OrcaProfile {
  _sources: Record<string, 'company' | 'dept' | 'user' | 'concat'>
}

interface Department {
  id: string
  name: string
  leadId?: string
  memberCount?: number
}
```

---

## 5. Test Plan

**Target:** >= 25 tests

### Test Files Can Tao

```
src/renderer/src/components/profile/__tests__/
├── ProfileEditor.test.tsx           (5+ tests)
│   ├── renders 'My Settings' tab for user scope
│   ├── renders 'Effective Settings' tab when scope=user
│   ├── security section locked when scope != 'company'
│   ├── locked field readOnly=true
│   └── saveProfile called on Save Changes click
├── ProfileSourceBadge.test.tsx      (4+ tests)
│   ├── company => purple badge
│   ├── dept => blue badge  
│   ├── locked=true => Lock icon + 'Company Only'
│   └── concat => grey badge
├── ModelSelector.test.tsx           (3+ tests)
│   ├── shows all models when approvedModels is empty
│   ├── filters to approvedModels when set (wildcard matching)
│   └── calls onChange with selected model id
└── DeptProfileAdmin.test.tsx        (4+ tests)
    ├── fetches departments on mount
    ├── selecting dept shows ProfileEditor scope="dept"
    ├── no dept selected => shows guidance text
    └── isLoading shows loading state

src/renderer/src/hooks/__tests__/
└── useProfile.test.ts               (6+ tests)
    ├── fetches resolvedProfile on mount
    ├── saveProfile scope='user' => calls profile.updateUser
    ├── saveProfile scope='user' => re-fetches resolved after save
    ├── saveProfile scope='company' => calls profile.updateCompany
    ├── saveProfile scope='dept' => calls profile.updateDept
    └── toast.success shown on save
```

---

## 6. Phu thuoc va Thu tu

**Prerequisite:** Khong co (doc lap hoat dong)

**Cac file phu thuoc vao SOL-FE-V6-001:**
- Admin SPA `AdminApp.tsx` — them route `/admin/profile`
- `SOL-FE-V6-002` (WorkspaceLayout) — co the inject ProfileEditor vao sidebar

**Dependencies:** Khong can install them
