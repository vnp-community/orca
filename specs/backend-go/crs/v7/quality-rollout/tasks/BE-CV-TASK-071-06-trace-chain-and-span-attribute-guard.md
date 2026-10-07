# BE-CV-TASK-071-06: Kiểm tra chuỗi trace xuyên hop và rào thuộc tính span

**From Solution:** BE-CV-SOL-071
**Priority:** P1
**Service:** `code-intel-service`, `api-gateway`, `infra-fleet-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/otel/trace_chain_test.go` (mới), `.../internal/adapter/otel/span_attribute_guard.go` (mới, gói test-support đặt theo nội dung), `backend-go/services/code-intel-service/internal/adapter/grpc/otel_wiring.go` (mới), `.../cmd/server/main.go`
**Depends on:** BE-CV-TASK-071-03, 071-04, 071-05; BE-CV-SOL-010 (dial tới infra-fleet)
**Status:** `[x] DONE`

---

## Context

- `common/tracing.Init` đặt `TracerProvider` và `propagation.TraceContext{}`; bộ xử lý đẩy mọi span lên JetStream `TRACE` mà gateway fan-out cho UI (`api-gateway/cmd/server/main.go:118–137`) ⇒ thuộc tính span là dữ liệu **hiển thị** cho người dùng.
- `grpcmw.StatsHandler()` ở server; `gatewaygrpc.Dial` có `otelgrpc.NewClientHandler()` (`dial.go:32`). Dial từ `code-intel-service` tới infra-fleet phải có `otelgrpc` (mẫu `project-service/.../dev_server_relay.go:29`).
- Có tiền lệ "metrics + trace chain" trong conformance MCP (`run-go-conformance.sh`; `mcpmetrics/metrics_test.go` dùng tracer trong bộ nhớ).

## Việc cần làm

1. `otel_wiring.go`: đặt `grpcmw.StatsHandler()` (server) và `otelgrpc.NewClientHandler()` (client tới infra-fleet) trong `main.go` của service.
2. `span_attribute_guard.go`: hàm `AssertSafeAttributes(t, spans)` từ chối giá trị khớp mẫu đường dẫn (`/`, `\`, `X:\`), UUID đầy đủ, chuỗi > 64 ký tự, từ khoá `CANARY-`.
3. `trace_chain_test.go`: dựng chuỗi trong tiến trình (gateway wrapper → service interceptor → collector → fleet wrapper → perf span) với tracer trong bộ nhớ và `propagation.TraceContext`; khẳng định **một** `trace_id`, quan hệ cha-con đúng, có ít nhất một `codeintel.agent.cli`, và gọi `AssertSafeAttributes` trên mọi span.
4. Ghi lại (trong PR) kết quả kiểm tay trên stack dev: một `codeIntel.changeOverlay` lạnh cho một `trace_id` trên Jaeger/Tempo (chưa kiểm chứng; không đặt làm tiêu chí tự động).

## Kiểm thử

- Hai test mục 3; đỏ khi thiếu propagator, thiếu `otelgrpc` ở dial hoặc thuộc tính là đường dẫn.
- `go test ./internal/adapter/otel/...`; không DB.

## Tiêu chí hoàn thành

- [x] Chuỗi trong tiến trình có một `trace_id` và đủ span bốn hop.
- [x] Rào thuộc tính chạy trên mọi span.
- [x] `main.go` có `StatsHandler` và client handler.

## Rủi ro và lưu ý

- Test trong tiến trình không chứng minh propagate qua mạng thật (gRPC + WS); phần đó là kiểm tay.
- Agent đứt chuỗi (không `traceparent` qua JSON-RPC, `_trace.traceparent` là P2 chưa làm).
