# TASK-REQ-025-02: Interceptor gRPC `flow_gate` với bảng phân loại mọi RPC

**From Solution:** BE-REQ-SOL-025
**Priority:** P0
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/adapter/grpc/flow_gate.go` (mới), `.../internal/adapter/grpc/flow_gate_test.go` (mới), `.../cmd/server/main.go`
**Depends on:** TASK-REQ-025-01
**Status:** `[ ] TODO`

---

## Context

- `usage-service`/`mcp-service` `cmd/server/main.go`: `grpc.NewServer(grpcmw.ChainUnary(logger), grpcmw.StatsHandler())`; interceptor thêm vào chuỗi (`common/grpcmw`). `common/internalcaller.Guard` là mẫu interceptor lọc theo `fullMethods`.
- Phân loại và hành vi khi tắt: CR-REQ-025 mục 2.2 (đọc, thoát an toàn, đi tiếp, nội bộ) và SOL-025 mục 2.2.
- Mã lỗi `REQUEST_FLOW_DISABLED`, gRPC `FailedPrecondition`.
- Danh sách RPC lấy từ proto `orca.request.v1` (RequestService, ApprovalService) ở thời điểm triển khai; thêm RPC của CR 026 đến 036 sau này.

## Việc cần làm

1. `flow_gate.go`: `type flowClass int` (`classRead`, `classSafeExit`, `classAdvance`, `classInternal`); `var flowMethodClass = map[string]flowClass{...}` có đủ mọi phương thức (`/orca.request.v1.RequestService/...`, `/orca.request.v1.ApprovalService/...`).
2. `func FlowGate(effective func(ctx context.Context) (bool, error)) grpc.UnaryServerInterceptor`: lớp `classRead`, `classSafeExit`, `classInternal` đi qua; `classAdvance` cần `effective == true`, lỗi đọc thì từ chối (fail closed); phương thức không có trong bảng: coi như `classAdvance` (từ chối khi tắt) và `slog.Warn`.
3. Đặt interceptor **sau** bước trích tenant (`grpcmw` tenant extraction) vì cờ theo tenant, và **trước** logic use case.
4. `main.go`: nối interceptor với `EffectiveFlow` của task 01.
5. Streaming RPC (nếu có sau này) cần `StreamInterceptor` tương ứng; hiện chưa có, ghi `TODO`.

## Kiểm thử

- `flow_gate_test.go`: (a) với mọi RPC khai trong `RequestService_ServiceDesc` và `ApprovalService_ServiceDesc` (lấy từ mã sinh), khẳng định có dòng trong `flowMethodClass` (test đỏ khi thêm RPC mà quên phân loại); (b) cờ tắt: lớp đọc, thoát an toàn, nội bộ thành công; lớp đi tiếp trả `REQUEST_FLOW_DISABLED`; (c) lỗi đọc cờ thì lớp đi tiếp bị từ chối; (d) cờ bật thì mọi lớp đi qua.
- `go test ./internal/adapter/grpc/...`.

## Tiêu chí hoàn thành

- [ ] Mọi RPC có phân loại (test quét proto).
- [ ] `ReportTaskOutcome` và consumer outbox không bị chặn khi tắt.
- [ ] Tenant A bật không ảnh hưởng tenant B (test với hai ngữ cảnh).

## Rủi ro và lưu ý

- Hết hạn Approval và consumer outbox không đi qua gRPC; chúng phải gọi `effective` hoặc không phụ thuộc cờ theo bảng (nội bộ chạy bất kể cờ). Ghi rõ trong mã.
- Mọi lối ghi khác (HTTP, MCP) đi qua cùng RPC nên không cần thêm cổng ở gateway.
