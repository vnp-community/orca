# FE-CV-TASK-056-05: Danh sách luồng, `useDataFlows`, `useDataFlow`

**From Solution:** [FE-CV-SOL-056-dataflow-lens](../solutions/FE-CV-SOL-056-dataflow-lens.md) mục 4.2
**Priority:** P1
**Area:** frontend / review-map + hooks
**File:** `hooks/useDataFlows.ts`, `hooks/useDataFlow.ts`, `components/review-map/DataFlowListPane.tsx`, `store/slices/review-ui.ts` (thêm `dataFlowId`), tests
**Depends on:** FE-CV-TASK-050-13, FE-CV-TASK-056-03, FE-CV-TASK-051-01
**Status:** [x] DONE (verified 2026-10-08: DataFlowFilters.test 3/3, dataflow/ 36/36 PASS)

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

## Ghi chú triển khai (2026-10-07)

`hooks/useDataFlows.ts` (limit 100, debounce 300 ms, `pageToken`, huỷ khi đổi lọc, khử trùng id), `hooks/useDataFlow.ts` (`includeSequence:true`, `maxSteps:200`), `dataflow/DataFlowListPane.tsx`. vitest useDataFlows.test.tsx 5/5, DataFlowLens.test.tsx. `dataFlowId` ở `review-ui.ts`. Công tắc khởi tạo bật khi `chipFilter==="flows"`.

## Ghi chú tích hợp (W6, 2026-10-07)

Liên kết dùng `setReviewDataFlowId(flow.id)` + `setReviewLens("dataflow")` (không cần `query=label`): `DataFlowLens` mở chi tiết theo id, kể cả khi luồng không nằm trong trang danh sách đã tải (`fallbackLabel`).

## Ghi chú hoàn thiện (2026-10-08, P4)

- Bộ lọc server `triggerKind`/`service`: `dataflow/DataFlowFilters.tsx` (shadcn `Select`, mục "Tất cả" = không lọc); danh sách dịch vụ lấy từ `entryService` của các luồng đã tải (hợp đồng không có API liệt kê dịch vụ), giữ lựa chọn hiện tại.
- Dưới 720 px (đo khung lens, không đo cửa sổ — `dataflow/use-pane-width-below.ts`): danh sách thành `Select` trong `DataFlowListPane` (`compact`), bố cục bỏ `md:flex-row`.
- Khoá i18n `auto.components.reviewMap.dataflow.trigger.*`, `triggerFilter`, `serviceFilter`, `allTriggers`, `allServices`, `pickShort` (5 locale).
