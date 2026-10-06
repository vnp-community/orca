# FE-CV-TASK-056-03: Mô hình bước và lớp phủ (`data-flow-overlay`)

**From Solution:** [FE-CV-SOL-056-dataflow-lens](../solutions/FE-CV-SOL-056-dataflow-lens.md) mục 4.4
**Priority:** P1
**Area:** frontend / review-map
**File:** `components/review-map/data-flow-overlay.ts` (mới), test
**Depends on:** FE-CV-TASK-053-01
**Status:** [ ] TODO

## Context

- `DataFlow.steps`, `stores`, `gaps`, `completeness`; `ChangeOverlay.changedSymbols`, `touchedTables`, `uncoveredSymbols`.

## Việc cần làm

1. `buildStepRows(flow, overlay, renderedMessages)`: hàng bước (+ hàng `gap` tại `afterStep`), cờ `changed` (theo `symbol.key` hoặc bảng thuộc `touchedTables`), `untested`, `unimplemented`, `inDiagram`.
2. `changedMessageSet(flow, overlay)` cho tiền tố `[đổi]` (khớp `messages[].n` = `steps[].n`).
3. `flowTouchesChange(summary, overlay)` giao `id` với `affectedFlows`; trạng thái `unknown` khi không có giao nào.

## Kiểm thử

- Đổi theo symbol/bảng; gap; bước cắt; `unknown`.

## Tiêu chí hoàn thành

- [ ] Thuần; test xanh.

## Rủi ro

- Giả định `messages.n = steps.n` (SOL câu hỏi 2).
