// frontend/src/main/runtime/orca-runtime-pty-foreground-agent-refresh.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-060): PTY foreground-process
// agent-ownership refresh commands extracted from OrcaRuntimeService via
// the composition pattern. Method-body dependency analysis (TASK-BIGFILE-054,
// corrected per its "Bài học phương pháp" note) confirmed 4 real host
// dependencies: touchMobileSessionSnapshotsForPty, getPtyController,
// getGraph, and the recognizeAgentProcess free function (imported fresh,
// stays in orca-runtime.ts too since it's used elsewhere there).
import { recognizeAgentProcess } from '../../shared/agent-process-recognition'
import type { PtyForegroundAgentRefresh } from './orca-runtime'
import type { RuntimeGraphStore } from './orca-runtime-graph-store'
import type { RuntimePtyController } from './orca-runtime-types'

export type RuntimePtyForegroundAgentRefreshCommandHost = {
  getGraph(): RuntimeGraphStore
  getPtyController(): RuntimePtyController | null
  touchMobileSessionSnapshotsForPty(ptyId: string, options?: { immediate?: boolean }): void
}

export class RuntimePtyForegroundAgentRefreshCommands {
  private readonly ptyForegroundAgentRefreshes = new Map<string, PtyForegroundAgentRefresh>()
  private readonly ptyDelayedForegroundSnapshotTitleObservations = new Map<string, number>()

  constructor(private readonly host: RuntimePtyForegroundAgentRefreshCommandHost) {}

  // Why: also called from OrcaRuntimeService outside this domain (session-startup resume path) — public, not private.
  refreshPtyForegroundAgent(ptyId: string): void {
    void this.refreshPtyForegroundAgentFromController(ptyId)
  }

  // Why: also called from OrcaRuntimeService outside this domain (pty-title-tracker's applyTrackedPtyTitle, not moved — see TASK-BIGFILE-054) — public, not private.
  getPendingForegroundAgentRefreshForTitle(
    ptyId: string,
    titleObservedAt: number
  ): Promise<boolean> | undefined {
    if (!this.ptyForegroundAgentRefreshes.has(ptyId)) {
      return undefined
    }
    return this.refreshPtyForegroundAgentFromController(ptyId, {
      afterTitleObservation: titleObservedAt
    })
  }

  // Why: also called from OrcaRuntimeService outside this domain (pty-title-tracker's applyTrackedPtyTitle, not moved — see TASK-BIGFILE-054) — public, not private.
  delayPtyBackedMobileSnapshotForForegroundAgent(
    ptyId: string,
    titleObservedAt: number,
    foregroundRefresh: Promise<boolean>
  ): void {
    this.ptyDelayedForegroundSnapshotTitleObservations.set(ptyId, titleObservedAt)
    void foregroundRefresh.then((foregroundAgentChanged) => {
      if (this.ptyDelayedForegroundSnapshotTitleObservations.get(ptyId) !== titleObservedAt) {
        return
      }
      this.ptyDelayedForegroundSnapshotTitleObservations.delete(ptyId)
      if (!foregroundAgentChanged) {
        this.host.touchMobileSessionSnapshotsForPty(ptyId)
      }
    })
  }

  /**
   * Deduplicates and manages in-flight foreground agent refresh queries
   * for a specific PTY.
   */
  // Why: also called from OrcaRuntimeService outside this domain (pty-title-tracker's applyTrackedPtyTitle, not moved — see TASK-BIGFILE-054) — public, not private.
  refreshPtyForegroundAgentFromController(
    ptyId: string,
    options: { afterTitleObservation?: number } = {}
  ): Promise<boolean> {
    const startedAfterTitleObservation = options.afterTitleObservation ?? 0
    const pendingRefresh = this.ptyForegroundAgentRefreshes.get(ptyId)
    if (pendingRefresh) {
      pendingRefresh.requestedAfterTitleObservation = Math.max(
        pendingRefresh.requestedAfterTitleObservation,
        startedAfterTitleObservation
      )
      return pendingRefresh.promise
    }
    const entry: PtyForegroundAgentRefresh = {
      promise: Promise.resolve(false),
      startedAfterTitleObservation,
      requestedAfterTitleObservation: startedAfterTitleObservation
    }
    const refresh = (async (): Promise<boolean> => {
      while (true) {
        entry.startedAfterTitleObservation = entry.requestedAfterTitleObservation
        const foregroundAgentChanged = await this.loadPtyForegroundAgentFromController(ptyId)
        if (
          foregroundAgentChanged ||
          entry.requestedAfterTitleObservation <= entry.startedAfterTitleObservation
        ) {
          return foregroundAgentChanged
        }
      }
    })().finally(() => {
      if (this.ptyForegroundAgentRefreshes.get(ptyId) === entry) {
        this.ptyForegroundAgentRefreshes.delete(ptyId)
      }
    })
    entry.promise = refresh
    this.ptyForegroundAgentRefreshes.set(ptyId, entry)
    return refresh
  }

  /**
   * Queries the PTY controller for the active foreground process, identifies if it
   * is a recognized agent, and updates the PTY's foreground agent state if changed.
   */
  private async loadPtyForegroundAgentFromController(ptyId: string): Promise<boolean> {
    const ptyController = this.host.getPtyController()
    if (!ptyController) {
      return false
    }
    const pty = this.host.getGraph().ptysById.get(ptyId)
    if (!pty?.connected) {
      return false
    }
    // Why: foregroundAgent is only consulted as the owner fallback when
    // launchAgent is unknown, so a known launchAgent makes the relay
    // getForegroundProcess round-trip pure waste (covers all launched agents).
    if (pty.launchAgent) {
      return false
    }
    let foregroundProcess: string | null
    try {
      foregroundProcess = await ptyController.getForegroundProcess(ptyId)
    } catch {
      return false
    }
    const foregroundAgent = foregroundProcess
      ? (recognizeAgentProcess(foregroundProcess)?.agent ?? null)
      : null
    if (pty.foregroundAgent === foregroundAgent) {
      return false
    }
    pty.foregroundAgent = foregroundAgent
    this.host.touchMobileSessionSnapshotsForPty(ptyId)
    return true
  }
}
