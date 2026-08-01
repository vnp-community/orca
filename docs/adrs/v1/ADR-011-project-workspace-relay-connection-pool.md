# ADR-011 — Project Workspace via AgentConnectionManager + WorkspaceContext

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-011 |
| **Trạng thái** | ⚠️ Amended (v6.0 — 2026-07-30) |
| **Ngày** | 2026-07-28 |
| **Cập nhật** | 2026-07-30 (v6.0 amendment) |
| **HLD Ref** | C3.12, C3.13, C4.10, C4.11 |
| **Code Ref** | `src/renderer/src/context/WorkspaceContext.tsx`, `src/main/dev-server/agent-connection-manager.ts`, `src/main/dev-server/agent-dispatcher.ts` |
| **Feature Ref** | F38 |
| **Liên quan** | ADR-013 (Dev Server Agent), ADR-014 (JSON-RPC Protocol), ADR-015 (Signed Context) |

---

> **🔄 AMENDMENT (v6.0 — 2026-07-30)**  
> ADR-011 được cập nhật trong v6.0 để phản ánh việc `RelayConnectionPool` được thay bằng `AgentConnectionManager` + `AgentDispatcher`.  
> **Nguyên tắc không đổi:** 1 connection per dev server, shared across panels, WorkspaceContext micro-emitter.  
> **Thay đổi:** Transport layer từ SSH relay → persistent WebSocket đến Agent; protocol từ binary frame → JSON-RPC 2.0 với signed context.

---

## Bối cảnh

Khi user chọn project, tất cả workspace panels (Explorer, Git, Agent, Workflows, Tasks, Terminal) đều cần:
1. Access cùng **relay connection** đến dev server
2. Chia sẻ **git status** và **worktree state**
3. Nhận **events** từ nhau (agent complete → refresh Explorer)
4. Handle **offline** khi dev server unreachable

Nếu mỗi panel tự quản lý connection riêng:
- N panels × 1 SSH connection = N connections (expensive)
- Events không propagate giữa panels
- Inconsistent state (Git tab stale khi Explorer refreshes)

---

## Quyết định

### 1. AgentConnectionManager (thay RelayConnectionPool, v6.0)

```typescript
// src/main/dev-server/agent-connection-manager.ts
class AgentConnectionManager {
  // Map agentId -> persistent WS connection
  private connections = new Map<string, AgentConnection>()

  // Get agent connection for devServerId
  getConnection(agentId: string): AgentConnection | null

  // Called when agent connects (outbound from agent)
  registerConnection(agentId: string, ws: WebSocket, capabilities: AgentCapabilities): void

  // Dispatch RPC call to agent with signed context
  async dispatch<T>(
    agentId: string,
    method: string,
    params: unknown,
    ctx: RpcExecutionContext
  ): Promise<T>

  // Subscribe to events from a specific agent
  onEvent(agentId: string, handler: (event: AgentEvent) => void): () => void

  // Health: list all connected agents
  listConnected(): { agentId: string; capabilities: AgentCapabilities; connectedAt: number }[]
}
```

**Thay đổi từ RelayConnectionPool:**
- `getOrConnect(devServerId)` → agents tự kết nối vào (không cần Gateway initiate)
- `release(devServerId)` → không cần (persistent connection, agent manages lifecycle)
- Connection reuse vẫn giữ: 1 WS connection per agent → nhiều user dùng chung

### 2. WorkspaceContext (React Context — Client-side)

```typescript
// src/renderer/src/context/WorkspaceContext.tsx
interface WorkspaceContextValue {
  project: OrcaProject | null
  devServer: SshHost | null
  isConnected: boolean
  isOffline: boolean
  relay: DevServerRelayBridge | null  // proxy via RPC to server-side pool
  currentWorktree: Worktree | null
  availableWorktrees: Worktree[]
  gitStatus: GitStatus | null
  resolvedProfile: ResolvedProfile | null
  activeAgentSessionId: string | null

  // Actions
  switchProject(projectId: string): Promise<void>
  setCurrentWorktree(wt: Worktree): void
  refreshGitStatus(): Promise<void>
  setActiveAgentSession(id: string | null): void

  // Event bus
  emit(event: WorkspaceEvent): void
  on(event: string, handler: Function): () => void  // returns unsubscribe
}

// Workspace event bus (micro-emitter, không dùng Redux)
type WorkspaceEvent =
  | { type: 'agent.complete'; filesChanged: number; sessionId: string }
  | { type: 'git.commit'; hash: string; message: string }
  | { type: 'git.push'; branch: string }
  | { type: 'worktree.switched'; path: string; branch: string }
  | { type: 'workflow.step.complete'; stepId: string }
```

### 3. Project Switch Flow

```typescript
async function switchProject(projectId: string): Promise<void> {
  // 1. Teardown previous workspace
  if (currentProject) {
    gitStatusPoller.stop()
    // No explicit release needed — AgentConnectionManager manages persistent WS
  }

  // 2. Load project + agent mapping
  const [project, agentId] = await Promise.all([
    rpc.call('project.get', { projectId }),
    rpc.call('project.resolveAgent', { projectId }),  // devServerId → agentId
  ])

  // 3. Check agent health (from AgentConnectionManager)
  const health = await rpc.call('agent.health', { agentId })
  setIsOffline(health.status !== 'online')

  // 4. Build signed context for this user + project
  const ctx = await rpc.call('context.build', { userId, projectId })  // Gateway signs

  // 5. Parallel init via Agent RPC
  const [gitStatus, worktrees, fileTree, workflows] = await Promise.all([
    rpc.call('agent.git.status', { agentId, params: { repoPath: project.repoPath }, ctx }),
    rpc.call('agent.worktree.list', { agentId, params: { repoPath: project.repoPath }, ctx }),
    rpc.call('agent.fs.readDir', { agentId, params: { path: project.repoPath, depth: 2 }, ctx }),
    rpc.call('workflow.getActive', { projectId }),  // Gateway-local
  ])

  // 6. Set context
  setProject(project)
  setGitStatus(parseGitStatus(gitStatus))
  setAvailableWorktrees(worktrees)
  setCurrentWorktree(worktrees.find(w => w.isMain) ?? worktrees[0])

  // 7. Subscribe to agent events (git changes, agent status)
  agentEventSub = rpc.subscribeAgentEvents(agentId, projectId, handleAgentEvent)

  // 8. Start git status poll (poll via agent events, not timer)
  // Agent emits event.gitChange on debounced fs events — no need for 5s timer
}
```

### 4. Cross-panel Event Bus

```typescript
// Built-in micro-emitter trong WorkspaceContext
// No external event library needed

// Agent panel fires:
emit({ type: 'agent.complete', filesChanged: 3, sessionId: 'sess-xxx' })

// Explorer subscribes:
on('agent.complete', () => refreshFileTree())

// Git panel subscribes:
on('agent.complete', () => refreshGitStatus())  // immediate, skip 5s poll

// Tasks panel subscribes:
on('agent.complete', ({ sessionId }) => checkLinkedTaskUpdate(sessionId))
```

---

## Lý do chọn

| Lựa chọn | Đánh giá |
|----------|---------|
| **RelayConnectionPool + WorkspaceContext + micro-emitter** ✅ | Single connection per server; React-native; no external state lib |
| Each panel manages own connection | N SSH connections; inconsistent state |
| Redux global state | Overkill; complex boilerplate; no built-in events |
| Zustand per panel | State not shared; events manual |
| WebSocket pub/sub from server | Extra round-trip for local events |

---

## Hậu quả

**Tích cực:**
- 1 SSH connection per dev server (tái dụng)
- Cross-panel events instant (client-side emitter)
- Offline mode: cached state, disable write ops
- Git status centralized → all panels see same state

**Tiêu cực:**
- RelayConnectionPool server-side: cần userId context trong relay calls (tránh cross-user access)
- WorkspaceContext re-render: phải memo selectors cẩn thận
- Event bus memory: unsubscribe phải được gọi trong useEffect cleanup
- Project switch: teardown cũ trước init mới → có khoảng trống UI

---

## Trạng thái Implementation (v6.0)

❌ Chưa implement (v6.0 proposed)  
🎯 `src/main/dev-server/agent-connection-manager.ts` (thay relay-connection-pool.ts)  
🎯 `src/main/dev-server/agent-dispatcher.ts`  
🎯 `src/renderer/src/context/WorkspaceContext.tsx`  
🎯 `src/renderer/src/components/workspace/WorkspaceLayout.tsx`  
🎯 `src/renderer/src/components/workspace/ExplorerPanel.tsx`  
🎯 Project RPC methods: `project.get`, `project.resolveAgent`, `agent.git.status`, `agent.fs.readDir`
