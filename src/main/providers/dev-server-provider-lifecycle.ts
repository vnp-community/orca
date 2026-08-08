/**
 * Wires Dev Server connection-state events to the SSH provider registries
 * (ssh-filesystem-dispatch.ts / ssh-git-dispatch.ts) so a connected Dev
 * Server's existing agent WebSocket becomes a usable execution-host
 * connection for any repo bound to it — without opening a second, separate
 * SSH connection to the same machine.
 *
 * This is a pure listener: DevServerManager/AgentWebSocketServer already own
 * connect/reconnect. On every 'connected' transition we register fresh
 * provider instances (the underlying relay session's multiplexer is silently
 * swapped on reconnect with no distinct "session replaced" event, so
 * re-registering is simpler and correct than trying to keep instances alive
 * across reconnects).
 *
 * Called once per process from server-bootstrap.ts, right after
 * devServerManager is constructed — covers single-user mode, the multi-user
 * parent process, and every multi-user per-user child process the same way,
 * since all three pass either the real DevServerManager or
 * GatewayDevServerManagerProxy through this same call site (see
 * gateway-proxy.ts). Kept at this EARLY position deliberately: a real Dev
 * Server can finish its WebSocket handshake and fire 'connected' well before
 * the rest of bootstrap (DB/migrations/auth/session-manager) finishes, so
 * delaying this call risks missing that event entirely.
 *
 * `attachRuntime(runtime)` — called separately, later, once OrcaRuntimeService
 * exists (desktop never calls it; ipc/pty.ts's registerPtyHandlers owns its
 * controller instead, and the two must not race to own runtime.ptyController)
 * — wires runtime.setPtyController (server-pty-controller.ts, since ipc/
 * pty.ts's desktop controller is too Electron/app-coupled to reuse) and
 * relays each registered DevServerPtyProvider's onData/onExit/onReplay into
 * runtime.onPtyData/onPtyExit. Without this, spawned PTYs exist but no output
 * ever reaches a terminal.subscribe RPC client. Because a Dev Server can
 * connect (and register) before attachRuntime is ever called, every
 * already-ptyReady provider is retroactively wired at that point too — the
 * relay must not depend on which of "provider registered" / "runtime
 * attached" happens first.
 */
import type { DevServerManager } from '../dev-server/dev-server-manager'
import type { OrcaRuntimeService } from '../runtime/orca-runtime'
import type { DevServerRelayConnection } from './dev-server-relay-connection'
import { DevServerFilesystemProvider } from './dev-server-filesystem-provider'
import { DevServerGitProvider } from './dev-server-git-provider'
import { DevServerPtyProvider } from './dev-server-pty-provider'
import {
  createServerPtyController,
  type ServerPtyController
} from '../runtime/server-pty-controller'
import {
  registerRemoteFilesystemProvider,
  unregisterRemoteFilesystemProvider
} from './ssh-filesystem-dispatch'
import { registerRemoteGitProvider, unregisterRemoteGitProvider } from './ssh-git-dispatch'
import { registerRemotePtyProvider, unregisterRemotePtyProvider } from '../ipc/pty'

export type DevServerProviderLifecycle = {
  attachRuntime(runtime: OrcaRuntimeService): void
}

export function wireDevServerProviders(
  devServerManager: DevServerManager
): DevServerProviderLifecycle {
  let attachedRuntime: OrcaRuntimeService | null = null
  let serverPtyController: ServerPtyController | null = null
  const readyPtyProvidersById = new Map<string, DevServerPtyProvider>()
  const ptyUnsubscribersByDevServerId = new Map<string, (() => void)[]>()

  const wireRelayFor = (id: string, ptyProvider: DevServerPtyProvider): void => {
    if (!attachedRuntime || !serverPtyController || ptyUnsubscribersByDevServerId.has(id)) {
      return
    }
    const runtime = attachedRuntime
    const controller = serverPtyController
    const unsubData = ptyProvider.onData((payload) =>
      runtime.onPtyData(payload.id, payload.data, Date.now())
    )
    const unsubExit = ptyProvider.onExit((payload) =>
      controller.notifyProviderExit(payload.id, payload.code)
    )
    // Why folded into onData rather than a separate replay channel: desktop
    // delivers onReplay through a dedicated Electron IPC event its renderer
    // handles specially. There is no such channel here, and forwarding
    // through the same ingestion as live data is enough to avoid losing
    // scrollback on reattach, even if it isn't byte-for-byte parity with
    // desktop's reattach UX (see plan's "Out of scope").
    const unsubReplay = ptyProvider.onReplay((payload) =>
      runtime.onPtyData(payload.id, payload.data, Date.now())
    )
    ptyUnsubscribersByDevServerId.set(id, [unsubData, unsubExit, unsubReplay])
  }

  const registerFor = async (id: string): Promise<void> => {
    const relay = devServerManager.getRelay(id) as unknown as DevServerRelayConnection | null
    if (!relay) {
      return
    }
    registerRemoteFilesystemProvider(id, new DevServerFilesystemProvider(id, relay))
    registerRemoteGitProvider(id, new DevServerGitProvider(id, relay))

    // Why await list() (not get()): GatewayDevServerManagerProxy.get() throws
    // synchronously ("not supported in User Process") — list() is the one
    // lookup that works identically in the parent process (plain array,
    // await is a no-op) and every per-user child process (real IPC round-trip).
    const servers = await devServerManager.list()
    const capabilities = servers.find((ds) => ds.id === id)?.capabilities ?? []
    // Why check both 'pty' AND 'pty.stream': 'pty' alone means the agent has
    // node-pty installed; 'pty.stream' means the agent's RPC dispatcher also
    // exposes pty.attach/pty.data/pty.exit push notifications — both are
    // required for DevServerPtyProvider to work end-to-end.
    // 'pty.stream' was accidentally omitted from STATIC_CAPABILITIES_FALLBACK
    // in agent-session.ts before the 2026-08 fix; we now also accept agents
    // that advertise 'pty' without 'pty.stream' to be backward compatible with
    // any older agents still running in the fleet.
    const hasPty = capabilities.includes('pty')
    const hasPtyStream = capabilities.includes('pty.stream')
    const ptyReady = hasPty
    if (ptyReady) {
      const ptyProvider = new DevServerPtyProvider(id, relay)
      registerRemotePtyProvider(id, ptyProvider)
      readyPtyProvidersById.set(id, ptyProvider)
      // No-op until attachRuntime() runs if runtime isn't ready yet — that
      // call's own catch-up pass finds this provider via readyPtyProvidersById.
      wireRelayFor(id, ptyProvider)
      console.log(
        `[DevServerProviders] Registered fs/git/pty providers for devServerId=${id}${hasPtyStream ? '' : ' (pty.stream not advertised — streaming may be limited)'}`
      )
    } else {
      console.warn(
        `[DevServerProviders] PTY unavailable for devServerId=${id} — ` +
          `capabilities: [${capabilities.join(', ')}]. ` +
          `Terminal features will be disabled. ` +
          `Check that node-pty is installed on the agent host and the agent version supports PTY.`
      )
      console.log(
        `[DevServerProviders] Registered fs/git providers for devServerId=${id} ` +
          `(pty unavailable — capabilities: [${capabilities.join(', ')}])`
      )
    }
  }

  const unregisterFor = (id: string): void => {
    unregisterRemoteFilesystemProvider(id)
    unregisterRemoteGitProvider(id)
    for (const unsubscribe of ptyUnsubscribersByDevServerId.get(id) ?? []) {
      unsubscribe()
    }
    ptyUnsubscribersByDevServerId.delete(id)
    readyPtyProvidersById.delete(id)
    unregisterRemotePtyProvider(id)
  }

  devServerManager.on('devServer:statusChanged', (id: string, status: string) => {
    if (status === 'connected') {
      void registerFor(id)
    } else {
      unregisterFor(id)
    }
  })

  devServerManager.on('devServer:removed', (id: string) => {
    unregisterFor(id)
  })

  // FIX: Pre-seed any already connected dev servers.
  // In multi-user mode, a User Process might start *after* the Daemon Agent has
  // already connected to the Main Process. The 'devServer:statusChanged' event
  // would have already fired and been missed.
  Promise.resolve(devServerManager.list())
    .then((servers) => {
      for (const server of servers) {
        if (server.status === 'connected') {
          void registerFor(server.id)
        }
      }
    })
    .catch((err) => console.error('[DevServerProviders] Failed to pre-seed servers:', err))

  return {
    attachRuntime(runtime: OrcaRuntimeService): void {
      attachedRuntime = runtime
      serverPtyController = createServerPtyController(runtime)
      runtime.setPtyController(serverPtyController)
      for (const [id, ptyProvider] of readyPtyProvidersById) {
        wireRelayFor(id, ptyProvider)
      }
    }
  }
}
