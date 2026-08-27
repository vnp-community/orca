/**
 * wrapStoreAsAutomationStoreDependency — bridges `Store` (sync, in-memory) to
 * `AutomationStoreDependency` (async, ADR-021 Phase 1)
 *
 * Every method here just `Promise.resolve()`-wraps the matching synchronous
 * `Store` call — safe in the sync→async direction (a resolved value awaits
 * to itself immediately, no behavior change for `AutomationService`, which
 * already awaits every store call). This is the ONLY place that imports both
 * `Store` and `AutomationStoreDependency` — every other automations/*.ts file
 * depends on the interface only, never the concrete class (see
 * automation-store-dependency.ts's module doc comment for why that split
 * exists).
 *
 * @module main/automations/automation-store-store-adapter
 */

import type { Store } from '../persistence'
import type { AutomationStoreDependency } from './automation-store-dependency'

export function wrapStoreAsAutomationStoreDependency(store: Store): AutomationStoreDependency {
  return {
    listAutomations: async () => store.listAutomations(),
    listAutomationRuns: async (automationId) => store.listAutomationRuns(automationId),
    createAutomationRun: async (automation, scheduledFor, trigger) =>
      store.createAutomationRun(automation, scheduledFor, trigger),
    updateAutomationRun: async (result) => store.updateAutomationRun(result),
    advanceAutomationNextRun: async (id, now) => store.advanceAutomationNextRun(id, now),
    getLatestAutomationOccurrence: (automation, now) =>
      store.getLatestAutomationOccurrence(automation, now),
    getRepo: async (id) => store.getRepo(id),
    getProjectHostSetups: async () => store.getProjectHostSetups()
  }
}
