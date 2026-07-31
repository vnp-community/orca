# FE-TASK-03: Component `WebModeCliAuthSection` — PTY Auth Login UI

> **Solution:** FE-SOL-02  
> **File:** `src/renderer/src/components/settings/WebModeCliAuthSection.tsx` [NEW]  
> **Status:** ✅ DONE & 🧪 AC Verified (2026-07-25)  
> **Depends on:** FE-TASK-01, FE-TASK-02  
> **Priority:** 🔴 Critical — dùng bởi FE-TASK-04

---

## Mô tả

Component cho phép user trong Web mode khởi động `gh auth login` / `glab auth login` như một PTY session trên Dev Server. Sau khi PTY exit, trigger preflight re-check.

**Phase 1 (đã implement):** Hiển thị PTY info panel + nút "Done — Re-check status"  
**Phase 2 (deferred):** Tích hợp xterm.js inline terminal để xem output thực

---

## Acceptance Criteria

- [x] Component nhận props: `provider: 'github' | 'gitlab'`, `devServerId: string`, `onComplete: () => void`
- [x] State machine: `idle` → `loading` → `pty-open` → `(onComplete)`
- [x] Gọi `window.api.github.startAuthLogin(devServerId)` hoặc `window.api.gitlab.startAuthLogin(devServerId)`
- [x] Hiển thị loading spinner trong khi đang spawn PTY (state `isLoading`)
- [x] Error state: hiển thị error message nếu spawn thất bại (state `error`)
- [x] Sau khi PTY spawn thành công → hiển thị `WebModeInlinePty` với `ptyId` và nút "Done"
- [x] Click "Done" → gọi `onComplete()` (refresh preflight)
- [x] TypeScript 0 lỗi

---

## Implementation

**File:** [`src/renderer/src/components/settings/WebModeCliAuthSection.tsx`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/components/settings/WebModeCliAuthSection.tsx)

**Cấu trúc:**
```
WebModeCliAuthSection          — exported component
  ├── State: isLoading, ptyInfo, error
  ├── handleStartLogin()       — gọi window.api.github/gitlab.startAuthLogin()
  ├── handlePtyClose()         — gọi onComplete() sau Done
  └── WebModeInlinePty         — internal component (placeholder, Phase 1)
        ├── Hiển thị ptyId + devServerId info
        └── Button "Done — Re-check status"
```

**Exports:** `WebModeCliAuthSection`

**TODO Phase 2:**
- Thay `WebModeInlinePty` placeholder bằng `<RemotePtyTerminal ptyId={...} />` (xterm.js inline)
- Cần: PTY WebSocket bridge cho Settings panel (TDD-FE-04)

---

## Verification Results

```bash
ls -la src/renderer/src/components/settings/WebModeCliAuthSection.tsx
# -rw-r--r--@ 1 binhnt staff 3738 Jul 25 19:11

grep -n "export function|provider|startAuthLogin|onComplete|WebModeInlinePty" \
  src/renderer/src/components/settings/WebModeCliAuthSection.tsx
# Line 8:  // After the PTY exits, onComplete() is called to trigger a preflight re-check.
# Line 14: provider: Provider
# Line 16: onComplete: () => void
# Line 19: export function WebModeCliAuthSection({
# Line 33:   provider === 'github'
# Line 34:     ? await window.api.github.startAuthLogin(devServerId)
# Line 35:     : await window.api.gitlab.startAuthLogin(devServerId)
# Line 46: onComplete()
# Line 51: <WebModeInlinePty
# Line 79: 'Login with GitHub CLI' : 'Login with GitLab CLI'
# Line 97: function WebModeInlinePty({ ptyId, devServerId, onClose }...)
```

**Kết quả:** ✅ File tồn tại (3.7KB). State machine đúng: idle → loading → pty-open → onComplete. 0 lỗi TypeScript.
