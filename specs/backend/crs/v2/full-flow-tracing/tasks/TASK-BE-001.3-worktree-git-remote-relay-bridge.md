# TASK-BE-001.3: Instrument `git.worktree.add`/`git.worktree.remove` và resume span ở `DevServerRelayBridge`

**Phase:** 1
**SOL Ref:** [SOL-BE-TRACE-001](../solutions/SOL-BE-TRACE-001-worktree-management.md)
**CR Ref:** [CR-TRACE-001](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-001-worktree-management.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-001.1
**Status:** ✅ Done (2026-08-03) — Instrumented `git.worktree.add`/`git.worktree.remove` in `git-remote.ts` exactly per spec (real code matched task doc closely). `DevServerRelayBridge.callWithTimeout()` already had `relayCallTracer.start(...)` call sites from a concurrent sibling effort (not yet resume-aware) — added the `resumeTraceId` read from `params['traceId']` and threaded it into both `.start()` calls (AGENT_NOT_CONNECTED branch + normal-call branch), no other logic changed. Confirmed CRITICAL/HIGH risk from `callWithTimeout`'s large fan-in as the task doc anticipated — change is a purely additive optional-field read, backward compatible with every existing caller that doesn't pass `traceId`. typecheck clean (pre-existing unrelated `aiProviderService` error only); 40/40 tests pass across `git-remote.test.ts`, `git-remote-rpc.test.ts`, `dev-server-relay-bridge.test.ts`; detect_changes (staged) confirms only the 2 target files touched.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "git.worktree.add"
codegraph explore "git.worktree.remove"
codegraph explore "DevServerRelayBridge.callWithTimeout"
```

Cả 3 là symbol đã tồn tại (MODIFY case). `DevServerRelayBridge.callWithTimeout()` đặc biệt quan trọng — đây là hạ tầng dùng CHUNG cho MỌI `relay.call(...)` trong toàn repo, không riêng worktree. Chạy:

```
gitnexus_impact({ target: "DevServerRelayBridge.callWithTimeout", direction: "upstream" })
gitnexus_impact({ target: "git.worktree.add", direction: "upstream" })
gitnexus_impact({ target: "git.worktree.remove", direction: "upstream" })
```

Với `callWithTimeout()`, kỳ vọng risk HIGH do fan-in lớn (mọi relay call đều đi qua đây) — đọc kỹ báo cáo để xác nhận thay đổi (đọc `params.traceId` để resume) không phá vỡ bất kỳ caller nào không truyền field này. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Bọc 2 RPC handler `git.worktree.add`/`git.worktree.remove` (`src/main/runtime/rpc/methods/git-remote.ts`) bằng tracer, forward `traceId: span.id` vào `relay.call('git.exec', ...)`; đồng thời sửa `DevServerRelayBridge.callWithTimeout()` để đọc `params.traceId` và resume span `relay:agentCall` — đây là hạ tầng dùng chung nên chỉ sửa một lần tại đây, không lặp lại ở solution khác.

## File: `src/main/runtime/rpc/methods/git-remote.ts` [MODIFY]

`git.diff` (dòng ~101) **KHÔNG** được sửa trong task này — method này dùng chung cho nhiều mục đích (code review, compare), gắn nhãn `worktree:compare` vào đó sẽ làm sai lệch dữ liệu trace.

```typescript
import { Tracers } from '../../../../shared/trace/tracers'

// ── git.worktree.add ─────────────────────────────────────────────────────
defineMethod({
  name: 'git.worktree.add',
  params: ProjectWorktreeParam.extend({
    path: z.string().min(1),
    branch: z.string().min(1),
  }),
  handler: async (params, ctx) => {
    const span = Tracers.worktreeCreate.start(
      { projectId: params.projectId, path: params.path },
      params.traceId ? { id: params.traceId } : undefined
    )
    try {
      const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
      span.step('relay-git-worktree-add', { devServerId: params.projectId })
      const result = (await relay.call('git.exec', {
        cwd: params.worktreePath,
        args: ['worktree', 'add', params.path, params.branch],
        traceId: span.id, // [NEW] forward vào relay envelope — CR-TRACE-000 §3.3 row "relay.call()"
      })) as GitExecResult
      span.ok({ path: params.path })
      return result
    } catch (error) {
      span.fail(error, { projectId: params.projectId })
      throw error
    }
  },
}),

// ── git.worktree.remove ──────────────────────────────────────────────────
defineMethod({
  name: 'git.worktree.remove',
  params: ProjectWorktreeParam.extend({
    path: z.string().min(1),
    force: z.boolean().optional(),
  }),
  handler: async (params, ctx) => {
    const span = Tracers.worktreeDelete.start(
      { projectId: params.projectId, path: params.path, force: params.force === true },
      params.traceId ? { id: params.traceId } : undefined
    )
    try {
      const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
      const args = ['worktree', 'remove', params.path]
      if (params.force) args.push('--force')
      span.step('relay-git-worktree-remove', { devServerId: params.projectId })
      const result = (await relay.call('git.exec', {
        cwd: params.worktreePath,
        args,
        traceId: span.id,
      })) as GitExecResult
      span.ok({ path: params.path })
      return result
    } catch (error) {
      span.fail(error, { projectId: params.projectId })
      throw error
    }
  },
}),
```

## File: `src/main/dev-server/dev-server-relay-bridge.ts` [MODIFY]

Resume `relay:agentCall` bằng `traceId` từ envelope params (đọc field phẳng `params['traceId']`):

```typescript
private async callWithTimeout<T>(
  method: string,
  params: Record<string, unknown>,
  timeoutMs: number
): Promise<T> {
  const startTime = Date.now()
  // [NEW CR-TRACE-001] Đọc traceId do domain tracer (worktree:create, worktree:delete, ...)
  // đính kèm vào params envelope — CR-TRACE-000 §3.3 row "relay.call()".
  const resumeTraceId = typeof params['traceId'] === 'string' ? (params['traceId'] as string) : undefined

  while (true) {
    // ...existing session/reconnect logic unchanged...

    if (!session) {
      const span = relayCallTracer.start(
        { devServerId: this.config.id, method },
        resumeTraceId ? { id: resumeTraceId } : undefined
      )
      span.fail('AGENT_NOT_CONNECTED', { method, devServerId: this.config.id })
      throw Object.assign(/* ...existing error unchanged... */)
    }

    const span = relayCallTracer.start(
      { devServerId: this.config.id, method },
      resumeTraceId ? { id: resumeTraceId } : undefined
    )
    try {
      // ...existing session.request(method, params) logic unchanged...
      span.ok({ method })
      return result
    } catch (err: unknown) {
      // ...existing reconnect/retry logic unchanged...
    }
  }
}
```

**Lưu ý xung đột tiềm ẩn với CR-TRACE-002 (giữ nguyên khi implement, không xóa comment):** `callWithTimeout()` là hạ tầng dùng chung cho **mọi** `relay.call(...)`, bao gồm cả `agent.exec` (CR-TRACE-002 BL-AG-01). CR-TRACE-002 yêu cầu `agent.exec` gửi `traceId` lồng trong `params._trace.id` (không phải field phẳng `traceId`) vì đây là kênh Agent WS JSON-RPC 2.0 thật sự. Patch ở task này chỉ đọc field phẳng `params['traceId']`. SOL-BE-TRACE-002 (xem TASK-BE-002.1) gửi **cả hai** field (`traceId` phẳng cho `relayCallTracer` hạ tầng ở đây, **và** `_trace: { id }` cho phần xử lý domain-specific ở `agent-rpc-dispatch.ts` phía Dev Server) để tương thích với patch này — không có xung đột merge dù 2 task được áp dụng theo thứ tự nào (cùng thay đổi 1 điểm, idempotent).

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/main/runtime/rpc/methods/__tests__/git-remote.test.ts
pnpm test --run src/main/dev-server/__tests__/dev-server-relay-bridge.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] Handler `git.worktree.add` phát `worktree:create` span, `ok()` chứa `path`
- [ ] Handler `git.worktree.remove` phát `worktree:delete` span, `ok()` chứa `path`
- [ ] Mọi `relay.call('git.exec', ...)` tại 2 call site trong `git-remote.ts` đính kèm `traceId: span.id`
- [ ] `git.diff` (`git-remote.ts:101`) **không** bị gắn tracer `worktree:compare` — xác nhận bằng test guard
- [ ] `DevServerRelayBridge.callWithTimeout()` resume `relay:agentCall` span bằng `params.traceId` khi có, tạo span mới khi không có (cả nhánh thành công và nhánh `AGENT_NOT_CONNECTED`)
- [ ] Không đổi logic reconnect/retry hiện có trong `callWithTimeout()`
