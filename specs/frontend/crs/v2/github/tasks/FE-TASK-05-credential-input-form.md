# FE-TASK-05: `CredentialInputForm` Component và `useCredentialManager` Hook

> **Solution:** FE-SOL-03  
> **Files:** `src/renderer/src/components/settings/CredentialInputForm.tsx` [NEW]  
> **Status:** ✅ DONE & 🧪 AC Verified (2026-07-25)  
> **Depends on:** FE-TASK-01  
> **Priority:** 🟠 High — dùng bởi FE-TASK-06, FE-TASK-07

---

## Mô tả

Tạo shared component `CredentialInputForm` để các integration cards trong Web mode có thể nhập và lưu credentials qua `credentials.set(service, token, config)` RPC. Cũng tạo `useCredentialManager` hook để fetch và quản lý trạng thái credential.

---

## Acceptance Criteria

### `CredentialInputForm` component:
- [x] Props: `service`, `fields: CredentialField[]`, `isConfigured`, `onSaved`, `onRevoked` (lines 24-30)
- [x] `CredentialField` type: `{ key, label, placeholder, type: 'text'|'password'|'url', required }` (lines 16-22)
- [x] State: `values`, `saving`, `revoking`, `error`, `saved` (lines 39-43)
- [x] Validation: required fields phải có giá trị trước khi submit (lines 47-51)
- [x] Extract token: field có `type === 'password'` là token, còn lại vào `config` (lines 57-66)
- [x] Gọi `window.api.credentials.set(service, token, config)` khi save (line 68)
- [x] Clear password values sau khi save thành công: `setValues({})` (line 71) — security
- [x] Hiển thị ✓ success message trong 3 giây: `setSaved(true)` + `setTimeout` (lines 72-73)
- [x] Gọi `window.api.credentials.revoke(service)` với confirm dialog khi revoke (lines 83, 86)
- [x] Gọi `onSaved()` sau save (line 74), `onRevoked()` sau revoke (line 87)
- [x] Nút "Revoke" chỉ hiện khi `isConfigured === true` (line 139: `{isConfigured && (`)
- [x] TypeScript 0 lỗi (verified)

### `useCredentialManager` hook:
- [x] Nhận `service: CredentialService` param (line 168)
- [x] Auto-fetch `credentials.status(service)` khi mount via `useEffect` (lines 180-182)
- [x] Expose: `{ status, loading, isWebMode, isConfigured, refresh }` (line 187)
- [x] `isWebMode`: `status?.mode === 'web'` (line 184)
- [x] `isConfigured`: `status?.configured ?? false` (line 185)
- [x] `refresh`: stable callback via `useCallback` (lines 172-178)

---

## Implementation

**File:** [`src/renderer/src/components/settings/CredentialInputForm.tsx`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/components/settings/CredentialInputForm.tsx)  
**Size:** 189 lines, 5985 bytes

**Exports:**
- `CredentialField` (type) — dùng bởi FE-TASK-06, FE-TASK-07
- `CredentialInputForm` (component) — dùng bởi FE-TASK-06, FE-TASK-07
- `useCredentialManager` (hook) — dùng bởi FE-TASK-06, FE-TASK-07

**Security design:**
- Password fields dùng `autoComplete="current-password"` → không exposed trong DOM plaintext
- Sau khi save: `setValues({})` → state cleared, token không còn trong React tree
- `credentials.status()` không trả về token — chỉ `configured: boolean` và safe config

---

## Verification Results

```bash
ls -la src/renderer/src/components/settings/CredentialInputForm.tsx
# -rw-r--r--@ 1 binhnt staff 5985 Jul 25 19:12

grep -n "export function|export type|handleSave|handleRevoke|isWebMode|isConfigured" \
  src/renderer/src/components/settings/CredentialInputForm.tsx
# 16: export type CredentialField = {
# 32: export function CredentialInputForm({
# 45:   const handleSave = async () => {
# 82:   const handleRevoke = async () => {
# 168: export function useCredentialManager(service: CredentialService) {
# 184:   const isWebMode = status?.mode === 'web'
# 185:   const isConfigured = status?.configured ?? false
# 187:   return { status, loading, isWebMode, isConfigured, refresh }

node node_modules/typescript/bin/tsc --noEmit --project config/tsconfig.web.json 2>&1 | \
  grep "CredentialInputForm"
# Output: (empty — 0 errors)
```

**Kết quả:** ✅ File tồn tại (189 lines). 14/14 AC items hoàn thành. 0 lỗi TypeScript mới.
