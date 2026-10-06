# FE-CV-TASK-056-05: Danh sách luồng, `useDataFlows`, `useDataFlow`

**From Solution:** [FE-CV-SOL-056-dataflow-lens](../solutions/FE-CV-SOL-056-dataflow-lens.md) mục 4.2
**Priority:** P1
**Area:** frontend / review-map + hooks
**File:** `hooks/useDataFlows.ts`, `hooks/useDataFlow.ts`, `components/review-map/DataFlowListPane.tsx`, `store/slices/review-ui.ts` (thêm `dataFlowId`), tests
**Depends on:** FE-CV-TASK-050-13, FE-CV-TASK-056-03, FE-CV-TASK-051-01
**Status:** [ ] TODO

## Context

- `dataFlows {triggerKind?, query? ≤128, service?, limit? ≤100, pageToken?}`; `dataFlow {flowId, detail?, maxSteps? ≤200, includeSequence?}`.

## Việc cần làm

1. `useDataFlows` phân trang, debounce 300 ms cho `query`, lọc `triggerKind`/`service`; `useDataFlow(flowId, {detail:'service'|'component'})` với `includeSequence:true`, `maxSteps:200`, huỷ khi đổi nhanh.
2. `DataFlowListPane`: ô tìm, công tắc "Chỉ luồng chạm thay đổi" (giao id; trạng thái "Chưa xác định"), danh sách (`label`, `trigger`, `serviceHops`, nhãn `partial`), "Tải thêm"; dưới 720 px thành `Select`.
3. `dataFlowId` trong `ReviewUiState`; id lạ ⇒ thông báo, không lỗi; vào từ "Luồng liên quan" bằng `query=label`.
4. Lỗi/trạng thái: rỗng ("Chưa dựng được luồng dữ liệu nào cho repo này"), `SYMBOL_NOT_FOUND`, `timeout`.

## Kiểm thử

- Debounce; phân trang; huỷ; id lạ; công tắc "chạm thay đổi"; rò rỉ khoá `dataFlowId` ở hai đường xoá.

## Tiêu chí hoàn thành

- [ ] Không tìm kiếm phía client trên trang đã tải (dùng `query` server).

## Rủi ro

- `query` tìm theo trường nào: hợp đồng không nói.
