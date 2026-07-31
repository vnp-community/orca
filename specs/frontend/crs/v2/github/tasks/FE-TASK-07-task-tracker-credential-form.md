# FE-TASK-07: Linear / Jira Cards — Web Mode Credential Form

> **Solution:** FE-SOL-03  
> **Files:** `src/renderer/src/components/settings/task-tracker-integration-cards.tsx`, `src/renderer/src/components/settings/jira-integration-card.tsx`  
> **Status:** ✅ DONE & 🧪 AC Verified (2026-07-25)  
> **Depends on:** FE-TASK-05  
> **Priority:** 🟠 High

---

## Acceptance Criteria

### `LinearIntegrationCard` (trong `task-tracker-integration-cards.tsx`):
- [x] Import `useCredentialManager` và `CredentialInputForm` từ `./CredentialInputForm`
- [x] Gọi `useCredentialManager('linear')` → `{ isWebMode, isConfigured, refresh: credRefresh }`
- [x] Khi `!connected && !checking && isWebMode`: render form với 1 field `lin_api_...` token
- [x] `onSaved` + `onRevoked`: `credRefresh()` + `checkLinearConnection(true)`
- [x] Electron mode: giữ nguyên text hướng dẫn + Re-check button

### `JiraIntegrationCard` (trong `jira-integration-card.tsx`):
- [x] Import `useCredentialManager` và `CredentialInputForm`
- [x] Gọi `useCredentialManager('jira')` → `{ isWebMode, isConfigured, refresh: credRefresh }`
- [x] Khi `!connected && !checking && isWebMode`: render form với 3 fields (token, email, apiBaseUrl)
- [x] `service="jira"`, `isConfigured={isConfigured}`
- [x] `onSaved` + `onRevoked`: `credRefresh()` + `checkJiraConnection()`
- [x] Electron mode: giữ nguyên `credentialCopy` text + Re-check button

### Chung:
- [x] Electron mode behavior giữ nguyên 100%
- [x] TypeScript 0 lỗi mới
- [x] Token fields cleared sau save (security — handled by CredentialInputForm)

---

## Implementation

### [`task-tracker-integration-cards.tsx`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/components/settings/task-tracker-integration-cards.tsx)
- Import thêm `useCredentialManager`, `CredentialInputForm` (line 17)
- Thêm `useCredentialManager('linear')` hook call (line 42)
- `!checking` section: `isWebMode ? <CredentialInputForm ... /> : <ElectronMode />`

### [`jira-integration-card.tsx`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/components/settings/jira-integration-card.tsx)
- Import thêm `useCredentialManager`, `CredentialInputForm` (line 18)
- Thêm `useCredentialManager('jira')` hook call (line 45)
- `!checking` section: `isWebMode ? <CredentialInputForm ... /> : <ElectronMode />`

---

## Credential Fields

| Service | Fields |
|---------|--------|
| Linear | `token` (password, required: lin_api_...) |
| Jira | `token` (password, required), `email` (text, required), `apiBaseUrl` (url, required: https://yourorg.atlassian.net) |

---

## Verification Results

```bash
grep -n "useCredentialManager\|CredentialInputForm" \
  src/renderer/src/components/settings/task-tracker-integration-cards.tsx \
  src/renderer/src/components/settings/jira-integration-card.tsx
# Expected: 2 imports + 2 hook calls + 2 form renders

node node_modules/typescript/bin/tsc --noEmit --project config/tsconfig.web.json 2>&1 | \
  grep "task-tracker\|jira-integration"
# Output: (empty — 0 errors)
```

**Kết quả:** ✅ LinearIntegrationCard + JiraIntegrationCard đều có web mode CredentialInputForm. Electron mode không đổi. 0 lỗi TypeScript mới.
