# TASK-BE-002.2: Instrument `ProfileAwareAgentSpawner.spawn()` — điểm hội tụ duy nhất

**Phase:** 1
**SOL Ref:** [SOL-BE-TRACE-002](../solutions/SOL-BE-TRACE-002-agent-orchestration.md)
**CR Ref:** [CR-TRACE-002](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-002-agent-orchestration.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-002.1
**Status:** ✅ Done (2026-08-04) — Implemented exactly per spec: added `traceId?: string` to `AgentSpawnOptions`, wrapped `spawn()` in the canonical `agentOrch:spawn` span with `resolve-context`/`resolve-provider`/`relay-agent-exec` steps, `agent.exec` call carries both flat `traceId` and nested `_trace: { id }`. Real code matched the task doc's assumed shape exactly (no drift). typecheck clean. 3 pre-existing test failures in `ProfileAwareAgentSpawner.test.ts` (asserting a `workdir` field that the code intentionally renamed to `cwd` per a prior fix, `FIX TASK-TG-001`) confirmed present identically with my change stashed out — unrelated to this task, left as-is. gitnexus detect_changes (staged) confirms LOW risk, only this file's expected symbols touched.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "ProfileAwareAgentSpawner.spawn"
```

Symbol đã tồn tại (MODIFY case) — đây là điểm hội tụ duy nhất cho việc spawn agent, được gọi từ 2 caller RPC thật (`project.agentSpawn` và `task.execute` → `TaskAgentExecutor.executeTask()`), và tiếp tục là điểm resume của `profile:agentSpawnRoute` (TASK-BE-015.4) lẫn `taskGraph:execute` (TASK-BE-018.5). Chạy:

```
gitnexus_impact({ target: "ProfileAwareAgentSpawner.spawn", direction: "upstream" })
```

Báo cáo đầy đủ blast radius (mọi caller, mọi process bị ảnh hưởng, risk level) cho người dùng trước khi sửa. Đặc biệt xác nhận không log `env`/`profileEnv`/credentials vào bất kỳ field span nào. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

`ProfileAwareAgentSpawner.spawn()` là điểm hội tụ duy nhất cho việc spawn agent — có đúng 2 caller RPC thật (`project.agentSpawn` và `task.execute` → `TaskAgentExecutor.executeTask()`, xem TASK-BE-002.3), nên chỉ cần instrument tại đây thay vì lặp lại ở từng RPC handler. Task này thêm field `traceId?` vào `AgentSpawnOptions` và bọc `spawn()` bằng span `agentOrch:spawn` bao trọn 3 bước `resolve-context` → `resolve-provider` → `relay-agent-exec`.

**✅ Known Conflicts đã resolve 2026-08-02 (xem `tasks/00-index.md`):** `agentOrch:spawn` là span CANONICAL duy nhất bọc `spawn()` — không có task nào khác (`TASK-BE-015.4`, `TASK-BE-018.4`) được phép mở thêm 1 span gốc khác bọc lại cùng thân hàm này. Khi caller đã tự mở span riêng của nó TRƯỚC KHI gọi `spawn()` (`profile:agentSpawnRoute` ở `project-rpc-handler.ts`, xem `TASK-BE-015.4`; hoặc `taskGraph:execute` ở `TaskAgentExecutor`, xem `TASK-BE-018.5`), caller đó forward `span.id` của nó qua `options.traceId` — code dưới đây (`options.traceId ? { id: options.traceId } : undefined`) đã sẵn sàng resume đúng cơ chế đó, không cần sửa gì thêm khi các task phụ thuộc chạy sau.

## File: `src/main/project/ProfileAwareAgentSpawner.ts` [MODIFY]

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

**Ràng buộc bảo mật bắt buộc:** KHÔNG log `env`/`profileEnv`/credentials trong bất kỳ field span nào — chỉ `binary`, `devServerId`, `providerId`, `projectId`, `userId`, `sessionId` được phép xuất hiện trong `span.step()`/`span.ok()`/`span.fail()`.

**Field `traceId` gửi cả 2 dạng — bắt buộc theo thiết kế đã chốt để giải quyết xung đột convention giữa CR-TRACE-001 (relay.call dùng field phẳng `traceId`) và CR-TRACE-002/013 (Agent WS JSON-RPC dùng `params._trace.id` lồng nhau):** `relay.call('agent.exec', ...)` PHẢI mang cả `traceId: span.id` (phẳng) VÀ `_trace: { id: span.id }` (lồng) — không được chỉ gửi một trong hai, nếu không `relayCallTracer` hạ tầng (đã sửa ở TASK-BE-001.3) sẽ không resume được cho riêng call `agent.exec`.

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/main/project/__tests__/ProfileAwareAgentSpawner.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `AgentSpawnOptions` có field `traceId?: string`
- [ ] `spawn()` phát `agentOrch:spawn` span bao trọn `resolve-context` → `resolve-provider` → `relay-agent-exec`, `ok()` chứa `sessionId`
- [ ] `spawn()` resume span id từ `options.traceId` khi có
- [ ] Trường `env`/`profileEnv`/credentials KHÔNG bao giờ xuất hiện trong field của span `agentOrch:spawn`
- [ ] `relay.call('agent.exec', ...)` mang cả field phẳng `traceId` và field lồng `_trace.id`, không phá vỡ patch của TASK-BE-001.3
- [ ] `span.fail()` propagate đúng lỗi gốc và field `projectId` trên cả nhánh `getProjectContext` reject lẫn `relay.call agent.exec` reject
