# SOL-BE-TRACE-001: Worktree Management — Backend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-001](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-001-worktree-management.md)
**TDD Ref:** TDD-07 (OrcaRuntimeService — Worktree Lifecycle §5), TDD-05 (SSH & Relay — `relay.call()` dispatch §5–6)
**Date:** 2026-08-02
**Status:** Proposed
**Test Targets:** ≥ 20 tests (xem mục 3)
**Strategy:** Additive-only — chỉ bọc (wrap) tracer quanh handler RPC đã tồn tại, không đổi business logic; field `traceId` optional trong params schema (backward-compatible)

---

## 1. Phân tích phạm vi (Backend-side only)

### 1.1 Đối chiếu với TDD-07 / TDD-05

TDD-07 §5 (Worktree Lifecycle) mô tả `createWorktree()`/`deleteWorktree()` như API của `OrcaRuntimeService` — khớp với `createManagedWorktree()` (`src/main/runtime/orca-runtime.ts:14543`) và `removeManagedWorktree()` (`orca-runtime.ts:16888`) mà CR-TRACE-001 đã xác nhận qua grep. TDD-05 §5 (`SshRelaySession`) và §6 (`relay-protocol.ts`) mô tả layer relay bên dưới `DevServerRelayBridge.call()` — đây chính là nơi `relayCallTracer` (`relay:agentCall`) hiện đang chạy không có `resume`.

Đã verify lại (grep + Read trực tiếp source, 2026-08-02) toàn bộ file:line mà CR-TRACE-001 trích dẫn — khớp:

| Trích dẫn CR-TRACE-001 | Verify | Ghi chú |
|---|---|---|
| `worktree.ts:71` handler `worktree.create` | ✅ đúng dòng `name: 'worktree.create'` | Handler thật có thêm `automationProvenance` reservation + `try/catch` release-on-fail mà ví dụ minh hoạ của CR không thể hiện — solution này giữ nguyên logic đó |
| `orca-runtime.ts:14543` `createManagedWorktree()` | ✅ | |
| `worktree.ts:230-241` handler `worktree.rm` | Thực tế ở dòng 230-240 (`name: 'worktree.rm'` dòng 230) | Sai lệch 1 dòng, không đáng kể |
| `orca-runtime.ts:16888` `removeManagedWorktree()` | ✅ | |
| `git-remote.ts:319-332` `git.worktree.add` | Thực tế dòng 317-332 (comment header ở 317) | |
| `git-remote.ts:336-348` `git.worktree.remove` | Thực tế dòng 334-347 | |
| `git-remote.ts:101-116` `git.diff` | ✅ | Method **dùng chung** cho nhiều mục đích (code review, compare), không riêng cho worktree |
| `dev-server-relay-bridge.ts:562-630` `callWithTimeout()` | ✅, 2 call site `relayCallTracer.start()` ở dòng 595 và 607 | |
| `RpcRequest` (`rpc/core.ts:33-38`) | Thực tế dòng 35-40 (`{ id, authToken, method, params }`) | Không có field trace — khớp mô tả CR |

### 1.2 Gap table

| Sub-flow | RPC method / File | Hiện trạng tracing | Hành động backend-side (SOL này) |
|---|---|---|---|
| BL-WT-01 Create | `worktree.create` (`worktree.ts:70`), `git.worktree.add` (`git-remote.ts:320`) | Không có instrumentation nào | Wrap cả 2 handler bằng `Tracers.worktreeCreate`, forward `traceId` vào `relay.call('git.exec', ...)` |
| BL-WT-02 Fan-out | Không tồn tại RPC method (`worktree.fanOut` không có trong code) | n/a | Chỉ đăng ký tên tracer `worktree:fanOut` trong `tracers.ts`, **không** viết call site giả định |
| BL-WT-03 Delete | `worktree.rm` (`worktree.ts:230`), `git.worktree.remove` (`git-remote.ts:337`) | Không có instrumentation nào | Wrap cả 2 handler bằng `Tracers.worktreeDelete`; `worktree.checkSafety` không tồn tại trong code nên không có gì để wrap (đúng như CR-TRACE-001 §4 BL-WT-03 ghi nhận) |
| BL-WT-04 Compare | Không có `worktree.compare`; gần nhất là `git.diff` (dùng chung) | n/a | **Không** gắn tracer `worktreeCompare` vào `git.diff` — method này được gọi từ nhiều luồng khác (code review, git panel thường), gắn nhãn `worktree:compare` vào đó sẽ sai lệch dữ liệu trace. Chỉ đăng ký tên tracer, chờ RPC method chuyên biệt |
| BL-WT-05 Merge | Không tồn tại RPC method | n/a | Chỉ đăng ký tên tracer `worktree:merge` |
| Relay bridge (hạ tầng dùng chung BL-WT-01/03) | `DevServerRelayBridge.callWithTimeout()` (`dev-server-relay-bridge.ts:562`) | `relayCallTracer.start({ devServerId, method })` không nhận `resume` → span `relay:agentCall` luôn tạo `id` random mới, không nối được với `worktree:create`/`worktree:delete` phía trên | Sửa để đọc `params.traceId` và `resume` (yêu cầu `Tracer.start(fields, resume)` — xem 1.3) |

### 1.3 Prerequisite: `Tracer.start(fields?, resume?)`

Đã verify `src/shared/trace/index.ts:46-52` — `Tracer.start()` hiện tại **chỉ nhận `fields`**, không có tham số `resume`. Đây là core API change được đặc tả ở CR-TRACE-000 §3.1, dùng chung cho toàn bộ 19 CR trong rollout — SOL này **giả định** API đó đã được triển khai trước khi merge (không lặp lại code ở đây, theo đúng chỉ dẫn "reference by section number" của CR-TRACE-000). Nếu `Tracer.start()` chưa có `resume` tại thời điểm implement, các đoạn code ở mục 2.4/2.5 (gọi `resume`) sẽ không compile — cần merge CR-TRACE-000 trước.

### 1.4 Ngoài phạm vi (out of scope)

- Bất kỳ điều gì xảy ra **sau** khi `relay.call('git.exec', ...)` rời khỏi `DevServerRelayBridge` — tức phần xử lý trong `agent-rpc-dispatch.ts` (case `git.exec`, `git.worktree.add`) trên Dev Server — thuộc solution phía agent (companion agent-domain solution), không phải backend/gateway.
- BL-WT-02/04/05: vì RPC method thật chưa tồn tại, solution này chỉ **đặt tên tracer trước** (theo đúng khuyến nghị CR-TRACE-001 §4), không viết code giả định cho method chưa tồn tại.

---

## 2. Full Implementation

### 2.1 `src/shared/trace/tracers.ts`

```typescript
// ─── Pre-built Tracers ────────────────────────────────────────────────────────
import { createTracer } from './index'

export const Tracers = {
  /** Browser → RPC → IPC → Relay → Agent: directory browse */
  browseDirFlow: createTracer('devServer:browseDir'),
  /** Browser → RPC → IPC → Relay → Agent: mkdir */
  mkdirFlow:     createTracer('devServer:mkdir'),
  /** Browser → RPC → IPC → Relay → Agent: rmdir */
  rmdirFlow:     createTracer('devServer:rmdir'),
  /** Agent WebSocket lifecycle (connect / disconnect) */
  agentWsFlow:   createTracer('agentWs:lifecycle'),
  /** IPC proxy call from user-process to main-process */
  ipcProxyFlow:  createTracer('ipc:devServerProxy'),

  // ─── CR-TRACE-001: Worktree Management (BL-WT-01→05) ──────────────────────
  /** worktree.create + git.worktree.add — BL-WT-01 */
  worktreeCreate:  createTracer('worktree:create'),
  /** worktree.fanOut — BL-WT-02 — reserved, chưa có RPC method thật */
  worktreeFanOut:  createTracer('worktree:fanOut'),
  /** worktree.rm + git.worktree.remove — BL-WT-03 */
  worktreeDelete:  createTracer('worktree:delete'),
  /** worktree.compare — BL-WT-04 — reserved, chưa có RPC method thật */
  worktreeCompare: createTracer('worktree:compare'),
  /** worktree.merge — BL-WT-05 — reserved, chưa có RPC method thật */
  worktreeMerge:   createTracer('worktree:merge'),
} as const
```

### 2.2 `src/main/runtime/rpc/methods/worktree-schemas.ts` — thêm field `traceId`

```typescript
// import OptionalString đã có sẵn ở đầu file (từ '../schemas')

export const WorktreeCreate = z
  .object({
    repo: z
      .unknown()
      .transform((v) => (typeof v === 'string' ? v : ''))
      .pipe(z.string().min(1, 'Missing repo selector')),
    // ...existing fields unchanged (name, baseBranch, linkedIssue, ...)...
    traceId: OptionalString, // [NEW CR-TRACE-001] wire-propagated span id, xem CR-TRACE-000 §3.2
  })
  // ...existing .superRefine(...) chain unchanged...

export const WorktreeRemove = WorktreeSelector.extend({
  force: OptionalBoolean,
  runHooks: OptionalBoolean,
  traceId: OptionalString, // [NEW CR-TRACE-001]
})
```

> Không đụng `WorktreeSelector` gốc (dùng chung bởi `worktree.activate`, `worktree.forceDeleteBranch`, ...) — chỉ `.extend()` tại `WorktreeRemove` để tránh field `traceId` xuất hiện lan man ở các method không liên quan.

### 2.3 `src/main/runtime/rpc/methods/worktree.ts` — instrument `worktree.create` / `worktree.rm`

```typescript
import { Tracers } from '../../../../shared/trace/tracers'

// ...

defineMethod({
  name: 'worktree.create',
  params: WorktreeCreate,
  handler: async (params, { runtime }) => {
    const span = Tracers.worktreeCreate.start(
      { repoSelector: params.repo, baseBranch: params.baseBranch ?? '' },
      params.traceId ? { id: params.traceId } : undefined
    )
    const repo = await runtime.showRepo(params.repo)
    const automationProvenance = resolveAutomationWorkspaceProvenance({
      authority: runtime,
      repoSelector: params.repo,
      repo,
      request: params.automationProvenanceRequest
    })
    // Why: provenance tokens are reserved before creation so retries can recover,
    // but failed create attempts must release the reservation for a safe retry.
    try {
      span.step('resolve-repo', { repoId: repo.id })
      const result = await runtime.createManagedWorktree({
        // ...existing args unchanged (name, baseBranch, compareBaseRef, linkedIssue, ...)...
        automationProvenance,
        // ...existing lineage/startup blocks unchanged...
      })
      finishAutomationWorkspaceProvenanceRequest(params.automationProvenanceRequest)
      span.ok({ worktreeId: result.worktreeId ?? result.id, path: result.path })
      // Why: agent callers need a stable dispatch target without traversing
      // terminal-list layout duplicates after creating the worktree.
      return params.startupAgent && result.startupTerminal?.handle
        ? { ...result, agentTerminalHandle: result.startupTerminal.handle }
        : result
    } catch (error) {
      releaseAutomationWorkspaceProvenanceRequest(params.automationProvenanceRequest)
      span.fail(error, { repoSelector: params.repo })
      throw error
    }
  }
}),

// ...

defineMethod({
  name: 'worktree.rm',
  params: WorktreeRemove,
  handler: async (params, { runtime }) => {
    const span = Tracers.worktreeDelete.start(
      { worktreeId: params.worktree, force: params.force === true },
      params.traceId ? { id: params.traceId } : undefined
    )
    try {
      const result = await runtime.removeManagedWorktree(
        params.worktree,
        params.force === true,
        params.runHooks === true
      )
      span.ok({ worktreeId: params.worktree })
      return { removed: true, ...result }
    } catch (error) {
      span.fail(error, { worktreeId: params.worktree })
      throw error
    }
  }
})
```

> **Không dùng `resume` giữa `worktree.checkSafety` và `worktree.rm`** — theo CR-TRACE-001 §4 BL-WT-03, method `worktree.checkSafety` không tồn tại trong code hiện tại, nên vấn đề "2 round-trip cách nhau bởi user-confirmation" hiện chưa áp dụng được; ghi chú này giữ lại cho lần implement `worktree.checkSafety` sau này (không được `resume` từ `worktree.rm`).

### 2.4 `src/main/runtime/rpc/methods/git-remote.ts` — instrument `git.worktree.add` / `git.worktree.remove`

```typescript
import { Tracers } from '../../../../shared/trace/tracers'

// ── Common schemas ─────────────────────────────────────────────────────────────
const ProjectWorktreeParam = z.object({
  projectId: z.string().min(1),
  worktreePath: z.string().min(1),
  traceId: OptionalString, // [NEW CR-TRACE-001], dùng bởi git.worktree.add/remove bên dưới
})

// ...

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

> `git.diff` (dòng 101) **KHÔNG** được sửa trong solution này — xem gap table 1.2 (dùng chung, không riêng cho worktree compare).

### 2.5 `src/main/dev-server/dev-server-relay-bridge.ts` — resume `relay:agentCall` bằng `traceId` từ envelope

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

> **Lưu ý xung đột tiềm ẩn với CR-TRACE-002:** `callWithTimeout()` là hạ tầng dùng chung cho **mọi** `relay.call(...)`, bao gồm cả `agent.exec` (CR-TRACE-002 BL-AG-01). CR-TRACE-002 §5 mục 4 yêu cầu `agent.exec` gửi `traceId` lồng trong `params._trace.id` (không phải field phẳng `traceId`) vì đây là kênh Agent WS JSON-RPC 2.0 thật sự. Patch ở mục này chỉ đọc field phẳng `params['traceId']` — nghĩa là với riêng call `agent.exec`, `relayCallTracer` (`relay:agentCall`) sẽ **không** resume được nếu SOL-BE-TRACE-002 chỉ gửi `_trace.id`. Khuyến nghị (để cả 2 nhất quán): SOL-BE-TRACE-002 nên gửi **cả hai** field (`traceId: span.id` phẳng cho `relayCallTracer` hạ tầng, **và** `_trace: { id: span.id }` cho phần xử lý domain-specific ở `agent-rpc-dispatch.ts` phía Dev Server) — xem ghi chú tương ứng trong SOL-BE-TRACE-002 §2.3. Đây là điểm cần chốt lại khi CR-TRACE-000 được review cuối cùng, không phải lỗi implement.

---

## 3. Test Plan (Vitest)

| Test file | Test case | Mục tiêu |
|---|---|---|
| `src/shared/trace/__tests__/tracers.test.ts` | `'exports Tracers.worktreeCreate with flow name worktree:create'` | Verify tên tracer đúng convention CR-TRACE-000 §4 |
| (cùng file) | `'exports worktreeFanOut/worktreeCompare/worktreeMerge as reserved tracers'` | Không throw khi `.start()` được gọi dù chưa có call site thật |
| `src/main/runtime/rpc/methods/__tests__/worktree.test.ts` | `'worktree.create emits worktreeCreate span with ok() on success'` | Mock `Tracers.worktreeCreate`, assert `start`/`ok` được gọi với field `worktreeId`/`path` |
| (cùng file) | `'worktree.create resumes span id from params.traceId when provided'` | Assert `start(fields, { id: 'abc123' })` khi `params.traceId = 'abc123'` |
| (cùng file) | `'worktree.create span.fail() called on createManagedWorktree rejection, provenance released'` | Assert `fail()` + `releaseAutomationWorkspaceProvenanceRequest` đều chạy |
| (cùng file) | `'worktree.rm emits worktreeDelete span, does not resume from a prior worktree.create span'` | Assert `worktree.rm` tạo `id` mới độc lập |
| `src/main/runtime/rpc/methods/__tests__/git-remote.test.ts` | `'git.worktree.add emits worktreeCreate span and forwards traceId into relay.call'` | Mock `relay.call`, assert `params.traceId === span.id` |
| (cùng file) | `'git.worktree.remove emits worktreeDelete span and forwards traceId'` | tương tự |
| (cùng file) | `'git.diff does NOT create any worktree:* span'` | Guard chống regressions — xác nhận method dùng chung không bị gắn nhầm tracer |
| `src/main/dev-server/__tests__/dev-server-relay-bridge.test.ts` | `'callWithTimeout resumes relay:agentCall span id from params.traceId'` | Assert `relayCallTracer.start` nhận đúng `resume.id` |
| (cùng file) | `'callWithTimeout starts a new relay:agentCall span id when params.traceId is absent'` | Backward-compat — không resume khi thiếu field |
| (cùng file) | `'callWithTimeout emits fail() with AGENT_NOT_CONNECTED before throwing, still resumes traceId'` | Cả nhánh lỗi cũng phải resume đúng |

**Test Targets:**

| Nhóm | Target |
|---|---|
| `tracers.test.ts` (mới + mở rộng) | ≥ 3 |
| `worktree.test.ts` (mở rộng) | ≥ 6 |
| `git-remote.test.ts` (mở rộng) | ≥ 6 |
| `dev-server-relay-bridge.test.ts` (mở rộng) | ≥ 5 |
| **Total** | **≥ 20** |

---

## 4. Acceptance Criteria

- [ ] `Tracers.worktreeCreate`, `worktreeFanOut`, `worktreeDelete`, `worktreeCompare`, `worktreeMerge` tồn tại trong `src/shared/trace/tracers.ts` với đúng flow name `worktree:create|fanOut|delete|compare|merge`
- [ ] Handler `worktree.create` (`worktree.ts`) và `git.worktree.add` (`git-remote.ts`) đều phát `worktree:create` span, `ok()` chứa `worktreeId`/`path`; logic `automationProvenance` reserve/release giữ nguyên không đổi
- [ ] Handler `worktree.rm` và `git.worktree.remove` đều phát `worktree:delete` span độc lập (không `resume` từ `worktree.create`)
- [ ] `WorktreeCreate`/`WorktreeRemove` (`worktree-schemas.ts`) và `ProjectWorktreeParam` (`git-remote.ts`) có field `traceId?: string` optional, không phá vỡ backward compatibility (test với params cũ không có `traceId` vẫn pass)
- [ ] Mọi `relay.call('git.exec', ...)` tại 2 call site BL-WT-01/03 trong `git-remote.ts` đính kèm `traceId: span.id`
- [ ] `DevServerRelayBridge.callWithTimeout()` resume `relay:agentCall` span bằng `params.traceId` khi có, tạo span mới khi không có
- [ ] `git.diff` (`git-remote.ts:101`) **không** bị gắn tracer `worktree:compare` — xác nhận bằng test guard
- [ ] Không có `span.step()` nào được thêm cho thao tác DB đơn dòng (theo CR-TRACE-000 §5) — review code xác nhận không có INSERT/SELECT/DELETE nào trong 2 handler được wrap ở mục 2.3/2.4 có `step()` riêng
