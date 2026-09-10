import { describe, expect, it, vi, beforeEach } from 'vitest'
import { execFile } from 'node:child_process'
import type * as ChildProcess from 'node:child_process'
import { EventEmitter } from 'node:events'
import { handleBrowserScreencastStart, handleBrowserScreencastStop } from './browser-screencast-handler'
import { decodeBrowserScreencastFrame } from '../shared/browser-screencast-protocol'
import type { AgentLogger } from './agent-logger'

// Why mock node:child_process (not agent-browser's CLI itself) and 'ws' (not
// a real CDP endpoint): this suite verifies browser-screencast-handler.ts's
// own contract — CDP command sequencing, frame encode/notify, ack-per-frame,
// throttling, cleanup — without a real Chrome/CDP endpoint, mirroring
// browser-handler.test.ts's own "mock the boundary, not the engine"
// convention.
vi.mock('node:child_process', async (importOriginal) => {
  const actual = await importOriginal<typeof ChildProcess>()
  return { ...actual, execFile: vi.fn() }
})
const execFileMock = vi.mocked(execFile)

// fakeWs is the one WebSocket instance every CdpClient.connect() call in
// this suite resolves to — tests drive it directly (emitOpen/emitMessage/
// emitClose) to simulate the CDP endpoint's side of the conversation, and
// inspect .sent to assert what browser-screencast-handler.ts sent it.
class FakeWs extends EventEmitter {
  sent: string[] = []
  terminated = false
  closed = false
  send(data: string): void {
    this.sent.push(data)
  }
  close(): void {
    this.closed = true
  }
  terminate(): void {
    this.terminated = true
  }
}

// vi.mock factories are hoisted above this file's own top-level statements,
// so the holder object they close over must itself come from vi.hoisted() —
// a plain `let lastFakeWs` here would be in the temporal dead zone when the
// factory below actually runs.
const wsHolder = vi.hoisted(() => ({ last: null as FakeWs | null }))
vi.mock('ws', () => ({
  WebSocket: vi.fn().mockImplementation(function (this: unknown) {
    wsHolder.last = new FakeWs()
    return wsHolder.last
  })
}))

const LOG: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }

type ExecFileArgs = [
  string,
  string[],
  { encoding: string; timeout: number; env: NodeJS.ProcessEnv },
  (error: Error | null, stdout: string, stderr: string) => void
]

function respondCdpUrl(url: string): void {
  const [, , , callback] = execFileMock.mock.calls.at(-1) as unknown as ExecFileArgs
  callback(null, JSON.stringify({ success: true, data: url, error: null }), '')
}

beforeEach(() => {
  execFileMock.mockClear()
  wsHolder.last = null
})

// waitFor polls instead of assuming a fixed microtask-tick count — mirrors
// browser-handler.test.ts's waitForCallCount, needed here for the same
// reason: resolving a chain of several awaited promises (execFileAsync's
// promise -> runBrowserCommand's own async-function promise -> ...) takes
// more than one microtask tick to unwind.
async function waitFor(predicate: () => boolean, description: string): Promise<void> {
  for (let i = 0; i < 200; i++) {
    if (predicate()) {return}
    await Promise.resolve()
  }
  throw new Error(`timed out waiting for: ${description}`)
}

// driveCdpConnect resolves `agent-browser get cdp-url`, then fires the fake
// WebSocket's 'open' event so CdpClient.connect()'s promise settles —
// every test needs this before it can drive CDP command/response traffic.
async function driveCdpConnect(): Promise<FakeWs> {
  await waitFor(() => execFileMock.mock.calls.length > 0, 'agent-browser get cdp-url to be invoked')
  respondCdpUrl('ws://127.0.0.1:9222/devtools/page/ABC123')
  await waitFor(() => wsHolder.last !== null, 'CdpClient to construct a WebSocket')
  const ws = wsHolder.last as FakeWs
  ws.emit('open')
  await Promise.resolve()
  return ws
}

// respondToNextSent waits for ws.sent to grow past previousCount (polling,
// same reasoning as waitFor above), then replies to that newly-sent CDP
// command — the caller passes the count *before* the command it's waiting
// for was sent.
async function respondToNextSent(ws: FakeWs, previousCount: number, result: unknown): Promise<void> {
  await waitFor(() => ws.sent.length > previousCount, `CDP command #${previousCount + 1} to be sent`)
  const lastSent = JSON.parse(ws.sent[previousCount]) as { id: number; method: string }
  ws.emit('message', Buffer.from(JSON.stringify({ id: lastSent.id, result })))
}

// driveHandshake drives the no-viewport handshake (Page.enable ->
// Page.startScreencast) to completion.
async function driveHandshake(ws: FakeWs): Promise<void> {
  await respondToNextSent(ws, 0, {}) // Page.enable
  await respondToNextSent(ws, 1, {}) // Page.startScreencast
}

describe('browser-screencast-handler', () => {
  it('rejects a missing worktree without touching agent-browser', async () => {
    const notify = vi.fn()
    const result = await handleBrowserScreencastStart(1, {}, LOG, notify)
    expect('error' in result).toBe(true)
    expect(execFileMock).not.toHaveBeenCalled()
  })

  it('sends Page.enable then Page.startScreencast, then notifies ready and acks the start', async () => {
    const notify = vi.fn()
    const promise = handleBrowserScreencastStart(
      1,
      { worktree: 'wt-1', format: 'jpeg', quality: 80, everyNthFrame: 3 },
      LOG,
      notify
    )
    const ws = await driveCdpConnect()
    await driveHandshake(ws)

    const result = await promise
    expect('result' in result && result.result).toEqual({ type: 'ack' })

    const sentMethods = ws.sent.map((s) => (JSON.parse(s) as { method: string }).method)
    expect(sentMethods).toEqual(['Page.enable', 'Page.startScreencast'])
    const startScreencastCall = JSON.parse(ws.sent[1]) as { params: Record<string, unknown> }
    expect(startScreencastCall.params).toMatchObject({ format: 'jpeg', quality: 80, everyNthFrame: 3 })

    expect(notify).toHaveBeenCalledWith(
      'browser.screencastReady',
      expect.objectContaining({ worktreeId: 'wt-1', format: 'jpeg', browserPageId: 'ABC123' })
    )
  })

  it('applies Emulation.setDeviceMetricsOverride/setVisibleSize when a viewport is given', async () => {
    const notify = vi.fn()
    const promise = handleBrowserScreencastStart(
      1,
      { worktree: 'wt-1', viewportWidth: 1024, viewportHeight: 768, deviceScaleFactor: 2 },
      LOG,
      notify
    )
    const ws = await driveCdpConnect()

    await respondToNextSent(ws, 0, {}) // Page.enable
    await respondToNextSent(ws, 1, {}) // Emulation.setDeviceMetricsOverride
    await respondToNextSent(ws, 2, {}) // Emulation.setVisibleSize
    await respondToNextSent(ws, 3, {}) // Page.startScreencast
    await promise

    const methods = ws.sent.map((s) => (JSON.parse(s) as { method: string }).method)
    expect(methods).toEqual([
      'Page.enable',
      'Emulation.setDeviceMetricsOverride',
      'Emulation.setVisibleSize',
      'Page.startScreencast'
    ])
    const deviceMetrics = JSON.parse(ws.sent[1]) as { params: Record<string, unknown> }
    expect(deviceMetrics.params).toEqual({ width: 1024, height: 768, deviceScaleFactor: 2, mobile: false })
  })

  it('decodes and notifies a Page.screencastFrame event as an opaque encoded frame, then acks it', async () => {
    const notify = vi.fn()
    const promise = handleBrowserScreencastStart(1, { worktree: 'wt-1', format: 'jpeg' }, LOG, notify)
    const ws = await driveCdpConnect()
    await driveHandshake(ws)
    await promise
    notify.mockClear()

    // Minimal valid JPEG SOF0 header (0xFFD8 SOI, 0xFFC0 SOF0 segment
    // encoding a 2x1 image) — just enough for readBrowserScreencastImageSize
    // to parse real dimensions, proving they reach the frame's metadata.
    const jpegBytes = Buffer.from([
      0xff, 0xd8, 0xff, 0xc0, 0x00, 0x11, 0x08, 0x00, 0x01, 0x00, 0x02, 0x03, 0x01, 0x11, 0x00, 0x02,
      0x11, 0x01, 0x03, 0x11, 0x01, 0xff, 0xd9
    ])

    ws.emit(
      'message',
      Buffer.from(
        JSON.stringify({
          method: 'Page.screencastFrame',
          params: { data: jpegBytes.toString('base64'), sessionId: 42, metadata: { timestamp: 123 } }
        })
      )
    )
    await Promise.resolve()

    expect(notify).toHaveBeenCalledWith(
      'browser.screencastFrame',
      expect.objectContaining({ worktreeId: 'wt-1' })
    )
    const [, frameArgs] = notify.mock.calls.find(([m]) => m === 'browser.screencastFrame') as [string, Record<string, unknown>]
    const encoded = Buffer.from(frameArgs.dataBase64 as string, 'base64')
    const decoded = decodeBrowserScreencastFrame(new Uint8Array(encoded))
    expect(decoded).not.toBeNull()
    expect(decoded?.format).toBe('jpeg')
    expect(decoded?.metadata.imageWidth).toBe(2)
    expect(decoded?.metadata.imageHeight).toBe(1)
    expect(Array.from(decoded?.image ?? [])).toEqual(Array.from(jpegBytes))

    // The frame must be ACKed so CDP keeps streaming.
    const ackCall = ws.sent.map((s) => JSON.parse(s) as { method: string; params: Record<string, unknown> }).at(-1)
    expect(ackCall).toMatchObject({ method: 'Page.screencastFrameAck', params: { sessionId: 42 } })
  })

  it('throttles frames arriving within minFrameIntervalMs (acks but does not notify)', async () => {
    const notify = vi.fn()
    const promise = handleBrowserScreencastStart(
      1,
      { worktree: 'wt-1', minFrameIntervalMs: 1000 },
      LOG,
      notify
    )
    const ws = await driveCdpConnect()
    await driveHandshake(ws)
    await promise
    notify.mockClear()

    const frame = (sessionId: number): void => {
      ws.emit(
        'message',
        Buffer.from(
          JSON.stringify({
            method: 'Page.screencastFrame',
            params: { data: Buffer.from('not-a-real-image').toString('base64'), sessionId }
          })
        )
      )
    }
    frame(1)
    await Promise.resolve()
    frame(2) // arrives immediately after — must be throttled
    await Promise.resolve()

    const frameNotifies = notify.mock.calls.filter(([m]) => m === 'browser.screencastFrame')
    expect(frameNotifies).toHaveLength(1)
    // Both frames must still be individually ACKed even when throttled,
    // so CDP doesn't stall waiting for an ack that never comes.
    const acks = ws.sent
      .map((s) => JSON.parse(s) as { method: string; params: Record<string, unknown> })
      .filter((m) => m.method === 'Page.screencastFrameAck')
    expect(acks.map((a) => a.params.sessionId)).toEqual([1, 2])
  })

  it('handleBrowserScreencastStop sends Page.stopScreencast and notifies ended', async () => {
    const notify = vi.fn()
    const startPromise = handleBrowserScreencastStart(1, { worktree: 'wt-1' }, LOG, notify)
    const ws = await driveCdpConnect()
    await driveHandshake(ws)
    await startPromise
    notify.mockClear()

    const stopResult = handleBrowserScreencastStop(2, { worktree: 'wt-1' }, notify)
    expect('result' in stopResult && stopResult.result).toEqual({ type: 'ack' })
    expect(notify).toHaveBeenCalledWith('browser.screencastEnded', { worktreeId: 'wt-1' })

    await Promise.resolve()
    const methods = ws.sent.map((s) => (JSON.parse(s) as { method: string }).method)
    expect(methods).toContain('Page.stopScreencast')
  })

  it('propagates a CDP connect failure as a synchronous RPC error', async () => {
    const notify = vi.fn()
    const promise = handleBrowserScreencastStart(1, { worktree: 'wt-1' }, LOG, notify)
    await waitFor(() => execFileMock.mock.calls.length > 0, 'agent-browser get cdp-url to be invoked')
    respondCdpUrl('ws://127.0.0.1:9222/devtools/page/ABC123')
    await waitFor(() => wsHolder.last !== null, 'CdpClient to construct a WebSocket')
    wsHolder.last?.emit('error', new Error('connection refused'))

    const result = await promise
    expect('error' in result).toBe(true)
    expect(notify).not.toHaveBeenCalledWith('browser.screencastReady', expect.anything())
  })
})
