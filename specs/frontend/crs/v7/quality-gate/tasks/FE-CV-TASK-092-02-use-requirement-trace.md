# FE-CV-TASK-092-02: Hook `useRequirementTrace` (đọc)

**From Solution:** [FE-CV-SOL-092-requirement-trace-view](../solutions/FE-CV-SOL-092-requirement-trace-view.md) mục 2.3
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/requirements/use-requirement-trace.ts` (mới) + test
**Depends on:** FE-CV-TASK-085-01, 092-01; FE-CV-SOL-050-store-and-query-hooks
**Status:** [ ] TODO

## Context

- `quality.trace` T/o 20 s; `inProgress` retry ≤ 90 s.
- Không lưu bền.

## Việc cần làm

1. Gọi khi lens hiển thị; `includeInferred` theo `showInferred`.
2. Làm mới theo push; đánh dấu cũ khi `codeIntel.changed`.
3. Phân loại lỗi theo hợp đồng 2.3 (disabled/quality-disabled → state `disabled`).

## Kiểm thử

- Cờ tắt 0 lời gọi; retry; huỷ khi unmount.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Không ném ra render.

## Rủi ro

- Quy mô dữ liệu chưa đo.
