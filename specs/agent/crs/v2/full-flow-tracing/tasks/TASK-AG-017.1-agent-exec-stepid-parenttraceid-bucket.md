# TASK-AG-017.1: Extend agent.exec bucket with stepId/parentTraceId + confirm shell.exec/notification.send gap

**Phase:** 3
**SOL Ref:** [SOL-AG-TRACE-017](../solutions/SOL-AG-TRACE-017-workflow-orchestration.md)
**CR Ref:** [CR-TRACE-017](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-017-workflow-orchestration.md)
**Precondition:** Phase 0 + [TASK-AG-015.1](./TASK-AG-015.1-agent-exec-trace-field-bucket.md) (extends the SAME `agent.exec` bucket in `extractTraceFields()` — do not duplicate)
**Estimated time:** 1.5h
**Status:** ✅ Done (2026-08-03) — Extended the same `agent.exec` object literal created by TASK-AG-015.1 (`stepId`/`parentTraceId` appended, no second `if` block). Confirmed no `shell.exec`/`notification.send` case exists in `route()` (grep on the case labels returns nothing) — no handler added, per scope. `pnpm run typecheck:node` clean; `pnpm vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts` → 32/32 passed (pre-existing suite unaffected). `gitnexus_impact` on `extractTraceFields` returned LOW risk (1 direct caller: `dispatch`).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

`extractTraceFields()` trong `agent-rpc-dispatch.ts` là hàm bucket bị 3 task cùng mở rộng bucket `method === 'agent.exec'` (015.1 tạo, task này thêm `stepId`/`parentTraceId`, 018.2 sẽ thêm `taskId` sau) — target ĐÚNG symbol này (không phải cả file):

```bash
codegraph explore "extractTraceFields"
```

`extractTraceFields` là symbol MODIFY (đã tồn tại) — chạy thêm

```
gitnexus_impact({ target: "extractTraceFields", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Bối cảnh

`WorkflowOrchestrator`/`DAGBuilder`/`StepExecutors` (`src/main/workflow/*.ts`) là backend-only. Phạm vi agent-side chỉ bắt đầu tại điểm `StepExecutors.executeAgent()` gọi `relay.call('agent.exec', ...)` — CÙNG handler mà TASK-AG-015.1 (Profile) đã instrument. Task này **không lặp lại** phần base (`binary`/`argsCount`/`hasEnvOverride`/`timeoutMs`, `exitCode`/`timedOut`) — chỉ bổ sung `stepId` và `parentTraceId`.

**Gap chức năng có sẵn (KHÔNG tự ý fix ở đây):** `type='shell'` (→ `shell.exec`) và `type='notification'` (→ `notification.send`) trong Workflow steps **không có agent-side RPC handler nào** — xác nhận qua `grep -n "case 'shell.exec'\|case 'notification.send'" src/relay/agent-rpc-dispatch.ts` (không có kết quả). Request rơi vào `default:` → `MethodNotFound`. Đây là gap chức năng có trước, ngoài phạm vi "chỉ thêm tracer" — KHÔNG tự ý implement 2 handler này.

## File: `src/relay/agent-rpc-dispatch.ts` [MODIFY]

### `extractTraceFields()` — mở rộng bucket `agent.exec` đã có (từ TASK-AG-015.1)

```typescript
// src/relay/agent-rpc-dispatch.ts

function extractTraceFields(method: string, params: Record<string, unknown>): TraceFields {
  const p = params
  const str = (v: unknown) => (typeof v === 'string' ? v : undefined)
  const num = (v: unknown) => (typeof v === 'number' ? v : undefined)
  // ...existing helpers unchanged...

  if (method === 'agent.exec') {
    return {
      // (TASK-AG-015.1) base fields — request shape:
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
      // của Tracer.start() (CR-TRACE-000 §3.1), vì core API đó chưa ship. Chỉ có
      // giá trị SAU KHI backend (WorkflowOrchestrator.ts) được cập nhật để gửi
      // `traceId: stepSpan.id` kèm `parentTraceId: rootTraceId` trong params của
      // relay.call('agent.exec', ...) — cho tới lúc đó field này luôn undefined,
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

Vì `extractTraceFields()` merge trực tiếp vào `rpcTracer.start({ method, id, ...ctxFields })`, không cần sửa `dispatch()` thêm — `stepId`/`parentTraceId` tự động xuất hiện trong span `start` ngay khi params có các field này.

### Không thêm handler cho `shell.exec`/`notification.send`

Không có code thay đổi — cố ý. Khung dưới đây CHỈ minh hoạ vị trí future work, KHÔNG áp dụng ngay:

```typescript
// KHÔNG áp dụng ngay — placeholder minh hoạ vị trí future work.
//
// case 'shell.exec': {
//   try {
//     const { handleShellExec } = await import('./agent-shell-exec-handler') // chưa tồn tại
//     return (await handleShellExec(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
//   } catch (err: unknown) { ... }
// }
```

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "agent-rpc-dispatch" || echo "No errors"
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] Bucket `agent.exec` trong `extractTraceFields()` bao gồm `stepId` (đã có giá trị thật từ `StepExecutors.executeAgent()` ngay hôm nay) — MỞ RỘNG object literal từ TASK-AG-015.1, không tạo `if` block riêng
- [ ] Bucket `agent.exec` bao gồm `parentTraceId` (forward-compatible, `undefined` cho tới khi backend gửi field này — không throw lỗi)
- [ ] Comment trong code phân biệt rõ `parentTraceId` (field nghiệp vụ) khác với `resume` (core API CR-TRACE-000 §3.1, chưa ship)
- [ ] KHÔNG tự ý thêm implementation cho `shell.exec`/`notification.send`
- [ ] KHÔNG thay đổi hành vi của `agent.spawn` hay các method khác không thuộc `agent.exec`
- [ ] `pnpm run typecheck:node` pass
