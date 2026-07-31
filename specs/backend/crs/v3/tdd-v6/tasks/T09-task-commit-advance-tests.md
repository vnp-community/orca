# T09 — Write task-commit-advance.test.ts

**Phase:** 2A  
**Effort:** ~30 min  
**Depends on:** T04 (TaskService), T05 (TaskGrantService)  
**Solution ref:** [05-tdd18-task-graph.md §2.6](../solutions/05-tdd18-task-graph.md), [07-tdd20-remote-git-ui.md §4](../solutions/07-tdd20-remote-git-ui.md)  
**TDD ref:** TDD-18 + TDD-20 (cross-cutting — commit → task status advance)

---

## Mục tiêu

Viết tests cho logic tự động cập nhật task status khi git commit chứa task reference `#TG-xxx`.

**Target: ≥ 5 tests**

---

## Files Cần Đọc Trước

1. `src/main/task/TaskService.ts` — xem `findByRef()` method (nếu có) hoặc cách lookup task by reference
2. `src/main/task/TaskGrantService.ts` — `resolvePermission()` signature
3. `src/relay/git-handler-ops.ts` — xem nếu có onCommitComplete hook

> **Note:** Logic commit-advance có thể chưa tồn tại. Nếu vậy, task này bao gồm:  
> 1. Kiểm tra `TaskService` có `findByRef()` không — nếu chưa: **TDD đầu tiên viết test trước, sau đó implement**  
> 2. Tạo `src/main/task/commit-task-advance.ts` nếu cần

---

## File Cần Tạo

### `src/main/task/__tests__/task-commit-advance.test.ts`

```typescript
/**
 * Tests for task auto-advance on git commit (TDD-18 + TDD-20) — T09
 *
 * Tests that commit messages containing #TG-<ref> automatically
 * advance the referenced task status to 'review'.
 *
 * Strategy: test the onCommitComplete() helper function.
 * If this function doesn't exist yet, this test file serves as the TDD spec
 * to guide its implementation.
 */

import { describe, it, expect, vi } from 'vitest'

// Import the function — this may need to be created first:
// src/main/task/commit-task-advance.ts
// export async function onCommitComplete(
//   commitMsg: string,
//   projectId: string,
//   userId: string,
//   taskService: TaskService,
//   grantService: TaskGrantService
// ): Promise<void>

// ── Helpers ───────────────────────────────────────────────────────────────────

function makeMockTaskService(tasks: Record<string, { id: string; projectId: string; status: string }>) {
  return {
    findByRef: vi.fn().mockImplementation(async (ref: string) =>
      Object.values(tasks).find(t => t.id === ref || `TG-${t.id}` === ref) ?? null
    ),
    update: vi.fn().mockResolvedValue(undefined),
    addComment: vi.fn().mockResolvedValue(undefined),
  }
}

function makeMockGrantService(permission: string | null = 'edit') {
  return {
    resolvePermission: vi.fn().mockResolvedValue(permission),
  }
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('onCommitComplete — task auto-advance', () => {
  it('commit with #TG-123 ref → task status set to review', async () => {
    const { onCommitComplete } = await import('../commit-task-advance')
    const taskSvc = makeMockTaskService({
      t1: { id: 'task-001', projectId: 'proj-A', status: 'in_progress' },
    })
    // Mock findByRef to return our task when ref is 'TG-123'
    taskSvc.findByRef.mockResolvedValue({ id: 'task-001', projectId: 'proj-A', status: 'in_progress' })

    await onCommitComplete('fix: implement login #TG-123', 'proj-A', 'user-001', taskSvc as any, makeMockGrantService() as any)

    expect(taskSvc.update).toHaveBeenCalledWith('task-001', { status: 'review' })
  })

  it('commit with "closes #TG-456" → task status review + activity comment', async () => {
    const { onCommitComplete } = await import('../commit-task-advance')
    const taskSvc = makeMockTaskService({})
    taskSvc.findByRef.mockResolvedValue({ id: 'task-456', projectId: 'proj-A', status: 'in_progress' })

    await onCommitComplete('feat: closes #TG-456 add payment', 'proj-A', 'user-001', taskSvc as any, makeMockGrantService() as any)

    expect(taskSvc.update).toHaveBeenCalledWith('task-456', { status: 'review' })
    expect(taskSvc.addComment).toHaveBeenCalledWith(
      'task-456', 'user-001', expect.stringContaining('closes #TG-456'), 'activity'
    )
  })

  it('commit with no task ref → no status change', async () => {
    const { onCommitComplete } = await import('../commit-task-advance')
    const taskSvc = makeMockTaskService({})

    await onCommitComplete('chore: update dependencies', 'proj-A', 'user-001', taskSvc as any, makeMockGrantService() as any)

    expect(taskSvc.update).not.toHaveBeenCalled()
  })

  it('commit with ref but user lacks edit perm → no status change', async () => {
    const { onCommitComplete } = await import('../commit-task-advance')
    const taskSvc = makeMockTaskService({})
    taskSvc.findByRef.mockResolvedValue({ id: 'task-789', projectId: 'proj-A', status: 'in_progress' })
    const grantSvc = makeMockGrantService('view') // only view — not edit

    await onCommitComplete('fix: #TG-789', 'proj-A', 'user-readonly', taskSvc as any, grantSvc as any)

    expect(taskSvc.update).not.toHaveBeenCalled()
  })

  it('commit with ref to task in different project → no status change', async () => {
    const { onCommitComplete } = await import('../commit-task-advance')
    const taskSvc = makeMockTaskService({})
    // Task belongs to proj-B, commit is for proj-A
    taskSvc.findByRef.mockResolvedValue({ id: 'task-999', projectId: 'proj-B', status: 'in_progress' })

    await onCommitComplete('fix: #TG-999', 'proj-A', 'user-001', taskSvc as any, makeMockGrantService() as any)

    expect(taskSvc.update).not.toHaveBeenCalled()
  })
})
```

---

## Implementation Note: commit-task-advance.ts (nếu chưa tồn tại)

Nếu file `src/main/task/commit-task-advance.ts` chưa tồn tại, tạo file này TRƯỚC khi chạy tests:

```typescript
// src/main/task/commit-task-advance.ts
import type { TaskService } from './TaskService'
import type { TaskGrantService } from './TaskGrantService'

const TASK_REF_REGEX = /(?:closes?\s+)?#(TG-[\w-]+)/gi

const EDIT_LEVEL_OR_HIGHER = new Set(['edit', 'execute', 'manage'])

/**
 * Parse commit message for task refs and advance matching tasks to 'review'.
 * Called after git commit on Orca Server.
 */
export async function onCommitComplete(
  commitMsg: string,
  projectId: string,
  userId: string,
  taskService: Pick<TaskService, 'findByRef' | 'update' | 'addComment'>,
  grantService: Pick<TaskGrantService, 'resolvePermission'>
): Promise<void> {
  const refs = [...commitMsg.matchAll(TASK_REF_REGEX)].map(m => m[1])
  for (const ref of refs) {
    const task = await taskService.findByRef(ref).catch(() => null)
    if (!task || task.projectId !== projectId) continue
    const perm = await grantService.resolvePermission(userId, task.id)
    if (!perm || !EDIT_LEVEL_OR_HIGHER.has(perm)) continue
    await taskService.update(task.id, { status: 'review' })
    if (/closes?\s+/i.test(commitMsg)) {
      await taskService.addComment(task.id, userId, `Commit: ${commitMsg}`, 'activity')
    }
  }
}
```

---

## Acceptance Criteria

- [x] `src/main/task/commit-task-advance.ts` tồn tại với `onCommitComplete()` function ✅ (line 19)
- [x] File tạo tại `src/main/task/__tests__/task-commit-advance.test.ts` ✅
- [x] `pnpm vitest run src/main/task/__tests__/task-commit-advance.test.ts` → ≥5 tests passing ✅ (7 tests pass)
- [x] 0 TypeScript errors ✅
