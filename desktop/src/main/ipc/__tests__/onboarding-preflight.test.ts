// ─── onboarding-preflight.test.ts ───────────────────────────────────────────
// Unit tests for Phase 2 onboarding handlers: getPreflightStatus,
// setGitIdentity, detectGhosttyConfig (TASK-027).

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { EventEmitter } from 'node:events'

// ── Hoisted mocks ─────────────────────────────────────────────────────────────

const { ipcHandleMock, ipcRemoveHandlerMock } = vi.hoisted(() => ({
  ipcHandleMock: vi.fn(),
  ipcRemoveHandlerMock: vi.fn()
}))

vi.mock('electron', () => ({
  ipcMain: {
    handle: ipcHandleMock,
    removeHandler: ipcRemoveHandlerMock
  }
}))

const mockCall = vi.fn()
const mockRelay = { call: mockCall }

const mockManagerGet = vi.fn()
const mockManagerGetRelay = vi.fn()
const mockManagerList = vi.fn(() => [])

vi.mock('../../../main/dev-server/dev-server-manager', () => ({
  DevServerManager: vi.fn().mockImplementation(() => ({
    get: mockManagerGet,
    getRelay: mockManagerGetRelay,
    list: mockManagerList,
    on: vi.fn()
  }))
}))

vi.mock('../../../shared/agent-detection-commands', () => ({
  buildAgentDetectionCommands: vi.fn(() => [])
}))

// ── Test module state helpers ─────────────────────────────────────────────────

// We must re-import the module to get fresh handler registrations per test suite.
// Caches are module-level Maps, so clearing them is sufficient for state isolation.
let preflightCache: Map<string, unknown>
let getPreflightStatusHandler: (event: unknown, params: unknown) => Promise<unknown>
let setGitIdentityHandler: (event: unknown, params: unknown) => Promise<void>
let detectGhosttyConfigHandler: (event: unknown, params: unknown) => Promise<unknown>

beforeEach(async () => {
  vi.resetModules()
  ipcHandleMock.mockReset()
  ipcRemoveHandlerMock.mockReset()
  mockCall.mockReset()
  mockManagerGet.mockReset()
  mockManagerGetRelay.mockReset()

  const module = await import('../../../main/ipc/onboarding-ipc')
  preflightCache = module.preflightCache
  preflightCache.clear()
  module.agentDetectionCache.clear()
  if ('windowsCapsCache' in module) {
    // @ts-expect-error — access exported cache for test isolation
    ;(module.windowsCapsCache as Map<string, unknown>).clear()
  }

  const fakeManager = {
    get: mockManagerGet,
    getRelay: mockManagerGetRelay,
    list: mockManagerList,
    on: vi.fn()
  }

  module.registerOnboardingIpcHandlers(fakeManager as never)

  // Extract registered handlers by channel name
  const channelHandlers = new Map<string, (e: unknown, p: unknown) => Promise<unknown>>()
  for (const call of ipcHandleMock.mock.calls) {
    channelHandlers.set(call[0] as string, call[1] as (e: unknown, p: unknown) => Promise<unknown>)
  }
  getPreflightStatusHandler = channelHandlers.get('onboarding.getPreflightStatus')!
  setGitIdentityHandler = channelHandlers.get('onboarding.setGitIdentity')! as unknown as typeof setGitIdentityHandler
  detectGhosttyConfigHandler = channelHandlers.get('onboarding.detectGhosttyConfig')!
})


// ── onboarding.getPreflightStatus ─────────────────────────────────────────────

describe('onboarding.getPreflightStatus', () => {
  const devServerId = 'ds-001'
  const preflightRaw = {
    platform: 'linux' as NodeJS.Platform,
    gh: { installed: true, authenticated: true, version: 'gh version 2.40.0' },
    git: { installed: true, version: 'git version 2.43.0', hasUserName: true, hasUserEmail: true }
  }

  it('cache miss → gọi relay, lưu cache', async () => {
    mockManagerGetRelay.mockReturnValue(mockRelay)
    mockCall.mockResolvedValue(preflightRaw)

    const result = await getPreflightStatusHandler(null, { devServerId })
    expect(mockCall).toHaveBeenCalledWith('preflight.check', {}, 30_000)
    expect(result).toMatchObject({ devServerId, platform: 'linux', gh: preflightRaw.gh })
    expect(preflightCache.has(devServerId)).toBe(true)
  })

  it('cache hit (<30s) → không gọi relay', async () => {
    const cached = {
      devServerId,
      platform: 'linux' as NodeJS.Platform,
      checkedAt: Date.now(),
      gh: { installed: true, authenticated: true },
      git: { installed: true, hasUserName: true, hasUserEmail: true }
    }
    preflightCache.set(devServerId, { result: cached, cachedAt: Date.now() })

    const result = await getPreflightStatusHandler(null, { devServerId })
    expect(mockCall).not.toHaveBeenCalled()
    expect(result).toEqual(cached)
  })

  it('force: true → bypass cache, gọi relay lại', async () => {
    const cachedResult = {
      devServerId,
      platform: 'linux' as NodeJS.Platform,
      checkedAt: Date.now() - 5_000,
      gh: { installed: true, authenticated: false },
      git: { installed: true, hasUserName: false, hasUserEmail: false }
    }
    preflightCache.set(devServerId, { result: cachedResult, cachedAt: Date.now() })

    mockManagerGetRelay.mockReturnValue(mockRelay)
    mockCall.mockResolvedValue(preflightRaw)

    await getPreflightStatusHandler(null, { devServerId, force: true })
    expect(mockCall).toHaveBeenCalledWith('preflight.check', {}, 30_000)
  })

  it('relay không kết nối → throw Error', async () => {
    mockManagerGetRelay.mockReturnValue(null)
    await expect(getPreflightStatusHandler(null, { devServerId })).rejects.toThrow(
      `Dev server '${devServerId}' not connected`
    )
  })

  it('gh installed + authenticated → { installed: true, authenticated: true }', async () => {
    mockManagerGetRelay.mockReturnValue(mockRelay)
    mockCall.mockResolvedValue({
      ...preflightRaw,
      gh: { installed: true, authenticated: true, version: '2.40.0' }
    })
    const result = (await getPreflightStatusHandler(null, { devServerId })) as { gh: { authenticated: boolean } }
    expect(result.gh.authenticated).toBe(true)
  })

  it('gh installed + not authenticated → { installed: true, authenticated: false }', async () => {
    mockManagerGetRelay.mockReturnValue(mockRelay)
    mockCall.mockResolvedValue({
      ...preflightRaw,
      gh: { installed: true, authenticated: false, version: '2.40.0' }
    })
    const result = (await getPreflightStatusHandler(null, { devServerId })) as { gh: { authenticated: boolean; installed: boolean } }
    expect(result.gh.installed).toBe(true)
    expect(result.gh.authenticated).toBe(false)
  })

  it('gh not installed → { installed: false, authenticated: false }', async () => {
    mockManagerGetRelay.mockReturnValue(mockRelay)
    mockCall.mockResolvedValue({
      ...preflightRaw,
      gh: { installed: false, authenticated: false }
    })
    const result = (await getPreflightStatusHandler(null, { devServerId })) as { gh: { installed: boolean } }
    expect(result.gh.installed).toBe(false)
  })

  it('git installed, có identity → { installed: true, hasUserName: true, hasUserEmail: true }', async () => {
    mockManagerGetRelay.mockReturnValue(mockRelay)
    mockCall.mockResolvedValue({
      ...preflightRaw,
      git: { installed: true, version: '2.43.0', hasUserName: true, hasUserEmail: true }
    })
    const result = (await getPreflightStatusHandler(null, { devServerId })) as {
      git: { hasUserName: boolean; hasUserEmail: boolean }
    }
    expect(result.git.hasUserName).toBe(true)
    expect(result.git.hasUserEmail).toBe(true)
  })

  it('git installed, chưa có email → { hasUserEmail: false }', async () => {
    mockManagerGetRelay.mockReturnValue(mockRelay)
    mockCall.mockResolvedValue({
      ...preflightRaw,
      git: { installed: true, version: '2.43.0', hasUserName: true, hasUserEmail: false }
    })
    const result = (await getPreflightStatusHandler(null, { devServerId })) as {
      git: { hasUserEmail: boolean }
    }
    expect(result.git.hasUserEmail).toBe(false)
  })
})

// ── onboarding.setGitIdentity ─────────────────────────────────────────────────

describe('onboarding.setGitIdentity', () => {
  const devServerId = 'ds-002'

  it('gọi preflight.setGitIdentity trên relay với đúng name + email', async () => {
    mockManagerGetRelay.mockReturnValue(mockRelay)
    mockCall.mockResolvedValue(undefined)

    await setGitIdentityHandler(null, {
      devServerId,
      name: 'Alice Developer',
      email: 'alice@example.com'
    })
    expect(mockCall).toHaveBeenCalledWith('preflight.setGitIdentity', {
      name: 'Alice Developer',
      email: 'alice@example.com'
    })
  })

  it('invalidate preflight cache sau khi set thành công', async () => {
    preflightCache.set(devServerId, {
      result: { devServerId, platform: 'linux', checkedAt: Date.now(), gh: {}, git: {} } as never,
      cachedAt: Date.now()
    })
    mockManagerGetRelay.mockReturnValue(mockRelay)
    mockCall.mockResolvedValue(undefined)

    await setGitIdentityHandler(null, {
      devServerId,
      name: 'Bob',
      email: 'bob@example.com'
    })
    expect(preflightCache.has(devServerId)).toBe(false)
  })

  it('relay không connected → throw Error', async () => {
    mockManagerGetRelay.mockReturnValue(null)
    await expect(
      setGitIdentityHandler(null, { devServerId, name: 'X', email: 'x@x.com' })
    ).rejects.toThrow(`Dev server '${devServerId}' not connected`)
  })
})

// ── onboarding.detectGhosttyConfig ───────────────────────────────────────────

describe('onboarding.detectGhosttyConfig', () => {
  const devServerId = 'ds-003'

  it('forward đến relay, trả về configPath + themeDir', async () => {
    mockManagerGetRelay.mockReturnValue(mockRelay)
    const expected = {
      configPath: '/home/user/.config/ghostty/config',
      themeDir: '/home/user/.config/ghostty/themes'
    }
    mockCall.mockResolvedValue(expected)

    const result = await detectGhosttyConfigHandler(null, { devServerId })
    expect(mockCall).toHaveBeenCalledWith('preflight.detectGhosttyConfig', {})
    expect(result).toEqual(expected)
  })

  it('relay không connected → throw Error', async () => {
    mockManagerGetRelay.mockReturnValue(null)
    await expect(detectGhosttyConfigHandler(null, { devServerId })).rejects.toThrow(
      'Dev server not connected'
    )
  })
})
