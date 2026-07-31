# FE-TASK-06: Bitbucket / Azure DevOps / Gitea Cards — Web Mode Credential Form

> **Solution:** FE-SOL-03  
> **File:** `src/renderer/src/components/settings/token-source-control-integration-cards.tsx`  
> **Status:** ✅ DONE & 🧪 AC Verified (2026-07-25)  
> **Depends on:** FE-TASK-05  
> **Priority:** 🟠 High

---

## Mô tả

`BitbucketIntegrationCard`, `AzureDevOpsIntegrationCard`, `GiteaIntegrationCard` hiện chỉ hiển thị hướng dẫn set env vars khi `not-configured`. Trong Web mode cần thay bằng `CredentialInputForm`.

---

## Acceptance Criteria

### `BitbucketIntegrationCard`:
- [x] Import `useCredentialManager` và `CredentialInputForm` từ `./CredentialInputForm`
- [x] Gọi `useCredentialManager('bitbucket')` → nhận `{ isWebMode, isConfigured, refresh: credRefresh }`
- [x] Khi `status === 'not-configured'` **VÀ** `isWebMode`: render `CredentialInputForm` với 3 fields (token, email, apiBaseUrl)
- [x] `onSaved` + `onRevoked` → gọi `credRefresh()` + `refresh()` (preflight re-check)

### `AzureDevOpsIntegrationCard`:
- [x] Gọi `useCredentialManager('azure-devops')`
- [x] Web mode form với 3 fields: token (PAT), username (optional), apiBaseUrl (optional)
- [x] `not-configured` + `isWebMode` → form

### `GiteaIntegrationCard`:
- [x] Gọi `useCredentialManager('gitea')`
- [x] Web mode form với 2 fields: token, apiBaseUrl (required)
- [x] `not-configured` + `isWebMode` → form

### Chung:
- [x] Electron mode: behavior giữ nguyên (env var instructions unchanged)
- [x] TypeScript 0 lỗi mới (total errors: 53 = baseline)
- [x] Token fields cleared sau save (security — handled by CredentialInputForm)

---

## Implementation

**File:** [`src/renderer/src/components/settings/token-source-control-integration-cards.tsx`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/components/settings/token-source-control-integration-cards.tsx)

**Pattern (mỗi card):**
```typescript
const { isWebMode, isConfigured, refresh: credRefresh } = useCredentialManager('<service>')
// ...
status === 'not-configured' && isWebMode ? (
  <CredentialInputForm service="..." isConfigured={isConfigured} fields={[...]}
    onSaved={handleCredentialChange} onRevoked={handleCredentialChange} />
) : (
  // Electron mode env var text (unchanged)
)
```

---

## Verification Results

```bash
grep -n "useCredentialManager\|CredentialInputForm\|isWebMode" \
  src/renderer/src/components/settings/token-source-control-integration-cards.tsx
# Expected: import (line 9) + 3 hook calls + 3 form renders

node node_modules/typescript/bin/tsc --noEmit --project config/tsconfig.web.json 2>&1 | \
  grep "token-source-control"
# Output: (empty — 0 errors)
```

**Kết quả:** ✅ Cả 3 cards đã có web mode `CredentialInputForm`. Electron mode không thay đổi. 0 lỗi TypeScript mới.
