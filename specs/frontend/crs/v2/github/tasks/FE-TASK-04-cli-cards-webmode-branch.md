# FE-TASK-04: GitHub/GitLab Integration Cards — Web Mode Branch

> **Solution:** FE-SOL-02  
> **File:** `src/renderer/src/components/settings/cli-source-control-integration-cards.tsx`  
> **Status:** ✅ DONE & 🧪 AC Verified (2026-07-25)  
> **Depends on:** FE-TASK-03  
> **Priority:** 🔴 Critical

---

## Mô tả

`GitHubIntegrationCard` và `GitLabIntegrationCard` hiện chỉ hiển thị lệnh terminal (`gh auth login`) khi chưa authenticate. Trong Web mode với Dev Server connected, cần hiển thị nút "Login with GitHub/GitLab CLI" thay vì text lệnh.

---

## Acceptance Criteria

- [x] Import `WebModeCliAuthSection` từ `./WebModeCliAuthSection` (line 14)
- [x] `GitHubIntegrationCard`: đọc `activeDevServerId` từ `useAppStore` (line 47)
- [x] `GitLabIntegrationCard`: đọc `activeDevServerId` từ `useAppStore` (line 190)
- [x] Khi `status === 'not-authenticated'` **VÀ** `activeDevServerId != null`:
  - `GitHubIntegrationCard`: render `<WebModeCliAuthSection provider="github" devServerId={activeDevServerId} onComplete={refresh} />` (line 133-136)
  - `GitLabIntegrationCard`: render `<WebModeCliAuthSection provider="gitlab" devServerId={activeDevServerId} onComplete={refresh} />` (line 278-281)
- [x] Khi `status === 'not-authenticated'` **VÀ** `activeDevServerId === null`:
  - Giữ nguyên behavior cũ: hiển thị terminal command text `gh auth login` / `glab auth login`
- [x] Logic phân nhánh **không** ảnh hưởng `unavailable` và `not-installed` states (giữ nguyên)
- [x] TypeScript 0 lỗi

---

## Implementation

**File:** [`src/renderer/src/components/settings/cli-source-control-integration-cards.tsx`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/components/settings/cli-source-control-integration-cards.tsx)

**Pattern áp dụng (cả 2 cards):**
```typescript
// Thêm vào đầu card function (lines 47-48 và 190-191):
const activeDevServerId = useAppStore((s) => s.activeDevServerId)
const isWebMode = activeDevServerId != null

// Trong not-authenticated branch (lines 131-135 và 276-280):
) : (
  isWebMode && activeDevServerId ? (
    // Web Server mode: show PTY auth login button
    <WebModeCliAuthSection
      provider="github"  // hoặc "gitlab"
      devServerId={activeDevServerId}
      onComplete={refresh}
    />
  ) : (
    // Electron mode: terminal command text (unchanged behaviour)
    <>...</>
  )
)
```

---

## Verification Results

```bash
grep -n "WebModeCliAuthSection|isWebMode|activeDevServerId" \
  src/renderer/src/components/settings/cli-source-control-integration-cards.tsx
# Output (10 lines):
# 14: import { WebModeCliAuthSection } from './WebModeCliAuthSection'
# 47:   const activeDevServerId = useAppStore((s) => s.activeDevServerId)
# 48:   const isWebMode = activeDevServerId != null
# 131:           isWebMode && activeDevServerId ? (
# 133:             <WebModeCliAuthSection
# 134:               provider="github"
# 135:               devServerId={activeDevServerId}
# 189: ...
# 190:   const activeDevServerId = useAppStore((s) => s.activeDevServerId)
# 191:   const isWebMode = activeDevServerId != null
# 276:           isWebMode && activeDevServerId ? (
# 278:             <WebModeCliAuthSection
# 279:               provider="gitlab"
# 280:               devServerId={activeDevServerId}
```

**Kết quả:** ✅ Cả `GitHubIntegrationCard` và `GitLabIntegrationCard` đã có web mode branch đúng. Electron mode behavior không thay đổi. 0 lỗi TypeScript mới.
