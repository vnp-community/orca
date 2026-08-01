# TASK-TRM-002: Fix binary frame forwarding trong WsSessionRouter

**Priority:** 🔴 HIGH — PTY output bị corrupt khi forward qua WS  
**Effort:** ~15 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-BE-TM-001  
**Solution ref:** [SOLUTION-TRM-BE-exact.md](../solutions/SOLUTION-TRM-BE-exact.md)

---

## Mục tiêu

Sửa `upstream.on('data')` handler trong `WsSessionRouter` để forward binary frames đúng cách thay vì ép chúng sang UTF-8 string.

## File cần sửa

```
src/main/session/ws-session-router.ts
```

## Thay đổi cụ thể

### Thay thế toàn bộ lines 122–137 (`let upstreamBuffer = ''` đến kết thúc upstream.on('data')):

**TRƯỚC (buggy):**
```typescript
let upstreamBuffer = ''
upstream.on('data', (chunk: Buffer) => {
  const wsAny = ws as unknown as { readyState: number; OPEN: number; send: (d: string) => void }
  if (wsAny.readyState !== wsAny.OPEN) return

  upstreamBuffer += chunk.toString('utf8')
  let newlineIndex = upstreamBuffer.indexOf('\n')
  while (newlineIndex !== -1) {
    const rawMessage = upstreamBuffer.slice(0, newlineIndex).trim()
    upstreamBuffer = upstreamBuffer.slice(newlineIndex + 1)
    if (rawMessage) {
      wsAny.send(rawMessage)
    }
    newlineIndex = upstreamBuffer.indexOf('\n')
  }
})
```

**SAU (fixed):**
```typescript
let upstreamBuffer = ''
upstream.on('data', (chunk: Buffer) => {
  const wsAny = ws as unknown as {
    readyState: number
    OPEN: number
    send: (d: string | Buffer, opts?: { binary: boolean }) => void
  }
  if (wsAny.readyState !== wsAny.OPEN) return

  // Binary wire-protocol frames have type byte 0x01–0x09 as first byte.
  // Forward them as binary WS frames without UTF-8 coercion.
  const firstByte = chunk[0]
  if (firstByte !== undefined && firstByte >= 0x01 && firstByte <= 0x09) {
    wsAny.send(chunk, { binary: true })
    return
  }

  // Text/JSON-RPC data — buffer by newline delimiter
  upstreamBuffer += chunk.toString('utf8')
  let newlineIndex = upstreamBuffer.indexOf('\n')
  while (newlineIndex !== -1) {
    const rawMessage = upstreamBuffer.slice(0, newlineIndex).trim()
    upstreamBuffer = upstreamBuffer.slice(newlineIndex + 1)
    if (rawMessage) {
      wsAny.send(rawMessage)
    }
    newlineIndex = upstreamBuffer.indexOf('\n')
  }
})
```

## Lý do

User process có thể gửi binary wire frames (TYPE byte 0x01–0x09 theo HLD §5). Code hiện tại ép tất cả sang UTF-8 string → corrupt binary header bytes → terminal hiển thị garbage hoặc không hiển thị.

## Verification

```bash
# Verify TypeScript compile clean:
pnpm tsc --noEmit

# Test: PTY output từ dev server hiển thị đúng trong terminal pane
# Không có garbage characters khi có output dày (ls -la, grep kết quả lớn)
```
