# TASK-E2E-002: Test P1: mọi RPC công khai của `RequestService` có kênh gateway hoặc loại trừ có lý do

**From Solution:** SOL-E2E-001 · **Resolves:** BUG-E2E-001
**Priority:** P0 · **Service/Area:** api-gateway
**Files:**
- backend-go/services/api-gateway/internal/adapter/wscompat/channels_request_parity_test.go (mới)
- backend-go/services/api-gateway/internal/adapter/wscompat/request_rpc_channel_map.go (mới, chỉ dùng trong test hoặc build tag test)
**Depends on:** TASK-E2E-001
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

Gateway có `parity_test.go` cho MCP nhưng không có kiểm tra "RPC công khai ↔ kênh WS". Đó là lý do thiếu `clarification.*` không bị bắt.

## Việc cần làm

- [x] Duyệt `requestv1.RequestService_ServiceDesc.Methods` và `ApprovalService_ServiceDesc.Methods`.
- [x] Bảng `map[rpcName]entry{channel string, exclude string}` tường minh; RPC nhóm nội bộ (`GroupInternal` trong catalog của request-service) ghi `exclude` kèm lý do.
- [x] So với `Registry.Channels()` của `registerRequestChannels`.
- [x] Báo lỗi liệt kê: RPC chưa nằm trong bảng; kênh trong bảng không đăng ký; kênh đăng ký không có RPC.

## Tiêu chí hoàn thành

- [x] Thêm RPC giả vào descriptor (test con) làm test đỏ với thông báo đúng.
- [x] Test xanh với bảng đầy đủ hiện tại; RPC nội bộ đều có lý do loại trừ.

## Kiểm thử

Chính test này; thêm một test con chứng minh nó đỏ khi thiếu (đảo bảng).

## Kết quả thực thi

Xong. `channels_request_parity_test.go` + `request_rpc_channel_map_test.go`: 3 service (Request, Approval, ApprovalPolicyAdmin), mỗi RPC có kênh hoặc lý do loại trừ (`internal:` / `deferred: CR-...`). 4 test con chứng minh test đỏ khi: RPC mới không có mục, kênh không đăng ký, kênh mồ côi, loại trừ không lý do.
