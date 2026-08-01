# TASK-CLI-001: Implement Unix socket daemon cho headless CLI mode

**Priority:** 🔴 HIGH — Headless mode không có IPC mechanism  
**Effort:** ~60 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-BE-CLI-001, BUG-CLI-001  
**Solution ref:** [SOLUTION-cli-headless.md](../solutions/SOLUTION-cli-headless.md)

## Mục tiêu

Tạo `PtyDaemon` class lắng nghe trên Unix socket, nhận commands từ CLI client, forward tới OrcaRuntime.

## File cần tạo

`src/main/cli/PtyDaemon.ts` (NEW)

## Skeleton

```typescript
// src/main/cli/PtyDaemon.ts
import * as net from 'node:net'
import { existsSync, unlinkSync } from 'node:fs'

export class PtyDaemon {
  private server: net.Server | null = null

  constructor(
    private readonly socketPath: string,   // e.g. /tmp/orca-daemon.sock
    private readonly handler:    (cmd: unknown) => Promise<unknown>
  ) {}

  start(): Promise<void> {
    return new Promise((resolve, reject) => {
      // Cleanup stale socket
      if (existsSync(this.socketPath)) unlinkSync(this.socketPath)

      this.server = net.createServer((client) => {
        let buffer = ''
        client.on('data', (chunk) => {
          buffer += chunk.toString('utf8')
          const newline = buffer.indexOf('\n')
          if (newline !== -1) {
            const line = buffer.slice(0, newline)
            buffer = buffer.slice(newline + 1)
            try {
              const cmd = JSON.parse(line)
              this.handler(cmd)
                .then(result => client.write(JSON.stringify({ ok: true, result }) + '\n'))
                .catch(err => client.write(JSON.stringify({ ok: false, error: String(err) }) + '\n'))
            } catch {
              client.write(JSON.stringify({ ok: false, error: 'invalid JSON' }) + '\n')
            }
          }
        })
      })

      this.server.listen(this.socketPath, () => {
        console.log(`[PtyDaemon] Listening on ${this.socketPath}`)
        resolve()
      })
      this.server.on('error', reject)
    })
  }

  stop(): void {
    this.server?.close()
    if (existsSync(this.socketPath)) unlinkSync(this.socketPath)
  }
}
```

## Verification

```bash
pnpm tsc --noEmit
# Test: start daemon → connect via nc/socat → send JSON command → receive response
```
