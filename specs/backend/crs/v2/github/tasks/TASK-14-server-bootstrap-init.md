# TASK-14: Khởi tạo `WebCredentialStore` và inject credentials vào User Process

**Status:** ✅ DONE — 2026-07-25 (AC verified 2026-07-25)  
**Phase:** 5 — Server Bootstrap  
**Priority:** 🟡 Medium  
**Depends on:** TASK-02 (WebCredentialStore)  
**Solution:** SOL-04-Credential-Store.md  
**CRs:** CR-INT-002, CR-INT-003, CR-INT-004  
**Estimated effort:** ~60 phút

---

## Mục tiêu

Tích hợp `WebCredentialStore` vào 2 điểm trong server lifecycle:

1. **Server Bootstrap** (`src/main/server-bootstrap.ts`): Khởi tạo `WebCredentialStore` singleton khi server start
2. **Session Manager** (`src/main/session/session-manager.ts`): Inject credential env vars vào child process env khi spawn user-process

---

## Phần 1: Server Bootstrap — Init WebCredentialStore singleton

### Hiện trạng code

**File:** `src/main/server-bootstrap.ts` — cần thêm init call cho `WebCredentialStore`.

### Thay đổi cần thực hiện

```typescript
// src/main/server-bootstrap.ts

// THÊM import:
import { initWebCredentialStore } from './credentials'

// Trong initializeOrcaServices(), sau khi init user data path:
export async function initializeOrcaServices(
  options: ServerBootstrapOptions
): Promise<ServerBootstrapResult> {

  // Existing code...
  const userDataPath = process.env['ORCA_USER_DATA_PATH'] ?? join(homedir(), '.orca')

  // THÊM: Init WebCredentialStore cho web mode
  if (process.env['ORCA_MULTI_USER'] === '1') {
    const userId = process.env['ORCA_USER_ID'] ?? 'default'
    const serverSecret = process.env['ORCA_SERVER_SECRET']
      ?? `orca-default-secret-${userDataPath}`  // Fallback deterministic (không dùng trong production)

    if (!process.env['ORCA_SERVER_SECRET']) {
      console.warn('[ServerBootstrap] ORCA_SERVER_SECRET not set — using deterministic fallback (not secure for production!)')
    }

    initWebCredentialStore(userDataPath, userId, serverSecret)
    console.log(`[ServerBootstrap] WebCredentialStore initialized for user ${userId}`)
  }

  // ... rest of existing code ...
}
```

---

## Phần 2: Session Manager — Inject credentials vào child env

### Hiện trạng code

**File:** `src/main/session/session-manager.ts` — `spawnUserProcess()` (line 52–69):

```typescript
private async spawnUserProcess(userId: string): Promise<UserProcess> {
  const userDataPath = join(this.config.baseDataPath, 'users', userId)
  const socketPath   = join(userDataPath, 'orca.sock')

  await mkdir(userDataPath, { recursive: true, mode: 0o700 })

  const child = fork(this.config.userProcessEntry, [], {
    env: {
      ...process.env,
      ORCA_USER_DATA_PATH: userDataPath,
      ORCA_USER_ID:        userId,
      ORCA_SOCKET_PATH:    socketPath,
      NODE_OPTIONS:        '--max-old-space-size=512'
      // ↑ Chưa có credentials injection!
    },
    ...
  })
}
```

### Thay đổi cần thực hiện

Thêm logic đọc credentials từ `WebCredentialStore` và inject vào child process env:

```typescript
// src/main/session/session-manager.ts

// THÊM import:
import { WebCredentialStore } from '../credentials/web-credential-store'

// THÊM trong SessionManagerConfig:
type SessionManagerConfig = {
  baseDataPath: string
  userProcessEntry: string
  idleTimeoutMs?: number
  maxRespawnAttempts?: number
  serverSecret?: string  // THÊM — để init WebCredentialStore per-user
}

// SỬA spawnUserProcess():
private async spawnUserProcess(userId: string): Promise<UserProcess> {
  const userDataPath = join(this.config.baseDataPath, 'users', userId)
  const socketPath   = join(userDataPath, 'orca.sock')

  await mkdir(userDataPath, { recursive: true, mode: 0o700 })

  // THÊM: Đọc credentials từ WebCredentialStore cho user này
  const credentialEnv: Record<string, string> = {}
  if (this.config.serverSecret) {
    try {
      const credStore = new WebCredentialStore(
        this.config.baseDataPath,
        userId,
        this.config.serverSecret
      )

      // Inject Bitbucket credentials
      const bitbucketToken = await credStore.getToken('bitbucket')
      const bitbucketConfig = await credStore.getConfig('bitbucket')
      if (bitbucketToken) {
        credentialEnv['ORCA_BITBUCKET_ACCESS_TOKEN'] = bitbucketToken
      }
      if (bitbucketConfig?.email) {
        credentialEnv['ORCA_BITBUCKET_EMAIL'] = bitbucketConfig.email
      }
      if (bitbucketConfig?.apiBaseUrl) {
        credentialEnv['ORCA_BITBUCKET_API_BASE_URL'] = bitbucketConfig.apiBaseUrl
      }

      // Inject Azure DevOps credentials
      const azureToken = await credStore.getToken('azure-devops')
      const azureConfig = await credStore.getConfig('azure-devops')
      if (azureToken) {
        credentialEnv['ORCA_AZURE_DEVOPS_TOKEN'] = azureToken
      }
      if (azureConfig?.apiBaseUrl) {
        credentialEnv['ORCA_AZURE_DEVOPS_API_BASE_URL'] = azureConfig.apiBaseUrl
      }
      if (azureConfig?.username) {
        credentialEnv['ORCA_AZURE_DEVOPS_USERNAME'] = azureConfig.username
      }

      // Inject Gitea credentials
      const giteaToken = await credStore.getToken('gitea')
      const giteaConfig = await credStore.getConfig('gitea')
      if (giteaToken) {
        credentialEnv['ORCA_GITEA_TOKEN'] = giteaToken
      }
      if (giteaConfig?.apiBaseUrl) {
        credentialEnv['ORCA_GITEA_API_BASE_URL'] = giteaConfig.apiBaseUrl
      }
    } catch (err) {
      // Non-fatal: credentials not available yet (user hasn't set them)
      console.warn(`[SessionManager] Could not load credentials for user ${userId}:`, err)
    }
  }

  const child = fork(this.config.userProcessEntry, [], {
    env: {
      ...process.env,
      ...credentialEnv,              // THÊM: inject credentials
      ORCA_USER_DATA_PATH: userDataPath,
      ORCA_USER_ID:        userId,
      ORCA_SOCKET_PATH:    socketPath,
      NODE_OPTIONS:        '--max-old-space-size=512'
    },
    ...
  })
}
```

### Truyền `serverSecret` vào SessionManager

**File:** `src/main/server-bootstrap.ts` — khi khởi tạo `SessionManager`:

```typescript
const sessionManager = new SessionManager({
  baseDataPath: userDataPath,
  userProcessEntry: join(__dirname, 'user-process-entry.js'),
  serverSecret: process.env['ORCA_SERVER_SECRET']  // THÊM
})
```

---

## Thêm `ORCA_SERVER_SECRET` vào deploy config

**File:** `deploy/dev/.env`:
```bash
# Credential encryption key — generate with: openssl rand -hex 32
ORCA_SERVER_SECRET=your-random-64-char-hex-string-here
```

**File:** `deploy/dev/.env.example` (nếu có):
```bash
# REQUIRED for multi-user credential encryption
# Generate: openssl rand -hex 32
ORCA_SERVER_SECRET=
```

---

## Limitation quan trọng: Credential reload sau khi spawn

Khi user đặt token qua `credentials.set()`, credentials chỉ được inject vào **session process mới** (lần spawn tiếp theo). Session process **đang chạy** sẽ không nhận được token mới ngay lập tức.

**Giải pháp Phase 1:** Document limitation này — user cần reconnect để token có hiệu lực.  
**Giải pháp Phase 2:** IPC message từ main process gửi updated env vars vào running child process.

---

## Acceptance Criteria

1. ✅ Khi server start với `ORCA_MULTI_USER=1`: `WebCredentialStore` được init
2. ✅ Khi spawn user process: credentials được đọc từ `WebCredentialStore` và inject vào env
3. ✅ Child process của user A có `ORCA_BITBUCKET_ACCESS_TOKEN` của user A
4. ✅ Child process của user B có token riêng của user B (khác A)
5. ✅ Warning log khi `ORCA_SERVER_SECRET` không được set
6. ✅ Lỗi đọc credential là non-fatal (không stop server)
7. ✅ `SessionManager` được instantiate trong `server-bootstrap.ts` với `serverSecret`
8. ✅ `ServerBootstrapResult` type expose `sessionManager` field

---

## Files cần sửa

- `src/main/server-bootstrap.ts` [MODIFY] — init WebCredentialStore
- `src/main/session/session-manager.ts` [MODIFY] — inject credential env vars
- `src/main/session/session-types.ts` [MODIFY] — thêm `serverSecret` vào config type
- `deploy/dev/.env` [MODIFY] — thêm `ORCA_SERVER_SECRET=`
