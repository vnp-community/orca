// Why: locks in the fix for specs/agent/api/gaps-and-findings.md #5 — see
// github-auth.test.ts for the full rationale (gitlab.startAuthLogin/
// revokeAuth had the identical bug).
import { describe, expect, it, vi, afterEach } from 'vitest'
import { GITLAB_AUTH_METHODS } from './gitlab-auth'
import { registerRemotePtyProvider, unregisterRemotePtyProvider } from '../../../ipc/pty'
import type { IPtyProvider, PtySpawnOptions } from '../../../providers/types'
import type { RpcContext } from '../core'

const startAuthLogin = GITLAB_AUTH_METHODS.find((m) => m.name === 'gitlab.startAuthLogin')!
const revokeAuth = GITLAB_AUTH_METHODS.find((m) => m.name === 'gitlab.revokeAuth')!

function fakeProvider(spawn: (opts: PtySpawnOptions) => Promise<{ id: string }>): IPtyProvider {
  return { spawn } as unknown as IPtyProvider
}

function fakeCtx(overrides: Partial<RpcContext> = {}): RpcContext {
  return { devServerManager: {} as never, ...overrides } as RpcContext
}

afterEach(() => {
  unregisterRemotePtyProvider('dev-1')
})

describe('gitlab.startAuthLogin', () => {
  it('spawns via the registered IPtyProvider with a single shell-quoted command', async () => {
    const spawn = vi.fn().mockResolvedValue({ id: 'ssh:dev-1@@pty-1' })
    registerRemotePtyProvider('dev-1', fakeProvider(spawn))

    const result = await startAuthLogin.handler(
      { devServerId: 'dev-1' },
      fakeCtx({ userId: 'user-42' })
    )

    expect(spawn).toHaveBeenCalledWith(
      expect.objectContaining({
        command: "'glab' 'auth' 'login'",
        commandDelivery: 'provider',
        userId: 'user-42'
      })
    )
    expect(result).toEqual({ ptyId: 'ssh:dev-1@@pty-1', devServerId: 'dev-1' })
  })

  it('shell-quotes a --hostname value for self-hosted GitLab instances', async () => {
    const spawn = vi.fn().mockResolvedValue({ id: 'pty-1' })
    registerRemotePtyProvider('dev-1', fakeProvider(spawn))

    await startAuthLogin.handler(
      { devServerId: 'dev-1', host: 'gitlab.internal.example.com' },
      fakeCtx()
    )

    const command = spawn.mock.calls[0]![0].command as string
    expect(command).toBe("'glab' 'auth' 'login' '--hostname' 'gitlab.internal.example.com'")
  })

  it('throws a clear error when the dev server has no registered PTY provider', async () => {
    await expect(
      startAuthLogin.handler({ devServerId: 'not-connected' }, fakeCtx())
    ).rejects.toThrow(/not connected/)
  })
})

describe('gitlab.revokeAuth', () => {
  it('spawns "glab auth logout" via the registered IPtyProvider', async () => {
    const spawn = vi.fn().mockResolvedValue({ id: 'pty-2' })
    registerRemotePtyProvider('dev-1', fakeProvider(spawn))

    const result = await revokeAuth.handler({ devServerId: 'dev-1' }, fakeCtx({ userId: 'user-42' }))

    expect(spawn).toHaveBeenCalledWith(
      expect.objectContaining({ command: "'glab' 'auth' 'logout'", commandDelivery: 'provider' })
    )
    expect(result).toEqual({ ptyId: 'pty-2', devServerId: 'dev-1' })
  })
})
