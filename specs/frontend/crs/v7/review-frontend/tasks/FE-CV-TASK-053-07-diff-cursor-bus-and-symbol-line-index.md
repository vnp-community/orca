# FE-CV-TASK-053-07: Diff → đồ thị: `diff-cursor-line-bus`, `symbol-line-index`

**From Solution:** [FE-CV-SOL-053-impact-lens-and-symbol-detail](../solutions/FE-CV-SOL-053-impact-lens-and-symbol-detail.md) mục 4.5
**Priority:** P1
**Area:** frontend / lib + editor + review-map
**File:** `lib/diff-cursor-line-bus.ts`, `components/editor/useDiffCursorEmitter.ts`, `components/review-map/symbol-line-index.ts` (mới), `DiffViewer.tsx` (gắn emitter), `ReviewWorkspace.tsx` (đăng ký), tests
**Depends on:** FE-CV-TASK-053-05, 053-06, 051-05
**Status:** [ ] TODO

## Context

- Mẫu bus: `lib/mcp-event-bus.ts`. Dòng 1-based (PQ-20): không cộng/trừ.

## Việc cần làm

1. Bus: `subscribeDiffCursorLine`, `emitDiffCursorLine`, `hasDiffCursorListeners`; `useDiffCursorEmitter` đăng ký `onDidChangeCursorPosition` **chỉ khi** có listener, debounce 150 ms.
2. `findInnermostSymbolAtLine(index, relativePath, line)`: chỉ mục khoảng theo file từ `changedSymbols` và nút `impact`; chọn khoảng hẹp nhất.
3. `ReviewWorkspace` đăng ký khi mount: đặt `selectedSymbolKey` nguồn `'diff'` (lens làm nổi + đưa vào khung nhìn, **không** mở drawer, không giành tiêu điểm); không có symbol ⇒ giữ nguyên; `suppressDiffCursorUntil` 500 ms sau reveal.

## Kiểm thử

- Không listener ⇒ không phát/không timer; debounce; suppress; symbol hẹp nhất; nhiều file.

## Tiêu chí hoàn thành

- [ ] Không vòng lặp reveal ↔ con trỏ.

## Rủi ro

- Đường dẫn tương đối vs tuyệt đối khi so khớp: chuẩn hoá bằng `normalizeRuntimePathSeparators`.
