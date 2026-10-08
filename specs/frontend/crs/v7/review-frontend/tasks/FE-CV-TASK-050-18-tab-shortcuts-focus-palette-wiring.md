# FE-CV-TASK-050-18: Chuyển tab, tiêu điểm, palette, zoom; loại khỏi sync mobile

**From Solution:** [FE-CV-SOL-050-review-tab-wiring](../solutions/FE-CV-SOL-050-review-tab-wiring.md) mục 4.2
**Priority:** P1
**Area:** frontend / lib + hooks + components
**File:** `lib/tab-number-shortcuts.ts`, `components/terminal/tab-type-cycle.ts`, `components/Terminal.tsx`, `hooks/ipc-tab-switch.ts`, `hooks/resolve-zoom-target.ts`, `hooks/modal-return-focus-action.ts`, `hooks/useModalReturnFocus.ts`, `lib/workspace-tab-palette-search.ts`, `components/WorktreeJumpPalette.tsx`, `components/floating-terminal/FloatingTerminalPanel.tsx` (đọc/xác nhận), `runtime/sync-runtime-graph.ts` (đọc), tests
**Depends on:** FE-CV-TASK-050-17
**Status:** [~] PARTIAL — tab cycle, number shortcut, zoom (ui), modal focus return, Terminal.tsx hide, mobile sync exclusion (read-only check) done; palette search for the Review tab (workspace-tab-palette-search results) NOT implemented, only types widened; no per-file test cases for tab-number-shortcuts/ipc-tab-switch

## Context

- Mỗi tệp có literal `'simulator'` (xem SOL mục 1). Phím: không thêm vào `KEYBINDING_DEFINITIONS`; chuyển tab số/vòng theo vị trí; nếu cần phím có phím bổ trợ thì `metaKey` Mac, `ctrlKey` nơi khác (AGENTS.md).

## Việc cần làm

1. Mở rộng union `activeTabType` ở các hook/palette; `review` ⇒ zoom target `'ui'`, trả tiêu điểm vào tab, bỏ qua lớp phủ terminal.
2. `workspace-tab-palette-search`/`WorktreeJumpPalette`: tìm được tab Review (nhãn `translate()`).
3. Xác nhận bằng đọc: `FloatingTerminalPanel` không bị vỡ; `sync-runtime-graph.ts` (`isEditorSurfaceTab`) **không** đưa review sang mobile; ghi vào PR.
4. Không sửa các tệp chỉ liên quan simulator (xem bảng "KHÔNG").

## Kiểm thử

- Mở rộng test hiện có của từng tệp; thêm ca `review` cho `tab-number-shortcuts`, `tab-type-cycle`, `resolve-zoom-target`, `modal-return-focus-action`.

## Tiêu chí hoàn thành

- [ ] `tsc` và `lint:switch-exhaustiveness` sạch; tab Review không xuất hiện trong đồ thị đồng bộ mobile.

## Rủi ro

- `Terminal.tsx` (:2020-2128 theo CR) chưa đọc kỹ.
