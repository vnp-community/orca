# BUG-BE-TM-001: WsSessionRouter chuyển đổi binary frame sang text — vi phạm giao thức JSON-RPC 2.0 binary

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-TRM-002  
**Note:** ws-session-router.ts: binary frame detection + {binary:true} forward  

## Mức độ: HIGH

## Tóm tắt

`WsSessionRouter.handleConnection()` chuyển đổi **tất cả** WebSocket frames (cả binary và text) sang UTF-8 string trước khi ghi vào Unix socket, và trả ngược lại cũng chỉ dưới dạng text `wsAny.send(rawMessage)`. Luồng chuẩn theo HLD sử dụng binary frames với 13-byte header (TYPE[1B] + SEQ[4B] + ACK[4B] + LEN[4B]). Việc ép toàn bộ thành UTF-8 string sẽ **corrupt** binary header bytes.

## File liên quan

- [`src/main/session/ws-session-router.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/session/ws-session-router.ts) — Lines 104-136

## Code sai

```typescript
// Line 104-119: ghi vào upstream
ws.on('message', (data: Buffer | string, isBinary: boolean) => {
  if (!upstream.writable) return
  if (!isBinary) {
    // ... JSON text path (OK)
  }
  upstream.write(isBinary ? data : Buffer.from(data as string))
  // ← OK khi ghi xuống socket

  // Line 122-136: đọc từ upstream và gửi về client — CHỈ dùng text path
  upstream.on('data', (chunk: Buffer) => {
    upstreamBuffer += chunk.toString('utf8')   // ← BUG: ép binary sang UTF-8
    let newlineIndex = upstreamBuffer.indexOf('\n')
    while (newlineIndex !== -1) {
      const rawMessage = upstreamBuffer.slice(0, newlineIndex).trim()
      // ...
      wsAny.send(rawMessage)  // ← BUG: luôn gửi dạng text string
    }
  })
```

## Hành vi đúng (theo HLD §5)

```
Binary frame protocol:
  TYPE[1B] | SEQ[4B BE] | ACK[4B BE] | LEN[4B BE] | PAYLOAD(JSON-RPC UTF-8)
```

Upstream socket có thể gửi binary frames. Router phải forward đúng type:
- Nếu frame là binary → `ws.send(chunk, { binary: true })`
- Nếu frame là text → forward dạng text

**Cách sửa:** Phát hiện loại dữ liệu từ upstream và chọn cách gửi phù hợp. Không nên buffer toàn bộ bằng cách nối string UTF-8 vì sẽ corrupt binary header.

## Ảnh hưởng

- Khi User Process gửi binary PTY output frame về Browser (qua Unix socket → WsSessionRouter → WebSocket), dữ liệu bị corrupt.
- Terminal hiển thị garbage characters hoặc không hiển thị gì.
- Severity tăng khi PTY data stream nhiều (output dày).

## Liên quan đến luồng

- **BL-TM-01**: Bước 6 — PTY output từ Dev Server → Backend → Browser bị hỏng.
- **Trace span**: `wsSession:route` phase `proxy-start`.
