# TASK-REQ-016-01: Nối client `request-service` vào `api-gateway` (config, dial, `ChannelDeps`, health)

**From Solution:** BE-REQ-SOL-016
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/config/config.go`, `backend-go/services/api-gateway/cmd/server/main.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/register_production.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_request_unavailable.go` (mới), `backend-go/services/api-gateway/internal/adapter/wscompat/channels_request.go` (mới, khung)
**Depends on:** CR-REQ-001 (proto `orca.request.v1` đã sinh vào `proto/gen/go/orca/request/v1`)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: cd backend-go/services/api-gateway && go build ./... && go vet ./... && go test ./... -count=1)

---

## Context

- `config.go:153-168`: `OtherServiceAddrs` là `map[string]string` đọc `*_SERVICE_ADDR`; chưa có `request-service`.
- `main.go:176-181`: mẫu dial `task-service` (`gatewaygrpc.Dial`, `defer conn.Close()`); `main.go:371` dựng `wscompat.ChannelDeps{...}`; `main.go:380` `TaskActivityEnabled: natsErr == nil`, `TaskActivityBus: natsConsumer`; `main.go:558` `healthSrv.Register("task-service", grpcConnHealthCheck(taskConn))`.
- `register_production.go:27-55` `ChannelDeps`, `:60` `RegisterProductionChannels`. Parity test dựng registry qua hàm này với field `nil`, nên mọi field mới phải chịu được `nil`.
- `internal/adapter/grpc/dial.go:31` `Dial(addr)` gọi `grpc.NewClient(addr, ...)`. Chưa kiểm chứng hành vi khi `addr == ""`; task này không dial khi rỗng.
- Proto `orca.request.v1` chưa tồn tại tại thời điểm viết (không có `backend-go/services/request-service`). Nếu CR-REQ-001 chưa merge, dùng nhánh chung hoặc đợi.

## Việc cần làm

1. `config.go`: thêm `"request-service": commonconfig.StringEnv("REQUEST_SERVICE_ADDR", "")` vào `OtherServiceAddrs`. Cập nhật test cấu hình (nếu có test liệt kê khoá map).
2. `main.go`: sau khối `task-service`, nếu `cfg.OtherServiceAddrs["request-service"] != ""` thì dial, tạo `requestv1.NewRequestServiceClient(conn)` và `requestv1.NewApprovalServiceClient(conn)`, `defer conn.Close()`, `healthSrv.Register("request-service", grpcConnHealthCheck(conn))`. Rỗng thì để hai client `nil` và `slog.Info` một dòng.
3. `register_production.go`: thêm `Request requestv1.RequestServiceClient` và `Approval requestv1.ApprovalServiceClient` vào `ChannelDeps`; cuối `RegisterProductionChannels` gọi `registerRequestChannels(r, d.Request, d.Approval, d.TaskActivityBus, d.TaskActivityEnabled)`.
4. `channels_request.go` (khung): `func registerRequestChannels(r *Registry, req requestv1.RequestServiceClient, appr requestv1.ApprovalServiceClient, bus ephemeralSubscriber, streamEnabled bool)` chưa đăng ký kênh nào (các task sau thêm). Truyền `bus` kiểu interface, không `*Consumer`, để test dùng fake.
5. `channels_request_unavailable.go`: `var errRequestUnavailable = errors.New("REQUEST_UNAVAILABLE: request service not configured")` và hàm `requestClientMissing(c any) bool` (kiểm `nil` interface và con trỏ nil).
6. `deploy/dev/docker-compose.yml` và `docker-compose` của backend (nếu có): chưa sửa ở task này (đăng ký service thuộc CR-REQ-001 và CR-REQ-025 task kiểm đăng ký).

## Kiểm thử

- `register_production_test.go` (sửa): dựng `ChannelDeps{}` rỗng, `RegisterProductionChannels` không panic.
- `config_test.go` (sửa hoặc mới): `REQUEST_SERVICE_ADDR` rỗng và có giá trị.
- Lệnh: `cd backend-go/services/api-gateway && go build ./... && go test ./internal/config/... ./internal/adapter/wscompat/...`.

## Tiêu chí hoàn thành

- [x] Gateway khởi động được khi `REQUEST_SERVICE_ADDR` rỗng; `/health` không báo `request-service` lỗi giả.
- [x] Có địa chỉ: client được dial và đăng ký health.
- [x] `go vet`, `go build`, test `wscompat` xanh; parity test chưa đổi kết quả (chưa có kênh mới).

## Rủi ro và lưu ý

- Không dial địa chỉ rỗng: tránh lỗi khởi động chưa kiểm chứng của `grpc.NewClient("")`.
- Không thêm `max-lines` disable; `main.go` đã dài, tách helper `dialRequestService(cfg) (...)` ra file riêng `cmd/server/request_wiring.go` nếu thêm quá 25 dòng.

## Ghi chú triển khai (2026-10-08)

Gateway thật dial khi `REQUEST_SERVICE_ADDR` có giá trị (`cmd/server/request_wiring.go`, `main.go`, health `request-service`); test bufconn `request_wiring_channels_test.go` kiểm token, danh tính, actor-type. Compose dev chưa sửa (ngoài phạm vi).
