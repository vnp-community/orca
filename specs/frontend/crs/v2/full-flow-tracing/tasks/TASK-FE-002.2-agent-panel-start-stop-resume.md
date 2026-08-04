# TASK-FE-002.2: Instrument `AgentPanel.tsx` start/stop/resume + mở rộng type IPC

**Phase:** 1
**SOL Ref:** [SOL-FE-TRACE-002 §2.3, §2.5](../solutions/SOL-FE-TRACE-002-agent-orchestration.md)
**CR Ref:** [CR-TRACE-002](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-002-agent-orchestration.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001) + TASK-FE-002.1 (dùng tracer + registry vừa tạo)
**Status:** ✅ Done (2026-08-03) — Instrumented start/stop/resume in `AgentPanel.tsx` using `Tracers.uiAgentOrchSpawnFlow/StopFlow/ResumeFlow` (see TASK-FE-002.1 note — these are new `ui:` entries distinct from agent-domain's `agentOrchSpawn/Stop/Resume`); extended `preload/index.ts` `agentOrchestration.start/stop/resume` opts with optional `traceId`; confirmed AgentPanel.tsx remains unmounted (orphan, gitnexus_impact LOW/0 callers) — noted per task instructions. Added new `AgentPanel.test.tsx` in `components/workspace/__tests__/` (matches sibling convention) with 8 tracing tests covering start/ok/fail, already-running immediate ok, stop immediate ok/fail, resume open-on-success/fail-on-resumed-false/fail-on-reject, traceId forwarding, and no-secret-fields assertion. `pnpm tsc --noEmit` clean, 8/8 tests pass.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "AgentPanel"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "AgentPanel", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

**Lưu ý orphan component:** `AgentPanel.tsx` hiện KHÔNG được mount/render ở đâu trong app (xem mục Mô tả) — `gitnexus_impact` trên symbol này nhiều khả năng trả về risk LOW hoặc không có caller thực nào. Đây là kết quả ĐÚNG NHƯ MONG ĐỢI, không phải dấu hiệu sai sót — không cần điều tra thêm vì lý do này.

## Mô tả

`AgentPanel.tsx` (component "Remote agent start/stop/resume control panel", **chưa mount** — xem TASK-FE-002.1) là entry point THẬT cho BL-AG-01 (start), BL-AG-02 (stop), BL-AG-03 (resume). `startAgent()`/`resumeAgent()` mở span nhưng KHÔNG `ok()` ngay — span ở lại "mở" trong registry (TASK-FE-002.1) chờ `statusChanged` xác nhận `'running'` (xử lý ở TASK-FE-002.3). `stopAgent()` là request/response đơn giản, đóng span ngay khi resolve.

## File: `src/renderer/src/components/workspace/AgentPanel.tsx` [MODIFY]

```typescript
import { Tracers } from '../../../../shared/trace/tracers'
import { registerOpenAgentOrchSpan, takeOpenAgentOrchSpan } from '@/lib/agent-orchestration-active-spans'

const startAgent = useCallback(async () => {
  setIsActing(true)
  updateAgentStatus({ worktreeId, status: 'starting' })
  // Why: span ở lại "mở" (không ok() ngay) — chờ statusChanged xác nhận 'running'
  // qua registry, khớp nguyên tắc BL-AG-05 của CR-TRACE-002 §4.
  const span = Tracers.agentOrchSpawn.start({ worktreeId, agentType, trustPreset })
  registerOpenAgentOrchSpan(worktreeId, span)
  try {
    const result = await window.api.agentOrchestration.start({ worktreeId, agentType, trustPreset, traceId: span.id })
    span.step('ipc-invoke-resolved', { sessionId: result.sessionId, status: result.status })
    setRemoteAgentSession(worktreeId, {
      sessionId: result.sessionId, worktreeId, agentType, trustPreset,
      status: result.status === 'already-running' ? 'running' : 'starting', startedAt: Date.now(),
    })
    if (result.status === 'already-running') {
      toast.info('Agent is already running')
      // Why: 'already-running' là trạng thái cuối — không còn statusChanged nào sẽ
      // tới để đóng span, phải ok() ngay tại đây.
      takeOpenAgentOrchSpan(worktreeId)
      span.ok({ sessionId: result.sessionId, status: result.status })
    }
    // Nếu result.status === 'started': span vẫn mở, chờ statusChanged 'running'|'error'
    // đóng nó trong useAgentOrchestrationEvents (TASK-FE-002.3).
  } catch (err: any) {
    takeOpenAgentOrchSpan(worktreeId)
    span.fail(err, { worktreeId, agentType })
    updateAgentStatus({ worktreeId, status: 'error', errorMessage: err.message })
    toast.error(`Failed to start agent: ${err.message}`)
  } finally {
    setIsActing(false)
  }
}, [worktreeId, agentType, trustPreset, updateAgentStatus, setRemoteAgentSession])

const stopAgent = useCallback(async () => {
  if (!session?.sessionId) return
  setIsActing(true)
  // Why: stop() là request/response đơn giản — không cần chờ statusChanged để đóng
  // span (khác spawn/resume, vốn "starting" là trạng thái trung gian).
  const span = Tracers.agentOrchStop.start({ worktreeId, sessionId: session.sessionId })
  try {
    await window.api.agentOrchestration.stop({ sessionId: session.sessionId, traceId: span.id })
    updateAgentStatus({ worktreeId, status: 'stopped' })
    span.ok({ worktreeId, sessionId: session.sessionId })
  } catch (err: any) {
    span.fail(err, { worktreeId, sessionId: session.sessionId })
    toast.error(`Failed to stop agent: ${err.message}`)
  } finally {
    setIsActing(false)
  }
}, [session, worktreeId, updateAgentStatus])

const resumeAgent = useCallback(async () => {
  if (!session?.sessionId) return
  setIsActing(true)
  updateAgentStatus({ worktreeId, status: 'starting' })
  const span = Tracers.agentOrchResume.start({ worktreeId, sessionId: session.sessionId })
  registerOpenAgentOrchSpan(worktreeId, span)
  try {
    const result = await window.api.agentOrchestration.resume({ sessionId: session.sessionId, traceId: span.id })
    if (!result.resumed) {
      takeOpenAgentOrchSpan(worktreeId)
      span.fail(new Error('resume returned resumed:false'), { worktreeId, sessionId: session.sessionId })
      toast.error('Could not resume agent session')
      updateAgentStatus({ worktreeId, status: 'stopped' })
      return
    }
    span.step('ipc-invoke-resolved', { sessionId: session.sessionId })
    // Span vẫn mở — statusChanged 'running' sẽ đóng nó (TASK-FE-002.3).
  } catch (err: any) {
    takeOpenAgentOrchSpan(worktreeId)
    span.fail(err, { worktreeId, sessionId: session.sessionId })
    updateAgentStatus({ worktreeId, status: 'error', errorMessage: err.message })
    toast.error(`Failed to resume agent: ${err.message}`)
  } finally {
    setIsActing(false)
  }
}, [session, worktreeId, updateAgentStatus])
```

## File: `src/preload/index.ts` [MODIFY, chỉ mở rộng type]

```typescript
// src/preload/index.ts — chỉ mở rộng type của opts, KHÔNG đổi hành vi hiện có
agentOrchestration: {
  start: (opts: {
    worktreeId: string
    agentType: 'claude' | 'codex' | 'custom'
    trustPreset?: 'standard' | 'permissive' | 'strict'
    traceId?: string   // NEW — optional, backward compatible
  }): Promise<{ sessionId: string; status: 'started' | 'already-running' }> =>
    ipcRenderer.invoke('agentOrchestration:start', opts),

  stop: (opts: { sessionId: string; traceId?: string }): Promise<void> =>
    ipcRenderer.invoke('agentOrchestration:stop', opts),

  resume: (opts: { sessionId: string; traceId?: string }): Promise<{ resumed: boolean }> =>
    ipcRenderer.invoke('agentOrchestration:resume', opts),

  // ...onStatusChanged unchanged...
}
```

> `preload/index.ts` kỹ thuật không nằm trong `src/renderer/src/`, nhưng đây là type signature phía renderer gọi tới — thay đổi tối thiểu, chỉ thêm field optional. Việc *đọc* `opts.traceId` ở `src/main/ipc/agent-orchestration.ts` thuộc phạm vi backend CR, không phải task này.

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/components/workspace/__tests__/AgentPanel.test.tsx
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `startAgent()`/`stopAgent()`/`resumeAgent()` mỗi hàm mở đúng 1 span tương ứng, đính `traceId: span.id` vào `opts` truyền cho `window.api.agentOrchestration.*`
- [ ] `startAgent()`/`resumeAgent()` KHÔNG gọi `span.ok()` ngay khi IPC invoke resolve với status `'started'`/`resumed:true` — span ở lại mở trong registry chờ `statusChanged`
- [ ] `startAgent()` với `result.status === 'already-running'` → `span.ok()` ngay, registry được clear
- [ ] `resumeAgent()` với `result.resumed === false` → `span.fail()`, KHÔNG `span.ok()`
- [ ] `stopAgent()` gọi `Tracers.agentOrchStop.start()` rồi `ok()` ngay khi invoke resolve (không dùng registry)
- [ ] Lỗi IPC reject ở cả 3 hàm → `span.fail(err, ...)`, registry được clear (`takeOpenAgentOrchSpan` trả `undefined` sau đó)
- [ ] `window.api.agentOrchestration.start/stop/resume` type nhận thêm field optional `traceId` — backward compatible, không đổi hành vi hiện có
- [ ] Trường `env`/credential KHÔNG xuất hiện trong field của bất kỳ span nào — chỉ `agentType`/`trustPreset`/`worktreeId`/`sessionId`
- [ ] Báo cáo rõ trong PR/commit rằng `AgentPanel.tsx` hiện **chưa được mount** vào App shell (xem TASK-FE-002.1) — instrumentation chỉ có giá trị thực khi gap mount đó được vá
- [ ] Test suite đạt ≥ 8 test case mới trong `AgentPanel.test.tsx` theo Test Plan của SOL-FE-TRACE-002 §3
