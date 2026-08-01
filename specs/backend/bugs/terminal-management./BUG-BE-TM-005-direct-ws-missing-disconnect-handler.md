# BUG-BE-TM-005: DevServerRelayBridge.directWebSocket không xử lý disconnect — session bị leak

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-TRM-004  
**Note:** dev-server-relay-bridge.ts: mux.onDispose() added  

## Mức độ: HIGH

## Tóm tắt

Trong `connectDirectWebSocket()` (direct-websocket mode), khi Agent kết nối thành công, không có listener `mux.onDispose()` được đăng ký để reset `this.session = null` khi Agent disconnect. Điều này khác với `connectWithExternalToken()` đã có xử lý đúng.

## File liên quan

- [`src/main/dev-server/dev-server-relay-bridge.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/dev-server/dev-server-relay-bridge.ts) — Lines 204-262 (connectDirectWebSocket) vs. Lines 293-333 (connectWithExternalToken)

## So sánh code

**`connectWithExternalToken` — CÓ disconnect handler (ĐÚNG):**
```typescript
// Lines 304-310
mux.onDispose(() => {
  if (this.session === mux) {
    console.log('[DevServerRelayBridge] Agent WS closed — clearing session')
    this.session = null
    this.onSessionDropped()  // ← queue subsequent calls
  }
})
```

**`connectDirectWebSocket` — THIẾU disconnect handler (SAI):**
```typescript
// Lines 220-233 — không có mux.onDispose()!
(mux, info) => {
  this._directWsDisposer = null
  this.session = mux
  // ← MISSING: mux.onDispose to reset session on disconnect
  if (opts.testOnly) {
    void this.disconnect()
  }
  resolve({ ... })
}
```

## Ảnh hưởng

Khi Agent ngắt kết nối (restart, network drop):
1. `this.session` vẫn trỏ đến `mux` đã chết (dead reference)
2. `isAlive()` trả về `true` → pool nghĩ connection vẫn active
3. Các call mới sẽ gọi `session.request()` trên mux đã dispose → throw "Connection lost"
4. Không kích hoạt reconnect queue (`_reconnecting = false`) → caller nhận error ngay lập tức
5. `connectDirectWebSocket()` không được gọi lại → **không auto-reconnect**

## Cách fix

Thêm `mux.onDispose()` trong `connectDirectWebSocket`, giống hệt pattern của `connectWithExternalToken`:

```typescript
(mux, info) => {
  this._directWsDisposer = null
  this.session = mux

  // ADD: disconnect handler
  mux.onDispose(() => {
    if (this.session === mux) {
      this.session = null
      this.onSessionDropped()
    }
  })
  
  resolve({ ... })
}
```

## Liên quan đến luồng

- **Pre-condition**: Dev Server Agent phải duy trì kết nối liên tục.
- **BL-TM-01**: Bước 4 — Relay Routing, `FAIL 'Not connected'`.
