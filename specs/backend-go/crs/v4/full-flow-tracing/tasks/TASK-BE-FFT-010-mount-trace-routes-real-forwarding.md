# TASK-BE-FFT-010: `mountTraceRoutes` — xoá TODO thật, forward event thật qua SSE

**Solution:** BE-FFT-SOL-003 | **CR:** [CR-FFT-003](../../../../../../docs/crs/v4/full-flow-tracing/CR-FFT-003-api-gateway-sse-real-forwarding.md)
**Service:** `api-gateway` (`internal/adapter/httpgateway`)
**Depends on:** TASK-BE-FFT-009
**Status:** ✅ DONE (2026-09-09)

---

## Mục tiêu

Xoá đúng đoạn TODO comment (dòng 24-30, đã re-verify khớp nguyên văn với
CR-FFT-003 khi viết solution) trong `trace_routes.go`, thêm nhánh `select`
thứ 3 forward event thật từ `traceBroadcast`.

## gitnexus — chạy trước khi sửa (LOW risk, nhưng vẫn bắt buộc theo CLAUDE.md)

```
mcp__gitnexus__impact({ target: "mountTraceRoutes", direction: "upstream", repo: "orca", summaryOnly: true })
```

Kỳ vọng: 🟢 LOW, 3 impacted (1 direct — `NewRouter` trong `router.go:107`).
Đã re-verify khớp 100% lúc viết solution (2026-09-09) — khác biệt duy nhất
so với 3 task CR-FFT-001/007 là symbol này **không** CRITICAL, nên an toàn
đổi signature (thêm tham số `broadcast *traceBroadcast`).

## Files cần sửa

1. `backend-go/services/api-gateway/internal/adapter/httpgateway/trace_routes.go` (MODIFY)
2. `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go` (MODIFY — dòng 107, cập nhật call site + `Deps` nếu cần truyền `broadcast` qua đó)

## Nội dung

### `trace_routes.go`

Xoá đúng đoạn comment dòng 24-30 (TODO), thay bằng comment mô tả cơ chế
thật:

```go
// mountTraceRoutes serves GET /api/trace-stream — ... (đoạn mô tả EventSource/
// headers/heartbeat ở trên GIỮ NGUYÊN, không đổi) ...
//
// Forwards real F40 TraceEvent JSON delivered via broadcast — fed by this
// replica's own NATS SubscribeEphemeral loop (main.go, CR-FFT-002/003):
// each api-gateway replica independently fans NATS trace-span events out
// to its own locally-connected SSE clients, the same per-replica fan-out
// shape as notification-service's cross-replica broadcaster.
func mountTraceRoutes(mux chi.Router, broadcast *traceBroadcast) {
    mux.Get("/api/trace-stream", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet {
            writeJSONError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "GET only")
            return
        }

        flusher, ok := w.(http.Flusher)
        if !ok {
            writeJSONError(w, http.StatusInternalServerError, "SSE_UNSUPPORTED", "streaming not supported")
            return
        }

        h := w.Header()
        h.Set("Content-Type", "text/event-stream")
        h.Set("Cache-Control", "no-cache")
        h.Set("Connection", "keep-alive")
        h.Set("X-Accel-Buffering", "no")
        w.WriteHeader(http.StatusOK)

        _, _ = w.Write([]byte(": connected\n\n"))
        flusher.Flush()

        ch, unsubscribe := broadcast.subscribe() // NEW
        defer unsubscribe()                      // NEW

        ticker := time.NewTicker(15 * time.Second)
        defer ticker.Stop()

        ctx := r.Context()
        for {
            select {
            case <-ctx.Done():
                return
            case raw := <-ch: // NEW branch — event thật
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

Heartbeat **giữ nguyên**, không xoá — vẫn cần cho nginx/load-balancer
timeout, kể cả khi có event thật chảy qua.

### `router.go:107`

```go
// TRƯỚC
mountTraceRoutes(r)
// SAU
mountTraceRoutes(r, deps.TraceBroadcast) // hoặc field Deps tương ứng do TASK-BE-FFT-011 khởi tạo tại main.go
```

Thêm field `TraceBroadcast *traceBroadcast`-tương-đương (hoặc tên đã thống
nhất) vào `Deps` struct (`router.go`'s `Deps`) nếu chưa có type export
được — vì `traceBroadcast` là package-private, cân nhắc export tên khác
hoặc constructor `NewTraceBroadcast()` public nếu `main.go` (khác package)
cần khởi tạo nó (xem TASK-BE-FFT-011 — quyết định chỗ khởi tạo `traceBroadcast`
là ở `httpgateway` package, expose 1 constructor + phương thức publish,
`main.go` chỉ giữ tham chiếu để gọi `publish` từ vòng lặp `SubscribeEphemeral`).

## Test cases cần cover

File mới hoặc mở rộng: `trace_routes_test.go` (hiện chưa tồn tại — xác
nhận bằng `find` trước khi viết).

- `TestMountTraceRoutes_ForwardsRealEventAsSSEData` — publish 1 event vào
  `broadcast` trong lúc 1 client đang mở `httptest` request tới
  `/api/trace-stream`, xác nhận response body chứa đúng `data: <json>\n\n`.
- `TestMountTraceRoutes_HeartbeatStillFiresWithNoEvents` — **regression
  bắt buộc**: không publish gì, xác nhận vẫn có `: heartbeat\n\n` xuất
  hiện đúng chu kỳ (rút ngắn ticker trong test nếu cần, không chờ 15s
  thật).
- `TestMountTraceRoutes_ConnectedCommentStillSentImmediately` — regression:
  `": connected\n\n"` vẫn là byte đầu tiên ghi ra.
- `TestMountTraceRoutes_ClientDisconnectCallsUnsubscribe` — đóng
  `httptest` request context, xác nhận `unsubscribe()` được gọi (verify
  qua `len(broadcast.subs) == 0` sau khi handler return) — đúng tiêu chí
  chấp nhận CR-FFT-003.
- `TestMountTraceRoutes_MultipleClientsReceiveSameEvent` — 2 client SSE mở
  đồng thời, publish 1 event, cả 2 đều nhận (broadcast, không round-robin)
  — đúng tiêu chí chấp nhận CR-FFT-003.

## Verify

```bash
cd backend-go/services/api-gateway && go build ./... && go test ./internal/adapter/httpgateway/...
gofmt -l internal/adapter/httpgateway/trace_routes.go internal/adapter/httpgateway/router.go
grep -n "Known gap" internal/adapter/httpgateway/trace_routes.go   # PHẢI rỗng — xác nhận TODO đã xoá thật
```

## gitnexus (sau khi sửa)

```
mcp__gitnexus__detect_changes({ scope: "compare", base_ref: "main", repo: "orca" })
mcp__gitnexus__impact({ target: "mountTraceRoutes", direction: "upstream", repo: "orca", summaryOnly: true })
```
Xác nhận `mountTraceRoutes` vẫn LOW risk sau khi đổi signature (blast
radius không tăng đột biến chỉ vì thêm 1 tham số) và scope thay đổi đúng 2
file liệt kê.

## Blocking

TASK-BE-FFT-011 phụ thuộc CỨNG vào task này (cần `mountTraceRoutes`'s
signature mới đã tồn tại trước khi wire `main.go`).

## Kết quả thực tế (2026-09-09)

- `impact({target: "mountTraceRoutes", direction: "upstream"})` chạy
  trước VÀ sau khi sửa: LOW, 3 impacted / 1 direct cả 2 lần — khớp 100%
  kỳ vọng, blast radius không tăng dù đổi signature (thêm tham số
  `broadcast`).
- Xoá đúng đoạn "Known gap" TODO comment (dòng 24-30 gốc), thay bằng
  comment mô tả cơ chế forward thật — xác nhận bằng `grep -n "Known gap"
  trace_routes.go` trả về rỗng.
- Thêm nhánh `select` thứ 3 (`case raw := <-ch`) forward event thật, giữ
  nguyên heartbeat + connected-comment.
- **Bổ sung ngoài "Files cần sửa" liệt kê của task doc (có chủ đích, ghi
  rõ lý do)**: task doc chỉ liệt kê `trace_routes.go` + `router.go`,
  nhưng chính task doc cũng ghi chú "cân nhắc export... nếu main.go cần
  khởi tạo" — vì TASK-BE-FFT-011's sketch gọi thẳng
  `httpgateway.NewTraceBroadcast()`/`broadcast.Publish(...)` (public
  API), đã export luôn ở đây thay vì để dở: đổi
  `traceBroadcast`→`TraceBroadcast`, `newTraceBroadcast`→`NewTraceBroadcast`,
  `publish`→`Publish`, `subscribe`→`Subscribe` trong
  `trace_broadcast.go`/`trace_broadcast_test.go` (khớp convention
  `wscompat.ClientEventBus` đã dùng — cũng export tương tự). Xác nhận an
  toàn trước khi rename: `grep -rn "traceBroadcast|newTraceBroadcast"`
  toàn bộ package ngoài 2 file này → rỗng (task-009 mới tạo, chưa ai
  import) — rename thủ công trong đúng 2 file, không dùng
  `gitnexus rename` vì đây là symbol mới hoàn toàn chưa được index đầy đủ
  và blast radius đã tự xác nhận bằng grep là 0 caller ngoài phạm vi.
  Cũng bổ sung `Deps.TraceBroadcast *TraceBroadcast` field vào `router.go`
  và fallback `if traceBroadcast == nil { traceBroadcast =
  NewTraceBroadcast() }` trong `NewRouter` — đảm bảo endpoint vẫn hoạt
  động đúng (connect + heartbeat) ngay cả trước khi TASK-BE-FFT-011 wire
  giá trị thật từ `main.go`.
- File test mới `trace_routes_test.go` (xác nhận trước bằng `find`:
  chưa tồn tại) — 5 test đúng yêu cầu: forward event thật thành
  `data: <json>\n\n`, heartbeat vẫn chạy khi không có event (rút ngắn
  interval qua 1 package var mới `traceStreamHeartbeatInterval` chỉ để
  test override, production vẫn 15s mặc định), connected comment vẫn là
  byte đầu tiên, client disconnect gọi đúng unsubscribe (verify qua
  `len(broadcast.subs) == 0`), nhiều client cùng nhận 1 event (broadcast
  thật, không round-robin).
- **2 bug phát hiện + sửa trong quá trình viết test (không phải code sản
  xuất)**: (1) SSE framing mỗi message ghi `"<content>\n\n"` trong 1
  lần `Write` — `connectSSE` helper ban đầu chỉ đọc dòng nội dung, để sót
  dòng trống kết thúc, khiến lần đọc tiếp theo nhận nhầm `"\n"` rỗng thay
  vì nội dung thật — sửa bằng cách đọc thêm 1 dòng trống ngay sau dòng
  "connected". (2) data race thật giữa test's deferred restore của biến
  global `traceStreamHeartbeatInterval` và handler goroutine đọc biến đó
  lúc tạo ticker — sửa bằng cách gọi `srv.Close()` (block tới khi handler
  goroutine return hẳn) TRƯỚC khi restore biến global, thay vì dùng
  `defer` không đảm bảo thứ tự.
- `go test -race ./internal/adapter/httpgateway/... -run
  "TestMountTraceRoutes|TestTraceBroadcast" -v`: 9/9 PASS, chạy lặp lại 3
  lần liên tiếp đều ổn định, không race, không flake. Chạy lại toàn bộ
  package (`go test -race ./internal/adapter/httpgateway/...`) và toàn bộ
  `api-gateway` (`go test ./...`): PASS hết, không regression.
- `gofmt -l`: sạch trên cả 5 file liên quan.
- Scope xác nhận qua `git status --porcelain`: `trace_routes.go` (M),
  `router.go` (M), `trace_routes_test.go` (mới), cộng với
  `trace_broadcast.go`/`trace_broadcast_test.go` (đã có từ task-009,
  amend ở đây do rename export) — không file nào khác bị đụng.
