# SOL-FE-TRACE-002: Agent Orchestration — Frontend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-002](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-002-agent-orchestration.md)
**TDD Ref:** TDD-FE-07 (Custom Hooks & IPC Events)
**Status:** Proposed
**Dependency:** F40 core tracing infra (đã implement) — `src/shared/trace/browser.ts`, `src/shared/trace/tracers.ts`, TracePanel. CR-TRACE-000 (naming convention, `resume` param, quy ước `traceId`).

---

## 1. Điểm khởi tạo trace trong Renderer

### 1.1 Phát hiện quan trọng: renderer có một đường Agent Orchestration THẬT mà CR-TRACE-002 chưa ghi nhận

CR-TRACE-002 §1 kết luận: *"Không tìm thấy call site thật nào gọi `relay.call('agent.kill'|'agent.sendInput'|'agent.status')` từ phía Orca Server"* cho BL-AG-02/03/04, và toàn bộ phân tích tập trung vào `ProfileAwareAgentSpawner.spawn()` (kích hoạt qua đường `relay.call('agent.exec', ...)`, dùng khi launch agent bằng cách chạy PTY với lệnh CLI của agent — xem `buildAgentStartupPlan()`/`startup` param trong `createWorktree()`, SOL-FE-TRACE-001 §2.2).

Khi grep renderer cho `agentOrchestration`, phát hiện một component **độc lập, đầy đủ chức năng, có sẵn cả 3 action start/stop/resume**, gắn nhãn `BUG-FE-ORCH-001`:

- `src/renderer/src/components/workspace/AgentPanel.tsx` — UI component "Remote agent start/stop/resume control panel"
- `src/renderer/src/hooks/use-agent-orchestration-events.ts` — hook subscribe push event `agentOrchestration:statusChanged`
- `src/renderer/src/store/slices/remote-agent-sessions.ts` — Zustand slice `remoteAgentSessions`
- `src/preload/index.ts:4507-4532` — contextBridge, `window.api.agentOrchestration.{start,stop,resume,onStatusChanged}`
- `src/main/ipc/agent-orchestration.ts` — `ipcMain.handle('agentOrchestration:start'|'stop'|'resume', ...)`, forward vào `runtime.startAgent()`

Đây là **một transport hoàn toàn khác** với những gì CR-TRACE-002 mô tả:

| | CR-TRACE-002 mô tả | Thực tế tìm thấy (`AgentPanel.tsx`) |
|---|---|---|
| Transport | `relay.call('agent.exec')` qua `DevServerRelayBridge` (multi-tenant Dev Server) | Electron `contextBridge` IPC (`ipcRenderer.invoke('agentOrchestration:start', ...)`) — **không nằm trong 6 hàng transport của CR-TRACE-000 §3.3** |
| Đơn vị launch | PTY chạy lệnh CLI của agent (`command`/`launchAgent` trong `terminal.create`/`worktree.create`) | Session trừu tượng có `sessionId` riêng, không gắn trực tiếp với một PTY cụ thể trong renderer state |
| BL-AG-02/03 caller | Không tìm thấy | **Có** — `AgentPanel.tsx`'s `stopAgent()`/`resumeAgent()` |

**Kết luận:** đây là dấu hiệu doc/code drift kiểu CR-TRACE-000 §8 đã cảnh báo — có thể `AgentPanel.tsx` (`BUG-FE-ORCH-001`) được thêm sau khi CR-TRACE-002 khảo sát code, hoặc CR-TRACE-002 chỉ tìm theo pattern `relay.call('agent.*')` nên bỏ sót nhánh Electron IPC riêng này. CR này (SOL-FE-TRACE-002) coi `AgentPanel.tsx` là entry point THẬT cho BL-AG-01/02/03 ở renderer, bổ sung cho (không thay thế) phân tích BL-AG-01 gốc của CR-TRACE-002 về `ProfileAwareAgentSpawner`.

**Gap cần cảnh báo:** `AgentPanel.tsx` **chưa được mount ở bất kỳ đâu trong App shell** — đã grep toàn bộ `src/renderer/src` cho import `workspace/AgentPanel` hoặc `from '.../AgentPanel'`, chỉ có chính file này tham chiếu tới nó. Component + IPC bridge là code thật, hoạt động được (có thể unit test độc lập), nhưng **không reachable qua UI hiện tại** — không xuất hiện trong `App.tsx`, `WorkspaceLayout`, hay right-sidebar nào. Instrumentation dưới đây vẫn có giá trị (future-proof cho khi component được mount), nhưng acceptance criteria không thể bao gồm "verify qua click thật trong app" cho tới khi gap mount này được vá (ngoài phạm vi CR tracing).

### 1.2 BL-AG-01 — Khởi động Agent (đường `AgentPanel`)

`startAgent()` — `useCallback` tại `src/renderer/src/components/workspace/AgentPanel.tsx:74-101`, kích hoạt bởi nút "Start Agent" (dòng 195-203). Gọi `window.api.agentOrchestration.start({ worktreeId, agentType, trustPreset })`.

### 1.3 BL-AG-02 — Dừng Agent

`stopAgent()` — `AgentPanel.tsx:103-114`, nút "Stop" (dòng 230-244). Gọi `window.api.agentOrchestration.stop({ sessionId })`.

### 1.4 BL-AG-03 — Resume Agent Session

`resumeAgent()` — `AgentPanel.tsx:116-132`, nút "Resume" (dòng 206-217, chỉ hiện khi `canResume = status === 'stopped' && !!session?.sessionId`). Gọi `window.api.agentOrchestration.resume({ sessionId })`.

### 1.5 BL-AG-04 — Switch Account/Provider: xác nhận lại — không có UI

`AgentPanel.tsx` cho phép chọn `agentType`/`trustPreset` **trước khi start** (Select dropdown, dòng 160-190), nhưng không có action nào "switch" một session đang chạy sang account/provider khác. Đã grep `switchAccount`, `switchProvider`, `rateLimited` trong `components/workspace` và `lib/` — không có kết quả liên quan. Khớp với CR-TRACE-002 §4 BL-AG-04: chưa có implementation.

### 1.6 BL-AG-05 — Monitor Trạng thái Real-time: cơ chế thật đơn giản hơn CR-TRACE-002 dự đoán

CR-TRACE-002 §4 BL-AG-05 giả định một luồng PTY output tần suất cao cần OSC parsing (`AgentHookParser` không tồn tại). Với đường `AgentPanel`, monitor thật là **push event rời rạc** — không phải stream byte-level:

```typescript
// src/renderer/src/hooks/use-agent-orchestration-events.ts:15-29
export function useAgentOrchestrationEvents(): void {
  const updateAgentStatus = useAppStore(s => s.updateAgentStatus)
  useEffect(() => {
    const unsubscribe = window.api.agentOrchestration.onStatusChanged(event => {
      updateAgentStatus({ worktreeId: event.worktreeId, sessionId: event.sessionId, status: event.status, errorMessage: event.errorMessage })
    })
    return unsubscribe
  }, [updateAgentStatus])
}
```

`status` chỉ nhận 4 giá trị (`starting|running|stopped|error`) — đúng loại "state-transition quan trọng" mà CR-TRACE-002 §4 BL-AG-05 khuyến nghị `step()` trên span BL-AG-01/03 **nếu span đó còn sống** (không tạo tracer riêng). Mục 2.4 dưới đây implement chính xác cơ chế "attach vào span còn mở" này bằng một registry nhỏ theo `worktreeId`.

---

## 2. Full Implementation

### 2.1 Tracer mới trong `tracers.ts`

```typescript
// src/shared/trace/tracers.ts
export const Tracers = {
  // ...existing entries unchanged...
  agentOrchSpawn:      createTracer('ui:agentOrch.spawn'),      // BL-AG-01
  agentOrchStop:       createTracer('ui:agentOrch.stop'),       // BL-AG-02
  agentOrchResume:     createTracer('ui:agentOrch.resume'),     // BL-AG-03
  agentOrchSwitch:     createTracer('ui:agentOrch.switch'),     // BL-AG-04 — chưa có UI, đặt tên sẵn
  agentOrchStatusPoll: createTracer('ui:agentOrch.statusPoll'), // BL-AG-05 — dự phòng, xem mục 2.4 (không dùng làm span riêng)
} as const
```

Prefix `ui:` bắt buộc theo quyết định chung (xem `00-index.md` mục 1) — khác `agentOrch:spawn|stop|resume|switch|statusPoll` phía CR-TRACE-002 §3 (backend), để `isBackend` heuristic của `TracePanel.tsx:42` không gắn nhầm badge "▲ srv" cho event do browser tự phát; vẫn liên kết được với span backend qua `traceId`/`resume` khi companion CR áp dụng.

### 2.2 Registry span đang mở, dùng chung giữa `AgentPanel` và `useAgentOrchestrationEvents`

File mới, đặt tên theo đúng nội dung nó chứa (không dùng tên chung chung như `agent-utils.ts`):

```typescript
// src/renderer/src/lib/agent-orchestration-active-spans.ts
// Why: startAgent()/resumeAgent() mở span ui:agentOrch.spawn|resume nhưng kết quả
// "running" thật chỉ tới sau, qua push event agentOrchestration:statusChanged
// (IPC, không phải response của lệnh start/resume). Registry này cho phép hook
// nhận event gắn step()/ok()/fail() vào đúng span đang mở theo BL-AG-05
// (CR-TRACE-002 §4: "step() trên span đang mở của BL-AG-01/03 nếu còn sống").
import type { TraceSpan } from '../../../../shared/trace'

const openSpansByWorktreeId = new Map<string, TraceSpan>()

export function registerOpenAgentOrchSpan(worktreeId: string, span: TraceSpan): void {
  openSpansByWorktreeId.set(worktreeId, span)
}

/** Lấy và xoá span đang mở cho worktree này (nếu có) — dùng khi đóng span. */
export function takeOpenAgentOrchSpan(worktreeId: string): TraceSpan | undefined {
  const span = openSpansByWorktreeId.get(worktreeId)
  openSpansByWorktreeId.delete(worktreeId)
  return span
}

/** Xem span đang mở mà không xoá — dùng cho step() giữa chừng (status vẫn 'starting'). */
export function peekOpenAgentOrchSpan(worktreeId: string): TraceSpan | undefined {
  return openSpansByWorktreeId.get(worktreeId)
}
```

### 2.3 `AgentPanel.tsx` — start/stop/resume

```typescript
// src/renderer/src/components/workspace/AgentPanel.tsx
import { Tracers } from '../../../../shared/trace/tracers'
import { registerOpenAgentOrchSpan, takeOpenAgentOrchSpan } from '@/lib/agent-orchestration-active-spans'

// ...

const startAgent = useCallback(async () => {
  setIsActing(true)
  updateAgentStatus({ worktreeId, status: 'starting' })
  // Why: span ở lại "mở" (không ok() ngay) — chờ statusChanged xác nhận 'running'
  // qua registry (mục 2.2), khớp nguyên tắc BL-AG-05 của CR-TRACE-002 §4.
  const span = Tracers.agentOrchSpawn.start({ worktreeId, agentType, trustPreset })
  registerOpenAgentOrchSpan(worktreeId, span)
  try {
    const result = await window.api.agentOrchestration.start({
      worktreeId,
      agentType,
      trustPreset,
      traceId: span.id
    })
    span.step('ipc-invoke-resolved', { sessionId: result.sessionId, status: result.status })
    setRemoteAgentSession(worktreeId, {
      sessionId: result.sessionId,
      worktreeId,
      agentType,
      trustPreset,
      status: result.status === 'already-running' ? 'running' : 'starting',
      startedAt: Date.now(),
    })
    if (result.status === 'already-running') {
      toast.info('Agent is already running')
      // Why: 'already-running' là trạng thái cuối — không còn statusChanged nào
      // sẽ tới để đóng span, phải ok() ngay tại đây.
      takeOpenAgentOrchSpan(worktreeId)
      span.ok({ sessionId: result.sessionId, status: result.status })
    }
    // Nếu result.status === 'started': span vẫn mở, chờ statusChanged 'running'|'error'
    // đóng nó trong useAgentOrchestrationEvents (mục 2.4).
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
  // Why: stop() là request/response đơn giản — không cần chờ statusChanged để
  // đóng span (khác spawn/resume, vốn "starting" là trạng thái trung gian).
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
    const result = await window.api.agentOrchestration.resume({
      sessionId: session.sessionId,
      traceId: span.id
    })
    if (!result.resumed) {
      takeOpenAgentOrchSpan(worktreeId)
      span.fail(new Error('resume returned resumed:false'), { worktreeId, sessionId: session.sessionId })
      toast.error('Could not resume agent session')
      updateAgentStatus({ worktreeId, status: 'stopped' })
      return
    }
    span.step('ipc-invoke-resolved', { sessionId: session.sessionId })
    // Span vẫn mở — statusChanged 'running' sẽ đóng nó (mục 2.4).
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

### 2.4 `useAgentOrchestrationEvents` — đóng span đang mở khi statusChanged tới

```typescript
// src/renderer/src/hooks/use-agent-orchestration-events.ts
import { useEffect } from 'react'
import { useAppStore } from '../store'
import { peekOpenAgentOrchSpan, takeOpenAgentOrchSpan } from '@/lib/agent-orchestration-active-spans'

export function useAgentOrchestrationEvents(): void {
  const updateAgentStatus = useAppStore(s => s.updateAgentStatus)

  useEffect(() => {
    const unsubscribe = window.api.agentOrchestration.onStatusChanged(event => {
      updateAgentStatus({
        worktreeId: event.worktreeId,
        sessionId: event.sessionId,
        status: event.status,
        errorMessage: event.errorMessage,
      })

      // BL-AG-05 (CR-TRACE-002 §4): không tạo span riêng cho mỗi statusChanged —
      // chỉ step()/đóng span của BL-AG-01/03 NẾU nó còn mở cho worktree này.
      if (event.status === 'running') {
        const span = takeOpenAgentOrchSpan(event.worktreeId)
        span?.ok({ worktreeId: event.worktreeId, sessionId: event.sessionId ?? '', status: event.status })
      } else if (event.status === 'error') {
        const span = takeOpenAgentOrchSpan(event.worktreeId)
        span?.fail(new Error(event.errorMessage ?? 'agent error'), { worktreeId: event.worktreeId })
      } else if (event.status === 'starting') {
        // Trung gian — vẫn còn ui:agentOrch.spawn/resume đang chạy, chỉ log step().
        peekOpenAgentOrchSpan(event.worktreeId)?.step('statusChanged', { status: event.status })
      }
      // 'stopped' không đụng vào registry: stopAgent() đã tự đóng span ui:agentOrch.stop
      // của chính nó (mục 2.3) — statusChanged 'stopped' ở đây chỉ đồng bộ store.
    })
    return unsubscribe
  }, [updateAgentStatus])
}
```

### 2.5 Mở rộng `window.api.agentOrchestration` type để nhận `traceId` (additive)

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

> `src/preload/index.ts` kỹ thuật không nằm trong `src/renderer/src/`/`src/platform/adapters/web/`, nhưng đây là type signature phía renderer gọi tới (import type dùng chung) — thay đổi tối thiểu, chỉ thêm field optional. Việc *đọc* `opts.traceId` ở `src/main/ipc/agent-orchestration.ts` (companion resume `Tracers.*.start(fields, { id: opts.traceId })`) thuộc phạm vi backend CR, không phải CR này.

---

## 3. Test Plan (Vitest)

```
src/renderer/src/components/workspace/__tests__/AgentPanel.test.tsx   (mới)
// @vitest-environment happy-dom
├── startAgent() gọi Tracers.agentOrchSpawn.start({ worktreeId, agentType, trustPreset }) trước khi invoke IPC
├── startAgent() truyền traceId: span.id vào window.api.agentOrchestration.start(...)
├── startAgent() với result.status === 'already-running' → span.ok() được gọi ngay (không chờ statusChanged)
├── startAgent() với result.status === 'started' → span KHÔNG ok()/fail() ngay, vẫn nằm trong registry (peekOpenAgentOrchSpan trả về span)
├── startAgent() lỗi (IPC reject) → span.fail(err, ...), registry được clear (takeOpenAgentOrchSpan trả undefined sau đó)
├── stopAgent() gọi Tracers.agentOrchStop.start() rồi ok() ngay khi invoke resolve (không dùng registry)
├── resumeAgent() với result.resumed === false → span.fail(), KHÔNG span.ok()
└── resumeAgent() thành công (resumed: true) → span vẫn mở chờ statusChanged

src/renderer/src/hooks/__tests__/use-agent-orchestration-events.test.ts   (mới)
├── statusChanged { status: 'running' } với span đang mở trong registry → span.ok() được gọi, registry rỗng sau đó
├── statusChanged { status: 'running' } KHÔNG có span mở (registry rỗng) → không throw, không gọi gì trên span (branch an toàn)
├── statusChanged { status: 'error', errorMessage } với span mở → span.fail(Error(errorMessage), ...)
├── statusChanged { status: 'starting' } → span.step('statusChanged', { status: 'starting' }), span KHÔNG bị xoá khỏi registry
└── statusChanged { status: 'stopped' } → không đụng registry (không gọi peek/take)

src/renderer/src/lib/__tests__/agent-orchestration-active-spans.test.ts   (mới)
├── registerOpenAgentOrchSpan() rồi takeOpenAgentOrchSpan() cùng worktreeId → trả về đúng span, lần gọi thứ 2 trả undefined
├── peekOpenAgentOrchSpan() không xoá span khỏi registry
└── 2 worktreeId khác nhau không đụng nhau (Map theo key riêng)
```

**Mock pattern:** `vi.mock('../../../../shared/trace/tracers')` với `Tracers.agentOrchSpawn.start` trả về một `TraceSpan` giả có `id`, `step`, `ok`, `fail` là `vi.fn()` — assert số lần gọi và tham số, theo đúng pattern `createMockClient` đã dùng cho `IRpcClient` trong TDD-FE-03 §"Test pattern cho IRpcClient mocks".

**Target:** ≥ 15 test case mới (8 cho AgentPanel, 5 cho hook, 3 cho registry — dư ra vì registry test dễ, nhẹ).

---

## 4. Acceptance Criteria

- [ ] `Tracers.agentOrchSpawn/Stop/Resume/Switch/StatusPoll` được thêm vào `src/shared/trace/tracers.ts` đúng tên `ui:agentOrch.spawn|stop|resume|switch|statusPoll`
- [ ] `AgentPanel.tsx`'s `startAgent()`/`stopAgent()`/`resumeAgent()` mỗi hàm mở đúng 1 span tương ứng, đính `traceId: span.id` vào `opts` truyền cho `window.api.agentOrchestration.*`
- [ ] `startAgent()`/`resumeAgent()` KHÔNG gọi `span.ok()` ngay khi IPC invoke resolve với status `'started'`/`resumed:true` — span ở lại mở trong registry chờ `statusChanged`
- [ ] `useAgentOrchestrationEvents()` đóng đúng span đang mở khi nhận `statusChanged` với `status: 'running'` (→ `ok()`) hoặc `status: 'error'` (→ `fail()`), và không throw khi không có span nào đang mở cho worktree đó
- [ ] KHÔNG có tracer/span riêng nào được tạo cho mỗi sự kiện `statusChanged` — chỉ `step()`/`ok()`/`fail()` trên span đã có sẵn trong registry (đúng nguyên tắc chống over-instrumentation CR-TRACE-000 §5 mà CR-TRACE-002 §4 BL-AG-05 áp dụng)
- [ ] Báo cáo rõ trong PR/commit rằng `AgentPanel.tsx` hiện **chưa được mount** vào App shell — instrumentation này chỉ có giá trị thực khi gap mount đó được vá (theo dõi riêng, ngoài phạm vi CR tracing)
- [ ] `ui:agentOrch.switch` (BL-AG-04) chỉ có tên tracer trong `tracers.ts`, không có call site code nào (chưa có UI switch-account)
- [ ] Trường `env`/credential KHÔNG xuất hiện trong field của bất kỳ span nào — `AgentPanel` chỉ có `agentType`/`trustPreset`/`worktreeId`/`sessionId`, không có gì nhạy cảm để lọc thêm
