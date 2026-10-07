# BE-CV-TASK-040-08: `conn.SetReadLimit(320 KiB)` cho `/ws` và bất biến timeout

**From Solution:** BE-CV-SOL-040-codeintel-channel-foundation
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/handler.go`, `channels_codeintel_limits_test.go` (mới), `handler_read_limit_test.go` (mới)
**Depends on:** TASK-040-07 (hằng timeout); quyết định O-2 (duyệt chạm toàn `/ws`)
**Status:** [x] DONE

---

## Context

`handler.go:113` gọi `websocket.Accept` và không đặt `SetReadLimit` (grep toàn `api-gateway` không có). PQ-14 (4): gateway phải `conn.SetReadLimit(320 << 10)`; nếu không `reviewState.save` (≤ 256 KiB) không bao giờ tới handler. Giới hạn mặc định của `coder/websocket` v1.8.15 chưa kiểm chứng. Bất biến hiện có: `rpcTimeout < invokeTimeout`, biên >= 5 s (`channels_test.go:1212`).

## Việc cần làm

1. `gitnexus_impact` trên `ServeHTTP` của `Handler`; ghi vào PR (ảnh hưởng mọi kênh `/ws`, terminal multiplex, files).
2. `handler.go`: ngay sau `Accept` thành công và trước vòng `conn.Read`: `conn.SetReadLimit(wsReadLimitBytes)`, `const wsReadLimitBytes = 320 << 10` với comment "Why": PQ-14, cho phép `reviewState.save` 256 KiB; các trần theo kênh nhỏ hơn.
3. Test bất biến: `TestCodeIntelTimeouts_ShorterThanInvokeTimeout`: `codeIntelReadTimeout`, `codeIntelStateTimeout`, `codeIntelSummaryTimeout` < `invokeTimeout`; `invokeTimeout - ReadTimeout >= 5 s`; `invokeTimeout - SummaryTimeout >= 1 s` (ngoại lệ có chủ ý, khung ghi dùng `writeTimeout` độc lập, `handler.go:~250`); `codeIntelStateTimeout == rpcTimeout`. Test `wsReadLimitBytes > 256<<10 + 8<<10` (đủ phong bì).
4. Test mạng thật: `httptest.NewServer(Handler)` với `Registry` có kênh echo, client `coder/websocket`: khung 300 KiB qua; khung 400 KiB làm kết nối đóng (ghi nhận mã đóng thực tế vào test, **không** giả định `StatusMessageTooBig`; nếu khác thì sửa tài liệu).
5. Ghi vào README gateway (TASK-040-09) rằng khung > 320 KiB làm rớt kết nối.

## Kiểm thử

`go test ./internal/adapter/wscompat/ -run 'ReadLimit|CodeIntelTimeouts'` (test mạng cần cổng loopback). Chạy cả `go test ./internal/adapter/wscompat/...` để chắc không kênh cũ vỡ.

## Tiêu chí hoàn thành

- [x] `SetReadLimit` đặt đúng một chỗ.
- [x] Khung 300 KiB qua, 400 KiB đóng (đo thật).
- [x] Bất biến timeout có test.

## Rủi ro và lưu ý

- Bộ nhớ tối đa mỗi kết nối tăng; chưa đo (CR-071).
- Nếu O-2 bị từ chối: thay bằng tách `reviewState.save`/`c4.save` thành nhiều lời gọi; khi đó cần sửa hợp đồng trước.
