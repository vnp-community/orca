# TASK-AG-015.2: Add agent.exec trace-field-bucket tests to agent-rpc-dispatch.test.ts

**Phase:** 3
**SOL Ref:** [SOL-AG-TRACE-015](../solutions/SOL-AG-TRACE-015-profile.md)
**CR Ref:** [CR-TRACE-015](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-015-profile.md)
**Precondition:** Phase 0 + [TASK-AG-015.1](./TASK-AG-015.1-agent-exec-trace-field-bucket.md)
**Estimated time:** 1h
**Status:** ✅ Done (2026-08-03) — 4 test cases added to `agent-rpc-dispatch.test.ts`. Adapted the "agent.spawn unaffected" test to assert `argsCount`/`hasEnvOverride` are undefined (fields unique to the new bucket) rather than `binary` being undefined — the legacy `agent.` bucket already reads `binary` from `model`/`modelId`, which is pre-existing behavior unrelated to this task. `pnpm vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts` → 32/32 passed.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "extractTraceFields"
codegraph explore "createRpcDispatcher"
```

Cả 2 đều là symbol MODIFY (đã tồn tại, vừa được TASK-AG-015.1 mở rộng bucket `agent.exec`) — chạy thêm impact analysis:

```
gitnexus_impact({ target: "extractTraceFields", direction: "upstream" })
gitnexus_impact({ target: "createRpcDispatcher", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## File: `src/relay/__tests__/agent-rpc-dispatch.test.ts` [MODIFY]

Mở rộng file test hiện có — dùng lại `MockWs`/`lastResponseJson` đã có sẵn, thêm `registerTraceSink` từ `../../shared/trace` để bắt `TraceEvent[]` thay vì phải bật `ORCA_TRACE=1` (`emit()` luôn gọi sink bất kể cờ trace).

```typescript
import { registerTraceSink, type TraceEvent } from '../../shared/trace'

describe('agent.exec — extractTraceFields (CR-TRACE-015)', () => {
  it('start event includes binary/argsCount/hasEnvOverride/timeoutMs, not session/cmd', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    const dispatcher = createRpcDispatcher([], MOCK_CONFIG, MOCK_LOG)
    const ws = new MockWs()
    await dispatcher.dispatch(ws as unknown as WebSocket, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'agent.exec',
      params: { binary: 'echo', args: ['hi'], cwd: '/tmp', env: { FOO: 'bar' }, timeoutMs: 5000 },
    })
    unregister()
    const start = events.find(e => e.flow === 'agent:rpc' && e.level === 'start')!
    expect(start.fields.binary).toBe('echo')
    expect(start.fields.argsCount).toBe(1)
    expect(start.fields.hasEnvOverride).toBe(true)
    expect(start.fields.timeoutMs).toBe(5000)
    expect(start.fields.session).toBeUndefined()
  })

  it('ok event includes exitCode and timedOut from the agent.exec result', async () => { /* spawn echo, assert ok.fields.exitCode === 0, timedOut === false */ })

  it('agent.spawn (interactive) still uses the legacy session/cmd bucket, unaffected', async () => { /* method: agent.spawn — assert fields.session/cmd path, no binary/argsCount */ })

  it('agent.exec without env param → hasEnvOverride is false, not undefined-vs-false ambiguity', async () => { /* params without env key */ })
})
```

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

- [ ] 4 test case trên có mặt trong `agent-rpc-dispatch.test.ts`
- [ ] Test "ok event includes exitCode and timedOut" xác nhận `span.ok()` chứa cả 2 field từ `response.result`
- [ ] Test "agent.spawn ... unaffected" xác nhận bucket cũ (`session`/`cmd`) không bị phá vỡ cho `agent.spawn`
- [ ] Test "hasEnvOverride is false" phân biệt rõ `false` (không có key `env`) khác với `undefined`
- [ ] `pnpm vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts` pass toàn bộ (kể cả test case từ TASK-AG-002.4)
