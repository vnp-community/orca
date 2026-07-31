# TASK-V5-06: AI Provider Slice + credential-crypto + useAIProviders

**Order:** 6  
**Prerequisite:** TASK-V5-01 (shared types)  
**Solution Ref:** SOL-FE-V5-03 (section 3, 4, 5)  
**Est. effort:** ~60 min | **Tests:** 5

---

## Mô tả

Tạo `AIProviderSlice`, tiện ích encrypt credential `credential-crypto.ts`, và `useAIProviders` hook.

---

## Files Cần Tạo

### 1. `src/renderer/src/store/slices/ai-provider.ts`

```typescript
import type { AIProviderAccount, AIProviderUsage, AIProviderStatus } from '@shared/ai-provider-types'
import type { StateCreator } from 'zustand'

export type AIProviderSliceState = {
  accounts:          AIProviderAccount[]
  usageByAccount:    Record<string, AIProviderUsage>
  isLoadingAccounts: boolean
}

export type AIProviderSliceActions = {
  setAccounts(accounts: AIProviderAccount[]): void
  addAccount(account: AIProviderAccount): void
  removeAccount(id: string): void
  updateAccountStatus(id: string, status: AIProviderStatus): void
  updateAccount(id: string, patch: Partial<AIProviderAccount>): void
  setUsage(accountId: string, usage: AIProviderUsage): void
  setLoadingAccounts(v: boolean): void
}

export type AIProviderSlice = AIProviderSliceState & AIProviderSliceActions

export function createAIProviderSlice(
  set: StateCreator<AIProviderSlice>['arguments'][0]
): AIProviderSlice {
  return {
    accounts:          [],
    usageByAccount:    {},
    isLoadingAccounts: false,

    setAccounts:    (accounts) => set(s => { s.accounts = accounts }),
    addAccount:     (account)  => set(s => { s.accounts.push(account) }),
    removeAccount:  (id)       => set(s => { s.accounts = s.accounts.filter(a => a.id !== id) }),
    updateAccountStatus: (id, status) => set(s => {
      const idx = s.accounts.findIndex(a => a.id === id)
      if (idx !== -1) s.accounts[idx].status = status
    }),
    updateAccount: (id, patch) => set(s => {
      const idx = s.accounts.findIndex(a => a.id === id)
      if (idx !== -1) Object.assign(s.accounts[idx], patch)
    }),
    setUsage: (accountId, usage) => set(s => { s.usageByAccount[accountId] = usage }),
    setLoadingAccounts: (v)      => set(s => { s.isLoadingAccounts = v }),
  }
}
```

### 2. `src/renderer/src/lib/credential-crypto.ts`

```typescript
/**
 * credential-crypto.ts
 *
 * SECURITY:
 * - Plaintext API keys NEVER stored in state after encryption
 * - Session-derived AES-GCM key (PBKDF2, 100k iterations)
 * - Caller must clear rawValue after calling encryptCredential()
 */

const PBKDF2_ITERATIONS = 100_000
const SALT = new TextEncoder().encode('orca-cred-v1')

async function deriveKey(sessionToken: string): Promise<CryptoKey> {
  const keyMaterial = await crypto.subtle.importKey(
    'raw',
    new TextEncoder().encode(sessionToken),
    { name: 'PBKDF2' },
    false,
    ['deriveKey']
  )
  return crypto.subtle.deriveKey(
    { name: 'PBKDF2', salt: SALT, iterations: PBKDF2_ITERATIONS, hash: 'SHA-256' },
    keyMaterial,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt']
  )
}

export interface EncryptedCredential {
  encryptedBlob: string   // base64
  iv:            string   // base64
}

/**
 * Encrypt plaintext credential using session-derived key.
 * Returns base64-encoded ciphertext + IV.
 * Caller MUST clear the plaintext string from state after calling this.
 */
export async function encryptCredential(
  plaintext: string,
  sessionToken: string
): Promise<EncryptedCredential> {
  const key = await deriveKey(sessionToken)
  const iv  = crypto.getRandomValues(new Uint8Array(16))

  const encrypted = await crypto.subtle.encrypt(
    { name: 'AES-GCM', iv },
    key,
    new TextEncoder().encode(plaintext)
  )

  return {
    encryptedBlob: btoa(String.fromCharCode(...new Uint8Array(encrypted))),
    iv:            btoa(String.fromCharCode(...iv)),
  }
}
```

### 3. `src/renderer/src/hooks/useAIProviders.ts`

```typescript
import { useCallback, useEffect } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'
import type { AIProviderAccount } from '@shared/ai-provider-types'

export function useAIProviders(devServerId?: string) {
  const { accounts, isLoadingAccounts } = useAppStore(s => ({
    accounts: devServerId
      ? s.accounts.filter((a: AIProviderAccount) => a.devServerId === devServerId)
      : s.accounts,
    isLoadingAccounts: s.isLoadingAccounts,
  }))

  const refresh = useCallback(async () => {
    const store = useAppStore.getState()
    store.setLoadingAccounts(true)
    try {
      const result = await callRuntimeRpc('aiProvider.list', { devServerId }) as AIProviderAccount[]
      store.setAccounts(result)
    } finally {
      store.setLoadingAccounts(false)
    }
  }, [devServerId])

  useEffect(() => { refresh() }, [refresh])

  const testConnection = useCallback(async (accountId: string) => {
    const result = await callRuntimeRpc('aiProvider.testConnection', { accountId }) as {
      ok: boolean; latencyMs: number; error?: string
    }
    useAppStore.getState().updateAccountStatus(accountId, result.ok ? 'active' : 'invalid')
    return result
  }, [])

  const deleteAccount = useCallback(async (accountId: string) => {
    await callRuntimeRpc('aiProvider.delete', { accountId })
    useAppStore.getState().removeAccount(accountId)
  }, [])

  return { accounts, isLoading: isLoadingAccounts, refresh, testConnection, deleteAccount }
}
```

---

## Files Cần Sửa

### `src/renderer/src/store/index.ts`

```typescript
import { createAIProviderSlice } from './slices/ai-provider'
// Trong combined slice:
...createAIProviderSlice(...a),
```

---

## Tests — `src/renderer/src/hooks/__tests__/useAIProviders.test.ts`

```typescript
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
}))

const mockAccounts: any[] = []
const mockStore = {
  accounts: mockAccounts,
  isLoadingAccounts: false,
  setAccounts:       vi.fn(a => { mockStore.accounts = a }),
  updateAccountStatus: vi.fn(),
  removeAccount:     vi.fn(),
  setLoadingAccounts: vi.fn(v => { mockStore.isLoadingAccounts = v }),
}
vi.mock('../../store', () => ({
  useAppStore: (fn?: any) => fn ? fn(mockStore) : mockStore,
}))

import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
const mockRpc = vi.mocked(callRuntimeRpc)

describe('useAIProviders', () => {
  beforeEach(() => vi.clearAllMocks())

  it('fetches accounts on mount', async () => {
    mockRpc.mockResolvedValueOnce([{ id: 'acc1', provider: 'anthropic' }])
    const { useAIProviders } = await import('../useAIProviders')
    renderHook(() => useAIProviders())
    await act(async () => {})
    expect(mockRpc).toHaveBeenCalledWith('aiProvider.list', { devServerId: undefined })
    expect(mockStore.setAccounts).toHaveBeenCalledWith([{ id: 'acc1', provider: 'anthropic' }])
  })

  it('devServerId filter applied to fetch', async () => {
    mockRpc.mockResolvedValueOnce([])
    const { useAIProviders } = await import('../useAIProviders')
    renderHook(() => useAIProviders('srv1'))
    await act(async () => {})
    expect(mockRpc).toHaveBeenCalledWith('aiProvider.list', { devServerId: 'srv1' })
  })

  it('testConnection ok → status active', async () => {
    mockRpc.mockResolvedValueOnce([])    // mount fetch
    mockRpc.mockResolvedValueOnce({ ok: true, latencyMs: 120 })
    const { useAIProviders } = await import('../useAIProviders')
    const { result } = renderHook(() => useAIProviders())
    await act(async () => { await result.current.testConnection('acc1') })
    expect(mockStore.updateAccountStatus).toHaveBeenCalledWith('acc1', 'active')
  })

  it('testConnection fail → status invalid', async () => {
    mockRpc.mockResolvedValueOnce([])
    mockRpc.mockResolvedValueOnce({ ok: false, latencyMs: 0, error: 'Invalid key' })
    const { useAIProviders } = await import('../useAIProviders')
    const { result } = renderHook(() => useAIProviders())
    await act(async () => { await result.current.testConnection('acc1') })
    expect(mockStore.updateAccountStatus).toHaveBeenCalledWith('acc1', 'invalid')
  })

  it('refresh re-fetches accounts', async () => {
    mockRpc.mockResolvedValue([])
    const { useAIProviders } = await import('../useAIProviders')
    const { result } = renderHook(() => useAIProviders())
    await act(async () => { await result.current.refresh() })
    expect(mockRpc).toHaveBeenCalledTimes(2)  // mount + manual refresh
  })
})
```

---

## Acceptance Criteria

- [x] `AIProviderSlice` registered trong store (via `ai-provider-slice.ts`)
- [x] `credential-crypto.ts` — `encryptCredential()` returns `{ encryptedBlob, iv }` (base64)
- [x] `encryptCredential()` sử dụng `crypto.subtle` (không bao giờ log plaintext)
- [x] `useAIProviders()` fetch khi mount, filter theo `devServerId`
- [x] `testConnection()` → updateAccountStatus 'active' hoặc 'invalid'
- [x] `setLoadingAccounts(true)` trước fetch, `false` sau
- [x] 5/5 tests pass
