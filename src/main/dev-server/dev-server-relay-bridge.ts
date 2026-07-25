// ─── DevServerRelayBridge ─────────────────────────────────────────────────────
// Wraps the existing SSH relay deploy infrastructure to provide a clean
// interface for DevServerManager. Only relay-ssh mode is implemented in Phase 1.
// relay-websocket / direct-websocket are planned for Phase 2.

import type { SshConnectionManager } from '../ssh/ssh-connection-manager'
import type { PersistedDevServer } from '../../shared/dev-server-types'
import { deployAndLaunchRelay } from '../ssh/ssh-relay-deploy'
import type { RelayPlatform } from '../ssh/relay-protocol'
import type { SshChannelMultiplexer } from '../ssh/ssh-channel-multiplexer'
import type { AgentDetectionCommand } from '../../shared/agent-detection-commands'

export type RelayHandshakeInfo = {
  platform: NodeJS.Platform
  arch: string
  nodeVersion: string
  relayVersion: string
}

/**
 * Maps a RelayPlatform string (e.g. 'darwin-arm64') to NodeJS.Platform
 * and arch string.
 */
function parseRelayPlatform(relayPlatform: RelayPlatform): {
  platform: NodeJS.Platform
  arch: string
} {
  const [os, arch] = relayPlatform.split('-') as [string, string]
  const platform: NodeJS.Platform =
    os === 'linux' ? 'linux' : os === 'darwin' ? 'darwin' : 'win32'
  return { platform, arch: arch ?? 'x64' }
}

export class DevServerRelayBridge {
  /** The active relay multiplexer. Exposed so IPC handlers can forward relay calls. */
  session: SshChannelMultiplexer | null = null

  constructor(
    private config: PersistedDevServer,
    private sshManager: SshConnectionManager
  ) {}

  async connect(opts: { testOnly?: boolean } = {}): Promise<RelayHandshakeInfo> {
    if (this.config.connectionType === 'relay-ssh') {
      const sshTargetId = this.config.sshTargetId
      if (!sshTargetId) {
        throw new Error(
          `DevServer '${this.config.name}' (${this.config.id}) has no sshTargetId set`
        )
      }

      const conn = this.sshManager.getConnection(sshTargetId)
      if (!conn) {
        throw new Error(
          `SSH connection for target '${sshTargetId}' not found. ` +
            `Connect to the SSH target before connecting the dev server.`
        )
      }

      // Deploy (or reuse) the relay on the remote host.
      // deployAndLaunchRelay returns a RelayDeployResult with transport + platform.
      const result = await deployAndLaunchRelay(conn, undefined, undefined, undefined)

      const { platform: nodePlatform, arch } = parseRelayPlatform(result.platform)

      // Store transport as session for downstream relay calls (TASK-013+).
      // The actual session object from the existing infrastructure is the transport.
      this.session = result.transport

      // Close immediately if this is a test-only probe.
      if (opts.testOnly) {
        await this.disconnect()
      }

      return {
        platform: nodePlatform,
        arch,
        // nodeVersion and relayVersion are not part of RelayDeployResult yet
        // (extended in TASK-006). Use placeholder until handshake extension lands.
        nodeVersion: 'unknown',
        relayVersion: 'unknown'
      }
    }

    // relay-websocket / direct-websocket: Phase 2 — not yet implemented
    throw new Error(
      `Connection type '${this.config.connectionType}' is not yet implemented. ` +
        `Only 'relay-ssh' is supported in Phase 1.`
    )
  }

  async disconnect(): Promise<void> {
    if (this.session && typeof this.session.close === 'function') {
      await this.session.close()
    } else if (this.session && typeof this.session.destroy === 'function') {
      this.session.destroy()
    }
    this.session = null
  }

  /**
   * Forward agent detection to the relay process.
   * The relay runs the probe commands and returns which agents are installed
   * along with the dev server's platform.
   *
   * @throws Error('Not connected') when the relay session is not established.
   */
  async detectAgents(commands: AgentDetectionCommand[]): Promise<{
    agents: string[]
    platform: NodeJS.Platform
  }> {
    if (!this.session) throw new Error('Not connected')

    // Timeout: agent detection is bounded to 15 seconds (relay probes PATH).
    const result = await this.callWithTimeout<{
      agents: string[]
      platform: NodeJS.Platform
    }>('preflight.detectAgents', { commands }, 15_000)
    return { agents: result.agents, platform: result.platform }
  }

  /**
   * Forward an arbitrary relay RPC call.
   * Used by onboarding-ipc handlers that need to invoke relay methods
   * (preflight.check, preflight.setGitIdentity, preflight.detectGhosttyConfig, etc.)
   * without exposing the raw SshChannelMultiplexer session.
   *
   * @throws Error('Not connected') when the relay session is not established.
   */
  async call<T = unknown>(
    method: string,
    params: Record<string, unknown> = {},
    timeoutMs = 30_000
  ): Promise<T> {
    if (!this.session) throw new Error('Not connected')
    return this.callWithTimeout<T>(method, params, timeoutMs)
  }


  /**
   * Wraps SshChannelMultiplexer.request() with an explicit timeout guard.
   * Why: the multiplexer has its own 30s default but agent detection should fail
   * faster (15s) so the UI remains responsive if the relay stalls.
   */
  private async callWithTimeout<T>(
    method: string,
    params: Record<string, unknown>,
    timeoutMs: number
  ): Promise<T> {
    return new Promise<T>((resolve, reject) => {
      const timer = setTimeout(
        () => reject(new Error(`Relay call '${method}' timed out after ${timeoutMs}ms`)),
        timeoutMs
      )
      this.session!.request(method, params)
        .then((result: unknown) => {
          clearTimeout(timer)
          resolve(result as T)
        })
        .catch((err: unknown) => {
          clearTimeout(timer)
          reject(err)
        })
    })
  }
}
