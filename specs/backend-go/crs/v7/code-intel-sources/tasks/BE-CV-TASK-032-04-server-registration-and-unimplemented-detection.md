# BE-CV-TASK-032-04: `Register<Svc>Server`, handler và phát hiện RPC `Unimplemented`

**From Solution:** BE-CV-SOL-032-proto-and-wscompat-contract-catalog
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/gocallgraph/server_registration.go`, `handler_detection.go` (mới) và `_test.go`
**Depends on:** BE-CV-TASK-032-03, BE-CV-TASK-030-05
**Status:** [x] DONE

---

## Context

SOL-032 mục 2.C.2. 18/18 service có một chỗ đăng ký (CR); `infra-fleet-service` đăng ký qua wrapper `withAgentSessionList`.

## Việc cần làm

1. `go/parser` `cmd/server/main.go`: `*ast.CallExpr` dạng `<alias>.Register<Svc>Server`; ánh xạ alias → package proto qua import `…/proto/gen/go/orca/<pkg>/v1`; `ImplementedBy` = thư mục service.
2. Chịu wrapper: nếu đối số thứ hai là lời gọi hàm bọc, ghi nhận receiver của hàm bọc để tìm handler.
3. `handler_detection.go`: thu `FuncDecl` có receiver tên trùng RPC trong `internal/adapter/grpc/*.go`; tham số thứ hai kiểu `*<alias>.<Rpc>Request` (bất kỳ nếu streaming); gồm receiver thứ hai (`AgentSessionListServer`).
4. RPC không handler → `Unimplemented=true` + `RPC_UNIMPLEMENTED`; thân chỉ trả `status.Error(codes.Unimplemented…)` → `Unimplemented` kèm nhãn suy luận.
5. `SymbolRef` handler theo PQ-20.

## Kiểm thử

`go test ./services/code-intel-service/internal/adapter/gocallgraph/... -run Server` (chưa chạy); fixture `infra-fleet-service`: 7 RPC `Unimplemented`; `mcp-service` hai đăng ký; thân `Unimplemented` giả.

## Tiêu chí hoàn thành

- [x] 18/18 `ImplementedBy`.
- [x] Số `Unimplemented` của `infra-fleet-service` đúng số thực (CR: 7).

## Rủi ro và lưu ý

- Hành vi runtime `Unimplemented` chưa chạy; chỉ suy từ nhúng `Unimplemented<Svc>Server`.
