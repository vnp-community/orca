# TASK-AG-002.4: Add agentOrch tracing tests to agent-rpc-dispatch.test.ts and agent-spawner.test.ts

**Phase:** 1
**SOL Ref:** [SOL-AG-TRACE-002](../solutions/SOL-AG-TRACE-002-agent-orchestration.md)
**CR Ref:** [CR-TRACE-002](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-002-agent-orchestration.md)
**Precondition:** Phase 0 + [TASK-AG-002.2](./TASK-AG-002.2-agent-rpc-dispatch-resume-and-exec-span.md) + [TASK-AG-002.3](./TASK-AG-002.3-agent-spawner-orchestration-spans.md)
**Estimated time:** 1.5h
**Status:** ✅ Done (2026-08-03) — added a `vi.mock('node-pty', ...)` fake-PTY harness to `agent-spawner.test.ts` (didn't exist there before; mirrors the one in `pty-agent-bridge.test.ts`) plus a `SPAWNABLE_CONFIG` (empty `toolPath`) to bypass the pre-spawn binary-exists filesystem check that the file's existing tests deliberately stop short of. 28/28 pass in `agent-rpc-dispatch.test.ts` (23 pre-existing + 5 new), 54/54 pass in `agent-spawner.test.ts` (45 pre-existing + 9 new — one extra beyond the 8 specced, for pty-exit ok()).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

Task này viết test cho các symbol đã tồn tại (vừa được TASK-AG-002.2/002.3 thêm tracer) — chạy `codegraph explore` để hiểu implementation thật trước khi viết assertion:

```bash
codegraph explore "createRpcDispatcher"
codegraph explore "handleAgentSpawn"
codegraph explore "handleAgentKill"
codegraph explore "handleAgentSendInput"
```

Đây đều là symbol MODIFY (đã tồn tại) — chạy thêm impact analysis:

```
gitnexus_impact({ target: "createRpcDispatcher", direction: "upstream" })
gitnexus_impact({ target: "handleAgentSpawn", direction: "upstream" })
gitnexus_impact({ target: "handleAgentKill", direction: "upstream" })
gitnexus_impact({ target: "handleAgentSendInput", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## File: `src/relay/__tests__/agent-rpc-dispatch.test.ts` [MODIFY]

```typescript
// src/relay/__tests__/agent-rpc-dispatch.test.ts (thêm)
describe('dispatch() — trace resume', () => {
  it('resumes agent:rpc span id from params._trace.id', async () => { /* ... */ })
  it('generates a new span id when params._trace is absent', async () => { /* ... */ })
})

describe("case 'agent.exec' — agentOrch:spawn", () => {
  it('emits agentOrch:spawn span with ok() containing exitCode on success', async () => { /* ... */ })
  it('emits fail() when binary is missing', async () => { /* ... */ })
  it('emits fail() with timeout field when subprocess times out', async () => { /* ... */ })
})
```

## File: `src/relay/__tests__/agent-spawner.test.ts` [MODIFY]

```typescript
// src/relay/__tests__/agent-spawner.test.ts (thêm)
import { registerTraceSink, type TraceEvent } from '../../shared/trace'

describe('handleAgentSpawn — agentOrch tracing', () => {
  it('emits agentOrch:spawn when resumeId is absent (BL-AG-01)', async () => { /* ... */ })
  it('emits agentOrch:resume instead of agentOrch:spawn when resumeId is present (BL-AG-03)', async () => { /* ... */ })
  it('does not emit a new span per pty.onData frame (BL-AG-05) — only one "first-output" step', async () => {
    // simulate pty.onData firing 50 times → assert only 1 step event with label 'first-output'
  })
  it('resumes span id from params._trace.id', async () => { /* ... */ })
})

describe('handleAgentKill — agentOrch:stop', () => {
  it('emits agentOrch:stop span with ok() when pty found and killed', async () => { /* ... */ })
  it('emits ok() with note=already dead when ptyId not in registry', async () => { /* ... */ })
})

describe('handleAgentSendInput — agentOrch:stop (Ctrl+C only)', () => {
  it('emits agentOrch:stop span when data === "\\x03"', async () => { /* ... */ })
  it('does NOT emit any span for arbitrary interactive keystrokes', async () => {
    // assert zero TraceEvent with flow='agentOrch:stop' when data='a'
  })
})
```

## Verification

```bash
pnpm vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts src/relay/__tests__/agent-spawner.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] `agent-rpc-dispatch.test.ts` có đủ 5 test case ở mục "dispatch() — trace resume" và "case 'agent.exec' — agentOrch:spawn"
- [ ] `agent-spawner.test.ts` có đủ 8 test case ở 3 describe block trên
- [ ] Test "does not emit a new span per pty.onData frame" giả lập ≥ 50 lần gọi `pty.onData` callback và đếm chính xác số event `step(label='first-output')` bằng 1
- [ ] Test "does NOT emit any span for arbitrary interactive keystrokes" đếm `events.filter(e => e.flow === 'agentOrch:stop').length === 0` khi `data` khác `'\x03'`
- [ ] `pnpm vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts src/relay/__tests__/agent-spawner.test.ts` pass toàn bộ
