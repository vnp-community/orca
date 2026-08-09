/**
 * server-pty-controller.ts — RuntimePtyController for the pure-Node
 * multi-user server (server-bootstrap.ts), where every repo is Dev-Server- or
 * SSH-bound and there is no local-shell concept at all.
 *
 * Why not reuse src/main/ipc/pty.ts's desktop controller: that implementation
 * is deeply coupled to Electron's `app` module, Claude-account switching, and
 * Windows-shell resolution for its local-PTY branch. This controller only
 * implements the remote (connectionId-bound) branch, delegating to the same
 * registerRemotePtyProvider/getRemotePtyProvider registry (pty.ts) that
 * SshPtyProvider and DevServerPtyProvider already register into today.
 *
 * Id namespacing: DevServerPtyProvider (and SshPtyProvider) already return
 * globally-unique, self-describing ids (`ssh:<connectionId>@@<relayId>`, see
 * ssh-pty-id.ts) from spawn() and embed the same id in onData/onExit/onReplay
 * payloads — this controller never needs to encode/decode ids itself, only
 * track which connectionId owns which id for provider lookup.
 */
import type { RuntimePtyController } from './orca-runtime'
import type { OrcaRuntimeService } from './orca-runtime'
import type { IPtyProvider } from '../providers/types'
import { getRemotePtyProvider } from '../ipc/pty'
import { parseAppSshPtyId } from '../providers/ssh-pty-id'
import { makePaneKey } from '../../shared/stable-pane-id'

export type ServerPtyController = RuntimePtyController & {
  /**
   * Called by the Dev Server provider's onExit relay (dev-server-provider-
   * lifecycle.ts) so kill()/stopAndWait()'s own synthetic exit doesn't
   * double-fire runtime.onPtyExit for a PTY that already delivered a real one.
   */
  notifyProviderExit(ptyId: string, exitCode: number): void
}

export function createServerPtyController(runtime: OrcaRuntimeService): ServerPtyController {
  const ptyOwnership = new Map<string, string>()
  const ptySizes = new Map<string, { cols: number; rows: number }>()
  const exitedPtyIds = new Set<string>()

  function resolveProvider(ptyId: string): { connectionId: string; provider: IPtyProvider } {
    const connectionId = ptyOwnership.get(ptyId) ?? parseAppSshPtyId(ptyId)?.connectionId
    if (!connectionId) {
      throw new Error(`No connection known for PTY "${ptyId}"`)
    }
    const provider = getRemotePtyProvider(connectionId)
    if (!provider) {
      throw new Error(`No PTY provider registered for connection "${connectionId}"`)
    }
    return { connectionId, provider }
  }

  function notifyProviderExit(ptyId: string, exitCode: number): void {
    if (exitedPtyIds.has(ptyId)) {
      return
    }
    exitedPtyIds.add(ptyId)
    ptyOwnership.delete(ptyId)
    ptySizes.delete(ptyId)
    runtime.onPtyExit(ptyId, exitCode)
  }

  return {
    notifyProviderExit,

    spawn: async (args) => {
      if (!args.connectionId) {
        throw new Error(
          'This server only supports Dev Server- or SSH-backed terminals (no local shell).'
        )
      }
      const provider = getRemotePtyProvider(args.connectionId)
      if (!provider) {
        throw new Error(`No PTY provider registered for connection "${args.connectionId}"`)
      }
      const paneKey =
        args.tabId && args.leafId ? makePaneKey(args.tabId, args.leafId) : undefined
      const result = await provider.spawn({
        cols: args.cols,
        rows: args.rows,
        cwd: args.cwd,
        command: args.command,
        startupCommandDelivery: args.startupCommandDelivery,
        env: args.env,
        envToDelete: args.envToDelete,
        worktreeId: args.worktreeId,
        paneKey,
        tabId: args.tabId,
        sessionId: args.sessionId,
        isNewSession: args.sessionId === undefined ? true : undefined
      })
      ptyOwnership.set(result.id, args.connectionId)
      ptySizes.set(result.id, { cols: args.cols, rows: args.rows })
      return { id: result.id }
    },

    write: (ptyId, data) => {
      try {
        resolveProvider(ptyId).provider.write(ptyId, data)
        return true
      } catch {
        return false
      }
    },

    resize: (ptyId, cols, rows) => {
      try {
        resolveProvider(ptyId).provider.resize(ptyId, cols, rows)
        ptySizes.set(ptyId, { cols, rows })
        return true
      } catch {
        return false
      }
    },

    getSize: (ptyId) => ptySizes.get(ptyId) ?? null,

    kill: (ptyId) => {
      let resolved: { connectionId: string; provider: IPtyProvider }
      try {
        resolved = resolveProvider(ptyId)
      } catch {
        return false
      }
      void resolved.provider
        .shutdown(ptyId, { immediate: false })
        .then(() => notifyProviderExit(ptyId, -1))
        .catch(() => notifyProviderExit(ptyId, -1))
      return true
    },

    stopAndWait: async (ptyId, opts) => {
      let resolved: { connectionId: string; provider: IPtyProvider }
      try {
        resolved = resolveProvider(ptyId)
      } catch {
        return false
      }
      try {
        await resolved.provider.shutdown(ptyId, {
          immediate: true,
          keepHistory: opts?.keepHistory ?? false
        })
      } catch {
        // best effort — still report exit below
      }
      notifyProviderExit(ptyId, -1)
      return true
    },

    getForegroundProcess: async (ptyId) => {
      try {
        return await resolveProvider(ptyId).provider.getForegroundProcess(ptyId)
      } catch {
        return null
      }
    },

    confirmForegroundProcess: async (ptyId) => {
      try {
        return (await resolveProvider(ptyId).provider.confirmForegroundProcess?.(ptyId)) ?? null
      } catch {
        return null
      }
    },

    getCwd: async (ptyId) => {
      try {
        return (await resolveProvider(ptyId).provider.getCwd(ptyId)) || null
      } catch {
        return null
      }
    },

    hasChildProcesses: async (ptyId) => {
      try {
        return await resolveProvider(ptyId).provider.hasChildProcesses(ptyId)
      } catch {
        return false
      }
    },

    clearBuffer: async (ptyId) => {
      try {
        await resolveProvider(ptyId).provider.clearBuffer(ptyId)
      } catch {
        // best effort
      }
    }
  }
}
