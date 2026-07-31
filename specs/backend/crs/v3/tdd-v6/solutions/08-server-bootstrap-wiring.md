# Solution: Server Bootstrap v5.0 Wiring

**Mục tiêu:** Document trạng thái wiring hiện tại và các gaps còn lại.  
**Status:** ✅ Wiring DONE — `server-bootstrap.ts` đã initialize tất cả v5.0 services

---

## 1. Trạng thái Hiện Tại — server-bootstrap.ts

File `src/main/server-bootstrap.ts` (548 lines, 26.5KB) đã được update đầy đủ:

### ServerBootstrapResult Interface ✅

```typescript
export interface ServerBootstrapResult {
  shutdown(): Promise<void>
  // v4.0 services (existing)
  devServerManager: DevServerManager
  dbMonitor: HealthChecker
  pushManager: WebPushManager
  authManager: AuthManager
  sessionManager: SessionManager | null
  agentWsServer: AgentWebSocketServer
  rpcAuthToken: string
  // v5.0 services (TDD-14 → TDD-19)
  relayConnectionPool: RelayConnectionPool    // TDD-19
  profileService: ProfileService              // TDD-14
  profileResolver: ProfileResolver            // TDD-14
  projectService: ProjectService              // TDD-15
  aiProviderService: AIProviderService        // TDD-16
  workflowOrchestrator: WorkflowOrchestrator  // TDD-17
  taskService: TaskService                    // TDD-18
}
```

### Initialization Sequence (v5.0 additions)

```
Step 2a-pool: RelayConnectionPool(connectFn)         [TDD-19]
Step 9:       ProfileService(pool)                   [TDD-14]
              ProfileResolver(profileService)         [TDD-14]
Step 10:      ProjectService(pool, devServerManager)  [TDD-15]
              ProjectServerRouter(...)                [TDD-15]
Step 11:      AIProviderService(pool, dsm, pool)      [TDD-16]
              ProviderResolver(aiProviderService)     [TDD-16]
              ProviderHealthChecker.start()           [TDD-16]
              rpcServer.addMethods(aiProvider methods) [TDD-16]
Step 12:      DAGBuilder()                            [TDD-17]
              StepExecutors(projectRouter)            [TDD-17]
              TemplateResolver(pool)                  [TDD-17]
              WorkflowOrchestrator(pool, dag, step, router) [TDD-17]
              workflowOrchestrator.resumeRunningExecutions()  [TDD-17]
              rpcServer.addMethods(workflow methods)  [TDD-17]
Step 13:      TaskDAGValidator(pool)                  [TDD-18]
              TaskService(pool, taskDagValidator)     [TDD-18]
              TaskGrantService(pool, taskService)     [TDD-18]
              TaskAIPlanner(taskService, ...)         [TDD-18]
              ProfileAwareAgentSpawner(router, ...)   [TDD-15]
              TaskAgentExecutor(task, spawner, grant) [TDD-18]
              rpcServer.addMethods(task methods)      [TDD-18]
Step 14:      WorkspaceService(router, resolver, ...) [TDD-19]
              rpcServer.addMethods(workspace methods) [TDD-19]
```

---

## 2. ✅ Gaps đã được khắc phục — 2026-07-30T23:43 ICT

### 2.1 Profile RPC Methods — ✅ Đã register

```typescript
// src/main/server-bootstrap.ts (line 387-388) — đã có:
const { createProfileMethods } = await import('./profile/profile-rpc-handler')
rpcServer.addMethods(createProfileMethods(profileService, profileResolver))
```

### 2.2 Project RPC Methods — ✅ Đã register

```typescript
// src/main/server-bootstrap.ts (line 399-400) — đã có:
const { createProjectMethods } = await import('./project/project-rpc-handler')
rpcServer.addMethods(createProjectMethods(projectService))
```

### 2.3 TDD-20 Git Remote RPC — ✅ Đã tạo (routing qua relay)

```typescript
// src/main/runtime/rpc/methods/git-remote-rpc.ts đã tạo (9 methods)
// Relay tier chọn git-remote-handler-v6.ts khi __ORCA_GIT_V6__ = true
// thông qua git-remote-handler-index.ts selector
```

---

## 3. http-server.ts — v5.0 Additions

### Trạng thái hiện tại

`src/server/http-server.ts` đã có:
- `cookie-parser` → AuthMiddleware → AuthRouter `/auth`
- Health endpoints `/health`, `/health/ready`, `/health/metrics`
- Push API routes `/push`

### Gaps cần thêm

```typescript
// src/server/http-server.ts — cần thêm:

// Option 1: Profile/Project/Task REST endpoints (nếu cần REST ngoài RPC)
// → Không cần thiết nếu tất cả đều qua WebSocket RPC

// Option 2: workspace.poolStatus admin endpoint
app.get('/admin/api/workspace/pool', requireAdmin, (req, res) => {
  res.json(relayConnectionPool.getStatus())
})
```

> **Kết luận:** Không có HTTP endpoint mới cần thiết cho v5.0 —  
> tất cả APIs đều qua WebSocket RPC (`OrcaRuntimeRpcServer`).

---

## 4. ALL_MIGRATIONS — ✅ Xác nhận 0006→0010

```typescript
// src/main/db/migrations/index.ts — đã có đủ 10 entries:
export const ALL_MIGRATIONS = [
  // ... 0001-0005 ...
  migration0006CompanyDept,  // ✅ orca_company, orca_departments
  migration0007Projects,     // ✅ orca_projects, orca_project_members
  migration0008AiProviders,  // ✅ orca_ai_provider_accounts, orca_provider_usage
  migration0009Workflows,    // ✅ orca_workflow_templates, orca_workflow_executions
  migration0010Tasks,        // ✅ orca_tasks, orca_task_edges, orca_task_grants
]
```

---

## 5. Summary — server-bootstrap.ts Status

| Change | Priority | Status |
|--------|----------|--------|
| Register profile RPC methods | HIGH | ✅ L387-388 |
| Register project RPC methods | HIGH | ✅ L399-400 |
| Git remote handlers (TDD-20) | MEDIUM | ✅ routing qua relay |
| ALL_MIGRATIONS 0006-0010 | HIGH | ✅ index.ts confirmed |
| workspace/pool admin endpoint | LOW | optional, not required |

> **Đã hoàn tất:** 2026-07-30T23:43 ICT
