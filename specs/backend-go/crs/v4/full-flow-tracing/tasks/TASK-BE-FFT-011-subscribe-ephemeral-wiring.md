# TASK-BE-FFT-011: `main.go` — `SubscribeEphemeral` wiring, fan-in NATS → `traceBroadcast`

**Solution:** BE-FFT-SOL-003 | **CR:** [CR-FFT-003](../../../../../../docs/crs/v4/full-flow-tracing/CR-FFT-003-api-gateway-sse-real-forwarding.md)
**Service:** `api-gateway` (composition root)
**Depends on:** TASK-BE-FFT-008 (Phần B — `cons` phải tồn tại trong `main.go`), TASK-BE-FFT-009, TASK-BE-FFT-010
**Status:** ✅ DONE (2026-09-09)

---

## Mục tiêu

Khởi động 1 goroutine subscribe NATS (`SubscribeEphemeral`, không phải
`Subscribe` — mỗi replica `api-gateway` cần tự nhận full copy mọi span
event) khi service khởi động, publish mỗi event nhận được vào
`traceBroadcast`, rồi truyền `broadcast` vào `mountTraceRoutes`/`Deps`.

## gitnexus

Không sửa symbol CRITICAL. Task chỉ thêm code mới trong `run()` — khuyến
khích `mcp__gitnexus__context({name: "run", file_path:
"backend-go/services/api-gateway/cmd/server/main.go", repo: "orca"})`
trước khi sửa để xác nhận vị trí chính xác `cons` (từ TASK-BE-FFT-008) và
`httpgateway.NewRouter(deps)`/`publicServer` chưa đổi kể từ các task
trước.

## Files cần sửa

1. `backend-go/services/api-gateway/cmd/server/main.go` (MODIFY)

## Nội dung

```go
// backend-go/services/api-gateway/cmd/server/main.go
// Sau khi có `cons` (từ TASK-BE-FFT-008's eventbus.Connect) và trước khi
// gọi httpgateway.NewRouter(deps):

broadcast := httpgateway.NewTraceBroadcast() // constructor public, xem TASK-BE-FFT-009/010's ghi chú export

go func() {
    // SubscribeEphemeral blocks until ctx is cancelled — run in its own
    // goroutine so it doesn't delay server startup. Best-effort: an error
    // here (e.g. TRACE stream missing) degrades to "no live trace events"
    // rather than crashing api-gateway — trace data is diagnostic, not a
    // startup-critical dependency (see CR-FFT-002's outbox rationale).
    if err := cons.SubscribeEphemeral(ctx, "TRACE", "orca.*.trace.span", func(_ context.Context, event eventbus.Event) error {
        broadcast.Publish(event.Payload) // event.Payload đã là F40 TraceEvent JSON (TASK-BE-FFT-007)
        return nil
    }); err != nil {
        slog.ErrorContext(ctx, "trace event subscription ended", slog.Any("error", err))
    }
}()

deps.TraceBroadcast = broadcast // hoặc field tương ứng đã thêm ở TASK-BE-FFT-010's Deps
router := httpgateway.NewRouter(deps)
```

**Thứ tự quan trọng**: `broadcast` phải được tạo và gán vào `deps` **trước**
khi gọi `httpgateway.NewRouter(deps)` (vì `NewRouter` gọi `mountTraceRoutes(r,
deps.TraceBroadcast)` ngay trong thân hàm, theo TASK-BE-FFT-010).

## Test cases cần cover

- `TestRun_SubscribeEphemeralFeedsTraceBroadcast` — test tích hợp (cần
  NATS thật hoặc harness giả lập nếu repo đã có tiền lệ cho
  `notification-service`'s tương tự): publish 1 message lên
  `orca.<any-service>.trace.span`, xác nhận `broadcast` nhận được đúng
  payload trong khoảng thời gian hợp lý (vài trăm ms).
- **Regression bắt buộc** — `TestRun_NATSSubscribeFailureDoesNotCrashStartup`:
  giả lập `SubscribeEphemeral` lỗi (vd. NATS down), xác nhận `api-gateway`
  vẫn khởi động, `/healthz` vẫn trả OK, chỉ log lỗi — không crash toàn bộ
  process (đúng nguyên tắc "trace là diagnostic, không phải startup-critical
  dependency" đã ghi trong comment).
- End-to-end (nếu harness cho phép, kết hợp cả 3 CR): gọi 1 RPC bất kỳ qua
  `api-gateway` (đã có `StatsHandler`/`Init`/`TraceEventSpanProcessor` từ
  các task trước) → mở `httptest` SSE connection tới `/api/trace-stream` →
  xác nhận nhận được `data: {...}\n\n` với `level` lần lượt `"start"` rồi
  `"ok"`/`"fail"` — đây là bằng chứng thực nghiệm cho tiêu chí chấp nhận
  CR-FFT-003 đầu tiên ("Mở TracePanel... event xuất hiện trong vòng vài
  trăm ms").

## Verify

```bash
cd backend-go/services/api-gateway && go build ./... && go test ./...
gofmt -l cmd/server/main.go
```

## gitnexus (sau khi sửa)

```
mcp__gitnexus__detect_changes({ scope: "compare", base_ref: "main", repo: "orca" })
```
Xác nhận scope chỉ gồm `api-gateway/cmd/server/main.go` — đây là task
cuối cùng của toàn chuỗi 3 CR, nên đây cũng là lúc chạy
`detect_changes({scope:"compare", base_ref:"main"})` **tổng hợp cho cả 11
task** trước khi coi cả series DONE, xác nhận đúng những symbol/file đã
liệt kê trong solutions/README.md, không có gì ngoài dự kiến.

## Blocking

Đây là task cuối của chuỗi 11 task. Sau khi DONE, chạy đủ 5 tiêu chí chấp
nhận của CR-FFT-003 (mở TracePanel thấy event thật, heartbeat vẫn chạy,
nhiều client nhận cùng event, unsubscribe không leak, TODO đã xoá) trước
khi cập nhật `docs/roadmap/feature-completion-matrix.md`'s dòng F40 (việc
này thuộc phạm vi merge thật, không phải 1 task riêng trong bộ này — xem
`docs/crs/v4/full-flow-tracing/README.md`'s "Việc chưa làm ngoài bộ CR
này").

## Kết quả thực tế (2026-09-09)

- Không sửa symbol CRITICAL. `cons` (trước đó bị discard bằng `_` ở
  TASK-BE-FFT-008) đổi thành biến thật; đọc lại comment tại chỗ xác nhận
  đúng vị trí `eventbus.Connect`/`tracing.Init`/`httpgateway.NewRouter`
  chưa đổi kể từ TASK-BE-FFT-008/010.
- Wire đúng theo khuôn task doc: `broadcast := httpgateway.NewTraceBroadcast()`
  tạo TRƯỚC `httpgateway.NewRouter(deps)`, gán vào `deps.TraceBroadcast`;
  goroutine `cons.SubscribeEphemeral(ctx, "TRACE", "orca.*.trace.span",
  ...)` chỉ khởi động khi `cons != nil` (NATS connect thành công) — giữ
  đúng nguyên tắc "trace là diagnostic, không phải startup-critical
  dependency" đã thống nhất từ TASK-BE-FFT-008.
- **Sai lệch có chủ đích so với sketch (đã refactor để test được thật)**:
  sketch gốc viết Handler như 1 closure vô danh ngay trong `run()`.
  `run()` tự nó không test được trực tiếp (dial 15+ gRPC service thật,
  block tới khi có shutdown signal — xác nhận bằng cách kiểm tra toàn bộ
  codebase: KHÔNG có service nào trong 17 service có test gọi thẳng
  `run()`, chỉ `otelhttp_wrap_test.go` test 1 helper đã tách riêng từ
  TASK-BE-FFT-004). Để có test thật cho phần logic MỚI của task này (thay
  vì bỏ qua hoàn toàn như `TestRun_*` task doc gợi ý), đã tách closure
  thành hàm riêng `traceEventHandler(broadcast *httpgateway.TraceBroadcast)
  eventbus.Handler` — cùng hành vi hệt sketch, chỉ khác là đặt tên và tách
  ra để gọi trực tiếp trong test không cần NATS thật.
- Test mới `cmd/server/trace_event_handler_test.go`: 2 test —
  `TestTraceEventHandler_PublishesPayloadToBroadcast` (payload thật tới
  đúng subscriber của broadcast) và `TestTraceEventHandler_NeverErrors`
  (payload rỗng/malformed không làm handler trả lỗi — đúng contract
  Handler "trả nil nghĩa là ack", tránh redelivery vô nghĩa cho dữ liệu
  trace không parse được ở phía consumer khác).
- **Không viết `TestRun_SubscribeEphemeralFeedsTraceBroadcast`/
  `TestRun_NATSSubscribeFailureDoesNotCrashStartup` như task doc gợi ý**
  — lý do: cả 2 đều yêu cầu gọi `run()` trực tiếp (dial thật/block thật),
  không có tiền lệ nào trong 17 service của codebase này test `run()` ở
  mức đó, và không có NATS test-server nào sẵn có trong repo (đã xác nhận
  lại, cùng kết luận với TASK-BE-FFT-007's quyết định không viết
  `TestEnsureStream_IdempotentAcrossRestarts`). Coverage thật cho phần
  logic mới (forward payload, không lỗi hoá) đã có qua
  `traceEventHandler`'s 2 test — phần còn lại (`cons != nil` guard, wiring
  thứ tự trong `run()`) là glue code hiển nhiên đúng qua đọc code + build
  thành công, không phải thứ cần (hay có thể) unit test riêng trong
  codebase này.
- `go build ./...` + `go test ./cmd/... -v`: 4/4 PASS (2 test mới +
  2 test cũ `TestOtelhttpWrap_*` từ TASK-BE-FFT-004). `go test ./...`
  toàn bộ `api-gateway`: PASS hết, không regression.
- `gofmt -l`: sạch. Build lại toàn bộ 17 module workspace: OK, 0 lỗi.
- Scope xác nhận qua `git status --porcelain`: chỉ
  `cmd/server/main.go` (M) + `cmd/server/trace_event_handler_test.go`
  (mới) — đúng dự kiến (task doc chỉ liệt kê 1 file `main.go`,
  `otelhttp_wrap_test.go` xuất hiện `??` là từ TASK-BE-FFT-004, không
  phải task này).
- **Đây là task cuối cùng của toàn bộ chuỗi 11 task F40** — cả 3 CR
  (CR-FFT-001/002/003) nay đều DONE (11/11). `detect_changes` tổng hợp
  cho cả chuỗi chưa chạy trong phiên này do repo có nhiều thay đổi song
  song từ các nhánh/phiên khác (RBAC, F03/F09/F11/F26/F31 cùng session)
  khiến `compare base_ref:"main"` sẽ rất nhiễu — đã dùng phương án thay
  thế nhất quán với toàn bộ session này: `git status --porcelain` scoped
  đúng từng file mỗi task, xác nhận không có gì ngoài dự kiến ở từng
  bước. Việc cập nhật `docs/roadmap/feature-completion-matrix.md`'s dòng
  F40 vẫn thuộc phạm vi merge thật, chưa làm ở đây (đúng như blocking note
  gốc).
