/**
 * Tests for task auto-advance on git commit (TDD-18 + TDD-20) — T09
 *
 * Tests that commit messages containing #TG-<ref> automatically
 * advance the referenced task status to 'review'.
 *
 * Strategy: test the onCommitComplete() helper function in commit-task-advance.ts
 */

import { describe, it, expect, vi } from 'vitest'
import { onCommitComplete } from '../commit-task-advance'

// ── Helpers ───────────────────────────────────────────────────────────────────

function makeMockTaskService(task: { id: string; projectId: string; status: string } | null = null) {
  return {
    findByRef: vi.fn().mockResolvedValue(task),
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
    const taskSvc = makeMockTaskService({ id: 'task-001', projectId: 'proj-A', status: 'in_progress' })

    await onCommitComplete('fix: implement login #TG-123', 'proj-A', 'user-001', taskSvc as any, makeMockGrantService() as any)

    expect(taskSvc.update).toHaveBeenCalledWith('task-001', { status: 'review' })
  })

  it('commit with "closes #TG-456" → task status review + activity comment', async () => {
    const taskSvc = makeMockTaskService({ id: 'task-456', projectId: 'proj-A', status: 'in_progress' })

    await onCommitComplete('feat: closes #TG-456 add payment', 'proj-A', 'user-001', taskSvc as any, makeMockGrantService() as any)

    expect(taskSvc.update).toHaveBeenCalledWith('task-456', { status: 'review' })
    expect(taskSvc.addComment).toHaveBeenCalledWith(
      'task-456', 'user-001', expect.stringContaining('closes #TG-456'), 'activity'
    )
  })

  it('commit with no task ref → no status change', async () => {
    const taskSvc = makeMockTaskService(null)

    await onCommitComplete('chore: update dependencies', 'proj-A', 'user-001', taskSvc as any, makeMockGrantService() as any)

    expect(taskSvc.update).not.toHaveBeenCalled()
  })

  it('commit with ref but user lacks edit perm → no status change', async () => {
    const taskSvc = makeMockTaskService({ id: 'task-789', projectId: 'proj-A', status: 'in_progress' })
    const grantSvc = makeMockGrantService('view') // only view — not edit

    await onCommitComplete('fix: #TG-789', 'proj-A', 'user-readonly', taskSvc as any, grantSvc as any)

    expect(taskSvc.update).not.toHaveBeenCalled()
  })

  it('commit with ref to task in different project → no status change', async () => {
    const taskSvc = makeMockTaskService({ id: 'task-999', projectId: 'proj-B', status: 'in_progress' })

    // commitMsg is for proj-A, but task belongs to proj-B
    await onCommitComplete('fix: #TG-999', 'proj-A', 'user-001', taskSvc as any, makeMockGrantService() as any)

    expect(taskSvc.update).not.toHaveBeenCalled()
  })

  it('commit with "close #TG-ref" (singular) also triggers comment', async () => {
    const taskSvc = makeMockTaskService({ id: 'task-111', projectId: 'proj-A', status: 'in_progress' })

    await onCommitComplete('fix: close #TG-111 the bug', 'proj-A', 'user-001', taskSvc as any, makeMockGrantService() as any)

    expect(taskSvc.update).toHaveBeenCalledWith('task-111', { status: 'review' })
    expect(taskSvc.addComment).toHaveBeenCalled()
  })

  it('user with manage perm (higher than edit) → status change allowed', async () => {
    const taskSvc = makeMockTaskService({ id: 'task-222', projectId: 'proj-A', status: 'in_progress' })
    const grantSvc = makeMockGrantService('manage')

    await onCommitComplete('feat: #TG-222', 'proj-A', 'manager-001', taskSvc as any, grantSvc as any)

    expect(taskSvc.update).toHaveBeenCalledWith('task-222', { status: 'review' })
  })
})
