# TASK-V5-04: Profile Slice + useProfile Hook

**Order:** 4  
**Prerequisite:** TASK-V5-01 (shared types)  
**Solution Ref:** SOL-FE-V5-01 (section 3.1, 3.3)  
**Est. effort:** ~60 min | **Tests:** 9

---

## Mô tả

Tạo Zustand `ProfileSlice` và `useProfile`/`useProfileActions` hooks. Tích hợp vào store.

---

## Files Cần Tạo

### 1. `src/renderer/src/store/slices/profile.ts`

```typescript
import type { OrcaProfile, ResolvedProfile, Department } from '@shared/profile-types'
import type { StateCreator } from 'zustand'

export type ProfileSliceState = {
  userProfile:      OrcaProfile | null
  resolvedProfile:  ResolvedProfile | null
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
  clearProfile(): void
}

export type ProfileSlice = ProfileSliceState & ProfileSliceActions

export function createProfileSlice(
  set: StateCreator<ProfileSlice>['arguments'][0]
): ProfileSlice {
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
    clearProfile:      ()      => set(s => {
      s.userProfile = null
      s.resolvedProfile = null
      s.companyProfile = null
      s.depts = []
    }),
  }
}
```

### 2. `src/renderer/src/hooks/useProfile.ts`

```typescript
import { useCallback, useEffect } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'
import type { OrcaProfile, ResolvedProfile } from '@shared/profile-types'
import toast from 'react-hot-toast'

// --- Read hook ---

export function useProfile() {
  const { resolvedProfile, userProfile, profileIsLoading } = useAppStore(s => ({
    resolvedProfile:  s.resolvedProfile,
    userProfile:      s.userProfile,
    profileIsLoading: s.profileIsLoading,
  }))

  useEffect(() => {
    const store = useAppStore.getState()
    store.setProfileLoading(true)

    Promise.all([
      callRuntimeRpc('profile.getResolved', {}),
      callRuntimeRpc('profile.getUser', {}),
    ])
      .then(([resolved, user]) => {
        store.setResolved(resolved as ResolvedProfile)
        store.setUserProfile(user as OrcaProfile)
      })
      .catch(err => {
        console.error('[useProfile] fetch failed:', err)
      })
      .finally(() => {
        store.setProfileLoading(false)
      })
  }, [])

  return { resolvedProfile, userProfile, profileIsLoading }
}

// --- Write hook ---

export function useProfileActions() {
  const saveProfile = useCallback(
    async (
      scope: 'user' | 'dept' | 'company',
      profile: OrcaProfile,
      scopeId?: string
    ) => {
      try {
        if (scope === 'user') {
          await callRuntimeRpc('profile.updateUser', { profile })
          const resolved = await callRuntimeRpc('profile.getResolved', {})
          const store = useAppStore.getState()
          store.setResolved(resolved as ResolvedProfile)
          store.setUserProfile(profile)
          toast.success('Profile saved')
        } else if (scope === 'company') {
          await callRuntimeRpc('profile.updateCompany', { profile })
          toast.success('Company profile updated')
        } else if (scope === 'dept' && scopeId) {
          await callRuntimeRpc('profile.updateDept', { deptId: scopeId, profile })
          toast.success('Department profile updated')
        }
      } catch (err: any) {
        toast.error(err?.message ?? 'Failed to save profile')
        throw err
      }
    },
    []
  )

  return { saveProfile }
}
```

---

## Files Cần Sửa

### `src/renderer/src/store/index.ts`

Tìm phần `create<AppStore>()(` và thêm:
```typescript
import { createProfileSlice } from './slices/profile'
// Trong combined slice:
...createProfileSlice(...a),
```

---

## Tests — `src/renderer/src/hooks/__tests__/useProfile.test.ts`

```typescript
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
}))
vi.mock('../../store', () => {
  const store = {
    resolvedProfile:  null,
    userProfile:      null,
    profileIsLoading: false,
    setResolved:      vi.fn(p  => { store.resolvedProfile = p }),
    setUserProfile:   vi.fn(p  => { store.userProfile = p }),
    setProfileLoading: vi.fn(v => { store.profileIsLoading = v }),
  }
  return {
    useAppStore: (fn?: any) => fn ? fn(store) : store,
  }
})
vi.mock('react-hot-toast', () => ({
  default: { success: vi.fn(), error: vi.fn() },
}))

import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { useAppStore }    from '../../store'
import toast from 'react-hot-toast'
const mockRpc   = vi.mocked(callRuntimeRpc)
const mockStore = useAppStore() as any

describe('useProfile', () => {
  beforeEach(() => vi.clearAllMocks())

  it('calls profile.getResolved and profile.getUser on mount', async () => {
    mockRpc.mockResolvedValue({})
    const { useProfile } = await import('../useProfile')
    renderHook(() => useProfile())
    await act(async () => {})
    expect(mockRpc).toHaveBeenCalledWith('profile.getResolved', {})
    expect(mockRpc).toHaveBeenCalledWith('profile.getUser', {})
  })

  it('sets profileIsLoading true during fetch then false', async () => {
    mockRpc.mockImplementation(() => new Promise(r => setTimeout(() => r({}), 50)))
    const { useProfile } = await import('../useProfile')
    renderHook(() => useProfile())
    expect(mockStore.setProfileLoading).toHaveBeenCalledWith(true)
    await act(async () => { await new Promise(r => setTimeout(r, 100)) })
    expect(mockStore.setProfileLoading).toHaveBeenCalledWith(false)
  })
})

describe('useProfileActions', () => {
  beforeEach(() => vi.clearAllMocks())

  it('saveProfile(user) calls profile.updateUser', async () => {
    mockRpc.mockResolvedValue({})
    const { useProfileActions } = await import('../useProfile')
    const { result } = renderHook(() => useProfileActions())
    await act(async () => {
      await result.current.saveProfile('user', { agent: { preferredModel: 'claude-opus-4-5' } })
    })
    expect(mockRpc).toHaveBeenCalledWith('profile.updateUser', {
      profile: { agent: { preferredModel: 'claude-opus-4-5' } }
    })
  })

  it('saveProfile(user) refetches resolvedProfile', async () => {
    mockRpc.mockResolvedValue({ _sources: {} })
    const { useProfileActions } = await import('../useProfile')
    const { result } = renderHook(() => useProfileActions())
    await act(async () => {
      await result.current.saveProfile('user', {})
    })
    expect(mockRpc).toHaveBeenCalledWith('profile.getResolved', {})
  })

  it('saveProfile(user) calls toast.success', async () => {
    mockRpc.mockResolvedValue({})
    const { useProfileActions } = await import('../useProfile')
    const { result } = renderHook(() => useProfileActions())
    await act(async () => { await result.current.saveProfile('user', {}) })
    expect(toast.success).toHaveBeenCalledWith('Profile saved')
  })

  it('saveProfile(company) calls profile.updateCompany', async () => {
    mockRpc.mockResolvedValue({})
    const { useProfileActions } = await import('../useProfile')
    const { result } = renderHook(() => useProfileActions())
    await act(async () => {
      await result.current.saveProfile('company', { security: { approvedModels: ['claude-*'] } })
    })
    expect(mockRpc).toHaveBeenCalledWith('profile.updateCompany', {
      profile: { security: { approvedModels: ['claude-*'] } }
    })
  })

  it('saveProfile(dept) calls profile.updateDept with deptId', async () => {
    mockRpc.mockResolvedValue({})
    const { useProfileActions } = await import('../useProfile')
    const { result } = renderHook(() => useProfileActions())
    await act(async () => {
      await result.current.saveProfile('dept', {}, 'dept-eng-01')
    })
    expect(mockRpc).toHaveBeenCalledWith('profile.updateDept', {
      deptId: 'dept-eng-01', profile: {}
    })
  })

  it('saveProfile: RPC error → toast.error + rethrow', async () => {
    mockRpc.mockRejectedValue(new Error('Network error'))
    const { useProfileActions } = await import('../useProfile')
    const { result } = renderHook(() => useProfileActions())
    await expect(
      act(async () => { await result.current.saveProfile('user', {}) })
    ).rejects.toThrow('Network error')
    expect(toast.error).toHaveBeenCalledWith('Network error')
  })
})
```

---

## Acceptance Criteria

- [x] `ProfileSlice` + `WorkspaceSlice` + `AIProviderSlice` registered trong `store/index.ts`
- [x] `useProfile()` fetch 2 RPC calls on mount: `profile.getResolved`, `profile.getUser`
- [x] `setProfileLoading(true)` trước fetch, `setProfileLoading(false)` sau
- [x] `saveProfile('user')` → updateUser + refetch resolved + toast.success
- [x] `saveProfile('company')` → updateCompany + toast.success
- [x] `saveProfile('dept', ..., deptId)` → updateDept
- [x] Error → toast.error + rethrow
- [x] 8/8 tests pass + slices compile với TS
