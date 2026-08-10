# BUG-FE-BIGFILE-002 — `orca-runtime.ts` (26,730 dòng) — file lớn nhất `frontend/src`

**Mức độ:** 🔴 Critical
**Status:** 🔴 Open
**Solution:** [SOLUTION-FE-BIGFILE-002](./solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md)
**Module:** `frontend/src/main/runtime/orca-runtime.ts`
**Phát hiện:** 2026-08-10, `scripts/find-frontend-bigfiles.mjs` — xem tổng quan
tại `BUG-FE-BIGFILE-001`

---

## Mô tả

26,730 dòng — gần **90 lần** ngưỡng oxlint mặc định cho `.ts` (300 dòng). Đây là
file lớn nhất toàn bộ `frontend/src`, gấp đôi file lớn thứ 2
(`TaskPage.tsx`, 12,833 dòng).

Comment giải thích ngay dòng 1 (lý do disable `max-lines` hiện tại):

```
/* eslint-disable max-lines -- Why: OrcaRuntimeService still owns the mutable
live graph, PTY handles, waiters, mobile floor/layout state, and
managed-worktree reconciliation. Stateless browser and file command adapters
live beside it; the remaining split points need state-owner extraction before
enforcing max-lines. */
```

Class `OrcaRuntimeService` bắt đầu ở dòng 2,109 — chiếm phần lớn phần còn lại
của file (~24,600 dòng cho 1 class). Trước class có ~2,000 dòng type
definitions (`RemoteFetchResult`, `AccountsSnapshot`, `RuntimeAutomationCreateInput`,
`RuntimeTerminalAgentStatusEvent`, `RuntimePtyController`,
`MobileNotificationDispatchEvent`, `PtyLayoutState`, ...). Sau class (từ dòng
~24,786) là ~1,900 dòng pure helper function (`appendRecentPtyOutput`,
`computeTerminalTailWaitState`, `appendNormalizedToTailBuffer`, ...) — các hàm
này KHÔNG phụ thuộc `this`, đã là ứng viên tách file rõ ràng nhất.

Đã có tiền lệ tách một phần: `orca-runtime-files.ts` (1,885 dòng, #50 trong
bảng tổng) và `orca-runtime-browser.ts` (1,841 dòng, #55) đã được tách RA khỏi
file này trước đây — đúng như comment "Stateless browser and file command
adapters live beside it" mô tả.

## Hậu quả

- File này là trung tâm của gần như mọi flow terminal/PTY — chính investigation
  BUG-FE-PTY-001 (xem `specs/frontend/bugs/terminal-management./` và memory
  session `bug-fe-pty-001-investigation.md`) phải liên tục đối chiếu ngược lại
  các type/method trong file này (`RuntimePtyController`, `closeTerminal`,
  `stopExactTerminalsForWorktree`, ...) trong lúc điều tra 1 bug không liên
  quan trực tiếp đến phần lớn nội dung file.
- 1 class 24,600 dòng gần như chắc chắn có nhiều trách nhiệm chồng chéo
  (terminal lifecycle, PTY liveness, mobile session mirror, worktree
  reconciliation, automation) — vi phạm Single Responsibility ở quy mô cực
  đoan.

## Bằng chứng

```
wc -l frontend/src/main/runtime/orca-runtime.ts        → 26730
grep -n "^export class OrcaRuntimeService" ...ts        → dòng 2109
grep -n "^export function" ...ts (sau dòng 24786)       → ~15 pure helper functions
```

## Đề xuất fix (gợi ý điểm tách, không bắt buộc theo đúng thứ tự)

1. **Tách trước, rủi ro thấp nhất**: ~1,900 dòng pure helper function sau class
   (dòng 24,786 → cuối file: `appendRecentPtyOutput`,
   `appendRecentPtyPathCandidates`, `recentTerminalPathCandidatesIncludePath`,
   `recentTerminalOutputIncludesPath`, `buildPreview`,
   `computeTerminalTailWaitState`, `tailGainedNewerBlockedReason`,
   `appendNormalizedToTailBuffer`,
   `appendNormalizedToMultilineTailBufferUnwindowed`, ...) — không phụ thuộc
   `this`, có thể chuyển sang `orca-runtime-tail-buffer.ts` (hoặc tên tương tự)
   mà không đổi hành vi.
2. **Tách type definitions** (dòng ~773–2,108, trước class) sang
   `orca-runtime-types.ts` — đã là pattern quen thuộc trong repo (nhiều domain
   khác đã có file `*-types.ts` riêng).
3. **Trong class `OrcaRuntimeService`**: theo đúng hướng comment dòng 1 đã gợi
   ý — tách tiếp theo domain trách nhiệm ra các "state owner" riêng, tương tự
   `orca-runtime-files.ts`/`orca-runtime-browser.ts` đã làm:
   - Mobile session mirror / floor-layout state (`MobileNotificationDispatchEvent`,
     `PtyLayoutState`, `ApplyLayoutResult`) → có thể tách
     `orca-runtime-mobile-session.ts`.
   - Automation (`RuntimeAutomationCreateInput`/`UpdateInput`) → có thể tách
     `orca-runtime-automation.ts`.
   - Worktree reconciliation logic → ứng viên tách tiếp theo sau khi 2 domain
     trên đã tách xong (giảm kích thước class đủ để nhìn rõ ranh giới còn lại).

## Tham khảo

- Tổng quan: `BUG-FE-BIGFILE-001`
- File liên quan đã tách trước đây: `orca-runtime-files.ts`,
  `orca-runtime-browser.ts` (cùng thư mục)
- Chính sách: `AGENTS.md` → "Lint Rules: Do Not Disable Max Lines"
