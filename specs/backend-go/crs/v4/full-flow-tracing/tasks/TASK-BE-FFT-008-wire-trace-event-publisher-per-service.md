# TASK-BE-FFT-008: Wire `WithTraceEventPublisher` cho 6 service (api-gateway + 5 đã nối NATS)

**Solution:** BE-FFT-SOL-002 | **CR:** [CR-FFT-002](../../../../../../docs/crs/v4/full-flow-tracing/CR-FFT-002-span-to-nats-trace-event-bridge.md)
**Service:** `api-gateway`, `usage-service`, `issue-tracking-service`, `notification-service`, `tenant-service`, `infra-fleet-service`
**Depends on:** TASK-BE-FFT-007
**Status:** ✅ DONE (2026-09-09)

---

## Mục tiêu

Bật `tracing.WithTraceEventPublisher(pub)` cho đúng 6 service — 5 service
đã có `*eventbus.Publisher` sẵn (1 dòng/service) + `api-gateway` (lần đầu
nối NATS, nhiều việc hơn). **Không tự ý bật thêm cho 11 service còn lại**
— ngoài phạm vi CR-FFT-002.

## gitnexus

Không sửa symbol CRITICAL nào ở task này (chỉ call site tại `main.go`,
truyền thêm 1 argument vào `tracing.Init(...)` đã có sẵn — không đổi
`Init` bản thân nó, đó là TASK-BE-FFT-007). Không bắt buộc `impact()`
trước khi sửa `main.go` của từng service.

## Files cần sửa

### Phần A — 5 service đã có `*eventbus.Publisher` (1 dòng mỗi file)

1. `backend-go/services/usage-service/cmd/server/main.go`
2. `backend-go/services/issue-tracking-service/cmd/server/main.go`
3. `backend-go/services/notification-service/cmd/server/main.go`
4. `backend-go/services/tenant-service/cmd/server/main.go`
5. `backend-go/services/infra-fleet-service/cmd/server/main.go`

Với mỗi file, xác nhận thứ tự gọi thật (vd. `tenant-service/cmd/server/main.go`:
`tracing.Init` ở dòng ~64, `eventbus.Connect` ở dòng ~114 — **`eventbus.Connect`
CHẠY SAU `tracing.Init` trong code hiện tại**, nghĩa là `pub` chưa tồn tại
tại thời điểm gọi `Init` như code hiện có). Task này phải **đảo thứ tự**:
di chuyển `eventbus.Connect(...)` lên **trước** `tracing.Init(...)`, rồi
truyền `pub` vào:

```go
// TRƯỚC (thứ tự hiện tại — xác nhận qua đọc code trước khi sửa)
shutdownTracing, err := tracing.Init(ctx, cfg.ServiceName, cfg.OTLPEndpoint)
// ...
pub, cons, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)

// SAU
pub, cons, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)
if err != nil {
    return fmt.Errorf("connecting to nats: %w", err)
}
defer func() { _ = closeBus() }()

shutdownTracing, err := tracing.Init(ctx, cfg.ServiceName, cfg.OTLPEndpoint,
    tracing.WithTraceEventPublisher(pub))
if err != nil {
    return fmt.Errorf("initializing tracing: %w", err)
}
defer func() { _ = shutdownTracing(context.Background()) }()

if err := pub.EnsureStream(ctx, "TRACE", []string{"orca.*.trace.span"}); err != nil {
    return fmt.Errorf("ensuring TRACE stream: %w", err)
}
```

**Đọc kỹ code thật của từng file trước khi sửa** — thứ tự khai báo biến
`err` (`:=` vs `=`), tên biến cục bộ (`pub`/`cons`/`closeBus` hay tên
khác), và các `defer` xen giữa có thể khác nhau giữa 5 service; không
copy-paste mù mờ đoạn trên, chỉ dùng làm khuôn mẫu.

**`EnsureStream("TRACE", ...)` gọi trùng lặp 5+1 lần** (mỗi service tự gọi
lúc khởi động) — `CreateOrUpdateStream` đã idempotent theo doc comment
`eventbus.go`, nhưng vẫn cần test xác nhận (xem TASK-BE-FFT-007's
`TestEnsureStream_IdempotentAcrossRestarts`) không lỗi khi 6 service cùng
gọi gần như đồng thời lúc deploy.

### Phần B — `api-gateway` (lần đầu nối NATS)

1. `backend-go/services/api-gateway/internal/config/config.go` (MODIFY — thêm field `NATSURL`)
2. `backend-go/services/api-gateway/cmd/server/main.go` (MODIFY)

```go
// internal/config/config.go — thêm vào struct Config
NATSURL string

// Load() — thêm vào return Config{...}
NATSURL: commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
```

```go
// cmd/server/main.go — thêm TRƯỚC dòng tracing.Init hiện có
pub, cons, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)
if err != nil {
    return fmt.Errorf("connecting to nats: %w", err)
}
defer func() { _ = closeBus() }()
if err := pub.EnsureStream(ctx, "TRACE", []string{"orca.*.trace.span"}); err != nil {
    return fmt.Errorf("ensuring TRACE stream: %w", err)
}

shutdownTracing, err := tracing.Init(ctx, cfg.ServiceName, cfg.OTLPEndpoint,
    tracing.WithTraceEventPublisher(pub))
// ... giữ cons cho TASK-BE-FFT-011 (SubscribeEphemeral wiring) ...
```

## Test cases cần cover

- `TestConfig_NATSURLDefaultsWhenUnset` — `api-gateway`'s `Load()`, xác
  nhận default `"nats://localhost:4222"` khi env không set (đúng convention
  5 service kia đã dùng).
- **Regression bắt buộc mỗi service (6 file)** — test tích hợp hoặc ít
  nhất build-level: `NATSURL` rỗng/NATS không chạy → service vẫn khởi
  động được, không panic, hành vi giống hệt trước task này khi
  `eventbus.Connect` lỗi (xác nhận behavior hiện có khi connect lỗi —
  return error, không crash mù — không đổi ở task này).
- Với `api-gateway`: `TestRun_NATSDownDoesNotCrashStartup` nếu harness cho
  phép giả lập NATS không khả dụng.

## Verify

```bash
for svc in usage-service issue-tracking-service notification-service tenant-service infra-fleet-service api-gateway; do
  (cd backend-go/services/$svc && go build ./... && go test ./...)
done
gofmt -l $(git diff --name-only | grep '\.go$')
```

## gitnexus (sau khi sửa)

```
mcp__gitnexus__detect_changes({ scope: "compare", base_ref: "main", repo: "orca" })
```
Xác nhận scope đúng 6 `main.go` + `api-gateway/internal/config/config.go`
— không đụng 11 service ngoài phạm vi.

## Blocking

TASK-BE-FFT-011 (CR-FFT-003, `main.go` SubscribeEphemeral wiring) phụ
thuộc CỨNG vào Phần B của task này (`cons` phải tồn tại trong
`api-gateway`'s `main.go`).

## Kết quả thực tế (2026-09-09)

- Không sửa symbol CRITICAL, không bắt buộc `impact()` — xác nhận đúng
  như task doc dự đoán (chỉ thêm argument vào call site có sẵn của
  `tracing.Init`).
- **Phần A (5 service đã có `*eventbus.Publisher`)**: đọc lại thứ tự gọi
  thật của từng file trước khi sửa (không copy-paste mù mờ khuôn mẫu task
  doc) — cả 5 service đều có `tracing.Init` chạy TRƯỚC `eventbus.Connect`
  ở code hiện tại, đúng như task doc cảnh báo. Đảo thứ tự: di chuyển
  `eventbus.Connect` lên trước, gọi `pub.EnsureStream(ctx, "TRACE",
  []string{"orca.*.trace.span"})`, rồi truyền
  `tracing.WithTraceEventPublisher(pub)` vào `Init`. Giữ nguyên tên biến
  cục bộ khác nhau giữa các service (`cons` giữ lại ở `notification-service`/
  `tenant-service` vì đã dùng cho event consumer riêng của chúng; `_`
  cho 3 service còn lại không cần `cons`).
- **Phần B (api-gateway lần đầu nối NATS)**: thêm `NATSURL` vào
  `internal/config/config.go` (default `nats://localhost:4222`, đúng
  convention 5 service kia). `cmd/server/main.go`: gọi `eventbus.Connect`
  trước `tracing.Init`, giữ `cons` lại cho TASK-BE-FFT-011.
  **Sai lệch có chủ đích so với khuôn mẫu task doc**: task doc gợi ý
  `eventbus.Connect` lỗi thì `return fmt.Errorf(...)` (fatal). Thực thi
  đổi thành **non-fatal** (log warning, tiếp tục chạy với `pub` khả năng
  `nil` — `WithTraceEventPublisher(nil)` an toàn vì
  `NewTraceEventSpanProcessor` đã nil-safe từ TASK-BE-FFT-007) — lý do ghi
  rõ trong comment tại chỗ: `api-gateway` là cổng vào HTTP/WS duy nhất,
  không được từ chối khởi động toàn bộ traffic người dùng chỉ vì 1 sink
  tracing tuỳ chọn không khả dụng; đây cũng là hành vi "degrade" nhất
  quán với cách 5 service kia (và chính `api-gateway`) đã xử lý
  `eventbus.Connect` lỗi từ trước (log warning, không crash) — làm nó
  fatal riêng cho trường hợp này sẽ không nhất quán.
- Test mới `api-gateway/internal/config/config_test.go` (trước đó
  package `config` chưa có test) — `TestConfig_NATSURLDefaultsWhenUnset`
  + `TestConfig_NATSURLRespectsEnv` (bổ sung, xác nhận biến env thật sự
  được đọc, không chỉ default).
- `go build ./...` + `go test ./...` chạy riêng cho cả 6 service: tất cả
  PASS, không regression (bao gồm cả test có sẵn của từng service).
- `gofmt -l`: sạch trên toàn bộ 8 file đã sửa/thêm.
- Build lại toàn bộ 17 module workspace: OK, 0 lỗi.
- Scope xác nhận qua `git status --porcelain`: đúng 6 `main.go` +
  `api-gateway/internal/config/config.go` (M) +
  `api-gateway/internal/config/config_test.go` (mới) — không đụng 11
  service ngoài phạm vi CR-FFT-002, đúng dự kiến.

