# TASK-REQ-001-05: `main.go`, máy chủ gRPC khung, health, relay, EnsureStream

**From Solution:** BE-REQ-SOL-001
**Priority:** P0
**Service:** `request-service`
**File:** `cmd/server/main.go`, `cmd/server/mysql_dsn.go`, `cmd/server/mysql_dsn_test.go`, `internal/adapter/grpc/server.go`, `internal/adapter/grpc/server_test.go` (tất cả mới)
**Depends on:** TASK-REQ-001-02 (stub proto), TASK-REQ-001-04 (repository, relay store)
**Status:** `[x] DONE`

---

## Context

`task-service/cmd/server/main.go` có `switch caps.Dialect` (khoảng dòng 148 đến 182), đăng ký health `postgres`/`mysql`, khởi `outbox.NewRelay(repo, pub, outbox.DefaultConfig, logger)` và chờ bằng `WaitGroup` (dòng 399 đến 420, 496 đến 500), và hàm `toMySQLDriverDSN` ở dòng 516. `mcp-service/cmd/server/main.go:128` gọi `pub.EnsureStream(ctx, "MCP", []string{"orca.mcp.>"})`. `grpcmw.ChainUnary(logger)` + `grpcmw.StatsHandler()` là bộ interceptor chuẩn. Không import chéo service: viết lại `toMySQLDriverDSN` trong `cmd/server`.

## Việc cần làm

1. `cmd/server/mysql_dsn.go`: `func toMySQLDriverDSN(dsn string) (string, error)`: nhận `mysql://user:pass@host:port/db?...` và `tidb://...`, trả DSN driver `user:pass@tcp(host:port)/db?parseTime=true&loc=UTC&multiStatements=false`; lỗi khi scheme sai hoặc thiếu host. Đọc hàm gốc của `task-service` để giữ cùng tham số (`parseTime`, `loc`).
2. `internal/adapter/grpc/server.go`: `type Server struct { requestv1.UnimplementedRequestServiceServer }` và `NewServer()`. `GetRequest`, `ListRequests` chưa viết: nhúng `Unimplemented` trả `codes.Unimplemented`. Nếu TASK-REQ-001-02 giữ `ApprovalService`, thêm `type ApprovalServer struct{ requestv1.UnimplementedApprovalServiceServer }`.
3. `cmd/server/main.go` theo thứ tự SOL-001 mục C: logger, `apperrors.SetLogger`, tracing, DSN (`secrets.DatabaseCredentialsFromFile(cfg.DatabaseCredentialsFile)` rồi `cfg.DatabaseDSN`), `DetectDialectFromDSN` (lỗi thì in "unsupported database dialect" và thoát mã 1), `switch caps.Dialect` tạo `repo` và đăng ký health.
4. `eventbus.Connect(ctx, cfg.NATSURL)`: `NATSURL` rỗng hoặc lỗi kết nối thì log cảnh báo và bỏ qua relay (như `task-service`). Có kết nối: `pub.EnsureStream(ctx, "REQUEST", []string{"orca.request.>"})`, `outbox.NewRelay(repo, pub, outbox.DefaultConfig, logger)` trong goroutine có `WaitGroup`; đăng ký health `nats`.
5. `grpc.NewServer(grpcmw.ChainUnary(logger), grpcmw.StatsHandler())`, `requestv1.RegisterRequestServiceServer`, đăng ký `grpc_health_v1`; HTTP `health.New().Handler()` trên `HTTPPort` (`/healthz`, `/readyz`).
6. Log một dòng khi khởi động: `request_flow_enabled=<bool>` (chỉ đọc).
7. Tắt êm: bắt `SIGINT`/`SIGTERM`, `grpcServer.GracefulStop()`, `httpServer.Shutdown`, `outboxRelayWG.Wait()`, đóng pool.
8. Cập nhật bảng real vs stub trong `README.md`: `GetRequest`, `ListRequests` là `Unimplemented`; outbox relay, health là thật.

## Kiểm thử

- `TestToMySQLDriverDSN` (bảng: `mysql://`, `tidb://`, scheme `http://`, thiếu host, có query).
- `TestServer_UnimplementedRPCs`: khởi server in-process (`bufconn`), gọi `GetRequest`, mong `codes.Unimplemented`, không panic.
- `TestMain_UnknownDSNExitsNonZero`: chạy hàm `run(ctx, cfg)` (tách `run` khỏi `main`) với `DATABASE_DSN=sqlite://x`, mong lỗi chứa "unrecognized DSN scheme".
- Integration (từng dialect): khởi `run` với DSN container, `GET /healthz` và `/readyz` trả 200, gRPC health `SERVING`.
- Lệnh: `go test ./services/request-service/cmd/... ./services/request-service/internal/adapter/grpc/...`.

## Tiêu chí hoàn thành

- [x] Service khởi động với Postgres và với MySQL; `/healthz`, `/readyz`, gRPC health đều OK.
- [x] DSN scheme lạ thoát với lỗi nêu dialect không hỗ trợ.
- [x] RPC chưa viết trả `Unimplemented`, không panic.
- [x] Ghi một dòng outbox rồi relay publish lên NATS (kiểm bằng subscriber thử trong test tích hợp, `NATS_URL` trỏ container NATS).
- [x] Không có `helpers`, `utils`, `common`, `misc`.

## Rủi ro và lưu ý

- Stream `REQUEST` không tạo được (NATS không có JetStream) thì consumer ở CR-REQ-005 không đăng ký; chỉ log cảnh báo, không thoát.
- `RegisterApprovalServiceServer` với service rỗng: nếu bỏ `approval.proto` ở TASK-REQ-001-02 thì bỏ dòng này.
