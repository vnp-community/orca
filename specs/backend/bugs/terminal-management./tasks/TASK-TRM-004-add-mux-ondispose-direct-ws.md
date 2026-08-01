# TASK-TRM-004: Thêm mux.onDispose() trong connectDirectWebSocket

**Priority:** 🔴 HIGH — session leak khi Agent disconnect (direct-ws mode)  
**Effort:** ~10 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-BE-TM-005  
**Solution ref:** [SOLUTION-TRM-BE-exact.md](../solutions/SOLUTION-TRM-BE-exact.md)

---

## Mục tiêu

Thêm `mux.onDispose()` handler trong `connectDirectWebSocket()` callback để clear session khi Agent disconnect, giống pattern đã có trong `connectWithExternalToken()`.

## File cần sửa

```
src/main/dev-server/dev-server-relay-bridge.ts
```

## Thay đổi cụ thể

Tìm `onConnected callback` trong method `connectDirectWebSocket()` (khoảng line 220–233):

**TRƯỚC (thiếu onDispose):**
```typescript
(mux, info) => {
  this._directWsDisposer = null
  this.session = mux

  if (opts.testOnly) {
    void this.disconnect()
  }

  resolve({
    platform: (info.platform as NodeJS.Platform) ?? 'linux',
    arch: info.arch,
    nodeVersion: info.nodeVersion,
    relayVersion: info.agentVersion,
  })
},
```

**SAU (với onDispose):**
```typescript
(mux, info) => {
  this._directWsDisposer = null
  this.session = mux

  // FIX BE-TM-005: clear session when agent disconnects (pattern from connectWithExternalToken lines 304-310)
  mux.onDispose(() => {
    if (this.session === mux) {
      console.log(`[DevServerRelayBridge] Agent WS closed — clearing session (direct-ws mode)`)
      this.session = null
      this.onSessionDropped()
    }
  })

  if (opts.testOnly) {
    void this.disconnect()
  }

  resolve({
    platform: (info.platform as NodeJS.Platform) ?? 'linux',
    arch: info.arch,
    nodeVersion: info.nodeVersion,
    relayVersion: info.agentVersion,
  })
},
```

## Reference pattern

Xem `connectWithExternalToken()` tại khoảng lines 304–310 — đây là pattern đã hoạt động đúng cần copy:
```typescript
mux.onDispose(() => {
  if (this.session === mux) {
    console.log(`[DevServerRelayBridge] Agent WS closed — clearing session (direct-ws mode)`)
    this.session = null
    this.onSessionDropped()
  }
})
```

## Lý do

Không có `onDispose`: khi Agent disconnect, `this.session` vẫn trỏ đến `mux` đã chết → `isAlive()` trả `true` → relay calls fail với "Connection lost" thay vì trigger auto-reconnect.

## Verification

```bash
pnpm tsc --noEmit

# Kiểm tra pattern đã consistent giữa hai methods:
grep -A 10 "onDispose" src/main/dev-server/dev-server-relay-bridge.ts
# Expected: 2 occurrences (connectWithExternalToken + connectDirectWebSocket)
```
