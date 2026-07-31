# TASK-07, 08, 09: Chuyển Bitbucket, Azure DevOps, Gitea sang `WebCredentialStore`

**Status:** ✅ DONE — 2026-07-25  
**Phase:** 4 — Integration Clients  
**Priority:** 🟠 High  
**Depends on:** TASK-02 (WebCredentialStore)  
**Solution:** SOL-04-Credential-Store.md  
**CRs:** CR-INT-002  
**Estimated effort:** ~60 phút (3 integrations)

---

## Mục tiêu

Thay thế việc đọc credentials từ global `process.env.*` bằng `WebCredentialStore` khi chạy trong Web mode. Giữ nguyên fallback sang `process.env.*` cho Electron mode.

---

## Pattern chung cho cả 3 integrations

```typescript
// Pattern áp dụng cho Bitbucket, Azure DevOps, Gitea:

function getAuthConfig() {
  if (isWebCredentialMode()) {
    const store = getWebCredentialStore()
    const token = await store.getToken(SERVICE_NAME)
    const config = await store.getConfig(SERVICE_NAME)
    return buildAuthConfigFromStore(token, config)
  }
  // Electron/env fallback (unchanged)
  return buildAuthConfigFromEnv()
}
```

---

## TASK-07: `src/main/bitbucket/client.ts`

### Thay đổi

Sửa `getAuthConfig()` (line 43–50 hiện tại):

```typescript
// THÊM imports đầu file:
import { isWebCredentialMode, getWebCredentialStore } from '../credentials'

// SỬA getAuthConfig():
function getAuthConfig(): BitbucketAuthConfig {
  // Web mode: đọc từ WebCredentialStore per-user
  if (isWebCredentialMode()) {
    // Lưu ý: getToken là async nhưng getAuthConfig cần đồng bộ trong code hiện tại.
    // Cần refactor getBitbucketAuthStatus() thành async-aware hoặc
    // dùng getToken sync wrapper từ credential store.
    // --> Giải pháp: đọc cached token được inject qua ORCA_BITBUCKET_TOKEN env
    //     khi spawn user-process, hoặc refactor thành async.
    //
    // Tạm thời: user-process được inject biến môi trường ORCA_BITBUCKET_ACCESS_TOKEN
    // từ credential store trước khi spawn (trong SessionManager.getOrSpawn).
    // Xem TASK-14 để biết cơ chế inject.
  }

  // Electron mode / env fallback (unchanged):
  return {
    baseUrl: envValue('ORCA_BITBUCKET_API_BASE_URL') ?? DEFAULT_API_BASE_URL,
    accessToken: envValue('ORCA_BITBUCKET_ACCESS_TOKEN'),
    email: envValue('ORCA_BITBUCKET_EMAIL'),
    apiToken: envValue('ORCA_BITBUCKET_API_TOKEN')
  }
}
```

### Lưu ý quan trọng — Approach thay thế (recommended)

Thay vì sửa `getAuthConfig()` thành async, sử dụng cơ chế **env var injection tại spawn time**:

1. Khi `SessionManager.getOrSpawn(userId)` fork process con, đọc credential từ `WebCredentialStore`
2. Inject các biến môi trường này vào `env` của child process: `ORCA_BITBUCKET_ACCESS_TOKEN`, v.v.
3. Child process đọc token từ `process.env` như bình thường → **zero code change trong client.ts**

Cơ chế này được xử lý ở **TASK-14 (Server Bootstrap / SessionManager)**.

---

## TASK-08: `src/main/azure-devops/azure-devops-api-request.ts`

### Thay đổi

Tương tự Bitbucket. Sửa `getAzureDevOpsAuthConfig()` hoặc inject env vars tại spawn time:

**Env vars cần inject:**
- `ORCA_AZURE_DEVOPS_TOKEN`
- `ORCA_AZURE_DEVOPS_API_BASE_URL`
- `ORCA_AZURE_DEVOPS_USERNAME`

---

## TASK-09: `src/main/gitea/client.ts`

### Thay đổi

Tương tự. Sửa `getAuthConfig()` hoặc inject env vars:

**Env vars cần inject:**
- `ORCA_GITEA_TOKEN`
- `ORCA_GITEA_API_BASE_URL`

---

## Approach được chọn: Env Var Injection tại Spawn Time

**Lý do:** Category B integrations (Bitbucket, AzDO, Gitea) đọc token từ `process.env` ở nhiều nơi (client.ts, pull-request-creation.ts, v.v.). Sửa từng chỗ đọc sẽ phức tạp và có thể bỏ sót.

**Approach env injection không cần sửa client code:**

```typescript
// SessionManager.getOrSpawn() sẽ làm:
const credStore = new WebCredentialStore(userDataPath, userId, serverSecret)

const [bitbucketToken, azureToken, giteaToken] = await Promise.all([
  credStore.getToken('bitbucket'),
  credStore.getToken('azure-devops'),
  credStore.getToken('gitea')
])
const [bitbucketConfig, azureConfig, giteaConfig] = await Promise.all([
  credStore.getConfig('bitbucket'),
  credStore.getConfig('azure-devops'),
  credStore.getConfig('gitea')
])

const childEnv = {
  ...process.env,
  ORCA_USER_ID: userId,
  // Inject Bitbucket tokens
  ...(bitbucketToken ? { ORCA_BITBUCKET_ACCESS_TOKEN: bitbucketToken } : {}),
  ...(bitbucketConfig?.email ? { ORCA_BITBUCKET_EMAIL: bitbucketConfig.email } : {}),
  // Inject Azure DevOps tokens
  ...(azureToken ? { ORCA_AZURE_DEVOPS_TOKEN: azureToken } : {}),
  ...(azureConfig?.apiBaseUrl ? { ORCA_AZURE_DEVOPS_API_BASE_URL: azureConfig.apiBaseUrl } : {}),
  // Inject Gitea tokens
  ...(giteaToken ? { ORCA_GITEA_TOKEN: giteaToken } : {}),
  ...(giteaConfig?.apiBaseUrl ? { ORCA_GITEA_API_BASE_URL: giteaConfig.apiBaseUrl } : {}),
}

// Spawn child process với env được inject
fork('user-process-entry', { env: childEnv })
```

**Files cần sửa cho approach này: chỉ `SessionManager` (TASK-14)**

---

## Acceptance Criteria (với env injection approach)

1. User A lưu Bitbucket token → session process của User A có `ORCA_BITBUCKET_ACCESS_TOKEN` đúng
2. User B lưu Bitbucket token khác → session process User B có token riêng
3. `getBitbucketAuthStatus()` trả về `authenticated: true` khi token hợp lệ
4. Không có thay đổi code trong `bitbucket/client.ts`, `azure-devops/azure-devops-api-request.ts`, `gitea/client.ts`

---

## Files cần sửa

- **Không cần sửa** các integration client files nếu dùng env injection approach
- Xem **TASK-14** để thực hiện env injection trong SessionManager
