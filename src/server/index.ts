/**
 * Orca Server Entry Point — Node.js Server Mode
 *
 * Starts the Orca backend without Electron GUI:
 * 1. Initialize NodeAdapter (platform abstraction)
 * 2. Set platform singleton (MUST be before any src/main/ imports)
 * 3. Start HTTP server (static web files) if out/web/ exists
 * 4. Initialize all Orca services (DB, PTY, IPC, RPC) via server-bootstrap
 *
 * Usage:
 *   node out/server/index.js
 *
 * Environment variables:
 *   ORCA_PORT            - WebSocket/RPC port (default: 6768)
 *   ORCA_HTTP_PORT       - HTTP port for static files (default: ORCA_PORT + 1 = 6769)
 *   ORCA_USER_DATA_PATH  - Override userData directory (default: ~/.orca)
 *   ORCA_WEB_ROOT        - Path to web bundle dir (default: <out>/web)
 *   ORCA_VERSION         - App version string
 *   ORCA_MULTI_USER      - Set to '1' to enable per-user process isolation (Phase 2)
 *   ORCA_ADMIN_EMAIL     - Initial admin email (default: admin@localhost)
 *   ORCA_ADMIN_PASSWORD  - Initial admin password (auto-generated if not set)
 */

import { resolve } from 'node:path'
import { existsSync } from 'node:fs'

// ── STEP 1: Create NodeAdapter and set platform BEFORE any src/main/ imports ──
import { createNodeAdapter } from '../platform/adapters/node'
import { setPlatform } from '../platform/context'

const userDataPath = process.env.ORCA_USER_DATA_PATH
  ? resolve(process.env.ORCA_USER_DATA_PATH)
  : undefined

const adapter = createNodeAdapter(userDataPath ? { userDataPath } : {})
setPlatform(adapter)

console.log('[Orca Server] Platform: NodeAdapter')
console.log('[Orca Server] userData:', adapter.app.getPath('userData'))
console.log('[Orca Server] Version:', adapter.app.getVersion())

// ── STEP 2: Parse configuration ───────────────────────────────────────────────
const rpcPort = parseInt(process.env.ORCA_PORT ?? '6768', 10)
const httpPort = parseInt(process.env.ORCA_HTTP_PORT ?? String(rpcPort + 1), 10)

// Web bundle root: prefer env var, otherwise look for out/web/ next to out/server/
const webRoot = process.env.ORCA_WEB_ROOT
  ? resolve(process.env.ORCA_WEB_ROOT)
  : resolve(__dirname, '..', 'web')

// ── STEP 3: Main startup sequence ─────────────────────────────────────────────
async function main(): Promise<void> {
  let httpServer: import('node:http').Server | null = null

  // Log DB configuration at startup (password masked)
  const dbUrl = process.env['ORCA_DB_URL']
  const dbDialect = process.env['ORCA_DB_DIALECT']
  if (dbUrl) {
    try {
      const { formatDsn, parseDsn } = await import('../main/db/dsn-parser')
      const config = parseDsn(dbUrl)
      console.log(`[Orca Server] Database: ${formatDsn(config)}`) // password masked
    } catch {
      console.log('[Orca Server] Database: ORCA_DB_URL is set (invalid DSN — will error on connect)')
    }
  } else if (dbDialect) {
    console.log(`[Orca Server] Database dialect: ${dbDialect}`)
  } else {
    console.log('[Orca Server] Database: SQLite (default)')
  }

  // Initialize all Orca backend services
  const { initializeOrcaServices } = await import('../main/server-bootstrap')
  const { shutdown, dbMonitor, pushManager, authManager } = await initializeOrcaServices({
    platform: adapter,
    port: rpcPort
  })

  // Start HTTP server for web bundle (if available)
  if (existsSync(webRoot)) {
    const { startHttpServer } = await import('./http-server')
    httpServer = await startHttpServer(httpPort, webRoot, { dbMonitor, authManager })
    // Register Web Push API routes on the same HTTP server (Phase 3 — TASK-035)
    const { registerPushApiRoutes } = await import('./push-api-routes')
    registerPushApiRoutes(httpServer, pushManager)
    console.log(`[Orca Server] Web UI:  http://0.0.0.0:${httpPort}`)
  } else {
    console.warn(`[Orca Server] Web bundle not found at: ${webRoot}`)
    console.warn('[Orca Server] Run `pnpm build:frontend:web` to build the web bundle.')
    console.warn('[Orca Server] Continuing in API-only mode (no static files).')
  }

  console.log(`[Orca Server] RPC:     ws://0.0.0.0:${rpcPort}`)

  // ── Phase 2: Multi-User Mode (ORCA_MULTI_USER=1) ──────────────────────────
  // When enabled: each authenticated user gets their own forked OrcaRuntime process.
  // WsSessionRouter intercepts WS connections and proxies to the per-user process.
  let sessionManagerShutdown: (() => Promise<void>) | null = null
  const multiUserMode = process.env['ORCA_MULTI_USER'] === '1'

  if (multiUserMode) {
    const { SessionManager }   = await import('../main/session/session-manager')
    const { WsSessionRouter }  = await import('../main/session/ws-session-router')
    const { resolve: resolvePath } = await import('node:path')

    const baseDataPath      = adapter.app.getPath('userData')
    const userProcessEntry  = resolvePath(__dirname, '..', 'main', 'session', 'user-process-entry.js')

    const sessionManager = new SessionManager({ baseDataPath, userProcessEntry })
    const wsRouter       = new WsSessionRouter({ sessionManager, authManager })
    sessionManagerShutdown = () => sessionManager.shutdown()

    console.log('[Orca Server] ✅ Multi-user mode: SessionManager initialized')
    console.log(`[Orca Server]    User process entry: ${userProcessEntry}`)
    // WsSessionRouter is available for WebSocket server integration (TASK-021)
    void wsRouter  // suppress unused-var warning — wired in future WS server task
  } else {
    console.log('[Orca Server] Single-user mode (set ORCA_MULTI_USER=1 to enable per-user isolation)')
  }

  console.log('[Orca Server] ✅ Ready! Press Ctrl+C to stop.')

  // ── Graceful shutdown ───────────────────────────────────────────────────────
  const handleShutdown = async (signal: string) => {
    console.log(`\n[Orca Server] ${signal} received — shutting down gracefully...`)
    try {
      httpServer?.close()
      if (sessionManagerShutdown) await sessionManagerShutdown()
      await shutdown()
    } catch (err) {
      console.error('[Orca Server] Error during shutdown:', err)
    } finally {
      process.exit(0)
    }
  }

  process.on('SIGINT', () => handleShutdown('SIGINT'))
  process.on('SIGTERM', () => handleShutdown('SIGTERM'))
}

main().catch((error) => {
  console.error('[Orca Server] Fatal error during startup:', error)
  process.exit(1)
})
