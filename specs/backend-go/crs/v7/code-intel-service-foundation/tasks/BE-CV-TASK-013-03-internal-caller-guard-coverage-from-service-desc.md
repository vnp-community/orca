# BE-CV-TASK-013-03: `internalcaller` phủ mọi RPC (sinh từ `ServiceDesc`), kể cả stream và `QualityGateService`

**From Solution:** BE-CV-SOL-013-authorization-flags-and-audit
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/grpc/internal_caller_guard.go`, `internal_caller_guard_test.go` (mới); `cmd/server/main.go` / hàm `buildServerOptions` (sửa)
**Depends on:** BE-CV-TASK-010-07, 010-04
**Status:** [ ] TODO

---

## Context

`internalcaller.Guard(token, fullMethods...)` chỉ chặn method **được liệt kê**; method không liệt kê đi qua (`common/internalcaller/internalcaller.go`). `mcp-service` chỉ thêm interceptor khi token khác rỗng. Ở đây hợp đồng §3 yêu cầu "token rỗng = chặn mọi RPC": phải liệt kê đầy đủ và luôn thêm interceptor. Gateway gắn token bằng `ClientInterceptor`/`StreamClientInterceptor` (SOL-040).

## Việc cần làm

1. `GuardedMethods() (unary, stream []string)`: duyệt `codeintelv1.CodeIntelService_ServiceDesc` (và `QualityGateService_ServiceDesc` khi có; dùng danh sách `[]*grpc.ServiceDesc` tiêm được) → `"/"+ServiceName+"/"+MethodName` cho `Methods`, và `Streams`.
2. Trong `buildServerOptions`: **luôn** `grpc.ChainUnaryInterceptor(internalcaller.Guard(cfg.InternalCallerToken, unary...))` và `grpc.ChainStreamInterceptor(internalcaller.StreamGuard(cfg.InternalCallerToken, stream...))` đặt **sau** tenant extraction và trước pipeline chính sách; token rỗng → log WARN (mọi RPC bị chặn).
3. Test phản chiếu: dựng server bufconn đăng ký mọi service; với từng method trong `ServiceDesc` gọi không token → `PermissionDenied` + `INTERNAL_CALLER_REQUIRED`; có token đúng → chạm handler (`Unimplemented` chấp nhận); token sai → từ chối; token rỗng cấu hình → từ chối cả khi client gửi token rỗng.
4. Test "RPC mới": thêm một `ServiceDesc` giả có method không có trong danh sách guard → test đỏ (chứng minh danh sách sinh tự động, không viết tay).
5. Không đăng ký `reflection` nếu không muốn lộ danh sách method? Giữ `reflection` (SOL-010) vì reflection **không** qua guard; ghi vào README "chỉ sau mesh".

## Kiểm thử

- `go test ./services/code-intel-service/internal/adapter/grpc/... -run InternalCaller`

## Tiêu chí hoàn thành

- [ ] Không method nào (unary hoặc stream) lọt khi không token.
- [ ] Token rỗng = chặn hết; WARN được log.

## Rủi ro và lưu ý

- Reflection không bị guard (thông tin cấu trúc, không dữ liệu).
- Token chung là lớp nông, không thay mTLS.
