# TASK-001: Tách `dispatchCtx` và `writeCtx` trong `handleInvoke` và `handleSend`

**From Solution:** SOL-001  
**Priority:** P0 — thực thi TRƯỚC TẤT CẢ tasks khác  
**Service:** `api-gateway`  
**File:** `services/api-gateway/internal/adapter/wscompat/handler.go`  
**Status:** `[x]` DONE

---

## Context

Hiện tại `handleInvoke` dùng chung một `ctx` với `invokeTimeout=25s` cho cả
dispatch lẫn write. Khi dispatch timeout hoặc parent ctx bị cancel, `wsjson.Write`
được gọi với ctx đã expired → frame bị drop silently → frontend thấy timeout 30s.

---

## Thay đổi cần thực hiện

**File:** `services/api-gateway/internal/adapter/wscompat/handler.go`

### Bước 1: Thêm constant `writeTimeout` sau `invokeTimeout`

Tìm dòng:
```go
const invokeTimeout = 25 * time.Second
```

Thêm ngay bên dưới:
```go
// writeTimeout is the deadline for sending a single WS response frame back
// to the client. Kept short: by the time we reach a write, the dispatch is
// already done; a 5s window is generous for a single JSON frame over a
// local/LAN WebSocket connection. Deliberately independent of invokeTimeout
// so a timed-out or cancelled dispatch context does not silently drop the
// response (the original bug — BUG-001).
const writeTimeout = 5 * time.Second
```

### Bước 2: Refactor `handleInvoke`

Thay toàn bộ function `handleInvoke` (hiện tại lines 127–140 trong handler.go):

```go
func (h *Handler) handleInvoke(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, identity Identity, msg InboundMessage) {
	dispatchCtx, dispatchCancel := context.WithTimeout(ctx, invokeTimeout)
	defer dispatchCancel()

	result, err := h.Registry.Dispatch(dispatchCtx, identity, msg.Channel, msg.Args)

	// Attempt to acquire writeMu; log if we have to wait significantly
	// (indicates concurrent timeout contention — see BUG-004 Cause B).
	lockStart := time.Now()
	writeMu.Lock()
	defer writeMu.Unlock()
	if waited := time.Since(lockStart); waited > 100*time.Millisecond {
		h.Logger.WarnContext(context.Background(), "wscompat: writeMu contention detected",
			slog.String("channel", msg.Channel),
			slog.Duration("lock_wait", waited))
	}

	// Use a fresh context for the write so a cancelled or timed-out
	// dispatchCtx does not silently drop the error or result frame.
	// context.Background() is intentional: the write must succeed even if
	// the parent HTTP request context has been cancelled (e.g. proxy
	// timeout, client navigation) — the WS connection itself is still open.
	writeCtx, writeCancel := context.WithTimeout(context.Background(), writeTimeout)
	defer writeCancel()

	if err != nil {
		_ = wsjson.Write(writeCtx, conn, ErrorMessage{Type: "error", ID: msg.ID, Message: err.Error()})
		return
	}
	_ = wsjson.Write(writeCtx, conn, ResultMessage{Type: "result", ID: msg.ID, Result: result})
}
```

### Bước 3: Refactor `handleSend`

Thay toàn bộ function `handleSend`:

```go
func (h *Handler) handleSend(ctx context.Context, identity Identity, msg InboundMessage) {
	dispatchCtx, cancel := context.WithTimeout(ctx, invokeTimeout)
	defer cancel()

	var args []json.RawMessage
	if len(msg.Data) > 0 {
		args = []json.RawMessage{msg.Data}
	}
	if _, err := h.Registry.Dispatch(dispatchCtx, identity, msg.Channel, args); err != nil {
		// Log with background ctx so the log entry is not dropped if the
		// HTTP request ctx was cancelled before the dispatch finished.
		h.Logger.WarnContext(context.Background(), "wscompat: send channel failed",
			slog.String("channel", msg.Channel), slog.Any("error", err))
	}
}
```

---

## Kiểm tra import

Đảm bảo `handler.go` đã có import `"log/slog"`. Nếu chưa có, thêm vào block import.

---

## Verify sau khi thay đổi

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca/backend-go/services/api-gateway
go build ./internal/adapter/wscompat/...
go vet ./internal/adapter/wscompat/...
go test ./internal/adapter/wscompat/... -count=1 -v
```

Expected: build thành công, tất cả tests hiện có vẫn pass.
