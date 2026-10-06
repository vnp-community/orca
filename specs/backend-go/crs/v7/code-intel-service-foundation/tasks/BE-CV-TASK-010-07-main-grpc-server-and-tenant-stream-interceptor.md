# BE-CV-TASK-010-07: `main.go`, gRPC server khung, interceptor stream lấy tenant, health, relay, `EnsureStream`

**From Solution:** BE-CV-SOL-010-scaffold-code-intel-service
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/cmd/server/main.go`, `cmd/server/mysql_driver_dsn.go`, `cmd/server/mysql_driver_dsn_test.go`, `internal/adapter/grpc/server.go`, `internal/adapter/grpc/tenant_stream_interceptor.go`, `internal/adapter/grpc/tenant_stream_interceptor_test.go`, `internal/adapter/eventbus/outbox_relay.go` (đều mới)
**Depends on:** BE-CV-TASK-010-03, 010-04, 010-06
**Status:** [ ] TODO

---

## Context

Mẫu: `notification-service/cmd/server/main.go` (dialect switch dòng 124, `toMySQLDriverDSN` dòng 331), `task-service/cmd/server/main.go` (`eventbus.Connect` dòng 400, `EnsureStream` dòng 408, `NewRelay` dòng 411, `reflection.Register` dòng 441), `mcp-service/cmd/server/main.go` (interceptor guard dòng 222–227, stream tự đọc metadata `governance_server.go:272`). `grpcmw` chỉ có `ChainUnary`; không stream interceptor nào trong `common`.

## Việc cần làm

1. `mysql_driver_dsn.go`: sao `toMySQLDriverDSN` (hai dạng đầu vào; tự thêm `parseTime=true`; scheme lạ lỗi). Test bảng: `mysql://u:p@h:3306/db`, `mysql://u:p@tcp(h:3306)/db`, `tidb://…`, `postgres://…` (lỗi), thiếu host (lỗi), đã có `parseTime=`.
2. `tenant_stream_interceptor.go`: `StreamServerInterceptor` đọc bốn khoá `grpcmw.MetadataTenantID/UserID/Role/ClientIP`, gọi `tenant.WithTenantID/WithUserID/WithRole/WithClientIP`, bọc `grpc.ServerStream` để `Context()` trả ctx đã gắn. Không khoá mới. Test với stream giả: có metadata → `tenant.RequireTenantID` ok; không → lỗi.
3. `server.go`: `type Server struct{ codeintelv1.UnimplementedCodeIntelServiceServer }`.
4. `outbox_relay.go`: hàm khởi chạy `outbox.NewRelay(repo, pub, outbox.DefaultConfig, logger).Run(ctx)` trong goroutine, trả hàm đợi (`WaitGroup`).
5. `main.go` theo SOL-010 mục 2.D: tách `buildServerOptions(cfg, logger) []grpc.ServerOption` (chứa `grpcmw.ChainUnary`, `grpcmw.StatsHandler()`, `grpc.ChainStreamInterceptor(tenantStreamInterceptor())`; SOL-013 chèn `internalcaller` vào đúng hàm này). Cảnh báo (log WARN) khi `CODEINTEL_INTERNAL_CALLER_TOKEN` rỗng (mọi RPC bị chặn).
6. Health: chỉ HTTP `health.New()` `/healthz`, `/readyz` (đăng ký `postgres` hoặc `mysql` ping 2 s); **không** gRPC health.
7. `EnsureStream(ctx, "CODEINTEL", []string{"orca.codeintel.>"})`; lỗi bus: log WARN, không thoát.
8. Tắt êm: `GracefulStop`, `httpServer.Shutdown(10s)`, `WaitGroup.Wait()`.

## Kiểm thử

- `go test ./services/code-intel-service/cmd/... ./services/code-intel-service/internal/adapter/grpc/...`.
- Integration (tag): khởi động server với DSN Postgres rồi MySQL (testcontainers), gọi `/healthz`, `/readyz` → 200; reflection liệt kê `orca.codeintel.v1.CodeIntelService`; DSN `sqlite://` → `run()` trả lỗi nêu dialect.
- Gọi RPC chưa có (nếu có RPC thêm sau) trả `Unimplemented`, không panic.

## Tiêu chí hoàn thành

- [ ] Hai dialect khởi động được; DSN lạ thoát mã khác 0.
- [ ] Stream không metadata tenant bị từ chối; có metadata đọc được tenant.
- [ ] Relay publish được sự kiện lên NATS testcontainer; lỗi NATS không làm service thoát.
- [ ] Không gRPC health; `reflection` bật kèm chú thích "sau mesh".

## Rủi ro và lưu ý

- `LoggingInterceptor` chỉ unary: RPC stream không có log request; ghi log tại handler (SOL-024).
- `buildServerOptions` là điểm tích hợp của SOL-013; không đổi chữ ký sau khi merge.
