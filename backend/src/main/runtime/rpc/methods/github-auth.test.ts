// Why: locks in the fix for specs/agent/api/gaps-and-findings.md #5 —
// github.startAuthLogin/revokeAuth used to call the raw, connection-type-
// unaware relay.call('pty.spawn', {command:'gh', args:[...]}), which threw
// MethodNotFound on direct-websocket/relay-websocket dev servers (the
// default mode) and silently dropped `args` even on relay-ssh (the one mode
// where 'pty.spawn' exists at all).
import { describe, expect, it, vi, afterEach } from 'vitest'
import { GITHUB_AUTH_METHODS } from './github-auth'
import { registerRemotePtyProvider, unregisterRemotePtyProvider } from '../../../ipc/pty'
import type { IPtyProvider, PtySpawnOptions } from '../../../providers/types'
import type { RpcContext } from '../core'

const startAuthLogin = GITHUB_AUTH_METHODS.find((m) => m.name === 'github.startAuthLogin')!
const revokeAuth = GITHUB_AUTH_METHODS.find((m) => m.name === 'github.revokeAuth')!

function fakeProvider(spawn: (opts: PtySpawnOptions) => Promise<{ id: string }>): IPtyProvider {
  return { spawn } as unknown as IPtyProvider
}

function fakeCtx(overrides: Partial<RpcContext> = {}): RpcContext {
  return { devServerManager: {} as never, ...overrides } as RpcContext
}

afterEach(() => {
  unregisterRemotePtyProvider('dev-1')
})

describe('github.startAuthLogin', () => {
  it('spawns via the registered IPtyProvider with a single shell-quoted command', async () => {
    const spawn = vi.fn().mockResolvedValue({ id: 'ssh:dev-1@@pty-1' })
    registerRemotePtyProvider('dev-1', fakeProvider(spawn))

    const result = await startAuthLogin.handler(
      { devServerId: 'dev-1' },
      fakeCtx({ userId: 'user-42' })
    )

    expect(spawn).toHaveBeenCalledWith(
      expect.objectContaining({
        command: "'gh' 'auth' 'login'",
        commandDelivery: 'provider',
        userId: 'user-42'
      })
    )
    expect(result).toEqual({ ptyId: 'ssh:dev-1@@pty-1', devServerId: 'dev-1' })
  })

  it('shell-quotes a --hostname value so it cannot break out of the command', async () => {
    const spawn = vi.fn().mockResolvedValue({ id: 'pty-1' })
    registerRemotePtyProvider('dev-1', fakeProvider(spawn))

    await startAuthLogin.handler(
      { devServerId: 'dev-1', host: "evil'; rm -rf /" },
      fakeCtx()
    )

    const command = spawn.mock.calls[0]![0].command as string
    expect(command).toBe("'gh' 'auth' 'login' '--hostname' 'evil'\\''; rm -rf /'")
  })

  it('throws a clear error when the dev server has no registered PTY provider', async () => {
    await expect(
      startAuthLogin.handler({ devServerId: 'not-connected' }, fakeCtx())
    ).rejects.toThrow(/not connected/)
  })

  it('throws when devServerManager is unavailable (Electron/local mode)', async () => {
    await expect(
      startAuthLogin.handler({ devServerId: 'dev-1' }, fakeCtx({ devServerManager: undefined }))
    ).rejects.toThrow(/Web Server mode/)
  })
})

describe('github.revokeAuth', () => {
  it('spawns "gh auth logout" via the registered IPtyProvider', async () => {
    const spawn = vi.fn().mockResolvedValue({ id: 'pty-2' })
    registerRemotePtyProvider('dev-1', fakeProvider(spawn))

    const result = await revokeAuth.handler({ devServerId: 'dev-1' }, fakeCtx({ userId: 'user-42' }))

    expect(spawn).toHaveBeenCalledWith(
      expect.objectContaining({ command: "'gh' 'auth' 'logout'", commandDelivery: 'provider' })
    )
    expect(result).toEqual({ ptyId: 'pty-2', devServerId: 'dev-1' })
  })
})
