# TASK-006: Sửa `src/relay/relay-handshake.ts` — Thêm platform/arch/nodeVersion

**Phase:** 1 — Foundation  
**Solution:** [SOL-002](../solutions/SOL-002-dev-server-manager.md) §7  
**Depends on:** (không có — relay-side change)  
**Blocks:** TASK-005 (relay handshake phải gửi info trước khi bridge đọc)

---

## Mục tiêu

Sửa `relay-handshake.ts` để relay daemon gửi thêm `platform`, `arch`, `nodeVersion` trong handshake frame. Thông tin này sẽ được `DevServerRelayBridge.connect()` đọc về.

---

## File cần sửa

**Path:** `src/relay/relay-handshake.ts`

---

## Thay đổi cần thực hiện

### 1. Mở rộng `RelayHandshakeInfo` type (nếu chưa có)

```typescript
// Thêm hoặc cập nhật type RelayHandshakeInfo trong file:
export type RelayHandshakeInfo = {
  platform: NodeJS.Platform   // NEW — process.platform của relay
  arch: string                // NEW — process.arch của relay
  nodeVersion: string         // NEW — process.version của relay
  relayVersion: string        // existing hoặc thêm mới
}
```

### 2. Mở rộng handshake frame

```typescript
// Trong hàm xây dựng handshake frame của daemon side:
const handshakeFrame = encodeHandshakeFrame({
  type: MessageType.Handshake,
  version: launchVersion,
  platform: process.platform,      // NEW
  arch: process.arch,              // NEW
  nodeVersion: process.version     // NEW
})
```

### 3. Cập nhật `DaemonHandshakeCallbacks` (nếu cần)

```typescript
export type DaemonHandshakeCallbacks = {
  onAccepted: (sock: Socket, leftover: Buffer, info: RelayHandshakeInfo) => void  // thêm info
  launchVersion: string
}
```

---

## Acceptance Criteria

- [x] `RelayHandshakeInfo` export có đủ các fields: `platform`, `arch`, `nodeVersion`, `relayVersion`
- [x] Daemon gửi `platform`, `arch`, `nodeVersion` trong handshake frame
- [x] Caller (host side) nhận đủ thông tin sau handshake
- [x] Không breaking: relay vẫn kết nối thành công với Orca host
- [x] TypeScript compile thành công

---

## Lưu ý cho AI

1. Đọc toàn bộ file `src/relay/relay-handshake.ts` trước khi sửa
2. Tìm đúng nơi encode handshake frame (daemon side)
3. Tìm đúng nơi decode (host side) và cập nhật để parse thêm fields mới
4. Đảm bảo backward compat: nếu relay cũ chưa gửi `platform` → host side phải handle gracefully (default `null` hoặc `process.platform`)
