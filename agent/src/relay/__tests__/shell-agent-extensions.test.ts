// src/relay/__tests__/shell-agent-extensions.test.ts
// Split out of fs-agent-extensions.test.ts to mirror the shell-agent-extensions.ts
// source split (max-lines ratchet) — pure test move, no behavior change.
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { EventEmitter } from 'node:events'
import { spawn } from 'node:child_process'
import type * as ChildProcess from 'node:child_process'
import { join } from 'node:path'
import { mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { createWireState, decodeFrame } from 'orca-dev-agent-transport'
import { handleShellExec, handleShellExecStream } from '../shell-agent-extensions'
import type { AgentConfig } from '../agent-config'

vi.mock('node:child_process', async (importOriginal) => {
  const actual = await importOriginal<typeof ChildProcess>()
  return { ...actual, spawn: vi.fn(actual.spawn) }
})
const spawnMock = vi.mocked(spawn)

type FakeChild = EventEmitter & {
  stdout: EventEmitter
  stderr: EventEmitter
  kill: ReturnType<typeof vi.fn>
}
function createFakeChild(): FakeChild {
  return Object.assign(new EventEmitter(), {
    stdout: new EventEmitter(),
    stderr: new EventEmitter(),
    kill: vi.fn()
  })
}

function makeConfig(): AgentConfig {
  const tmpDir = mkdtempSync(join(tmpdir(), 'fs-ext-test-'))
  return { workDir: tmpDir, toolEnv: { PATH: '/usr/bin' } } as unknown as AgentConfig
}

// ─── handleShellExec ──────────────────────────────────────────────────────────
describe('handleShellExec', () => {
  beforeEach(() => {
    spawnMock.mockClear()
  })

  it('emits shell.exec.output notifications for each stdout/stderr chunk', async () => {
    const child = createFakeChild()
    // mockReturnValueOnce overrides the default (real spawn) for this one call only.
    spawnMock.mockReturnValueOnce(child as never)
    const notify = vi.fn()

    const pending = handleShellExec(
      1,
      { script: 'true', traceId: 'trace-abc' },
      makeConfig(),
      notify
    )
    child.stdout.emit('data', Buffer.from('out-1'))
    child.stderr.emit('data', Buffer.from('err-1'))
    child.emit('close', 0)
    await pending

    expect(notify).toHaveBeenCalledWith('shell.exec.output', {
      traceId: 'trace-abc',
      stream: 'stdout',
      data: 'out-1'
    })
    expect(notify).toHaveBeenCalledWith('shell.exec.output', {
      traceId: 'trace-abc',
      stream: 'stderr',
      data: 'err-1'
    })
  })

  it('still resolves with the unchanged response shape when notify is omitted', async () => {
    const child = createFakeChild()
    spawnMock.mockReturnValueOnce(child as never)

    const pending = handleShellExec(2, { script: 'true' }, makeConfig())
    child.emit('close', 0)
    const result = (await pending) as { result?: { exitCode: number } }
    expect(result.result?.exitCode).toBe(0)
  })
})

// ─── handleShellExecStream (CR-TG-006) ─────────────────────────────────────
// Mirrors agent-ephemeral-vm-handler.test.ts's describe('handleVmProvision', ...)
// pattern — the one real streaming-handler test precedent in this codebase —
// using a local MockWs + orca-dev-agent-transport's createWireState/decodeFrame
// to capture and decode every frame the handler sends. Real `sh -c` spawns are
// used (no child_process mock), same as fs-agent-search-extensions.test.ts's
// handlePreflightCheck tests which spawn real binaries.
class MockWs {
  readyState = 1
  sent: Buffer[] = []
  send = vi.fn((frame: Buffer) => {
    this.sent.push(frame)
  })
}

type StreamFrame = {
  id?: string | number | null
  result?: { type: string; line?: string; source?: string; exitCode?: number }
  error?: { message: string; code?: number }
}

function decodeSentFrames(ws: MockWs): StreamFrame[] {
  const receiver = createWireState()
  return ws.sent.map((frame) => {
    const decoded = decodeFrame(receiver, frame)!
    return JSON.parse(decoded.payload.toString('utf8')) as StreamFrame
  })
}

// handleShellExecStream is fire-and-forget (same convention as
// handleGitExecStream/handleVmProvision) — it returns before the real child
// process closes, so awaiting the call itself does not wait for stream.end.
// Poll ws.sent until a stream.end frame (or an error frame) has arrived.
async function waitForStreamEnd(ws: MockWs, timeoutMs = 5_000): Promise<void> {
  const start = Date.now()
  while (Date.now() - start < timeoutMs) {
    const frames = decodeSentFrames(ws)
    if (frames.some((f) => f.result?.type === 'stream.end' || f.error)) {
      return
    }
    await new Promise((r) => setTimeout(r, 10))
  }
  throw new Error('stream.end was never sent')
}

describe('handleShellExecStream', () => {
  it('sends one stream.chunk frame per stdout/stderr data event, then one stream.end, all sharing the request id', async () => {
    const ws = new MockWs()
    const wireState = createWireState()

    void handleShellExecStream(
      ws as unknown as never,
      wireState,
      'req-1',
      { script: 'printf "out line"; printf "err line" 1>&2' },
      makeConfig()
    )
    await waitForStreamEnd(ws)

    const frames = decodeSentFrames(ws)
    const chunkFrames = frames.filter((f) => f.result?.type === 'stream.chunk')
    const endFrames = frames.filter((f) => f.result?.type === 'stream.end')
    expect(endFrames).toHaveLength(1)
    expect(endFrames[0]).toMatchObject({ id: 'req-1', result: { type: 'stream.end', exitCode: 0 } })
    expect(chunkFrames.some((f) => f.result?.line === 'out line' && !f.result.source)).toBe(true)
    expect(
      chunkFrames.some((f) => f.result?.line === 'err line' && f.result.source === 'stderr')
    ).toBe(true)
    // stream.end must be the last frame sent.
    expect(frames.at(-1)).toBe(endFrames[0])
  }, 10_000)

  it('sends a single error frame (no stream.chunk/stream.end) for a missing script', async () => {
    const ws = new MockWs()
    const wireState = createWireState()

    await handleShellExecStream(ws as unknown as never, wireState, 'req-2', {}, makeConfig())

    const frames = decodeSentFrames(ws)
    expect(frames).toHaveLength(1)
    expect(frames[0].error?.message).toContain('Missing required param: script')
  })

  it('kills the child and sends stream.end with exitCode -1 on timeout', async () => {
    const ws = new MockWs()
    const wireState = createWireState()

    void handleShellExecStream(
      ws as unknown as never,
      wireState,
      'req-3',
      { script: 'sleep 5', timeoutMs: 1_000 },
      makeConfig()
    )
    await waitForStreamEnd(ws)

    const frames = decodeSentFrames(ws)
    expect(frames).toHaveLength(1)
    expect(frames[0]).toMatchObject({ id: 'req-3', result: { type: 'stream.end', exitCode: -1 } })
  }, 10_000)

  it('propagates the real exit code of a failing script', async () => {
    const ws = new MockWs()
    const wireState = createWireState()

    void handleShellExecStream(
      ws as unknown as never,
      wireState,
      'req-4',
      { script: 'exit 7' },
      makeConfig()
    )
    await waitForStreamEnd(ws)

    const frames = decodeSentFrames(ws)
    expect(frames.at(-1)).toMatchObject({
      id: 'req-4',
      result: { type: 'stream.end', exitCode: 7 }
    })
  }, 10_000)
})
