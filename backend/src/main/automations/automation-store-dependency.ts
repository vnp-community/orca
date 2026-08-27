/**
 * AutomationStoreDependency — narrow interface seam for AutomationService's
 * persistence dependency (ADR-021 Phase 1)
 *
 * `AutomationService`/`run-target-resolution.ts` were written against the
 * concrete `Store` class (persistence.ts) — Electron desktop mode's ~3900-line
 * JSON store. ADR-021's plan is a Postgres-backed implementation for server
 * mode (specs/backend/models/08-postgres-microservices-target-architecture.md
 * §2 `automation-service`/`automation` schema, migration 0021), but `Store`
 * itself must NOT change (desktop mode keeps it — ADR-021 §"Không áp dụng cho
 * Electron Desktop mode").
 *
 * ⚠️ Deliberately ASYNC, unlike `Store`'s own (synchronous, in-memory) methods
 * — a first version of this file mirrored `Store`'s sync signatures via
 * `Pick<Store, ...>`, which turned out to be the wrong shape the moment a real
 * Postgres implementation (`PgAutomationStore`) was written: `Automation[]` is
 * not assignable to `Promise<Automation[]>`, so a truly async store can never
 * structurally satisfy a sync interface — no amount of "just await it" fixes
 * a method that doesn't return a Promise. `Store` no longer satisfies this
 * interface for free; `automation-store-store-adapter.ts`'s
 * `wrapStoreAsAutomationStoreDependency()` bridges the two (trivial
 * `Promise.resolve()`-wrapping, since Store's calls are synchronous already —
 * "async-ifying" a sync call is always safe, the reverse is not).
 *
 * This is the exact, minimal surface `AutomationService` and
 * `resolveAutomationRunTarget()` actually call — verified by grepping every
 * `store.<method>(` / `this.store.<method>(` call site in automations/*.ts.
 * `PgAutomationStore` (backed by migration 0021's
 * `automation.automations`/`automation.automation_runs` tables) implements
 * this directly.
 *
 * Follows the same narrowing idiom already used elsewhere in this codebase —
 * see `usage-worktree-metadata.ts`'s `Pick<Store, 'getAllWorktreeMeta'>` (that
 * one stays sync because its only consumers — the usage stores — are
 * themselves synchronous JSON-file readers, not a mixed sync/async boundary
 * like this one).
 *
 * @module main/automations/automation-store-dependency
 */

import type {
  Automation,
  AutomationDispatchResult,
  AutomationRun,
  AutomationRunTrigger
} from '../../shared/automations-types'
import type { Repo, ProjectHostSetup } from '../../shared/types'
import type { Store } from '../persistence'

/**
 * `PgAutomationStore.getRepo()`/`getProjectHostSetups()` delegate to a real
 * `Store` instance (`Repo`/`ProjectHostSetup` live in `PersistedState`) —
 * safe to do in server mode now that `Store` itself is Postgres-hydrated
 * (`Store.hydrateFromPostgres()`, server-bootstrap.ts, ADR-021). Narrowed via
 * `Pick` (same idiom as `usage-worktree-metadata.ts`'s
 * `UsageStoreRepoSource`) rather than importing the full `Store` type where
 * this is consumed.
 */
export type AutomationRepoSource = Pick<Store, 'getRepo' | 'getProjectHostSetups'>

export type AutomationStoreDependency = {
  listAutomations(): Promise<Automation[]>
  listAutomationRuns(automationId?: string): Promise<AutomationRun[]>
  createAutomationRun(
    automation: Automation,
    scheduledFor: number,
    trigger?: AutomationRunTrigger
  ): Promise<AutomationRun>
  updateAutomationRun(result: AutomationDispatchResult): Promise<AutomationRun>
  advanceAutomationNextRun(id: string, now?: number): Promise<Automation>
  /** Pure computation, no I/O (shared/automation-schedules.ts) — stays sync
   *  even though every other method here is async; see PgAutomationStore's
   *  identical choice for the same reason. */
  getLatestAutomationOccurrence(automation: Automation, now?: number): number | null
  getRepo(id: string): Promise<Repo | undefined>
  getProjectHostSetups(): Promise<ProjectHostSetup[]>
}
