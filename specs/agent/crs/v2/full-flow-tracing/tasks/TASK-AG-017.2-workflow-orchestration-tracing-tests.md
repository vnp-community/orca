# TASK-AG-017.2: Add stepId/parentTraceId tests + shell.exec/notification.send gap regression guard

**Phase:** 3
**SOL Ref:** [SOL-AG-TRACE-017](../solutions/SOL-AG-TRACE-017-workflow-orchestration.md)
**CR Ref:** [CR-TRACE-017](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-017-workflow-orchestration.md)
**Precondition:** Phase 0 + [TASK-AG-017.1](./TASK-AG-017.1-agent-exec-stepid-parenttraceid-bucket.md)
**Estimated time:** 1h
**Status:** ✅ Done (2026-08-03) — Added the 5 test cases (3× stepId/parentTraceId + 2× MethodNotFound regression guard) to `agent-rpc-dispatch.test.ts`; added `AgentErrorCode` import. `pnpm run typecheck:node` clean; `pnpm vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts` → 37/37 passed.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "extractTraceFields"
codegraph explore "createRpcDispatcher"
```

Cả 2 đều là symbol MODIFY (đã tồn tại, vừa được TASK-AG-017.1 mở rộng bucket `agent.exec`) — chạy thêm impact analysis:

```
gitnexus_impact({ target: "extractTraceFields", direction: "upstream" })
gitnexus_impact({ target: "createRpcDispatcher", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## File: `src/relay/__tests__/agent-rpc-dispatch.test.ts` [MODIFY]

Mở rộng, dùng lại `registerTraceSink` pattern từ TASK-AG-015.2.

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
    // TASK-AG-015.1/TASK-AG-018.2 callers don't send stepId — confirm no crash, field simply absent
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

Hai test cuối **cố tình** khẳng định trạng thái gap hiện tại (regression guard) — nếu ai đó implement `shell.exec`/`notification.send` sau này mà quên cập nhật test này, test sẽ fail và nhắc review lại phần "Gap 2" của SOL-AG-TRACE-017.

## Verification

```bash
pnpm vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] Test `registerTraceSink`-based xác nhận field xuất hiện đúng khi params có, biến mất sạch (không phải chuỗi `"undefined"`) khi params không có
- [ ] Test "omits stepId/parentTraceId cleanly" xác nhận caller khác (Profile/Task Graph) không bị crash
- [ ] 2 test regression guard cho `shell.exec`/`notification.send` xác nhận `MethodNotFound` — PHẢI fail rõ ràng nếu ai đó implement 2 handler này mà không cập nhật lại test
- [ ] `pnpm vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts` pass toàn bộ
