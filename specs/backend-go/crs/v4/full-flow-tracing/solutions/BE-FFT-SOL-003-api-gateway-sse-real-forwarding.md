# BE-FFT-SOL-003: `api-gateway` forward trace event thật qua `/api/trace-stream`

> **🔲 Designed — chưa implement.** Phụ thuộc cứng BE-FFT-SOL-001, BE-FFT-SOL-002.

**CR:** [CR-FFT-003](../../../../../../docs/crs/v4/full-flow-tracing/CR-FFT-003-api-gateway-sse-real-forwarding.md)
**Service:** `api-gateway` (`internal/adapter/httpgateway`, `cmd/server/main.go`)
**TDD tham chiếu:** không có TDD riêng — đối chiếu trực tiếp `backend/src/server/trace-sse-routes.ts` (tham chiếu wire behavior gốc, không sửa)

---

## 1. Re-verify trạng thái hiện tại (2026-09-09) — TODO comment xác nhận nguyên văn

Đọc trực tiếp `backend-go/services/api-gateway/internal/adapter/httpgateway/trace_routes.go`:

- Dòng 24-30 — TODO comment **khớp 100% nguyên văn** với trích dẫn trong CR-FFT-003:
  ```go
  // Known gap: this only keeps the connection alive (heartbeats) — it does
  // NOT forward any real trace/debug events yet, since backend-go has no
  // equivalent to the old backend's global registerTraceSink() fan-out.
  // TracePanel will show a live-but-empty stream rather than the 404 that
  // was breaking EventSource's connection state before this existed. Wiring
  // real event forwarding (e.g. from common/eventbus) is tracked as a
  // follow-up in docs/execution-plan.md, not attempted here.
  ```
- `mountTraceRoutes(mux chi.Router)` (dòng 31-70) — xác nhận đúng: ghi
  `": connected\n\n"` 1 lần, sau đó vòng lặp `select` chỉ có 2 case
  (`ctx.Done()`, `ticker.C` mỗi 15s) — không có channel nhận event thật,
  không có branch thứ 3.
- `internal/adapter/httpgateway/router.go:107` — `mountTraceRoutes(r)` là
  **call site duy nhất** (khớp `impact()` LOW risk, 3 impacted/1 direct,
  xem mục 2).
- `internal/adapter/wscompat/channels_push.go:80-116`'s `ClientEventBus` —
  đọc lại xác nhận đúng shape gợi ý trong CR: `sync.Mutex` + `map[chan
  PushEvent]struct{}`, `Subscribe()`/`Publish()` với "slow subscriber —
  drop rather than block Publish" (dòng ~113-116) — pattern `traceBroadcast`
  tái dùng gần như y hệt, chỉ đổi payload type từ `PushEvent` (struct có
  `Channel`/`Args`) sang `[]byte` (JSON đã encode sẵn, vì `mountTraceRoutes`
  chỉ cần ghi thẳng ra `http.ResponseWriter`, không cần định tuyến theo
  channel name như wscompat).

## 2. Impact analysis — re-verify

| Symbol | Direction | Risk | Impacted | Ghi chú |
|---|---|---|---|---|
| `mountTraceRoutes` | upstream | 🟢 **LOW** | 3 (1 direct — `NewRouter` trong `router.go:107`, xuyên qua `run` trong `api-gateway/cmd/server/main.go`) | Khớp 100% với CR gốc — an toàn để đổi signature (thêm tham số `broadcast`) |

Symbol duy nhất chạm ở CR này **không** CRITICAL — khác hẳn CR-FFT-001/002.
`traceBroadcast` là symbol mới, không cần `impact()` trước khi tạo.

## 3. Giải pháp

### 3.1. `traceBroadcast` — fan-out registry (file mới)

```go
// backend-go/services/api-gateway/internal/adapter/httpgateway/trace_broadcast.go
package httpgateway

import "sync"

// traceBroadcast fans a stream of already-JSON-encoded F40 TraceEvent
// bytes out to every locally-connected SSE client on this api-gateway
// replica. Simpler than wscompat.ClientEventBus (channels_push.go:80-116)
// because trace data has no per-user routing — every authorized SSE
// client gets every event, matching the old backend's registerTraceSink
// fan-out (trace-sse-routes.ts) and this endpoint's own "intentionally
// low-security... diagnostic, not sensitive" stance.
type traceBroadcast struct {
    mu   sync.Mutex
    subs map[chan []byte]struct{}
}

func newTraceBroadcast() *traceBroadcast {
    return &traceBroadcast{subs: make(map[chan []byte]struct{})}
}

func (b *traceBroadcast) subscribe() (<-chan []byte, func()) {
    ch := make(chan []byte, 16)
    b.mu.Lock()
    b.subs[ch] = struct{}{}
    b.mu.Unlock()
    unsubscribe := func() {
        b.mu.Lock()
        if _, ok := b.subs[ch]; ok {
            delete(b.subs, ch)
            close(ch)
        }
        b.mu.Unlock()
    }
    return ch, unsubscribe
}

// publish is best-effort — a slow/stalled SSE client is dropped, never
// allowed to block delivery to other clients (mirrors ClientEventBus.Publish
// and notification-service's Broadcaster.Broadcast).
func (b *traceBroadcast) publish(raw []byte) {
    b.mu.Lock()
    defer b.mu.Unlock()
    for ch := range b.subs {
        select {
        case ch <- raw:
        default:
        }
    }
}
```

### 3.2. `mountTraceRoutes` — nhận thêm tham số, xoá TODO

```go
// backend-go/services/api-gateway/internal/adapter/httpgateway/trace_routes.go
func mountTraceRoutes(mux chi.Router, broadcast *traceBroadcast) {
    mux.Get("/api/trace-stream", func(w http.ResponseWriter, r *http.Request) {
        // ... phần method-check, flusher-check, header, ": connected\n\n" giữ nguyên ...

        ch, unsubscribe := broadcast.subscribe()
        defer unsubscribe()

        ticker := time.NewTicker(15 * time.Second)
        defer ticker.Stop()

        ctx := r.Context()
        for {
            select {
            case <-ctx.Done():
                return
            case raw := <-ch: // NEW — event thật
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

Comment TODO (dòng 24-30) bị xoá, thay bằng comment mô tả cơ chế thật (đúng
convention AGENTS.md — giải thích "why", không narrate lại code):

```go
// Forwards real F40 TraceEvent JSON received from traceBroadcast (fed by
// this replica's own NATS SubscribeEphemeral loop, see main.go) — each
// api-gateway replica independently fans NATS events out to its own
// locally-connected SSE clients (CR-FFT-002/003), same shape as
// notification-service's cross-replica broadcaster.
```

`router.go:107`'s call site cập nhật theo: `mountTraceRoutes(r,
deps.TraceBroadcast)` (hoặc tham số tương đương truyền qua `Deps`).

### 3.3. `main.go` — subscribe NATS 1 lần, fan-in vào `traceBroadcast`

```go
// api-gateway/cmd/server/main.go, sau khi có cons từ BE-FFT-SOL-002's eventbus.Connect
broadcast := httpgateway.NewTraceBroadcast() // hoặc constructor export tương đương
go func() {
    _ = cons.SubscribeEphemeral(ctx, "TRACE", "orca.*.trace.span", func(ctx context.Context, event eventbus.Event) error {
        broadcast.Publish(event.Payload) // event.Payload đã là F40 TraceEvent JSON (BE-FFT-SOL-002)
        return nil
    })
}()
```

`SubscribeEphemeral` (không phải `Subscribe`) — mỗi replica `api-gateway`
cần tự nhận **full copy** mọi span event để fan-out cho SSE client của
chính replica đó, đúng phân biệt đã re-verify ở `eventbus.go`'s doc comment.

## 4. Không thuộc phạm vi solution này

- Đổi cơ chế auth của endpoint — giữ nguyên "intentionally low-security".
- Buffer/replay event đã publish trước khi client connect — chấp nhận mất,
  giống bản TS cũ.
- Rate-limit riêng cho endpoint này.

## 5. Test cases bắt buộc

- `TestTraceBroadcast_PublishReachesAllSubscribers` — 2+ subscriber cùng
  nhận 1 event.
- `TestTraceBroadcast_SlowSubscriberDropsWithoutBlocking` — 1 subscriber
  không đọc channel, `publish()` vẫn không block cho subscriber khác.
- `TestTraceBroadcast_UnsubscribeRemovesFromRegistry` — mở/đóng lặp lại,
  `len(subs)` về 0 (verify không leak, đúng tiêu chí chấp nhận CR gốc).
- `TestMountTraceRoutes_ForwardsRealEventAsSSEData` — publish 1 event vào
  `broadcast`, xác nhận response body có đúng `data: <json>\n\n`.
- `TestMountTraceRoutes_HeartbeatStillFiresWithNoEvents` — không regress
  F40's acceptance criteria gốc.

## 6. Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| Phụ thuộc cứng BE-FFT-SOL-001, BE-FFT-SOL-002 | Cao | Không có gì để forward nếu 2 solution trước chưa xong |
| `mountTraceRoutes` risk thật | 🟢 LOW | Đã re-verify — an toàn đổi signature, 1 call site duy nhất |
| Goroutine leak nếu `unsubscribe()` không được gọi | Trung bình | Bắt buộc `defer unsubscribe()` trong handler — test riêng cho việc này |
| `SubscribeEphemeral` chưa từng chạy thật trong `api-gateway` (service mới nối NATS) | Trung bình | Cần test tích hợp có NATS thật (hoặc `testcontainers`/embedded NATS nếu repo đã có tiền lệ) trước khi coi solution này DONE |

## Liên quan

- `backend-go/services/api-gateway/internal/adapter/httpgateway/trace_routes.go` (TODO dòng 24-30)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go:107`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_push.go:80-116` (`ClientEventBus`, mẫu tái dùng)
- `backend-go/services/notification-service/internal/adapter/broadcaster/broadcaster.go` (tiền lệ `SubscribeEphemeral`)
- `backend/src/server/trace-sse-routes.ts` (tham chiếu wire behavior gốc, KHÔNG sửa)
- `frontend/src/shared/trace/browser.ts:41-48` (`startSseClient`, không đổi)
- [BE-FFT-SOL-001](./BE-FFT-SOL-001-otel-span-instrumentation.md), [BE-FFT-SOL-002](./BE-FFT-SOL-002-span-to-nats-trace-event-bridge.md) — phụ thuộc cứng
- [README.md](./README.md) — cảnh báo rủi ro CRITICAL tổng hợp
