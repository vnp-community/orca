# backend-go Solutions — Full-Flow Tracing (v4)

**CRs:** [docs/crs/v4/full-flow-tracing/](../../../../../../docs/crs/v4/full-flow-tracing/README.md)
**TDD tham chiếu:** `specs/backend-go/tdd/architecture/09-observability-reliability.md` (yêu cầu gốc), `specs/backend-go/tdd/architecture/08-inter-service-communication.md` (NATS subject convention)

> **KHÔNG nhầm với** `specs/backend/crs/v2/full-flow-tracing/`,
> `specs/frontend/crs/v2/full-flow-tracing/`, `specs/agent/crs/v2/full-flow-tracing/`
> (CR-TRACE-000…019) — series đó giải quyết việc rollout instrumentation
> (`createTracer`/`span.start/step/ok/fail`) vào 19 luồng nghiệp vụ của kiến
> trúc `backend/` Electron cũ. Thư mục này (v4, CR-FFT-001…003) giải quyết
> một gap hoàn toàn khác và hẹp hơn nhiều: `backend-go/services/api-gateway`'s
> SSE endpoint `/api/trace-stream` chưa forward được event trace thật vì
> chưa có span OTel nào được tạo, và chưa có bridge span → NATS. Hai việc
> không chia sẻ code, không chia sẻ convention wire-format.

## ⚠️ CẢNH BÁO RỦI RO CRITICAL — đọc trước khi làm bất cứ điều gì ở đây

**Đây là mức rủi ro cao nhất trong toàn bộ các bộ CR backend-go đã audit
tính tới nay.** CR-FFT-001 chạm vào **3 symbol nền tảng, mỗi symbol đều
CRITICAL risk theo `impact()`, dùng bởi toàn bộ hoặc gần toàn bộ 17 service
backend-go**:

| Symbol | File | Risk | Impacted | Re-verify 2026-09-09 (`impact()` thật, MCP) |
|---|---|---|---|---|
| `ChainUnary` | `common/grpcmw/grpcmw.go` | 🔴 CRITICAL | 32 (16 direct, 9 process) | **Khớp 100%** với số liệu CR-FFT-001 đã ghi |
| `Init` | `common/tracing/tracing.go` | 🔴 CRITICAL | 34 (17 direct, 10 process) | **Khớp 100%** |
| `correlatingHandler` | `common/logging/logging.go` | 🔴 CRITICAL | 37 (3 direct, 10 process) | **Khớp 100%** |

Ba con số này **không giảm** so với audit gốc — đã chạy lại `impact()` qua
MCP `mcp__gitnexus__impact` (không phải suy đoán/dùng lại số liệu cũ) ngay
trước khi viết solution này, cùng ngày. Risk **KHÔNG được hạ xuống** ở bất
kỳ task nào bên dưới.

**Vì sao vẫn an toàn để làm**: cả 2 CR (001, 002) đều thiết kế theo nguyên
tắc **additive-only** — không sửa signature/behavior mặc định của
`ChainUnary`/`Init`/`correlatingHandler` hiện có:
- `ChainUnary`: **không sửa** — thêm hàm `StatsHandler()` mới đứng song
  song, mỗi `main.go` tự thêm option mới vào `grpc.NewServer(...)`.
- `Init`: chỉ thêm 1 dòng nội bộ (`otel.SetTextMapPropagator`, CR-FFT-001)
  và 1 tham số biến-đổi `opts ...Option` (CR-FFT-002) — không đổi 2 tham
  số bắt buộc hiện có, mọi call site cũ compile nguyên trạng nếu không
  truyền option mới.
- `correlatingHandler`: chỉ thêm logic **bên trong** `Handle()` (đọc thêm
  span context nếu có) — không đổi struct field, không đổi method
  signature, không đổi behavior khi ngoài span (log JSON giữ nguyên shape).

Mỗi task chạm 1 trong 3 symbol này **PHẢI** tự chạy lại `impact()` ngay
trước khi sửa (không dùng lại bảng trên nếu code đã đổi kể từ 2026-09-09) —
xem mục "gitnexus" trong từng task. Và **PHẢI** có test xác nhận không có
regression hành vi hiện tại của 17 service khi thêm OTel interceptor —
đây là điều kiện bắt buộc, không tuỳ chọn.

## Solutions

| Solution | CR | Symbol CRITICAL chạm vào | Status |
|---|---|---|---|
| [BE-FFT-SOL-001](./BE-FFT-SOL-001-otel-span-instrumentation.md) | CR-FFT-001 | `ChainUnary`, `Init`, `correlatingHandler` (cả 3) | 🔲 Designed — chưa implement |
| [BE-FFT-SOL-002](./BE-FFT-SOL-002-span-to-nats-trace-event-bridge.md) | CR-FFT-002 | `Init` (lại — thêm option thứ 2) | 🔲 Designed — chưa implement |
| [BE-FFT-SOL-003](./BE-FFT-SOL-003-api-gateway-sse-real-forwarding.md) | CR-FFT-003 | Không — `mountTraceRoutes` là 🟢 LOW (3 impacted, 1 direct, đã re-verify khớp) | 🔲 Designed — chưa implement |

## Thứ tự implement — tuyến tính bắt buộc, không làm song song được

```
BE-FFT-SOL-001 (OTel span thật)
        │  span tồn tại, nhưng chưa ai đọc được từ process khác
        ▼
BE-FFT-SOL-002 (Span → NATS JetStream, orca.<service>.trace.span)
        │  event đã ra NATS, nhưng api-gateway chưa consume
        ▼
BE-FFT-SOL-003 (api-gateway subscribe NATS → fan-out SSE)
```

Khác với `automations`/`storage` series (nhiều CR độc lập, làm song song
được) — ở đây **mỗi solution phụ thuộc cứng vào solution trước**, đúng như
CR gốc đã ghi ở `docs/crs/v4/full-flow-tracing/README.md`'s "Thứ tự thực
thi". Không có 2 track song song trong series này.

## Phạm vi được xác nhận lại (2026-09-09) khi viết solutions — khác biệt nhỏ so với CR gốc

Đọc trực tiếp code (không suy đoán) trong lúc viết solution phát hiện thêm
vài chi tiết CR gốc chưa nói rõ, không đổi kết luận nhưng ảnh hưởng cách
chia task:

1. **Không có 1 package `common/grpcclient` dùng chung** cho outbound gRPC
   dial như CR-FFT-001's bảng "Changes Required" gợi ý (`backend-go/common/
   grpcclient (hoặc package dial dùng chung mỗi service)`) — mỗi service tự
   có `internal/adapter/grpc/dial.go` hoặc `internal/adapter/grpcclient/
   dial.go` riêng, cùng khuôn `func Dial(addr string) (*grpc.ClientConn,
   error)` nhưng là code **trùng lặp per-service**, không phải 1 symbol
   chung. Outbound propagation (mục 3 trong CR-FFT-001) vì vậy là N sửa đổi
   nhỏ độc lập (mỗi service 1 dòng thêm `grpc.WithStatsHandler(...)`), rủi
   ro thấp hơn nhiều so với 3 symbol CRITICAL ở trên — xem
   [TASK-BE-FFT-005](../tasks/TASK-BE-FFT-005-outbound-client-span-propagation.md).
2. **`common/eventbus.Publisher.Publish`'s doc comment tự nêu rõ**: "Called
   by an outbox relay loop, not directly from a request handler" — CR-FFT-002
   publish trace span **trực tiếp**, bỏ qua outbox, là lệch khỏi convention
   mặc định của chính package này. CR-FFT-002 đã tự biện minh việc này
   (trace data diagnostic/best-effort, không phải domain-of-record) — solution
   giữ nguyên quyết định đó, chỉ nhấn mạnh lại đây **không phải sơ suất**.
3. Frontend's `TraceEvent` (`frontend/src/shared/trace/index.ts:19-34`) có
   thêm 1 field optional `elapsedMs?: number` mà CR-FFT-002's bảng mapping
   không liệt kê — vì field này optional và không có OTel span timing
   tương đương rõ ràng ở mức boundary-span (chỉ có ý nghĩa cho `step`, mà
   CR-FFT-001/002 chưa tạo `level: "step"`), backend-go **không cần gửi**
   field này (JSON thiếu field optional vẫn hợp lệ với `TraceEvent` phía
   TS) — ghi rõ trong BE-FFT-SOL-002 để không ai tưởng nhầm là thiếu sót.

## Nguyên tắc bảo mật/vận hành xuyên suốt

- Trace event là **diagnostic, không nhạy cảm** (kế thừa nguyên văn từ
  `trace_routes.go`'s comment và bản TS `trace-sse-routes.ts:63`) — không
  áp dụng outbox, không áp dụng tenant-scoping cho payload (khác hẳn
  nguyên tắc `tenantID`/`userID` luôn qua `tenant.RequireTenantID(ctx)` áp
  dụng cho *domain* data ở series `storage`/`automations`). Endpoint SSE
  vẫn giữ nguyên chính sách "intentionally low-security" đã có, không đổi
  ở series này.
- Additive-only là nguyên tắc xuyên suốt cả 3 solution — bất kỳ task nào
  phát hiện cần đổi signature/behavior mặc định của 1 trong 3 symbol
  CRITICAL để implement, PHẢI dừng lại và escalate, không tự ý mở rộng.
