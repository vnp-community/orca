import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import { requiredString } from '../schemas'
import { getRemotePtyProvider } from '../../../ipc/pty'
import { buildPosixShellCommand } from '../../../../shared/posix-shell-quote'

// Why: GitLab CLI (glab) auth works the same way as gh — interactive Device Flow
// in a PTY. Spawning on the Dev Server relay ensures credentials are stored
// in ~/.config/glab-cli on the remote machine, isolated per Linux user.
const StartAuthLoginParams = z.object({
  devServerId: requiredString('devServerId is required'),
  host: z.string().optional()          // For self-hosted GitLab instances
})

const RevokeAuthParams = z.object({
  devServerId: requiredString('devServerId is required'),
  host: z.string().optional()
})

export const GITLAB_AUTH_METHODS: RpcMethod[] = [
  /**
   * Spawn `glab auth login` in a remote PTY on the Dev Server relay.
   * Returns a ptyId that the client subscribes to for terminal output.
   *
   * Supports both gitlab.com (default) and self-hosted instances via `host`.
   * Credentials are stored in ~/.config/glab-cli on the dev machine.
   */
  defineMethod({
    name: 'gitlab.startAuthLogin',
    params: StartAuthLoginParams,
    handler: async (params, ctx) => {
      if (!ctx.devServerManager) {
        throw new Error(
          'gitlab.startAuthLogin requires Web Server mode (devServerManager not available)'
        )
      }
      // Why routed through the IPtyProvider registry instead of a raw
      // relay.call('pty.spawn', ...): 'pty.spawn' only exists on relay-ssh
      // dev servers — direct-websocket/relay-websocket (the default
      // connection mode) only register 'pty.create', which this bypassed
      // entirely, throwing MethodNotFound. getRemotePtyProvider() resolves
      // to whichever provider (SshPtyProvider/DevServerPtyProvider) the
      // connection type actually supports, and both now forward `command`/
      // `commandDelivery`/`userId` correctly. See
      // specs/agent/api/gaps-and-findings.md #5.
      const provider = getRemotePtyProvider(params.devServerId)
      if (!provider) {
        throw new Error(
          `Dev server '${params.devServerId}' relay is not connected. ` +
          `Connect to the dev server first.`
        )
      }

      // FIX BUG-BE-HLD-005: forward the authenticated user so the Agent can
      // namespace GLAB_CONFIG_DIR per user (see external-api-connector.ts buildGlabEnv).
      const result = await provider.spawn({
        cols: 120,
        rows: 30,
        command: buildPosixShellCommand([
          'glab', 'auth', 'login',
          ...(params.host ? ['--hostname', params.host] : [])
        ]),
        commandDelivery: 'provider',
        ...(ctx.userId ? { userId: ctx.userId } : {})
      })

      return { ptyId: result.id, devServerId: params.devServerId }
    }
  }),

  /**
   * Spawn `glab auth logout` in a remote PTY on the Dev Server relay.
   * Returns a ptyId for the client to observe the logout output.
   */
  defineMethod({
    name: 'gitlab.revokeAuth',
    params: RevokeAuthParams,
    handler: async (params, ctx) => {
      if (!ctx.devServerManager) {
        throw new Error(
          'gitlab.revokeAuth requires Web Server mode (devServerManager not available)'
        )
      }
      const provider = getRemotePtyProvider(params.devServerId)
      if (!provider) {
        throw new Error(
          `Dev server '${params.devServerId}' relay is not connected.`
        )
      }

      // FIX BUG-BE-HLD-005: same per-user GLAB_CONFIG_DIR namespacing as startAuthLogin.
      const result = await provider.spawn({
        cols: 80,
        rows: 10,
        command: buildPosixShellCommand([
          'glab', 'auth', 'logout',
          ...(params.host ? ['--hostname', params.host] : [])
        ]),
        commandDelivery: 'provider',
        ...(ctx.userId ? { userId: ctx.userId } : {})
      })

      return { ptyId: result.id, devServerId: params.devServerId }
    }
  })
]
