import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import { requiredString } from '../schemas'

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
      const relay = ctx.devServerManager.getRelay(params.devServerId)
      if (!relay) {
        throw new Error(
          `Dev server '${params.devServerId}' relay is not connected. ` +
          `Connect to the dev server first.`
        )
      }

      const args = ['auth', 'login']
      if (params.host) {
        args.push('--hostname', params.host)
      }

      // FIX BUG-BE-HLD-005: forward the authenticated user so the Agent can
      // namespace GLAB_CONFIG_DIR per user (see external-api-connector.ts buildGlabEnv).
      const ptyId = await relay.call<string>('pty.spawn', {
        command: 'glab',
        args,
        env: {},
        userId: ctx.userId,
        cols: 120,
        rows: 30
      })

      return { ptyId, devServerId: params.devServerId }
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
      const relay = ctx.devServerManager.getRelay(params.devServerId)
      if (!relay) {
        throw new Error(
          `Dev server '${params.devServerId}' relay is not connected.`
        )
      }

      const args = params.host
        ? ['auth', 'logout', '--hostname', params.host]
        : ['auth', 'logout']

      // FIX BUG-BE-HLD-005: same per-user GLAB_CONFIG_DIR namespacing as startAuthLogin.
      const ptyId = await relay.call<string>('pty.spawn', {
        command: 'glab',
        args,
        env: {},
        userId: ctx.userId,
        cols: 80,
        rows: 10
      })

      return { ptyId, devServerId: params.devServerId }
    }
  })
]
