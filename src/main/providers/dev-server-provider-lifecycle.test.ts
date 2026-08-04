import { describe, it, expect, vi, beforeEach } from 'vitest'
import { EventEmitter } from 'node:events'

const registerRemotePtyProviderMock = vi.fn()
const unregisterRemotePtyProviderMock = vi.fn()
vi.mock('../ipc/pty', () => ({
  registerRemotePtyProvider: (...args: unknown[]) => registerRemotePtyProviderMock(...args),
  unregisterRemotePtyProvider: (...args: unknown[]) => unregisterRemotePtyProviderMock(...args)
}))

vi.mock('./ssh-filesystem-dispatch', () => ({
  registerRemoteFilesystemProvider: vi.fn(),
  unregisterRemoteFilesystemProvider: vi.fn()
}))

vi.mock('./ssh-git-dispatch', () => ({
  registerRemoteGitProvider: vi.fn(),
  unregisterRemoteGitProvider: vi.fn()
}))

vi.mock('./dev-server-filesystem-provider', () => ({
  DevServerFilesystemProvider: class {}
}))

vi.mock('./dev-server-git-provider', () => ({
  DevServerGitProvider: class {}
}))

type NotificationHandler = (method: string, params: Record<string, unknown>) => void

class FakeRelay {
  private handler: NotificationHandler | null = null
  onNotification(handler: NotificationHandler): () => void {
    this.handler = handler
    return () => {
      this.handler = null
    }
  }
  call(): Promise<never> {
    return new Promise(() => {})
  }
  emit(method: string, params: Record<string, unknown>): void {
    this.handler?.(method, params)
  }
}

class FakeDevServerManager extends EventEmitter {
  private relaysById = new Map<string, FakeRelay>()
  private serversById = new Map<string, { id: string; capabilities: string[] }>()

  addServer(id: string, capabilities: string[]): FakeRelay {
    const relay = new FakeRelay()
    this.relaysById.set(id, relay)
    this.serversById.set(id, { id, capabilities })
    return relay
  }

  getRelay(id: string): FakeRelay | null {
    return this.relaysById.get(id) ?? null
  }

  async list(): Promise<Array<{ id: string; capabilities: string[] }>> {
    return [...this.serversById.values()]
  }
}

function makeFakeRuntime(): { onPtyData: ReturnType<typeof vi.fn>; onPtyExit: ReturnType<typeof vi.fn>; setPtyController: ReturnType<typeof vi.fn> } {
  return {
    onPtyData: vi.fn(),
    onPtyExit: vi.fn(),
    setPtyController: vi.fn()
  }
}

describe('wireDevServerProviders', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('does not require attachRuntime to be called (desktop-safe default)', async () => {
    const { wireDevServerProviders } = await import('./dev-server-provider-lifecycle')
    const manager = new FakeDevServerManager()

    expect(() => wireDevServerProviders(manager as never)).not.toThrow()
  })

  it('wires runtime.setPtyController exactly once when attachRuntime is called', async () => {
    const { wireDevServerProviders } = await import('./dev-server-provider-lifecycle')
    const manager = new FakeDevServerManager()
    const runtime = makeFakeRuntime()

    const lifecycle = wireDevServerProviders(manager as never)
    lifecycle.attachRuntime(runtime as never)

    expect(runtime.setPtyController).toHaveBeenCalledTimes(1)
  })

  it('relays a pty-ready Dev Server provider onData into runtime.onPtyData (runtime attached before connect)', async () => {
    const { wireDevServerProviders } = await import('./dev-server-provider-lifecycle')
    const manager = new FakeDevServerManager()
    const relay = manager.addServer('dev-01', ['pty', 'pty.stream'])
    const runtime = makeFakeRuntime()

    const lifecycle = wireDevServerProviders(manager as never)
    lifecycle.attachRuntime(runtime as never)
    manager.emit('devServer:statusChanged', 'dev-01', 'connected')
    await vi.waitFor(() => {
      if (registerRemotePtyProviderMock.mock.calls.length === 0) throw new Error('not yet')
    })

    relay.emit('pty.data', { id: 'agent-pty-1', data: 'hello\n' })

    expect(runtime.onPtyData).toHaveBeenCalledWith(
      'ssh:dev-01@@agent-pty-1',
      'hello\n',
      expect.any(Number)
    )
  })

  it('regression: retroactively wires the relay when a Dev Server connects BEFORE attachRuntime is called', async () => {
    // This is the exact race that broke live terminal creation: bootstrap
    // attaches the devServer:statusChanged listener immediately, but runtime
    // (and therefore attachRuntime) isn't ready until much later — a fast
    // real Dev Server connection can register its provider in that gap.
    const { wireDevServerProviders } = await import('./dev-server-provider-lifecycle')
    const manager = new FakeDevServerManager()
    const relay = manager.addServer('dev-01', ['pty', 'pty.stream'])
    const runtime = makeFakeRuntime()

    const lifecycle = wireDevServerProviders(manager as never)
    manager.emit('devServer:statusChanged', 'dev-01', 'connected')
    await vi.waitFor(() => {
      if (registerRemotePtyProviderMock.mock.calls.length === 0) throw new Error('not yet')
    })

    // Runtime only becomes available well after the provider already registered.
    lifecycle.attachRuntime(runtime as never)

    relay.emit('pty.data', { id: 'agent-pty-1', data: 'hello\n' })
    expect(runtime.onPtyData).toHaveBeenCalledWith(
      'ssh:dev-01@@agent-pty-1',
      'hello\n',
      expect.any(Number)
    )
  })

  it('does not wire a data relay for a Dev Server without pty capabilities', async () => {
    const { wireDevServerProviders } = await import('./dev-server-provider-lifecycle')
    const manager = new FakeDevServerManager()
    manager.addServer('dev-02', ['fs', 'git'])
    const runtime = makeFakeRuntime()

    const lifecycle = wireDevServerProviders(manager as never)
    lifecycle.attachRuntime(runtime as never)
    manager.emit('devServer:statusChanged', 'dev-02', 'connected')
    // registerFor() awaits devServerManager.list() once before deciding
    // ptyReady — flush that microtask hop before asserting the negative.
    await Promise.resolve()
    await Promise.resolve()

    expect(registerRemotePtyProviderMock).not.toHaveBeenCalled()
  })

  it('unsubscribes the data relay on disconnect so a stale provider cannot forward further events', async () => {
    const { wireDevServerProviders } = await import('./dev-server-provider-lifecycle')
    const manager = new FakeDevServerManager()
    const relay = manager.addServer('dev-01', ['pty', 'pty.stream'])
    const runtime = makeFakeRuntime()

    const lifecycle = wireDevServerProviders(manager as never)
    lifecycle.attachRuntime(runtime as never)
    manager.emit('devServer:statusChanged', 'dev-01', 'connected')
    await vi.waitFor(() => {
      if (registerRemotePtyProviderMock.mock.calls.length === 0) throw new Error('not yet')
    })

    manager.emit('devServer:statusChanged', 'dev-01', 'disconnected')
    expect(unregisterRemotePtyProviderMock).toHaveBeenCalledWith('dev-01')

    relay.emit('pty.data', { id: 'agent-pty-1', data: 'should not arrive' })
    expect(runtime.onPtyData).not.toHaveBeenCalled()
  })
})
