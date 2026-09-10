# TASK-BE-FFT-009: `traceBroadcast` — fan-out registry mới (file mới)

**Solution:** BE-FFT-SOL-003 | **CR:** [CR-FFT-003](../../../../../../docs/crs/v4/full-flow-tracing/CR-FFT-003-api-gateway-sse-real-forwarding.md)
**Service:** `api-gateway` (`internal/adapter/httpgateway`)
**Depends on:** Không (type mới, độc lập cấu trúc — nhưng cả CR-FFT-002 nên DONE trước khi chuỗi có ý nghĩa end-to-end)
**Status:** ✅ DONE (2026-09-09)

---

## Mục tiêu

Registry fan-out in-process, đơn giản hơn `notification-service`'s
Broadcaster (không cần khoá theo `tenantID+userID` — trace event là
public/không định danh người nhận theo đúng "intentionally low-security"
đã ghi trong `trace_routes.go`).

## gitnexus

Symbol mới — không cần `impact()` trước khi tạo. Đọc lại
`internal/adapter/wscompat/channels_push.go:80-116`'s `ClientEventBus`
trước khi viết (mẫu tái dùng đã xác nhận đúng shape khi viết solution).

## Files cần sửa

1. `backend-go/services/api-gateway/internal/adapter/httpgateway/trace_broadcast.go` (MỚI)

## Nội dung

```go
package httpgateway

import "sync"

// traceBroadcast fans a stream of already-JSON-encoded F40 TraceEvent
// bytes out to every locally-connected SSE client on THIS api-gateway
// replica. Simpler than wscompat.ClientEventBus (channels_push.go:80-116)
// because trace data has no per-user routing — every authorized SSE
// client receives every event, matching the old backend's
// registerTraceSink fan-out (trace-sse-routes.ts) and this endpoint's own
// "intentionally low-security... diagnostic, not sensitive" stance
// (see trace_routes.go's doc comment).
type traceBroadcast struct {
    mu   sync.Mutex
    subs map[chan []byte]struct{}
}

func newTraceBroadcast() *traceBroadcast {
    return &traceBroadcast{subs: make(map[chan []byte]struct{})}
}

// subscribe registers a new SSE client, returning a receive-only channel
// and an unsubscribe func the caller MUST invoke exactly once (typically
// via defer) when the client disconnects — otherwise the channel and its
// registry entry leak.
func (b *traceBroadcast) subscribe() (<-chan []byte, func()) {
    ch := make(chan []byte, 16) // buffer matches wscompat.ClientEventBus's convention
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

// publish is best-effort — mirrors ClientEventBus.Publish and
// notification-service's Broadcaster.Broadcast: a slow/stalled SSE client
// is dropped for that one event, never allowed to block delivery to
// other clients.
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

## Test cases cần cover

File mới: `backend-go/services/api-gateway/internal/adapter/httpgateway/trace_broadcast_test.go`.

- `TestTraceBroadcast_PublishReachesAllSubscribers` — 2+ subscriber, publish
  1 message, cả 2 đều nhận đúng byte đó.
- `TestTraceBroadcast_SlowSubscriberDropsWithoutBlockingOthers` — 1
  subscriber không đọc channel (buffer đầy), publish nhiều message, xác
  nhận subscriber còn lại vẫn nhận được message mới, `publish()` không
  block/deadlock.
- `TestTraceBroadcast_UnsubscribeRemovesFromRegistry` — subscribe rồi
  unsubscribe N lần lặp lại, xác nhận `len(b.subs) == 0` sau mỗi lần (đúng
  tiêu chí chấp nhận CR-FFT-003: "không leak goroutine/channel").
- `TestTraceBroadcast_UnsubscribeIsIdempotent` — gọi `unsubscribe()` 2 lần
  liên tiếp, không panic (double-close protection đã có trong code qua
  `if _, ok := b.subs[ch]; ok`).
- `TestTraceBroadcast_ConcurrentSubscribePublish` (`-race`) — nhiều
  goroutine subscribe/publish/unsubscribe đồng thời, không race, không
  panic.

## Verify

```bash
cd backend-go/services/api-gateway && go build ./... && go test -race ./internal/adapter/httpgateway/...
gofmt -l internal/adapter/httpgateway/trace_broadcast.go internal/adapter/httpgateway/trace_broadcast_test.go
```

## gitnexus (sau khi sửa)

```
mcp__gitnexus__detect_changes({ scope: "compare", base_ref: "main", repo: "orca" })
```
Xác nhận scope chỉ gồm 2 file mới — chưa ai import `traceBroadcast` (đó là
TASK-BE-FFT-010/011).

## Blocking

TASK-BE-FFT-010 (`mountTraceRoutes` nhận tham số `*traceBroadcast`) và
TASK-BE-FFT-011 (`main.go` khởi tạo + publish vào registry này) phụ thuộc
CỨNG vào task này.

## Kết quả thực tế (2026-09-09)

- Symbol hoàn toàn mới, không cần `impact()`. Đối chiếu
  `wscompat/channels_push.go`'s `ClientEventBus` (dòng 80-116) trước khi
  viết — shape khớp 100% với mô tả task doc (`sync.Mutex` + `map[chan
  T]struct{}`, `subscribe`/`unsubscribe` với double-close guard,
  `publish` dùng `select`/`default` để không block).
- Tạo đúng 1 file mới `trace_broadcast.go` theo đúng nội dung task doc,
  không sửa gì khác.
- File test mới `trace_broadcast_test.go` — 5 test đúng yêu cầu: fan-out
  tới nhiều subscriber, slow subscriber bị drop không chặn subscriber
  khác (buffer 16 đầy, publish 32 message vẫn không deadlock), registry
  rỗng sau mỗi lần unsubscribe (lặp 5 lần), unsubscribe gọi 2 lần không
  panic, và test concurrent subscribe/publish/unsubscribe chạy với
  `-race`.
- `go test -race ./internal/adapter/httpgateway/... -run TestTraceBroadcast -v`:
  5/5 PASS. Chạy lại toàn bộ package (`go test -race
  ./internal/adapter/httpgateway/...`): PASS, không regression.
- `gofmt -l`: sạch.
- Scope xác nhận qua `git status --porcelain`: đúng 2 file mới, chưa ai
  import `traceBroadcast` (đúng dự kiến — đó là TASK-BE-FFT-010/011).

## Amendment (2026-09-09, từ TASK-BE-FFT-010)

`traceBroadcast`/`newTraceBroadcast`/`subscribe`/`publish` đã được **export**
thành `TraceBroadcast`/`NewTraceBroadcast`/`Subscribe`/`Publish` khi thực thi
TASK-BE-FFT-010, vì TASK-BE-FFT-011's sketch cần gọi
`httpgateway.NewTraceBroadcast()`/`broadcast.Publish(...)` từ `main.go`
(package khác). Rename thực hiện thủ công trong đúng 2 file
(`trace_broadcast.go`/`trace_broadcast_test.go`), xác nhận an toàn trước
đó bằng grep toàn package: 0 caller ngoài 2 file này. Hành vi/logic không
đổi, chỉ đổi tên. Xem TASK-BE-FFT-010's "Kết quả thực tế" để biết chi tiết
đầy đủ.
