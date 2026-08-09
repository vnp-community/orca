/**
 * TaskGrantService — BFS ancestor grant resolution for task permissions (TDD-18)
 *
 * Permission hierarchy (highest → lowest):
 *   manage > execute > edit > comment > view
 *
 * Grant resolution algorithm:
 * 1. Check direct grants on taskId
 * 2. Walk parent_id chain (BFS up ancestor tree)
 * 3. For each ancestor: check grants with apply_tree = 1
 * 4. Return the HIGHEST permission found across all matching grants
 *
 * Scope matching:
 * - 'everyone' → always matches
 * - 'user' → matches if grant.scopeId === userId
 * - 'team' → matches if userId is in team via orca_team_members
 * - 'role' → matches if userId has that role (stored in orca_users.role field)
 *
 * @module main/task/TaskGrantService
 */

import { randomUUID } from 'node:crypto'
import type { IConnectionPool } from '../db/pool'
import type { TaskService } from './TaskService'
import type { TaskGrant, TaskPermission } from '../../shared/task-types'
import { TASK_PERMISSION_ORDER } from '../../shared/task-types'
import { Tracers } from '../../shared/trace/tracers'

// ── DB row types ──────────────────────────────────────────────────────────────

type GrantRow = {
  id: string
  taskId: string
  scope: string
  scopeId: string | null
  permission: string
  applyTree: number
  grantedBy: string
  expiresAt: number | null
  createdAt: number
}

function rowToGrant(r: GrantRow): TaskGrant {
  return {
    id: r.id,
    taskId: r.taskId,
    scope: r.scope as TaskGrant['scope'],
    scopeId: r.scopeId ?? undefined,
    permission: r.permission as TaskPermission,
    applyTree: r.applyTree === 1,
    grantedBy: r.grantedBy,
    expiresAt: r.expiresAt ? new Date(r.expiresAt) : undefined,
    createdAt: new Date(r.createdAt),
  }
}

// ── TaskGrantService ──────────────────────────────────────────────────────────

export class TaskGrantService {
  constructor(
    private readonly pool: IConnectionPool,
    private readonly taskService: TaskService
  ) {}

  // ── Public API ────────────────────────────────────────────────────────────

  /**
   * Grant a permission on a task.
   * If applyTree=true, the grant propagates to all descendants.
   * @returns The new grant ID
   */
  async grantPermission(params: {
    taskId: string
    scope: TaskGrant['scope']
    scopeId?: string
    permission: TaskPermission
    applyTree?: boolean
    grantedBy: string
    expiresAt?: Date
  }): Promise<string> {
    const id = randomUUID()
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_task_grants
           (id, task_id, scope, scope_id, permission, apply_tree, granted_by, expires_at, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [
          id,
          params.taskId,
          params.scope,
          params.scopeId ?? null,
          params.permission,
          params.applyTree ? 1 : 0,
          params.grantedBy,
          params.expiresAt ? params.expiresAt.getTime() : null,
          now,
        ]
      )
    )
    return id
  }

  /**
   * Resolve the effective permission for a user on a task.
   *
   * Algorithm:
   * 1. Collect all applicable grants for this user on taskId (direct)
   * 2. Walk ancestor chain: for each ancestor, collect grants with apply_tree=1
   * 3. Return the HIGHEST permission found; null if none
   */
  async resolvePermission(userId: string, taskId: string): Promise<TaskPermission | null> {
    const span = Tracers.taskGraphGrantFlow.start({ userId, taskId })
    const now = Date.now()

    // Collect candidate task IDs: the task itself + ancestors
    const ancestorIds = await this.getAncestorIds(taskId)
    const candidates: { taskId: string; requireApplyTree: boolean }[] = [
      { taskId, requireApplyTree: false },     // direct grants on the task
      ...ancestorIds.map(id => ({ taskId: id, requireApplyTree: true })), // inherited grants
    ]

    let highest: TaskPermission | null = null
    let matchedScope: string | undefined
    let matchedDirect: boolean | undefined

    for (const { taskId: tid, requireApplyTree } of candidates) {
      const grants = await this.getGrantsForTask(tid, requireApplyTree)
      for (const grant of grants) {
        // Skip expired grants
        if (grant.expiresAt && grant.expiresAt.getTime() < now) {continue}
        // Check scope match
        const matches = await this.matchesScope(userId, grant)
        if (!matches) {continue}

        // Compare with current highest
        const level = TASK_PERMISSION_ORDER[grant.permission] ?? 0
        const currentLevel = highest ? (TASK_PERMISSION_ORDER[highest] ?? 0) : -1
        if (level > currentLevel) {
          highest = grant.permission
          matchedScope = grant.scope
          matchedDirect = tid === taskId
        }
      }
    }

    if (highest === null) {
      span.fail('NO_GRANT_FOUND', { userId, taskId, ancestorCount: ancestorIds.length })
      return null
    }

    // Exactly 1 summary step per call — no step per-candidate/per-grant in the nested
    // loop above (this runs on every permission check, avoid hot-path noise).
    span.step('grant-match', { matchedScope, direct: matchedDirect })
    span.ok({ permission: highest, matchedScope, direct: matchedDirect })
    return highest
  }

  /**
   * Revoke a grant by ID.
   */
  async revokeGrant(grantId: string): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(`DELETE FROM orca_task_grants WHERE id = ?`, [grantId])
    )
  }

  /**
   * List all grants on a task (direct grants only, not inherited).
   */
  async listGrants(taskId: string): Promise<TaskGrant[]> {
    const rows = await this.pool.withConnection((db) =>
      db.query<GrantRow>(
        `SELECT id, task_id as taskId, scope, scope_id as scopeId,
                permission, apply_tree as applyTree, granted_by as grantedBy,
                expires_at as expiresAt, created_at as createdAt
         FROM orca_task_grants WHERE task_id = ? ORDER BY created_at DESC`,
        [taskId]
      )
    )
    return rows.map(rowToGrant)
  }

  // ── Private helpers ───────────────────────────────────────────────────────

  /** Walk parent_id chain and return ancestor IDs in order (immediate parent first). */
  private async getAncestorIds(taskId: string): Promise<string[]> {
    const ancestors: string[] = []
    let current = await this.taskService.get(taskId)
    while (current?.parentId) {
      ancestors.push(current.parentId)
      current = await this.taskService.get(current.parentId)
    }
    return ancestors
  }

  /** Get grants on a specific task; optionally filter by apply_tree=1 */
  private async getGrantsForTask(taskId: string, requireApplyTree: boolean): Promise<TaskGrant[]> {
    const sql = requireApplyTree
      ? `SELECT id, task_id as taskId, scope, scope_id as scopeId,
                permission, apply_tree as applyTree, granted_by as grantedBy,
                expires_at as expiresAt, created_at as createdAt
         FROM orca_task_grants WHERE task_id = ? AND apply_tree = 1`
      : `SELECT id, task_id as taskId, scope, scope_id as scopeId,
                permission, apply_tree as applyTree, granted_by as grantedBy,
                expires_at as expiresAt, created_at as createdAt
         FROM orca_task_grants WHERE task_id = ?`

    const rows = await this.pool.withConnection((db) =>
      db.query<GrantRow>(sql, [taskId])
    )
    return rows.map(rowToGrant)
  }

  /** Check if a grant applies to a given userId */
  private async matchesScope(userId: string, grant: TaskGrant): Promise<boolean> {
    switch (grant.scope) {
      case 'everyone':
        return true

      case 'user':
        return grant.scopeId === userId

      case 'team': {
        if (!grant.scopeId) {return false}
        const rows = await this.pool.withConnection((db) =>
          db.query<{ userId: string }>(
            `SELECT user_id as userId FROM orca_team_members
             WHERE team_id = ? AND user_id = ?`,
            [grant.scopeId, userId]
          )
        )
        return rows.length > 0
      }

      case 'role': {
        if (!grant.scopeId) {return false}
        const rows = await this.pool.withConnection((db) =>
          db.query<{ role: string }>(
            `SELECT role FROM orca_users WHERE id = ?`,
            [userId]
          )
        )
        return rows[0]?.role === grant.scopeId
      }

      default:
        return false
    }
  }
}
