# BE-CV-TASK-024-01: Re-verify nền sự kiện (outbox, processed_events, ephemeral consumer, gateway stream)

**From Solution:** BE-CV-SOL-024-event-distribution
**Priority:** P0
**Service:** `code-intel-service`
**File:** (không sửa mã) `backend-go/services/code-intel-service/{migrations,internal}`, `backend-go/common/{eventbus,outbox}`
**Depends on:** —
**Status:** [ ] TODO

---

## Context

Hợp đồng đánh dấu nhiều điểm chưa kiểm: tên stream `CODEINTEL`, `SubscribeEphemeral`, MySQL `INSERT IGNORE`, gateway không mở lại luồng.

## Việc cần làm

1. Xác nhận bảng `outbox_events`, `processed_events (tenant_id,event_id)`, `reindex_jobs` đã có (migration 0001/0002) và cổng `BindingStreamTargets` (đọc xuyên tenant) do SOL-011 cài; nếu thiếu, ghi vào PR.
2. `grep -rn "CODEINTEL\"" backend-go --include=*.go` để chắc tên stream không trùng stream đang chạy.
3. Đọc `notification-service/internal/adapter/mysql/repository.go` (`MarkProcessed`) để chốt cách đếm hàng với driver.
4. Xác nhận SOL-023 đã có `StreamCodeIntelEvents`, SOL-022 đã có `InvalidateBinding/Probe`, SOL-021 có `Watch`/`ReindexStatus`.
5. Đọc `api-gateway/.../channels_push.go` xác nhận `Recv` lỗi → đóng kênh (không mở lại) và ghi vào PR cho SOL-040.

## Kiểm thử

- Lệnh grep/đọc ở trên; dán kết quả.

## Tiêu chí hoàn thành

- [ ] Danh sách phụ thuộc chưa sẵn ghi vào PR. - [ ] Cách đếm hàng MySQL chốt.

## Rủi ro và lưu ý

- Chỉ-đọc.
