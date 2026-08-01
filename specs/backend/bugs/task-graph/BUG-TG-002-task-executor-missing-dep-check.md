# BUG-TG-002 [BACKEND]: `TaskAgentExecutor.executeTask()` blocking dep check bị thiếu — BL-TG-04 không check BLOCKED_BY_DEPS

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-TG-001,002  
**Note:** task-graph domain fixed  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD BL-TG-04 yêu cầu:
```
[TaskAgentExecutor.execute(taskId, userId)]
├─ Check blocking deps:
│   SELECT from orca_task_edges WHERE from_task_id=taskId AND edge_type='depends_on'
│   → check all to_task status = 'done'
│   IF any not done: return error 'BLOCKED_BY_DEPS'
```

Thực tế `src/main/task/TaskAgentExecutor.ts:45-102`:
```typescript
async executeTask(params: ExecuteTaskParams): Promise<void> {
  // 1. Check permission ✅
  // 2. Get task ✅
  // 3. Build prompt ✅
  // 4. Update status → in_progress ✅
  // 5. Spawn agent ✅
  // 6. Success/Error ✅
  
  // ← THIẾU: check blocking deps!
}
```

**Không có code nào check `orca_task_edges` hay `status !== 'done'` của dependencies.**

Kết quả: Task bị "blocked" vẫn có thể spawn agent → agent làm việc dù dependencies chưa xong → output có thể sai.

## Fix đề xuất

```typescript
// Thêm vào TaskAgentExecutor:
async executeTask(params: ExecuteTaskParams): Promise<void> {
  // 1. Permission check...
  
  // 1b. Check blocking deps:
  const blockedBy = await this.taskService.getBlockingDeps(taskId)
  if (blockedBy.length > 0) {
    throw new Error(`BLOCKED_BY_DEPS: task ${taskId} is blocked by ${blockedBy.map(t => t.id).join(', ')}`)
  }
  
  // 2. Get task...
}
```

`TaskService.getBlockingDeps(taskId)`:
```typescript
SELECT orca_tasks.* FROM orca_task_edges
JOIN orca_tasks ON orca_tasks.id = orca_task_edges.to_task_id
WHERE orca_task_edges.from_task_id = ?
  AND orca_task_edges.edge_type = 'depends_on'
  AND orca_tasks.status != 'done'
```

## Files liên quan

- `src/main/task/TaskAgentExecutor.ts:45-102`: thiếu dep check
- `src/main/task/TaskService.ts`: cần thêm getBlockingDeps()
