# TASK-BE-017.3: Forward `traceId` vào `relay.call()` trong `StepExecutors.ts`

**Phase:** 3
**SOL Ref:** [SOL-BE-TRACE-017](../solutions/SOL-BE-TRACE-017-workflow-orchestration.md)
**CR Ref:** [CR-TRACE-017](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-017-workflow-orchestration.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-017.2 (cần `StepExecutorFn` đã có tham số thứ 4 `traceId?`)
**Status:** ✅ Done (2026-08-04) — `execute()`/`executeByType()` gained the 4th `traceId?: string` param and forward it to `executeAgent`/`executeShell`/`executeNotification`, which each add `traceId` to their `relay.call()` params (`agent.exec`/`shell.exec`/`notification.send`). `executeWebhook()`/`executeCondition()` unchanged (no relay, no traceId param), confirmed via `gitnexus_impact` (risk LOW) before editing. New `src/main/workflow/__tests__/StepExecutors.test.ts` (6 tests, all pass) covers all 3 relay-backed forwards, the 2 non-relay types not requiring traceId, and `traceId: undefined` backward compatibility. `pnpm tsc --noEmit` — no new errors.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "StepExecutors.execute"
codegraph explore "StepExecutors.executeAgent"
codegraph explore "StepExecutors.executeShell"
codegraph explore "StepExecutors.executeNotification"
```

Cả 4 là method đã tồn tại (MODIFY case), nhận thêm tham số thứ 4 `traceId?` do `TASK-BE-017.2` mở rộng `StepExecutorFn`. Chạy:

```
gitnexus_impact({ target: "StepExecutors.execute", direction: "upstream" })
```

Báo cáo blast radius trước khi sửa — xác nhận `executeWebhook()`/`executeCondition()` KHÔNG nhận `traceId` (không qua relay). Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

`WorkflowOrchestrator.executeStep()` (TASK-BE-017.2) gọi `executor(interpolatedStep, execution.inputs, signal, stepSpan.id)` — tham số `traceId` thứ 4 này cần được forward tiếp vào `relay.call()` bên trong `StepExecutors.execute()`/`executeAgent()`/`executeShell()`/`executeNotification()` để `relay:agentCall` (`dev-server-relay-bridge.ts:21`) resume đúng span. `executeWebhook()`/`executeCondition()` KHÔNG qua relay nên không cần `traceId`.

## File: `src/main/workflow/StepExecutors.ts` [MODIFY]

```typescript
async execute(
  step: WorkflowStep,
  inputs: Record<string, unknown>,
  signal: AbortSignal,
  traceId?: string   // [NEW]
): Promise<StepOutput> {
  if (signal.aborted) throw new Error('EXECUTION_CANCELLED')
  const timeoutMs = step.timeout ?? DEFAULT_TIMEOUT_MS
  return Promise.race([
    this.executeByType(step, inputs, signal, traceId),
    new Promise<never>((_, reject) => {
      const timer = setTimeout(() => reject(new Error(`STEP_TIMEOUT: step "${step.id}" exceeded ${timeoutMs}ms`)), timeoutMs)
      signal.addEventListener('abort', () => clearTimeout(timer), { once: true })
    }),
  ])
}

private async executeByType(step: WorkflowStep, inputs: Record<string, unknown>, signal: AbortSignal, traceId?: string): Promise<StepOutput> {
  const { type } = step.config
  switch (type) {
    case 'agent':        return this.executeAgent(step, signal, traceId)
    case 'shell':         return this.executeShell(step, signal, traceId)
    case 'webhook':       return this.executeWebhook(step, signal)       // không qua relay — không cần traceId
    case 'notification':  return this.executeNotification(step, signal, traceId)
    case 'condition':     return this.executeCondition(step, inputs)     // sync, không I/O
    default: throw new Error(`UNSUPPORTED_STEP_TYPE: "${String(type)}"`)
  }
}

private async executeAgent(step: WorkflowStep, signal: AbortSignal, traceId?: string): Promise<StepOutput> {
  const relay = await this.getRelay(step)
  if (signal.aborted) throw new Error('EXECUTION_CANCELLED')
  const result = (await relay.call('agent.exec', {
    stepId: step.id,
    prompt: step.config['prompt'],
    worktreePath: step.config['worktreePath'],
    trustPreset: step.config['trustPreset'] ?? 'default',
    traceId,   // [NEW] — relay:agentCall (dev-server-relay-bridge.ts:21) resume theo id này
  })) as { exitCode?: number; stdout?: string; stderr?: string }
  return { exitCode: result.exitCode ?? 0, stdout: result.stdout, stderr: result.stderr }
}

private async executeShell(step: WorkflowStep, signal: AbortSignal, traceId?: string): Promise<StepOutput> {
  const relay = await this.getRelay(step)
  if (signal.aborted) throw new Error('EXECUTION_CANCELLED')
  const result = (await relay.call('shell.exec', {
    script: step.config['script'], env: step.config['env'] ?? {}, traceId,   // [NEW]
  })) as { exitCode?: number; stdout?: string; stderr?: string }
  return { exitCode: result.exitCode ?? 0, stdout: result.stdout, stderr: result.stderr }
}

private async executeNotification(step: WorkflowStep, signal: AbortSignal, traceId?: string): Promise<StepOutput> {
  const relay = await this.getRelay(step)
  if (signal.aborted) throw new Error('EXECUTION_CANCELLED')
  await relay.call('notification.send', {
    channel: step.config['channel'], message: step.config['message'], traceId,   // [NEW]
  })
  return { exitCode: 0 }
}
```

## Verification

```bash
pnpm tsc --noEmit
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `execute()`/`executeByType()` nhận thêm tham số thứ 4 `traceId?: string`, forward xuống đúng executor theo `step.config.type`
- [ ] `traceId` forward đúng vào `relay.call()` params ở cả 3 loại step dùng relay (`agent`/`shell`/`notification`)
- [ ] `executeWebhook()`/`executeCondition()` KHÔNG nhận `traceId` param (không qua relay) — type-check + runtime không lỗi khi omit
- [ ] Không đổi hành vi khi `traceId` là `undefined` (100% backward compatible với call site cũ, nếu còn)
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
