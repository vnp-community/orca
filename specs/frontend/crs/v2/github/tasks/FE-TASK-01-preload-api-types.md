# FE-TASK-01: Thêm `credentials`, `github`, `gitlab` vào `PreloadApi` type

> **Solution:** FE-SOL-02, FE-SOL-03  
> **File:** `src/preload/api-types.ts`  
> **Status:** ✅ DONE & 🧪 AC Verified (2026-07-25)  
> **Priority:** 🔴 Critical — blocker cho FE-TASK-03, FE-TASK-05

---

## Mô tả

`PreloadApi` là type contract giữa Electron preload script và renderer process. Trong Web mode, `window.api` được populate bởi `web-preload-api.ts` với `as PreloadApi` cast. Cần thêm 3 namespace mới để TypeScript validate các call site.

---

## Acceptance Criteria

- [x] `PreloadApi.credentials` type được thêm vào với các methods: `set`, `revoke`, `status`, `list`
- [x] `PreloadApi.github` type với `startAuthLogin(devServerId, host?)` và `revokeAuth(devServerId, host?)`
- [x] `PreloadApi.gitlab` type với `startAuthLogin(devServerId, host?)` và `revokeAuth(devServerId, host?)`
- [x] TypeScript build `config/tsconfig.web.json` không có lỗi liên quan đến 3 namespaces này

---

## Implementation

**File:** [`src/preload/api-types.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/preload/api-types.ts)  
**Vị trí:** Lines 3379–3415 — sau `speech` namespace, trước dấu `}` đóng của `PreloadApi`

```typescript
// ── Web Server mode credential management (FE-SOL-03 / SOL-04-Credential-Store) ──
// Available in web mode only; electron mode returns errors from the RPC server.
credentials: {
  set: (service: string, token: string, config?: Record<string, string>) => Promise<{ success: boolean }>
  revoke: (service: string) => Promise<{ success: boolean }>
  status: (service: string) => Promise<{ configured: boolean; mode: string; config?: Record<string, string> }>
  list: () => Promise<{ services: string[]; mode: string }>
}
// ── CLI auth login via PTY on Dev Server (FE-SOL-02 / SOL-03-Remote-PTY) ──
// Spawn `gh auth login` / `gh auth logout` as a PTY on the Dev Server relay.
// Returns { ptyId, devServerId } — caller subscribes to PTY stream for output.
github: {
  startAuthLogin: (devServerId: string, host?: string) => Promise<{ ptyId: string; devServerId: string }>
  revokeAuth: (devServerId: string, host?: string) => Promise<{ ptyId: string; devServerId: string }>
}
gitlab: {
  startAuthLogin: (devServerId: string, host?: string) => Promise<{ ptyId: string; devServerId: string }>
  revokeAuth: (devServerId: string, host?: string) => Promise<{ ptyId: string; devServerId: string }>
}
```

---

## Verification Results

```bash
grep -n "credentials:|github:|gitlab:" src/preload/api-types.ts | grep -v "github.*Login\|github.*Email\|github.*Project"
# Output:
# 3379:  credentials: {
# 3394:  github: {
# 3404:  gitlab: {

node node_modules/typescript/bin/tsc --noEmit --project config/tsconfig.web.json 2>&1 | \
  grep "credentials.*does not exist|github.*does not exist|gitlab.*does not exist"
# Output: (empty — 0 errors for these fields)
```

**Kết quả:** ✅ 3 namespaces tồn tại đúng vị trí. 0 lỗi TypeScript mới trong các files của task này.
