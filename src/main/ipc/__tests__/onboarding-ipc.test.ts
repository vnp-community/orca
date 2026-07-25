// Unit tests for onboarding-ipc.ts — detectAgents + detectAgentsAllServers
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

// ── Electron stub ─────────────────────────────────────────────────────────────
vi.mock('electron', () => ({
  ipcMain: {
    handle: vi.fn(),
    removeHandler: vi.fn()
  }
}))

// ── buildAgentDetectionCommands mock ─────────────────────────────────────────
// Why: isolate tests from the full TUI_AGENT_CONFIG catalog.
const { mockBuildAgentDetectionCommands } = vi.hoisted(() => ({
  mockBuildAgentDetectionCommands: vi.fn().mockReturnValue([
    { id: 'claude', cmd: 'claude' },
    { id: 'codex', cmd: 'codex' }
  ])
}))

vi.mock('../../../shared/agent-detection-commands', () => ({
  buildAgentDetectionCommands: mockBuildAgentDetectionCommands
}))

// ── Types ─────────────────────────────────────────────────────────────────────
type MockIpcMain = { handle: ReturnType<typeof vi.fn>; removeHandler: ReturnType<typeof vi.fn> }

// ── Mock helpers ──────────────────────────────────────────────────────────────

function createRelayStub(
  detectResult: { agents: string[]; platform: NodeJS.Platform } = {
    agents: ['claude'],
    platform: 'linux'
  }
) {
  return {
    detectAgents: vi.fn().mockResolvedValue(detectResult),
    connect: vi.fn(),
    disconnect: vi.fn(),
    session: {}
  }
}

function createManagerStub(
  servers: Array<{ id: string; status: string; platform: NodeJS.Platform | null }> = [],
  relayByServerId: Record<string, ReturnType<typeof createRelayStub> | null> = {}
) {
  return {
    list: vi.fn().mockReturnValue(servers),
    getRelay: vi.fn((id: string) => relayByServerId[id] ?? null),
    add: vi.fn(),
    remove: vi.fn(),
    connect: vi.fn(),
    disconnect: vi.fn(),
    get: vi.fn(),
    on: vi.fn()
  }
}

/** Register handlers and return the freshly created ipcMain mock for this module instance. */
async function setupTest(
  servers: Array<{ id: string; status: string; platform: NodeJS.Platform | null }> = [],
  relayByServerId: Record<string, ReturnType<typeof createRelayStub> | null> = {}
) {
  // Reset modules so each test gets a fresh cache and fresh ipcMain mock
  vi.resetModules()
  const electronModule = await import('electron')
  const ipc = electronModule.ipcMain as unknown as MockIpcMain
  ipc.handle.mockClear()
  ipc.removeHandler.mockClear()

  const { registerOnboardingIpcHandlers, agentDetectionCache } = await import('../onboarding-ipc')
  agentDetectionCache.clear()

  const manager = createManagerStub(servers, relayByServerId)
  registerOnboardingIpcHandlers(manager as never)

  /** Invoke a registered IPC channel handler as the renderer would */
  async function invoke(channel: string, ...args: unknown[]): Promise<unknown> {
    for (const [ch, fn] of ipc.handle.mock.calls) {
      if (ch === channel) {
        return (fn as (_event: unknown, ...args: unknown[]) => Promise<unknown>)(null, ...args)
      }
    }
    throw new Error(`No handler registered for '${channel}'`)
  }

  return { ipc, invoke, agentDetectionCache, manager }
}

// ── Tests: onboarding.detectAgents ───────────────────────────────────────────

describe('onboarding.detectAgents', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('devServerId = null → trả về { agents: [], platform: null, devServerId: null }', async () => {
    const { invoke } = await setupTest()
    const result = await invoke('onboarding.detectAgents', { devServerId: null })
    expect(result).toEqual({ agents: [], platform: null, devServerId: null })
  })

  it('relay null (server không tồn tại hoặc không connected) → throw Error', async () => {
    const { invoke } = await setupTest([], { 'ds-1': null })
    await expect(invoke('onboarding.detectAgents', { devServerId: 'ds-1' })).rejects.toThrow(
      "Dev server 'ds-1' not connected"
    )
  })

  it('dev server connected → forward detectAgents đến relay với đúng commands', async () => {
    const relay = createRelayStub({ agents: ['claude'], platform: 'linux' })
    const { invoke } = await setupTest(
      [{ id: 'ds-1', status: 'connected', platform: 'linux' }],
      { 'ds-1': relay }
    )
    await invoke('onboarding.detectAgents', { devServerId: 'ds-1' })
    expect(relay.detectAgents).toHaveBeenCalledWith([
      { id: 'claude', cmd: 'claude' },
      { id: 'codex', cmd: 'codex' }
    ])
  })

  it('relay trả về agents và platform → return đúng format', async () => {
    const relay = createRelayStub({ agents: ['claude', 'codex'], platform: 'darwin' })
    const { invoke } = await setupTest(
      [{ id: 'ds-1', status: 'connected', platform: 'darwin' }],
      { 'ds-1': relay }
    )
    const result = await invoke('onboarding.detectAgents', { devServerId: 'ds-1' })
    expect(result).toEqual({
      agents: ['claude', 'codex'],
      platform: 'darwin',
      devServerId: 'ds-1'
    })
  })

  it('cache hit trong 60s → không gọi relay lần 2', async () => {
    const relay = createRelayStub({ agents: ['claude'], platform: 'linux' })
    const { invoke } = await setupTest(
      [{ id: 'ds-1', status: 'connected', platform: 'linux' }],
      { 'ds-1': relay }
    )
    // First call populates cache
    await invoke('onboarding.detectAgents', { devServerId: 'ds-1' })
    // Advance time by 59s (still within TTL)
    vi.advanceTimersByTime(59_000)
    // Second call should use cache
    await invoke('onboarding.detectAgents', { devServerId: 'ds-1' })
    expect(relay.detectAgents).toHaveBeenCalledTimes(1)
  })

  it('cache miss sau 60s → gọi relay lại', async () => {
    const relay = createRelayStub({ agents: ['claude'], platform: 'linux' })
    const { invoke } = await setupTest(
      [{ id: 'ds-1', status: 'connected', platform: 'linux' }],
      { 'ds-1': relay }
    )
    // First call populates cache
    await invoke('onboarding.detectAgents', { devServerId: 'ds-1' })
    // Advance time by 61s (TTL expired)
    vi.advanceTimersByTime(61_000)
    // Second call should call relay again
    await invoke('onboarding.detectAgents', { devServerId: 'ds-1' })
    expect(relay.detectAgents).toHaveBeenCalledTimes(2)
  })

  it('relay throw error → không lưu vào cache', async () => {
    const relay = { detectAgents: vi.fn().mockRejectedValue(new Error('relay error')), session: {} }
    const { invoke, agentDetectionCache } = await setupTest(
      [{ id: 'ds-2', status: 'connected', platform: 'linux' }],
      { 'ds-2': relay }
    )
    await expect(invoke('onboarding.detectAgents', { devServerId: 'ds-2' })).rejects.toThrow(
      'relay error'
    )
    // Cache should not have an entry for 'ds-2'
    expect(agentDetectionCache.has('ds-2')).toBe(false)
  })
})

// ── Tests: onboarding.detectAgentsAllServers ──────────────────────────────────

describe('onboarding.detectAgentsAllServers', () => {
  it('0 connected servers → trả về {} rỗng', async () => {
    const { invoke } = await setupTest(
      [{ id: 'ds-offline', status: 'disconnected', platform: null }],
      {}
    )
    const result = await invoke('onboarding.detectAgentsAllServers')
    expect(result).toEqual({})
  })

  it('2 connected servers → map { dsId: { agents, platform } }', async () => {
    const relay1 = createRelayStub({ agents: ['claude'], platform: 'linux' })
    const relay2 = createRelayStub({ agents: ['codex'], platform: 'darwin' })
    const { invoke } = await setupTest(
      [
        { id: 'ds-1', status: 'connected', platform: 'linux' },
        { id: 'ds-2', status: 'connected', platform: 'darwin' }
      ],
      { 'ds-1': relay1, 'ds-2': relay2 }
    )
    const result = await invoke('onboarding.detectAgentsAllServers')
    expect(result).toEqual({
      'ds-1': { agents: ['claude'], platform: 'linux' },
      'ds-2': { agents: ['codex'], platform: 'darwin' }
    })
  })

  it('1 thành công, 1 lỗi → { ds1: { agents }, ds2: { agents: [], error } }', async () => {
    const relay1 = createRelayStub({ agents: ['claude'], platform: 'linux' })
    const relay2 = {
      detectAgents: vi.fn().mockRejectedValue(new Error('Connection lost')),
      session: {}
    }
    const { invoke } = await setupTest(
      [
        { id: 'ds-1', status: 'connected', platform: 'linux' },
        { id: 'ds-2', status: 'connected', platform: 'linux' }
      ],
      { 'ds-1': relay1, 'ds-2': relay2 }
    )
    const result = (await invoke('onboarding.detectAgentsAllServers')) as Record<
      string,
      { agents: string[]; platform: NodeJS.Platform | null; error?: string }
    >
    expect(result['ds-1']).toEqual({ agents: ['claude'], platform: 'linux' })
    expect(result['ds-2']).toMatchObject({ agents: [], error: 'Connection lost' })
  })

  it('chạy song song (Promise.allSettled) — cả 2 relay được gọi ngay cả khi relay 1 lỗi', async () => {
    const relay1 = { detectAgents: vi.fn().mockRejectedValue(new Error('err')), session: {} }
    const relay2 = createRelayStub({ agents: ['codex'], platform: 'darwin' })
    const { invoke } = await setupTest(
      [
        { id: 'ds-1', status: 'connected', platform: 'linux' },
        { id: 'ds-2', status: 'connected', platform: 'darwin' }
      ],
      { 'ds-1': relay1, 'ds-2': relay2 }
    )
    await invoke('onboarding.detectAgentsAllServers')
    // Both should be called (Promise.allSettled does not short-circuit)
    expect(relay1.detectAgents).toHaveBeenCalled()
    expect(relay2.detectAgents).toHaveBeenCalled()
  })
})
