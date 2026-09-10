# CR-FFT-003 — `api-gateway`: forward trace event thật qua `/api/trace-stream` (xoá TODO heartbeat-only)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-FFT-003 |
| **Tên** | Thay heartbeat-only bằng real event-forwarding cho SSE endpoint `/api/trace-stream` |
| **Loại** | Feature / Observability |
| **Priority** | 🔴 P0 (đây chính là gap user-facing — TracePanel trống rỗng) |
| **Effort** | Small–Medium (miễn CR-FFT-001/002 đã xong) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Audit trực tiếp mã nguồn theo yêu cầu "thực thi đầy đủ F40 ở lớp backend-go" |
| **Tác động Features** | F40 |
| **Phụ thuộc** | **CR-FFT-001, CR-FFT-002** (cần có event thật để forward trước khi sửa endpoint này) |

---

## Bối cảnh & Vấn đề

`backend-go/services/api-gateway/internal/adapter/httpgateway/trace_routes.go:24-30` — dòng TODO thật, trích nguyên văn:

```go
// Known gap: this only keeps the connection alive (heartbeats) — it does
// NOT forward any real trace/debug events yet, since backend-go has no
// equivalent to the old backend's global registerTraceSink() fan-out.
// TracePanel will show a live-but-empty stream rather than the 404 that
// was breaking EventSource's connection state before this existed. Wiring
// real event forwarding (e.g. from common/eventbus) is tracked as a
// follow-up in docs/execution-plan.md, not attempted here.
```

Đúng như comment tự nhận: `mountTraceRoutes` (`trace_routes.go:31-70`) hiện chỉ làm 2 việc — ghi `": connected\n\n"` 1 lần rồi lặp `ticker := time.NewTicker(15 * time.Second)` ghi `": heartbeat\n\n"` mỗi 15s cho tới khi client ngắt kết nối. Không có `registerTraceSink`-tương-đương, không có channel nhận event, không có branch nào trong `select` xử lý event thật ngoài `ctx.Done()` và `ticker.C`.

`backend-go/docs/execution-plan.md` dòng 465-479 xác nhận đây là quyết định có chủ đích tại thời điểm route được thêm vào ("Heartbeat-only for now — real backend trace-event forwarding... has no backend-go equivalent yet") — không phải bug, mà là 1 gap đã biết, tồn tại đúng vì lý do audit trước đó đã chỉ ra: chưa có nguồn event thật (CR-FFT-001/002 giải quyết phần đó). CR này đóng nốt phần cuối: nối `mountTraceRoutes` vào nguồn event thật đó.

Đối chiếu với bản gốc TS (`backend/src/server/trace-sse-routes.ts:36-47`) để giữ đúng wire behavior — file cũ dùng `registerTraceSink` (in-process callback, vì Node backend là 1 process duy nhất) fan-out tới `Set<ServerResponse>` các client đang mở, format mỗi message là:

```ts
const data = `data: ${JSON.stringify(event)}\n\n`
```

Frontend's `startSseClient` (`frontend/src/shared/trace/browser.ts:41-48`, **không đổi trong CR này**) parse trực tiếp `e.data` thành `TraceEvent` — nghĩa là backend-go chỉ cần ghi đúng format `data: <json>\n\n` với JSON đúng shape `TraceEvent` (đã định nghĩa ở CR-FFT-002) là tương thích ngay, không cần đổi 1 dòng frontend nào.

Khác biệt duy nhất với bản TS: nguồn event ở đây là **nhiều process** (17 service backend-go), không phải 1 process — nên không thể dùng lại nguyên `registerTraceSink` in-process. Cần đúng pattern "N publisher → NATS → mỗi replica `api-gateway` tự subscribe → fan-out in-process tới SSE client của replica đó" mà CR-FFT-002 đã chuẩn bị sẵn nguồn, và `notification-service/internal/adapter/broadcaster/broadcaster.go` đã có tiền lệ chạy thật cho đúng hình dạng bài toán này (khác ở chỗ broadcaster đó fan-out theo `tenantID+userID` cho WS notification cá nhân hoá; trace event là public/không cần định danh người nhận — chỉ cần "mọi client SSE đang mở kết nối trên replica này").

## Giải pháp đề xuất

### 1. `traceBroadcast` — fan-out registry đơn giản hơn `notification-service`'s Broadcaster

Không cần khoá theo `tenantID+userID` (trace event là "intentionally low-security... diagnostic, not sensitive" — theo đúng chính comment gốc trong `trace_routes.go:19-21` và bản TS `trace-sse-routes.ts:63`) — mọi client SSE đã pass `isAuthorized` nhận **toàn bộ** event, giống hệt bản TS cũ (`clients.add(res)` không phân biệt ai gọi).

```go
// backend-go/services/api-gateway/internal/adapter/httpgateway/trace_broadcast.go
type traceBroadcast struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{} // already-JSON-encoded TraceEvent bytes
}

func (b *traceBroadcast) subscribe() (<-chan []byte, func())
func (b *traceBroadcast) publish(raw []byte) // best-effort, drop on full channel — mirrors ClientEventBus.Publish (wscompat/channels_push.go) and Broadcaster.Broadcast's "slow reader is dropped, never blocks" rule
```

Cấu trúc gần như y hệt `wscompat.ClientEventBus` (`channels_push.go:80-116`, đã có sẵn trong cùng service) hơn là `notification-service`'s Broadcaster (vì không cần key theo user) — tái dùng đúng shape đã proven, đặt trong `httpgateway` package vì đây là route-local, không phải WS channel.

### 2. `mountTraceRoutes` nhận thêm 1 tham số: nguồn subscribe

```go
func mountTraceRoutes(mux chi.Router, broadcast *traceBroadcast) {
	mux.Get("/api/trace-stream", func(w http.ResponseWriter, r *http.Request) {
		...
		ch, unsubscribe := broadcast.subscribe()
		defer unsubscribe()
		...
		for {
			select {
			case <-ctx.Done():
				return
			case raw := <-ch:
				if _, err := w.Write(append(append([]byte("data: "), raw...), '\n', '\n')); err != nil {
					return
				}
				flusher.Flush()
			case <-ticker.C:
				if _, err := w.Write([]byte(": heartbeat\n\n")); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	})
}
```

Heartbeat **giữ nguyên** (không xoá) — vẫn cần để giữ kết nối qua nginx/load-balancer timeout đúng như F40's acceptance criteria gốc, kể cả khi có event thật chảy qua (event thưa hơn 15s là bình thường).

### 3. Composition root: subscribe NATS 1 lần khi service khởi động, fan-in vào `traceBroadcast`

```go
// main.go, sau khi eventbus.Connect (CR-FFT-002 đã thêm)
broadcast := &traceBroadcast{subs: make(map[chan []byte]struct{})}
go func() {
	_ = cons.SubscribeEphemeral(ctx, "TRACE", "orca.*.trace.span", func(ctx context.Context, event eventbus.Event) error {
		broadcast.publish(event.Payload) // event.Payload already the F40 TraceEvent JSON — see CR-FFT-002
		return nil
	})
}()
mountTraceRoutes(router, broadcast)
```

Dùng `SubscribeEphemeral` (không phải `Subscribe`) — mỗi `api-gateway` replica cần **tự** nhận full copy mọi span event để fan-out cho SSE client của chính replica đó (không phải competing-consumer, theo đúng phân biệt `eventbus.go:134-150`'s doc comment và tiền lệ `notification-service/broadcaster.go`).

## Changes Required

| File | Thay đổi |
|------|---------|
| `backend-go/services/api-gateway/internal/adapter/httpgateway/trace_routes.go` | Xoá đoạn TODO comment (dòng 24-30); `mountTraceRoutes` nhận thêm `broadcast *traceBroadcast`; thêm nhánh `case raw := <-ch` vào `select` hiện có |
| `backend-go/services/api-gateway/internal/adapter/httpgateway/trace_broadcast.go` (mới) | `traceBroadcast` type — fan-out in-process, không khoá theo user, best-effort drop |
| `backend-go/services/api-gateway/cmd/server/main.go` | Sau khi có `cons` (từ CR-FFT-002's `eventbus.Connect`), spawn goroutine `cons.SubscribeEphemeral(...)` fan-in vào `traceBroadcast`; truyền `broadcast` vào `mountTraceRoutes` |

## Không thuộc phạm vi CR này

- Đổi cơ chế auth của endpoint (`isAuthorized`-tương-đương hiện có trong `trace_routes.go`, chưa đọc chi tiết trong CR này vì không đổi) — giữ nguyên chính sách "intentionally low-security" đã có.
- Buffer/replay event đã publish trước khi client connect (giống bản TS cũ, event chỉ chảy tới client **đang mở** kết nối tại thời điểm publish — client connect muộn bỏ lỡ event trước đó, chấp nhận được cho use-case debug real-time).
- Giới hạn số lượng client SSE đồng thời / rate-limit riêng cho endpoint này — ngoài scope, dùng chung rate-limit tầng ngoài nếu có.

## Tiêu chí chấp nhận

- [ ] Mở TracePanel (kết nối `/api/trace-stream`), gọi 1 request bất kỳ qua `api-gateway` → event `start`/`ok` (hoặc `fail` nếu request lỗi) xuất hiện trên TracePanel trong vòng vài trăm ms (độ trễ NATS round-trip), không còn "live-but-empty".
- [ ] Heartbeat mỗi 15s vẫn hoạt động khi không có event thật nào trong khoảng đó (không regress F40's acceptance criteria gốc).
- [ ] Nhiều client SSE mở đồng thời đều nhận cùng 1 event (broadcast, không phải round-robin).
- [ ] Client ngắt kết nối (đóng tab) → `unsubscribe()` được gọi, không leak goroutine/channel (verify bằng test mở/đóng lặp lại nhiều lần, kiểm `len(broadcast.subs)` về 0).
- [ ] Dòng TODO comment gốc (`trace_routes.go:24-30`) được xoá, thay bằng comment mô tả cơ chế thật (theo đúng "why" convention của AGENTS.md).

## Impact analysis (gitnexus)

| Symbol | Direction | Risk | Impacted | Ghi chú |
|---|---|---|---|---|
| `mountTraceRoutes` (`backend-go/services/api-gateway/internal/adapter/httpgateway/trace_routes.go`) | upstream | 🟢 LOW | 3 (1 direct — `run` trong `api-gateway/cmd/server/main.go`) | An toàn để đổi signature (thêm tham số `broadcast`) — chỉ 1 call site, đã xác nhận qua `impact()` trước khi thiết kế CR này |

## Liên quan

- [F40-full-flow-tracing.md](../../../features/F40-full-flow-tracing.md) — mục "Backend SSE Stream (`/api/trace-stream`)", "Heartbeat: Server gửi SSE comment `: heartbeat` mỗi 15 giây"
- [CR-TRACE-000](../../v2/full-flow-tracing/CR-TRACE-000-tracing-rollout-overview.md) — convention id/wire-envelope cho kiến trúc TS Electron cũ; CR này chỉ tiêu thụ `TraceEvent` JSON đã chuẩn hoá ở CR-FFT-002, không định nghĩa lại convention riêng
- `backend/src/server/trace-sse-routes.ts` (tham chiếu wire behavior gốc, KHÔNG sửa — service `backend/` đang bị thay thế)
- `frontend/src/shared/trace/browser.ts:41-48` (`startSseClient`, không đổi)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_push.go` (`ClientEventBus`, dòng 80-116 — pattern fan-out in-process tái dùng)
- `backend-go/services/notification-service/internal/adapter/broadcaster/broadcaster.go` (tiền lệ `SubscribeEphemeral` → fan-out per-replica)
- `backend-go/docs/execution-plan.md` dòng 465-479
- [CR-FFT-001](./CR-FFT-001-otel-span-instrumentation.md), [CR-FFT-002](./CR-FFT-002-span-to-nats-trace-event-bridge.md) — phụ thuộc cứng
- [README.md](./README.md)
