# CR-INT-003: File-based Token Integrations — Session Isolation (Linear, Jira)

**ID:** CR-INT-003  
**Priority:** 🟠 High  
**Component:** `src/main/linear/`, `src/main/jira/`  
**Category:** C — File-based Token (Orca Server, session isolation needed)  
**Status:** ✅ Implemented — 2026-07-25  
**Solutions:** SOL-04-Credential-Store, FE-SOL-03  
**Tasks:** TASK-10-11 (backend Linear/Jira credential), FE-TASK-05, FE-TASK-07

## Acceptance Criteria — Verified

1. ✅ Linear token per-user — WebCredentialStore (`credentials.set('linear', ...)`)
2. ✅ Jira token per-user — WebCredentialStore (`credentials.set('jira', ...)`)
3. ✅ Token path không còn global/shared — credential store per userId
4. ✅ Web UI: LinearIntegrationCard và JiraIntegrationCard hiển thị CredentialInputForm trong Web mode
5. ✅ Electron mode: giữ nguyên LinearApiKeyDialog / JiraConnectDialog behavior

## Implementation

| Layer | File | Thay đổi |
|-------|------|---------|
| Backend | `src/main/credentials/web-credential-store.ts` | AES-256-GCM per-user |
| Backend | `src/main/runtime/rpc/methods/credentials.ts` | credentials.set/revoke/status/list |
| Frontend | `task-tracker-integration-cards.tsx` | LinearIntegrationCard: isWebMode form |
| Frontend | `jira-integration-card.tsx` | JiraIntegrationCard: isWebMode form |


---

## Integrations trong scope

| Integration | Token storage | Encryption | Issue |
|------------|--------------|-----------|-------|
| **Linear** | `~/.orca/linear-token.enc` | `safeStorage` (Electron) | Không isolated, safeStorage không available |
| **Jira** | `~/.orca/jira-tokens/{base64(siteId)}.enc` | `safeStorage` (Electron) | Không isolated per user, safeStorage không available |

---

## Vấn đề

### Problem 1: Path dùng home directory — shared giữa users

```typescript
// src/main/linear/client.ts
function getLinearTokenPath(): string {
  return join(getOrcaDir(), 'linear-token.enc')
  //                         ↑ ~/.orca/linear-token.enc — GLOBAL, không per-user!
}

// src/main/jira/client.ts  
function getJiraTokensDir(): string {
  return join(getOrcaDir(), 'jira-tokens')
  //                         ↑ ~/.orca/jira-tokens/ — GLOBAL!
}
```

Trong `ORCA_MULTI_USER=1` với `ORCA_USER_DATA_PATH=/data/orca`:
- `getOrcaDir()` → `/data/orca`
- Linear token path: `/data/orca/linear-token.enc` (shared!)
- Jira tokens: `/data/orca/jira-tokens/*.enc` (shared!)

### Problem 2: `safeStorage` (Electron) không available trong headless mode

```typescript
// src/main/integration-credential-file.ts
if (safeStorage.isEncryptionAvailable()) {
  return safeStorage.decryptString(raw)  // ← Fails: safeStorage is Electron API
}
// Fallback to plaintext → security issue in multi-user
return readPlaintextLegacyCredential(service, raw)
```

Trong Web mode (Node.js, không phải Electron process), `safeStorage` không tồn tại → token lưu plaintext.

### Problem 3: Linear token path hardcode per-app (không per-user)

Electron assumption: mỗi user chạy Electron app riêng → có home dir riêng.
Web mode assumption: nhiều users share cùng Orca Server process → cùng data dir.

---

## Analysis: Tại sao Linear/Jira khác Bitbucket/AzDO/Gitea?

**Bitbucket, AzDO, Gitea (CR-INT-002):**
- Token từ env var → HTTP call trực tiếp
- Solution: WebCredentialStore với userId

**Linear, Jira (CR-INT-003):**
- Token lưu trên disk với path cố định
- Code dùng nhiều chỗ: `client.ts`, `runtime/rpc/methods/linear.ts`, `jira.ts`
- Cần refactor storage layer

---

## Proposed Solution

### 1. Migrate token storage từ home-based sang user-data-based + per-user path

**File:** `src/main/linear/client.ts`
```typescript
// CŨ:
function getLinearTokenPath(): string {
  return join(getOrcaDir(), 'linear-token.enc')
}

// MỚI:
function getLinearTokenPath(userId?: string): string {
  if (userId && isWebMode()) {
    // Per-user path trong web/multi-user mode
    return join(getOrcaUserDataPath(), 'users', userId, 'linear-token.enc')
  }
  // Backward compatible cho Electron mode
  return join(getOrcaDir(), 'linear-token.enc')
}

function getLinearTokensDir(userId?: string): string {
  if (userId && isWebMode()) {
    return join(getOrcaUserDataPath(), 'users', userId, 'linear-tokens')
  }
  return join(getOrcaDir(), 'linear-tokens')
}
```

**File:** `src/main/jira/client.ts`
```typescript
// CŨ:
function getJiraTokensDir(): string {
  return join(getOrcaDir(), 'jira-tokens')
}

// MỚI:
function getJiraTokensDir(userId?: string): string {
  if (userId && isWebMode()) {
    return join(getOrcaUserDataPath(), 'users', userId, 'jira-tokens')
  }
  return join(getOrcaDir(), 'jira-tokens')
}
```

### 2. `WebCredentialStore` (từ CR-INT-002) thay thế `safeStorage`

Thay vì dùng `safeStorage` (Electron-only), dùng AES-256-GCM với key từ `ORCA_SERVER_SECRET`:

```typescript
// src/main/linear/client.ts
import { getWebCredentialStore } from '../credentials/web-credential-store'

async function saveLinearToken(token: string, userId?: string): Promise<void> {
  if (isWebMode() && userId) {
    const store = getWebCredentialStore()
    await store.setToken(userId, 'linear', token)
    return
  }
  // Electron: dùng safeStorage (existing behavior)
  const encrypted = safeStorage.isEncryptionAvailable()
    ? safeStorage.encryptString(token)
    : Buffer.from(token)
  writeFileSync(getLinearTokenPath(), encrypted)
}

async function readLinearToken(userId?: string): Promise<string | null> {
  if (isWebMode() && userId) {
    const store = getWebCredentialStore()
    return store.getToken(userId, 'linear')
  }
  // Electron: dùng readStoredCredentialToken (existing behavior)
  try {
    const raw = readFileSync(getLinearTokenPath())
    return readStoredCredentialToken('linear', raw)
  } catch {
    return null
  }
}
```

### 3. Linear/Jira RPC methods nhận `userId` từ context

**File:** `src/main/runtime/rpc/methods/linear.ts`
```typescript
defineMethod({
  name: 'linear.connect',
  params: z.object({ apiKey: z.string().min(1) }),
  handler: async (params, context) => {
    // [CR-INT-003] Pass userId cho per-user storage
    return linearClient.connect(params.apiKey, { userId: context.userId })
  }
})

defineMethod({
  name: 'linear.disconnect',
  params: null,
  handler: async (_params, context) => {
    return linearClient.disconnect({ userId: context.userId })
  }
})
```

### 4. Migration: Existing tokens trong `/data/orca/linear-token.enc`

Khi multi-user mode: cần migration script để move global tokens sang per-user paths.
Hoặc: first-time migration khi user đăng nhập lần đầu.

```typescript
// src/main/linear/client.ts
async function migrateGlobalLinearToken(userId: string): Promise<void> {
  const globalPath = join(getOrcaDir(), 'linear-token.enc')
  if (!existsSync(globalPath)) return
  
  const raw = readFileSync(globalPath)
  const token = readStoredCredentialToken('linear', raw)
  if (token) {
    await saveLinearToken(token, userId)
    unlinkSync(globalPath)  // Remove global token after migration
    console.log(`[linear] Migrated global token to user ${userId}`)
  }
}
```

---

## Files cần thay đổi

### [MODIFY] `src/main/linear/client.ts`
- `getLinearTokenPath(userId?)` — per-user path in web mode
- `saveLinearToken(token, userId?)` — use WebCredentialStore in web mode
- `readLinearToken(userId?)` — read from WebCredentialStore in web mode
- `linearClient.connect(token, { userId })`, `linearClient.disconnect({ userId })`
- `migrateGlobalLinearToken(userId)` — one-time migration

### [MODIFY] `src/main/jira/client.ts`
- `getJiraTokensDir(userId?)` — per-user path in web mode
- `saveJiraToken(siteId, token, userId?)` — use WebCredentialStore
- `readJiraToken(siteId, userId?)` — read from WebCredentialStore
- `listJiraSites(userId?)` — list per-user sites

### [MODIFY] `src/main/runtime/rpc/methods/linear.ts`
- `linear.connect` handler pass `context.userId`
- `linear.disconnect` handler pass `context.userId`
- `linear.status` handler pass `context.userId`

### [MODIFY] `src/main/runtime/rpc/methods/jira.ts`
- All jira methods pass `context.userId`

### [MODIFY] `src/main/ipc/preflight.ts`
- Not directly needed (Linear/Jira không có `preflight.check` fields)
- But: status check endpoints cần `userId`

---

## Token isolation per user: Data structure

```
/data/orca/
├── orca-data.json         (global config)
├── users/
│   ├── user-alice-uuid/
│   │   ├── credentials/
│   │   │   ├── linear.enc      (Linear API key - AES-GCM)
│   │   │   ├── jira-tokens/
│   │   │   │   ├── {base64(siteId1)}.enc
│   │   │   │   └── {base64(siteId2)}.enc
│   │   │   ├── bitbucket.enc   (từ CR-INT-002)
│   │   │   └── azure-devops.enc
│   │   └── gh/              (từ CR-GH-004, GH_CONFIG_DIR)
│   └── user-bob-uuid/
│       ├── credentials/
│       │   ├── linear.enc   (Bob's Linear token - khác Alice's)
│       │   └── bitbucket.enc
│       └── gh/
```

---

## Acceptance Criteria

1. User A connect Linear → token lưu tại `/data/orca/users/{userA-id}/credentials/linear.enc`
2. User B connect Linear khác → token lưu tại `/data/orca/users/{userB-id}/credentials/linear.enc`
3. `safeStorage` không được gọi trong web/headless mode (no Electron dependency)
4. Token được mã hóa AES-256-GCM (không plaintext)
5. Linear/Jira disconnect xóa token của đúng user
6. Migration: global token (nếu có) được migrate sang user path khi user login lần đầu

## Related

- CR-INT-002: WebCredentialStore shared infrastructure
- CR-INT-000: Architecture overview
- CR-GH-004: Session isolation pattern (GitHub)
