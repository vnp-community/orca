import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { IPtyProvider } from '../providers/types'

const getRemotePtyProviderMock = vi.fn()
vi.mock('../ipc/pty', () => ({
  getRemotePtyProvider: (connectionId: string) => getRemotePtyProviderMock(connectionId)
}))

function makeFakeProvider(overrides: Partial<IPtyProvider> = {}): IPtyProvider {
  return {
    spawn: vi.fn().mockResolvedValue({ id: 'ssh:dev-01@@agent-pty-1' }),
    attach: vi.fn(),
    write: vi.fn(),
    resize: vi.fn(),
    shutdown: vi.fn().mockResolvedValue(undefined),
    sendSignal: vi.fn(),
    getCwd: vi.fn().mockResolvedValue('/srv/repo'),
    getInitialCwd: vi.fn().mockResolvedValue('/srv/repo'),
    clearBuffer: vi.fn().mockResolvedValue(undefined),
    acknowledgeDataEvent: vi.fn(),
    hasChildProcesses: vi.fn().mockResolvedValue(false),
    getForegroundProcess: vi.fn().mockResolvedValue(null),
    confirmForegroundProcess: vi.fn().mockResolvedValue(null),
    serialize: vi.fn(),
    revive: vi.fn(),
    listProcesses: vi.fn().mockResolvedValue([]),
    getDefaultShell: vi.fn(),
    getProfiles: vi.fn(),
    onData: vi.fn(() => () => {}),
    onReplay: vi.fn(() => () => {}),
    onExit: vi.fn(() => () => {}),
    ...overrides
  } as unknown as IPtyProvider
}

function makeFakeRuntime(): { onPtyData: ReturnType<typeof vi.fn>; onPtyExit: ReturnType<typeof vi.fn> } {
  return {
    onPtyData: vi.fn(),
    onPtyExit: vi.fn()
  }
}

describe('createServerPtyController', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('rejects a spawn with no connectionId (no local-shell concept in server mode)', async () => {
    const { createServerPtyController } = await import('./server-pty-controller')
    const runtime = makeFakeRuntime()
    const controller = createServerPtyController(runtime as never)

    await expect(controller.spawn!({ cols: 80, rows: 24, connectionId: null })).rejects.toThrow(
      /no local shell/i
    )
  })

  it('spawns through the registered remote provider and records ownership for the returned id', async () => {
    const provider = makeFakeProvider()
    getRemotePtyProviderMock.mockReturnValue(provider)
    const { createServerPtyController } = await import('./server-pty-controller')
    const runtime = makeFakeRuntime()
    const controller = createServerPtyController(runtime as never)

    const result = await controller.spawn!({
      cols: 80,
      rows: 24,
      connectionId: 'dev-01',
      worktreeId: 'wt-1',
      tabId: 'tab-1',
      leafId: '11111111-1111-4111-8111-111111111111'
    })

    expect(result.id).toBe('ssh:dev-01@@agent-pty-1')
    expect(provider.spawn).toHaveBeenCalledWith(
      expect.objectContaining({ cols: 80, rows: 24, worktreeId: 'wt-1', tabId: 'tab-1' })
    )

    // Ownership recorded: a subsequent write for this id routes without re-deriving connectionId.
    controller.write('ssh:dev-01@@agent-pty-1', 'echo hi\n')
    expect(provider.write).toHaveBeenCalledWith('ssh:dev-01@@agent-pty-1', 'echo hi\n')
    expect(getRemotePtyProviderMock).toHaveBeenCalledWith('dev-01')
  })

  it('resolves the connectionId for an unowned id by parsing its ssh:<connectionId>@@ prefix', async () => {
    const provider = makeFakeProvider()
    getRemotePtyProviderMock.mockReturnValue(provider)
    const { createServerPtyController } = await import('./server-pty-controller')
    const controller = createServerPtyController(makeFakeRuntime() as never)

    const ok = controller.resize!('ssh:dev-02@@agent-pty-9', 100, 40)

    expect(ok).toBe(true)
    expect(getRemotePtyProviderMock).toHaveBeenCalledWith('dev-02')
    expect(provider.resize).toHaveBeenCalledWith('ssh:dev-02@@agent-pty-9', 100, 40)
  })

  it('write/resize return false instead of throwing when no provider is registered', async () => {
    getRemotePtyProviderMock.mockReturnValue(undefined)
    const { createServerPtyController } = await import('./server-pty-controller')
    const controller = createServerPtyController(makeFakeRuntime() as never)

    expect(controller.write('ssh:dev-99@@agent-pty-1', 'x')).toBe(false)
    expect(controller.resize!('ssh:dev-99@@agent-pty-1', 80, 24)).toBe(false)
  })

  it('kill() returns false for a completely unknown id and does not throw', async () => {
    getRemotePtyProviderMock.mockReturnValue(undefined)
    const { createServerPtyController } = await import('./server-pty-controller')
    const controller = createServerPtyController(makeFakeRuntime() as never)

    expect(controller.kill('not-a-real-id')).toBe(false)
  })

  it('kill() shuts the PTY down and reports a synthetic exit exactly once', async () => {
    const provider = makeFakeProvider()
    getRemotePtyProviderMock.mockReturnValue(provider)
    const { createServerPtyController } = await import('./server-pty-controller')
    const runtime = makeFakeRuntime()
    const controller = createServerPtyController(runtime as never)

    await controller.spawn!({ cols: 80, rows: 24, connectionId: 'dev-01' })
    const killed = controller.kill('ssh:dev-01@@agent-pty-1')
    expect(killed).toBe(true)
    await vi.waitFor(() => {
      if (runtime.onPtyExit.mock.calls.length === 0) {throw new Error('not yet')}
    })
    expect(runtime.onPtyExit).toHaveBeenCalledTimes(1)
    expect(runtime.onPtyExit).toHaveBeenCalledWith('ssh:dev-01@@agent-pty-1', -1)

    // A real exit delivered afterward (e.g. via the data-plane relay) must not double-fire.
    controller.notifyProviderExit('ssh:dev-01@@agent-pty-1', 0)
    expect(runtime.onPtyExit).toHaveBeenCalledTimes(1)
  })

  it('getSize reflects the most recent successful resize', async () => {
    const provider = makeFakeProvider()
    getRemotePtyProviderMock.mockReturnValue(provider)
    const { createServerPtyController } = await import('./server-pty-controller')
    const controller = createServerPtyController(makeFakeRuntime() as never)

    const created = await controller.spawn!({ cols: 80, rows: 24, connectionId: 'dev-01' })
    expect(controller.getSize!(created.id)).toEqual({ cols: 80, rows: 24 })

    controller.resize!(created.id, 120, 40)
    expect(controller.getSize!(created.id)).toEqual({ cols: 120, rows: 40 })
  })

  it('notifyProviderExit clears size/ownership bookkeeping for the exited id', async () => {
    const provider = makeFakeProvider()
    getRemotePtyProviderMock.mockReturnValue(provider)
    const { createServerPtyController } = await import('./server-pty-controller')
    const runtime = makeFakeRuntime()
    const controller = createServerPtyController(runtime as never)

    const created = await controller.spawn!({ cols: 80, rows: 24, connectionId: 'dev-01' })
    controller.notifyProviderExit(created.id, 0)

    expect(runtime.onPtyExit).toHaveBeenCalledWith(created.id, 0)
    expect(controller.getSize!(created.id)).toBeNull()
  })
})
