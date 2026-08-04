# SOL-AG-TRACE-017: Workflow Orchestration — Agent-Side Tracing Implementation

**CR Ref:** [CR-TRACE-017](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-017-workflow-orchestration.md)
**TDD Ref:** TDD-AG-12 (ProfileAware Agent Spawner), TDD-AG-06 (Tool Handlers)
**File(s):** `src/relay/agent-rpc-dispatch.ts` [MODIFY]
**Mức độ:** 🟡 trung bình
**Thời gian ước tính:** 2h
**Status:** Proposed

---

## 1. Phạm vi (Agent-side only)

`WorkflowOrchestrator`/`DAGBuilder`/`StepExecutors` (`src/main/workflow/*.ts`) là backend-only — build DAG, chia wave, `Promise.allSettled` đều chạy trong Orca Server process. Phạm vi agent-side chỉ bắt đầu tại điểm `StepExecutors` gọi `relay.call()` cho từng loại step. Đã đọc trực tiếp `src/main/workflow/StepExecutors.ts` để xác định chính xác 3 method RPC mà step type nào gọi (dòng 4-9, 84-155):

| Step type | Method gọi qua `relay.call()` | File:line backend | Có handler phía agent? |
|---|---|---|---|
| `agent` | `agent.exec` | `StepExecutors.ts:88` (`executeAgent()`) | **CÓ** — `agent-rpc-dispatch.ts:502-557`, cùng path với SOL-AG-TRACE-015/018 |
| `shell` | `shell.exec` | `StepExecutors.ts:108` (`executeShell()`) | **KHÔNG** — xem Gap 2 bên dưới |
| `notification` | `notification.send` | `StepExecutors.ts:149` (`executeNotification()`) | **KHÔNG** — xem Gap 2 bên dưới |
| `webhook` | — (native `fetch()`, không qua relay) | `StepExecutors.ts:122-141` | Không áp dụng — không chạm Dev Server |
| `condition` | — (in-process eval) | `StepExecutors.ts:159-178` | Không áp dụng |

**Kết luận phạm vi:** duy nhất `type='agent'` có một handler thật sự tồn tại phía agent để instrument. `type='shell'` và `type='notification'` **không có RPC handler tương ứng trên agent** — xem Gap 2, đây là gap chức năng có trước, không phải điều solution tracing này có thể lấp bằng cách thêm tracer.

**Tái sử dụng minh bạch:** đường đi `agent.exec` (case tại `agent-rpc-dispatch.ts:502-557`) là đúng path đã được SOL-AG-TRACE-015 (Profile → `ProfileAwareAgentSpawner.spawn()`) và SOL-AG-TRACE-018 (Task Graph → `TaskAgentExecutor` → `ProfileAwareAgentSpawner.spawn()`) phân tích. Solution này **không lặp lại** phần base (bucket `agent.exec` cho `binary`/`argsCount`/`hasEnvOverride`/`timeoutMs`, và `exitCode`/`timedOut` trong `ok()`) — xem SOL-AG-TRACE-015 §3.1-3.2 cho phần đó. Solution này chỉ bổ sung phần **riêng của CR-TRACE-017**: field `stepId` và `parentTraceId` để nhóm các step-span của cùng 1 workflow execution.

## 2. Gap hiện tại

**Gap 1 — thiếu `stepId`/`parentTraceId` trong bucket `agent.exec`:** `StepExecutors.executeAgent()` (`StepExecutors.ts:88-93`) đã gửi `stepId: step.id` trong params **ngay hôm nay** (không cần thay đổi backend gì thêm để field này bắt đầu có giá trị) — nhưng `extractTraceFields()` phía agent hiện không đọc field này. `parentTraceId` thì CR-TRACE-017 §4 (mô hình parent-correlation) yêu cầu backend gửi kèm (`rootTraceId` của `workflow:execute` span cha) — đây là **prerequisite backend** chưa tồn tại trong `StepExecutors.ts` hiện tại (`relay.call('agent.exec', {...})` ở dòng 88 không có field `traceId`/`parentTraceId` nào cả) — thuộc solution backend riêng, ngoài phạm vi tài liệu này, nhưng code phía agent cần sẵn sàng đọc field này ngay khi backend bổ sung.

**Gap 2 — `shell.exec` và `notification.send` không tồn tại phía agent (xác nhận qua grep, không phải suy đoán):**

```
$ grep -n "case 'shell.exec'\|case 'notification.send'" src/relay/agent-rpc-dispatch.ts
(không có kết quả)
```

Route switch trong `agent-rpc-dispatch.ts` (dòng 164-708) có `case 'shell.eval'` (dòng 592 — method **khác tên**, dùng nội bộ để resolve `~` cho `devServer.browseDir`, params khác hẳn `{script, env}` mà `shell.exec` cần) và hoàn toàn không có case nào cho `notification.send`. Nghĩa là: khi một workflow chạy step `type='shell'` hoặc `type='notification'`, request tới Dev Server Agent rơi vào nhánh `default:` (dòng 706-707) → trả về `MethodNotFound`. Đây là **gap chức năng có sẵn từ trước**, không liên quan tới tracing — không thể thêm `span` cho một handler chưa tồn tại. Solution này **không tự ý implement** `shell.exec`/`notification.send` (nằm ngoài phạm vi "chỉ thêm tracer" của nhiệm vụ), chỉ ghi nhận rõ và đề xuất: khi hai method này được implement (ticket riêng, ngoài CR-TRACE-017), áp dụng đúng pattern instrumentation của `git.exec`/`agent.exec` đã có trong cùng file (mỗi case dùng `try { const {...} = await import(...) } catch ...`, handler riêng tự tạo tracer theo namespace `agent:*`).

## 3. Full Implementation

### 3.1. Mở rộng bucket `agent.exec` với `stepId` + `parentTraceId`

```typescript
// src/relay/agent-rpc-dispatch.ts

function extractTraceFields(method: string, params: Record<string, unknown>): TraceFields {
  const p = params
  const str = (v: unknown) => (typeof v === 'string' ? v : undefined)
  const num = (v: unknown) => (typeof v === 'number' ? v : undefined)
  // ...existing helpers unchanged...

  if (method === 'agent.exec') {
    return {
      // (SOL-AG-TRACE-015) base fields — request shape:
      binary:         str(p['binary']),
      argsCount:      Array.isArray(p['args']) ? (p['args'] as unknown[]).length : undefined,
      hasEnvOverride: p['env'] !== undefined && p['env'] !== null,
      timeoutMs:      num(p['timeoutMs']),
      // CR-TRACE-017 BL-WF-02: StepExecutors.executeAgent() đã gửi `stepId` từ
      // hôm nay (StepExecutors.ts:89) — field này lập tức có giá trị không cần
      // thay đổi gì ở backend.
      stepId: str(p['stepId']),
      // CR-TRACE-017 §4: `parentTraceId` là field nghiệp vụ để TracePanel nhóm
      // mọi step-span của cùng 1 workflow execution — KHÔNG PHẢI cơ chế `resume`
      // của Tracer.start() (CR-TRACE-000 §3.1), vì core API đó chưa ship (xem
      // src/shared/trace/index.ts — Tracer.start() hiện chỉ nhận `fields`).
      // Chỉ có giá trị SAU KHI backend (WorkflowOrchestrator.ts) được cập nhật để
      // gửi `traceId: stepSpan.id` kèm `parentTraceId: rootTraceId` trong params
      // của relay.call('agent.exec', ...) — cho tới lúc đó field này luôn undefined,
      // không gây lỗi (agent-side code đã sẵn sàng nhận, không cần sửa lại lần 2).
      parentTraceId: str(p['parentTraceId']),
    }
  }

  if (method.startsWith('agent.')) {
    return {
      session: str(p['sessionId']),
      cmd:     truncCmd(p['cmd'] ?? p['command']),
    }
  }

  return {}
}
```

Vì `extractTraceFields()` được merge trực tiếp vào `rpcTracer.start({ method, id, ...ctxFields })` (`agent-rpc-dispatch.ts:128`), không cần sửa `dispatch()` thêm — `stepId`/`parentTraceId` tự động xuất hiện trong span `start` ngay khi params có các field này.

### 3.2. Không thêm handler cho `shell.exec`/`notification.send`

Không có code thay đổi ở mục này — cố ý. Nếu trong tương lai 2 method này được implement, đoạn khung dưới đây minh hoạ **vị trí** sẽ cần thêm tracer (không phải patch áp dụng ngay — chỉ để tài liệu hoá quyết định "chờ implementation trước"):

```typescript
// KHÔNG áp dụng ngay — placeholder minh hoạ vị trí future work, tương tự cách
// CR-TRACE-017 §4 BL-WF-03 tự đánh dấu "placeholder instrumentation spec".
//
// case 'shell.exec': {
//   try {
//     const { handleShellExec } = await import('./agent-shell-exec-handler') // chưa tồn tại
//     return (await handleShellExec(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
//   } catch (err: unknown) { ... }
// }
```

## 4. Test Plan (Vitest)

File: `src/relay/__tests__/agent-rpc-dispatch.test.ts` (mở rộng, dùng lại `registerTraceSink` pattern từ SOL-AG-TRACE-015).

```typescript
describe('agent.exec — stepId / parentTraceId (CR-TRACE-017)', () => {
  it('surfaces stepId when StepExecutors sends it', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    await dispatcher.dispatch(ws, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'agent.exec',
      params: { binary: 'echo', args: [], cwd: '/tmp', stepId: 'step-42' },
    })
    unregister()
    const start = events.find(e => e.flow === 'agent:rpc' && e.level === 'start')!
    expect(start.fields.stepId).toBe('step-42')
  })

  it('surfaces parentTraceId when present (forward-compat with future backend change)', async () => { /* params.parentTraceId = 'root-abc123' */ })

  it('omits stepId/parentTraceId cleanly for non-workflow agent.exec callers (Profile/Task Graph)', async () => {
    // SOL-AG-TRACE-015/018 callers don't send stepId — confirm no crash, field simply absent
  })
})

describe('shell.exec / notification.send — documented gap (CR-TRACE-017)', () => {
  it('shell.exec returns MethodNotFound today (no agent-side handler exists)', async () => {
    await dispatcher.dispatch(ws, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'shell.exec', params: { script: 'echo hi' },
    })
    const resp = lastResponseJson(ws)
    expect(resp.error.code).toBe(AgentErrorCode.MethodNotFound)
  })

  it('notification.send returns MethodNotFound today (no agent-side handler exists)', async () => { /* same pattern */ })
})
```

Hai test cuối **cố tình** khẳng định trạng thái gap hiện tại (regression guard) — nếu ai đó implement `shell.exec`/`notification.send` sau này mà quên cập nhật test này, test sẽ fail và nhắc review lại phần "Gap 2" của solution.

## 5. Acceptance Criteria

- [ ] Bucket `agent.exec` trong `extractTraceFields()` bao gồm `stepId` (đã có giá trị thật từ `StepExecutors.executeAgent()` ngay hôm nay)
- [ ] Bucket `agent.exec` bao gồm `parentTraceId` (forward-compatible, giá trị `undefined` cho tới khi backend `WorkflowOrchestrator.ts` gửi field này — không throw lỗi, không cần sửa lại agent-side lần 2)
- [ ] Comment trong code phân biệt rõ `parentTraceId` (field nghiệp vụ) khác với `resume` (core API CR-TRACE-000 §3.1, chưa ship) — tránh nhầm lẫn khi review
- [ ] Xác nhận và test rõ ràng: `shell.exec`/`notification.send` **không có** agent-side handler — không tự ý thêm implementation ngoài phạm vi tracing
- [ ] Không thay đổi hành vi của `agent.spawn` (interactive PTY) hay các method khác không thuộc `agent.exec`
- [ ] Test `registerTraceSink`-based xác nhận field xuất hiện đúng khi params có, biến mất sạch (không phải `"undefined"` string) khi params không có
- [ ] Ghi rõ trong tài liệu: phần base instrumentation của `agent.exec` (`binary`/`argsCount`/`exitCode`/...) thuộc SOL-AG-TRACE-015, không lặp lại ở đây
