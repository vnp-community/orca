/**
 * WorktreeCleanupService — BUG-BE-AT-002 (BL-AT-04)
 *
 * Background daemon that automatically cleans up idle/stopped worktrees
 * older than the configured max age. Implements HLD BL-AT-04 safety checks:
 *
 *   1. Query worktrees WHERE age > maxAge AND status IN ('idle', 'stopped')
 *   2. For each: git status check (uncommitted changes → skip + log)
 *   3. If clean: delete worktree via DevServerRelayBridge
 *   4. Log result: { cleanedCount, skippedCount }
 *
 * Safety guarantees:
 *   - Never deletes worktrees with uncommitted changes
 *   - Non-throwing: individual failures are logged, service continues
 *   - Dry-run mode available for testing
 *
 * Usage:
 *   const svc = new WorktreeCleanupService(relay, { maxAgeMs: 7 * 24 * 3600_000 })
 *   svc.start()
 *   // on shutdown:
 *   svc.stop()
 *
 * @module main/automations/WorktreeCleanupService
 */

import type { DevServerRelayBridge } from '../dev-server/dev-server-relay-bridge'

// ── Types ─────────────────────────────────────────────────────────────────────

export type WorktreeCleanupPolicy = {
  /** Max age in ms before worktree is eligible for cleanup. Default: 7 days */
  maxAgeMs?:     number
  /** Check interval in ms. Default: 1 hour */
  intervalMs?:   number
  /** If true, log what would be done but don't delete. Default: false */
  dryRun?:       boolean
}

export type CleanupResult = {
  cleanedCount:  number
  skippedCount:  number
  errorCount:    number
  at:            Date
}

type WorktreeRecord = {
  id:         string
  path:       string
  repoPath:   string
  status:     string
  createdAt:  number  // epoch ms
  branch?:    string
}

// ── WorktreeCleanupService ────────────────────────────────────────────────────

const DEFAULT_MAX_AGE_MS    = 7 * 24 * 60 * 60 * 1000   // 7 days
const DEFAULT_INTERVAL_MS   = 60 * 60 * 1000              // 1 hour
const ELIGIBLE_STATUSES     = new Set(['idle', 'stopped', 'archived'])

export class WorktreeCleanupService {
  private timer: ReturnType<typeof setInterval> | null = null
  private readonly maxAgeMs:   number
  private readonly intervalMs: number
  private readonly dryRun:     boolean

  /** Optional callback to list worktrees — injected for testability */
  listWorktrees: (() => Promise<WorktreeRecord[]>) | null = null

  /** Callback fired after each cleanup cycle */
  onCleanupComplete: ((result: CleanupResult) => void) | null = null

  constructor(
    private readonly relay: DevServerRelayBridge,
    policy: WorktreeCleanupPolicy = {}
  ) {
    this.maxAgeMs   = policy.maxAgeMs   ?? DEFAULT_MAX_AGE_MS
    this.intervalMs = policy.intervalMs ?? DEFAULT_INTERVAL_MS
    this.dryRun     = policy.dryRun     ?? false
  }

  /** Start periodic cleanup. Idempotent. */
  start(): void {
    if (this.timer) {return}
    console.log(`[WorktreeCleanupService] Starting (maxAge=${Math.round(this.maxAgeMs / 3600_000)}h, interval=${Math.round(this.intervalMs / 3600_000)}h, dryRun=${this.dryRun})`)
    this.timer = setInterval(() => void this.runCleanup(), this.intervalMs)
    if ((this.timer as NodeJS.Timeout).unref) {(this.timer as NodeJS.Timeout).unref()}
  }

  /** Stop periodic cleanup. */
  stop(): void {
    if (this.timer) {
      clearInterval(this.timer)
      this.timer = null
    }
  }

  /** Run a single cleanup cycle. Can be called manually. */
  async runCleanup(): Promise<CleanupResult> {
    const result: CleanupResult = { cleanedCount: 0, skippedCount: 0, errorCount: 0, at: new Date() }
    const now = Date.now()
    const cutoff = now - this.maxAgeMs

    try {
      // Get eligible worktrees
      const worktrees = await this.getEligibleWorktrees(cutoff)
      console.log(`[WorktreeCleanupService] Found ${worktrees.length} eligible worktrees for cleanup`)

      for (const wt of worktrees) {
        try {
          // FIX BUG-BE-AT-002 (BL-AT-04): Safety check — uncommitted changes?
          const hasChanges = await this.hasUncommittedChanges(wt)
          if (hasChanges) {
            console.warn(`[WorktreeCleanupService] Skip ${wt.path}: has uncommitted changes`)
            result.skippedCount++
            continue
          }

          if (this.dryRun) {
            console.log(`[WorktreeCleanupService] DRY-RUN: would delete ${wt.path} (age=${Math.round((now - wt.createdAt) / 3600_000)}h)`)
            result.cleanedCount++
            continue
          }

          // Delete via relay
          await this.deleteWorktree(wt)
          console.log(`[WorktreeCleanupService] Deleted worktree ${wt.path}`)
          result.cleanedCount++
        } catch (err) {
          console.error(`[WorktreeCleanupService] Error processing worktree ${wt.id}:`, err)
          result.errorCount++
        }
      }
    } catch (err) {
      console.error('[WorktreeCleanupService] Cleanup cycle failed:', err)
      result.errorCount++
    }

    const { cleanedCount, skippedCount, errorCount } = result
    console.log(`[WorktreeCleanupService] Cycle complete: cleaned=${cleanedCount}, skipped=${skippedCount}, errors=${errorCount}`)

    this.onCleanupComplete?.(result)
    return result
  }

  // ── Private helpers ─────────────────────────────────────────────────────────

  private async getEligibleWorktrees(cutoff: number): Promise<WorktreeRecord[]> {
    if (this.listWorktrees) {
      const all = await this.listWorktrees()
      return all.filter((wt) => ELIGIBLE_STATUSES.has(wt.status) && wt.createdAt < cutoff)
    }

    // Fallback: query via relay
    const result = await this.relay.call('worktree.list', {}) as { worktrees?: WorktreeRecord[] } | null
    const all = result?.worktrees ?? []
    return all.filter((wt) => ELIGIBLE_STATUSES.has(wt.status) && wt.createdAt < cutoff)
  }

  private async hasUncommittedChanges(wt: WorktreeRecord): Promise<boolean> {
    try {
      const result = await this.relay.call('git.exec', {
        cwd:  wt.path,
        args: ['status', '--porcelain'],
      }) as { stdout?: string } | null

      const output = result?.stdout?.trim() ?? ''
      return output.length > 0
    } catch {
      // If we can't check, assume safe to skip (conservative)
      return true
    }
  }

  private async deleteWorktree(wt: WorktreeRecord): Promise<void> {
    // Remove git worktree
    await this.relay.call('git.exec', {
      cwd:  wt.repoPath,
      args: ['worktree', 'remove', '--force', wt.path],
    })
  }
}
