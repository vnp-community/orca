# TASK-005: Extend ServerBootstrapResult Interface

**Phase:** 1 — Foundation  
**Solution ref:** [SOL-V5-000](../solutions/SOL-V5-000-server-bootstrap-changes.md) §1, §2  
**Prerequisite:** TASK-004 (shared types phải tồn tại để TypeScript resolve)  
**Status:** ✅ DONE — 2026-07-28

> **Kết quả:** `ServerBootstrapResult` extended với 7 fields v5.0 (RelayConnectionPool, ProfileService, ProfileResolver, ProjectService, AIProviderService, WorkflowOrchestrator, TaskService). Return block có placeholder `undefined as unknown as T`. Zero TypeScript errors.


---

## Mô tả

Mở rộng `ServerBootstrapResult` interface trong `src/main/server-bootstrap.ts` để include v5.0 services. Đây là **interface-only change** — không thêm implementation logic chưa (implementation sẽ thêm ở các TASK sau theo Phase).

---

## File cần đọc

```bash
head -50 src/main/server-bootstrap.ts
```

---

## Thay đổi cần thực hiện

Tìm và modify `ServerBootstrapResult` interface (khoảng line 27–45 trong file hiện tại):

**Before:**
```typescript
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
  /**
   * SessionManager — forks per-user child processes in multi-user mode.
   * null when ORCA_MULTI_USER is not set (single-user / Electron mode).
   */
  sessionManager: import('./session/session-manager').SessionManager | null
  /** AgentWebSocketServer — attach to HTTP server for direct-websocket agent connections */
  agentWsServer: AgentWebSocketServer
}
```

**After:**
```typescript
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
  /**
   * SessionManager — forks per-user child processes in multi-user mode.
   * null when ORCA_MULTI_USER is not set (single-user / Electron mode).
   */
  sessionManager: import('./session/session-manager').SessionManager | null
  /** AgentWebSocketServer — attach to HTTP server for direct-websocket agent connections */
  agentWsServer: AgentWebSocketServer
  // ─── v5.0 services ────────────────────────────────────────────────────────
  /** RelayConnectionPool — manages relay connections with ref-counting (v5.0) */
  relayConnectionPool: import('./dev-server/relay-connection-pool').RelayConnectionPool
  /** ProfileService — company/dept/user profile CRUD (v5.0 TDD-14) */
  profileService: import('./profile/ProfileService').ProfileService
  /** ProfileResolver — 3-layer merge + cache (v5.0 TDD-14) */
  profileResolver: import('./profile/ProfileResolver').ProfileResolver
  /** ProjectService — project CRUD + member management (v5.0 TDD-15) */
  projectService: import('./project/ProjectService').ProjectService
  /** AIProviderService — AI provider accounts + relay credential (v5.0 TDD-16) */
  aiProviderService: import('./ai-providers/AIProviderService').AIProviderService
  /** WorkflowOrchestrator — DAG-based multi-server workflow (v5.0 TDD-17) */
  workflowOrchestrator: import('./workflow/WorkflowOrchestrator').WorkflowOrchestrator
  /** TaskService — task graph CRUD + BFS tree ops (v5.0 TDD-18) */
  taskService: import('./task/TaskService').TaskService
}
```

---

## Implementation Note

Sau khi chỉnh sửa interface, TypeScript sẽ báo lỗi tại `return { ... }` cuối hàm `initializeOrcaServices()` vì thiếu các fields mới. Đây là **expected** — các fields này sẽ được thêm dần theo từng Phase.

Để tạm thời tắt lỗi TypeScript trong khi dev, thêm `// @ts-expect-error v5.0 services pending` trước return statement, hoặc cast: `return { ...existingFields } as ServerBootstrapResult`.

**Hoặc cách đơn giản hơn:** Thêm `undefined` placeholder vào return cho giai đoạn đầu:

```typescript
return {
  // ... existing fields ...
  // [v5.0] — sẽ được thay thế khi từng service init xong
  relayConnectionPool: undefined as unknown as any,
  profileService: undefined as unknown as any,
  profileResolver: undefined as unknown as any,
  projectService: undefined as unknown as any,
  aiProviderService: undefined as unknown as any,
  workflowOrchestrator: undefined as unknown as any,
  taskService: undefined as unknown as any,
  ...
}
```

---

## Verification

```bash
pnpm tsc --noEmit 2>&1 | head -20
```

## Acceptance Criteria

- [x] `ServerBootstrapResult` interface có 7 fields v5.0 mới
- [x] Type imports dùng `import(...)` inline (không thêm top-level import)
- [x] File không break existing tests
