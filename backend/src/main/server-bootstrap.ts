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
import { join as pathJoin } from 'node:path'
import { existsSync } from 'node:fs'
import { DevServerManager } from './dev-server/dev-server-manager'
import { registerDevServerIpcHandlers } from './ipc/dev-server-ipc'
import { registerOnboardingIpcHandlers } from './ipc/onboarding-ipc'
import { registerRepoRemoteIpcHandlers } from './ipc/repo-remote-ipc'
import { WebPushManager } from './notifications/web-push-manager'
import { AuthManager } from './auth/auth-manager'
import { initWebCredentialStore } from './credentials'
import type { DatabaseConfig } from './db/config'
import type { IConnectionPool } from './db/pool'
import { AgentWebSocketServer } from './dev-server/agent-ws-server'

export type ServerBootstrapResult = {
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
  /**
   * SessionManager — forks per-user child processes in multi-user mode.
   * null when ORCA_MULTI_USER is not set (single-user / Electron mode).
   */
  sessionManager: import('./session/session-manager').SessionManager | null
  /** AgentWebSocketServer — attach to HTTP server for direct-websocket agent connections */
  agentWsServer: AgentWebSocketServer
  // ─── v5.0 services (TDD-14 → TDD-20) ─────────────────────────────────────
  /**
   * RelayConnectionPool — ref-counted pool of DevServerRelayBridge connections.
   * Prerequisite for ProjectServerRouter, AIProviderService, and WorkspaceService.
   * (v5.0 TDD-15)
   */
  relayConnectionPool: import('./dev-server/relay-connection-pool').RelayConnectionPool
  /**
   * ProfileService — CRUD for company / department / user profiles.
   * (v5.0 TDD-14)
   */
  profileService: import('./profile/ProfileService').ProfileService
  /**
   * ProfileResolver — 3-layer merge engine with 60s TTL cache.
   * (v5.0 TDD-14)
   */
  profileResolver: import('./profile/ProfileResolver').ProfileResolver
  /**
   * ProjectService — project CRUD and member management.
   * (v5.0 TDD-15)
   */
  projectService: import('./project/ProjectService').ProjectService
  /**
   * AIProviderService — AI provider account registry + relay credential store.
   * (v5.0 TDD-16)
   */
  aiProviderService: import('./ai-providers/AIProviderService').AIProviderService
  /**
   * WorkflowOrchestrator — DAG-based multi-server workflow execution.
   * (v5.0 TDD-17)
   */
  workflowOrchestrator: import('./workflow/WorkflowOrchestrator').WorkflowOrchestrator
  /**
   * TaskService — task graph CRUD, BFS tree operations, dependency edges.
   * (v5.0 TDD-18)
   */
  taskService: import('./task/TaskService').TaskService
  /**
   * RPC auth token from OrcaRuntimeRpcServer — used by user-process-entry to
   * report back to SessionManager so WsSessionRouter can inject it when proxying
   * WebSocket messages over the Unix socket.
   */
  rpcAuthToken: string
  /**
   * AutomationService — scheduler + dispatch for `automation.*` RPC methods.
   * (D1 fix — docs/guides/fix-proposals-per-issue.md §D1)
   */
  automationService: import('./automations/service').AutomationService
  /** ClaudeUsageStore — usage attribution for automation runs. */
  claudeUsage: import('./claude-usage/store').ClaudeUsageStore
  /** CodexUsageStore — usage attribution for automation runs. */
  codexUsage: import('./codex-usage/store').CodexUsageStore
}


export type ServerBootstrapOptions = {
  platform: IPlatformServices
  /** Port for RPC WebSocket. Default: 6768. Pass 0 to disable TCP WebSocket (user-process mode). */
  port?: number
  /**
   * Unix domain socket path for the RPC server to listen on.
   * Used in user-process mode: WsSessionRouter connects here to proxy
   * browser WebSocket traffic to the per-user OrcaRuntime.
   * When set, the server listens on this socket instead of (or in addition to)
   * the TCP WebSocket port.
   */
  socketPath?: string
  /**
   * Override database config (takes priority over env vars).
   * Pass `null` to explicitly force JSON file fallback.
   * Omit (undefined) to use loadDatabaseConfig() from environment.
   */
  database?: DatabaseConfig | null
  /**
   * True if running inside a User Process (child of SessionManager).
   */
  isUserProcess?: boolean
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
  const { platform, port: requestedPort = 6768, socketPath } = options

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

  // 2a. Initialize DevServerManager + AgentWebSocketServer
  const { SshConnectionManager } = await import('./ssh/ssh-connection-manager')
  const sshManager = new SshConnectionManager({
    onStateChanged: () => {/* no-op in server bootstrap mode */}
  } as never)

  // Why: AgentWebSocketServer must be created BEFORE DevServerManager so it can
  // be injected into DevServerRelayBridge for direct-websocket mode.
  const agentWsServer = new AgentWebSocketServer(platform.app.getVersion())

  let devServerManager: DevServerManager
  if (options.isUserProcess) {
    const { GatewayDevServerManagerProxy } = await import('./dev-server/gateway-proxy')
    devServerManager = new GatewayDevServerManagerProxy() as unknown as DevServerManager
    console.log('[ServerBootstrap] ✅ GatewayDevServerManagerProxy initialized (Proxying to Main Process)')
  } else {
    devServerManager = new DevServerManager(store, sshManager, agentWsServer)
    console.log('[ServerBootstrap] ✅ DevServerManager + AgentWebSocketServer initialized')
  }

  registerDevServerIpcHandlers(devServerManager, store)
  registerOnboardingIpcHandlers(devServerManager, store)
  registerRepoRemoteIpcHandlers(devServerManager, store)

  // Why: makes a connected Dev Server's agent WebSocket usable as a repo
  // execution-host connection (fs/git/pty), reusing the ssh-*-dispatch.ts
  // provider registries orca-runtime.ts already resolves through — see
  // dev-server-provider-lifecycle.ts. Kept here, immediately after
  // devServerManager, rather than after OrcaRuntimeService exists further
  // down: a real Dev Server can finish its WebSocket handshake and fire
  // 'connected' before the rest of bootstrap (DB/migrations/auth/session-
  // manager) finishes, so attaching this listener any later risks missing
  // that event outright. The runtime-dependent half (PTY controller + data
  // relay) is wired separately via .attachRuntime(runtime) once runtime
  // exists — see that call site for why it's still race-safe despite the
  // gap between this line and that one.
  const { wireDevServerProviders } = await import('./providers/dev-server-provider-lifecycle')
  const devServerProviderLifecycle = wireDevServerProviders(devServerManager)

  // 2a-pool. Initialize RelayConnectionPool (v5.0 — prerequisite for Project + AI services)
  const { RelayConnectionPool } = await import('./dev-server/relay-connection-pool')
  const { DevServerRelayBridge } = await import('./dev-server/dev-server-relay-bridge')
  const relayConnectionPool = new RelayConnectionPool(async (server) => {
    const bridge = new DevServerRelayBridge(server, sshManager, agentWsServer)
    await bridge.connect()
    return bridge
  })
  console.log('[ServerBootstrap] ✅ RelayConnectionPool initialized (v5.0)')

  // 2b-pre. Initialize WebCredentialStore for multi-user Web mode.
  // Why: must be init'd before any user session is spawned so SessionManager
  // can call buildCredentialEnv() when forking child processes.
  const serverSecret = process.env['ORCA_SERVER_SECRET']
  if (process.env['ORCA_MULTI_USER'] === '1') {
    if (!serverSecret) {
      console.warn(
        '[ServerBootstrap] ⚠️  ORCA_SERVER_SECRET not set — credential encryption will be weak. ' +
        'Generate a secret with: openssl rand -hex 32'
      )
    }
    const credUserId = process.env['ORCA_USER_ID'] ?? 'default'
    initWebCredentialStore(userDataPath, credUserId, serverSecret ?? `orca-fallback-${userDataPath}`)
    console.log(`[ServerBootstrap] ✅ WebCredentialStore initialized (userId: ${credUserId})`)
  }

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

  // 2d. Initialize SessionManager for multi-user Web mode (TASK-14).
  // Why: SessionManager forks per-user child processes and injects credential
  // env vars (ORCA_BITBUCKET_*, ORCA_AZURE_DEVOPS_*, etc.) at spawn time.
  // The serverSecret enables WebCredentialStore.getToken() to decrypt tokens.
  let sessionManager: import('./session/session-manager').SessionManager | null = null
  if (process.env['ORCA_MULTI_USER'] === '1') {
    const { SessionManager, resolveIdleTimeoutMsFromEnv } = await import('./session/session-manager')
    const userProcessEntry = pathJoin(
      platform.app.getAppPath(), 'out', 'main', 'user-process-entry.js'
    )
    sessionManager = new SessionManager({
      baseDataPath: userDataPath,
      userProcessEntry,
      serverSecret: process.env['ORCA_SERVER_SECRET'],
      // FIX BUG-BE-HLD-011: allow ops to override idle timeout without a code change.
      idleTimeoutMs: resolveIdleTimeoutMsFromEnv(process.env),
      devServerManager
    })
    console.log('[ServerBootstrap] ✅ SessionManager initialized (multi-user mode, serverSecret present:', !!process.env['ORCA_SERVER_SECRET'], ')')
  }


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
  // Why: In server mode (Node.js Docker), out/main/daemon-entry.js is not bundled
  // (it's Electron-only). We check for its presence before attempting init so
  // the child_process.fork() doesn't emit MODULE_NOT_FOUND errors to the log.
  let daemonShutdown: (() => Promise<void>) | null = null
  try {
    const appPath = platform.app.getAppPath()
    const daemonEntryPath = pathJoin(appPath, 'out', 'main', 'daemon-entry.js')

    if (!existsSync(daemonEntryPath)) {
      console.log(
        '[ServerBootstrap] PTY daemon skipped (daemon-entry.js not found — expected in server/web mode)'
      )
    } else {
      const { initDaemonPtyProvider, disconnectDaemon } = await import('./daemon/daemon-init')
      await initDaemonPtyProvider()
      daemonShutdown = disconnectDaemon
      console.log('[ServerBootstrap] ✅ PTY daemon initialized')
    }
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

  // Why only now, not alongside wireDevServerProviders() above: runtime
  // didn't exist yet there. attachRuntime() wires runtime.setPtyController +
  // the PTY data-plane relay (this pure-Node process has no Electron
  // ipc/pty.ts controller wiring it any other way — without this every
  // terminal.create would throw 'runtime_unavailable') and retroactively
  // wires the relay for any Dev Server that already connected and registered
  // during the gap between these two calls, so registration order never
  // matters.
  devServerProviderLifecycle.attachRuntime(runtime)

  const { OrcaRuntimeRpcServer } = await import('./runtime/runtime-rpc')
  const rpcServer = new OrcaRuntimeRpcServer({
    runtime,
    userDataPath,
    enableWebSocket: !options.socketPath, // disable TCP WS when using Unix socket proxy
    wsPort: requestedPort,
    // Why: in user-process mode, the supervisor (WsSessionRouter) connects to
    // this Unix socket to proxy browser WebSocket traffic. Passing socketPath
    // overrides the default userData-derived socket so the process binds exactly
    // where SessionManager expects it (ORCA_SOCKET_PATH).
    ...(socketPath ? { socketPath } : {}),
    // Why: proxy methods (preflight.check, github.startAuthLogin, etc.) need
    // to reach the active relay for a given Dev Server.
    devServerManager
  })
  await rpcServer.start()
  if (socketPath) {
    console.log(`[ServerBootstrap] ✅ RPC server listening (Unix socket) on: ${socketPath}`)
  } else {
    console.log(`[ServerBootstrap] ✅ RPC server listening (WS) on :${requestedPort}`)
  }

  // 9. ProfileService + ProfileResolver [v5.0 TDD-14]
  const { ProfileService } = await import('./profile/ProfileService')
  const { ProfileResolver } = await import('./profile/ProfileResolver')
  const profileService = new ProfileService(pool)
  const profileResolver = new ProfileResolver(profileService)
  console.log('[ServerBootstrap] ✅ ProfileService + ProfileResolver initialized (v5.0)')

  // FIX BUG-BE-HLD-001/002: shared org-level role lookup, same source of truth as
  // admin-middleware.ts (session.role) — both read from orca_users via AuthUserStore.
  const getUserRole = async (userId: string) => (await authManager.userStore.getUser(userId))?.role ?? null

  // Register profile RPC methods [T01]
  const { createProfileMethods } = await import('./profile/profile-rpc-handler')
  rpcServer.addMethods(createProfileMethods(profileService, profileResolver, getUserRole))
  console.log('[ServerBootstrap] ✅ Profile RPC methods registered (v5.0)')

  // 10. ProjectService + ProjectServerRouter [v5.0 TDD-15]
  const { ProjectService } = await import('./project/ProjectService')
  const { ProjectServerRouter } = await import('./project/ProjectServerRouter')
  const projectService = new ProjectService(pool, devServerManager)
  const _projectRouter = new ProjectServerRouter(projectService, devServerManager, relayConnectionPool)
  console.log('[ServerBootstrap] ✅ ProjectService + ProjectServerRouter initialized (v5.0)')

  // 11. AIProviderService + ProviderResolver + ProviderHealthChecker [v5.0 TDD-16]
  const { AIProviderService } = await import('./ai-providers/AIProviderService')
  const { ProviderResolver } = await import('./ai-providers/ProviderResolver')
  const { ProviderHealthChecker } = await import('./ai-providers/ProviderHealthChecker')
  const { createAIProviderMethods } = await import('./ai-providers/ai-provider-rpc-handler')
  // BUG-BE-HLD-014: audit trail for account CRUD + key rotation (rotateKey.started/
  // completed/failed) — without this, rotation events would only ever hit console.log.
  const { AuditLogger } = await import('./auth/audit-logger')
  const aiProviderAuditLogger = new AuditLogger(pool)
  const aiProviderService = new AIProviderService(pool, devServerManager, relayConnectionPool, aiProviderAuditLogger)
  const providerResolver = new ProviderResolver(aiProviderService)
  const providerHealthChecker = new ProviderHealthChecker()
  // FIX BUG-AIP-004: Removed relayPool param (unused — service manages relay internally)
  providerHealthChecker.start(aiProviderService)
  // FIX BUG-AIP-003: Wire status change alerts to console log (can be extended with WS push/webhook)
  providerHealthChecker.onStatusChanged = (event) => {
    console.log(`[ProviderHealthChecker] Status change: account=${event.accountId} ${event.oldStatus}→${event.newStatus}`)
    // TODO: extend with rpcServer.broadcast('provider:statusChanged', event) and webhook call
  }
  // BUG-BE-HLD-015: wire quota warnings the same way status changes are wired.
  providerHealthChecker.onQuotaWarning = (event) => {
    console.warn(
      `[ProviderHealthChecker] Quota warning: account=${event.accountId} ` +
      `${event.tokensUsed}/${event.quotaLimitDay} (${Math.round(event.ratio * 100)}%)`
    )
    // TODO: extend with rpcServer.broadcast('provider:quotaWarning', event) and webhook call
  }
  // Register AI provider RPC methods into the already-running rpcServer
  rpcServer.addMethods(createAIProviderMethods(aiProviderService, providerResolver))
  console.log('[ServerBootstrap] ✅ AIProviderService + ProviderResolver + ProviderHealthChecker initialized (v5.0)')

  // B1 fix (docs/guides/fix-proposals-per-issue.md §B1): ProfileAwareAgentSpawner must exist
  // before project RPC methods are registered — createProjectMethods' 3rd (optional) param
  // wires project.agentSpawn; leaving it undefined made project.agentSpawn always throw
  // AGENT_SPAWNER_NOT_AVAILABLE. Needs _projectRouter (step 10) + aiProviderService (step 11),
  // so it can only be created here — TaskAgentExecutor (step 13) now reuses this instance.
  const { ProfileAwareAgentSpawner } = await import('./project/ProfileAwareAgentSpawner')
  const agentSpawner = new ProfileAwareAgentSpawner(_projectRouter, profileResolver, aiProviderService)

  // Register project RPC methods [T01]
  const { createProjectMethods } = await import('./project/project-rpc-handler')
  rpcServer.addMethods(createProjectMethods(projectService, getUserRole, agentSpawner))
  console.log('[ServerBootstrap] ✅ Project RPC methods registered (v5.0)')

  // 12. WorkflowOrchestrator + TemplateResolver + StepExecutors [v5.0 TDD-17]
  const { DAGBuilder } = await import('./workflow/DAGBuilder')
  const { WorkflowOrchestrator } = await import('./workflow/WorkflowOrchestrator')
  const { StepExecutors } = await import('./workflow/StepExecutors')
  const { TemplateResolver } = await import('./workflow/TemplateResolver')
  const { createWorkflowMethods } = await import('./workflow/workflow-rpc-handler')
  const dagBuilder = new DAGBuilder()
  // Note: _projectRouter from step 10 is used here — it is in scope
  // Note: _projectRouter from step 10, aiProviderService/providerResolver from step 11 —
  // FIX BUG-BE-HLD-008: cross-reference the AI Provider domain into the Workflow domain so
  // agent steps can pin a provider (F36's "mix Claude/GPT-4o across steps" use-case).
  const stepExecutors = new StepExecutors(_projectRouter, providerResolver, aiProviderService)
  const templateResolver = new TemplateResolver(pool)
  const workflowOrchestrator = new WorkflowOrchestrator(pool, dagBuilder, stepExecutors, _projectRouter)
  await workflowOrchestrator.resumeRunningExecutions().catch(err =>
    console.warn('[ServerBootstrap] resumeRunningExecutions (non-fatal):', (err as Error).message)
  )
  // Register workflow RPC methods into the already-running rpcServer
  rpcServer.addMethods(createWorkflowMethods(workflowOrchestrator, templateResolver, pool))
  console.log('[ServerBootstrap] ✅ WorkflowOrchestrator initialized (v5.0)')

  // 13. TaskService + TaskAgentExecutor [v5.0 TDD-18]
  const { TaskDAGValidator } = await import('./task/TaskDAGValidator')
  const { TaskService } = await import('./task/TaskService')
  const { TaskGrantService } = await import('./task/TaskGrantService')
  const { TaskAIPlanner } = await import('./task/TaskAIPlanner')
  const { TaskAgentExecutor } = await import('./task/TaskAgentExecutor')
  const { createTaskMethods } = await import('./task/task-rpc-handler')
  const taskDagValidator = new TaskDAGValidator(pool)
  const taskService = new TaskService(pool, taskDagValidator)
  const taskGrantService = new TaskGrantService(pool, taskService)
  const taskAIPlanner = new TaskAIPlanner(taskService, aiProviderService, _projectRouter)
  // Why: reuse the same agentSpawner instance wired into project.agentSpawn above (B1 fix)
  // instead of constructing a second one.
  const taskAgentExecutor = new TaskAgentExecutor(taskService, agentSpawner, taskGrantService)
  rpcServer.addMethods(createTaskMethods(taskService, taskGrantService, taskAIPlanner, taskAgentExecutor))
  console.log('[ServerBootstrap] ✅ TaskService + TaskAgentExecutor initialized (v5.0)')

  // 14. WorkspaceService [v5.0 TDD-19]
  const { WorkspaceService } = await import('./workspace/WorkspaceService')
  const { createWorkspaceMethods } = await import('./workspace/workspace-rpc-handler')
  const workspaceService = new WorkspaceService(
    _projectRouter, profileResolver, taskService, workflowOrchestrator, relayConnectionPool
  )
  rpcServer.addMethods(createWorkspaceMethods(workspaceService))
  console.log('[ServerBootstrap] ✅ WorkspaceService initialized (v5.0)')

  // 15. AutomationService [D1 fix — docs/guides/fix-proposals-per-issue.md §D1 /
  // docs/guides/audit-backend-agent-2026-08-13.md §D3]: this was never instantiated in
  // backend/src, so automation.runNow always threw runtime_unavailable and the rrule
  // scheduler never ran on the server. Mirrors desktop/src/main/index.ts's wiring.
  const { ClaudeUsageStore } = await import('./claude-usage/store')
  const { CodexUsageStore } = await import('./codex-usage/store')
  const claudeUsage = new ClaudeUsageStore(store)
  const codexUsage = new CodexUsageStore(store)
  const { AutomationService } = await import('./automations/service')
  const automationService = new AutomationService(store, {
    claudeUsage,
    codexUsage,
    // Why: unlike desktop (which only mirrors remote-host automations), this process
    // IS the server that owns executing schedules for `remote_host_service` targets.
    allowRemoteHostScheduling: true
    // TODO(D1, docs/guides/fix-proposals-per-issue.md §D1): headlessDispatcher is
    // intentionally left unset. Desktop's dispatcher (desktop/src/main/index.ts:~1810)
    // creates a managed worktree and spawns an agent via runtimeService — porting
    // that to this headless server needs its own design pass and is out of scope
    // here. Without it, a due run simply falls back to `skipped_unavailable`
    // (AutomationService.requestDispatch's existing no-dispatcher path) instead of
    // throwing — still a strict improvement over today's 100%-throw state, since
    // list/CRUD/precheck and the rrule scheduler itself now work.
  })
  automationService.start()
  runtime.setAutomationService(automationService)
  console.log('[ServerBootstrap] ✅ AutomationService initialized + scheduler started (D1 fix)')

  // 8. Wire FleetHealthMonitor (SOL-005 — CR-005: fleet health wiring)
  try {
    const { fleetHealthMonitor } = await import('./ssh/fleet-health-monitor')
    const { SshConnectionStore } = await import('./ssh/ssh-connection-store')
    const sshStore = new SshConnectionStore(store)
    // Wire dependency injection properties
    fleetHealthMonitor.getSshTargets = async () => {
      const sshTargets = sshStore.listTargets().map((t) => ({ id: t.id, label: t.label, project: t.project }))
      const listRes = await devServerManager.list()
      const devServers = listRes.map((ds: any) => ({ id: ds.id, label: ds.name }))
      // De-duplicate in case a dev server has the same ID as an SSH target
      const combined = new Map([...sshTargets, ...devServers].map(t => [t.id, t]))
      return Array.from(combined.values())
    }
    
    fleetHealthMonitor.getConnectionState = (targetId) => {
      // DevServerManager takes precedence for DevServers
      const dsState = typeof devServerManager.getRuntimeState === 'function' ? devServerManager.getRuntimeState(targetId) : null
      if (dsState) {
        return {
          status: dsState.status,
          error: dsState.lastError ? dsState.lastError.message : null,
          remotePlatform: dsState.platform
        }
      }
      
      // Fallback to legacy SSH connection state
      const conn = sshManager.getState(targetId)
      return conn ?? null
    }
    // FIX BUG-BE-HLD-010: expose the live SshConnection for exec-based
    // CPU/RAM/disk/latency probes. Reuses the same `sshManager` instance
    // already wired above for getConnectionState's legacy fallback.
    fleetHealthMonitor.getSshConnection = (targetId) => sshManager.getConnection(targetId)
    // IStateRepository exposes settings.get() — NOT getState()
    const settings = await stateRepo.settings.get().catch(() => null)
    if (settings?.fleetAlertWebhookUrl) {
      fleetHealthMonitor.getWebhookUrl = () => settings.fleetAlertWebhookUrl!
      console.log('[ServerBootstrap] ✅ Fleet alert webhook configured')
    }
    const pingIntervalMs = settings?.fleetHealthPingIntervalMs ?? 30_000 // FIX BUG-BE-HLD-010: was 60_000
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
    sessionManager,
    agentWsServer,
    // ─── v5.0 services (Phase 1+2 wired, later phases remain placeholder) ───
    relayConnectionPool,                                                          // TASK-012 ✅
    profileService,                                                               // TASK-012 ✅
    profileResolver,                                                              // TASK-012 ✅
    projectService,                                                               // TASK-020 ✅
    aiProviderService,                                                            // TASK-021 ✅
    workflowOrchestrator,                                                         // TASK-029 ✅
    taskService,                                                                  // TASK-041 ✅
    rpcAuthToken: rpcServer.getAuthToken(),                                       // BUG-PC-001 ✅
    automationService,                                                            // D1 fix ✅
    claudeUsage,                                                                  // D1 fix ✅
    codexUsage,                                                                   // D1 fix ✅
    async shutdown() {
      console.log('[ServerBootstrap] Shutting down...')
      // Stop AutomationService's scheduler first, before rpcServer/db teardown below.
      try {
        automationService.stop()
        console.log('[ServerBootstrap] ✅ AutomationService stopped')
      } catch (err) {
        console.warn('[ServerBootstrap] AutomationService stop error:', err)
      }
      // Stop agent WS server first (clear pending slots)
      try {
        agentWsServer.stop()
        console.log('[ServerBootstrap] ✅ AgentWebSocketServer stopped')
      } catch (err) {
        console.warn('[ServerBootstrap] AgentWebSocketServer stop error:', err)
      }
      try {
        if (authManager) {authManager.destroy()}
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
        if (daemonShutdown) {await daemonShutdown()}
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
      try {
        if (sessionManager) {await sessionManager.shutdown()}
        console.log('[ServerBootstrap] ✅ SessionManager shutdown complete')
      } catch (err) {
        console.warn('[ServerBootstrap] SessionManager shutdown error:', err)
      }
      // v5.0: disconnect all relay connections
      try {
        await relayConnectionPool.disconnectAll()
        console.log('[ServerBootstrap] ✅ RelayConnectionPool disconnected')
      } catch (err) {
        console.warn('[ServerBootstrap] RelayConnectionPool disconnect error:', err)
      }
      // v5.0: stop AI provider health checker
      try {
        providerHealthChecker.stop()
        console.log('[ServerBootstrap] ✅ ProviderHealthChecker stopped')
      } catch (err) {
        console.warn('[ServerBootstrap] ProviderHealthChecker stop error:', err)
      }
      console.log('[ServerBootstrap] Shutdown complete')
    }
  }
}
