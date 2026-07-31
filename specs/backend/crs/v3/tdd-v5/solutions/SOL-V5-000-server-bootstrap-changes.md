# SOL-V5-000: server-bootstrap.ts — Full Diff Summary

**Document:** SOL-V5-000  
**Purpose:** Tổng hợp tất cả changes cần thiết cho `src/main/server-bootstrap.ts` để implement TDD v5.0  
**Date:** 2026-07-28  
**Status:** ✅ IMPLEMENTED  
**Implementation Date:** 2026-07-29  
**Tests:** 289/289 pass | TypeScript: 0 errors  

---

## 1. Updated `ServerBootstrapResult` interface

```typescript
export interface ServerBootstrapResult {
  /** Existing fields — KHÔNG thay đổi */
  shutdown(): Promise<void>
  devServerManager: DevServerManager
  dbMonitor: import('./db/health').HealthChecker
  pushManager: WebPushManager
  authManager: AuthManager
  sessionManager: import('./session/session-manager').SessionManager | null
  agentWsServer: AgentWebSocketServer

  /** [NEW v5.0] — thêm vào sau agentWsServer */
  profileService: import('./profile/ProfileService').ProfileService
  profileResolver: import('./profile/ProfileResolver').ProfileResolver
  projectService: import('./project/ProjectService').ProjectService
  aiProviderService: import('./ai-providers/AIProviderService').AIProviderService
  workflowOrchestrator: import('./workflow/WorkflowOrchestrator').WorkflowOrchestrator
  taskService: import('./task/TaskService').TaskService
  relayConnectionPool: import('./dev-server/relay-connection-pool').RelayConnectionPool
}
```

---

## 2. Initialization sequence — v5.0 additions

Các bước dưới đây được **thêm vào** sau step 2a (DevServerManager) và trước step 3 (StatsCollector):

### Step 2a-pool — RelayConnectionPool *(MUST be first — prerequisite for all v5.0 services)*

```typescript
// Sau DevServerManager initialization (line ~118 hiện tại):

// 2a-pool. Initialize RelayConnectionPool (prerequisite cho Project + AI Provider)
const { RelayConnectionPool } = await import('./dev-server/relay-connection-pool')
const { DevServerRelayBridge } = await import('./dev-server/dev-server-relay-bridge')
const relayConnectionPool = new RelayConnectionPool(async (server) => {
  const bridge = new DevServerRelayBridge(server, sshManager, agentWsServer)
  await bridge.connect()
  return bridge
})
console.log('[ServerBootstrap] ✅ RelayConnectionPool initialized')
```

### Step 7 — ProfileService + ProfileResolver *(sau OrcaRuntimeRpcServer start)*

```typescript
// Sau step 6 (OrcaRuntimeRpcServer):

// 7. ProfileService + ProfileResolver [v5.0]
const { ProfileService } = await import('./profile/ProfileService')
const { ProfileResolver } = await import('./profile/ProfileResolver')
const profileService = new ProfileService(pool)
const profileResolver = new ProfileResolver(profileService)
console.log('[ServerBootstrap] ✅ ProfileService + ProfileResolver initialized')
```

### Step 8 — ProjectService + ProjectServerRouter

```typescript
// 8. ProjectService + ProjectServerRouter [v5.0]
const { ProjectService } = await import('./project/ProjectService')
const { ProjectServerRouter } = await import('./project/ProjectServerRouter')
const projectService = new ProjectService(pool, devServerManager)
const projectRouter = new ProjectServerRouter(projectService, devServerManager, relayConnectionPool)
console.log('[ServerBootstrap] ✅ ProjectService + ProjectServerRouter initialized')
```

### Step 9 — AIProviderService + ProviderHealthChecker

```typescript
// 9. AIProviderService + ProviderHealthChecker [v5.0]
const { AIProviderService } = await import('./ai-providers/AIProviderService')
const { ProviderHealthChecker } = await import('./ai-providers/ProviderHealthChecker')
const aiProviderService = new AIProviderService(pool, devServerManager, relayConnectionPool)
const providerHealthChecker = new ProviderHealthChecker()
providerHealthChecker.start(aiProviderService, relayConnectionPool)
console.log('[ServerBootstrap] ✅ AIProviderService initialized')
```

### Step 10 — WorkflowOrchestrator

```typescript
// 10. WorkflowOrchestrator [v5.0]
const { DAGBuilder } = await import('./workflow/DAGBuilder')
const { WorkflowOrchestrator } = await import('./workflow/WorkflowOrchestrator')
const { StepExecutors } = await import('./workflow/StepExecutors')
const dagBuilder = new DAGBuilder()
const stepExecutors = new StepExecutors(projectRouter)
const workflowOrchestrator = new WorkflowOrchestrator(pool, dagBuilder, stepExecutors, projectRouter)
await workflowOrchestrator.resumeRunningExecutions().catch(err =>
  console.warn('[ServerBootstrap] resumeRunningExecutions (non-fatal):', err.message)
)
console.log('[ServerBootstrap] ✅ WorkflowOrchestrator initialized')
```

### Step 11 — TaskService + TaskAgentExecutor

```typescript
// 11. TaskService + TaskAgentExecutor [v5.0]
const { TaskService } = await import('./task/TaskService')
const { TaskDAGValidator } = await import('./task/TaskDAGValidator')
const { TaskGrantService } = await import('./task/TaskGrantService')
const { ProfileAwareAgentSpawner } = await import('./project/ProfileAwareAgentSpawner')
const { TaskAgentExecutor } = await import('./task/TaskAgentExecutor')
const taskDagValidator = new TaskDAGValidator(pool)
const taskService = new TaskService(pool, taskDagValidator)
const taskGrantService = new TaskGrantService(pool, taskService)
const agentSpawner = new ProfileAwareAgentSpawner(projectRouter, profileResolver, aiProviderService)
const taskAgentExecutor = new TaskAgentExecutor(taskService, agentSpawner, taskGrantService)
console.log('[ServerBootstrap] ✅ TaskService + TaskAgentExecutor initialized')
```

### Step 12 — WorkspaceService

```typescript
// 12. WorkspaceService [v5.0]
const { WorkspaceService } = await import('./workspace/WorkspaceService')
const workspaceService = new WorkspaceService(
  projectRouter, profileResolver, taskService, workflowOrchestrator, relayConnectionPool
)
console.log('[ServerBootstrap] ✅ WorkspaceService initialized')
```

### Wire Remote Git RPC Methods *(sau rpcServer.start())*

```typescript
// Wire remote git methods (sau rpcServer.start(), trước FleetHealthMonitor):
try {
  const { registerRemoteGitRpcMethods } = await import('./runtime/rpc/methods/git-remote')
  registerRemoteGitRpcMethods(projectRouter, aiProviderService, taskService, taskGrantService, rpcServer.dispatcher)
  console.log('[ServerBootstrap] ✅ Remote git RPC methods registered')
} catch (err) {
  console.warn('[ServerBootstrap] Remote git RPC methods failed (non-fatal):', (err as Error)?.message)
}
```

---

## 3. Updated `return` statement

```typescript
return {
  // Existing fields — UNCHANGED
  devServerManager,
  dbMonitor,
  pushManager,
  authManager: authManager!,
  sessionManager,
  agentWsServer,

  // [NEW v5.0]
  profileService,
  profileResolver,
  projectService,
  aiProviderService,
  workflowOrchestrator,
  taskService,
  relayConnectionPool,

  async shutdown() {
    // Existing shutdown steps — UNCHANGED
    // ...

    // [NEW v5.0] — thêm vào shutdown:
    try {
      providerHealthChecker.stop()
      console.log('[ServerBootstrap] ✅ ProviderHealthChecker stopped')
    } catch (err) {
      console.warn('[ServerBootstrap] ProviderHealthChecker stop error:', err)
    }
    try {
      await relayConnectionPool.disconnectAll()
      console.log('[ServerBootstrap] ✅ RelayConnectionPool disconnected')
    } catch (err) {
      console.warn('[ServerBootstrap] RelayConnectionPool disconnect error:', err)
    }
  }
}
```

---

## 4. `src/main/db/migrations/index.ts` — full updated

```typescript
import { migration0001InitialSchema } from './0001_initial_schema'
import { migration0002AddAutomations } from './0002_add_automations'
import { migration0003AddWorkspaceSessions } from './0003_add_workspace_sessions'
import { migration0004OrcaAppTables } from './0004_orca_app_tables'
import { migration0005AddAuthSchema } from './0005_add_auth_schema'
import { migration0006CompanyDept } from './0006_company_dept'      // [NEW v5.0]
import { migration0007Projects } from './0007_projects'              // [NEW v5.0]
import { migration0008AiProviders } from './0008_ai_providers'      // [NEW v5.0]
import { migration0009Workflows } from './0009_workflows'            // [NEW v5.0]
import { migration0010Tasks } from './0010_tasks'                    // [NEW v5.0]
import type { Migration } from './types'

export const ALL_MIGRATIONS: Migration[] = [
  migration0001InitialSchema,
  migration0002AddAutomations,
  migration0003AddWorkspaceSessions,
  migration0004OrcaAppTables,
  migration0005AddAuthSchema,
  migration0006CompanyDept,   // [NEW]
  migration0007Projects,      // [NEW]
  migration0008AiProviders,   // [NEW]
  migration0009Workflows,     // [NEW]
  migration0010Tasks,         // [NEW]
]
```

---

## 5. Summary — Files Changed vs New

### Modified (existing files)

| File | Change |
|------|--------|
| `src/main/server-bootstrap.ts` | Add steps 2a-pool, 7–12; extend interface; update return + shutdown |
| `src/main/db/migrations/index.ts` | Add imports + entries for 0006–0010 |

### New Files (v5.0)

| File | SOL |
|------|-----|
| `src/main/db/migrations/0006_company_dept.ts` | SOL-001 |
| `src/main/db/migrations/0007_projects.ts` | SOL-002 |
| `src/main/db/migrations/0008_ai_providers.ts` | SOL-003 |
| `src/main/db/migrations/0009_workflows.ts` | SOL-004 |
| `src/main/db/migrations/0010_tasks.ts` | SOL-005 |
| `src/main/profile/OrcaProfile.ts` | SOL-001 |
| `src/main/profile/ProfileService.ts` | SOL-001 |
| `src/main/profile/ProfileResolver.ts` | SOL-001 |
| `src/main/profile/__tests__/` (3 files) | SOL-001 |
| `src/shared/project-types.ts` | SOL-002 |
| `src/main/project/ProjectService.ts` | SOL-002 |
| `src/main/project/ProjectServerRouter.ts` | SOL-002 |
| `src/main/project/ProfileAwareAgentSpawner.ts` | SOL-002 |
| `src/main/project/__tests__/` (4 files) | SOL-002 |
| `src/shared/ai-provider-types.ts` | SOL-003 |
| `src/main/ai-providers/AIProviderService.ts` | SOL-003 |
| `src/main/ai-providers/ProviderResolver.ts` | SOL-003 |
| `src/main/ai-providers/ProviderHealthChecker.ts` | SOL-003 |
| `src/relay/ai-provider-handler.ts` | SOL-003 |
| `src/main/ai-providers/__tests__/` (4 files) | SOL-003 |
| `src/main/workflow/WorkflowTypes.ts` | SOL-004 |
| `src/main/workflow/DAGBuilder.ts` | SOL-004 |
| `src/main/workflow/WorkflowOrchestrator.ts` | SOL-004 |
| `src/main/workflow/TemplateResolver.ts` | SOL-004 |
| `src/main/workflow/StepExecutors.ts` | SOL-004 |
| `src/main/workflow/__tests__/` (4 files) | SOL-004 |
| `src/shared/task-types.ts` | SOL-005 |
| `src/main/task/TaskService.ts` | SOL-005 |
| `src/main/task/TaskDAGValidator.ts` | SOL-005 |
| `src/main/task/TaskGrantService.ts` | SOL-005 |
| `src/main/task/TaskAIPlanner.ts` | SOL-005 |
| `src/main/task/TaskAgentExecutor.ts` | SOL-005 |
| `src/main/task/__tests__/` (6 files) | SOL-005 |
| `src/main/dev-server/relay-connection-pool.ts` | SOL-006 |
| `src/main/workspace/WorkspaceService.ts` | SOL-006 |
| `src/renderer/src/context/WorkspaceContext.tsx` | SOL-006 |
| `src/renderer/src/hooks/useWorkspace.ts` | SOL-006 |
| `src/main/dev-server/__tests__/relay-connection-pool.test.ts` | SOL-006 |
| `src/main/workspace/__tests__/WorkspaceService.test.ts` | SOL-006 |
| `src/relay/git-handler.ts` | SOL-007 |
| `src/main/runtime/rpc/methods/git-remote.ts` | SOL-007 |
| `src/renderer/src/components/workspace/git/*.tsx` (5 files) | SOL-007 |
| `src/relay/__tests__/git-handler.test.ts` | SOL-007 |
| `src/main/runtime/rpc/methods/__tests__/git-remote.test.ts` | SOL-007 |

**Total modified:** 2 files  
**Total new:** ~52 files  
**Zero files deleted**
