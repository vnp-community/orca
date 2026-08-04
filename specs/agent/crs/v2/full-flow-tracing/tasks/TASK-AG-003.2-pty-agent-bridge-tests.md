# TASK-AG-003.2: Create pty-agent-bridge.test.ts

**Phase:** 1
**SOL Ref:** [SOL-AG-TRACE-003](../solutions/SOL-AG-TRACE-003-terminal-management.md)
**CR Ref:** [CR-TRACE-003](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-003-terminal-management.md)
**Precondition:** Phase 0 + [TASK-AG-003.1](./TASK-AG-003.1-pty-agent-bridge-terminal-tracing.md)
**Estimated time:** 1.5h
**Status:** ✅ Done (2026-08-03) — file location/approach differs from the spec below: a sibling test file `src/relay/pty-agent-bridge.test.ts` (not `__tests__/pty-agent-bridge.test.ts`) already existed, created concurrently by the PTY-daemon reattach work in progress, with a reusable `makeFakePty()`/`spawnMock` mock harness. Extended that SAME file with a new `describe('pty-agent-bridge — terminal:* tracing (CR-TRACE-003)', ...)` block (11 tests) rather than creating a duplicate file/mock setup. Dropped the "fail() on node-pty import failure" case from the original spec — the file's top-level `vi.mock('node-pty', ...)` is a static/hoisted mock that always succeeds; per-test override via `vi.doMock`+`resetModules()` would also reset the `shared/trace` module instance and silently break `registerTraceSink` capture, so it was deemed not worth the fragility for one edge case. 20/20 tests pass (11 new + 9 pre-existing reattach tests, unaffected).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

File test là NEW, nhưng nó test các handler đã tồn tại (vừa được TASK-AG-003.1 thêm tracer) — chạy `codegraph explore` để hiểu implementation thật trước khi viết assertion:

```bash
codegraph explore "handlePtyCreate"
codegraph explore "handlePtyResize"
codegraph explore "handlePtyDestroy"
codegraph explore "handlePtySendSignal"
codegraph explore "handlePtyWrite"
codegraph explore "handlePtyScrollback"
```

Đây đều là symbol MODIFY/pre-existing (không phải symbol NEW) — chạy thêm impact analysis:

```
gitnexus_impact({ target: "handlePtyCreate", direction: "upstream" })
gitnexus_impact({ target: "handlePtyResize", direction: "upstream" })
gitnexus_impact({ target: "handlePtyDestroy", direction: "upstream" })
gitnexus_impact({ target: "handlePtySendSignal", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## File: `src/relay/__tests__/pty-agent-bridge.test.ts` [NEW]

Chưa có file test cho `pty-agent-bridge.ts` trong `src/relay/__tests__/` — tạo mới theo pattern isomorphic sink giống TASK-AG-001.2/002.4.

```typescript
// src/relay/__tests__/pty-agent-bridge.test.ts (NEW)
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { registerTraceSink, type TraceEvent } from '../../shared/trace'
import {
  handlePtyCreate, handlePtyResize, handlePtyDestroy,
  handlePtySendSignal, handlePtyWrite, handlePtyScrollback,
} from '../pty-agent-bridge'

describe('pty-agent-bridge — terminal:* tracing', () => {
  let events: TraceEvent[]
  let unregister: () => void

  beforeEach(() => {
    events = []
    unregister = registerTraceSink((e) => events.push(e))
  })
  afterEach(() => unregister())

  it('handlePtyCreate emits terminal:create span, ok() contains ptyId+shell+cwd', async () => { /* mock node-pty */ })
  it('handlePtyCreate emits fail() when node-pty import fails', async () => { /* mock import failure */ })
  it('handlePtyResize emits terminal:resize span with cols/rows', async () => { /* ... */ })
  it('handlePtyDestroy emits terminal:destroy span with graceful field', async () => { /* ... */ })
  it('handlePtyDestroy emits ok(alreadyDead=true) span when ptyId not registered', async () => { /* ... */ })

  it('handlePtySendSignal emits terminal:destroy span for SIGKILL', async () => { /* ... */ })
  it('handlePtySendSignal emits terminal:destroy span for SIGTERM', async () => { /* ... */ })
  it('handlePtySendSignal does NOT emit any span for SIGINT/SIGHUP/SIGTSTP', async () => {
    // assert events.filter(e => e.flow === 'terminal:destroy').length === 0
  })

  it('handlePtyWrite does NOT emit any trace span regardless of call count', async () => {
    for (let i = 0; i < 20; i++) await handlePtyWrite(i, { id: 'x', data: 'a' }, log)
    expect(events.filter(e => e.flow.startsWith('terminal:'))).toHaveLength(0)
  })

  it('handlePtyScrollback does NOT emit any trace span', async () => {
    await handlePtyScrollback(1, { id: 'x' }, log)
    expect(events.filter(e => e.flow.startsWith('terminal:'))).toHaveLength(0)
  })

  it('resumes span id from params._trace.id when present', async () => {
    // handlePtyResize with _trace: { id: 'resumed-1' } → assert start event id === 'resumed-1'
  })

  it('generates a new span id when params._trace is absent', async () => { /* ... */ })
})
```

## Verification

```bash
pnpm vitest run src/relay/__tests__/pty-agent-bridge.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] File mới `src/relay/__tests__/pty-agent-bridge.test.ts` tồn tại với tất cả 12 test case trên
- [ ] Mock `node-pty` được thiết lập đúng cách (module external, cần `vi.mock`/dynamic import mock) — không phụ thuộc binary thật có mặt trên máy CI
- [ ] Test đếm span count xác nhận `handlePtyWrite`/`handlePtyScrollback` không phát bất kỳ event nào (theo nguyên tắc CR-TRACE-000 §5)
- [ ] Test `handlePtySendSignal` phân biệt rõ `SIGKILL`/`SIGTERM` (có span) vs `SIGINT`/`SIGHUP`/`SIGTSTP` (không span)
- [ ] `pnpm vitest run src/relay/__tests__/pty-agent-bridge.test.ts` pass
