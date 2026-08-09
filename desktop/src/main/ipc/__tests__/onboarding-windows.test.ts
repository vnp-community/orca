// ─── onboarding-windows.test.ts ───────────────────────────────────────────────
// Unit tests for onboarding.detectWindowsCapabilities IPC handler — TASK-041.
// Tests: platform guard, not-found, relay forward, cache hit/miss, response shape.

import { describe, it, expect, vi, beforeEach } from 'vitest'

// ── Electron mock ──────────────────────────────────────────────────────────────
const ipcHandleMock = vi.fn()
const ipcRemoveHandlerMock = vi.fn()

vi.mock('electron', () => ({
  ipcMain: {
    handle: ipcHandleMock,
    removeHandler: ipcRemoveHandlerMock
  }
}))

// ── web-push mock ──────────────────────────────────────────────────────────────
vi.mock('web-push', () => ({
  default: {
    setVapidDetails: vi.fn(),
    generateVAPIDKeys: vi.fn(() => ({ publicKey: 'pk', privateKey: 'sk' })),
    sendNotification: vi.fn()
  }
}))

import type { WindowsTerminalCapabilities } from '../../../shared/dev-server-types'

// ── Fake relay call ────────────────────────────────────────────────────────────

const mockRelayCall = vi.fn<[string, unknown], Promise<WindowsTerminalCapabilities>>()

function makeCapabilities(overrides: Partial<WindowsTerminalCapabilities> = {}): WindowsTerminalCapabilities {
  return {
    hasWindowsTerminal: true,
    hasConPTY: true,
    pwshVersion: '7.4.0',
    gitBashPath: null,
    ...overrides
  }
}

// ── Test setup ─────────────────────────────────────────────────────────────────

describe('onboarding.detectWindowsCapabilities', () => {
  let handler: (e: null, params: unknown) => Promise<WindowsTerminalCapabilities>
  let mockManagerGet: ReturnType<typeof vi.fn>
  let mockManagerGetRelay: ReturnType<typeof vi.fn>

  beforeEach(async () => {
    vi.resetModules()
    ipcHandleMock.mockReset()
    ipcRemoveHandlerMock.mockReset()
    mockRelayCall.mockReset()

    mockManagerGet = vi.fn()
    mockManagerGetRelay = vi.fn(() => ({ call: mockRelayCall }))

    const module = await import('../../../main/ipc/onboarding-ipc')
    // Clear caches between tests
    module.windowsCapsCache.clear()

    const fakeManager = {
      get: mockManagerGet,
      getRelay: mockManagerGetRelay,
      list: vi.fn(),
      on: vi.fn()
    }
    module.registerOnboardingIpcHandlers(fakeManager as never)

    // Extract the detectWindowsCapabilities handler
    const handlers = new Map<string, (e: null, p: unknown) => Promise<unknown>>()
    for (const call of ipcHandleMock.mock.calls) {
      handlers.set(call[0] as string, call[1] as (e: null, p: unknown) => Promise<unknown>)
    }
    handler = handlers.get('onboarding.detectWindowsCapabilities') as typeof handler
    expect(handler).toBeTruthy()
  })

  it('dev server không tồn tại → throw "not found"', async () => {
    mockManagerGet.mockReturnValue(undefined)

    await expect(handler(null, { devServerId: 'ds-missing' })).rejects.toThrow('not found')
  })

  it('dev server không phải Windows → throw Error với platform name', async () => {
    mockManagerGet.mockReturnValue({ platform: 'linux', id: 'ds-1' })

    await expect(handler(null, { devServerId: 'ds-1' })).rejects.toThrow('not Windows')
  })

  it('dev server Windows nhưng relay không kết nối → throw Error', async () => {
    mockManagerGet.mockReturnValue({ platform: 'win32', id: 'ds-1' })
    mockManagerGetRelay.mockReturnValue(null)

    await expect(handler(null, { devServerId: 'ds-1' })).rejects.toThrow('not connected')
  })

  it('dev server Windows, relay connected → forward đến relay', async () => {
    mockManagerGet.mockReturnValue({ platform: 'win32', id: 'ds-1' })
    mockManagerGetRelay.mockReturnValue({ call: mockRelayCall })
    mockRelayCall.mockResolvedValue(makeCapabilities())

    const result = await handler(null, { devServerId: 'ds-1' })

    expect(mockRelayCall).toHaveBeenCalledWith(
      'preflight.detectWindowsTerminalCapabilities',
      {}
    )
    expect(result).toMatchObject({ hasWindowsTerminal: true, hasConPTY: true })
  })

  it('pwshVersion được include trong response', async () => {
    mockManagerGet.mockReturnValue({ platform: 'win32', id: 'ds-1' })
    mockManagerGetRelay.mockReturnValue({ call: mockRelayCall })
    mockRelayCall.mockResolvedValue(makeCapabilities({ pwshVersion: '7.4.1' }))

    const result = await handler(null, { devServerId: 'ds-1' })
    expect(result.pwshVersion).toBe('7.4.1')
  })

  it('gitBashPath được include khi available', async () => {
    mockManagerGet.mockReturnValue({ platform: 'win32', id: 'ds-1' })
    mockManagerGetRelay.mockReturnValue({ call: mockRelayCall })
    mockRelayCall.mockResolvedValue(
      makeCapabilities({ gitBashPath: 'C:\\Program Files\\Git\\bin\\bash.exe' })
    )

    const result = await handler(null, { devServerId: 'ds-1' })
    expect(result.gitBashPath).toBe('C:\\Program Files\\Git\\bin\\bash.exe')
  })

  it('cache miss → gọi relay, lưu cache', async () => {
    mockManagerGet.mockReturnValue({ platform: 'win32', id: 'ds-1' })
    mockManagerGetRelay.mockReturnValue({ call: mockRelayCall })
    mockRelayCall.mockResolvedValue(makeCapabilities())

    await handler(null, { devServerId: 'ds-1' })
    expect(mockRelayCall).toHaveBeenCalledOnce()
  })

  it('cache hit (<60s) → không gọi relay', async () => {
    mockManagerGet.mockReturnValue({ platform: 'win32', id: 'ds-1' })
    mockManagerGetRelay.mockReturnValue({ call: mockRelayCall })
    mockRelayCall.mockResolvedValue(makeCapabilities())

    // First call — populates cache
    await handler(null, { devServerId: 'ds-1' })
    // Second call — should hit cache
    await handler(null, { devServerId: 'ds-1' })

    expect(mockRelayCall).toHaveBeenCalledOnce()
  })
})
