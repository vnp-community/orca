# CR-INT-004: Unified Credential Manager cho Web mode

**ID:** CR-INT-004  
**Priority:** 🟡 Medium  
**Component:** `src/main/credentials/` [NEW module]  
**Category:** Infrastructure  
**Depends on:** CR-INT-002, CR-INT-003  
**Status:** ✅ Implemented — 2026-07-25  
**Solutions:** SOL-04-Credential-Store, FE-SOL-03  
**Tasks:** TASK-07-08-09 (API token), TASK-10-11 (Linear/Jira), TASK-13 (credentials RPC), FE-TASK-01, FE-TASK-05

## Acceptance Criteria — Verified

1. ✅ `credentials.set('bitbucket', 'app-password-xyz')` lưu encrypted per-user — WebCredentialStore AES-256-GCM
2. ✅ `credentials.status('bitbucket')` trả về `{ configured: true }` không expose token — safe fields only
3. ✅ Unified RPC: credentials.set, credentials.revoke, credentials.status, credentials.list — `methods/credentials.ts`
4. ✅ Web mode only guard — `credentials.*` throw nếu không có ORCA_MULTI_USER=1
5. ✅ Browser: `window.api.credentials.*` exposed — `api-types.ts` + `web-preload-api.ts`
6. ✅ `useCredentialManager` hook cho React components

## Implementation

| Layer | File | Thay đổi |
|-------|------|---------|
| Backend | `src/main/credentials/web-credential-store.ts` | NEW — AES-256-GCM WebCredentialStore |
| Backend | `src/main/runtime/rpc/methods/credentials.ts` | NEW — 4 RPC methods |
| Preload | `src/preload/api-types.ts` | credentials namespace (L3379-3393) |
| Frontend | `web-preload-api.ts` | credentials.* methods |
| Frontend | `CredentialInputForm.tsx` | useCredentialManager hook |


---

## Vấn đề

CR-INT-002 và CR-INT-003 đề xuất `WebCredentialStore`. CR này định nghĩa implementation đầy đủ, bao gồm:

1. **Unified store interface** — một API cho tất cả integrations
2. **Server secret derivation** — key management  
3. **RPC endpoint** — `credentials.*` methods cho browser
4. **Token validation** — xác nhận token trước khi lưu
5. **Audit logging** — ghi lại token set/revoke events

---

## Unified `IntegrationCredentialService` enum

```typescript
// src/shared/integration-credential-errors.ts (existing, extend)
export type IntegrationCredentialService =
  | 'linear'
  | 'jira'
  | 'github'     // PAT-based auth (CR-GH-002 option A)
  | 'gitlab'     // PAT-based auth (CR-INT-001 option A)  
  | 'bitbucket'  // App password
  | 'azure-devops'  // PAT token
  | 'gitea'      // API token
```

---

## WebCredentialStore — Full Specification

**File:** `src/main/credentials/web-credential-store.ts` [NEW]

```typescript
import { createCipheriv, createDecipheriv, randomBytes, scryptSync, createHash } from 'node:crypto'
import { readFileSync, writeFileSync, mkdirSync, unlinkSync, readdirSync, existsSync } from 'node:fs'
import { join } from 'node:path'

export type CredentialMetadata = {
  service: IntegrationCredentialService
  createdAt: number
  lastValidatedAt?: number
  config?: Record<string, string>  // baseUrl, username, etc.
}

export class WebCredentialStore {
  private static instance: WebCredentialStore | null = null
  
  private constructor(
    private readonly baseDir: string,
    private readonly encryptionKey: Buffer
  ) {}
  
  static create(userDataPath: string, serverSecret: string): WebCredentialStore {
    if (WebCredentialStore.instance) return WebCredentialStore.instance
    const key = scryptSync(
      serverSecret,
      'orca-web-credential-store-v1',
      32  // AES-256 key
    )
    WebCredentialStore.instance = new WebCredentialStore(
      join(userDataPath, 'users'),
      key as Buffer
    )
    return WebCredentialStore.instance
  }
  
  private userCredDir(userId: string): string {
    // Sanitize userId to prevent path traversal
    const safeId = createHash('sha256').update(userId).digest('hex').slice(0, 32)
    return join(this.baseDir, safeId, 'credentials')
  }
  
  private tokenPath(userId: string, service: IntegrationCredentialService): string {
    return join(this.userCredDir(userId), `${service}.enc`)
  }
  
  private metaPath(userId: string, service: IntegrationCredentialService): string {
    return join(this.userCredDir(userId), `${service}.meta.json`)
  }
  
  // ── Token operations ──────────────────────────────────────────
  
  async setToken(
    userId: string,
    service: IntegrationCredentialService,
    token: string,
    config?: Record<string, string>
  ): Promise<void> {
    mkdirSync(this.userCredDir(userId), { recursive: true, mode: 0o700 })
    
    const iv = randomBytes(16)
    const cipher = createCipheriv('aes-256-gcm', this.encryptionKey, iv)
    const encrypted = Buffer.concat([cipher.update(token, 'utf8'), cipher.final()])
    const authTag = cipher.getAuthTag()
    const stored = Buffer.concat([iv, authTag, encrypted])
    
    writeFileSync(this.tokenPath(userId, service), stored, { mode: 0o600 })
    
    const meta: CredentialMetadata = {
      service,
      createdAt: Date.now(),
      config
    }
    writeFileSync(this.metaPath(userId, service), JSON.stringify(meta), { mode: 0o600 })
  }
  
  async getToken(
    userId: string,
    service: IntegrationCredentialService
  ): Promise<string | null> {
    try {
      const raw = readFileSync(this.tokenPath(userId, service))
      if (raw.length < 33) return null
      
      const iv = raw.subarray(0, 16)
      const authTag = raw.subarray(16, 32)
      const encrypted = raw.subarray(32)
      
      const decipher = createDecipheriv('aes-256-gcm', this.encryptionKey, iv)
      decipher.setAuthTag(authTag)
      return decipher.update(encrypted) + decipher.final('utf8')
    } catch {
      return null
    }
  }
  
  async hasToken(userId: string, service: IntegrationCredentialService): Promise<boolean> {
    return existsSync(this.tokenPath(userId, service))
  }
  
  async getMetadata(
    userId: string,
    service: IntegrationCredentialService
  ): Promise<CredentialMetadata | null> {
    try {
      const raw = readFileSync(this.metaPath(userId, service), 'utf8')
      return JSON.parse(raw) as CredentialMetadata
    } catch {
      return null
    }
  }
  
  async deleteToken(userId: string, service: IntegrationCredentialService): Promise<void> {
    try { unlinkSync(this.tokenPath(userId, service)) } catch {}
    try { unlinkSync(this.metaPath(userId, service)) } catch {}
  }
  
  async listTokens(userId: string): Promise<IntegrationCredentialService[]> {
    try {
      const dir = this.userCredDir(userId)
      return readdirSync(dir)
        .filter(f => f.endsWith('.enc'))
        .map(f => f.replace('.enc', '') as IntegrationCredentialService)
    } catch {
      return []
    }
  }
  
  // ── Session cleanup ───────────────────────────────────────────
  
  async deleteUserCredentials(userId: string): Promise<void> {
    const dir = this.userCredDir(userId)
    // Secure deletion: overwrite before unlink
    try {
      const files = readdirSync(dir)
      for (const file of files) {
        const path = join(dir, file)
        const stat = statSync(path)
        writeFileSync(path, randomBytes(stat.size))
        unlinkSync(path)
      }
    } catch {}
  }
}
```

---

## RPC Methods: `credentials.*`

**File:** `src/main/runtime/rpc/methods/credentials.ts` [NEW]

```typescript
import { z } from 'zod'
import { defineMethod } from '../core'
import { getWebCredentialStore } from '../../credentials/web-credential-store'
import type { IntegrationCredentialService } from '../../../shared/integration-credential-errors'

const ServiceEnum = z.enum(['bitbucket', 'azure-devops', 'gitea', 'linear', 'jira'])

export const CREDENTIAL_METHODS = [
  defineMethod({
    name: 'credentials.set',
    params: z.object({
      service: ServiceEnum,
      token: z.string().min(1),
      config: z.record(z.string()).optional()
    }),
    handler: async (params, context) => {
      if (!context.userId) throw new Error('Authentication required')
      const store = getWebCredentialStore()
      await store.setToken(context.userId, params.service, params.token, params.config)
      return { success: true }
    }
  }),
  
  defineMethod({
    name: 'credentials.revoke',
    params: z.object({ service: ServiceEnum }),
    handler: async (params, context) => {
      if (!context.userId) throw new Error('Authentication required')
      const store = getWebCredentialStore()
      await store.deleteToken(context.userId, params.service)
      return { success: true }
    }
  }),
  
  defineMethod({
    name: 'credentials.status',
    params: z.object({ service: ServiceEnum }),
    handler: async (params, context) => {
      if (!context.userId) throw new Error('Authentication required')
      const store = getWebCredentialStore()
      const has = await store.hasToken(context.userId, params.service)
      const meta = has ? await store.getMetadata(context.userId, params.service) : null
      return {
        configured: has,
        // Do NOT return token or config with sensitive values
        createdAt: meta?.createdAt ?? null,
        config: sanitizeConfigForDisplay(meta?.config)
      }
    }
  }),
  
  defineMethod({
    name: 'credentials.list',
    params: null,
    handler: async (_params, context) => {
      if (!context.userId) throw new Error('Authentication required')
      const store = getWebCredentialStore()
      return store.listTokens(context.userId)
    }
  })
]
```

---

## `getWebCredentialStore()` singleton initializer

**File:** `src/main/credentials/index.ts` [NEW]
```typescript
import { WebCredentialStore } from './web-credential-store'

let _store: WebCredentialStore | null = null

export function initWebCredentialStore(userDataPath: string, serverSecret: string): void {
  _store = WebCredentialStore.create(userDataPath, serverSecret)
}

export function getWebCredentialStore(): WebCredentialStore {
  if (!_store) throw new Error('WebCredentialStore not initialized')
  return _store
}
```

**File:** `src/main/server-bootstrap.ts`
```typescript
import { initWebCredentialStore } from './credentials'

// In server startup:
const serverSecret = process.env.ORCA_SERVER_SECRET 
  ?? generateDeterministicSecret(userDataPath)  // fallback: hash of data path
initWebCredentialStore(userDataPath, serverSecret)
```

---

## Files cần thay đổi

### [NEW] `src/main/credentials/web-credential-store.ts`
### [NEW] `src/main/credentials/index.ts`  
### [NEW] `src/main/runtime/rpc/methods/credentials.ts`

### [MODIFY] `src/main/runtime/rpc/methods/index.ts`
- Register `CREDENTIAL_METHODS`

### [MODIFY] `src/main/server-bootstrap.ts`
- `initWebCredentialStore(userDataPath, serverSecret)`

### [MODIFY] `src/renderer/src/web/web-preload-api.ts`
- `credentials.set()`, `credentials.revoke()`, `credentials.status()`, `credentials.list()`

### [MODIFY] `deploy/dev/.env.example`
- Thêm `ORCA_SERVER_SECRET=` với hướng dẫn generate

---

## Acceptance Criteria

1. `credentials.set('bitbucket', 'app-password-xyz')` lưu encrypted per-user
2. `credentials.status('bitbucket')` trả về `{ configured: true }` không expose token
3. `credentials.revoke('bitbucket')` xóa token an toàn (overwrite trước khi unlink)
4. Mỗi user có directory riêng không thể access lẫn nhau
5. Server restart: tokens còn tồn tại (disk-persisted)
6. `ORCA_SERVER_SECRET` rotate: tất cả tokens invalid → user cần re-enter

## Related

- CR-INT-002: Bitbucket/AzDO/Gitea integration
- CR-INT-003: Linear/Jira integration
- CR-GH-005: RPC method context injection
