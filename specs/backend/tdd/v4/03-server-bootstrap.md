# TDD-BE-03: Server Bootstrap

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/main/server-bootstrap.ts`

---

## 1. `ServerBootstrapOptions`

```typescript
export interface ServerBootstrapOptions {
  platform:    IPlatformServices
  port?:       number           // RPC WebSocket port (default: 6768)
  database?:   DatabaseConfig | null  // null = force JSON fallback
  isUserProcess?: boolean       // true khi chạy trong user fork
}
```

---

## 2. `ServerBootstrapResult`

```typescript
export interface ServerBootstrapResult {
  shutdown():        Promise<void>
  devServerManager: DevServerManager
  dbMonitor:        HealthChecker
  pushManager:      WebPushManager
  authManager:      AuthManager
  sessionManager:   SessionManager | null
  agentWsServer:    AgentWebSocketServer
}
```

---

## 3. Initialization Sequence (initializeOrcaServices)

```
1.  initDataPath()                    — resolve userData path
2.  new Store()                       — SQLite persistence (Electron-compat)
3.  new WebPushManager(store)         — VAPID key init
4.  new AgentWebSocketServer(ver)     — không attach httpServer ở đây
    new DevServerManager(store, sshMgr, agentWsServer)
5.  initDataPath()                    — data path (stats collector)
6.  new OrcaStatsCollector(store)     — usage stats
7.  loadDatabaseConfig()              — env vars → DatabaseConfig
8.  initConnectionPool(config)        — IConnectionPool (SQLite/MySQL/PG)
9.  runMigrations(pool)               — auto-migrate 0001→0005
10. new HealthChecker(pool)           — DB monitor (10s interval)
11. initStateRepository(pool)         — IStateRepository (SQL or JSON)
12. new AuthManager(authDb)           — bcrypt, sessions, 30min cleanup
13. ensureFirstAdminUser()            — seed admin if no admin exists
14. registerIpcHandlers()             — dev-server, onboarding, repo-remote
15. initOrcaRuntimeService(...)       — core runtime (worktrees, git, PTY)
16. new OrcaRuntimeRpcServer(port)    — WebSocket :6768 (or Unix socket if user process)
17. devServerManager.restoreConnections()  — emit 'connecting' for direct-ws servers
```

---

## 4. Auth Initialization Detail

```typescript
// AuthManager dùng dedicated SQLite connection (KHÔNG dùng chung với app DB)
// Lý do: auth data phải luôn available ngay cả khi main DB đang migrate

const authDb = initAuthDatabase(userDataPath)  // userData/auth.db
const authManager = new AuthManager({
  db:               authDb,
  sessionStore:     new AuthSessionStore(authDb),
  userStore:        new AuthUserStore(authDb),
  localHandler:     new AuthLocalHandler(authDb),
  cleanupInterval:  30 * 60 * 1000   // 30 phút
})
```

---

## 5. User Process Mode (isUserProcess=true)

Khi `isUserProcess=true` (fork từ SessionManager):
- KHÔNG khởi tạo AuthManager (supervisor xử lý auth)
- KHÔNG khởi tạo DevServerManager (supervisor owns it)
- OrcaRuntimeRpcServer lắng nghe trên Unix socket (`ORCA_SOCKET_PATH`)
- Receive IPC messages từ supervisor (`devServer:event`)

---

## 6. Migration 0005 (Auth Schema)

Tạo 4 tables:
- `orca_users` — id, email, name, role, provider, password_hash, active
- `orca_sessions` — session_id, user_id, expires_at, ip_address, user_agent
- `orca_audit_log` — id, user_id, email, action, detail, ip, created_at
- `orca_access_policies` — id, name, effect, resource, action, condition
