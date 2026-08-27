// Why: locks in the fix for specs/agent/api/gaps-and-findings.md #5 —
// DevServerPtyProvider.spawn() used to silently drop opts.command/
// commandDelivery/userId when forwarding to the agent's 'pty.create' RPC, so
// any commandDelivery:'provider' caller (AI-agent terminal launches, pane
// splits, gh/glab auth-login) got a bare shell on direct-websocket/
// relay-websocket dev servers instead of the intended command.
import { describe, expect, it, vi } from 'vitest'
import { DevServerPtyProvider } from './dev-server-pty-provider'
import type { DevServerRelayConnection } from './dev-server-relay-connection'

function makeMockRelay(result: Record<string, unknown>): {
  relay: DevServerRelayConnection
  call: ReturnType<typeof vi.fn>
} {
  const call = vi.fn().mockResolvedValue(result)
  return { relay: { call } as unknown as DevServerRelayConnection, call }
}

describe('DevServerPtyProvider.spawn', () => {
  it('forwards command, commandDelivery, and userId to pty.create', async () => {
    const { relay, call } = makeMockRelay({ id: 'agent-pty-1', cols: 120, rows: 30, cwd: '/home/u', shell: '/bin/bash' })
    const provider = new DevServerPtyProvider('dev-1', relay)

    await provider.spawn({
      cols: 120,
      rows: 30,
      command: 'gh auth login',
      commandDelivery: 'provider',
      userId: 'user-42'
    })

    expect(call).toHaveBeenCalledWith(
      'pty.create',
      expect.objectContaining({
        command: 'gh auth login',
        commandDelivery: 'provider',
        userId: 'user-42'
      })
    )
  })

  it('omits command/commandDelivery/userId when not provided (no behavior change for existing callers)', async () => {
    const { relay, call } = makeMockRelay({ id: 'agent-pty-2', cols: 80, rows: 24, cwd: '/home/u', shell: '/bin/bash' })
    const provider = new DevServerPtyProvider('dev-1', relay)

    await provider.spawn({ cols: 80, rows: 24 })

    const params = call.mock.calls[0]?.[1] as Record<string, unknown>
    expect(params).not.toHaveProperty('command')
    expect(params).not.toHaveProperty('commandDelivery')
    expect(params).not.toHaveProperty('userId')
  })

  it('forwards a non-empty envToDelete', async () => {
    const { relay, call } = makeMockRelay({ id: 'agent-pty-3', cols: 80, rows: 24, cwd: '/home/u', shell: '/bin/bash' })
    const provider = new DevServerPtyProvider('dev-1', relay)

    await provider.spawn({ cols: 80, rows: 24, envToDelete: ['FOO', 'BAR'] })

    expect(call).toHaveBeenCalledWith(
      'pty.create',
      expect.objectContaining({ envToDelete: ['FOO', 'BAR'] })
    )
  })
})
