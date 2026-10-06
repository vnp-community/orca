# BE-CV-TASK-034-06: `ListDataFlows` (ứng viên từ catalog, phân trang, lọc)

**From Solution:** BE-CV-SOL-034-data-flow-model
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/list_data_flows.go` (mới), `.../usecase/list_data_flows_test.go` (mới)
**Depends on:** BE-CV-TASK-034-03, BE-CV-TASK-032-07
**Status:** [ ] TODO

---

## Context

Solution 2.B; chỉ cần catalog; rẻ.

## Việc cần làm

1. Ứng viên = mỗi kênh (`ws:<name>`) và mỗi RPC (`grpc:<Svc>.<Rpc>`); `DataFlowSummary{entry_service, entry_rpc, service_hops (biết từ catalog), completeness (`partial` nếu Unimplemented đã biết)}`.
2. Lọc `trigger_kind`, `query` (≤ 128, không phân biệt hoa/thường, trên tên), `service`; sắp theo `id`; `page_size ≤ 100`; `page_token` mờ (base64 của khoá keyset), kiểm hợp lệ.
3. `total` chính xác khi không cắt.

## Kiểm thử

`go test ... -run ListDataFlows` (chưa chạy): phân trang không trùng/sót; token giả; lọc.

## Tiêu chí hoàn thành

- [ ] Số ứng viên = kênh + RPC của catalog.
- [ ] Không dựng chi tiết.

## Rủi ro và lưu ý

- Catalog `truncated` → `total` chỉ có nghĩa trong phần đã có.
