# TASK-013: Refactor `src/server/index.ts` dùng NodeAdapter

**Source:** SOL-BE-004  
**Phase:** 2 | **Effort:** S (30–45 min)  
**Depends on:** TASK-008, TASK-011, TASK-012

---

## Objective

Cập nhật `src/server/index.ts` để:
1. Dùng `createNodeAdapter()` thay vì Electron mock workaround
2. Gọi `setPlatform()` trước khi load bất kỳ module nào từ `src/main/`
3. Gọi `initializeOrcaServices()` để start backend
4. Start HTTP server nếu `out/web/` directory tồn tại

---

## Context cần đọc trước

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
cat src/server/index.ts
```

---

## File to modify

**`src/server/index.ts`** — thay thế toàn bộ nội dung bằng:

```typescript
/**
 * Orca Server Entry Point — Node.js Server Mode
 *
 * Starts the Orca backend without Electron GUI:
 * 1. Initialize NodeAdapter (platform abstraction)
 * 2. Set platform singleton
 * 3. Start HTTP server (static web files)
 * 4. Initialize all Orca services (DB, PTY, IPC, RPC)
 *
 * Usage:
 *   node out/server/index.js
 *
 * Environment variables:
 *   ORCA_PORT          - WebSocket/RPC port (default: 6768)
 *   ORCA_VERSION       - App version string
 *   ORCA_USER_DATA_PATH - Override userData directory
 *   ORCA_WEB_ROOT      - Path to web bundle (default: out/web/)
 *   ORCA_HTTP_PORT     - HTTP port for static files (default: same as ORCA_PORT + 1)
 */

import { resolve } from 'node:path'
import { existsSync } from 'node:fs'

// 1. Create NodeAdapter FIRST — before any src/main/ imports
import { createNodeAdapter } from '../platform/adapters/node'
import { setPlatform } from '../platform/context'

const userDataPath = process.env.ORCA_USER_DATA_PATH
  ? resolve(process.env.ORCA_USER_DATA_PATH)
  : undefined

const adapter = createNodeAdapter(userDataPath ? { userDataPath } : {})
setPlatform(adapter)

console.log('[Orca Server] Platform: NodeAdapter')
console.log('[Orca Server] userData:', adapter.app.getPath('userData'))

// 2. Start HTTP server for web bundle (if available)
const rpcPort = parseInt(process.env.ORCA_PORT ?? '6768', 10)
const httpPort = parseInt(process.env.ORCA_HTTP_PORT ?? String(rpcPort + 1), 10)
const webRoot = process.env.ORCA_WEB_ROOT
  ? resolve(process.env.ORCA_WEB_ROOT)
  : resolve(__dirname, '..', 'web')

async function main(): Promise<void> {
  let httpServer: import('node:http').Server | null = null

  // Start HTTP server for web files
  if (existsSync(webRoot)) {
    const { startHttpServer } = await import('./http-server')
    httpServer = await startHttpServer(httpPort, webRoot)
    console.log(`[Orca Server] Web UI: http://0.0.0.0:${httpPort}`)
  } else {
    console.warn(`[Orca Server] Web bundle not found at: ${webRoot}`)
    console.warn('[Orca Server] Run `pnpm build:frontend:web` to build the web bundle.')
  }

  // 3. Initialize all Orca backend services
  const { initializeOrcaServices } = await import('../main/server-bootstrap')
  const { rpcServer, port: actualRpcPort } = await initializeOrcaServices({
    platform: adapter,
    port: rpcPort,
    serveWebFiles: false  // HTTP server is separate (above)
  })

  console.log(`[Orca Server] RPC: ws://0.0.0.0:${actualRpcPort}`)
  console.log('[Orca Server] Ready! Press Ctrl+C to stop.')

  // Graceful shutdown
  const shutdown = async (signal: string) => {
    console.log(`\n[Orca Server] ${signal} received — shutting down...`)
    httpServer?.close()
    await rpcServer.close()
    process.exit(0)
  }

  process.on('SIGINT', () => shutdown('SIGINT'))
  process.on('SIGTERM', () => shutdown('SIGTERM'))
}

main().catch(err => {
  console.error('[Orca Server] Fatal error during startup:', err)
  process.exit(1)
})
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

# TypeScript compile
npx tsc --noEmit 2>&1 | grep "server/index" | head -10

# Check no Electron mock workaround remains
grep -n "mocks/electron\|Module.prototype.require\|createRequire" src/server/index.ts
# Expected: empty

# Check NodeAdapter is used
grep "createNodeAdapter\|setPlatform" src/server/index.ts
# Expected: both lines found
```

---

## Done criteria

- [x] `src/server/index.ts` cập nhật thành công
- [x] `setPlatform(createNodeAdapter())` được gọi TRƯỚC `import('../main/server-bootstrap')`
- [x] Không còn dùng `mocks/electron` hoặc Module.prototype hack
- [x] HTTP server khởi động nếu `out/web/` tồn tại
- [x] `SIGINT`/`SIGTERM` → graceful shutdown
- [x] TypeScript compile clean
