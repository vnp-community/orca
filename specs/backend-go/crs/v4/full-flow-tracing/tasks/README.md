# backend-go Tasks — Full-Flow Tracing

**Solutions:** [../solutions/](../solutions/README.md)

## ⚠️ Nhắc lại cảnh báo CRITICAL (xem [solutions/README.md](../solutions/README.md) cho chi tiết đầy đủ)

3 symbol nền tảng dùng bởi 17/17 (hoặc gần hết) service backend-go đều
🔴 **CRITICAL risk** theo `impact()` (đã re-verify 2026-09-09, khớp 100%
với audit gốc, không giảm): `ChainUnary` (32 impacted), `Init` (34
impacted), `correlatingHandler` (37 impacted). Mọi task chạm 1 trong 3
symbol này (TASK-BE-FFT-001, 002, 003, 007) **PHẢI**:
1. Tự chạy lại `impact({target, direction: "upstream"})` qua MCP
   `mcp__gitnexus__impact` **ngay trước khi sửa** — không dùng số liệu ở
   trên nếu code đã đổi kể từ 2026-09-09 (task trước trong cùng chuỗi có
   thể đã sửa cùng file).
2. Giữ đúng additive-only — không đổi signature/behavior mặc định.
3. Có test xác nhận không regression hành vi hiện tại của 17 service.
4. Chạy `detect_changes({scope:"compare", base_ref:"main"})` trước khi
   commit, xác nhận scope thay đổi đúng dự kiến.

## Track duy nhất — tuyến tính, không có track song song

Khác với `automations`/`storage` (nhiều track độc lập), series này **phụ
thuộc tuyến tính gần như hoàn toàn theo CR** (CR-FFT-001 → 002 → 003), nên
task cũng theo đúng thứ tự đó. Trong nội bộ mỗi CR, một vài task có thể
làm song song (đánh dấu ở cột "Depends on").

| Task | CR / Solution | Symbol chạm | Depends on | Status |
|---|---|---|---|---|
| [TASK-BE-FFT-001](./TASK-BE-FFT-001-stats-handler-server-span.md) — `grpcmw.StatsHandler()` + wire 16 service | CR-FFT-001 / BE-FFT-SOL-001 | — (hàm mới; **đọc** `ChainUnary` để không đụng vào) | Không | ✅ DONE |
| [TASK-BE-FFT-002](./TASK-BE-FFT-002-tracing-init-propagator.md) — `tracing.Init` set propagator | CR-FFT-001 / BE-FFT-SOL-001 | 🔴 `Init` | Không | ✅ DONE |
| [TASK-BE-FFT-003](./TASK-BE-FFT-003-correlating-handler-trace-id.md) — `correlatingHandler.Handle` đọc `trace_id`/`span_id` | CR-FFT-001 / BE-FFT-SOL-001 | 🔴 `correlatingHandler` | Không | ✅ DONE |
| [TASK-BE-FFT-004](./TASK-BE-FFT-004-api-gateway-http-edge-span.md) — `api-gateway` HTTP edge span (`otelhttp`) | CR-FFT-001 / BE-FFT-SOL-001 | — (composition root only) | TASK-BE-FFT-002 (mềm — cần propagator để có ý nghĩa) | ✅ DONE |
| [TASK-BE-FFT-005](./TASK-BE-FFT-005-outbound-client-span-propagation.md) — outbound `Dial` propagation (N service) | CR-FFT-001 / BE-FFT-SOL-001 | — (per-service, không CRITICAL) | TASK-BE-FFT-002 (mềm) | ✅ DONE |
| [TASK-BE-FFT-006](./TASK-BE-FFT-006-trace-event-json-shape.md) — `TraceEvent` Go struct + mapping | CR-FFT-002 / BE-FFT-SOL-002 | — (type mới) | Không | ✅ DONE |
| [TASK-BE-FFT-007](./TASK-BE-FFT-007-trace-event-span-processor.md) — `TraceEventSpanProcessor` + `Init`'s `opts` | CR-FFT-002 / BE-FFT-SOL-002 | 🔴 `Init` (lần 2) | TASK-BE-FFT-001,002,003 (cả CR-FFT-001 xong), TASK-BE-FFT-006 | ✅ DONE |
| [TASK-BE-FFT-008](./TASK-BE-FFT-008-wire-trace-event-publisher-per-service.md) — wire `WithTraceEventPublisher` cho 6 service | CR-FFT-002 / BE-FFT-SOL-002 | — (composition root only) | TASK-BE-FFT-007 | ✅ DONE |
| [TASK-BE-FFT-009](./TASK-BE-FFT-009-trace-broadcast-fanout-registry.md) — `traceBroadcast` type mới | CR-FFT-003 / BE-FFT-SOL-003 | — (type mới) | Không (nhưng cả CR-FFT-002 phải DONE trước khi merge chuỗi) | ✅ DONE |
| [TASK-BE-FFT-010](./TASK-BE-FFT-010-mount-trace-routes-real-forwarding.md) — `mountTraceRoutes` xoá TODO, forward event thật | CR-FFT-003 / BE-FFT-SOL-003 | 🟢 `mountTraceRoutes` (LOW) | TASK-BE-FFT-009 | ✅ DONE |
| [TASK-BE-FFT-011](./TASK-BE-FFT-011-subscribe-ephemeral-wiring.md) — `main.go` SubscribeEphemeral wiring | CR-FFT-003 / BE-FFT-SOL-003 | — (composition root only) | TASK-BE-FFT-008, 009, 010 | ✅ DONE |

## Thứ tự thực thi

```
CR-FFT-001 (BE-FFT-SOL-001):
  001 ┐
  002 ├─ độc lập nhau, có thể song song (file khác nhau: grpcmw.go / tracing.go / logging.go)
  003 ┘
  004 → phụ thuộc mềm 002 (cần propagator có ý nghĩa)
  005 → phụ thuộc mềm 002

CR-FFT-002 (BE-FFT-SOL-002):
  006 → độc lập, có thể làm song song với cả nhóm 001-005
  007 → phụ thuộc CỨNG 001+002+003 (toàn bộ CR-FFT-001 xong) + 006
  008 → phụ thuộc CỨNG 007

CR-FFT-003 (BE-FFT-SOL-003):
  009 → độc lập cấu trúc (type mới), nhưng CR-FFT-002 phải DONE để chuỗi có ý nghĩa
  010 → phụ thuộc CỨNG 009
  011 → phụ thuộc CỨNG 008 + 009 + 010
```

Không có 2 CR nào chạy song song hoàn toàn — 007 luôn phải đợi cả 3 task
của CR-FFT-001 xong (không chỉ 002), vì `Init`'s option thứ 2 (CR-FFT-002)
được thêm lên trên bản đã có option đầu (CR-FFT-001) trong cùng file.

## Nguyên tắc chung cho AI thực thi các task này

- **Luôn `impact()` lại ngay trước khi sửa 1 trong 3 symbol CRITICAL** —
  không dùng số liệu đã ghi trong solution/CR nếu chưa tự kiểm tra lại tại
  thời điểm task đó thực thi (task trước có thể đã đổi file).
- **Không tự ý mở rộng phạm vi rollout** — CR-FFT-002 chỉ bật
  `WithTraceEventPublisher` cho 6/17 service (api-gateway + 5 đã nối NATS
  sẵn); TASK-BE-FFT-008 không được tự ý thêm service thứ 7 dù thấy "tiện
  làm luôn" — ghi nhận, không sửa.
- **Test regression bắt buộc, không tuỳ chọn** — mọi task chạm `ChainUnary`
  (gián tiếp qua `StatsHandler`), `Init`, hoặc `correlatingHandler` phải
  có ít nhất 1 test xác nhận 17 service khác không đổi hành vi (log JSON
  shape không đổi khi ngoài span; `grpc.NewServer(...)` vẫn nhận đúng 2
  option cũ nguyên vẹn khi thêm option thứ 3).
- **`detect_changes({scope:"compare", base_ref:"main"})` trước mỗi commit**
  — xác nhận scope thay đổi khớp đúng file task đã liệt kê, không có symbol
  ngoài dự kiến bị ảnh hưởng.
- **`go build ./... && go test ./...` thật, không suy đoán pass** — đặc
  biệt với `common/tracing`, `common/logging`, `common/grpcmw` vì 3 package
  này không có test hiện có (`common/logging`, `common/tracing` chưa có
  file `_test.go` nào tính tới lúc audit — xác nhận bằng `find`) — mọi
  test trong các task dưới đây là **file mới**, không phải mở rộng test có
  sẵn.


**Toàn bộ 11/11 task DONE (2026-09-09).** CR-FFT-001/002/003 hoàn thành.
