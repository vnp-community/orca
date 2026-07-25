/**
 * Server Bootstrap — Selective initialization for Node.js server mode.
 *
 * Initializes only the backend services needed for headless operation,
 * adapted from the patterns in src/main/index.ts.
 *
 * This module MUST be imported AFTER setPlatform() has been called.
 * All heavy imports are done via dynamic import() to allow tree-shaking.
 *
 * @module main/server-bootstrap
 */

import type { IPlatformServices } from '../platform/types'
import { DevServerManager } from './dev-server/dev-server-manager'
import { registerDevServerIpcHandlers } from './ipc/dev-server-ipc'
import { registerOnboardingIpcHandlers } from './ipc/onboarding-ipc'
import { registerRepoRemoteIpcHandlers } from './ipc/repo-remote-ipc'
import { WebPushManager } from './notifications/web-push-manager'
import { AuthManager } from './auth/auth-manager'
import type { DatabaseConfig } from './db/config'
import type { IConnectionPool } from './db/pool'

export interface ServerBootstrapResult {
  /** Shutdown function — call to cleanly stop all services */
  shutdown(): Promise<void>
  /** DevServerManager — exposed for http-server and downstream services */
  devServerManager: DevServerManager
  /** Database health monitor — exposed for /health endpoint integration */
  dbMonitor: import('./db/health').HealthChecker
  /** Web Push manager — exposed so server/index.ts can register push API routes */
  pushManager: WebPushManager
  /** AuthManager — exposed for HTTP server to mount /auth routes and admin panel */
  authManager: AuthManager
}

export interface ServerBootstrapOptions {
  platform: IPlatformServices
  /** Port for RPC WebSocket. Default: 6768 */
  port?: number
  /**
   * Override database config (takes priority over env vars).
   * Pass `null` to explicitly force JSON file fallback.
   * Omit (undefined) to use loadDatabaseConfig() from environment.
   */
  database?: DatabaseConfig | null
}

/**
 * Initialize all Orca backend services for server mode.
 *
 * Initialization sequence (mirrors src/main/index.ts but GUI-free):
 * 1. Set up data path (via initDataPath — reads from app stub internally)
 * 2. Initialize SQLite persistence (new Store())
 * 3. Initialize stats collector
 * 4. Initialize Orca profile paths (non-fatal)
 * 5. Initialize PTY daemon provider (non-fatal)
 * 6. Bootstrap OrcaRuntimeService with Store + stats
 * 7. Start OrcaRuntimeRpcServer with WebSocket enabled
 */
export async function initializeOrcaServices(
  options: ServerBootstrapOptions
): Promise<ServerBootstrapResult> {
  const { platform, port: requestedPort = 6768 } = options

  const userDataPath = platform.app.getPath('userData')
  console.log('[ServerBootstrap] userData:', userDataPath)
  console.log('[ServerBootstrap] Initializing Orca backend services...')

  // 1. Initialize data path (reads from app module — aliased to our NodeApp stub)
  const { initDataPath } = await import('./persistence')
  initDataPath()
  console.log('[ServerBootstrap] ✅ Data path initialized')

  // 2. Initialize SQLite Store (legacy Electron-compatible persistence)
  const { Store } = await import('./persistence')
  const store = new Store()
  console.log('[ServerBootstrap] ✅ SQLite store initialized')

  // 2a-pre. Initialize WebPushManager (Phase 3 — TASK-035)
  // Why: initialized right after store so it can persist VAPID keys before any
  // other service touches the store. pushManager is returned to server/index.ts.
  const pushManager = new WebPushManager(store)
  console.log('[ServerBootstrap] ✅ WebPushManager initialized')

  // 2a. Initialize DevServerManager
  const { SshConnectionManager } = await import('./ssh/ssh-connection-manager')
  const sshManager = new SshConnectionManager({
    onStateChanged: () => {/* no-op in server bootstrap mode */}
  } as never)
  const devServerManager = new DevServerManager(store, sshManager)
  registerDevServerIpcHandlers(devServerManager, store)
  registerOnboardingIpcHandlers(devServerManager, store)
  registerRepoRemoteIpcHandlers(devServerManager, store)
  console.log('[ServerBootstrap] ✅ DevServerManager initialized')

  // 2b. Initialize DB pool, run migrations, create state repository, start health monitor
  const { join } = await import('node:path')
  const { loadDatabaseConfig } = await import('./db/config-loader')

  // options.database === undefined → use env vars; null → force JSON fallback
  const dbConfig = options.database !== undefined
    ? options.database
    : loadDatabaseConfig()

  let pool: IConnectionPool
  if (dbConfig && dbConfig.dialect !== 'sqlite') {
    const { GenericConnectionPool } = await import('./db/generic-pool')
    await import('./db/mysql/mysql-adapter')   // register MySQL/TiDB providers
    await import('./db/postgresql/pg-adapter') // register PostgreSQL provider
    pool = new GenericConnectionPool(dbConfig, (dbConfig as Record<string, unknown>)['pool'] as never)
    await (pool as GenericConnectionPool & { initialize(): Promise<void> }).initialize()
    console.log(`[ServerBootstrap] ✅ ${dbConfig.dialect} connection pool initialized`)
  } else {
    const { SqliteSingleConnectionPool } = await import('./db/sqlite/sqlite-pool')
    await import('./db/sqlite/sqlite-adapter') // ensure sqlite provider is registered
    const sqlitePath = dbConfig?.dialect === 'sqlite' && (dbConfig as Record<string, unknown>)['path']
      ? (dbConfig as Record<string, unknown>)['path'] as string
      : join(userDataPath, 'orca-server.db')
    pool = new SqliteSingleConnectionPool(sqlitePath)
    console.log('[ServerBootstrap] ✅ SQLite connection pool initialized')
  }

  // Auto-run migrations
  try {
    const { MigrationRunner } = await import('./db/migrations/runner')
    const { ALL_MIGRATIONS } = await import('./db/migrations')
    await pool.withConnection(async (db) => {
      const runner = new MigrationRunner(db, ALL_MIGRATIONS)
      const applied = await runner.migrate()
      if (applied.length > 0) {
        console.log(`[ServerBootstrap] ✅ Applied ${applied.length} migration(s):`, applied.map((m) => m.name))
      } else {
        console.log('[ServerBootstrap] ✅ DB schema up to date')
      }
    })
  } catch (err) {
    console.warn('[ServerBootstrap] Migration failed (non-fatal):', (err as Error)?.message)
  }

  // Create state repository
  const { createStateRepository } = await import('./repositories/factory')
  const stateRepo = dbConfig
    ? createStateRepository({ pool })
    : createStateRepository({ dataFile: join(userDataPath, 'store.json') })
  console.log('[ServerBootstrap] ✅ State repository created (backend:', dbConfig?.dialect ?? 'json-file', ')')

  // Start health monitor
  const { DatabaseHealthMonitor } = await import('./db/health-monitor')
  const dbMonitor = new DatabaseHealthMonitor(pool, dbConfig?.dialect ?? 'sqlite')
  dbMonitor.startPeriodicCheck(30_000)
  dbMonitor.onStatusChange((check) => {
    if (check.status === 'unhealthy') {
      console.error(`[ServerBootstrap] ❌ Database unhealthy: ${check.lastError}`)
    } else if (check.status === 'degraded') {
      console.warn(`[ServerBootstrap] ⚠️ Database degraded: ${check.latencyMs}ms`)
    }
  })
  console.log('[ServerBootstrap] ✅ Database health monitor started')

  // Suppress unused variable warnings — stateRepo is available for future IPC handlers
  void stateRepo

  // 2c. Initialize AuthManager (auth subsystem — always initialized, routes active when ORCA_MULTI_USER=1)
  // Opens a dedicated SQLite connection (not from pool) so sessions survive pool recycles.
  const { SqliteAdapter: SqliteAuthAdapter } = await import('./db/sqlite/sqlite-adapter')
  const authDbPath = dbConfig?.dialect === 'sqlite' && (dbConfig as Record<string, unknown>)['path']
    ? (dbConfig as Record<string, unknown>)['path'] as string
    : join(userDataPath, 'orca-server.db')
  const authDb = new SqliteAuthAdapter(authDbPath)
  const authManager = new AuthManager(authDb)
  // Seed initial admin user on first server boot (idempotent, non-fatal)
  try {
    const { ensureFirstAdminUser } = await import('./admin/first-run-setup')
    await ensureFirstAdminUser(authDb, authManager.userStore)
  } catch (err) {
    console.warn('[ServerBootstrap] first-run admin setup failed (non-fatal):', (err as Error)?.message)
  }
  console.log('[ServerBootstrap] ✅ AuthManager initialized')


  // 3. Initialize stats collector
  const { StatsCollector, initStatsPath } = await import('./stats/collector')
  try {
    initStatsPath()
  } catch (err) {
    console.warn('[ServerBootstrap] initStatsPath failed (non-fatal):', (err as Error)?.message)
  }
  const stats = new StatsCollector()

  // 4. Initialize Orca profile paths (non-fatal)
  try {
    const { initOrcaProfilePaths } = await import('./orca-profiles/profile-index-store')
    await initOrcaProfilePaths()
    console.log('[ServerBootstrap] ✅ Orca profile paths initialized')
  } catch (err) {
    console.warn('[ServerBootstrap] initOrcaProfilePaths failed (non-fatal):', (err as Error)?.message)
  }

  // 5. Initialize PTY daemon provider (non-fatal — terminal features may degrade)
  let daemonShutdown: (() => Promise<void>) | null = null
  try {
    const { initDaemonPtyProvider, disconnectDaemon } = await import('./daemon/daemon-init')
    await initDaemonPtyProvider()
    daemonShutdown = disconnectDaemon
    console.log('[ServerBootstrap] ✅ PTY daemon initialized')
  } catch (err) {
    console.warn(
      '[ServerBootstrap] PTY daemon unavailable (terminal features may not work):',
      (err as Error)?.message
    )
  }

  // 6. Create OrcaRuntimeService
  const { OrcaRuntimeService } = await import('./runtime/orca-runtime')
  const runtime = new OrcaRuntimeService(store, stats)
  // Wire push manager so the runtime can dispatch web push on agent task complete.
  runtime.setPushManager(pushManager)
  console.log('[ServerBootstrap] ✅ OrcaRuntimeService created')

  // 7. Start OrcaRuntimeRpcServer (WebSocket enabled for web clients)
  const { OrcaRuntimeRpcServer } = await import('./runtime/runtime-rpc')
  const rpcServer = new OrcaRuntimeRpcServer({
    runtime,
    userDataPath,
    enableWebSocket: true,
    wsPort: requestedPort
  })
  await rpcServer.start()
  console.log(`[ServerBootstrap] ✅ RPC server listening (WS) on :${requestedPort}`)

  // 8. Wire FleetHealthMonitor (SOL-005 — CR-005: fleet health wiring)
  try {
    const { fleetHealthMonitor } = await import('./ssh/fleet-health-monitor')
    const { SshConnectionStore } = await import('./ssh/ssh-connection-store')
    const sshStore = new SshConnectionStore()
    // Wire dependency injection properties
    fleetHealthMonitor.getSshTargets = () =>
      sshStore.getAll().map((t) => ({ id: t.id, label: t.label, project: t.project }))
    fleetHealthMonitor.getConnectionState = (targetId) => {
      const conn = sshStore.getConnectionState(targetId)
      return conn ?? null
    }
    // Read persisted webhook URL from store settings (may be undefined — no-op)
    const storedState = await stateRepo.getState().catch(() => null)
    const webhookUrl = storedState?.settings?.fleetAlertWebhookUrl
    if (webhookUrl) {
      fleetHealthMonitor.getWebhookUrl = () => webhookUrl
      console.log('[ServerBootstrap] ✅ Fleet alert webhook configured')
    }
    const pingIntervalMs = storedState?.settings?.fleetHealthPingIntervalMs ?? 60_000
    fleetHealthMonitor.start(pingIntervalMs)
    console.log(`[ServerBootstrap] ✅ FleetHealthMonitor started (interval: ${pingIntervalMs}ms)`)
  } catch (err) {
    console.warn('[ServerBootstrap] FleetHealthMonitor wiring failed (non-fatal):', (err as Error)?.message)
  }

  return {
    devServerManager,
    dbMonitor,
    pushManager,
    authManager: authManager!,
    async shutdown() {
      console.log('[ServerBootstrap] Shutting down...')
      try {
        if (authManager) authManager.destroy()
        console.log('[ServerBootstrap] ✅ AuthManager destroyed')
      } catch (err) {
        console.warn('[ServerBootstrap] AuthManager destroy error:', err)
      }
      try {
        await rpcServer.stop()
      } catch (err) {
        console.warn('[ServerBootstrap] RPC server stop error:', err)
      }
      try {
        if (daemonShutdown) await daemonShutdown()
      } catch (err) {
        console.warn('[ServerBootstrap] Daemon shutdown error:', err)
      }
      try {
        dbMonitor.stopPeriodicCheck()
        await pool.drain()
        console.log('[ServerBootstrap] ✅ Database pool drained')
      } catch (err) {
        console.warn('[ServerBootstrap] Pool drain error:', err)
      }
      console.log('[ServerBootstrap] Shutdown complete')
    }
  }
}
