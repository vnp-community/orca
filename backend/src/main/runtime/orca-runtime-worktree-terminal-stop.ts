// frontend/src/main/runtime/orca-runtime-worktree-terminal-stop.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-081): the "stop/check terminals for
// a worktree" cluster (stopTerminalsForWorktree, stopExactTerminalsForWorktree,
// getLivePtyIdsForWorktree, hasTerminalsForWorktree) extracted from
// OrcaRuntimeService via the composition pattern. A cohesive, contiguous
// block found by re-running the gap-analysis sweep with a stricter
// forwarding-field filter (excludes both single-line and multi-line
// `identifier: RuntimeXCommands['method'] = ...` wiring noise, which had
// been inflating earlier raw line-gap measurements).
import { setsEqual } from './orca-runtime-tail-buffer'
import type { ResolvedWorktree } from './orca-runtime'
import type { RuntimeGraphStore } from './orca-runtime-graph-store'
import type { RuntimePtyController } from './orca-runtime-types'

export type RuntimeWorktreeTerminalStopCommandHost = {
  getGraph(): RuntimeGraphStore
  getPtyController(): RuntimePtyController | null
  resolveWorktreeSelector(selector: string): Promise<ResolvedWorktree>
  captureReadyGraphEpoch(): number
  assertStableReadyGraph(expectedGraphEpoch: number): void
  refreshPtyWorktreeRecordsFromController(
    resolvedWorktrees: ResolvedWorktree[],
    targetWorktreeId?: string | null
  ): Promise<Set<string> | null>
  getResolvedWorktreeMap(): Promise<Map<string, ResolvedWorktree>>
}

// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-081): stop/verify-stopped/check-live
// terminals scoped to one worktree - shares the live-PTY-id scan
// (getLivePtyIdsForWorktree) between the exact-stop verification and its own
// simpler kill-everything sibling.
export class RuntimeWorktreeTerminalStopCommands {
  constructor(private readonly host: RuntimeWorktreeTerminalStopCommandHost) {}

  async stopTerminalsForWorktree(worktreeSelector: string): Promise<{ stopped: number }> {
    // Why: this mutates live PTYs, so the runtime must reject it while the
    // renderer graph is reloading instead of acting on cached leaf ownership.
    const graphEpoch = this.host.captureReadyGraphEpoch()
    const worktree = await this.host.resolveWorktreeSelector(worktreeSelector)
    this.host.assertStableReadyGraph(graphEpoch)
    const ptyIds = new Set<string>()
    for (const leaf of this.host.getGraph().leaves.values()) {
      if (leaf.worktreeId === worktree.id && leaf.ptyId) {
        ptyIds.add(leaf.ptyId)
      }
    }
    for (const pty of this.host.getGraph().ptysById.values()) {
      if (pty.worktreeId === worktree.id && pty.connected) {
        ptyIds.add(pty.ptyId)
      }
    }

    let stopped = 0
    for (const ptyId of ptyIds) {
      if (this.host.getPtyController()?.kill(ptyId)) {
        stopped += 1
      }
    }
    return { stopped }
  }

  async stopExactTerminalsForWorktree(
    worktreeSelector: string,
    expectedPtyIds: readonly string[],
    opts: { keepHistory?: boolean; targetOnly?: boolean } = {}
  ): Promise<{
    stopped: number
    stoppedPtyIds: string[]
    livePtyIds: string[]
    postStopVerified: boolean
    postStopFailure?: string
    remainingLivePtyIds?: string[]
  }> {
    // Why: worktree sleep needs proof of the complete live set; pane hibernation
    // only needs proof that its target PTY was live and is now gone.
    const graphEpoch = this.host.captureReadyGraphEpoch()
    const worktree = await this.host.resolveWorktreeSelector(worktreeSelector)
    this.host.assertStableReadyGraph(graphEpoch)
    const expected = new Set(expectedPtyIds.filter((ptyId) => ptyId.length > 0))
    if (expected.size !== 1) {
      throw new Error('terminal_exact_stop_requires_single_pty')
    }
    const resolvedWorktrees = [...(await this.host.getResolvedWorktreeMap()).values()]
    const refreshedPtyLiveness =
      await this.host.refreshPtyWorktreeRecordsFromController(resolvedWorktrees)
    if (!refreshedPtyLiveness) {
      throw new Error('terminal_liveness_unavailable')
    }
    const livePtyIds = this.getLivePtyIdsForWorktree(worktree.id, refreshedPtyLiveness)
    const targetOnly = opts.targetOnly === true
    const expectedIsLive = [...expected].every((ptyId) => livePtyIds.has(ptyId))
    if (targetOnly ? !expectedIsLive : !setsEqual(livePtyIds, expected)) {
      const error = Object.assign(new Error('terminal_stop_pty_set_mismatch'), {
        livePtyIds: [...livePtyIds].sort(),
        expectedPtyIds: [...expected].sort()
      })
      throw error
    }

    const ptyController = this.host.getPtyController()
    if (!ptyController?.stopAndWait) {
      throw new Error('terminal_exact_stop_unavailable')
    }

    const stoppedPtyIds: string[] = []
    for (const ptyId of [...expected].sort()) {
      if (!(await ptyController.stopAndWait(ptyId, { keepHistory: opts.keepHistory }))) {
        throw Object.assign(new Error('terminal_exact_stop_failed'), { ptyId })
      }
      stoppedPtyIds.push(ptyId)
    }
    const postStopLiveness =
      await this.host.refreshPtyWorktreeRecordsFromController(resolvedWorktrees)
    if (!postStopLiveness) {
      return {
        stopped: stoppedPtyIds.length,
        stoppedPtyIds,
        livePtyIds: [...livePtyIds].sort(),
        postStopVerified: false,
        postStopFailure: 'terminal_liveness_unavailable'
      }
    }
    const remainingLivePtyIds = this.getLivePtyIdsForWorktree(worktree.id, postStopLiveness)
    const stoppedTargetsStillLive = [...expected].filter((ptyId) => remainingLivePtyIds.has(ptyId))
    if (targetOnly ? stoppedTargetsStillLive.length > 0 : remainingLivePtyIds.size > 0) {
      return {
        stopped: stoppedPtyIds.length,
        stoppedPtyIds,
        livePtyIds: [...livePtyIds].sort(),
        postStopVerified: false,
        postStopFailure: 'terminal_exact_stop_still_live',
        remainingLivePtyIds: [...remainingLivePtyIds].sort()
      }
    }
    return {
      stopped: stoppedPtyIds.length,
      stoppedPtyIds,
      livePtyIds: [...livePtyIds].sort(),
      postStopVerified: true,
      ...(targetOnly && remainingLivePtyIds.size > 0
        ? { remainingLivePtyIds: [...remainingLivePtyIds].sort() }
        : {})
    }
  }

  private getLivePtyIdsForWorktree(
    worktreeId: string,
    freshPtyIds?: ReadonlySet<string>
  ): Set<string> {
    const ptyIds = new Set<string>()
    for (const leaf of this.host.getGraph().leaves.values()) {
      if (
        leaf.worktreeId === worktreeId &&
        leaf.connected &&
        leaf.ptyId &&
        (!freshPtyIds || freshPtyIds.has(leaf.ptyId))
      ) {
        ptyIds.add(leaf.ptyId)
      }
    }
    for (const pty of this.host.getGraph().ptysById.values()) {
      if (
        pty.worktreeId === worktreeId &&
        pty.connected &&
        (!freshPtyIds || freshPtyIds.has(pty.ptyId))
      ) {
        ptyIds.add(pty.ptyId)
      }
    }
    return ptyIds
  }

  async hasTerminalsForWorktree(worktreeSelector: string): Promise<boolean> {
    const graphEpoch = this.host.captureReadyGraphEpoch()
    const worktree = await this.host.resolveWorktreeSelector(worktreeSelector)
    this.host.assertStableReadyGraph(graphEpoch)
    for (const leaf of this.host.getGraph().leaves.values()) {
      if (leaf.worktreeId === worktree.id && leaf.ptyId) {
        return true
      }
    }
    for (const pty of this.host.getGraph().ptysById.values()) {
      if (pty.worktreeId === worktree.id && pty.connected) {
        return true
      }
    }
    return false
  }
}
