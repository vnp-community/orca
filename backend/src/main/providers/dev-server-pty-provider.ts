/**
 * DevServerPtyProvider — IPtyProvider backed by a Dev Server's existing agent
 * WebSocket connection (see dev-server-provider-lifecycle.ts).
 *
 * The dev-server agent's PTY surface (src/relay/pty-agent-bridge.ts) is
 * narrower than the SSH relay daemon's, but it does support reattach
 * (2026-08): pty.create/write/resize/destroy/scrollback/sendSignal/attach/
 * listProcesses, plus real-time pty.data/pty.exit/pty.replay push notifications
 * (src/relay/agent-rpc-dispatch.ts's makeNotifier). A dropped WebSocket does
 * not kill the shell — the agent arms a grace-period timer and only kills it
 * if no attach arrives in time (see scheduleGracePeriodCleanup). There is
 * still no process-tree inspection, live-cwd tracking, or cross-*process*-
 * restart persistence on the agent (a real agent process restart wipes its
 * in-memory PTY map regardless of any grace period).
 *
 * Methods with no honest agent equivalent return a documented approximation
 * or a safe default rather than throwing, since IPtyProvider requires all of
 * them — see the per-method comments below for what's approximated and why.
 *
 * Reuses the same `ssh:<connectionId>@@<relayPtyId>` app-facing id scheme as
 * SshPtyProvider (ssh-pty-id.ts) — the prefix is a historical wire-format
 * name, not a functional SSH-only gate: pty.ts's provider registry is keyed
 * generically by connectionId (real SSH target id or devServerId alike), the
 * same registry pattern already proven for fs/git providers.
 */
import type {
  IPtyProvider,
  PtyProcessInfo,
  PtySpawnOptions,
  PtySpawnResult
} from './types'
import type { DevServerRelayConnection } from './dev-server-relay-connection'
import { toAppSshPtyId, toRelaySshPtyId } from './ssh-pty-id'
import { SSH_SESSION_EXPIRED_ERROR } from './ssh-pty-provider'

type DataCallback = (payload: { id: string; data: string }) => void
type ReplayCallback = (payload: { id: string; data: string }) => void
type ExitCallback = (payload: { id: string; code: number }) => void

export class DevServerPtyProvider implements IPtyProvider {
  private readonly dataListeners = new Set<DataCallback>()
  private readonly replayListeners = new Set<ReplayCallback>()
  private readonly exitListeners = new Set<ExitCallback>()
  private readonly unsubscribeNotifications: (() => void) | null
  // Why: the agent has no getCwd RPC (would need shell-integration/OSC 7
  // tracking). Approximate with the spawn-time cwd, keyed by relay ptyId.
  private readonly initialCwdByRelayId = new Map<string, string>()

  constructor(
    private readonly devServerId: string,
    private readonly relay: DevServerRelayConnection
  ) {
    this.unsubscribeNotifications =
      this.relay.onNotification?.((method, params) => {
        const relayId = typeof params.id === 'string' ? params.id : null
        if (!relayId) {return}
        const appId = this.toAppPtyId(relayId)
        if (method === 'pty.data') {
          const data = typeof params.data === 'string' ? params.data : null
          if (data === null) {return}
          for (const cb of this.dataListeners) {cb({ id: appId, data })}
        } else if (method === 'pty.exit') {
          const exitCode = typeof params.exitCode === 'number' ? params.exitCode : 0
          this.initialCwdByRelayId.delete(relayId)
          for (const cb of this.exitListeners) {cb({ id: appId, code: exitCode })}
        } else if (method === 'pty.replay') {
          const data = typeof params.data === 'string' ? params.data : null
          if (data === null) {return}
          for (const cb of this.replayListeners) {cb({ id: appId, data })}
        }
      }) ?? null
  }

  dispose(): void {
    this.unsubscribeNotifications?.()
    this.dataListeners.clear()
    this.replayListeners.clear()
    this.exitListeners.clear()
  }

  getConnectionId(): string {
    return this.devServerId
  }

  private toRelayPtyId(id: string): string {
    return toRelaySshPtyId(this.devServerId, id)
  }

  private toAppPtyId(id: string): string {
    return toAppSshPtyId(this.devServerId, id)
  }

  async spawn(opts: PtySpawnOptions): Promise<PtySpawnResult> {
    if (opts.sessionId) {
      const relaySessionId = this.toRelayPtyId(opts.sessionId)
      try {
        const attachResult = await this.relay.call<{ replay?: string }>('pty.attach', {
          id: relaySessionId,
          cols: opts.cols,
          rows: opts.rows,
          suppressReplayNotification: true,
          ...(opts.paneKey ? { expectedPaneKey: opts.paneKey } : {}),
          ...(opts.tabId ? { expectedTabId: opts.tabId } : {})
        })
        return {
          id: this.toAppPtyId(relaySessionId),
          isReattach: true,
          ...(attachResult.replay ? { replay: attachResult.replay } : {})
        }
      } catch (err) {
        // Why: the agent's only failure mode for pty.attach is "not found"
        // (grace period elapsed, identity mismatch, or a genuine agent
        // process restart wiped its PTY map) — there is no narrower error to
        // distinguish, so any failure here means the same thing SSH's
        // isSshPtyNotFoundError check means: respawn fresh instead of retrying.
        const msg = err instanceof Error ? err.message : String(err)
        throw new Error(`${SSH_SESSION_EXPIRED_ERROR}: ${relaySessionId} (${msg})`)
      }
    }
    const result = await this.relay.call<{ id: string; cols: number; rows: number; cwd: string; shell: string }>(
      'pty.create',
      {
        cols: opts.cols,
        rows: opts.rows,
        ...(opts.cwd ? { cwd: opts.cwd } : {}),
        ...(opts.env ? { env: opts.env } : {}),
        ...(opts.shellOverride ? { shellOverride: opts.shellOverride } : {}),
        ...(opts.paneKey ? { paneKey: opts.paneKey } : {}),
        ...(opts.tabId ? { tabId: opts.tabId } : {})
      }
    )
    this.initialCwdByRelayId.set(result.id, result.cwd)
    return { id: this.toAppPtyId(result.id) }
  }

  async attach(id: string): Promise<void> {
    // Why: confirms the PTY is still alive agent-side and cancels its grace
    // timer if one is running — mirrors SshPtyProvider.attach(). Replay (if
    // any) arrives via the pty.replay notification handler above, not the
    // response, matching how a plain (non-spawn-embedded) attach behaves.
    await this.relay.call('pty.attach', { id: this.toRelayPtyId(id) })
  }

  write(id: string, data: string): void {
    void this.relay.call('pty.write', { id: this.toRelayPtyId(id), data }).catch(() => {})
  }

  resize(id: string, cols: number, rows: number): void {
    void this.relay.call('pty.resize', { id: this.toRelayPtyId(id), cols, rows }).catch(() => {})
  }

  async shutdown(id: string, opts: { immediate?: boolean; keepHistory?: boolean }): Promise<void> {
    // TEMP DIAG BUG-FE-PTY-001: capture the caller for the "created then
    // immediately destroyed" repro — stack trace pinpoints which orca-runtime
    // path decided to tear this PTY down seconds after spawn.
    console.error(
      `[DIAG BUG-FE-PTY-001] DevServerPtyProvider.shutdown() called id=${id} immediate=${opts.immediate} keepHistory=${opts.keepHistory}\n${new Error('shutdown call site').stack}`
    )
    const relayId = this.toRelayPtyId(id)
    await this.relay.call('pty.destroy', { id: relayId, graceful: !opts.immediate })
    this.initialCwdByRelayId.delete(relayId)
  }

  async sendSignal(id: string, signal: string): Promise<void> {
    await this.relay.call('pty.sendSignal', { id: this.toRelayPtyId(id), signal })
  }

  async getCwd(id: string): Promise<string> {
    return this.initialCwdByRelayId.get(this.toRelayPtyId(id)) ?? '~'
  }

  async getInitialCwd(id: string): Promise<string> {
    return this.initialCwdByRelayId.get(this.toRelayPtyId(id)) ?? '~'
  }

  async clearBuffer(): Promise<void> {
    // Why: no-op. The agent's own scrollback buffer is a small, self-trimming
    // 500-line ring (pty-agent-bridge.ts) with no clear RPC — harmless to skip.
  }

  acknowledgeDataEvent(): void {
    // Why: no-op. The agent's narrow surface has no flow-control notify (see
    // IPtyProvider's pauseProducer doc) — the caller's pending-output cap
    // still bounds memory without this.
  }

  async hasChildProcesses(): Promise<boolean> {
    // Why: no process-tree inspection RPC. False is the permissive default
    // (skip the "kill child process?" confirmation) rather than guessing.
    return false
  }

  async getForegroundProcess(): Promise<string | null> {
    return null
  }

  async serialize(): Promise<string> {
    // Why: no cross-restart session persistence exists — nothing to serialize.
    return JSON.stringify({ ptys: [] })
  }

  async revive(): Promise<void> {
    // Why: matches serialize() — there is never persisted state to revive.
  }

  async listProcesses(): Promise<PtyProcessInfo[]> {
    // Why: mirrors SshPtyProvider.listProcesses() — lets the backend's
    // liveness sweep (refreshPtyWorktreeRecordsFromController) detect a
    // Dev-Server-hosted PTY that died on its own, instead of leaving
    // session-tabs bookkeeping stuck reporting a dead ptyId as "ready"
    // forever (BUG-FE-PTY-001). A relay failure here must not be read as
    // "no PTYs exist" — surface it as a liveness-unknown empty result the
    // same way withTimeoutResult's !ok path already does for the caller.
    try {
      const result = await this.relay.call<{ id: string; cwd: string; title: string }[]>(
        'pty.listProcesses',
        {}
      )
      return result.map((session) => ({
        ...session,
        id: this.toAppPtyId(session.id)
      }))
    } catch {
      return []
    }
  }

  async getDefaultShell(): Promise<string> {
    // Why: the agent resolves the actual shell per-spawn (shellOverride →
    // $SHELL → /bin/sh); this is only a pre-spawn UI hint.
    return '/bin/bash'
  }

  async getProfiles(): Promise<{ name: string; path: string }[]> {
    return []
  }

  onData(callback: DataCallback): () => void {
    this.dataListeners.add(callback)
    return () => this.dataListeners.delete(callback)
  }

  onReplay(callback: ReplayCallback): () => void {
    this.replayListeners.add(callback)
    return () => this.replayListeners.delete(callback)
  }

  onExit(callback: ExitCallback): () => void {
    this.exitListeners.add(callback)
    return () => this.exitListeners.delete(callback)
  }
}
