/**
 * PgAutomationStore — Postgres-backed `AutomationStoreDependency` (ADR-021 Phase 1)
 *
 * Writes/reads `automation.automations` / `automation.automation_runs`
 * (migration 0021_automation_schema.ts) instead of `persistence.ts`'s `Store`
 * (Electron-desktop JSON file). Server mode only — see ADR-021 §"Không áp
 * dụng cho Electron Desktop mode".
 *
 * Scheduling math (`nextAutomationOccurrenceAfter`/
 * `latestAutomationOccurrenceAtOrBefore`) and run retention/numbering
 * (`pruneAutomationRuns`/`nextAutomationRunNumber`) are reused verbatim from
 * `shared/automation-schedules.ts`/`shared/automation-run-retention.ts` — both
 * are pure functions with no `Store` dependency, so `Store`'s own
 * `advanceAutomationNextRun()`/`createAutomationRun()` and this class compute
 * identical results from identical inputs.
 *
 * RESOLVED (previously the one gap keeping this off by default) —
 * `getRepo()`/`getProjectHostSetups()` delegate to a `Store` instance instead
 * of returning empty. This works now because `Store` itself is Postgres-
 * hydrated in server mode (`Store.hydrateFromPostgres()`,
 * server-bootstrap.ts) — `Repo`/`ProjectHostSetup` (the fields
 * `resolveAutomationRunTarget()` actually needs — both the legacy AND
 * current `runContext` paths call these two methods, not just the
 * deprecated one) live inside `PersistedState.repos`/`.projectHostSetups`,
 * which are part of the same Postgres blob `Store` now reads/writes
 * (migration 0024). So the "no server-mode equivalent" gap this comment
 * used to describe was really "Store hasn't been hydrated from Postgres
 * yet" — once it is, delegating to it IS the server-mode equivalent, not a
 * workaround. `repoSource` is optional (falls back to the old empty
 * behavior) only so this class stays constructible without a `Store`
 * instance in isolated tests.
 *
 * @module main/automations/pg-automation-store
 */

import type { IConnectionPool } from '../db/pool'
import { serviceQualifiedTable } from '../db/migrations/sql-dialect'
import type {
  Automation,
  AutomationDispatchResult,
  AutomationRun,
  AutomationRunTrigger
} from '../../shared/automations-types'
import {
  nextAutomationOccurrenceAfter,
  latestAutomationOccurrenceAtOrBefore
} from '../../shared/automation-schedules'
import type { AutomationRepoSource } from './automation-store-dependency'
import type { AutomationStoreDependency } from './automation-store-dependency'
import type { Repo, ProjectHostSetup } from '../../shared/types'
import type { AutomationRow, AutomationRunRow } from './pg-automation-store-rows'
import { AUTOMATION_COLUMNS, AUTOMATION_RUN_COLUMNS, rowToAutomation, rowToAutomationRun } from './pg-automation-store-rows'
import { createAutomationRunRow, updateAutomationRunRow } from './pg-automation-store-run-mutations'

export class PgAutomationStore implements AutomationStoreDependency {
  /**
   * @param tenantId Resolved once per user-process (ADR-021 §3, same
   * single-user-per-process invariant `RpcContext.tenantId` relies on — see
   * tenancy/tenant-resolver.ts). `undefined` means "no tenant scoping yet"
   * (pre-backfill / user has no company) — queries run unscoped rather than
   * matching nothing, consistent with tenant_id being nullable in Phase 0.
   */
  constructor(
    private readonly pool: IConnectionPool,
    private readonly tenantId: string | undefined,
    // Server mode passes the already-Postgres-hydrated `Store` instance
    // here — see this class's module doc comment.
    private readonly repoSource?: AutomationRepoSource
  ) {}

  private automationsTable(dialect: Parameters<typeof serviceQualifiedTable>[0]): string {
    return serviceQualifiedTable(dialect, 'automation', 'automations')
  }

  private automationRunsTable(dialect: Parameters<typeof serviceQualifiedTable>[0]): string {
    return serviceQualifiedTable(dialect, 'automation', 'automation_runs')
  }

  async listAutomations(): Promise<Automation[]> {
    const rows = await this.pool.withConnection((db) =>
      db.query<AutomationRow>(
        `SELECT ${AUTOMATION_COLUMNS} FROM ${this.automationsTable(db.capabilities.dialect)}${
          this.tenantId ? ' WHERE tenant_id = ?' : ''
        }`,
        this.tenantId ? [this.tenantId] : []
      )
    )
    return rows.map(rowToAutomation)
  }

  async listAutomationRuns(automationId?: string): Promise<AutomationRun[]> {
    const conditions: string[] = []
    const params: string[] = []
    if (this.tenantId) {
      conditions.push('tenant_id = ?')
      params.push(this.tenantId)
    }
    if (automationId) {
      conditions.push('automation_id = ?')
      params.push(automationId)
    }
    const where = conditions.length > 0 ? ` WHERE ${conditions.join(' AND ')}` : ''
    const rows = await this.pool.withConnection((db) =>
      db.query<AutomationRunRow>(
        `SELECT ${AUTOMATION_RUN_COLUMNS} FROM ${this.automationRunsTable(db.capabilities.dialect)}${where}
         ORDER BY created_at DESC`,
        params
      )
    )
    return rows.map(rowToAutomationRun)
  }

  async createAutomationRun(
    automation: Automation,
    scheduledFor: number,
    trigger: AutomationRunTrigger = 'scheduled'
  ): Promise<AutomationRun> {
    return createAutomationRunRow(this.pool, this.tenantId, automation, scheduledFor, trigger, (automationId) =>
      this.listAutomationRuns(automationId)
    )
  }

  async updateAutomationRun(result: AutomationDispatchResult): Promise<AutomationRun> {
    return updateAutomationRunRow(this.pool, result)
  }

  async advanceAutomationNextRun(id: string, now: number = Date.now()): Promise<Automation> {
    const rows = await this.pool.withConnection((db) =>
      db.query<AutomationRow>(
        `SELECT ${AUTOMATION_COLUMNS} FROM ${this.automationsTable(db.capabilities.dialect)} WHERE id = ?`,
        [id]
      )
    )
    if (!rows[0]) {throw new Error('Automation not found.')}
    const current = rowToAutomation(rows[0])
    const nextRunAt = nextAutomationOccurrenceAfter(current.rrule, current.dtstart, now)
    const updated: Automation = { ...current, nextRunAt, updatedAt: Date.now() }
    await this.pool.withConnection((db) =>
      db.query(
        `UPDATE ${this.automationsTable(db.capabilities.dialect)} SET next_run_at = ?, updated_at = ? WHERE id = ?`,
        [updated.nextRunAt, updated.updatedAt, id]
      )
    )
    return updated
  }

  getLatestAutomationOccurrence(automation: Automation, now: number = Date.now()): number | null {
    // Why sync (unlike every other method here): a pure computation over
    // `automation`'s own fields (shared/automation-schedules.ts) — it never
    // touches the pool, and the interface itself keeps this one sync (see
    // AutomationStoreDependency's doc comment).
    return latestAutomationOccurrenceAtOrBefore(automation.rrule, automation.dtstart, now)
  }

  // Delegates to the Postgres-hydrated Store (see module doc comment). Falls
  // back to undefined/[] — same "target vanished" degradation
  // resolveAutomationRunTarget() already handles for every other not-found
  // case — only when constructed without a repoSource (isolated tests).
  async getRepo(id: string): Promise<Repo | undefined> {
    if (!this.repoSource) {
      console.warn(
        '[PgAutomationStore] getRepo() called with no repoSource configured — construct with a ' +
          'Store instance (server-bootstrap.ts) to resolve real Repo data. Returning undefined.'
      )
      return undefined
    }
    return this.repoSource.getRepo(id)
  }

  async getProjectHostSetups(): Promise<ProjectHostSetup[]> {
    if (!this.repoSource) {
      console.warn(
        '[PgAutomationStore] getProjectHostSetups() called with no repoSource configured — construct ' +
          'with a Store instance (server-bootstrap.ts) to resolve real ProjectHostSetup data. Returning [].'
      )
      return []
    }
    return this.repoSource.getProjectHostSetups()
  }
}
