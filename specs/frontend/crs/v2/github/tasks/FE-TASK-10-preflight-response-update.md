# FE-TASK-10: Cập nhật `remotePreflightByServer` khi nhận Preflight Response

> **Solution:** FE-SOL-04  
> **File:** `src/renderer/src/store/slices/preflight.ts`  
> **Status:** ✅ DONE & 🧪 AC Verified (2026-07-25)  
> **Depends on:** FE-TASK-08  
> **Priority:** 🟡 Medium

---

## Acceptance Criteria

- [x] Trong `.then()` callback của `refreshPreflightStatus`:
  - Đọc `currentDevServerId = get().activeDevServerId`
  - Kiểm tra `runtimeTarget.kind === 'environment' && currentDevServerId`
  - Gọi `get().setRemotePreflightStatus(currentDevServerId, { ... })`
- [x] `adaptedRemoteStatus` có các fields:
  - `devServerId: currentDevServerId`
  - `platform: process.platform` (PreflightStatus không có platform field)
  - `checkedAt: Date.now()`
  - `gh: status.gh ?? { installed: false, authenticated: false }`
  - `glab: status.glab` (optional — pass-through)
  - `git: { installed: status.git.installed, hasUserName: false, hasUserEmail: false }`
- [x] Khi `currentDevServerId === null`: `setRemotePreflightStatus` không được gọi
- [x] `preflightStatus` vẫn được update bình thường sau khi relay cache
- [x] TypeScript 0 lỗi mới (total: 53 = baseline)

---

## Implementation

**File:** [`src/renderer/src/store/slices/preflight.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/store/slices/preflight.ts)  
**Vị trí:** Lines 121-142 — trong `.then()` callback trước `set({ preflightStatus: ... })`

```typescript
// Why: When the result came from a Dev Server relay (Web mode + devServerId),
// cache it in remotePreflightByServer so CLI integration cards can distinguish
// between Dev Server status and Orca Server status (FE-TASK-10 / FE-SOL-04).
const currentDevServerId = get().activeDevServerId
if (runtimeTarget.kind === 'environment' && currentDevServerId) {
  get().setRemotePreflightStatus(currentDevServerId, {
    devServerId: currentDevServerId,
    // Why: PreflightStatus has no platform field; use process.platform as fallback.
    platform: process.platform,
    checkedAt: Date.now(),
    gh: status.gh ?? { installed: false, authenticated: false },
    glab: status.glab, // optional — relayed from Dev Server (FE-TASK-08)
    // Why: PreflightStatus.git only has installed; default hasUserName/hasUserEmail false.
    git: {
      installed: status.git.installed,
      hasUserName: false,
      hasUserEmail: false,
    },
  })
}
```

## Type Note

`PreflightStatus.git = { installed: boolean }` — không có `hasUserName`/`hasUserEmail`.  
`RemotePreflightStatus.git` yêu cầu cả 3 — default false được dùng vì:
1. `preflight.check` chưa expose git user config
2. Dev Server relay có thể cung cấp full git status trong tương lai

---

## Verification Results

```bash
grep -n "setRemotePreflightStatus\|currentDevServerId\|checkedAt" \
  src/renderer/src/store/slices/preflight.ts
# Expected: setRemotePreflightStatus được gọi trong .then() block (line ~127)

node node_modules/typescript/bin/tsc --noEmit --project config/tsconfig.web.json 2>&1 | \
  grep "preflight.ts("
# Output: (empty — 0 errors in production preflight.ts)

# Total errors:
node node_modules/typescript/bin/tsc --noEmit --project config/tsconfig.web.json 2>&1 | grep "error TS" | wc -l
# Output: 53 (= baseline, 0 new errors added)
```

**Kết quả:** ✅ `setRemotePreflightStatus` được gọi sau mỗi relay preflight response. Type errors đã được fix (git shape mismatch). 0 lỗi TypeScript mới.
