// Unit tests for DevServerStore
// Pattern: mirrors persistence.test.ts — writes to a real tmpdir-based Store
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

// ── Electron stub ────────────────────────────────────────────────────────────
// Why: DevServerStore imports Store which imports from 'electron'.
// We must stub it before any dynamic import of the Store.
const testState = { dir: '' }
vi.mock('electron', () => ({
  app: { getPath: () => testState.dir },
  safeStorage: {
    isEncryptionAvailable: () => false,
    encryptString: (s: string) => Buffer.from(s),
    decryptString: (b: Buffer) => b.toString()
  }
}))

// ── Store stub ────────────────────────────────────────────────────────────────
// Rather than spinning up a full Store (which requires heavy mocking of SSH
// config, telemetry, etc.), we use a lightweight in-memory Store-alike that
// satisfies the getState()/mutate() contract DevServerStore needs.
import type { PersistedDevServer } from '../../../shared/dev-server-types'
import { getDefaultPersistedState } from '../../../shared/constants'
import { homedir } from 'node:os'

type MinimalState = { devServers: PersistedDevServer[] }

function createMinimalStore(initial?: Partial<MinimalState>) {
  let state: ReturnType<typeof getDefaultPersistedState> = {
    ...getDefaultPersistedState(homedir()),
    devServers: initial?.devServers ?? []
  }
  return {
    getState: () => state,
    mutate: (fn: (s: typeof state) => void) => {
      fn(state)
    }
  }
}

// ── Tests ─────────────────────────────────────────────────────────────────────
describe('DevServerStore', () => {
  let store: ReturnType<typeof createMinimalStore>

  beforeEach(() => {
    testState.dir = mkdtempSync(join(tmpdir(), 'orca-dev-server-store-test-'))
    store = createMinimalStore()
  })

  afterEach(() => {
    rmSync(testState.dir, { recursive: true, force: true })
  })

  it('list() trả về [] khi chưa có devServers', async () => {
    const { DevServerStore } = await import('../dev-server-store')
    const devStore = new DevServerStore(store as never)
    expect(devStore.list()).toEqual([])
  })

  it('list() trả về [] khi state.devServers = undefined', async () => {
    const { DevServerStore } = await import('../dev-server-store')
    // Simulate legacy state where devServers is absent
    const legacyStore = createMinimalStore()
    ;(legacyStore.getState() as Record<string, unknown>).devServers = undefined
    const devStore = new DevServerStore(legacyStore as never)
    expect(devStore.list()).toEqual([])
  })

  it('add() tạo id với prefix "ds-"', async () => {
    const { DevServerStore } = await import('../dev-server-store')
    const devStore = new DevServerStore(store as never)
    const result = devStore.add({
      name: 'My Server',
      connectionType: 'relay-ssh',
      sshTargetId: 'ssh-target-1'
    })
    expect(result.id).toMatch(/^ds-/)
  })

  it('add() set workspaceDir = null và addedAt gần Date.now()', async () => {
    const { DevServerStore } = await import('../dev-server-store')
    const devStore = new DevServerStore(store as never)
    const before = Date.now()
    const result = devStore.add({ name: 'Server', connectionType: 'relay-ssh' })
    const after = Date.now()
    expect(result.workspaceDir).toBeNull()
    expect(result.addedAt).toBeGreaterThanOrEqual(before)
    expect(result.addedAt).toBeLessThanOrEqual(after)
  })

  it('add() persist vào store', async () => {
    const { DevServerStore } = await import('../dev-server-store')
    const devStore = new DevServerStore(store as never)
    const result = devStore.add({ name: 'Server', connectionType: 'relay-ssh' })
    const state = store.getState()
    expect(state.devServers).toHaveLength(1)
    expect(state.devServers![0].id).toBe(result.id)
  })

  it('update() sửa đúng field của đúng record', async () => {
    const { DevServerStore } = await import('../dev-server-store')
    const devStore = new DevServerStore(store as never)
    const added = devStore.add({ name: 'Old Name', connectionType: 'relay-ssh' })
    devStore.update(added.id, { name: 'New Name' })
    const list = devStore.list()
    expect(list[0].name).toBe('New Name')
  })

  it('update() không ảnh hưởng record khác', async () => {
    const { DevServerStore } = await import('../dev-server-store')
    const devStore = new DevServerStore(store as never)
    const a = devStore.add({ name: 'A', connectionType: 'relay-ssh' })
    const b = devStore.add({ name: 'B', connectionType: 'relay-ssh' })
    devStore.update(a.id, { name: 'A Updated' })
    const list = devStore.list()
    const bRecord = list.find((ds) => ds.id === b.id)
    expect(bRecord?.name).toBe('B')
  })

  it('remove() xóa đúng record theo id', async () => {
    const { DevServerStore } = await import('../dev-server-store')
    const devStore = new DevServerStore(store as never)
    const added = devStore.add({ name: 'Server', connectionType: 'relay-ssh' })
    devStore.remove(added.id)
    expect(devStore.list()).toHaveLength(0)
  })

  it('remove() không ảnh hưởng record khác', async () => {
    const { DevServerStore } = await import('../dev-server-store')
    const devStore = new DevServerStore(store as never)
    const a = devStore.add({ name: 'A', connectionType: 'relay-ssh' })
    const b = devStore.add({ name: 'B', connectionType: 'relay-ssh' })
    devStore.remove(a.id)
    const list = devStore.list()
    expect(list).toHaveLength(1)
    expect(list[0].id).toBe(b.id)
  })
})
