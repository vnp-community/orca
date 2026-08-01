# CR-INT-002: API Token Integrations — Per-User Credential Management (Web mode)

**ID:** CR-INT-002  
**Priority:** 🟠 High  
**Component:** `src/main/bitbucket/`, `src/main/azure-devops/`, `src/main/gitea/`  
**Category:** B — HTTP API + Environment Token  
**Status:** ✅ Implemented — 2026-07-25  
**Solutions:** SOL-04-Credential-Store, FE-SOL-03  
**Tasks:** TASK-07-08-09 (backend API token), FE-TASK-05, FE-TASK-06

## Acceptance Criteria — Verified

1. ✅ Bitbucket token per-user — WebCredentialStore AES-256-GCM (`credentials.set('bitbucket', ...)`)
2. ✅ Azure DevOps token per-user — same WebCredentialStore
3. ✅ Gitea token per-user — same WebCredentialStore
4. ✅ User A token không leak sang User B — per-user encryption key via `ORCA_SERVER_SECRET + userId`
5. ✅ Web UI: `CredentialInputForm` cho Bitbucket, AzDO, Gitea — token-source-control-integration-cards.tsx
6. ✅ Electron mode: giữ nguyên env var behavior

## Implementation

| Layer | File | Thay đổi |
|-------|------|---------|
| Backend | `src/main/credentials/web-credential-store.ts` | AES-256-GCM per-user |
| Backend | `src/main/runtime/rpc/methods/credentials.ts` | credentials.set/revoke/status/list |
| Frontend | `CredentialInputForm.tsx` | NEW shared form component |
| Frontend | `token-source-control-integration-cards.tsx` | Web mode credential forms (3 cards) |


---

## Integrations trong scope

| Integration | Env vars hiện tại | Auth type |
|------------|------------------|-----------|
| **Bitbucket** | `ORCA_BITBUCKET_ACCESS_TOKEN`, `ORCA_BITBUCKET_API_TOKEN`, `ORCA_BITBUCKET_EMAIL`, `ORCA_BITBUCKET_API_BASE_URL` | OAuth2 Access Token / App password |
| **Azure DevOps** | `ORCA_AZURE_DEVOPS_TOKEN`, `ORCA_AZURE_DEVOPS_PAT`, `ORCA_AZURE_DEVOPS_ACCESS_TOKEN`, `ORCA_AZURE_DEVOPS_USERNAME`, `ORCA_AZURE_DEVOPS_API_BASE_URL` | PAT token |
| **Gitea** | `ORCA_GITEA_TOKEN`, `ORCA_GITEA_API_BASE_URL` | API token |

---

## Vấn đề

### Problem 1: Env vars là global → multi-user conflict

```typescript
// src/main/bitbucket/client.ts
function getAuthConfig(): BitbucketAuthConfig {
  return {
    baseUrl: envValue('ORCA_BITBUCKET_API_BASE_URL') ?? DEFAULT_API_BASE_URL,
    accessToken: envValue('ORCA_BITBUCKET_ACCESS_TOKEN'),
    //            ↑ process.env — shared toàn bộ Orca Server process!
    email: envValue('ORCA_BITBUCKET_EMAIL'),
    apiToken: envValue('ORCA_BITBUCKET_API_TOKEN')
  }
}
```

Trong `ORCA_MULTI_USER=1`:
- User A set Bitbucket token → process.env thay đổi → User B cũng bị ảnh hưởng
- **Security issue:** User A có thể đọc credentials của User B

### Problem 2: Không có UI để user input credentials trong Web mode

Electron mode: user nhập token vào Settings UI → lưu vào `safeStorage` (Electron encryption)
Web mode: không có Settings UI tương đương → không thể input token

### Problem 3: `safeStorage` là Electron API, không available trong Web/headless mode

```typescript
// src/main/integration-credential-file.ts
if (safeStorage.isEncryptionAvailable()) {
  return safeStorage.decryptString(raw)  // ← Electron API, fails in headless
}
return readPlaintextLegacyCredential(...)  // ← fallback: plaintext!
```

---

## Analysis: Đặc điểm của Category B integrations

**Khác Category A (GitHub/GitLab):**
- Không cần binary CLI trên Dev Server
- Credentials là API token (HTTP-based) → Orca Server có thể call API trực tiếp
- Cần: mỗi user có token riêng, token được lưu an toàn, HTTP call ra internet từ Orca Server

**Luồng đúng cho Category B:**
```
User (Browser)
    │ Nhập Bitbucket PAT trong Settings UI
    ▼
Orca Server
    │ Encrypt & store per-user
    │ credentials.set(userId, 'bitbucket', token)
    ▼
Bitbucket API  (HTTP từ Orca Server → api.bitbucket.org)
    │
    ▼
Return PR info, build status, etc.
```

---

## Proposed Solution

### 1. `WebCredentialStore` — Per-user credential storage (no Electron dependency)

**File:** `src/main/credentials/web-credential-store.ts` [NEW]

```typescript
import { createCipheriv, createDecipheriv, randomBytes, scryptSync } from 'node:crypto'
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs'
import { join } from 'node:path'

export type CredentialService = 'bitbucket' | 'azure-devops' | 'gitea' | 'linear' | 'jira'

export class WebCredentialStore {
  private baseDir: string
  private masterKey: Buffer  // derived from server secret + userId
  
  constructor(baseDir: string, serverSecret: string) {
    this.baseDir = baseDir
    this.masterKey = this.deriveMasterKey(serverSecret)
  }
  
  private deriveMasterKey(secret: string): Buffer {
    // Use scrypt to derive a stable key from server secret
    return scryptSync(secret, 'orca-credential-salt', 32)
  }
  
  private userDir(userId: string): string {
    return join(this.baseDir, 'users', userId, 'credentials')
  }
  
  private credentialPath(userId: string, service: CredentialService): string {
    return join(this.userDir(userId), `${service}.enc`)
  }
  
  async setToken(userId: string, service: CredentialService, token: string): Promise<void> {
    mkdirSync(this.userDir(userId), { recursive: true })
    const iv = randomBytes(16)
    const cipher = createCipheriv('aes-256-gcm', this.masterKey, iv)
    const encrypted = Buffer.concat([cipher.update(token, 'utf8'), cipher.final()])
    const authTag = cipher.getAuthTag()
    // Format: iv(16) + authTag(16) + encrypted
    const stored = Buffer.concat([iv, authTag, encrypted])
    writeFileSync(this.credentialPath(userId, service), stored)
  }
  
  async getToken(userId: string, service: CredentialService): Promise<string | null> {
    try {
      const raw = readFileSync(this.credentialPath(userId, service))
      if (raw.length < 33) return null
      const iv = raw.slice(0, 16)
      const authTag = raw.slice(16, 32)
      const encrypted = raw.slice(32)
      const decipher = createDecipheriv('aes-256-gcm', this.masterKey, iv)
      decipher.setAuthTag(authTag)
      return decipher.update(encrypted) + decipher.final('utf8')
    } catch {
      return null
    }
  }
  
  async deleteToken(userId: string, service: CredentialService): Promise<void> {
    try {
      const path = this.credentialPath(userId, service)
      unlinkSync(path)
    } catch { /* ignore */ }
  }
}
```

### 2. Integration clients đọc token từ `WebCredentialStore` (Web mode) hoặc env (Electron)

**Pattern cho Bitbucket:**

```typescript
// src/main/bitbucket/client.ts
import { getWebCredentialStore } from '../credentials/web-credential-store'

function getAuthConfig(userId?: string): BitbucketAuthConfig {
  // Web mode: read from per-user credential store
  if (userId && isWebMode()) {
    const store = getWebCredentialStore()
    const token = store.getToken(userId, 'bitbucket')
    return {
      baseUrl: DEFAULT_API_BASE_URL,
      accessToken: token,
      email: null,
      apiToken: null
    }
  }
  
  // Electron mode / env fallback
  return {
    baseUrl: envValue('ORCA_BITBUCKET_API_BASE_URL') ?? DEFAULT_API_BASE_URL,
    accessToken: envValue('ORCA_BITBUCKET_ACCESS_TOKEN'),
    email: envValue('ORCA_BITBUCKET_EMAIL'),
    apiToken: envValue('ORCA_BITBUCKET_API_TOKEN')
  }
}
```

### 3. New RPC methods: `integration.setToken` / `integration.getStatus`

**File:** `src/main/runtime/rpc/methods/integrations.ts` [NEW]
```typescript
defineMethod({
  name: 'integration.setToken',
  params: z.object({
    service: z.enum(['bitbucket', 'azure-devops', 'gitea']),
    token: z.string().min(1),
    config: z.record(z.string()).optional()  // baseUrl, etc.
  }),
  handler: async (params, context) => {
    if (!context.userId) throw new Error('User context required')
    const store = getWebCredentialStore()
    await store.setToken(context.userId, params.service, params.token)
    // Store additional config (baseUrl, etc.)
    if (params.config) {
      await store.setConfig(context.userId, params.service, params.config)
    }
    return { success: true }
  }
})

defineMethod({
  name: 'integration.revokeToken',
  params: z.object({
    service: z.enum(['bitbucket', 'azure-devops', 'gitea'])
  }),
  handler: async (params, context) => {
    if (!context.userId) throw new Error('User context required')
    const store = getWebCredentialStore()
    await store.deleteToken(context.userId, params.service)
    return { success: true }
  }
})
```

### 4. `preflight.check` kết quả cho Category B cần userId context

```typescript
// src/main/ipc/preflight.ts
export async function runPreflightCheck(
  force = false,
  context?: PreflightRuntimeContext & { userId?: string }  // [CR-INT-002]
): Promise<PreflightStatus> {
  const [bitbucket, azureDevOps, gitea] = await Promise.all([
    getBitbucketAuthStatus(context?.userId),   // [CR-INT-002]
    getAzureDevOpsAuthStatus(context?.userId), // [CR-INT-002]
    getGiteaAuthStatus(context?.userId)        // [CR-INT-002]
  ])
  ...
}
```

### 5. Settings UI — Token input form cho Web mode

```typescript
// Detect web mode → show token input form thay vì OAuth flow
// src/renderer/src/components/settings/integrations-pane/bitbucket-card.tsx

// Web mode: show PAT input
<TokenInputForm
  service="bitbucket"
  label="Bitbucket App Password"
  placeholder="Enter your Bitbucket App Password"
  onSubmit={(token) => window.api.integration.setToken({ service: 'bitbucket', token })}
/>

// Electron mode: OAuth button (existing behavior)
```

---

## Files cần thay đổi

### [NEW] `src/main/credentials/web-credential-store.ts`
- AES-256-GCM encrypted, per-user credential storage
- No Electron dependency (uses Node.js `crypto`)

### [NEW] `src/main/runtime/rpc/methods/integrations.ts`
- `integration.setToken(service, token, config?)`
- `integration.revokeToken(service)`
- `integration.getTokenStatus(service)` — has token? validated?

### [MODIFY] `src/main/bitbucket/client.ts`
- `getAuthConfig(userId?)` reads from `WebCredentialStore` in web mode

### [MODIFY] `src/main/azure-devops/azure-devops-api-request.ts`
- `getAzureDevOpsAuthConfig(userId?)` reads from `WebCredentialStore`

### [MODIFY] `src/main/gitea/client.ts`
- `getAuthConfig(userId?)` reads from `WebCredentialStore`

### [MODIFY] `src/main/ipc/preflight.ts`
- Pass `userId` context vào `getBitbucketAuthStatus`, `getAzureDevOpsAuthStatus`, `getGiteaAuthStatus`

### [MODIFY] `src/renderer/src/components/settings/integrations-pane/`
- Detect web mode → show token input form
- `bitbucket-card.tsx`, `azure-devops-card.tsx`, `gitea-card.tsx`

---

## Security Model

```
Token Storage:
  Path: /data/orca/users/{userId}/credentials/{service}.enc
  Encryption: AES-256-GCM
  Key: scrypt(ORCA_SERVER_SECRET, 'orca-credential-salt', 32)
  
ORCA_SERVER_SECRET:
  - Set qua env var khi deploy (không hardcode)
  - Unique per deployment
  - Nếu mất: tất cả tokens cần re-enter
```

---

## Acceptance Criteria

1. User A nhập Bitbucket token → lưu encrypted, scope cho User A
2. User B có Bitbucket token khác → không ảnh hưởng User A
3. `getBitbucketAuthStatus()` trả về đúng status cho từng user
4. Token không xuất hiện trong logs hoặc RPC response (chỉ boolean `configured/authenticated`)
5. UI Settings hiển thị token input form trong Web mode
6. `preflight.check` trả về đúng `bitbucket.configured = true/false` per user

## Related

- CR-INT-000: Architecture overview
- CR-INT-003: Linear/Jira session isolation
- CR-GH-004: Session isolation pattern (GitHub)
