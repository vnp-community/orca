# BE-CV-TASK-032-06: Trích danh mục kênh `wscompat` (literal, hàm dùng chung, push, trùng tên)

**From Solution:** BE-CV-SOL-032-proto-and-wscompat-contract-catalog
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/gocallgraph/ws_channel_extraction.go` (mới) và `_test.go`
**Depends on:** BE-CV-TASK-032-05
**Status:** [x] DONE

---

## Context

SOL-032 mục 2.C.4. `registry.go` có 4 bản đồ; hàm dùng chung đăng ký kênh biến (`registerAccountsRelay(r, client, channel, agentMethod)`).

## Việc cần làm

1. Parse `wscompat/*.go` không-test (qua cổng); receiver `r` lấy từ tham số kiểu `*Registry`.
2. Tên kênh: literal; nối `"browser."+op` với `op` thuộc `range []string{…}`; còn lại truy ngược lời gọi hàm đăng ký dùng chung để lấy tên literal; không được → `Dynamic=true`.
3. Đối số 2: closure hoặc tên hàm cùng package, đi vào thân **một** cấp; đích `rpc`/`agent-method`/`local`.
4. `PushEvent{Channel:"…"}` → `Kind:push` (tên kênh chuỗi mở, gồm `agent:rateLimited`).
5. `Duplicate` → `CHANNEL_DUPLICATE`; kết quả sắp theo tên; ngân sách 800 kênh.

## Kiểm thử

`go test ./services/code-intel-service/internal/adapter/gocallgraph/... -run WsChannel` (chưa chạy): fixture rút từ `channels_accounts.go`; `accounts.*` → `agent-method` đúng; `host.wsl.isAvailable` → `local`; trùng tên; `browser.*` khai triển.

## Tiêu chí hoàn thành

- [x] Số kênh literal bằng số do test đếm độc lập; `Dynamic` chỉ khi không khai triển được.
- [x] Không trả hình dạng `args`.

## Rủi ro và lưu ý

- Kênh đăng ký bằng bảng cấu hình ở file khác sẽ `Dynamic`.
