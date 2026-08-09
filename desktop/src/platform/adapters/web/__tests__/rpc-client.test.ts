import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { WebSocketRpcClient } from '../rpc-client'

// Why: MockWebSocket mirrors the real WebSocket onX property API so rpc-client.ts
// works without changes. We expose simulateOpen/simulateError/receive helpers
// to drive the mock from tests.
class MockWebSocket {
  static OPEN = 1
  static CLOSED = 3
  readyState = MockWebSocket.CLOSED
  sent: string[] = []
  url: string

  onopen: ((e: Event) => void) | null = null
  onerror: ((e: Event) => void) | null = null
  onclose: ((e: Event) => void) | null = null
  onmessage: ((e: { data: string }) => void) | null = null

  constructor(url: string) {
    this.url = url
  }

  send(data: string): void {
    this.sent.push(data)
  }

  close(): void {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.({ type: 'close' } as Event)
  }

  simulateOpen(): void {
    this.readyState = MockWebSocket.OPEN
    this.onopen?.({ type: 'open' } as Event)
  }

  simulateError(): void {
    this.onerror?.({ type: 'error' } as Event)
  }

  receive(data: object): void {
    this.onmessage?.({ data: JSON.stringify(data) })
  }
}

// Shared reference updated every time mock constructor is called
let mockWs: MockWebSocket = new MockWebSocket('ws://init')

/** Connect a client and drive the mock open — returns after Promise resolves */
async function connectClient(client: WebSocketRpcClient): Promise<void> {
  const connectPromise = client.connect()
  // Why: simulateOpen() must be called AFTER connect() sets ws.onopen,
  // but the Promise hasn't resolved yet. Using queueMicrotask ensures the
  // Promise chain is flushed before simulateOpen fires.
  await Promise.resolve() // flush microtask so ws.onopen is assigned
  mockWs.simulateOpen()
  await connectPromise
}

beforeEach(() => {
  vi.stubGlobal(
    'WebSocket',
    vi.fn(function (this: unknown, url: string) {
      mockWs = new MockWebSocket(url)
      return mockWs
    }) as unknown as typeof WebSocket
  )
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('WebSocketRpcClient', () => {
  describe('connect()', () => {
    it('resolves when WebSocket opens', async () => {
      const client = new WebSocketRpcClient('ws://localhost:6768/ws/runtime/api')
      await connectClient(client)
      expect(client.isConnected()).toBe(true)
    })

    it('rejects when WebSocket errors', async () => {
      const client = new WebSocketRpcClient('ws://error')
      const connectPromise = client.connect()
      await Promise.resolve()
      mockWs.simulateError()
      await expect(connectPromise).rejects.toThrow()
    })
  })

  describe('invoke()', () => {
    it('sends JSON-RPC invoke message and resolves with result', async () => {
      const client = new WebSocketRpcClient('ws://localhost:6768/ws/runtime/api')
      await connectClient(client)

      const invokePromise = client.invoke('repos:list')
      expect(mockWs.sent).toHaveLength(1)
      const sent = JSON.parse(mockWs.sent[0]) as Record<string, unknown>
      expect(sent.type).toBe('invoke')
      expect(sent.channel).toBe('repos:list')
      expect(sent.id).toBeDefined()

      mockWs.receive({ id: sent.id, type: 'result', result: [{ id: 'repo1' }] })
      const result = await invokePromise
      expect(result).toEqual([{ id: 'repo1' }])
    })

    it('rejects on error response', async () => {
      const client = new WebSocketRpcClient('ws://localhost:6768/ws/runtime/api')
      await connectClient(client)

      const invokePromise = client.invoke('bad:channel')
      const sent = JSON.parse(mockWs.sent[0]) as Record<string, unknown>
      mockWs.receive({ id: sent.id, type: 'error', message: 'Not found' })

      await expect(invokePromise).rejects.toThrow('Not found')
    })

    it('passes args correctly', async () => {
      const client = new WebSocketRpcClient('ws://localhost:6768/ws/runtime/api')
      await connectClient(client)

      const invokePromise = client.invoke('worktrees:create', 'repo1', 'main')
      const sent = JSON.parse(mockWs.sent[0]) as Record<string, unknown>
      expect(sent.args).toEqual(['repo1', 'main'])

      mockWs.receive({ id: sent.id, type: 'result', result: { id: 'wt1' } })
      await invokePromise
    })

    it('times out after INVOKE_TIMEOUT_MS', async () => {
      vi.useFakeTimers()

      const client = new WebSocketRpcClient('ws://localhost:6768/ws/runtime/api')
      const connectPromise = client.connect()
      mockWs.simulateOpen()
      await connectPromise

      const invokePromise = client.invoke('slow:operation')
      vi.advanceTimersByTime(31_000)

      await expect(invokePromise).rejects.toThrow('timeout')
      vi.useRealTimers()
    })

    it('throws when not connected', async () => {
      const client = new WebSocketRpcClient('ws://localhost:6768/ws/runtime/api')
      await expect(client.invoke('any:channel')).rejects.toThrow('Not connected')
    })
  })

  describe('on() — server push events', () => {
    it('receives push events from server', async () => {
      const client = new WebSocketRpcClient('ws://localhost:6768/ws/runtime/api')
      await connectClient(client)

      const handler = vi.fn()
      client.on('ssh:stateChanged', handler)
      mockWs.receive({ type: 'push', channel: 'ssh:stateChanged', args: [{ state: 'connected' }] })

      expect(handler).toHaveBeenCalledOnce()
      expect(handler).toHaveBeenCalledWith({ state: 'connected' })
    })

    it('returns unsubscribe function', async () => {
      const client = new WebSocketRpcClient('ws://localhost:6768/ws/runtime/api')
      await connectClient(client)

      const handler = vi.fn()
      const unsub = client.on('test:event', handler)
      unsub()
      mockWs.receive({ type: 'push', channel: 'test:event', args: [] })
      expect(handler).not.toHaveBeenCalled()
    })

    it('supports multiple listeners on same channel', async () => {
      const client = new WebSocketRpcClient('ws://localhost:6768/ws/runtime/api')
      await connectClient(client)

      const h1 = vi.fn()
      const h2 = vi.fn()
      client.on('test:multi', h1)
      client.on('test:multi', h2)
      mockWs.receive({ type: 'push', channel: 'test:multi', args: ['data'] })

      expect(h1).toHaveBeenCalledOnce()
      expect(h2).toHaveBeenCalledOnce()
    })
  })

  describe('once()', () => {
    it('receives event only once', async () => {
      const client = new WebSocketRpcClient('ws://localhost:6768/ws/runtime/api')
      await connectClient(client)

      const handler = vi.fn()
      client.once('one-time:event', handler)
      mockWs.receive({ type: 'push', channel: 'one-time:event', args: [] })
      mockWs.receive({ type: 'push', channel: 'one-time:event', args: [] })

      expect(handler).toHaveBeenCalledOnce()
    })
  })

  describe('send() — fire-and-forget', () => {
    it('sends message with type send', async () => {
      const client = new WebSocketRpcClient('ws://localhost:6768/ws/runtime/api')
      await connectClient(client)

      client.send('client:event', { data: 'value' })
      const sent = JSON.parse(mockWs.sent[0]) as Record<string, unknown>
      expect(sent.type).toBe('send')
      expect(sent.channel).toBe('client:event')
    })

    it('silently ignores when not connected', () => {
      const client = new WebSocketRpcClient('ws://localhost:6768/ws/runtime/api')
      expect(() => client.send('test', {})).not.toThrow()
    })
  })

  describe('disconnect()', () => {
    it('isConnected() returns false after disconnect', async () => {
      const client = new WebSocketRpcClient('ws://localhost:6768/ws/runtime/api')
      await connectClient(client)
      client.disconnect()
      expect(client.isConnected()).toBe(false)
    })
  })

  describe('URL auto-detection', () => {
    it('auto-detects ws URL when no url provided', () => {
      const client = new WebSocketRpcClient()
      expect(client).toBeDefined()
      client.disconnect()
    })
  })
})
