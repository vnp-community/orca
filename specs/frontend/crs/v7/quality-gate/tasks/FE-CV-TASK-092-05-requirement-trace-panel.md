# FE-CV-TASK-092-05: Panel, hàng yêu cầu, danh sách bằng chứng và thay đổi chưa gắn

**From Solution:** [FE-CV-SOL-092-requirement-trace-view](../solutions/FE-CV-SOL-092-requirement-trace-view.md) mục 2.3
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/requirements/RequirementTracePanel.tsx`, `RequirementRow.tsx`, `RequirementEvidenceList.tsx`, `UnlinkedChangesList.tsx` (mới) + test
**Depends on:** FE-CV-TASK-092-01..04; FE-CV-SOL-053-impact-lens-and-symbol-detail
**Status:** [x] DONE (verified 2026-10-07: 10 tests panel + RequirementRow/RequirementEvidenceList/UnlinkedChangesList (kiểm qua panel))

## Context

- Trạng thái rỗng có hành động trực tiếp; lỗi persistent inline; token, lucide, không hex.

## Việc cần làm

1. Hàng theo wireframe; icon khác hình theo state; nhãn "Suy luận".
2. Nhảy tới bằng chứng qua `pendingDiffReveal`; run → lens quality.
3. Trạng thái: loading (ngưỡng 100 ms/1 s/3 s), rỗng, lỗi, disabled.

## Kiểm thử

- Mỗi trạng thái; văn bản thuần; `aria` đúng.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Không chuỗi cấm.

## Rủi ro

- Phụ thuộc 053.

## Ghi chú triển khai (2026-10-07)

Tách thêm `RequirementRow.tsx`, `RequirementEvidenceList.tsx`, `UnlinkedChangesList.tsx` đúng cây file spec. Nhảy tới bằng chứng dùng `onOpenDiff` (chưa có `pendingDiffReveal` của SOL-053).
