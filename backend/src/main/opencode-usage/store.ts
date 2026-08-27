/* eslint-disable max-lines -- Why: this store owns OpenCode analytics persistence, scan policy, and renderer query semantics. Keeping range/scope queries next to scan persistence prevents UI totals from drifting from the SQLite projection. */
import { app } from 'electron'
import { join } from 'node:path'
import type {
  OpenCodeUsageBreakdownKind,
  OpenCodeUsageBreakdownRow,
  OpenCodeUsageDailyPoint,
  OpenCodeUsageRange,
  OpenCodeUsageScanState,
  OpenCodeUsageScope,
  OpenCodeUsageSessionRow,
  OpenCodeUsageSnapshot,
  OpenCodeUsageSummary
} from '../../shared/opencode-usage-types'
// ADR-021 Phase 1: narrowed from `Store` — see UsageStoreRepoSource's module doc comment.
import {
  loadKnownUsageWorktreesByRepo,
  type UsageStoreRepoSource,
  type UsageWorktreeRef
} from '../usage-worktree-metadata'
import type { OpenCodeUsageDailyAggregate, OpenCodeUsagePersistedState } from './types'
import { createWorktreeRefs, scanOpenCodeUsageDatabases } from './scanner'
// ADR-021 — "chỉ dùng 1 database": same swap as claude-usage/store.ts and
// codex-usage/store.ts, see their identical import's comment.
import type { UsageStatePersistence } from '../usage/usage-state-persistence'
import { JsonFileUsageStatePersistence } from '../usage/json-file-usage-state-persistence'

// Why: v2 adds per-database session ownership (stale sibling-copy dedupe).
// Older caches were built without it and can carry doubled sessions (#8006).
const SCHEMA_VERSION = 2
const STALE_MS = 5 * 60_000

let _openCodeUsageFile: string | null = null

function getDefaultState(): OpenCodeUsagePersistedState {
  return {
    schemaVersion: SCHEMA_VERSION,
    worktreeFingerprint: null,
    processedDatabases: [],
    sessions: [],
    dailyAggregates: [],
    scanState: {
      enabled: false,
      lastScanStartedAt: null,
      lastScanCompletedAt: null,
      lastScanError: null
    }
  }
}

export function normalizePersistedState(
  state: OpenCodeUsagePersistedState
): OpenCodeUsagePersistedState {
  if (state.schemaVersion !== SCHEMA_VERSION) {
    return getDefaultState()
  }
  return {
    ...state,
    processedDatabases: (state.processedDatabases ?? []).map((database) => ({
      ...database,
      sessions: (database.sessions ?? []).map(normalizeSessionCost),
      dailyAggregates: (database.dailyAggregates ?? []).map(normalizeDailyAggregateCost)
    })),
    sessions: state.sessions.map(normalizeSessionCost),
    dailyAggregates: state.dailyAggregates.map(normalizeDailyAggregateCost)
  }
}

export function initOpenCodeUsagePath(): void {
  _openCodeUsageFile = join(app.getPath('userData'), 'orca-opencode-usage.json')
}

function getOpenCodeUsageFile(): string {
  if (!_openCodeUsageFile) {
    _openCodeUsageFile = join(app.getPath('userData'), 'orca-opencode-usage.json')
  }
  return _openCodeUsageFile
}

function getRangeCutoff(range: OpenCodeUsageRange): string | null {
  if (range === 'all') {
    return null
  }
  const days = range === '7d' ? 7 : range === '30d' ? 30 : 90
  const now = new Date()
  now.setHours(0, 0, 0, 0)
  now.setDate(now.getDate() - (days - 1))
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function getLocalDay(timestamp: string): string | null {
  const parsed = new Date(timestamp)
  if (Number.isNaN(parsed.getTime())) {
    return null
  }
  const year = parsed.getFullYear()
  const month = String(parsed.getMonth() + 1).padStart(2, '0')
  const day = String(parsed.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function getWorktreeFingerprint(worktreesByRepo: Map<string, UsageWorktreeRef[]>): string {
  const rows = [...worktreesByRepo.entries()]
    .flatMap(([repoId, worktrees]) =>
      worktrees.map((worktree) =>
        JSON.stringify({
          repoId,
          worktreeId: worktree.worktreeId,
          path: worktree.path,
          displayName: worktree.displayName
        })
      )
    )
    .sort()
  return JSON.stringify(rows)
}

function addCost(left: number | null, right: number | null): number | null {
  if (left === null && right === null) {
    return null
  }
  return (left ?? 0) + (right ?? 0)
}

function normalizeDailyAggregateCost(
  entry: OpenCodeUsageDailyAggregate
): OpenCodeUsageDailyAggregate {
  return {
    ...entry,
    estimatedCostUsd: entry.estimatedCostUsd ?? null
  }
}

function normalizeSessionCost(
  session: OpenCodeUsagePersistedState['sessions'][number]
): OpenCodeUsagePersistedState['sessions'][number] {
  return {
    ...session,
    estimatedCostUsd: session.estimatedCostUsd ?? null,
    locationBreakdown: (session.locationBreakdown ?? []).map((entry) => ({
      ...entry,
      estimatedCostUsd: entry.estimatedCostUsd ?? null
    })),
    modelBreakdown: (session.modelBreakdown ?? []).map((entry) => ({
      ...entry,
      estimatedCostUsd: entry.estimatedCostUsd ?? null
    })),
    locationModelBreakdown: (session.locationModelBreakdown ?? []).map((entry) => ({
      ...entry,
      estimatedCostUsd: entry.estimatedCostUsd ?? null
    }))
  }
}

export class OpenCodeUsageStore {
  // Why nullable (was always-present before ADR-021): the persistence
  // backend is async now (Postgres needs real I/O), and constructors cannot
  // await — `state` starts unset and every public method gates on
  // ensureLoaded() before touching it, same lazy-memoized-promise pattern as
  // ClaudeUsageStore/CodexUsageStore (see their identical comment).
  private state: OpenCodeUsagePersistedState | null = null
  private loadPromise: Promise<void> | null = null
  private readonly store: UsageStoreRepoSource
  private readonly persistence: UsageStatePersistence<OpenCodeUsagePersistedState>
  private scanPromise: Promise<void> | null = null

  constructor(
    store: UsageStoreRepoSource,
    persistence: UsageStatePersistence<OpenCodeUsagePersistedState> = new JsonFileUsageStatePersistence(
      getOpenCodeUsageFile
    )
  ) {
    this.store = store
    this.persistence = persistence
  }

  /**
   * Every public method calls this first (see class doc comment). Safe to
   * call repeatedly/concurrently — memoized via `loadPromise` so two
   * overlapping first-callers don't both hit the backend.
   */
  private async ensureLoaded(): Promise<void> {
    if (this.state) {return}
    if (!this.loadPromise) {
      this.loadPromise = this.load().then((state) => {
        this.state = state
      })
    }
    await this.loadPromise
  }

  private async load(): Promise<OpenCodeUsagePersistedState> {
    try {
      const parsed = await this.persistence.load()
      if (!parsed) {
        return getDefaultState()
      }
      return normalizePersistedState({
        ...getDefaultState(),
        ...parsed,
        scanState: {
          ...getDefaultState().scanState,
          ...parsed.scanState
        }
      })
    } catch (error) {
      console.error('[opencode-usage] Failed to load persisted state, starting fresh:', error)
      return getDefaultState()
    }
  }

  private async persist(): Promise<void> {
    if (!this.state) {return}
    await this.persistence.save(this.state)
  }

  /** Non-null accessor — every caller has already awaited ensureLoaded() before touching this. */
  private get s(): OpenCodeUsagePersistedState {
    return this.state!
  }

  async setEnabled(enabled: boolean): Promise<OpenCodeUsageScanState> {
    await this.ensureLoaded()
    this.s.scanState.enabled = enabled
    await this.persist()
    return this.getScanState()
  }

  async getScanState(): Promise<OpenCodeUsageScanState> {
    await this.ensureLoaded()
    return {
      ...this.s.scanState,
      isScanning: this.scanPromise !== null,
      hasAnyOpenCodeData: this.s.sessions.length > 0 || this.s.dailyAggregates.length > 0
    }
  }

  async getSnapshot(
    scope: OpenCodeUsageScope,
    range: OpenCodeUsageRange,
    recentSessionLimit = 10
  ): Promise<OpenCodeUsageSnapshot> {
    await this.ensureLoaded()
    return {
      scanState: await this.getScanState(),
      summary: this.buildSummary(scope, range),
      daily: this.buildDaily(scope, range),
      modelBreakdown: this.buildBreakdown(scope, range, 'model'),
      projectBreakdown: this.buildBreakdown(scope, range, 'project'),
      recentSessions: this.buildRecentSessions(scope, range, recentSessionLimit)
    }
  }

  async refresh(force = false): Promise<OpenCodeUsageScanState> {
    await this.ensureLoaded()
    if (!this.s.scanState.enabled) {
      return this.getScanState()
    }
    const currentWorktreeFingerprint = await this.getCurrentWorktreeFingerprint()
    if (!force && this.s.scanState.lastScanCompletedAt) {
      const ageMs = Date.now() - this.s.scanState.lastScanCompletedAt
      if (ageMs < STALE_MS && this.s.worktreeFingerprint === currentWorktreeFingerprint) {
        return this.getScanState()
      }
    }
    await this.runScan()
    return this.getScanState()
  }

  private async runScan(): Promise<void> {
    if (this.scanPromise) {
      await this.scanPromise
      return
    }

    this.s.scanState.lastScanStartedAt = Date.now()
    this.s.scanState.lastScanError = null
    await this.persist()

    this.scanPromise = (async () => {
      try {
        const repos = this.store.getRepos()
        const worktreesByRepo = loadKnownUsageWorktreesByRepo(this.store, repos)
        const worktreeFingerprint = getWorktreeFingerprint(worktreesByRepo)
        const result = await scanOpenCodeUsageDatabases(
          createWorktreeRefs(repos, worktreesByRepo),
          this.s.worktreeFingerprint === worktreeFingerprint ? this.s.processedDatabases : []
        )
        this.s.processedDatabases = result.processedDatabases
        this.s.sessions = result.sessions
        this.s.dailyAggregates = result.dailyAggregates
        this.s.worktreeFingerprint = worktreeFingerprint
        this.s.scanState.lastScanCompletedAt = Date.now()
        this.s.scanState.lastScanError = null
        await this.persist()
      } catch (error) {
        this.s.scanState.lastScanError = error instanceof Error ? error.message : String(error)
        await this.persist()
      } finally {
        this.scanPromise = null
      }
    })()

    await this.scanPromise
  }

  async getSummary(
    scope: OpenCodeUsageScope,
    range: OpenCodeUsageRange
  ): Promise<OpenCodeUsageSummary> {
    await this.refresh(false)
    return this.buildSummary(scope, range)
  }

  private buildSummary(scope: OpenCodeUsageScope, range: OpenCodeUsageRange): OpenCodeUsageSummary {
    const filteredDaily = this.getFilteredDaily(scope, range)
    const filteredSessions = this.getFilteredSessions(scope, range)

    let inputTokens = 0
    let cachedInputTokens = 0
    let outputTokens = 0
    let reasoningOutputTokens = 0
    let totalTokens = 0
    let events = 0
    let estimatedCostUsd: number | null = null
    const byModel = new Map<string, number>()
    const byProject = new Map<string, number>()

    for (const row of filteredDaily) {
      inputTokens += row.inputTokens
      cachedInputTokens += row.cachedInputTokens
      outputTokens += row.outputTokens
      reasoningOutputTokens += row.reasoningOutputTokens
      totalTokens += row.totalTokens
      events += row.eventCount
      estimatedCostUsd = addCost(estimatedCostUsd, row.estimatedCostUsd)
      byModel.set(
        row.model ?? 'Unknown model',
        (byModel.get(row.model ?? 'Unknown model') ?? 0) + row.totalTokens
      )
      byProject.set(row.projectLabel, (byProject.get(row.projectLabel) ?? 0) + row.totalTokens)
    }

    const topModel =
      [...byModel.entries()].sort((left, right) => right[1] - left[1])[0]?.[0] ?? null
    const topProject =
      [...byProject.entries()].sort((left, right) => right[1] - left[1])[0]?.[0] ?? null

    return {
      scope,
      range,
      sessions: filteredSessions.length,
      events,
      inputTokens,
      cachedInputTokens,
      outputTokens,
      reasoningOutputTokens,
      totalTokens,
      estimatedCostUsd,
      topModel,
      topProject,
      hasAnyOpenCodeData: filteredSessions.length > 0 || filteredDaily.length > 0
    }
  }

  async getDaily(
    scope: OpenCodeUsageScope,
    range: OpenCodeUsageRange
  ): Promise<OpenCodeUsageDailyPoint[]> {
    await this.refresh(false)
    return this.buildDaily(scope, range)
  }

  private buildDaily(
    scope: OpenCodeUsageScope,
    range: OpenCodeUsageRange
  ): OpenCodeUsageDailyPoint[] {
    const byDay = new Map<string, OpenCodeUsageDailyPoint>()
    for (const row of this.getFilteredDaily(scope, range)) {
      const existing = byDay.get(row.day) ?? {
        day: row.day,
        inputTokens: 0,
        cachedInputTokens: 0,
        outputTokens: 0,
        reasoningOutputTokens: 0,
        totalTokens: 0
      }
      existing.inputTokens += row.inputTokens
      existing.cachedInputTokens += row.cachedInputTokens
      existing.outputTokens += row.outputTokens
      existing.reasoningOutputTokens += row.reasoningOutputTokens
      existing.totalTokens += row.totalTokens
      byDay.set(row.day, existing)
    }
    return [...byDay.values()].sort((left, right) => left.day.localeCompare(right.day))
  }

  async getBreakdown(
    scope: OpenCodeUsageScope,
    range: OpenCodeUsageRange,
    kind: OpenCodeUsageBreakdownKind
  ): Promise<OpenCodeUsageBreakdownRow[]> {
    await this.refresh(false)
    return this.buildBreakdown(scope, range, kind)
  }

  private buildBreakdown(
    scope: OpenCodeUsageScope,
    range: OpenCodeUsageRange,
    kind: OpenCodeUsageBreakdownKind
  ): OpenCodeUsageBreakdownRow[] {
    const rows = new Map<string, OpenCodeUsageBreakdownRow>()
    const filteredDaily = this.getFilteredDaily(scope, range)
    const filteredSessions = this.getFilteredSessions(scope, range)

    for (const daily of filteredDaily) {
      const key = kind === 'model' ? (daily.model ?? 'unknown') : daily.projectKey
      const label = kind === 'model' ? (daily.model ?? 'Unknown model') : daily.projectLabel
      const existing = rows.get(key) ?? {
        key,
        label,
        sessions: 0,
        events: 0,
        inputTokens: 0,
        cachedInputTokens: 0,
        outputTokens: 0,
        reasoningOutputTokens: 0,
        totalTokens: 0,
        estimatedCostUsd: null
      }
      existing.events += daily.eventCount
      existing.inputTokens += daily.inputTokens
      existing.cachedInputTokens += daily.cachedInputTokens
      existing.outputTokens += daily.outputTokens
      existing.reasoningOutputTokens += daily.reasoningOutputTokens
      existing.totalTokens += daily.totalTokens
      existing.estimatedCostUsd = addCost(existing.estimatedCostUsd, daily.estimatedCostUsd)
      rows.set(key, existing)
    }

    if (kind === 'model') {
      for (const session of filteredSessions) {
        for (const entry of session.modelBreakdown) {
          const row = rows.get(entry.modelKey)
          if (row) {
            row.sessions++
          }
        }
      }
    } else {
      for (const session of filteredSessions) {
        for (const entry of session.locationBreakdown) {
          const row = rows.get(entry.locationKey)
          if (row) {
            row.sessions++
          }
        }
      }
    }

    return [...rows.values()].sort((left, right) => right.totalTokens - left.totalTokens)
  }

  async getRecentSessions(
    scope: OpenCodeUsageScope,
    range: OpenCodeUsageRange,
    limit = 10
  ): Promise<OpenCodeUsageSessionRow[]> {
    await this.refresh(false)
    return this.buildRecentSessions(scope, range, limit)
  }

  private buildRecentSessions(
    scope: OpenCodeUsageScope,
    range: OpenCodeUsageRange,
    limit = 10
  ): OpenCodeUsageSessionRow[] {
    return this.getFilteredSessions(scope, range)
      .slice(0, limit)
      .map(
        (session): OpenCodeUsageSessionRow => ({
          sessionId: session.sessionId,
          lastActiveAt: session.lastTimestamp,
          durationMinutes: Math.max(
            0,
            Math.round(
              (new Date(session.lastTimestamp).getTime() -
                new Date(session.firstTimestamp).getTime()) /
                60_000
            )
          ),
          projectLabel: session.primaryProjectLabel,
          model: session.primaryModel,
          events: session.eventCount,
          inputTokens: session.totalInputTokens,
          cachedInputTokens: session.totalCachedInputTokens,
          outputTokens: session.totalOutputTokens,
          reasoningOutputTokens: session.totalReasoningOutputTokens,
          totalTokens: session.totalTokens
        })
      )
  }

  private getFilteredDaily(
    scope: OpenCodeUsageScope,
    range: OpenCodeUsageRange
  ): OpenCodeUsageDailyAggregate[] {
    const cutoff = getRangeCutoff(range)
    return this.s.dailyAggregates.filter((row) => {
      if (scope === 'orca' && !row.worktreeId) {
        return false
      }
      if (cutoff && row.day < cutoff) {
        return false
      }
      return true
    })
  }

  private getFilteredSessions(scope: OpenCodeUsageScope, range: OpenCodeUsageRange) {
    const cutoff = getRangeCutoff(range)
    return this.s.sessions.filter((session) => {
      if (scope === 'orca' && !session.primaryWorktreeId) {
        return false
      }
      if (cutoff) {
        const day = getLocalDay(session.lastTimestamp)
        if (!day || day < cutoff) {
          return false
        }
      }
      return true
    })
  }

  private async getCurrentWorktreeFingerprint(): Promise<string> {
    const repos = this.store.getRepos()
    return getWorktreeFingerprint(loadKnownUsageWorktreesByRepo(this.store, repos))
  }
}
