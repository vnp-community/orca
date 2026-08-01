# BUG-BE-TM-002: WsSessionRouter gửi keepalive `\n` vào Unix socket — corrupt JSON-RPC stream

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-TRM-001  
**Note:** ws-session-router.ts: keepalive newline removed  

## Mức độ: MEDIUM

## Tóm tắt

`WsSessionRouter.handleConnection()` có một keepalive timer gửi byte `\n` vào Unix socket mỗi 15 giây. Unix socket giữa WsSessionRouter và user-process-entry là giao thức JSON-RPC 2.0, không phải plain text. Việc gửi `\n` rỗng sẽ gây ra parse error ở phía User Process khi nó nhận được newline không thuộc JSON-RPC packet nào.

## File liên quan

- [`src/main/session/ws-session-router.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/session/ws-session-router.ts) — Lines 98-102

## Code sai

```typescript
// Lines 98-102
const keepaliveTimer = setInterval(() => {
  if (upstream.writable) {
    upstream.write('\n')  // ← BUG: gửi bare newline vào JSON-RPC stream
  }
}, 15000)
```

## Hành vi đúng

Theo HLD, keepalive giữa Backend và User Process cần sử dụng **KeepAlive frame** của wire protocol (TYPE = 0x09) nếu cần, hoặc hoàn toàn không dùng keepalive vì Unix domain socket không cần TCP keepalive — chúng là local IPC, không có NAT timeout hay idle timeout.

**Tùy chọn sửa:**
1. **Xóa hoàn toàn keepalive timer** — Unix sockets không cần keepalive.
2. Nếu keepalive thực sự cần: dùng frame đúng format `TYPE[0x09] + SEQ + ACK + LEN[0]` (zero-length KeepAlive frame).

## Ảnh hưởng

- Mỗi 15 giây, User Process nhận được `\n` rỗng và có thể:
  - Bỏ qua (nếu JSON parser handle empty lines)
  - Gây ra parse error tùy implementation
  - Worst case: disconnect toàn bộ session
- Bugs xuất hiện ở idle connections khi người dùng không gõ gì trong 15 giây.

## Liên quan đến luồng

- **BL-TM-01**: Bước 1 — Auth & Session Routing có thể gây gián đoạn session.
