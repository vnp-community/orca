# SOL-BE-TRACE-002: Agent Orchestration — Backend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-002](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-002-agent-orchestration.md)
**TDD Ref:** TDD-08 (Agent Orchestration) + cross-ref TDD-15/TDD-18 và Addendum F.11 của `00-index.md` (xem 1.1 — TDD-08 mô tả một luồng khác với code thật)
**Date:** 2026-08-02
**Status:** Proposed
**Test Targets:** ≥ 16 tests (xem mục 3)
**Strategy:** Additive-only — instrument điểm hội tụ duy nhất `ProfileAwareAgentSpawner.spawn()`, không sửa business logic; propagate `traceId` xuyên 2 caller thật (RPC trực tiếp + Task Graph executor)

---

## 1. Phân tích phạm vi (Backend-side only)

### 1.1 Lệch giữa TDD-08 và code thật — quan trọng, đọc trước khi implement

`specs/backend/tdd/v5/08-agent-orchestration.md` mô tả **inter-agent messaging protocol** (`orchestration.send`/`orchestration.check`, `Coordinator`, `OrchestrationDb` — bảng SQLite `messages`/`tasks`/`dispatch_contexts`). Đây là một hệ thống **khác** với luồng "spawn 1 AI agent CLI cho 1 user request" mà CR-TRACE-002 nhắm tới (`ProfileAwareAgentSpawner.spawn()`). Cả 2 đều tồn tại trong code (`src/main/runtime/orchestration/` cho Coordinator/messaging, `src/main/project/ProfileAwareAgentSpawner.ts` cho spawn) nhưng phục vụ 2 mục đích khác nhau:

| | TDD-08 (`orchestration.*`) | CR-TRACE-002 (`agentOrch:*`) |
|---|---|---|
| Đối tượng | Multi-agent fan-out, message passing giữa các agent đã chạy | Spawn **một** agent process qua relay |
| Entry point | `orchestration.send`/`.check`/`.create-task` RPC (`src/main/runtime/orchestration/`) | `project.agentSpawn`, `task.execute` RPC (xem 1.2) |
| Nằm trong phạm vi SOL này? | **Không** | Có |

TDD-08 không phải tài liệu "directly on point" cho `ProfileAwareAgentSpawner` như brief ban đầu giả định — nó mô tả một domain liền kề (multi-agent coordination) chạy trên cùng nền `OrcaRuntimeService` nhưng không đi qua `ProfileAwareAgentSpawner`/`ProjectServerRouter`. Cross-reference gần đúng nhất cho `ProfileAwareAgentSpawner` trong bộ TDD là `00-index.md` Addendum F.11 (`TaskService — Grant Resolution`, bước 6: `relay.call('agent.spawn', {...})`) — dù field name F.11 dùng (`agent.spawn`) cũng khác code thật (`agent.exec`, đã confirm — xem 1.3). SOL này bám theo code thật, ghi chú rõ điểm lệch thay vì theo TDD/F.11 verbatim.

### 1.2 Phát hiện bổ sung so với CR-TRACE-002 — 2 call site thật vào `spawn()`

CR-TRACE-002 §1 chỉ xác nhận `ProfileAwareAgentSpawner.spawn()` tồn tại nhưng không truy ngược caller. Grep trực tiếp (`grep -rn "ProfileAwareAgentSpawner"`) cho thấy **có đúng 2 RPC entry point thật** gọi vào `spawn()` — đây là thông tin mới, quan trọng cho việc đặt span đúng chỗ:

| Caller | File:line | RPC method | Ghi chú |
|---|---|---|---|
| `project-rpc-handler.ts` | `project-rpc-handler.ts:209-220` (`defineMethod({ name: 'project.agentSpawn', ... })`) | `project.agentSpawn` | Gọi thẳng `agentSpawner.spawn({ ...params, userId })` |
| `TaskAgentExecutor.executeTask()` | `src/main/task/TaskAgentExecutor.ts:45-92`, gọi `spawn()` ở dòng ~75 | `task.execute` (`src/main/task/task-rpc-handler.ts:373-388`) | `executeTask()` bọc thêm permission check + `TaskService` status transitions (`in_progress`→`review`/`blocked`) — thuộc domain Task Graph (CR-TRACE-018), không phải Agent Orchestration |

Việc có 2 caller khẳng định đúng lựa chọn thiết kế của CR-TRACE-002: đặt span **bên trong `ProfileAwareAgentSpawner.spawn()`** (điểm hội tụ duy nhất) thay vì lặp lại instrumentation ở từng RPC handler — nếu đặt ở `project-rpc-handler.ts` thì luồng qua `task.execute` sẽ không được trace.

### 1.3 Verify các trích dẫn CR-TRACE-002

| Trích dẫn CR | Verify | Ghi chú |
|---|---|---|
| `ProfileAwareAgentSpawner.spawn()` dòng 67-129 | ✅ khớp | |
| `resolve-context` ở dòng 71 (`getProjectContext`) | ✅ | |
| `resolve-provider` ở dòng 96 (`resolveForProject`) | ✅ | |
| `relay-agent-exec` dòng 115-121 (CR ghi 115-121, thực tế `relay.call` bắt đầu dòng 115, kết thúc dòng 121) | ✅ | |
| Comment "FIX TASK-TG-001... agent-rpc-dispatch.ts:506-508" | ✅ đúng — `agent.exec` case tại `agent-rpc-dispatch.ts:502` | Xác nhận field mapping `binary/args/cwd` đã fix, không phải `command/workdir` |
| `agent-rpc-dispatch.ts` case `agent.kill`/`agent.sendInput` dòng 475/488 | ✅ | Chỉ có handler phía Dev Server, không có caller phía Orca Server — khớp CR |
| `agentWsFlow` (`agentWs:lifecycle`) đã tồn tại | ✅ (`tracers.ts`) | |

### 1.4 Gap table

| Sub-flow | RPC method / File | Hiện trạng tracing | Hành động backend-side |
|---|---|---|---|
| BL-AG-01 Spawn | `ProfileAwareAgentSpawner.spawn()` (`ProfileAwareAgentSpawner.ts:67`), gọi từ `project.agentSpawn` VÀ `task.execute` (mục 1.2) | Không có instrumentation | Wrap `spawn()` bằng `Tracers.agentOrchSpawn`; propagate `traceId` optional qua cả 2 RPC schema |
| BL-AG-02 Stop | Không có call site thật (`agent.sendInput`/`agent.kill` chỉ có handler phía Dev Server) | n/a | Chỉ đăng ký tên tracer `agentOrch:stop`, không viết call site giả định |
| BL-AG-03 Resume | Không có call site thật | n/a | Chỉ đăng ký tên tracer `agentOrch:resume` |
| BL-AG-04 Switch | Không có call site thật (không có rate-limit matcher backend) | n/a | Chỉ đăng ký tên tracer `agentOrch:switch` |
| BL-AG-05 Status Poll | Stream liên tục qua `agent.output` — không nên có span per-frame | n/a | Chỉ đăng ký tên tracer `agentOrch:statusPoll`, khuyến nghị KHÔNG dùng cho stream, chỉ dùng nếu có polling loop rời rạc sau này |

### 1.5 Ngoài phạm vi (out of scope)

- Mọi thứ **sau** `relay.call('agent.exec', ...)` trên Dev Server — `agent-rpc-dispatch.ts` case `agent.exec`, `src/relay/agent-spawner.ts`, `src/relay/agent-session.ts` (node-pty spawn thật) — thuộc companion agent-domain solution.
- `TaskAgentExecutor.executeTask()` — permission check + `TaskService` status transitions thuộc domain Task Graph (CR-TRACE-018 `taskGraph:execute`), chỉ propagate field `traceId` xuyên qua, không tạo span riêng ở layer này trong SOL-BE-TRACE-002 (tránh double-instrument cùng 1 request).
- TDD-08's `orchestration.*` / `Coordinator` / `OrchestrationDb` — domain khác, xem 1.1.

---

## 2. Full Implementation

### 2.1 `src/shared/trace/tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries (worktree:* từ SOL-BE-TRACE-001, devServer:*, agentWs:lifecycle, ...) unchanged...

  // ─── CR-TRACE-002: Agent Orchestration (BL-AG-01→05) ───────────────────────
  /** ProfileAwareAgentSpawner.spawn() — BL-AG-01 */
  agentOrchSpawn:      createTracer('agentOrch:spawn'),
  /** BL-AG-02 — reserved, chưa có call site thật ở Orca Server */
  agentOrchStop:       createTracer('agentOrch:stop'),
  /** BL-AG-03 — reserved */
  agentOrchResume:     createTracer('agentOrch:resume'),
  /** BL-AG-04 — reserved */
  agentOrchSwitch:     createTracer('agentOrch:switch'),
  /** BL-AG-05 — reserved, KHÔNG dùng cho stream agent.output per-frame */
  agentOrchStatusPoll: createTracer('agentOrch:statusPoll'),
} as const
```

> Tên `agentOrch:*` (không phải `agent:*`) — tránh va chạm với `agent:rpc` (`agent-rpc-dispatch.ts:21`), tracer hạ tầng wrap generic mọi JSON-RPC method trên Dev Server (CR-TRACE-000 §4).

### 2.2 `ProfileAwareAgentSpawner.ts` — thêm `traceId` vào options + instrument `spawn()`

```typescript
import { Tracers } from '../../shared/trace/tracers'

/** Options for spawning an agent in a project */
export interface AgentSpawnOptions {
  projectId: string
  userId: string
  command: string
  extraEnv?: Record<string, string>
  workdir?: string
  /** [NEW CR-TRACE-002] wire-propagated span id — xem CR-TRACE-000 §3.2 */
  traceId?: string
}

export class ProfileAwareAgentSpawner {
  constructor(
    private readonly router: ProjectServerRouter,
    private readonly profileResolver: ProfileResolver,
    private readonly providerService: AIProviderResolver
  ) {}

  async spawn(options: AgentSpawnOptions): Promise<AgentSpawnResult> {
    const { projectId, userId, command, extraEnv, workdir } = options
    const span = Tracers.agentOrchSpawn.start(
      { projectId, userId },
      options.traceId ? { id: options.traceId } : undefined
    )
    try {
      // 1. Get project context (includes access check + merged profile)
      span.step('resolve-context', { projectId })
      const ctx = await this.router.getProjectContext(projectId, userId, this.profileResolver)
      const { project, resolvedProfile } = ctx

      // 2. Compose env: profile envVars + shell.envVars + extraEnv (last wins)
      const profileEnv: Record<string, string> = {
        ...(resolvedProfile.envVars ?? {}),
        ...(resolvedProfile.shell?.envVars ?? {}),
        ...(extraEnv ?? {}),
      }

      // 3. Prepend pathAdditions to PATH
      const pathAdditions = resolvedProfile.shell?.pathAdditions ?? []
      if (pathAdditions.length > 0) {
        const currentPath = process.env['PATH'] ?? ''
        profileEnv['PATH'] = [...pathAdditions, currentPath].join(':')
      }

      // 4. Add ORCA_* context vars
      profileEnv['ORCA_PROJECT_ID'] = project.id
      profileEnv['ORCA_USER_ID'] = userId
      profileEnv['ORCA_REPO_PATH'] = project.repoPath
      profileEnv['ORCA_DEV_SERVER_ID'] = project.devServerId

      // 5. Resolve AI provider
      const preferredModel = resolvedProfile.agent?.preferredModel
      const provider = await this.providerService.resolveForProject(projectId, preferredModel)
      span.step('resolve-provider', { providerId: provider?.providerId ?? 'none' })
      if (provider) {
        profileEnv['ORCA_AI_PROVIDER_ID'] = provider.providerId
        profileEnv['ORCA_AI_MODEL_ID']    = provider.modelId
        // FIX TASK-WT-002 (SECURITY): không đổi — giữ nguyên, KHÔNG log profileEnv
        // vào bất kỳ trường span nào (chứa PATH/ORCA_* nhưng không chứa credential thật).
        profileEnv['ORCA_ACCOUNT_ID']     = provider.providerId
      }

      // 6. Get relay and send agent.exec
      const relay = await this.router.getRelayForProject(projectId, userId)
      const commandParts = command.trim().split(/\s+/).filter(Boolean)
      const binary = commandParts[0] ?? ''
      const args   = commandParts.slice(1)

      span.step('relay-agent-exec', { binary, devServerId: project.devServerId })
      const result = await relay.call('agent.exec', {
        binary,
        args,
        cwd: workdir ?? project.repoPath,
        env: profileEnv,
        timeoutMs: 5 * 60 * 1000,
        // [NEW CR-TRACE-002] — 2 field song song, xem ghi chú "xung đột" ở SOL-BE-TRACE-001 §2.5:
        // 1) `traceId` phẳng: đọc bởi relayCallTracer hạ tầng (relay:agentCall) trong
        //    DevServerRelayBridge.callWithTimeout() — resume theo CR-TRACE-000 §3.3 row "relay.call()".
        // 2) `_trace.id` lồng: đọc bởi agent-rpc-dispatch.ts phía Dev Server (Agent WS JSON-RPC 2.0
        //    convention, CR-TRACE-000 §3.3 row "Agent WS JSON-RPC 2.0") — không đụng field `id`
        //    chuẩn của JSON-RPC 2.0 dùng match request/response.
        traceId: span.id,
        _trace: { id: span.id },
      })

      const sessionId = (result as { sessionId?: string }).sessionId ?? randomId()
      span.ok({ sessionId })

      return {
        sessionId,
        provider: provider ? { providerId: provider.providerId, modelId: provider.modelId } : undefined,
      }
    } catch (err) {
      span.fail(err, { projectId })
      throw err
    }
  }
}
```

> **Không log `env`/`profileEnv` trong bất kỳ field span nào** — chỉ `binary`, `devServerId`, `providerId` xuất hiện, khớp acceptance criteria CR-TRACE-002 và nguyên tắc bảo mật đã áp dụng cho `ORCA_ACCOUNT_ID` (dòng 100-104 gốc).

> **✅ `agentOrch:spawn` là span CANONICAL duy nhất cho `spawn()` (Known Conflicts resolved 2026-08-02 — xem `tasks/00-index.md`).** `spawn()` không đổi tên tracer, không tách nhánh theo caller — mọi caller (RPC trực tiếp `project.agentSpawn`, hay Task Graph `task.execute` → `TaskAgentExecutor`) đi qua cùng 1 span `agentOrch:spawn`. Khi caller đã tự mở 1 span "routing/prep" của riêng nó TRƯỚC KHI gọi `spawn()` (ví dụ `profile:agentSpawnRoute` ở `project-rpc-handler.ts`, SOL-BE-TRACE-015 §2.7; hoặc `taskGraph:execute` ở `TaskAgentExecutor.executeTask()`, SOL-BE-TRACE-018 §2.5), caller đó forward `span.id` của chính nó qua `options.traceId` — `agentOrch:spawn` sẽ **RESUME** (`resume: { id: options.traceId }`, đã có sẵn trong code trên) thay vì tạo span độc lập mới. Đây đúng là cơ chế `resume` mô tả ở **CR-TRACE-000 §3.1**: nhiều layer nội bộ nối tiếp nhau chia sẻ 1 `id` xuyên suốt thay vì hiển thị như nhiều trace rời rạc trên TracePanel. `agentOrch:spawn` KHÔNG BAO GIỜ là span thứ 2 chạy song song với 1 span khác cùng bọc `spawn()` — nó luôn là span duy nhất bọc thân hàm này.

### 2.3 Propagate `traceId` từ 2 RPC caller thật

**`src/main/project/project-rpc-handler.ts`** — `project.agentSpawn`:

```typescript
// AgentSpawnParam (schema hiện có) — thêm field:
const AgentSpawnParam = z.object({
  projectId: z.string().min(1),
  command: z.string().min(1),
  extraEnv: z.record(z.string(), z.string()).optional(),
  workdir: z.string().optional(),
  traceId: OptionalString, // [NEW CR-TRACE-002]
})

// Handler không đổi — options (bao gồm traceId) đã forward nguyên vẹn qua spread:
defineMethod({
  name: 'project.agentSpawn',
  params: AgentSpawnParam,
  handler: async (params, ctx) => {
    const userId = ctx.userId
    if (!userId) throw new Error('UNAUTHENTICATED')
    if (!agentSpawner) throw new Error('AGENT_SPAWNER_NOT_AVAILABLE')
    await projectService.assertAccess(params.projectId, userId)
    return agentSpawner.spawn({ ...params, userId }) // traceId đã nằm trong params, spread qua nguyên vẹn
  }
}),
```

**`src/main/task/TaskAgentExecutor.ts`** — chỉ propagate field, KHÔNG tạo span riêng (xem 1.5):

```typescript
export interface ExecuteTaskParams {
  taskId: string
  projectId: string
  userId: string
  worktreePath: string
  accountId?: string
  /** [NEW CR-TRACE-002] forwarded to ProfileAwareAgentSpawner.spawn(), không dùng ở layer này */
  traceId?: string
}

// Trong executeTask(), tại bước 5 (Spawn agent) — chỉ thêm 1 field:
await this.agentSpawner.spawn({
  projectId,
  userId,
  command: prompt,
  workdir: worktreePath,
  extraEnv: params.accountId ? { ORCA_ACCOUNT_ID: params.accountId } : undefined,
  traceId: params.traceId, // [NEW] — span agentOrch:spawn resume nếu caller (task.execute) gửi kèm
})
```

**`src/main/task/task-rpc-handler.ts`** — `task.execute`, thêm `traceId` vào `ExecuteParam` schema và forward:

```typescript
const ExecuteParam = z.object({
  taskId: z.string().min(1),
  projectId: z.string().min(1),
  worktreePath: z.string().min(1),
  accountId: OptionalString,
  traceId: OptionalString, // [NEW CR-TRACE-002]
})

defineMethod({
  name: 'task.execute',
  params: ExecuteParam,
  handler: async (params, ctx) => {
    const userId = ctx.userId ?? ''
    await executor.executeTask({
      taskId: params.taskId,
      projectId: params.projectId,
      userId,
      worktreePath: params.worktreePath,
      accountId: params.accountId,
      traceId: params.traceId, // [NEW]
    })
    return { started: true }
  },
}),
```

### 2.4 Relay bridge (`dev-server-relay-bridge.ts`)

Đã sửa bởi **SOL-BE-TRACE-001 §2.5** (cùng file `callWithTimeout()`, đọc `params.traceId` phẳng). SOL-BE-TRACE-002 **không sửa lại** file này — chỉ đảm bảo `agent.exec` call gửi cả field `traceId` phẳng (mục 2.2) để tương thích với patch đã có ở SOL-BE-TRACE-001. Nếu 2 solution được apply theo thứ tự bất kỳ, không có xung đột merge (cùng thay đổi 1 điểm, idempotent).

---

## 3. Test Plan (Vitest)

| Test file | Test case | Mục tiêu |
|---|---|---|
| `src/shared/trace/__tests__/tracers.test.ts` | `'exports Tracers.agentOrchSpawn/Stop/Resume/Switch/StatusPoll with correct flow names'` | Verify convention CR-TRACE-000 §4, không trùng `agent:rpc` |
| `src/main/project/__tests__/ProfileAwareAgentSpawner.test.ts` | `'spawn() emits agentOrch:spawn span wrapping resolve-context → resolve-provider → relay-agent-exec steps in order'` | Mock `Tracers.agentOrchSpawn`, assert thứ tự `step()` calls |
| (cùng file) | `'spawn() ok() contains sessionId only, never env/profileEnv/credentials'` | Guard bảo mật — assert field span không chứa key nhạy cảm |
| (cùng file) | `'spawn() resumes span id from options.traceId when provided'` | Assert `start(fields, { id })` |
| (cùng file) | `'spawn() sends both traceId (flat) and _trace.id (nested) to relay.call agent.exec'` | Assert cả 2 field cùng có mặt trong params gửi xuống relay |
| (cùng file) | `'spawn() fail() propagates original error and projectId field on getProjectContext rejection'` | |
| (cùng file) | `'spawn() fail() propagates on relay.call agent.exec rejection'` | |
| `src/main/project/__tests__/project-rpc-handler.test.ts` | `'project.agentSpawn forwards traceId from params into agentSpawner.spawn()'` | Spy vào `agentSpawner.spawn`, assert `traceId` có trong argument |
| `src/main/task/__tests__/TaskAgentExecutor.test.ts` | `'executeTask forwards traceId to agentSpawner.spawn() without creating its own span'` | Assert không có `createTracer`/`Tracers.*` nào được gọi trong `TaskAgentExecutor` |
| `src/main/task/__tests__/task-rpc-handler.test.ts` | `'task.execute accepts optional traceId param and forwards to executor.executeTask'` | |
| `src/main/dev-server/__tests__/dev-server-relay-bridge.test.ts` (mở rộng, chung với SOL-BE-TRACE-001) | `'callWithTimeout resumes relay:agentCall span for agent.exec calls carrying flat traceId'` | Regression test đảm bảo field `_trace.id` (chỉ dùng phía Dev Server) không phá vỡ resume ở tầng bridge |

**Test Targets:**

| Nhóm | Target |
|---|---|
| `tracers.test.ts` (mở rộng) | ≥ 2 |
| `ProfileAwareAgentSpawner.test.ts` (mới) | ≥ 6 |
| `project-rpc-handler.test.ts` (mở rộng) | ≥ 2 |
| `TaskAgentExecutor.test.ts` (mở rộng) | ≥ 2 |
| `task-rpc-handler.test.ts` (mở rộng) | ≥ 2 |
| `dev-server-relay-bridge.test.ts` (mở rộng, cross-check với SOL-001) | ≥ 2 |
| **Total** | **≥ 16** |

---

## 4. Acceptance Criteria

- [ ] `Tracers.agentOrchSpawn/Stop/Resume/Switch/StatusPoll` tồn tại trong `tracers.ts` với đúng flow name `agentOrch:spawn|stop|resume|switch|statusPoll`, không trùng `agent:rpc`
- [ ] `ProfileAwareAgentSpawner.spawn()` phát `agentOrch:spawn` span bao trọn `resolve-context` → `resolve-provider` → `relay-agent-exec`, `ok()` chứa `sessionId`
- [ ] Trường `env`/`profileEnv`/credentials KHÔNG bao giờ xuất hiện trong field của span `agentOrch:spawn` — chỉ `binary`, `devServerId`, `providerId`, `projectId`, `userId`, `sessionId`
- [ ] Cả 2 caller thật (`project.agentSpawn` và `task.execute` → `TaskAgentExecutor.executeTask`) đều propagate `traceId` xuyên tới `spawn()` mà không tạo span trùng lặp ở layer trung gian
- [ ] `relay.call('agent.exec', ...)` mang cả field phẳng `traceId` (cho `relayCallTracer` hạ tầng) và field lồng `_trace.id` (cho Agent WS JSON-RPC 2.0 convention) — không phá vỡ patch của SOL-BE-TRACE-001 §2.5
- [ ] KHÔNG có tracer/span mới nào được tạo cho `orchestration.send`/`.check`/`Coordinator` (TDD-08) — xác nhận đây là domain khác, ngoài phạm vi CR-TRACE-002
- [ ] KHÔNG có span mới được tạo cho mỗi `agent.output` PTY stream frame (BL-AG-05)
- [ ] BL-AG-02/03/04 chỉ có tracer name đăng ký sẵn trong `tracers.ts`, không có call site giả định nào được viết cho method chưa tồn tại (`agent.sendInput`/`agent.kill`/switch-account từ Orca Server)
