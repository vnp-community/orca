# TASK-WF-002: Fix resume orphan step execution (BUG-WF-004)

**Priority:** 🔴 HIGH — Resumed workflows có thể chạy lại steps đã completed  
**Effort:** ~20 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-WF-004  
**Solution ref:** [SOLUTION-workflow-orchestration.md](../solutions/SOLUTION-workflow-orchestration.md)

---

## Mục tiêu

Khi workflow được resume sau crash/cancel, kiểm tra step status trong DB trước khi execute. Chỉ execute các steps chưa `completed`.

## Bước 1 — Tìm resume logic

```bash
grep -n "resume\|Resume\|currentWave\|status.*running" src/main/workflow/WorkflowOrchestrator.ts | head -20
```

## Bước 2 — File cần sửa

```
src/main/workflow/WorkflowOrchestrator.ts
```

## Thay đổi cụ thể

### Trong wave execution loop, thêm skip logic cho completed steps:

Tìm đoạn `Promise.allSettled(executeSteps)` hoặc tương đương:

**TRƯỚC (re-execute all steps in wave, kể cả đã completed):**
```typescript
const waveResults = await Promise.allSettled(
  wave.steps.map(step => this.executeStep(step, inputs, signal))
)
```

**SAU (skip already-completed steps on resume):**
```typescript
const waveResults = await Promise.allSettled(
  wave.steps.map(async (step) => {
    // FIX WF-004: On resume, skip steps that already completed in a previous run
    if (isResuming) {
      const stepRecord = await this.pool.withConnection((db) =>
        db.queryOne<{ status: string }>(
          `SELECT status FROM orca_workflow_step_executions
           WHERE execution_id = ? AND step_id = ?`,
          [executionId, step.id]
        )
      )
      if (stepRecord?.status === 'completed') {
        return { stepId: step.id, status: 'completed', skippedOnResume: true }
      }
    }
    return this.executeStep(step, inputs, signal)
  })
)
```

### Phát hiện `isResuming` state

```typescript
// Trong execute() hoặc resume() entry point:
const isResuming = execution.status === 'failed' || execution.status === 'cancelled'
// Nếu không có DB table orca_workflow_step_executions → tạo trong migration
```

## Migration nếu cần

```sql
-- Nếu chưa có bảng step executions:
CREATE TABLE IF NOT EXISTS orca_workflow_step_executions (
  id            TEXT NOT NULL DEFAULT (lower(hex(randomblob(16)))),
  execution_id  TEXT NOT NULL,
  step_id       TEXT NOT NULL,
  status        TEXT NOT NULL DEFAULT 'pending',
  started_at    INTEGER,
  completed_at  INTEGER,
  output_json   TEXT,
  error_message TEXT,
  PRIMARY KEY (id),
  UNIQUE (execution_id, step_id)
)
```

## Verification

```bash
pnpm tsc --noEmit

# Test: run workflow → interrupt after wave 1 → resume → wave 1 steps NOT re-executed
# wave 1 steps should show skippedOnResume: true in logs
```
