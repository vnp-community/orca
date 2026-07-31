# TASK-10 & TASK-11: Chuyển Linear và Jira sang `WebCredentialStore`

**Status:** ✅ DONE — 2026-07-25  
**Phase:** 4 — Integration Clients  
**Priority:** 🟡 Medium  
**Depends on:** TASK-02 (WebCredentialStore)  
**Solution:** SOL-04-Credential-Store.md  
**CRs:** CR-INT-003  
**Estimated effort:** ~90 phút (complex — Linear có token path phức tạp)

---

## Mục tiêu

Thay thế `safeStorage` (Electron-only) bằng `WebCredentialStore` khi chạy trong Web mode. Vì Linear và Jira dùng token file phức tạp hơn (không chỉ là 1 env var), cần sửa trực tiếp trong client code.

---

## TASK-10: `src/main/linear/client.ts`

### Hiện trạng

Linear lưu tokens tại:
- `~/.orca/linear-token.enc` (legacy single token)
- `~/.orca/linear-tokens/{workspaceId}.enc` (per-workspace tokens)

Sử dụng `safeStorage.encryptString()` / `safeStorage.decryptString()` để mã hóa.

### Approach: Dual-mode token storage

Thêm helper functions phát hiện web mode và điều hướng sang WebCredentialStore:

```typescript
// src/main/linear/client.ts
// THÊM imports:
import { isWebCredentialMode, getWebCredentialStore } from '../credentials'

// THÊM helper để đọc token per-workspace:
async function readWorkspaceToken(workspaceId: string): Promise<string | null> {
  if (isWebCredentialMode()) {
    const store = getWebCredentialStore()
    // Trong WebCredentialStore, lưu với key 'linear:{workspaceId}'
    // Nhưng CredentialService type hiện tại chỉ có 'linear'
    // → Cần mở rộng hoặc dùng config storage của WebCredentialStore:
    const token = await store.getToken('linear')
    // Nếu cần per-workspace: lưu workspaceId trong config file riêng
    // Hoặc dùng JSON multi-token: { [workspaceId]: encryptedToken }
    return token
  }

  // Electron mode: dùng existing file-based storage
  const tokenPath = getWorkspaceTokenPath(workspaceId)
  try {
    const raw = readFileSync(tokenPath)
    return readStoredCredentialToken('linear', raw)
  } catch {
    return null
  }
}

async function writeWorkspaceToken(workspaceId: string, token: string): Promise<void> {
  if (isWebCredentialMode()) {
    const store = getWebCredentialStore()
    await store.setToken('linear', token)
    return
  }

  // Electron mode: existing safeStorage path
  const tokenPath = getWorkspaceTokenPath(workspaceId)
  const toStore = safeStorage.isEncryptionAvailable()
    ? safeStorage.encryptString(token)
    : Buffer.from(token)
  writeFileSync(tokenPath, toStore, { mode: 0o600 })
}
```

### Lưu ý về multi-workspace Linear

Linear hỗ trợ nhiều workspace, mỗi workspace có token riêng.
Trong Web mode ban đầu: **chỉ hỗ trợ 1 Linear workspace (đơn giản nhất)**.

Nếu cần multi-workspace: mở rộng `CredentialService` type hoặc dùng config JSON:
```typescript
// store.setConfig('linear', { [workspaceId]: token })
// → Nhưng config không được mã hóa! Cần encrypted multi-key support
```

→ **Phase 1**: Chỉ hỗ trợ 1 Linear token trong Web mode.

---

## TASK-11: `src/main/jira/client.ts`

### Hiện trạng

Jira lưu per-site tokens tại: `~/.orca/jira-tokens/{base64(siteId)}.enc`

### Approach: Tương tự Linear

```typescript
// src/main/jira/client.ts
// THÊM imports:
import { isWebCredentialMode, getWebCredentialStore } from '../credentials'

// THÊM helper:
async function readJiraSiteToken(siteId: string): Promise<string | null> {
  if (isWebCredentialMode()) {
    // Dùng siteId làm suffix để phân biệt trong config
    const store = getWebCredentialStore()
    const config = await store.getConfig('jira')
    return config?.[`token_${siteId}`] ?? null
  }

  // Electron mode: existing path
  const tokenPath = getJiraSiteTokenPath(siteId)
  try {
    const raw = readFileSync(tokenPath)
    return readStoredCredentialToken('jira', raw)
  } catch {
    return null
  }
}

async function writeJiraSiteToken(siteId: string, token: string): Promise<void> {
  if (isWebCredentialMode()) {
    const store = getWebCredentialStore()
    // Lưu token vào config (chú ý: config file không mã hóa → chuyển sang encrypted multi-key)
    // Hoặc: lưu token vào service key 'jira' + encrypted config cho site metadata
    await store.setToken('jira', token, { activeSiteId: siteId })
    return
  }

  // Electron mode: existing code
}
```

### Lưu ý về multi-site Jira

Tương tự Linear, Jira hỗ trợ nhiều Atlassian sites.
→ **Phase 1**: Chỉ hỗ trợ 1 Jira site trong Web mode (active site).

---

## Mở rộng `WebCredentialStore` cho multi-service support (cần cho Phase 1.5)

Nếu cần lưu multiple tokens (multi-workspace Linear, multi-site Jira), cần mở rộng `CredentialService`:

```typescript
// Option A: Extend CredentialService type với dynamic keys
export type CredentialService =
  | 'bitbucket' | 'azure-devops' | 'gitea' | 'linear' | 'jira'
  | `linear:${string}`  // per-workspace
  | `jira:${string}`    // per-site
```

Hoặc:
```typescript
// Option B: Encrypted config JSON (cho Phase 2)
// store.setEncryptedConfig('linear', { workspaceId1: token1, workspaceId2: token2 })
```

---

## Acceptance Criteria

### TASK-10 (Linear)
1. `linear.connect(apiKey)` trong Web mode lưu token vào `WebCredentialStore`
2. `linear.disconnect()` xóa token khỏi `WebCredentialStore`
3. Restart Orca Server → token vẫn tồn tại (disk-persisted)
4. Không gọi `safeStorage` trong Web mode (không crash)

### TASK-11 (Jira)
1. Jira OAuth token được lưu vào `WebCredentialStore`
2. Jira site ID được lưu trong config cùng với token
3. Không gọi `safeStorage` trong Web mode

---

## Files cần sửa

- `src/main/linear/client.ts` — thêm web mode branch trong read/write token functions
- `src/main/jira/client.ts` — thêm web mode branch trong read/write site token functions
- `src/main/credentials/web-credential-store.ts` — có thể cần mở rộng nếu dùng dynamic keys
