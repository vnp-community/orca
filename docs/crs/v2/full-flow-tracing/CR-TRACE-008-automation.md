# CR-TRACE-008 — Automation Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-008 |
| **Tên** | Automation — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P2 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/automation.md`, `src/main/ipc/automations.ts`, `src/main/automations/service.ts`, `src/main/automations/AutomationEventBridge.ts`, `src/main/automations/WorktreeCleanupService.ts`, `src/main/dev-server/dev-server-relay-bridge.ts`, `src/cli/handlers/automations.ts` |

---

## 1. Vấn đề

`AutomationService` (`src/main/automations/service.ts`) chạy một `setInterval` tick 60s (`DEFAULT_TICK_MS`) gọi `evaluateDueRuns()` → `evaluateAutomation()` → `requestDispatch()`/`requestHeadlessDispatch()`. Khi một automation không chạy đúng lịch hoặc bị kẹt ở trạng thái `dispatching`, hiện tại chỉ có `console.log`/`console.error` rải rác — không có cách nào trả lời nhanh: automation có được `evaluateDueRuns()` nhặt đúng chu kỳ không? `resolveAutomationRunTarget()` có fail âm thầm không (route "unavailable")? Request có tới được renderer (`webContents.send`) hay rơi vào nhánh headless (`requestHeadlessDispatch`)?

Tương tự, `AutomationEventBridge` (`src/main/automations/AutomationEventBridge.ts`) subscribe `git.push`/`pr.created`/`worktree.created` trên một `EventEmitter` nội bộ và match automation theo `triggerType`/tên prefix `[eventType]` — nếu filter sai (project không khớp, hoặc automation bị disable), sự kiện bị bỏ qua **hoàn toàn im lặng** (chỉ log ra console, không throw). Không có tracing nghĩa là admin không thể biết event có đến `EventBus` hay không, có match automation nào không, hay bị lỗi khi dispatch.

Cuối cùng, `WorktreeCleanupService` (`src/main/automations/WorktreeCleanupService.ts`) chạy một chu kỳ dọn dẹp hàng giờ gọi `relay.call('worktree.list')`, `relay.call('git.exec', ...)` (safety check), rồi `relay.call('git.exec', ['worktree','remove'])` — mỗi lệnh này băng qua `DevServerRelayBridge` tới agent từ xa và có thể timeout/fail độc lập; hiện không có span nào cho biết worktree nào bị skip vì uncommitted changes vs bị lỗi network.

## 2. Thành phần & Transport liên quan

| Thành phần | Layer | Transport | Quy ước lan truyền (CR-TRACE-000 §3.3) |
|---|---|---|---|
| Renderer (React UI) | UI | IPC `automations:create` (`src/main/ipc/automations.ts:54-56`) | Sibling field cạnh method/params trong request envelope (giống WS RPC) |
| CLI Tool | Actor alt | Unix Socket, method `automation.create` (`src/cli/handlers/automations.ts:444`) | Giống convention WS RPC — CLI tự tạo `traceId` bằng `createTracer('cli:automationCreate').start()` trước khi gửi |
| `AutomationService` (`src/main/automations/service.ts`) | Business Logic | In-process (`setInterval` tick, không băng qua wire) | Không cần `traceId` từ ngoài — span tự sinh id mới khi tick bắt đầu chu kỳ |
| `AutomationEventBridge` (`AutomationEventBridge.ts`) | Business Logic | In-process `EventEmitter` (`EventBus`), trừ nhánh GitHub Webhook (`POST /webhook/github`, chưa xác định file cụ thể) | Webhook HTTP request nên mang `traceId` theo convention "WS RPC" nếu tồn tại route thật; nội bộ EventBus không cần |
| `WorktreeCleanupService` (`WorktreeCleanupService.ts`) → `DevServerRelayBridge.call()` (`src/main/dev-server/dev-server-relay-bridge.ts:530`) | Runtime → External | `relay.call()` (Orca ↔ Dev Server) | Field `traceId` trong params envelope của `.call()`, resume bằng `relayCallTracer` (`relay:agentCall`) ở phía `DevServerRelayBridge` |
| SQLite (`Store`, `src/main/persistence.ts`) | Persistence | In-process | N/A |
| Git Binary | External | Qua `relay.call('git.exec', ...)` — không có exec cục bộ trực tiếp trong `WorktreeCleanupService` | Cùng dòng `relay.call()` ở trên |

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  automationConfigureFlow: createTracer('automation:configure'),   // BL-AT-01
  automationScheduleRunFlow: createTracer('automation:scheduleRun'), // BL-AT-02
  automationEventTriggerFlow: createTracer('automation:eventTrigger'), // BL-AT-03
  automationCleanupFlow: createTracer('automation:cleanup'),        // BL-AT-04
}
```

*Ghi chú đặt tên:* assignment gợi ý ví dụ `automation:trigger`/`automation:eventDispatch` — CR này dùng `automation:eventTrigger` cho BL-AT-03 để tránh trùng nghĩa với `dispatchMatchingAutomations()` (một hàm nội bộ của cùng sub-flow, không phải sub-flow riêng), theo đúng nguyên tắc CR-TRACE-000 §4 "1 tracer = 1 sub-flow BL-XXX-NN".

## 4. Instrumentation theo từng sub-flow

### BL-AT-01 — Cấu hình Automation

| Bước | span event | fields | File:function |
|---|---|---|---|
| Renderer gọi tạo automation | `start` | `triggerType`, `hasSchedule` | `src/main/ipc/automations.ts:54-56` (`automations:create` handler) |
| Validate + INSERT | `step('persist')` (băng qua wire tới Main, và ghi DB — gộp vì cùng 1 lời gọi đồng bộ `store.createAutomation`) | `automationId` | `src/main/persistence.ts` (`Store.createAutomation`) |
| CLI variant | `start` (span riêng, không resume từ IPC) | `name`, `cron` | `src/cli/handlers/automations.ts:435-444` (`'automations create'` handler gọi `client.call('automation.create', ...)`) |

```typescript
// src/main/ipc/automations.ts
ipcMain.handle('automations:create', (_event, input: AutomationCreateInput): Automation => {
  const span = Tracers.automationConfigureFlow.start({ triggerType: input.triggerType })
  try {
    const automation = store.createAutomation(input)
    span.ok({ automationId: automation.id })
    return automation
  } catch (err) {
    span.fail(err, { triggerType: input.triggerType })
    throw err
  }
})
```

### BL-AT-02 — Chạy Automation theo Schedule

| Bước | span event | fields | File:function |
|---|---|---|---|
| Tick nhặt automation đến hạn | `start` (1 span/automation, không phải 1 span/tick — tick có thể match 0..N automations) | `automationId`, `scheduledFor` | `src/main/automations/service.ts` (`evaluateAutomation()`, dòng ~195) |
| Resolve target (local/ssh) | `step('resolveTarget')` — điểm rẽ nhánh quan trọng (ok/unavailable) | `executionTargetType` | `service.ts` (`resolveAutomationRunTarget`, gọi trong `requestDispatch()`) |
| Dispatch tới renderer hoặc headless dispatcher | `step('dispatch')` | `mode: 'renderer' \| 'headless'` | `service.ts` (`requestDispatch()`/`requestHeadlessDispatch()`, dòng 218-323) |
| Kết quả cuối (đồng bộ hoặc qua `markDispatchResult`) | `ok`/`fail` | `status` (`dispatched`/`skipped_unavailable`/`dispatch_failed`) | `service.ts` (`markDispatchResult()`) |

```typescript
// src/main/automations/service.ts — trong evaluateAutomation()
private async evaluateAutomation(automation: Automation, now: number): Promise<void> {
  const scheduledFor = this.store.getLatestAutomationOccurrence(automation, now)
  if (scheduledFor === null) {
    this.store.advanceAutomationNextRun(automation.id, now)
    return
  }
  const span = Tracers.automationScheduleRunFlow.start({ automationId: automation.id, scheduledFor })
  const run = this.store.createAutomationRun(automation, scheduledFor)
  const graceMs = automation.missedRunGraceMinutes * 60 * 1000
  if (now - scheduledFor > graceMs) {
    span.fail('missed run grace window exceeded', { runId: run.id })
    this.store.updateAutomationRun({ runId: run.id, status: 'skipped_missed', workspaceId: automation.workspaceId, error: '...' })
    this.store.advanceAutomationNextRun(automation.id, now)
    return
  }
  const updated = await this.requestDispatch(automation, run) // span.step('dispatch', ...) bên trong requestDispatch
  span.ok({ runId: run.id, status: updated.status })
  this.store.advanceAutomationNextRun(automation.id, now)
}
```

*Ghi chú:* `markDispatchResult()` (gọi bất đồng bộ sau khi `launch.completion` resolve, có thể vài phút sau khi span `ok()` đã đóng) không nên cố gắn vào cùng span — theo CR-TRACE-000 §3.1, mỗi layer đo latency cục bộ; hoàn tất run là một sự kiện muộn hơn nên ghi bằng field trong log domain (`orca_automation_runs.usage`), không phải kéo dài span tracing.

### BL-AT-03 — Event-based Automation Trigger

| Bước | span event | fields | File:function |
|---|---|---|---|
| Nhận sự kiện từ `EventBus` | `start` | `eventType` (`git.push`/`pr.created`/`worktree.created`) | `src/main/automations/AutomationEventBridge.ts` (`onGitPush`/`onPRCreated`/`onWorktreeCreated`, dòng 90-114) |
| Tìm automation khớp trigger | `step('matchAutomations')` — điểm rẽ nhánh quan trọng (0 match vs N match) | `projectId`, `matchedCount` | `AutomationEventBridge.ts` (`dispatchMatchingAutomations()`, dòng 122-141) |
| Dispatch từng automation khớp | `step('dispatch')` mỗi automation (có thể fail độc lập, không chặn các automation khác) | `automationId`, `automationName` | `AutomationEventBridge.ts` (dòng 143-154, gọi `this.automationService.dispatchAutomation(...)`) |
| GitHub Webhook variant | `start` riêng, resume nếu webhook mang `traceId` | `eventType: push \| pull_request` | **chưa xác định file cụ thể** — route `POST /webhook/github` không tìm thấy khi grep `src/`; cần điều tra thêm khi triển khai |

```typescript
// src/main/automations/AutomationEventBridge.ts — trong dispatchMatchingAutomations()
private async dispatchMatchingAutomations(
  triggerType: string,
  projectId: string,
  context: Record<string, unknown>
): Promise<void> {
  const span = Tracers.automationEventTriggerFlow.start({ eventType: triggerType, projectId })
  try {
    const matching = /* ... filter allAutomations ... */
    span.step('matchAutomations', { projectId, matchedCount: matching.length })
    for (const automation of matching) {
      try {
        await this.automationService.dispatchAutomation(automation.id, { trigger: 'manual', projectId, context })
        span.step('dispatch', { automationId: automation.id, automationName: automation.name })
      } catch (err) {
        span.fail(err, { automationId: automation.id })
      }
    }
    span.ok({ matchedCount: matching.length })
  } catch (err) {
    span.fail(err, { eventType: triggerType })
  }
}
```

*Cảnh báo phát hiện khi điều tra:* `dispatchMatchingAutomations()` gọi `this.automationService.dispatchAutomation(automation.id, ...)` (dòng 145), nhưng lớp `AutomationService` đọc được ở `src/main/automations/service.ts` **không có** public method tên `dispatchAutomation` (chỉ có `runNow`, `runPrecheck`, `markDispatchResult`). Đây có thể là dead code path hoặc method được mở rộng ở nơi khác chưa tìm thấy — team triển khai CR này nên xác minh trước khi thêm tracing vào đúng call site, nếu không span sẽ không bao giờ bắt được lỗi runtime `dispatchAutomation is not a function`.

### BL-AT-04 — Cleanup Worktrees Theo Policy

| Bước | span event | fields | File:function |
|---|---|---|---|
| Bắt đầu chu kỳ cleanup | `start` | `maxAgeMs`, `dryRun` | `src/main/automations/WorktreeCleanupService.ts` (`runCleanup()`, dòng 99) |
| Query worktree đủ điều kiện | `step('queryEligible')` (băng qua `relay.call('worktree.list')` — cross-boundary) | `eligibleCount` | `WorktreeCleanupService.ts` (`getEligibleWorktrees()`, dòng 148-158) |
| Safety check git status mỗi worktree | `step('safetyCheck')` mỗi worktree (có thể fail/timeout độc lập qua relay) | `worktreeId`, `hasChanges` | `WorktreeCleanupService.ts` (`hasUncommittedChanges()`, dòng 160-173, gọi `relay.call('git.exec', ['status','--porcelain'])`) |
| Xoá worktree an toàn | `step('delete')` mỗi worktree | `worktreeId` | `WorktreeCleanupService.ts` (`deleteWorktree()`, dòng 175-181, gọi `relay.call('git.exec', ['worktree','remove'])`) |
| Kết thúc chu kỳ | `ok` | `cleanedCount`, `skippedCount`, `errorCount` | `WorktreeCleanupService.ts` (`runCleanup()` return, dòng 139-143) |

```typescript
// src/main/automations/WorktreeCleanupService.ts — trong runCleanup()
async runCleanup(): Promise<CleanupResult> {
  const span = Tracers.automationCleanupFlow.start({ maxAgeMs: this.maxAgeMs, dryRun: this.dryRun })
  const result: CleanupResult = { cleanedCount: 0, skippedCount: 0, errorCount: 0, at: new Date() }
  const cutoff = Date.now() - this.maxAgeMs
  const worktrees = await this.getEligibleWorktrees(cutoff)
  span.step('queryEligible', { eligibleCount: worktrees.length })

  for (const wt of worktrees) {
    const hasChanges = await this.hasUncommittedChanges(wt)
    span.step('safetyCheck', { worktreeId: wt.id, hasChanges })
    if (hasChanges) { result.skippedCount++; continue }
    if (this.dryRun) { result.cleanedCount++; continue }
    await this.deleteWorktree(wt)
    span.step('delete', { worktreeId: wt.id })
    result.cleanedCount++
  }
  span.ok({ cleanedCount: result.cleanedCount, skippedCount: result.skippedCount, errorCount: result.errorCount })
  return result
}
```

## 5. Lan truyền traceId qua transport của flow này

Điểm băng qua boundary duy nhất trong domain Automation (ngoài IPC/Unix-socket khởi tạo config) là `DevServerRelayBridge.call()`, dùng lại đúng convention đã ship cho `devServer:*`:

1. `WorktreeCleanupService` không tự tạo `traceId` mới cho mỗi lệnh — nó **resume** vào chính span `automation:cleanup` đang mở: `this.relay.call('git.exec', { cwd: wt.path, args: [...], traceId: span.id })`.
2. `DevServerRelayBridge.callWithTimeout()` (`dev-server-relay-bridge.ts:562`) hiện tự tạo span riêng qua `relayCallTracer.start({ devServerId, method })` (dòng 595, 607) — **không đọc `params.traceId`**. Để nối liền `automation:cleanup` → `relay:agentCall`, cần sửa dòng 607 thành:
   ```typescript
   const span = relayCallTracer.start(
     { devServerId: this.config.id, method },
     params.traceId ? { id: params.traceId as string } : undefined
   )
   ```
   Đây là thay đổi runtime nhỏ, nằm trong "Core API Change" đã được CR-TRACE-000 §3.1 phê duyệt (backward-compatible), không phải phạm vi riêng của CR-TRACE-008 — chỉ nêu ở đây vì `WorktreeCleanupService` là call site cụ thể cần nó hoạt động đúng.
3. IPC (`automations:create`) và Unix Socket (`automation.create`) không băng qua network thật (cùng máy), nhưng vẫn nên theo convention WS RPC ở §3.3 để nhất quán khi Automation domain sau này hỗ trợ multi-user/relay — đặt `traceId` cạnh `input`/`params` trong payload IPC/socket.

## Acceptance Criteria

- [ ] `Tracers.automationConfigureFlow/automationScheduleRunFlow/automationEventTriggerFlow/automationCleanupFlow` thêm vào `tracers.ts` với tên `automation:configure`/`automation:scheduleRun`/`automation:eventTrigger`/`automation:cleanup`
- [ ] `automation:configure` bao phủ cả nhánh Renderer (`automations:create` IPC) và CLI (`automation.create` RPC), mỗi nhánh 1 span độc lập
- [ ] `automation:scheduleRun` có `step('resolveTarget')` phân biệt rõ `ok` vs `skipped_unavailable`
- [ ] `automation:eventTrigger` có `step('matchAutomations')` ghi rõ `matchedCount` — bao gồm cả trường hợp 0 match (không log im lặng nữa)
- [ ] Call site `this.automationService.dispatchAutomation(...)` trong `AutomationEventBridge.ts:145` được xác minh tồn tại (hoặc sửa) trước khi thêm tracing, tránh trace một method không chạy được
- [ ] `automation:cleanup` có `step('safetyCheck')` và `step('delete')` riêng biệt cho từng worktree, không gộp thành 1 step cho cả batch
- [ ] `DevServerRelayBridge.callWithTimeout()` đọc `params.traceId` khi resume span `relay:agentCall`, để `automation:cleanup` nối liền được với `relay:agentCall` trong TracePanel
- [ ] Route GitHub Webhook (nếu tồn tại thật trong code) được xác định file cụ thể và bổ sung tracing riêng, hoặc được gỡ khỏi CR nếu xác nhận chưa implement
