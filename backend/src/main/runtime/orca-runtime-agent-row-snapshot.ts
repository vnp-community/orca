// frontend/src/main/runtime/orca-runtime-agent-row-snapshot.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-069): OSC 9999 agent-status
// payload retention/dispatch commands extracted from OrcaRuntimeService
// via the composition pattern. Small, clean cluster directly adjacent to
// pty-title-tracker/terminal-side-effects (TASK-BIGFILE-067/068) — same
// onPtyData-adjacent risk, same standing high-risk acceptance.
import type { ParsedAgentStatusPayload } from '../../shared/agent-status-types'
import type { ProcessedAgentStatusChunk } from '../../shared/agent-status-osc'
import type { RuntimeSyncedLeaf } from '../../shared/runtime-types'
import type { RuntimeAgentRowSnapshot, RuntimeLeafRecord } from './orca-runtime'
import type { RuntimeGraphStore } from './orca-runtime-graph-store'
import type { RuntimeTerminalAgentStatusEvent } from './orca-runtime-types'

export type RuntimeAgentRowSnapshotCommandHost = {
  getGraph(): RuntimeGraphStore
  getLeavesForPty(ptyId: string): RuntimeLeafRecord[]
  makeRuntimePaneKey(leaf: Pick<RuntimeSyncedLeaf, 'tabId' | 'leafId' | 'paneRuntimeId'>): string
  getOnTerminalAgentStatus(): ((event: RuntimeTerminalAgentStatusEvent) => void) | null
  getLatestAgentStatusByPaneKey(): Map<string, RuntimeAgentRowSnapshot>
}

export class RuntimeAgentRowSnapshotCommands {
  constructor(private readonly host: RuntimeAgentRowSnapshotCommandHost) {}

  // Why: also called from OrcaRuntimeService outside this domain (onPtyData) — public, not private.
  emitTerminalAgentStatusEvents(ptyId: string, chunk: ProcessedAgentStatusChunk): boolean {
    // Why: snapshot retention (for mobile worktree.ps) must run even when no
    // renderer listener is attached, so we don't early-return on a missing
    // onTerminalAgentStatus — only the per-target emit below is gated on it.
    if (chunk.payloads.length === 0) {
      return false
    }
    const targets = new Map<
      string,
      {
        source: 'mounted-leaf' | 'pty-record'
        paneKey: string
        tabId?: string
        worktreeId?: string
        connectionId?: string | null
      }
    >()
    const pty = this.host.getGraph().ptysById.get(ptyId)
    const connectionId = pty?.connectionId ?? null
    for (const leaf of this.host.getLeavesForPty(ptyId)) {
      const paneKey = this.host.makeRuntimePaneKey(leaf)
      targets.set(paneKey, {
        source: 'mounted-leaf',
        paneKey,
        tabId: leaf.tabId,
        worktreeId: leaf.worktreeId,
        connectionId
      })
    }
    if (targets.size === 0 && pty?.paneKey) {
      targets.set(pty.paneKey, {
        source: 'pty-record',
        paneKey: pty.paneKey,
        tabId: pty.tabId ?? undefined,
        worktreeId: pty.worktreeId,
        connectionId
      })
    }
    const onTerminalAgentStatus = this.host.getOnTerminalAgentStatus()
    let retainedChanged = false
    for (const payload of chunk.payloads) {
      for (const target of targets.values()) {
        retainedChanged =
          this.retainAgentRowSnapshot(
            ptyId,
            target.paneKey,
            target.worktreeId,
            target.tabId,
            payload
          ) || retainedChanged
        if (!onTerminalAgentStatus) {
          continue
        }
        try {
          onTerminalAgentStatus({
            ptyId,
            ...target,
            payload
          })
        } catch (err) {
          console.error('[runtime] terminal agent status listener threw', {
            ptyId,
            paneKey: target.paneKey,
            state: payload.state,
            agentType: payload.agentType,
            err
          })
        }
      }
    }
    return retainedChanged
  }

  private retainAgentRowSnapshot(
    ptyId: string,
    paneKey: string,
    worktreeId: string | undefined,
    tabId: string | undefined,
    payload: ParsedAgentStatusPayload
  ): boolean {
    const latestAgentStatusByPaneKey = this.host.getLatestAgentStatusByPaneKey()
    const now = Date.now()
    const previous = latestAgentStatusByPaneKey.get(paneKey)
    // Why: stateStartedAt must mark the transition into the current state, not
    // every within-state ping (tool/prompt updates keep the state but refresh
    // updatedAt) — mirrors AgentStatusEntry.stateStartedAt on the desktop side.
    const stateStartedAt =
      previous && previous.payload.state === payload.state ? previous.stateStartedAt : now
    latestAgentStatusByPaneKey.set(paneKey, {
      paneKey,
      ptyId,
      worktreeId,
      tabId,
      payload,
      stateStartedAt,
      updatedAt: now
    })
    // Client-visible change detection: snapshot republish is gated on this so
    // repeated same-state hook pings don't fan a rebuild out to every client.
    return (
      !previous ||
      previous.payload.state !== payload.state ||
      previous.payload.prompt !== payload.prompt ||
      (previous.payload.agentType ?? null) !== (payload.agentType ?? null) ||
      (previous.payload.toolName ?? null) !== (payload.toolName ?? null) ||
      (previous.payload.interactivePrompt ?? null) !== (payload.interactivePrompt ?? null) ||
      (previous.payload.interrupted ?? false) !== (payload.interrupted ?? false)
    )
  }

  // Why: also called from OrcaRuntimeService outside this domain (onPtyExit, pruneDisconnectedPtyRecords) — public, not private.
  clearAgentRowSnapshotsForPty(ptyId: string): void {
    const latestAgentStatusByPaneKey = this.host.getLatestAgentStatusByPaneKey()
    for (const [paneKey, snapshot] of latestAgentStatusByPaneKey) {
      if (snapshot.ptyId === ptyId) {
        latestAgentStatusByPaneKey.delete(paneKey)
      }
    }
  }
}
