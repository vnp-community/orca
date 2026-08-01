# TASK-SSH-001: Thêm exponential backoff cho SSH reconnect

**Priority:** 🟠 HIGH — SSH disconnect → reconnect flood server  
**Effort:** ~20 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-BE-SSH-001  
**Solution ref:** [SOLUTION-remote-development.md](../solutions/SOLUTION-remote-development.md)

## Bước 1 — Tìm SSH reconnect logic

```bash
grep -rn "reconnect\|reconnect\|backoff\|retry" src/main/dev-server/ --include="*.ts" | head -10
```

## Bước 2 — Thêm exponential backoff vào dev-server-relay-bridge.ts

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts
// Tìm reconnect loop và thêm backoff:

private async reconnectWithBackoff(): Promise<void> {
  const MAX_DELAY_MS = 60_000  // 60s max
  const BASE_DELAY   =  1_000  // 1s initial
  let attempt = 0

  while (this._reconnecting) {
    const delay = Math.min(BASE_DELAY * Math.pow(2, attempt), MAX_DELAY_MS)
    const jitter = Math.random() * 1000  // up to 1s jitter
    console.log(`[DevServerRelayBridge] Reconnect attempt ${attempt + 1} in ${Math.round((delay + jitter) / 1000)}s`)

    await new Promise(r => setTimeout(r, delay + jitter))
    
    if (!this._reconnecting) break  // cancelled

    try {
      await this.connect()
      console.log(`[DevServerRelayBridge] Reconnected after ${attempt + 1} attempts`)
      return
    } catch (err) {
      attempt++
      if (attempt > 10) {
        console.error('[DevServerRelayBridge] Max reconnect attempts reached, giving up')
        this._reconnecting = false
        this.emit('devServer:statusChanged', this.config.id, 'error', err)
        return
      }
    }
  }
}
```

## Verification

```bash
pnpm tsc --noEmit
# Test: kill SSH connection → logs show increasing delay between reconnects
```
