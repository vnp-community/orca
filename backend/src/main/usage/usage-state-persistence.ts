/**
 * UsageStatePersistence — swappable load/save boundary for
 * `ClaudeUsageStore`/`CodexUsageStore` (ADR-021, "chỉ dùng 1 database")
 *
 * Both classes historically owned their persistence directly (`private
 * load()`/`private writeToDisk()`, `readFileSync`/`writeFileSync` on a
 * hardcoded `orca-{claude,codex}-usage.json` path). This interface is that
 * boundary extracted so a Postgres-backed implementation can be a drop-in
 * swap — see usage/pg-usage-state-persistence.ts and
 * usage/pg-usage-store.ts's module doc comment for why this went with a
 * whole-state-blob table (migration 0023) rather than the granular
 * session/daily tables from migration 0022.
 *
 * @module main/usage/usage-state-persistence
 */

export type UsageStatePersistence<TState> = {
  /** Returns `null` when nothing has been persisted yet — caller substitutes its own default state. */
  load(): Promise<TState | null>
  save(state: TState): Promise<void>
}
