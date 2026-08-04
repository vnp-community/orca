# CR-TRACE-001 — Worktree Management Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-001 |
| **Tên** | Worktree Management — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P1 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/worktree-management.md`, `src/main/runtime/rpc/methods/worktree.ts`, `src/main/runtime/rpc/methods/git-remote.ts`, `src/main/runtime/orca-runtime.ts`, `src/main/project/ProjectServerRouter.ts`, `src/main/dev-server/relay-connection-pool.ts`, `src/main/dev-server/dev-server-relay-bridge.ts`, `src/relay/agent-rpc-dispatch.ts`, `src/shared/trace/tracers.ts` |

---

## 1. Vấn đề

`docs/flows/logic/worktree-management.md` mô tả một luồng "lý tưởng" cho Web/Server mode: `relay.call('fs.stat')` (check disk) → `relay.call('git.worktree.add')` → INSERT DB → `relay.call('pty.create')`, tất cả trong một orchestration duy nhất ở `WorktreeManager.create()`. Khi điều tra source thật, bức tranh phức tạp hơn và **không có bất kỳ điểm nào phát tracer**:

1. **Không có class `WorktreeManager`.** Có hai đường dẫn thực thi khác nhau tuỳ mode:
   - **Desktop/local**: RPC method `worktree.create` (`src/main/runtime/rpc/methods/worktree.ts:70-146`) gọi thẳng `runtime.createManagedWorktree()` (`src/main/runtime/orca-runtime.ts:14543`) — chạy git trực tiếp hoặc qua SSH provider (`requireSshGitProvider`), không qua `relay.call`.
   - **Project/Web mode (multi-tenant Dev Server)**: các primitive git chạy qua `ProjectServerRouter.getRelayForProject()` → `DevServerRelayBridge.call()`, lộ ra qua RPC methods trong `git-remote.ts` (`git.worktree.add` dòng 319-332, `git.worktree.remove` dòng 336-348, `git.diff` dòng 101). **Không có** RPC method nào lắp ráp toàn bộ orchestration "tạo worktree + insert DB + tạo PTY" thành một lời gọi duy nhất như flow doc mô tả — mỗi phần là một RPC riêng, gọi từ client theo trình tự do UI quyết định.
2. **`git.worktree.add` không thực sự gọi relay method tên `git.worktree.add`.** Handler thật (`git-remote.ts:325-331`) forward xuống `relay.call('git.exec', { cwd, args: ['worktree', 'add', path, branch] })` — nghĩa là ở tầng relay/Dev Server, mọi thao tác git đều đi qua **một** relay method chung (`git.exec`), dù `agent-rpc-dispatch.ts` (Dev Server side) vẫn có case riêng `'git.worktree.add'` (dòng 341) không rõ còn được caller nào khác dùng. Không có trace nào để xác nhận method nào thực sự chạy trong một lần gọi cụ thể — hiện chỉ có thể suy luận từ đọc code, không thể debug từ log runtime.
3. **Không có bước disk-space check nào tồn tại trong code thật** (`fs.stat` trước khi tạo worktree không được gọi ở đâu cả) — nếu Dev Server hết dung lượng, `git worktree add` sẽ fail muộn, sâu bên trong `git.exec`, và log hiện tại (`relay:agentCall` — GAP-3 trong CR-TRACE-000) chỉ ghi `{ devServerId, method: 'git.exec' }`, không có `args` hay `path`, nên không biết lệnh nào fail vì lý do gì.
4. **BL-WT-03 (xoá an toàn) chạy 2 RPC round-trip cách nhau bởi thời gian người dùng xác nhận dialog** (`worktree.checkSafety` rồi `worktree.delete`) và mỗi round-trip lại gồm nhiều `relay.call` (agent status, PTY destroy, git worktree remove) cộng DELETE ở 2 bảng DB. Không có cách nào hôm nay biết bước nào trong chuỗi đó bị treo khi user báo "xoá worktree bị đứng".
5. **BL-WT-02 (fan-out) và BL-WT-05 (merge) chưa có RPC method chuyên biệt nào trong `git-remote.ts` hay `orca-runtime.ts`** khớp với mô tả trong flow doc (không tìm thấy `worktree.fanOut`, `git.merge`, hay tương đương qua grep). Đây là gap triển khai thực tế, không chỉ gap tracing — CR này đặc tả tracer name/convention trước, để khi 2 sub-flow này được implement thật, engineer chỉ việc cắm instrumentation theo đúng convention thay vì tự nghĩ ra field name.

Vì không có instrumentation ở bất kỳ layer nào (RPC handler → ProjectServerRouter → RelayConnectionPool → DevServerRelayBridge → Dev Server dispatch), khi một worktree create/delete chậm hoặc lỗi, hiện tại **không có cách nào biết bước nào (auth/permission check, relay connection setup, git exec trên Dev Server, DB write) là nguyên nhân** ngoài việc thêm `console.log` thủ công và reproduce lại.

## 2. Thành phần & Transport liên quan

| Thành phần | Layer | Transport | CR-TRACE-000 §3.3 row áp dụng |
|------------|-------|-----------|-------------------------------|
| Browser (React UI, worktree sidebar/dialogs) | UI | — | Nguồn tạo `traceId` đầu tiên |
| `src/main/runtime/rpc/methods/worktree.ts` (`worktree.create`, `worktree.rm`) | Business Logic (desktop/local) | WebSocket RPC (Browser ↔ Orca Server) | "WebSocket RPC (Browser ↔ Orca Server)" |
| `src/main/runtime/rpc/methods/git-remote.ts` (`git.worktree.add/remove`, `git.diff`) | Business Logic (Project/Web mode) | WebSocket RPC (Browser ↔ Orca Server) | "WebSocket RPC (Browser ↔ Orca Server)" |
| `ProjectServerRouter.getRelayForProject()` (`src/main/project/ProjectServerRouter.ts`) | Infrastructure | in-process (không băng qua boundary) | — (không cần span riêng, gộp vào step của RPC handler) |
| `RelayConnectionPool.getOrConnect()` (`src/main/dev-server/relay-connection-pool.ts`) | Infrastructure | in-process | — |
| `DevServerRelayBridge.call()` / `callWithTimeout()` (`src/main/dev-server/dev-server-relay-bridge.ts:562-630`) | Transport | `relay.call()` (Orca Server ↔ Dev Server, SSH-multiplexed session hoặc Agent WS) | "`relay.call()` (Orca Server ↔ Dev Server)" — đã có `relayCallTracer` (`relay:agentCall`), sẽ resume bằng `traceId` do domain tracer truyền xuống |
| `src/relay/agent-rpc-dispatch.ts` (case `git.exec`, `git.worktree.*`) | Runtime (Dev Server) | nhận qua Agent WS JSON-RPC 2.0 hoặc SSH exec tuỳ session mode | "Agent WS JSON-RPC 2.0" nếu direct/relay-websocket mode; nếu session là SSH channel thuần, áp dụng ghi chú "SSH exec ... không lan truyền" của §3.3 — **cần xác định session mode thực tế trước khi wire propagation ở layer này** |
| Server DB (SQLite `orca_worktrees`, `orca_terminal_sessions`) | Persistence | in-process | — |

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  worktreeCreate:      createTracer('worktree:create'),      // BL-WT-01
  worktreeFanOut:      createTracer('worktree:fanOut'),      // BL-WT-02
  worktreeDelete:      createTracer('worktree:delete'),      // BL-WT-03 (safetyCheck + delete dùng chung tracer, span riêng)
  worktreeCompare:     createTracer('worktree:compare'),     // BL-WT-04
  worktreeMerge:       createTracer('worktree:merge'),       // BL-WT-05
}
```

## 4. Instrumentation theo từng sub-flow

### BL-WT-01 — Tạo Worktree

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận RPC `worktree.create` | `start` | `{ repoSelector, name, baseBranch }` | `src/main/runtime/rpc/methods/worktree.ts:71` (handler) |
| Gọi `createManagedWorktree` | `step('createManagedWorktree')` | `{ repoSelector }` | `src/main/runtime/orca-runtime.ts:14543` |
| (Project/Web mode) git worktree add qua relay | `step('relay-git-worktree-add')` | `{ devServerId, path }` | `src/main/runtime/rpc/methods/git-remote.ts:319-332` → `relay.call('git.exec', {...})` |
| Hoàn tất, trả kết quả | `ok` | `{ worktreeId, path }` hoặc `fail(err, { repoSelector })` | cùng handler |

```typescript
// src/main/runtime/rpc/methods/worktree.ts — trong handler 'worktree.create'
const span = Tracers.worktreeCreate.start(
  { repoSelector: params.repo, baseBranch: params.baseBranch },
  params.traceId ? { id: params.traceId } : undefined
)
try {
  const repo = await runtime.showRepo(params.repo)
  span.step('resolve-repo', { repoId: repo.id, isFolderRepo: isFolderRepo(repo) })
  const result = await runtime.createManagedWorktree({ /* ... existing args ... */ })
  span.ok({ worktreeId: result.id, path: result.path })
  return result
} catch (error) {
  span.fail(error, { repoSelector: params.repo })
  throw error
}
```

> Ghi chú: `git-remote.ts`'s `git.worktree.add` RPC (Project/Web mode) là một lời gọi RPC **riêng biệt**, không nằm trong cùng call stack với `worktree.create` ở trên — nếu UI gọi cả hai (ví dụ Project mode dùng `git.worktree.add` trực tiếp thay vì `worktree.create`), instrument thêm một `span.step()` tương tự trong handler `git.worktree.add` của `git-remote.ts`, dùng **cùng tracer** `worktreeCreate` vì cùng thuộc BL-WT-01.

### BL-WT-02 — Fan-out Prompt tới Nhiều Worktree

**Chưa có RPC method thực thi (`worktree.fanOut` không tồn tại trong code hiện tại — chưa xác định file cụ thể, cần điều tra/implement khi triển khai).** Đặc tả instrumentation dưới đây áp dụng khi sub-flow này được xây:

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận RPC `worktree.fanOut` | `start` | `{ projectId, n, baseRef }` | chưa xác định file cụ thể |
| Mỗi child worktree (i = 1..N) | `worktreeCreate.start({ parentTraceId: fanOutSpan.id, index: i })` (span/id riêng) | `{ parentTraceId, index }` | tái dùng handler BL-WT-01 |
| Mỗi child agent spawn (i = 1..N) | dùng `agentOrch:spawn` (CR-TRACE-002) với `parentTraceId` tương tự | `{ parentTraceId, index }` | xem CR-TRACE-002 BL-AG-01 |
| Tổng hợp kết quả `Promise.allSettled` | `ok` | `{ succeeded, failed }` | chưa xác định file cụ thể |

```typescript
// Ví dụ khi worktree.fanOut được implement
const span = Tracers.worktreeFanOut.start({ projectId, n })
const results = await Promise.allSettled(
  Array.from({ length: n }, (_, i) =>
    createOneWorktree({ ...opts, index: i, parentTraceId: span.id })
  )
)
span.ok({ succeeded: results.filter(r => r.status === 'fulfilled').length, failed: results.filter(r => r.status === 'rejected').length })
```

> **Nguyên tắc correlation fan-out:** mỗi child KHÔNG dùng `resume` (không nên N con chia sẻ 1 span `id`, vì mỗi lần tạo worktree có latency/lỗi độc lập cần thấy riêng trong TracePanel). Thay vào đó, child span mang field `parentTraceId` trỏ về `id` của span `worktree:fanOut` cha — đây là quy ước bổ sung cho pattern 1-cha-N-con, khác với quy ước "resume" (1-1 nối tiếp) của CR-TRACE-000 §3.1.

### BL-WT-03 — Xóa Worktree An Toàn

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận RPC `worktree.checkSafety` | `start` (span #1) | `{ worktreeId }` | chưa xác định file cụ thể (không tìm thấy `worktree.checkSafety` qua grep trong `rpc/methods/`) |
| Check git status / PTY / agent qua relay | `step('git-status')`, `step('pty-check')`, `step('agent-status')` | `{ devServerId }` | chưa xác định file cụ thể |
| Trả safety result | `ok` (đóng span #1) | `{ hasUncommittedChanges, hasRunningAgent }` | — |
| Nhận RPC `worktree.rm` (thực thi xoá) | `start` (span #2, tracer giống, id mới) | `{ worktreeId, force }` | `src/main/runtime/rpc/methods/worktree.ts:230-241` (handler `worktree.rm`) → `runtime.removeManagedWorktree()` (`src/main/runtime/orca-runtime.ts:16888`) |
| Kill agent / destroy PTY / git worktree remove | `step('kill-agent')`, `step('pty-destroy')`, `step('git-worktree-remove')` | `{ devServerId }` | Project/Web mode: `git-remote.ts:336-348` (`git.worktree.remove` → `relay.call('git.exec', ['worktree','remove',...])`); desktop mode: `orca-runtime.ts:16888` gọi `killAllProcessesForWorktree` |
| Hoàn tất | `ok` / `fail` (đóng span #2) | `{ worktreeId }` | — |

```typescript
// src/main/runtime/rpc/methods/worktree.ts — handler 'worktree.rm'
const span = Tracers.worktreeDelete.start(
  { worktreeId: params.worktree, force: params.force === true },
  params.traceId ? { id: params.traceId } : undefined
)
try {
  const result = await runtime.removeManagedWorktree(params.worktree, params.force === true, params.runHooks === true)
  span.ok({ removed: true })
  return { removed: true, ...result }
} catch (error) {
  span.fail(error, { worktreeId: params.worktree })
  throw error
}
```

> **Vì `worktree.checkSafety` và `worktree.rm` là hai round-trip RPC cách nhau bởi thời gian người dùng đọc confirmation dialog, KHÔNG dùng `resume` giữa 2 span này** — mỗi cái tạo `id` riêng qua `.start()` bình thường (không truyền `resume`), tránh `elapsedMs` bị tính luôn cả thời gian người dùng "suy nghĩ". Nếu cần liên kết 2 span trong UI, dùng field `worktreeId` chung để join, không dùng `id`.

### BL-WT-04 — So sánh Kết quả Giữa Worktrees

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận RPC `worktree.compare` | `start` | `{ projectId, worktreeIds, baseRef }` | chưa xác định file cụ thể (không có RPC method `worktree.compare`; nguyên liệu gần nhất là `git.diff` — `src/main/runtime/rpc/methods/git-remote.ts:101-116`) |
| `git diff --stat` × N qua relay | `step('git-diff', { index: i })` | `{ devServerId, worktreeId }` | `git-remote.ts:101-116` (`git.diff` → `relay.call('git.exec', ['diff', ...])`), gọi N lần |
| SELECT session summary × N | không cần `step()` riêng (single-row SELECT, theo CR-TRACE-000 §5) — gộp field vào `ok()` | `{ sessionsLoaded: N }` | chưa xác định file cụ thể |
| Trả comparison data | `ok` | `{ worktreeCount: N }` | — |

```typescript
// Khi worktree.compare được implement — mỗi git.diff dùng resume để giữ traceId
const span = Tracers.worktreeCompare.start({ projectId, worktreeCount: worktreeIds.length })
const diffs = await Promise.all(
  worktreeIds.map((id, i) => {
    span.step('git-diff', { index: i, worktreeId: id })
    return relay.call('git.exec', { cwd: paths[i], args: ['diff', `${baseRef}...`, '--stat'], traceId: span.id })
  })
)
span.ok({ worktreeCount: worktreeIds.length })
```

### BL-WT-05 — Merge Worktree Thắng

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận RPC `worktree.merge` | `start` | `{ worktreeId, strategy }` | chưa xác định file cụ thể (không có RPC method `worktree.merge` hay `git.merge` trong `git-remote.ts` hiện tại) |
| Conflict check (`merge-base --is-ancestor`) | `step('conflict-check')` | `{ branch }` | chưa xác định file cụ thể |
| Thực thi strategy (merge/squash/rebase) | `step('execute-strategy', { strategy })` | `{ strategy }` | chưa xác định file cụ thể |
| Cleanup (BL-WT-03 × N-1) nếu `cleanup=true` | `step('cleanup')`, gọi lại `Tracers.worktreeDelete` cho từng worktree con | `{ count: N-1 }` | tái dùng BL-WT-03 |
| Hoàn tất | `ok` / `fail` | `{ strategy, mergedBranch }` | — |

```typescript
// Khi worktree.merge được implement
const span = Tracers.worktreeMerge.start({ worktreeId, strategy })
try {
  span.step('conflict-check')
  await relay.call('git.exec', { args: ['merge-base', '--is-ancestor', branch, 'main'], traceId: span.id })
  span.step('execute-strategy', { strategy })
  await relay.call('git.exec', { args: mergeArgsFor(strategy, branch), traceId: span.id })
  if (cleanup) span.step('cleanup', { count: siblingWorktreeIds.length })
  span.ok({ strategy })
} catch (err) {
  span.fail(err, { strategy })
  throw err
}
```

## 5. Lan truyền traceId qua transport của flow này

Áp dụng CR-TRACE-000 §3.3 cụ thể cho worktree flows:

1. **Browser → Orca Server (WebSocket RPC)**: Browser tạo `traceId` bằng tracer riêng (browser-side, xem `src/shared/trace/browser.ts`) trước khi gửi `worktree.create` / `worktree.rm` / `git.worktree.add` v.v. Envelope RPC thật hiện tại (`RpcRequest` — `src/main/runtime/rpc/core.ts:33-38`) chỉ có `{ id, authToken, method, params }`; `id` ở đây dùng để match request/response, **không phải** trace id. Cần thêm field `traceId?: string` **bên trong `params`** của mỗi method schema (ví dụ `WorktreeCreate`, `WorktreeRemove` trong `worktree-schemas.ts`) — không đụng field `id` sẵn có, nhất quán với lý do CR-TRACE-000 đưa `traceId` vào `params._trace.id` cho Agent WS JSON-RPC (tránh đụng id request/response).
2. **RPC handler → `ProjectServerRouter`/`RelayConnectionPool`**: không băng qua boundary (in-process) nên không cần field wire riêng — span `step()` là đủ.
3. **`DevServerRelayBridge.call()` → Dev Server**: tại điểm gọi (`git-remote.ts:327` `relay.call('git.exec', { cwd, args })`), thêm `traceId: span.id` vào params trước khi gọi. `callWithTimeout()` (`dev-server-relay-bridge.ts:562`) hiện gọi `relayCallTracer.start({ devServerId, method })` không có `resume` — sau khi CR-TRACE-000 core API ship, sửa thành `relayCallTracer.start({ devServerId, method }, params.traceId ? { id: params.traceId } : undefined)` để span `relay:agentCall` tiếp nối đúng `id` từ `worktree:create`.
4. **Dev Server nhận request (`agent-rpc-dispatch.ts` case `git.exec`)**: đây là nơi cần xác nhận session mode (Agent WS JSON-RPC 2.0 hay SSH exec thuần) — nếu là JSON-RPC 2.0 thật, `traceId` nằm ở `params._trace.id` theo §3.3; nếu route qua SSH channel multiplexer thuần (`SshChannelMultiplexer.request()` thấy trong `dev-server-relay-bridge.ts:616`), traceId vẫn có thể đi trong `params` vì đây không phải raw shell exec — **không rơi vào trường hợp "SSH exec / remote shell không lan truyền được"** của §3.3 (trường hợp đó chỉ áp dụng khi lệnh chạy trực tiếp trong remote shell của user, không phải JSON-RPC qua kênh SSH).

## Acceptance Criteria

- [ ] `Tracers.worktreeCreate`, `worktreeFanOut`, `worktreeDelete`, `worktreeCompare`, `worktreeMerge` được thêm vào `tracers.ts` theo đúng tên ở mục 3
- [ ] Handler `worktree.create` (`worktree.ts`) và `git.worktree.add` (`git-remote.ts`) phát `worktree:create` span với `ok()` chứa `worktreeId` và `path`
- [ ] Handler `worktree.rm` phát `worktree:delete` span riêng biệt với span của `worktree.checkSafety` (không dùng `resume` giữa 2 round-trip có user-confirmation ở giữa)
- [ ] `WorktreeCreate`/`WorktreeRemove` params schema (`worktree-schemas.ts`) có field `traceId?: string` optional, không phá vỡ backward compatibility
- [ ] `relay.call('git.exec', ...)` tại các call site BL-WT-01/03/04/05 đính kèm `traceId: span.id`
- [ ] `worktree:fanOut` span có field `parentTraceId` xuất hiện ở từng child `worktree:create`/`agentOrch:spawn` span khi BL-WT-02 được implement
- [ ] Không có `span.step()` nào được thêm cho single-row SQLite SELECT/INSERT/DELETE thuần tuý (theo CR-TRACE-000 §5) — các thao tác DB chỉ xuất hiện như field trong `ok()`/`fail()`
- [ ] BL-WT-04/05 mà chưa có RPC method thật: tracer đã được đặt tên và tài liệu hoá trong `tracers.ts`, sẵn sàng cắm vào khi method được implement, không cần đặt tên lại
