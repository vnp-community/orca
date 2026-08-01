# BUG-WF-004 [BACKEND]: `WorkflowOrchestrator.resumeRunningExecutions()` không reset wave state — orphan executions sau server restart

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-WF-002  
**Note:** WorkflowOrchestrator.ts: skip completed steps on resume  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

`src/main/workflow/WorkflowOrchestrator.ts:218-224`:
```typescript
async resumeRunningExecutions(): Promise<void> {
  const running = await this.listExecutions({ status: 'running' })
  for (const execution of running) {
    console.log(`[WorkflowOrchestrator] Resuming execution ${execution.id} from wave ${execution.currentWave}`)
    void this.runExecution(execution, execution.currentWave)
  }
}
```

Vấn đề:
1. Sau server restart, `status='running'` executions được resume từ `currentWave`
2. Nhưng `abortControllers` Map đã bị xóa (server restart → memory cleared)
3. `cancel()` check `this.abortControllers.get(executionId)` — sau resume, AbortController được set
4. **Problem**: Nếu execution đang ở wave 3 khi restart, wave 1 và 2 steps có `status='done'` trong DB, nhưng wave 3 steps có thể có `status='running'` (orphaned steps)

Wave 3 re-executes → **duplicate step execution** cho steps đã bắt đầu trước restart.

## Thêm: Missing `markStepFailed` on Resume

```typescript
// Khi resume, steps đang chạy (status='running') trong wave hiện tại cần reset:
UPDATE orca_step_executions SET status='failed', error='server_restarted'
WHERE execution_id = ? AND status = 'running'
```

Hiện tại không có logic reset này.

## Fix đề xuất

```typescript
async resumeRunningExecutions(): Promise<void> {
  const running = await this.listExecutions({ status: 'running' })
  for (const execution of running) {
    // Reset orphaned steps from current wave
    await this.resetOrphanedSteps(execution.id, execution.currentWave)
    void this.runExecution(execution, execution.currentWave)
  }
}

private async resetOrphanedSteps(executionId: string, waveIndex: number): Promise<void> {
  await this.pool.withConnection((db) =>
    db.query(
      `UPDATE orca_step_executions SET status='failed', error='server_restarted_mid_wave'
       WHERE execution_id = ? AND wave_index = ? AND status = 'running'`,
      [executionId, waveIndex]
    )
  )
}
```

## Files liên quan

- `src/main/workflow/WorkflowOrchestrator.ts:218-224`: resume logic
