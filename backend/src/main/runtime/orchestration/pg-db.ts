/**
 * PgOrchestrationDb — Postgres-backed async port of `OrchestrationDb`
 * (ADR-021, "chỉ dùng 1 database")
 *
 * Faithful method-for-method port of `runtime/orchestration/db.ts`'s public
 * API against the `orchestration` schema (migration 0020), same table
 * shapes, same business rules (circuit breaker at 3 failures, DAG
 * promotion, pane-key-equivalent dispatch dedup, etc.).
 *
 * ⚠️⚠️ NOT WIRED INTO THE RUNTIME — `coordinator.ts` and its ~9 other
 * dependents (`TaskOrchestrationBridge.ts`, `orca-runtime.ts`,
 * `rpc/methods/orchestration*.ts`, ...) still construct and use the SQLite
 * `OrchestrationDb` exclusively (`gitnexus impact` on `OrchestrationDb`:
 * CRITICAL, 176 impacted symbols, 51 direct callers). This class exists so
 * that cutover can happen later as a reviewed, tested, DEDICATED change —
 * not bundled into this pass. See specs/backend/models/08-postgres-microservices-target-architecture.md
 * Phase 2.
 *
 * THE ATOMICITY DIFFERENCE THIS PORT HAD TO SOLVE: `OrchestrationDb` runs on
 * `better-sqlite3`, which executes every statement synchronously on one
 * thread — so a sequence like "update task status, then read+promote every
 * pending task whose deps are now satisfied" (`updateTaskStatus` →
 * `promoteReadyTasks`) is atomic *for free*: nothing else can interleave
 * between those statements. Postgres access here is async — every `await`
 * is a potential interleaving point for a concurrent coordinator tick or a
 * second dispatch. Every method that chains more than one read/write where
 * the original synchronous code relied on that implicit atomicity
 * (`updateTaskStatus`, `createDispatchContext`, `createGate`, `resolveGate`,
 * `failDispatch`, `completeActiveDispatchForTask`, `failActiveDispatchForTask`,
 * `convertLifecycleMessageToRejection`) is wrapped in `pool.withTransaction()`
 * here — same net behavior, but the atomicity is now explicit and enforced
 * by Postgres's transaction isolation instead of by JS's single-threadedness.
 * Read-only multi-step methods (`listTasksWithDispatch`'s correlated
 * subquery, `getIdleTerminals`) don't need this — nothing they read can be
 * invalidated by another writer in a way that produces a wrong *answer*, only
 * a stale one, same as the SQLite version already tolerated between ticks.
 *
 * Timestamps: `OrchestrationDb` stores TEXT timestamps (SQLite
 * `datetime('now')` / `new Date().toISOString()`), compared lexicographically
 * (see its `getStaleDispatches` doc comment). This port always writes
 * `new Date().toISOString()` — also lexicographically sortable, and every
 * consumer only ever compares/reads these as opaque strings, never parses a
 * specific non-ISO shape out of them.
 *
 * @module main/runtime/orchestration/pg-db
 */

import { randomBytes } from 'node:crypto'
import type { IConnectionPool } from '../../db/pool'
import type { IDatabase } from '../../db/types'
import { serviceQualifiedTable } from '../../db/migrations/sql-dialect'
import type {
  MessageType,
  MessagePriority,
  TaskStatus,
  DispatchStatus,
  GateStatus,
  CoordinatorStatus,
  MessageRow,
  TaskRow,
  DispatchContextRow,
  DecisionGateRow,
  CoordinatorRun
} from './types'
import { buildOrchestrationTaskDisplayMetadata } from '../../../shared/orchestration-task-display'
import { parsePaneKey } from '../../../shared/stable-pane-id'

function isEquivalentPaneKey(a: string, b: string): boolean {
  if (a === b) {return true}
  const aLeaf = parsePaneKey(a)?.leafId
  const bLeaf = parsePaneKey(b)?.leafId
  return Boolean(aLeaf && bLeaf && aLeaf === bLeaf)
}

function generateId(prefix: string): string {
  return `${prefix}_${randomBytes(6).toString('hex')}`
}

function nowIso(): string {
  return new Date().toISOString()
}

function addLifecycleRejectionMarker(payload: string | null, reason: string): string {
  let parsed: Record<string, unknown> = {}
  try {
    const value: unknown = payload ? JSON.parse(payload) : {}
    if (value && typeof value === 'object' && !Array.isArray(value)) {
      parsed = value as Record<string, unknown>
    }
  } catch {
    // Authority reconciliation only reaches this path with object payloads.
  }
  return JSON.stringify({ ...parsed, _orcaLifecycleRejection: { code: 'sender_not_assignee', reason } })
}

export class PgOrchestrationDb {
  /**
   * @param tenantId Resolved once per user-process (ADR-021 §3). `undefined`
   * runs unscoped (pre-backfill) — see PgAutomationStore's identical choice.
   */
  constructor(
    private readonly pool: IConnectionPool,
    private readonly tenantId: string | undefined
  ) {}

  private table(name: 'messages' | 'tasks' | 'dispatch_contexts' | 'decision_gates' | 'coordinator_runs') {
    return (dialect: Parameters<typeof serviceQualifiedTable>[0]): string =>
      serviceQualifiedTable(dialect, 'orchestration', name)
  }

  // ── Messages ──

  async insertMessage(msg: {
    from: string
    to: string
    subject: string
    body?: string
    type?: MessageType
    priority?: MessagePriority
    threadId?: string
    payload?: string
    senderPaneKey?: string
  }): Promise<MessageRow> {
    const id = generateId('msg')
    return this.pool.withConnection(async (db) => {
      const table = this.table('messages')(db.capabilities.dialect)
      await db.query(
        `INSERT INTO ${table}
           (id, tenant_id, from_handle, to_handle, subject, body, type, priority, thread_id, payload, sender_pane_key, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [
          id, this.tenantId ?? null, msg.from, msg.to, msg.subject, msg.body ?? '',
          msg.type ?? 'status', msg.priority ?? 'normal', msg.threadId ?? null,
          msg.payload ?? null, msg.senderPaneKey ?? null, nowIso()
        ]
      )
      return this.getMessageByIdOnConn(db, id) as Promise<MessageRow>
    })
  }

  private async getMessageByIdOnConn(db: IDatabase, id: string): Promise<MessageRow | undefined> {
    const table = this.table('messages')(db.capabilities.dialect)
    const rows = await db.query<MessageRow>(`SELECT * FROM ${table} WHERE id = ?`, [id])
    return rows[0]
  }

  async getMessageById(id: string): Promise<MessageRow | undefined> {
    return this.pool.withConnection((db) => this.getMessageByIdOnConn(db, id))
  }

  async getUnreadMessages(toHandle: string, types?: MessageType[]): Promise<MessageRow[]> {
    return this.pool.withConnection(async (db) => {
      const table = this.table('messages')(db.capabilities.dialect)
      if (types && types.length > 0) {
        const placeholders = types.map(() => '?').join(',')
        return db.query<MessageRow>(
          `SELECT * FROM ${table} WHERE to_handle = ? AND read = 0 AND type IN (${placeholders}) ORDER BY sequence`,
          [toHandle, ...types]
        )
      }
      return db.query<MessageRow>(
        `SELECT * FROM ${table} WHERE to_handle = ? AND read = 0 ORDER BY sequence`,
        [toHandle]
      )
    })
  }

  async convertLifecycleMessageToRejection(messageId: string, reason: string): Promise<MessageRow | undefined> {
    return this.pool.withTransaction(async (db) => {
      const message = await this.getMessageByIdOnConn(db, messageId)
      if (!message || (message.type !== 'worker_done' && message.type !== 'heartbeat')) {
        return message
      }
      const table = this.table('messages')(db.capabilities.dialect)
      const originalBody = message.body ? `\n\nOriginal body:\n${message.body}` : ''
      const body = `Orca rejected this ${message.type}: ${reason}${originalBody}`
      const payload = addLifecycleRejectionMarker(message.payload, reason)
      await db.query(
        `UPDATE ${table} SET priority = 'high', subject = ?, body = ?, payload = ? WHERE id = ?`,
        [`Rejected ${message.type}: ${message.subject}`, body, payload, messageId]
      )
      return this.getMessageByIdOnConn(db, messageId)
    })
  }

  async getUndeliveredUnreadMessages(toHandle: string, types?: MessageType[]): Promise<MessageRow[]> {
    return this.pool.withConnection(async (db) => {
      const table = this.table('messages')(db.capabilities.dialect)
      if (types && types.length > 0) {
        const placeholders = types.map(() => '?').join(',')
        return db.query<MessageRow>(
          `SELECT * FROM ${table} WHERE to_handle = ? AND read = 0 AND delivered_at IS NULL AND type IN (${placeholders}) ORDER BY sequence`,
          [toHandle, ...types]
        )
      }
      return db.query<MessageRow>(
        `SELECT * FROM ${table} WHERE to_handle = ? AND read = 0 AND delivered_at IS NULL ORDER BY sequence`,
        [toHandle]
      )
    })
  }

  async getAllMessages(toHandle: string, limit = 20): Promise<MessageRow[]> {
    return this.pool.withConnection((db) => {
      const table = this.table('messages')(db.capabilities.dialect)
      return db.query<MessageRow>(
        `SELECT * FROM ${table} WHERE to_handle = ? ORDER BY sequence DESC LIMIT ?`,
        [toHandle, limit]
      )
    })
  }

  async markAsRead(ids: string[]): Promise<void> {
    if (ids.length === 0) {return}
    await this.pool.withConnection((db) => {
      const table = this.table('messages')(db.capabilities.dialect)
      const placeholders = ids.map(() => '?').join(',')
      return db.query(`UPDATE ${table} SET read = 1 WHERE id IN (${placeholders})`, ids)
    })
  }

  // Why nowIso() as a bind param, not a SQL-side datetime() call (unlike the
  // SQLite version): keeps this cross-dialect without a dialect branch — see
  // module doc comment's Timestamps section.
  async markAsDelivered(ids: string[]): Promise<void> {
    if (ids.length === 0) {return}
    await this.pool.withConnection((db) => {
      const table = this.table('messages')(db.capabilities.dialect)
      const placeholders = ids.map(() => '?').join(',')
      return db.query(
        `UPDATE ${table} SET delivered_at = ? WHERE id IN (${placeholders})`,
        [nowIso(), ...ids]
      )
    })
  }

  async markAsReadAndDelivered(ids: string[]): Promise<void> {
    if (ids.length === 0) {return}
    await this.pool.withConnection((db) => {
      const table = this.table('messages')(db.capabilities.dialect)
      const placeholders = ids.map(() => '?').join(',')
      // Why COALESCE(delivered_at, ?) not a raw overwrite: superseded
      // lifecycle messages must keep their original delivered_at if already
      // set — same intent as the SQLite version's COALESCE(delivered_at, datetime('now')).
      return db.query(
        `UPDATE ${table} SET read = 1, delivered_at = COALESCE(delivered_at, ?) WHERE id IN (${placeholders})`,
        [nowIso(), ...ids]
      )
    })
  }

  async getInbox(limit = 20): Promise<MessageRow[]> {
    return this.pool.withConnection((db) => {
      const table = this.table('messages')(db.capabilities.dialect)
      return db.query<MessageRow>(`SELECT * FROM ${table} ORDER BY sequence DESC LIMIT ?`, [limit])
    })
  }

  async getAllMessagesForHandle(toHandle: string, limit = 100, types?: MessageType[]): Promise<MessageRow[]> {
    return this.pool.withConnection(async (db) => {
      const table = this.table('messages')(db.capabilities.dialect)
      if (types && types.length > 0) {
        const placeholders = types.map(() => '?').join(',')
        return db.query<MessageRow>(
          `SELECT * FROM ${table} WHERE to_handle = ? AND type IN (${placeholders}) ORDER BY sequence DESC LIMIT ?`,
          [toHandle, ...types, limit]
        )
      }
      return db.query<MessageRow>(
        `SELECT * FROM ${table} WHERE to_handle = ? ORDER BY sequence DESC LIMIT ?`,
        [toHandle, limit]
      )
    })
  }

  async getThreadMessagesFor(threadId: string, toHandle: string, afterSequence?: number): Promise<MessageRow[]> {
    return this.pool.withConnection((db) => {
      const table = this.table('messages')(db.capabilities.dialect)
      if (afterSequence !== undefined) {
        return db.query<MessageRow>(
          `SELECT * FROM ${table} WHERE thread_id = ? AND to_handle = ? AND sequence > ? ORDER BY sequence ASC`,
          [threadId, toHandle, afterSequence]
        )
      }
      return db.query<MessageRow>(
        `SELECT * FROM ${table} WHERE thread_id = ? AND to_handle = ? ORDER BY sequence ASC`,
        [threadId, toHandle]
      )
    })
  }

  // ── Tasks ──

  async createTask(task: {
    spec: string
    taskTitle?: string
    displayName?: string
    deps?: string[]
    parentId?: string
    createdByTerminalHandle?: string
  }): Promise<TaskRow> {
    const id = generateId('task')
    const depsJson = JSON.stringify(task.deps ?? [])
    const hasDeps = (task.deps ?? []).length > 0
    const status: TaskStatus = hasDeps ? 'pending' : 'ready'
    const display = buildOrchestrationTaskDisplayMetadata({
      spec: task.spec,
      taskTitle: task.taskTitle,
      displayName: task.displayName
    })
    return this.pool.withConnection(async (db) => {
      const table = this.table('tasks')(db.capabilities.dialect)
      await db.query(
        `INSERT INTO ${table}
           (id, tenant_id, parent_id, created_by_terminal_handle, task_title, display_name, spec, status, deps, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [
          id, this.tenantId ?? null, task.parentId ?? null, task.createdByTerminalHandle ?? null,
          display.taskTitle || null, display.displayName || null, task.spec, status, depsJson, nowIso()
        ]
      )
      return this.getTaskOnConn(db, id) as Promise<TaskRow>
    })
  }

  private async getTaskOnConn(db: IDatabase, id: string): Promise<TaskRow | undefined> {
    const table = this.table('tasks')(db.capabilities.dialect)
    const rows = await db.query<TaskRow>(`SELECT * FROM ${table} WHERE id = ?`, [id])
    return rows[0]
  }

  async getTask(id: string): Promise<TaskRow | undefined> {
    return this.pool.withConnection((db) => this.getTaskOnConn(db, id))
  }

  async listTasks(filter?: { status?: TaskStatus; ready?: boolean }): Promise<TaskRow[]> {
    return this.pool.withConnection((db) => {
      const table = this.table('tasks')(db.capabilities.dialect)
      if (filter?.ready) {
        return db.query<TaskRow>(`SELECT * FROM ${table} WHERE status = 'ready' ORDER BY created_at`)
      }
      if (filter?.status) {
        return db.query<TaskRow>(`SELECT * FROM ${table} WHERE status = ? ORDER BY created_at`, [filter.status])
      }
      return db.query<TaskRow>(`SELECT * FROM ${table} ORDER BY created_at`)
    })
  }

  // Why not the SQLite version's `rowid`-based correlated subquery: Postgres
  // has no implicit `rowid`. `sequence` doesn't exist on dispatch_contexts —
  // substitute `created_at` (also monotonic-enough for "most recent" here,
  // same as the messages table uses `sequence` only because it needed a
  // stable sort key for a high-frequency mailbox, not because dispatch
  // ordering needs sub-timestamp precision).
  async listTasksWithDispatch(
    filter?: { status?: TaskStatus; ready?: boolean }
  ): Promise<(TaskRow & { assignee_handle: string | null; dispatch_id: string | null })[]> {
    return this.pool.withConnection((db) => {
      const dialect = db.capabilities.dialect
      const tasksTable = this.table('tasks')(dialect)
      const dispatchTable = this.table('dispatch_contexts')(dialect)
      const whereClauses: string[] = []
      const params: string[] = []
      if (filter?.ready) {
        whereClauses.push("t.status = 'ready'")
      } else if (filter?.status) {
        whereClauses.push('t.status = ?')
        params.push(filter.status)
      }
      const where = whereClauses.length > 0 ? `WHERE ${whereClauses.join(' AND ')}` : ''
      const sql = `
        SELECT t.*, d.assignee_handle AS assignee_handle, d.id AS dispatch_id
        FROM ${tasksTable} t
        LEFT JOIN (
          SELECT dc.*
          FROM ${dispatchTable} dc
          INNER JOIN (
            SELECT task_id, MAX(created_at) AS max_created_at
            FROM ${dispatchTable}
            WHERE status IN ('pending', 'dispatched')
            GROUP BY task_id
          ) latest ON latest.task_id = dc.task_id AND latest.max_created_at = dc.created_at
        ) d ON d.task_id = t.id
        ${where}
        ORDER BY t.created_at
      `
      return db.query<TaskRow & { assignee_handle: string | null; dispatch_id: string | null }>(sql, params)
    })
  }

  async updateTaskStatus(id: string, status: TaskStatus, result?: string): Promise<TaskRow | undefined> {
    return this.pool.withTransaction(async (db) => {
      const completedAt = status === 'completed' || status === 'failed' ? nowIso() : null
      const table = this.table('tasks')(db.capabilities.dialect)
      await db.query(
        `UPDATE ${table} SET status = ?, result = COALESCE(?, result), completed_at = COALESCE(?, completed_at) WHERE id = ?`,
        [status, result ?? null, completedAt, id]
      )
      if (status === 'completed') {
        await this.promoteReadyTasksOnConn(db, id)
        await this.completeActiveDispatchForTaskOnConn(db, id)
      }
      return this.getTaskOnConn(db, id)
    })
  }

  private async promoteReadyTasksOnConn(db: IDatabase, completedTaskId: string): Promise<void> {
    const table = this.table('tasks')(db.capabilities.dialect)
    const candidates = await db.query<TaskRow>(`SELECT * FROM ${table} WHERE status = 'pending'`)
    for (const task of candidates) {
      const deps: string[] = JSON.parse(task.deps)
      if (!deps.includes(completedTaskId)) {continue}
      let allDepsCompleted = true
      for (const depId of deps) {
        const dep = await this.getTaskOnConn(db, depId)
        if (dep?.status !== 'completed') {
          allDepsCompleted = false
          break
        }
      }
      if (allDepsCompleted) {
        await db.query(`UPDATE ${table} SET status = 'ready' WHERE id = ?`, [task.id])
      }
    }
  }

  // ── Dispatch Contexts ──

  async createDispatchContext(
    taskId: string,
    assigneeHandle: string,
    assigneePaneKey?: string
  ): Promise<DispatchContextRow> {
    return this.pool.withTransaction(async (db) => {
      const task = await this.getTaskOnConn(db, taskId)
      if (!task) {throw new Error(`Task not found: ${taskId}`)}
      if (task.status !== 'ready') {
        throw new Error(`Task ${taskId} is ${task.status}; only ready tasks can be dispatched`)
      }

      const existing = await this.findActiveDispatchForAssigneeOnConn(db, assigneeHandle, assigneePaneKey)
      if (existing) {
        throw new Error(
          `Terminal ${assigneeHandle} already has an active dispatch (${existing.id} for task ${existing.task_id})`
        )
      }

      const dispatchTable = this.table('dispatch_contexts')(db.capabilities.dialect)
      const [prior] = await db.query<{ max_failures: number | null }>(
        `SELECT MAX(failure_count) as max_failures FROM ${dispatchTable} WHERE task_id = ?`,
        [taskId]
      )
      const priorFailures = prior?.max_failures ?? 0

      const id = generateId('ctx')
      await db.query(
        `INSERT INTO ${dispatchTable}
           (id, tenant_id, task_id, assignee_handle, assignee_pane_key, status, failure_count, dispatched_at, created_at)
         VALUES (?, ?, ?, ?, ?, 'dispatched', ?, ?, ?)`,
        [id, this.tenantId ?? null, taskId, assigneeHandle, assigneePaneKey ?? null, priorFailures, nowIso(), nowIso()]
      )

      const tasksTable = this.table('tasks')(db.capabilities.dialect)
      await db.query(`UPDATE ${tasksTable} SET status = 'dispatched' WHERE id = ?`, [taskId])

      return this.getDispatchContextByIdOnConn(db, id) as Promise<DispatchContextRow>
    })
  }

  private async getDispatchContextByIdOnConn(db: IDatabase, id: string): Promise<DispatchContextRow | undefined> {
    const table = this.table('dispatch_contexts')(db.capabilities.dialect)
    const rows = await db.query<DispatchContextRow>(`SELECT * FROM ${table} WHERE id = ?`, [id])
    return rows[0]
  }

  async getDispatchContextById(dispatchId: string): Promise<DispatchContextRow | undefined> {
    return this.pool.withConnection((db) => this.getDispatchContextByIdOnConn(db, dispatchId))
  }

  async getDispatchContext(taskId: string): Promise<DispatchContextRow | undefined> {
    return this.pool.withConnection(async (db) => {
      const table = this.table('dispatch_contexts')(db.capabilities.dialect)
      const rows = await db.query<DispatchContextRow>(
        `SELECT * FROM ${table} WHERE task_id = ? ORDER BY created_at DESC LIMIT 1`,
        [taskId]
      )
      return rows[0]
    })
  }

  async getActiveDispatchForTerminal(handle: string): Promise<DispatchContextRow | undefined> {
    return this.pool.withConnection((db) => this.findActiveDispatchForAssigneeOnConn(db, handle))
  }

  private async findActiveDispatchForAssigneeOnConn(
    db: IDatabase,
    assigneeHandle: string,
    assigneePaneKey?: string
  ): Promise<DispatchContextRow | undefined> {
    const table = this.table('dispatch_contexts')(db.capabilities.dialect)
    const [byHandle] = await db.query<DispatchContextRow>(
      `SELECT * FROM ${table} WHERE assignee_handle = ? AND status IN ('pending', 'dispatched') LIMIT 1`,
      [assigneeHandle]
    )
    if (byHandle) {return byHandle}
    if (!assigneePaneKey) {return undefined}

    const actives = await db.query<DispatchContextRow>(
      `SELECT * FROM ${table} WHERE assignee_pane_key IS NOT NULL AND status IN ('pending', 'dispatched')`
    )
    for (const row of actives) {
      if (row.assignee_pane_key && isEquivalentPaneKey(row.assignee_pane_key, assigneePaneKey)) {
        return row
      }
    }
    return undefined
  }

  async getLatestDispatchForTerminal(handle: string): Promise<DispatchContextRow | undefined> {
    return this.pool.withConnection(async (db) => {
      const table = this.table('dispatch_contexts')(db.capabilities.dialect)
      const rows = await db.query<DispatchContextRow>(
        `SELECT * FROM ${table} WHERE assignee_handle = ? ORDER BY created_at DESC LIMIT 1`,
        [handle]
      )
      return rows[0]
    })
  }

  private async completeDispatchOnConn(db: IDatabase, ctxId: string): Promise<void> {
    const table = this.table('dispatch_contexts')(db.capabilities.dialect)
    await db.query(
      `UPDATE ${table} SET status = 'completed', completed_at = ? WHERE id = ?`,
      [nowIso(), ctxId]
    )
  }

  async completeDispatch(ctxId: string): Promise<void> {
    await this.pool.withConnection((db) => this.completeDispatchOnConn(db, ctxId))
  }

  private async completeActiveDispatchForTaskOnConn(db: IDatabase, taskId: string): Promise<void> {
    const table = this.table('dispatch_contexts')(db.capabilities.dialect)
    const [active] = await db.query<DispatchContextRow>(
      `SELECT * FROM ${table} WHERE task_id = ? AND status IN ('pending', 'dispatched') ORDER BY created_at DESC LIMIT 1`,
      [taskId]
    )
    if (active) {await this.completeDispatchOnConn(db, active.id)}
  }

  async completeActiveDispatchForTask(taskId: string): Promise<void> {
    await this.pool.withTransaction((db) => this.completeActiveDispatchForTaskOnConn(db, taskId))
  }

  async failActiveDispatchForTask(taskId: string, error: string): Promise<DispatchContextRow | undefined> {
    return this.pool.withTransaction(async (db) => {
      const table = this.table('dispatch_contexts')(db.capabilities.dialect)
      const [active] = await db.query<DispatchContextRow>(
        `SELECT * FROM ${table} WHERE task_id = ? AND status IN ('pending', 'dispatched') ORDER BY created_at DESC LIMIT 1`,
        [taskId]
      )
      return active ? this.failDispatchOnConn(db, active.id, error) : undefined
    })
  }

  private async failDispatchOnConn(db: IDatabase, ctxId: string, error: string): Promise<DispatchContextRow | undefined> {
    const dispatchTable = this.table('dispatch_contexts')(db.capabilities.dialect)
    const ctx = await this.getDispatchContextByIdOnConn(db, ctxId)
    if (!ctx) {return undefined}

    const newFailureCount = ctx.failure_count + 1
    const newStatus: DispatchStatus = newFailureCount >= 3 ? 'circuit_broken' : 'failed'
    await db.query(
      `UPDATE ${dispatchTable} SET status = ?, failure_count = ?, last_failure = ? WHERE id = ?`,
      [newStatus, newFailureCount, error, ctxId]
    )

    // Why 'ready' not 'pending' (same as the SQLite version): deps are
    // already satisfied — 'pending' would strand the task since
    // promoteReadyTasks only runs when a dep completes.
    const taskStatus: TaskStatus = newStatus === 'circuit_broken' ? 'failed' : 'ready'
    const tasksTable = this.table('tasks')(db.capabilities.dialect)
    await db.query(`UPDATE ${tasksTable} SET status = ? WHERE id = ?`, [taskStatus, ctx.task_id])

    return this.getDispatchContextByIdOnConn(db, ctxId)
  }

  async failDispatch(ctxId: string, error: string): Promise<DispatchContextRow | undefined> {
    return this.pool.withTransaction((db) => this.failDispatchOnConn(db, ctxId, error))
  }

  async recordHeartbeat(dispatchId: string, at: string): Promise<void> {
    await this.pool.withConnection((db) => {
      const table = this.table('dispatch_contexts')(db.capabilities.dialect)
      return db.query(
        `UPDATE ${table} SET last_heartbeat_at = ? WHERE id = ? AND status = 'dispatched'`,
        [at, dispatchId]
      )
    })
  }

  async getStaleDispatches(thresholdIso: string): Promise<DispatchContextRow[]> {
    return this.pool.withConnection((db) => {
      const table = this.table('dispatch_contexts')(db.capabilities.dialect)
      return db.query<DispatchContextRow>(
        `SELECT * FROM ${table}
         WHERE status = 'dispatched'
           AND dispatched_at IS NOT NULL
           AND dispatched_at < ?
           AND (last_heartbeat_at IS NULL OR last_heartbeat_at < ?)`,
        [thresholdIso, thresholdIso]
      )
    })
  }

  // ── Decision Gates ──

  async createGate(gate: { taskId: string; question: string; options?: string[] }): Promise<DecisionGateRow> {
    return this.pool.withTransaction(async (db) => {
      const id = generateId('gate')
      const gatesTable = this.table('decision_gates')(db.capabilities.dialect)
      await db.query(
        `INSERT INTO ${gatesTable} (id, tenant_id, task_id, question, options, created_at)
         VALUES (?, ?, ?, ?, ?, ?)`,
        [id, this.tenantId ?? null, gate.taskId, gate.question, JSON.stringify(gate.options ?? []), nowIso()]
      )
      await this.completeActiveDispatchForTaskOnConn(db, gate.taskId)
      const tasksTable = this.table('tasks')(db.capabilities.dialect)
      await db.query(`UPDATE ${tasksTable} SET status = 'blocked' WHERE id = ?`, [gate.taskId])

      const rows = await db.query<DecisionGateRow>(`SELECT * FROM ${gatesTable} WHERE id = ?`, [id])
      return rows[0]!
    })
  }

  async resolveGate(gateId: string, resolution: string): Promise<DecisionGateRow | undefined> {
    return this.pool.withTransaction(async (db) => {
      const gatesTable = this.table('decision_gates')(db.capabilities.dialect)
      const [gate] = await db.query<DecisionGateRow>(`SELECT * FROM ${gatesTable} WHERE id = ?`, [gateId])
      if (!gate) {return undefined}

      await db.query(
        `UPDATE ${gatesTable} SET status = 'resolved', resolution = ?, resolved_at = ? WHERE id = ?`,
        [resolution, nowIso(), gateId]
      )
      // Why 'ready' (same as SQLite version): the worker needs to be
      // re-engaged with the decision outcome, not resumed at its prior status.
      const tasksTable = this.table('tasks')(db.capabilities.dialect)
      await db.query(`UPDATE ${tasksTable} SET status = 'ready' WHERE id = ?`, [gate.task_id])

      const rows = await db.query<DecisionGateRow>(`SELECT * FROM ${gatesTable} WHERE id = ?`, [gateId])
      return rows[0]
    })
  }

  async timeoutGate(gateId: string): Promise<DecisionGateRow | undefined> {
    return this.pool.withConnection(async (db) => {
      const table = this.table('decision_gates')(db.capabilities.dialect)
      await db.query(
        `UPDATE ${table} SET status = 'timeout', resolved_at = ? WHERE id = ?`,
        [nowIso(), gateId]
      )
      const rows = await db.query<DecisionGateRow>(`SELECT * FROM ${table} WHERE id = ?`, [gateId])
      return rows[0]
    })
  }

  async listGates(filter?: { taskId?: string; status?: GateStatus }): Promise<DecisionGateRow[]> {
    return this.pool.withConnection((db) => {
      const table = this.table('decision_gates')(db.capabilities.dialect)
      if (filter?.taskId && filter?.status) {
        return db.query<DecisionGateRow>(
          `SELECT * FROM ${table} WHERE task_id = ? AND status = ? ORDER BY created_at`,
          [filter.taskId, filter.status]
        )
      }
      if (filter?.taskId) {
        return db.query<DecisionGateRow>(`SELECT * FROM ${table} WHERE task_id = ? ORDER BY created_at`, [filter.taskId])
      }
      if (filter?.status) {
        return db.query<DecisionGateRow>(`SELECT * FROM ${table} WHERE status = ? ORDER BY created_at`, [filter.status])
      }
      return db.query<DecisionGateRow>(`SELECT * FROM ${table} ORDER BY created_at`)
    })
  }

  async getGate(id: string): Promise<DecisionGateRow | undefined> {
    return this.pool.withConnection(async (db) => {
      const table = this.table('decision_gates')(db.capabilities.dialect)
      const rows = await db.query<DecisionGateRow>(`SELECT * FROM ${table} WHERE id = ?`, [id])
      return rows[0]
    })
  }

  // ── Coordinator Runs ──

  async createCoordinatorRun(run: { spec: string; coordinatorHandle: string; pollIntervalMs?: number }): Promise<CoordinatorRun> {
    const id = generateId('run')
    return this.pool.withConnection(async (db) => {
      const table = this.table('coordinator_runs')(db.capabilities.dialect)
      await db.query(
        `INSERT INTO ${table} (id, tenant_id, spec, status, coordinator_handle, poll_interval_ms, created_at)
         VALUES (?, ?, ?, 'running', ?, ?, ?)`,
        [id, this.tenantId ?? null, run.spec, run.coordinatorHandle, run.pollIntervalMs ?? 2000, nowIso()]
      )
      const rows = await db.query<CoordinatorRun>(`SELECT * FROM ${table} WHERE id = ?`, [id])
      return rows[0]!
    })
  }

  async getCoordinatorRun(id: string): Promise<CoordinatorRun | undefined> {
    return this.pool.withConnection(async (db) => {
      const table = this.table('coordinator_runs')(db.capabilities.dialect)
      const rows = await db.query<CoordinatorRun>(`SELECT * FROM ${table} WHERE id = ?`, [id])
      return rows[0]
    })
  }

  async updateCoordinatorRun(id: string, status: CoordinatorStatus): Promise<CoordinatorRun | undefined> {
    return this.pool.withConnection(async (db) => {
      const table = this.table('coordinator_runs')(db.capabilities.dialect)
      const completedAt = status === 'completed' || status === 'failed' ? nowIso() : null
      await db.query(
        `UPDATE ${table} SET status = ?, completed_at = COALESCE(?, completed_at) WHERE id = ?`,
        [status, completedAt, id]
      )
      const rows = await db.query<CoordinatorRun>(`SELECT * FROM ${table} WHERE id = ?`, [id])
      return rows[0]
    })
  }

  async getActiveCoordinatorRun(): Promise<CoordinatorRun | undefined> {
    return this.pool.withConnection(async (db) => {
      const table = this.table('coordinator_runs')(db.capabilities.dialect)
      const rows = await db.query<CoordinatorRun>(
        `SELECT * FROM ${table} WHERE status = 'running' ORDER BY created_at DESC LIMIT 1`
      )
      return rows[0]
    })
  }

  // ── Queries for Coordinator ──

  async getIdleTerminals(excludeHandles: string[] = []): Promise<string[]> {
    return this.pool.withConnection(async (db) => {
      const dispatchTable = this.table('dispatch_contexts')(db.capabilities.dialect)
      const messagesTable = this.table('messages')(db.capabilities.dialect)
      const active = await db.query<{ assignee_handle: string }>(
        `SELECT DISTINCT assignee_handle FROM ${dispatchTable} WHERE status IN ('pending', 'dispatched')`
      )
      const busyHandles = new Set(active.map((r) => r.assignee_handle))
      for (const h of excludeHandles) {busyHandles.add(h)}
      const allHandles = await db.query<{ to_handle: string }>(
        `SELECT DISTINCT to_handle FROM ${messagesTable} UNION SELECT DISTINCT from_handle FROM ${messagesTable}`
      )
      return [...new Set(allHandles.map((r) => r.to_handle))].filter((h) => !busyHandles.has(h))
    })
  }

  // ── Lifecycle ──

  async resetAll(): Promise<void> {
    await this.pool.withTransaction(async (db) => {
      const dialect = db.capabilities.dialect
      await db.query(`DELETE FROM ${this.table('coordinator_runs')(dialect)}`)
      await db.query(`DELETE FROM ${this.table('decision_gates')(dialect)}`)
      await db.query(`DELETE FROM ${this.table('dispatch_contexts')(dialect)}`)
      await db.query(`DELETE FROM ${this.table('tasks')(dialect)}`)
      await db.query(`DELETE FROM ${this.table('messages')(dialect)}`)
    })
  }

  async resetTasks(): Promise<void> {
    await this.pool.withTransaction(async (db) => {
      const dialect = db.capabilities.dialect
      await db.query(`DELETE FROM ${this.table('coordinator_runs')(dialect)}`)
      await db.query(`DELETE FROM ${this.table('decision_gates')(dialect)}`)
      await db.query(`DELETE FROM ${this.table('dispatch_contexts')(dialect)}`)
      await db.query(`DELETE FROM ${this.table('tasks')(dialect)}`)
    })
  }

  async resetMessages(): Promise<void> {
    await this.pool.withConnection((db) => db.query(`DELETE FROM ${this.table('messages')(db.capabilities.dialect)}`))
  }

  // Why no-op (unlike OrchestrationDb.close(), which closes its dedicated
  // SQLite file handle): this class never owns the pool's lifecycle — same
  // reasoning as PooledDatabaseAdapter.close() (db/pooled-database-adapter.ts).
  async close(): Promise<void> {}
}
