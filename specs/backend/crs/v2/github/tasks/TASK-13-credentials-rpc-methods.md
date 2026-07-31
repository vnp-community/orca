# TASK-13: RPC Methods — `credentials.*` (Set, Revoke, Status)

**Status:** ✅ DONE — 2026-07-25 (AC verified 2026-07-25)  
**Phase:** 5 — Frontend (Backend RPC for credential management UI)  
**Priority:** 🟡 Medium  
**Depends on:** TASK-02 (WebCredentialStore)  
**Solution:** SOL-04-Credential-Store.md, CR-INT-004  
**Estimated effort:** ~45 phút

---

## Mục tiêu

Tạo RPC methods cho phép Browser UI quản lý tokens cho tất cả integrations (Bitbucket, AzDO, Gitea, Linear, Jira) thông qua Settings page.

---

## File cần tạo: `src/main/runtime/rpc/methods/credentials.ts` [NEW]

```typescript
import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import { isWebCredentialMode, getWebCredentialStore } from '../../credentials'
import type { CredentialService } from '../../credentials/web-credential-store'

const ServiceEnum = z.enum(['bitbucket', 'azure-devops', 'gitea', 'linear', 'jira'])

const SetTokenParams = z.object({
  service: ServiceEnum,
  token: z.string().min(1, 'Token cannot be empty'),
  config: z.record(z.string()).optional()  // baseUrl, email, etc.
})

const ServiceParams = z.object({
  service: ServiceEnum
})

export const CREDENTIAL_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'credentials.set',
    params: SetTokenParams,
    handler: async (params, _ctx) => {
      if (!isWebCredentialMode()) {
        throw new Error('credentials.set is only available in Web Server mode')
      }
      const store = getWebCredentialStore()
      await store.setToken(params.service as CredentialService, params.token, params.config)
      return { success: true }
    }
  }),

  defineMethod({
    name: 'credentials.revoke',
    params: ServiceParams,
    handler: async (params, _ctx) => {
      if (!isWebCredentialMode()) {
        throw new Error('credentials.revoke is only available in Web Server mode')
      }
      const store = getWebCredentialStore()
      await store.deleteToken(params.service as CredentialService)
      return { success: true }
    }
  }),

  defineMethod({
    name: 'credentials.status',
    params: ServiceParams,
    handler: async (params, _ctx) => {
      if (!isWebCredentialMode()) {
        // In Electron mode: report not configured via web store
        return { configured: false, mode: 'electron' }
      }
      const store = getWebCredentialStore()
      const hasToken = await store.hasToken(params.service as CredentialService)
      const config = hasToken ? await store.getConfig(params.service as CredentialService) : null
      return {
        configured: hasToken,
        mode: 'web',
        // Chỉ expose safe fields từ config (không expose token!)
        config: sanitizeConfigForDisplay(params.service as CredentialService, config)
      }
    }
  }),

  defineMethod({
    name: 'credentials.list',
    params: null,
    handler: async (_params, _ctx) => {
      if (!isWebCredentialMode()) {
        return { services: [], mode: 'electron' }
      }
      const store = getWebCredentialStore()
      const services = await store.listServices()
      return { services, mode: 'web' }
    }
  })
]

// Chỉ trả về các config field an toàn để hiển thị (không bao giờ return token)
function sanitizeConfigForDisplay(
  service: CredentialService,
  config: Record<string, string> | null
): Record<string, string> | null {
  if (!config) return null
  const SAFE_FIELDS: Record<CredentialService, string[]> = {
    'bitbucket': ['email', 'apiBaseUrl'],
    'azure-devops': ['apiBaseUrl', 'username'],
    'gitea': ['apiBaseUrl'],
    'linear': [],
    'jira': ['activeSiteId', 'siteUrl']
  }
  const safe = SAFE_FIELDS[service] ?? []
  return Object.fromEntries(
    Object.entries(config).filter(([key]) => safe.includes(key))
  )
}
```

---

## Cập nhật `src/main/runtime/rpc/methods/index.ts`

```typescript
import { CREDENTIAL_METHODS } from './credentials'

export const ALL_RPC_METHODS = [
  // ... existing methods ...
  ...CREDENTIAL_METHODS,  // THÊM
]
```

---

## Web Preload API — Expose tới Browser

**File:** `src/renderer/src/web/web-preload-api.ts`

Thêm credentials API để browser gọi được:

```typescript
// Trong api object:
credentials: {
  set: (service: string, token: string, config?: Record<string, string>) =>
    callRuntimeResult<{ success: boolean }>('credentials.set', { service, token, config }),
  revoke: (service: string) =>
    callRuntimeResult<{ success: boolean }>('credentials.revoke', { service }),
  status: (service: string) =>
    callRuntimeResult<{ configured: boolean; mode: string; config?: Record<string, string> }>(
      'credentials.status',
      { service }
    ),
  list: () =>
    callRuntimeResult<{ services: string[]; mode: string }>('credentials.list', null)
}
```

---

## Tests cần viết: `src/main/runtime/rpc/methods/credentials.test.ts` [NEW]

```typescript
describe('credentials.set', () => {
  it('stores token in WebCredentialStore when isWebMode')
  it('throws when not in web mode')
  it('stores config alongside token')
})

describe('credentials.revoke', () => {
  it('deletes token from WebCredentialStore')
  it('does not throw when token does not exist')
})

describe('credentials.status', () => {
  it('returns { configured: false } when no token')
  it('returns { configured: true } when token set')
  it('does NOT include token value in response')
  it('only includes safe config fields in response')
})
```

---

## Acceptance Criteria

1. ✅ `credentials.set('bitbucket', 'app-password')` → token được lưu trong `WebCredentialStore`
2. ✅ `credentials.status('bitbucket')` → `{ configured: true }` — không leak token
3. ✅ `credentials.revoke('bitbucket')` → token bị xóa
4. ✅ Trong Electron mode: `credentials.set()` throw error với message rõ ràng
5. ✅ Web preload API exposed: `credentials.set/revoke/status/list` có thể gọi từ browser
6. ✅ Tests pass: `credentials.test.ts` — 14/14 test cases

---

## Files cần tạo/sửa

- `src/main/runtime/rpc/methods/credentials.ts` [NEW]
- `src/main/runtime/rpc/methods/index.ts` [MODIFY]
- `src/renderer/src/web/web-preload-api.ts` [MODIFY] — expose credentials API
- `src/main/runtime/rpc/methods/credentials.test.ts` [NEW]
