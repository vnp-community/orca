# BE-CV-TASK-036-10: RPC `GetReadingOrder`, bộ test xác định và cô lập tenant

**From Solution:** BE-CV-SOL-036-reading-order-and-risk
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/get_reading_order.go`, `get_reading_order_test.go`; `internal/adapter/grpc/reading_order_handler.go`, `reading_order_handler_test.go` (mới); `internal/domain/readingorder/determinism_test.go` (mới); `cmd/server/main.go` (sửa: đăng ký)
**Depends on:** BE-CV-TASK-036-05, BE-CV-TASK-036-07
**Status:** [x] DONE

---

## Context

`GetReadingOrder` (hợp đồng §3.1) trả `steps[]` + `components[]`. Dùng lại kết quả overlay từ cache; không tính lại khi đã có.

## Việc cần làm

1. `get_reading_order.go`: cổng như `GetChangeOverlay` (cờ, quyền `read`, `selector`); tra snapshot `readingOrder`/`changeOverlay` cùng khoá; miss ⇒ gọi `GetChangeOverlay.Execute` nội bộ (cùng singleflight) rồi trích `steps`, `components`; `if_none_match`.
2. Handler: ánh xạ domain ⇄ proto; lỗi `CODEINTEL_*`; `ResultMeta` phẳng.
3. `determinism_test.go`: chạy cả pipeline thuần (06–09) 100 lần trên fixture 3 000 tệp/2 000 symbol, so byte; chạy với hoán vị đầu vào.
4. Test tenant: hai tenant cùng khoá; tenant B không đọc được snapshot của A; quyền sai không chạm cache.
5. Ghi metric thời gian tính domain (`codeintel_overlay_compute_seconds`, tên đề xuất; `BE-CV-SOL-071` chốt) — không log nội dung.

## Kiểm thử

- `go test ./services/code-intel-service/internal/usecase/ -run ReadingOrder -race`; `go test ./services/code-intel-service/internal/domain/readingorder/ -run Determinism`; `go test ./services/code-intel-service/internal/adapter/grpc/ -run ReadingOrder`.
- Hai dialect: không có truy vấn riêng; ma trận snapshot ở `BE-CV-SOL-022` (ghi rõ nếu chưa có).

## Tiêu chí hoàn thành

- [x] 100 lần giống byte; `GetReadingOrder` không tính lại khi overlay đã cache.
- [x] Cô lập tenant xanh; `buf breaking` xanh.
- [x] Kênh `codeIntel.readingOrder` chưa đăng ký ở gateway (việc của `BE-CV-SOL-040-codeintel-view-channels`).

## Rủi ro và lưu ý

- Hai RPC dùng chung kết quả: cẩn thận khoá cache khác `detail`; `GetReadingOrder` luôn dùng `detail=full`.
