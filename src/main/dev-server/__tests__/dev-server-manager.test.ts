// Unit tests for DevServerManager
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { homedir } from 'node:os'
import type { PersistedDevServer } from '../../../shared/dev-server-types'
import { getDefaultPersistedState } from '../../../shared/constants'

// ── Electron stub ────────────────────────────────────────────────────────────
vi.mock('electron', () => ({
  app: { getPath: () => '/tmp/orca-test' },
  safeStorage: {
    isEncryptionAvailable: () => false,
    encryptString: (s: string) => Buffer.from(s),
    decryptString: (b: Buffer) => b.toString()
  }
}))

// ── DevServerRelayBridge mock ─────────────────────────────────────────────────
// Why: Manager tests should not require real SSH; mock the bridge to control
// connect/disconnect outcomes independently.
// Why vi.hoisted: vi.mock factories run before module imports; hoisted vars
// must be created inside vi.hoisted() to be accessible from the factory.
const { mockBridgeConnect, mockBridgeDisconnect } = vi.hoisted(() => ({
  mockBridgeConnect: vi.fn(),
  mockBridgeDisconnect: vi.fn(),
}))

vi.mock('../dev-server-relay-bridge', () => {
  // Return a constructor function that produces objects with the minimal
  // EventEmitter surface (on/off) that DevServerManager.connect() calls.
  // Providing no-op on/off means bridge.on('agentTokenGenerated', ...) does
  // not throw; actual event-forwarding is tested in relay-bridge unit tests.
  function MockBridge(this: {
    connect: typeof mockBridgeConnect
    disconnect: typeof mockBridgeDisconnect
    session: null
    on: () => void
    off: () => void
    emit: () => void
  }) {
    this.connect = mockBridgeConnect
    this.disconnect = mockBridgeDisconnect
    this.session = null
    this.on = () => { /* no-op */ }
    this.off = () => { /* no-op */ }
    this.emit = () => { /* no-op */ }
  }
  return { DevServerRelayBridge: MockBridge }
})

// Why static import (not per-test dynamic import): vi.mock() is hoisted and
// runs before any import, so the mock is already registered when this module-
// level import executes. Using static import avoids the 30s+ timeout caused
// by the first dynamic import waiting for Vite's server.deps.inline transform
// to complete when renderer tests are running concurrently in the same worker.
import { DevServerManager } from '../dev-server-manager'

// ── Minimal in-memory store ───────────────────────────────────────────────────
function createMinimalStore(devServers: PersistedDevServer[] = []) {
  let state: ReturnType<typeof getDefaultPersistedState> = {
    ...getDefaultPersistedState(homedir()),
    devServers
  }
  return {
    getState: () => state,
    mutate: (fn: (s: typeof state) => void) => { fn(state) }
  }
}

// ── Minimal SshConnectionManager stub ────────────────────────────────────────
function createSshManagerStub() {
  return {
    getConnection: vi.fn().mockReturnValue({/* mock SshConnection */}),
    connect: vi.fn(),
    disconnect: vi.fn(),
    disconnectAll: vi.fn(),
    getState: vi.fn().mockReturnValue(null),
    getAllStates: vi.fn().mockReturnValue(new Map())
  }
}

// ── Tests ─────────────────────────────────────────────────────────────────────
describe('DevServerManager', () => {
  let persistStore: ReturnType<typeof createMinimalStore>
  let sshManager: ReturnType<typeof createSshManagerStub>

  beforeEach(() => {
    vi.clearAllMocks()
    persistStore = createMinimalStore()
    sshManager = createSshManagerStub()
  })

  it('add() persists devServer với id và addedAt', async () => {
    const manager = new DevServerManager(persistStore as never, sshManager as never)
    await manager.add({ name: 'Test', connectionType: 'relay-ssh' })
    expect(persistStore.getState().devServers).toHaveLength(1)
    expect(persistStore.getState().devServers![0].addedAt).toBeGreaterThan(0)
  })

  it('add() set runtime status = "disconnected"', async () => {
    const manager = new DevServerManager(persistStore as never, sshManager as never)
    const ds = await manager.add({ name: 'Test', connectionType: 'relay-ssh' })
    expect(ds.status).toBe('disconnected')
  })

  it('add() emit "devServer:added" event', async () => {
    const manager = new DevServerManager(persistStore as never, sshManager as never)
    const addedIds: string[] = []
    manager.on('devServer:added', (id: string) => addedIds.push(id))
    const ds = await manager.add({ name: 'Test', connectionType: 'relay-ssh' })
    expect(addedIds).toContain(ds.id)
  })

  it('testConnection() relay-ssh success → return { ok: true, platform, nodeVersion }', async () => {
    mockBridgeConnect.mockResolvedValueOnce({
      platform: 'linux' as NodeJS.Platform,
      arch: 'x64',
      nodeVersion: 'v20.0.0',
      relayVersion: '0.1.0'
    })
    const manager = new DevServerManager(persistStore as never, sshManager as never)
    const result = await manager.testConnection({
      name: 'Test',
      connectionType: 'relay-ssh',
      sshTargetId: 'ssh-1'
    })
    expect(result.ok).toBe(true)
    if (result.ok) {
      expect(result.platform).toBe('linux')
      expect(result.nodeVersion).toBe('v20.0.0')
    }
  })

  it('testConnection() relay-ssh failure → return { ok: false, error }', async () => {
    mockBridgeConnect.mockRejectedValueOnce(new Error('Connection refused'))
    const manager = new DevServerManager(persistStore as never, sshManager as never)
    const result = await manager.testConnection({
      name: 'Test',
      connectionType: 'relay-ssh',
      sshTargetId: 'ssh-1'
    })
    expect(result.ok).toBe(false)
    if (!result.ok) {
      expect(result.error).toContain('Connection refused')
    }
  })

  it('connect() relay-ssh → status: "connecting" → "connected"', async () => {
    mockBridgeConnect.mockResolvedValueOnce({
      platform: 'darwin' as NodeJS.Platform,
      arch: 'arm64',
      nodeVersion: 'v20.0.0',
      relayVersion: '0.1.0'
    })
    const manager = new DevServerManager(persistStore as never, sshManager as never)
    const ds = await manager.add({ name: 'Test', connectionType: 'relay-ssh' })
    await manager.connect(ds.id)
    const after = manager.get(ds.id)!
    expect(after.status).toBe('connected')
  })

  it('connect() relay-ssh → emit statusChanged "connecting" rồi "connected"', async () => {
    mockBridgeConnect.mockResolvedValueOnce({
      platform: 'darwin' as NodeJS.Platform,
      arch: 'arm64',
      nodeVersion: 'v20.0.0',
      relayVersion: '0.1.0'
    })
    const manager = new DevServerManager(persistStore as never, sshManager as never)
    const ds = await manager.add({ name: 'Test', connectionType: 'relay-ssh' })
    const statuses: string[] = []
    manager.on('devServer:statusChanged', (_id: string, s: string) => statuses.push(s))
    await manager.connect(ds.id)
    expect(statuses).toContain('connecting')
    expect(statuses).toContain('connected')
  })

  it('connect() relay-ssh failure → status: "error" + lastError set', async () => {
    mockBridgeConnect.mockRejectedValueOnce(new Error('timeout'))
    const manager = new DevServerManager(persistStore as never, sshManager as never)
    const ds = await manager.add({ name: 'Test', connectionType: 'relay-ssh' })
    await expect(manager.connect(ds.id)).rejects.toThrow('timeout')
    const after = manager.get(ds.id)!
    expect(after.status).toBe('error')
    expect(after.lastError).toContain('timeout')
  })

  it('connect() relay-ssh failure → emit statusChanged "error"', async () => {
    mockBridgeConnect.mockRejectedValueOnce(new Error('timeout'))
    const manager = new DevServerManager(persistStore as never, sshManager as never)
    const ds = await manager.add({ name: 'Test', connectionType: 'relay-ssh' })
    const statuses: string[] = []
    manager.on('devServer:statusChanged', (_id: string, s: string) => statuses.push(s))
    await manager.connect(ds.id).catch(() => {})
    expect(statuses).toContain('error')
  })

  it('disconnect() → relay.close() được gọi, status: "disconnected"', async () => {
    mockBridgeConnect.mockResolvedValueOnce({
      platform: 'linux' as NodeJS.Platform,
      arch: 'x64',
      nodeVersion: 'v20.0.0',
      relayVersion: '0.1.0'
    })
    mockBridgeDisconnect.mockResolvedValue(undefined)
    const manager = new DevServerManager(persistStore as never, sshManager as never)
    const ds = await manager.add({ name: 'Test', connectionType: 'relay-ssh' })
    await manager.connect(ds.id)
    await manager.disconnect(ds.id)
    expect(mockBridgeDisconnect).toHaveBeenCalled()
    const after = manager.get(ds.id)!
    expect(after.status).toBe('disconnected')
  })

  it('disconnect() → emit statusChanged "disconnected"', async () => {
    mockBridgeConnect.mockResolvedValueOnce({
      platform: 'linux' as NodeJS.Platform,
      arch: 'x64',
      nodeVersion: 'v20.0.0',
      relayVersion: '0.1.0'
    })
    mockBridgeDisconnect.mockResolvedValue(undefined)
    const manager = new DevServerManager(persistStore as never, sshManager as never)
    const ds = await manager.add({ name: 'Test', connectionType: 'relay-ssh' })
    await manager.connect(ds.id)
    const statuses: string[] = []
    manager.on('devServer:statusChanged', (_id: string, s: string) => statuses.push(s))
    await manager.disconnect(ds.id)
    expect(statuses).toContain('disconnected')
  })

  it('remove() → disconnect() được gọi trước', async () => {
    mockBridgeConnect.mockResolvedValueOnce({
      platform: 'linux' as NodeJS.Platform,
      arch: 'x64',
      nodeVersion: 'v20.0.0',
      relayVersion: '0.1.0'
    })
    mockBridgeDisconnect.mockResolvedValue(undefined)
    const manager = new DevServerManager(persistStore as never, sshManager as never)
    const ds = await manager.add({ name: 'Test', connectionType: 'relay-ssh' })
    await manager.connect(ds.id)
    await manager.remove(ds.id)
    expect(mockBridgeDisconnect).toHaveBeenCalled()
  })

  it('remove() → xóa khỏi store', async () => {
    const manager = new DevServerManager(persistStore as never, sshManager as never)
    const ds = await manager.add({ name: 'Test', connectionType: 'relay-ssh' })
    await manager.remove(ds.id)
    expect(persistStore.getState().devServers).toHaveLength(0)
  })

  it('remove() → emit "devServer:removed"', async () => {
    const manager = new DevServerManager(persistStore as never, sshManager as never)
    const ds = await manager.add({ name: 'Test', connectionType: 'relay-ssh' })
    const removedIds: string[] = []
    manager.on('devServer:removed', (id: string) => removedIds.push(id))
    await manager.remove(ds.id)
    expect(removedIds).toContain(ds.id)
  })

  it('getRelay() trả về bridge khi connected', async () => {
    mockBridgeConnect.mockResolvedValueOnce({
      platform: 'linux' as NodeJS.Platform,
      arch: 'x64',
      nodeVersion: 'v20.0.0',
      relayVersion: '0.1.0'
    })
    const manager = new DevServerManager(persistStore as never, sshManager as never)
    const ds = await manager.add({ name: 'Test', connectionType: 'relay-ssh' })
    await manager.connect(ds.id)
    expect(manager.getRelay(ds.id)).not.toBeNull()
  })

  it('getRelay() trả về null khi không connected', async () => {
    const manager = new DevServerManager(persistStore as never, sshManager as never)
    const ds = await manager.add({ name: 'Test', connectionType: 'relay-ssh' })
    expect(manager.getRelay(ds.id)).toBeNull()
  })

  it('list() merge persisted + runtime state', async () => {
    const manager = new DevServerManager(persistStore as never, sshManager as never)
    await manager.add({ name: 'Test', connectionType: 'relay-ssh' })
    const list = manager.list()
    expect(list).toHaveLength(1)
    // Runtime fields should be present (merged)
    expect(list[0].status).toBe('disconnected')
    expect(list[0].platform).toBeNull()
  })

  it('get() trả về null khi không tìm thấy id', async () => {
    const manager = new DevServerManager(persistStore as never, sshManager as never)
    expect(manager.get('non-existent-id')).toBeNull()
  })

  it('constructor() restore runtime state với status = "disconnected" cho tất cả persisted servers', async () => {
    const preSeeded: PersistedDevServer = {
      id: 'ds-existing',
      name: 'Pre-seeded',
      connectionType: 'relay-ssh',
      workspaceDir: null,
      addedAt: Date.now()
    }
    const seededStore = createMinimalStore([preSeeded])
    const manager = new DevServerManager(seededStore as never, sshManager as never)
    const ds = manager.get('ds-existing')
    expect(ds).not.toBeNull()
    expect(ds!.status).toBe('disconnected')
  })
})
