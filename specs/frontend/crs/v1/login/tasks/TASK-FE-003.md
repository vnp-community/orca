# TASK-FE-003 — Tạo `store/slices/auth.ts` (Zustand AuthSlice)

**Phase:** 1 — Auth Foundation
**Solution:** [SOL-FE-LG-001](../solutions/SOL-FE-LG-001-login-page.md) §4.3
**Depends on:** TASK-FE-002
**Blocks:** TASK-FE-006, TASK-FE-008, TASK-FE-009, TASK-FE-022
**Effort:** S (~20 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo Zustand slice quản lý auth state. Slice này được compose vào `useAppStore` tại TASK-FE-008.

---

## File cần tạo

### `src/renderer/src/store/slices/auth.ts` [NEW]

```typescript
import { StateCreator } from 'zustand'
import { AuthState } from '../../auth/auth-types'
import { fetchCurrentUser } from '../../auth/auth-api-client'

export type AuthSlice = {
  auth: AuthState
  setAuth: (state: AuthState) => void
  clearAuth: () => void
  checkSession: () => Promise<void>
}

export const createAuthSlice: StateCreator<AuthSlice> = (set) => ({
  auth: { status: 'unknown' },

  setAuth: (state) => set({ auth: state }),

  clearAuth: () => set({ auth: { status: 'unauthenticated' } }),

  checkSession: async () => {
    try {
      const user = await fetchCurrentUser()
      if (user) {
        set({ auth: { status: 'authenticated', user } })
      } else {
        set({ auth: { status: 'unauthenticated' } })
      }
    } catch (err) {
      set({ auth: { status: 'error', message: (err as Error).message } })
    }
  },
})
```

### `src/renderer/src/store/slices/__tests__/auth.test.ts` [NEW]

Test cases (4 tests):
- `checkSession()` → user returned → status = 'authenticated'
- `checkSession()` → null returned → status = 'unauthenticated'
- `checkSession()` → throws → status = 'error'
- `clearAuth()` → status = 'unauthenticated'

```typescript
// @vitest-environment happy-dom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createAuthSlice, AuthSlice } from '../auth'
import * as apiClient from '../../../auth/auth-api-client'

vi.mock('../../../auth/auth-api-client')

function makeSlice(): AuthSlice {
  let state: AuthSlice
  const set = (partial: Partial<AuthSlice> | ((s: AuthSlice) => Partial<AuthSlice>)) => {
    const update = typeof partial === 'function' ? partial(state) : partial
    state = { ...state, ...update }
  }
  state = createAuthSlice(set as any, () => state as any, {} as any)
  return state
}

describe('AuthSlice', () => {
  it('checkSession sets authenticated when user returned', async () => {
    const mockUser = { id: 'u1', email: 'a@b.com', name: 'A', role: 'developer' as const, provider: 'local' as const }
    vi.mocked(apiClient.fetchCurrentUser).mockResolvedValueOnce(mockUser)
    const slice = makeSlice()
    await slice.checkSession()
    expect(slice.auth.status).toBe('authenticated')
  })

  it('checkSession sets unauthenticated when null returned', async () => {
    vi.mocked(apiClient.fetchCurrentUser).mockResolvedValueOnce(null)
    const slice = makeSlice()
    await slice.checkSession()
    expect(slice.auth.status).toBe('unauthenticated')
  })

  it('checkSession sets error on throw', async () => {
    vi.mocked(apiClient.fetchCurrentUser).mockRejectedValueOnce(new Error('Network'))
    const slice = makeSlice()
    await slice.checkSession()
    expect(slice.auth.status).toBe('error')
  })

  it('clearAuth sets unauthenticated', () => {
    const slice = makeSlice()
    slice.clearAuth()
    expect(slice.auth.status).toBe('unauthenticated')
  })
})
```

---

## Verify

```bash
npx vitest run src/renderer/src/store/slices/__tests__/auth.test.ts
# Expected: 4 pass
```
