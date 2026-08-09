// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useBrowserNotificationPermission } from '../useBrowserNotificationPermission'
import { useWebPushSubscription, urlBase64ToUint8Array } from '../useWebPushSubscription'

// ── useBrowserNotificationPermission ──────────────────────────────────────────

describe('useBrowserNotificationPermission', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('returns unsupported when Notification API is absent', () => {
    vi.stubGlobal('window', { Notification: undefined })
    const { result } = renderHook(() => useBrowserNotificationPermission())
    expect(result.current.state).toBe('unsupported')
  })

  it('reads current permission on mount (granted)', () => {
    vi.stubGlobal('Notification', {
      permission: 'granted',
      requestPermission: vi.fn().mockResolvedValue('granted')
    })
    const { result } = renderHook(() => useBrowserNotificationPermission())
    expect(result.current.state).toBe('granted')
  })

  it('reads current permission on mount (denied)', () => {
    vi.stubGlobal('Notification', {
      permission: 'denied',
      requestPermission: vi.fn().mockResolvedValue('denied')
    })
    const { result } = renderHook(() => useBrowserNotificationPermission())
    expect(result.current.state).toBe('denied')
  })

  it('requestPermission() updates state to granted', async () => {
    vi.stubGlobal('Notification', {
      permission: 'default',
      requestPermission: vi.fn().mockResolvedValue('granted')
    })
    const { result } = renderHook(() => useBrowserNotificationPermission())
    await act(async () => {
      await result.current.requestPermission()
    })
    expect(result.current.state).toBe('granted')
  })

  it('requestPermission() state stays denied when result is denied', async () => {
    vi.stubGlobal('Notification', {
      permission: 'default',
      requestPermission: vi.fn().mockResolvedValue('denied')
    })
    const { result } = renderHook(() => useBrowserNotificationPermission())
    await act(async () => {
      await result.current.requestPermission()
    })
    expect(result.current.state).toBe('denied')
  })
})

// ── urlBase64ToUint8Array ──────────────────────────────────────────────────────

describe('urlBase64ToUint8Array', () => {
  it('converts a URL-safe base64 string to Uint8Array', () => {
    vi.stubGlobal('window', {
      atob: (s: string) => Buffer.from(s, 'base64').toString('binary')
    })
    const result = urlBase64ToUint8Array('YWJj')
    expect(result).toBeInstanceOf(Uint8Array)
    expect(result.length).toBeGreaterThan(0)
    vi.unstubAllGlobals()
  })
})

// ── useWebPushSubscription ────────────────────────────────────────────────────

describe('useWebPushSubscription', () => {
  const mockSubscription = {
    endpoint: 'https://push.example.com/sub123',
    unsubscribe: vi.fn().mockResolvedValue(true)
  }
  const mockPushManager = {
    subscribe: vi.fn().mockResolvedValue(mockSubscription),
    getSubscription: vi.fn().mockResolvedValue(mockSubscription)
  }
  const mockSW = { ready: Promise.resolve({ pushManager: mockPushManager }) }

  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => ({ publicKey: 'dGVzdA' }) }))
    vi.stubGlobal('navigator', { serviceWorker: mockSW })
    vi.stubGlobal('window', { atob: (s: string) => Buffer.from(s, 'base64').toString('binary'), PushManager: {} })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.clearAllMocks()
  })

  it('initial state is idle', () => {
    const { result } = renderHook(() => useWebPushSubscription())
    expect(result.current.state).toBe('idle')
  })

  it('subscribe() transitions to subscribed on success', async () => {
    const { result } = renderHook(() => useWebPushSubscription())
    await act(async () => {
      await result.current.subscribe()
    })
    expect(result.current.state).toBe('subscribed')
  })

  it('subscribe() transitions to failed on fetch error', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({ ok: false, status: 500 } as Response)
    const { result } = renderHook(() => useWebPushSubscription())
    await act(async () => {
      await result.current.subscribe()
    })
    expect(result.current.state).toBe('failed')
  })

  it('unsubscribe() transitions back to idle', async () => {
    const { result } = renderHook(() => useWebPushSubscription())
    await act(async () => {
      await result.current.subscribe()
    })
    await act(async () => {
      await result.current.unsubscribe()
    })
    expect(result.current.state).toBe('idle')
  })
})
