# FE-TASK-02: Expose `github.*` và `gitlab.*` auth methods trong `web-preload-api.ts`

> **Solution:** FE-SOL-02  
> **File:** `src/renderer/src/web/web-preload-api.ts`  
> **Status:** ✅ DONE & 🧪 AC Verified (2026-07-25)  
> **Depends on:** FE-TASK-01  
> **Priority:** 🔴 Critical — blocker cho FE-TASK-03, FE-TASK-04

---

## Mô tả

`web-preload-api.ts` tạo `window.api` cho Web mode. Cần thêm `github` và `gitlab` namespaces với `startAuthLogin` và `revokeAuth` methods để renderer có thể spawn PTY trên Dev Server relay.

Backend RPC methods đã sẵn sàng: `github.startAuthLogin`, `github.revokeAuth`, `gitlab.startAuthLogin`, `gitlab.revokeAuth` (SOL-03).

---

## Acceptance Criteria

- [x] `window.api.github.startAuthLogin(devServerId, host?)` → gọi `callRuntimeResult('github.startAuthLogin', { devServerId, host? })`
- [x] `window.api.github.revokeAuth(devServerId, host?)` → gọi `callRuntimeResult('github.revokeAuth', { devServerId, host? })`
- [x] `window.api.gitlab.startAuthLogin(devServerId, host?)` → tương tự với `gitlab.startAuthLogin`
- [x] `window.api.gitlab.revokeAuth(devServerId, host?)` → tương tự với `gitlab.revokeAuth`
- [x] Return type: `Promise<{ ptyId: string; devServerId: string }>` cho mọi method
- [x] TypeScript 0 lỗi mới

---

## Implementation

**File:** [`src/renderer/src/web/web-preload-api.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/web/web-preload-api.ts)  
**Vị trí:** Lines 747–776 — sau `credentials` namespace, trước `wsl`

```typescript
// Why: CLI auth login methods for Web Server mode (FE-SOL-02 / SOL-03-Remote-PTY).
// Spawns `gh auth login` / `gh auth logout` as a PTY on the Dev Server relay.
// Returns { ptyId, devServerId } — the caller subscribes to the PTY stream for output.
github: {
  startAuthLogin: (devServerId: string, host?: string) =>
    callRuntimeResult<{ ptyId: string; devServerId: string }>(
      'github.startAuthLogin',
      { devServerId, ...(host ? { host } : {}) }
    ),
  revokeAuth: (devServerId: string, host?: string) =>
    callRuntimeResult<{ ptyId: string; devServerId: string }>(
      'github.revokeAuth',
      { devServerId, ...(host ? { host } : {}) }
    ),
},
// Why: CLI auth methods for GitLab (glab CLI) in Web Server mode (FE-SOL-02).
gitlab: {
  startAuthLogin: (devServerId: string, host?: string) =>
    callRuntimeResult<{ ptyId: string; devServerId: string }>(
      'gitlab.startAuthLogin',
      { devServerId, ...(host ? { host } : {}) }
    ),
  revokeAuth: (devServerId: string, host?: string) =>
    callRuntimeResult<{ ptyId: string; devServerId: string }>(
      'gitlab.revokeAuth',
      { devServerId, ...(host ? { host } : {}) }
    ),
},
```

---

## Verification Results

```bash
grep -n "startAuthLogin|revokeAuth" src/renderer/src/web/web-preload-api.ts
# Output (8 lines):
# 748:      startAuthLogin: (devServerId: string, host?: string) =>
# 750:          'github.startAuthLogin',
# 753:      revokeAuth: (devServerId: string, host?: string) =>
# 755:          'github.revokeAuth',
# 761:      startAuthLogin: (devServerId: string, host?: string) =>
# 763:          'gitlab.startAuthLogin',
# 766:      revokeAuth: (devServerId: string, host?: string) =>
# 768:          'gitlab.revokeAuth',
```

**Kết quả:** ✅ 4 methods (2 github + 2 gitlab) đã được expose đúng. 0 lỗi TypeScript mới.
