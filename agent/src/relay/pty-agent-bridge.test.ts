import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import type { AgentLogger } from './agent-logger'
import { registerTraceSink, type TraceEvent } from '../shared/trace'

// ── Fake node-pty ────────────────────────────────────────────────────────────
// Each spawn() call returns a fresh fake IPty whose onData/onExit callbacks
// are captured so tests can drive output/exit deterministically.
type FakePty = {
  onData: (cb: (data: string) => void) => void
  onExit: (cb: (e: { exitCode: number; signal?: number }) => void) => void
  write: ReturnType<typeof vi.fn>
  resize: ReturnType<typeof vi.fn>
  kill: ReturnType<typeof vi.fn>
  emitData: (data: string) => void
  emitExit: (exitCode: number, signal?: number) => void
}

function makeFakePty(): FakePty {
  let dataCb: ((data: string) => void) | null = null
  let exitCb: ((e: { exitCode: number; signal?: number }) => void) | null = null
  return {
    onData: (cb) => { dataCb = cb },
    onExit: (cb) => { exitCb = cb },
    write: vi.fn(),
    resize: vi.fn(),
    kill: vi.fn(),
    emitData: (data) => dataCb?.(data),
    emitExit: (exitCode, signal) => exitCb?.({ exitCode, signal })
  }
}

let lastSpawnedPty: FakePty | null = null
const spawnMock = vi.fn((..._args: unknown[]) => {
  lastSpawnedPty = makeFakePty()
  return lastSpawnedPty
})

vi.mock('node-pty', () => ({
  spawn: spawnMock
}))

const log: AgentLogger = {
  info: vi.fn(),
  warn: vi.fn(),
  error: vi.fn()
} as unknown as AgentLogger

describe('pty-agent-bridge reattach', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    lastSpawnedPty = null
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('handlePtyCreate spawns a PTY and streams data via notify', async () => {
    const { handlePtyCreate } = await import('./pty-agent-bridge')
    const notify = vi.fn()
    const response = (await handlePtyCreate(1, { cols: 80, rows: 24 }, log, notify)) as {
      result: { id: string }
    }
    expect(response.result.id).toMatch(/^agent-pty-/)
    expect(lastSpawnedPty).not.toBeNull()

    lastSpawnedPty!.emitData('hello\n')
    expect(notify).toHaveBeenCalledWith('pty.data', { id: response.result.id, data: 'hello\n' })
  })

  it('handlePtyAttach returns the scrollback buffer as replay when suppressed', async () => {
    const { handlePtyCreate, handlePtyAttach } = await import('./pty-agent-bridge')
    const notify = vi.fn()
    const created = (await handlePtyCreate(1, {}, log, notify)) as { result: { id: string } }
    const ptyId = created.result.id

    lastSpawnedPty!.emitData('line one\n')

    const attachResponse = (await handlePtyAttach(
      2,
      { id: ptyId, suppressReplayNotification: true },
      log,
      notify
    )) as { result: { replay: string } }

    expect(attachResponse.result.replay).toContain('line one')
    // Suppressed: the notify() used for a normal push must not also fire for this attach.
    expect(notify).not.toHaveBeenCalledWith('pty.replay', expect.anything())
  })

  it('handlePtyAttach pushes a pty.replay notification when not suppressed', async () => {
    const { handlePtyCreate, handlePtyAttach } = await import('./pty-agent-bridge')
    const notify = vi.fn()
    const created = (await handlePtyCreate(1, {}, log, notify)) as { result: { id: string } }
    const ptyId = created.result.id
    lastSpawnedPty!.emitData('buffered output\n')

    await handlePtyAttach(2, { id: ptyId }, log, notify)

    expect(notify).toHaveBeenCalledWith('pty.replay', { id: ptyId, data: expect.stringContaining('buffered output') })
  })

  it('handlePtyAttach rejects an unknown id', async () => {
    const { handlePtyAttach } = await import('./pty-agent-bridge')
    const response = (await handlePtyAttach(1, { id: 'agent-pty-does-not-exist' }, log, vi.fn())) as {
      error: { message: string }
    }
    expect(response.error.message).toContain('not found')
  })

  it('handlePtyAttach rejects on paneKey identity mismatch', async () => {
    const { handlePtyCreate, handlePtyAttach } = await import('./pty-agent-bridge')
    const created = (await handlePtyCreate(1, { paneKey: 'pane-a' }, log, vi.fn())) as {
      result: { id: string }
    }
    const response = (await handlePtyAttach(
      2,
      { id: created.result.id, expectedPaneKey: 'pane-b' },
      log,
      vi.fn()
    )) as { error: { message: string } }
    expect(response.error.message).toContain('identity mismatch')
  })

  it('scheduleGracePeriodCleanup kills the PTY only after the grace period elapses with no attach', async () => {
    const { handlePtyCreate, scheduleGracePeriodCleanup, PTY_GRACE_PERIOD_MS } = await import(
      './pty-agent-bridge'
    )
    const created = (await handlePtyCreate(1, {}, log, vi.fn())) as { result: { id: string } }
    const pty = lastSpawnedPty!

    scheduleGracePeriodCleanup(log)
    expect(pty.kill).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(PTY_GRACE_PERIOD_MS - 1)
    expect(pty.kill).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(2)
    expect(pty.kill).toHaveBeenCalledWith('SIGTERM')
    void created
  })

  it('a reattach within the grace period cancels the pending kill', async () => {
    const { handlePtyCreate, handlePtyAttach, scheduleGracePeriodCleanup, PTY_GRACE_PERIOD_MS } =
      await import('./pty-agent-bridge')
    const created = (await handlePtyCreate(1, {}, log, vi.fn())) as { result: { id: string } }
    const pty = lastSpawnedPty!

    scheduleGracePeriodCleanup(log)
    await vi.advanceTimersByTimeAsync(PTY_GRACE_PERIOD_MS / 2)

    await handlePtyAttach(2, { id: created.result.id, suppressReplayNotification: true }, log, vi.fn())

    await vi.advanceTimersByTimeAsync(PTY_GRACE_PERIOD_MS)
    expect(pty.kill).not.toHaveBeenCalled()
  })

  it('handlePtyDestroy cancels a pending grace timer so it never fires', async () => {
    const { handlePtyCreate, handlePtyDestroy, scheduleGracePeriodCleanup, PTY_GRACE_PERIOD_MS } =
      await import('./pty-agent-bridge')
    const created = (await handlePtyCreate(1, {}, log, vi.fn())) as { result: { id: string } }
    const pty = lastSpawnedPty!

    scheduleGracePeriodCleanup(log)
    await handlePtyDestroy(2, { id: created.result.id }, log)
    pty.kill.mockClear()

    await vi.advanceTimersByTimeAsync(PTY_GRACE_PERIOD_MS + 10)
    // Already destroyed explicitly — the grace timer must not double-kill.
    expect(pty.kill).not.toHaveBeenCalled()
  })

  it('rebinds live output to the reattaching connection\'s notify, not the one PTY was created with', async () => {
    // Regression: term.onData/onExit are registered once at spawn time and
    // must keep pushing for the PTY's whole lifetime, but the notify a
    // caller supplies is bound to one specific WebSocket connection. A
    // reconnect (very common — see pty-daemon-client.ts) hands handlePtyAttach
    // a brand-new notify; before this fix, onData/onExit kept calling the
    // ORIGINAL notify forever, silently dropping all output after any
    // reconnect even though pty.create/pty.attach kept reporting success.
    const { handlePtyCreate, handlePtyAttach } = await import('./pty-agent-bridge')
    const originalNotify = vi.fn()
    const created = (await handlePtyCreate(1, {}, log, originalNotify)) as { result: { id: string } }
    const ptyId = created.result.id
    const pty = lastSpawnedPty!

    pty.emitData('before reconnect\n')
    expect(originalNotify).toHaveBeenCalledWith('pty.data', { id: ptyId, data: 'before reconnect\n' })

    // Simulate a reconnect: a new connection attaches with its own notify.
    const newNotify = vi.fn()
    await handlePtyAttach(2, { id: ptyId, suppressReplayNotification: true }, log, newNotify)

    originalNotify.mockClear()
    pty.emitData('after reconnect\n')
    expect(newNotify).toHaveBeenCalledWith('pty.data', { id: ptyId, data: 'after reconnect\n' })
    expect(originalNotify).not.toHaveBeenCalled()

    newNotify.mockClear()
    pty.emitExit(0)
    expect(newNotify).toHaveBeenCalledWith('pty.exit', { id: ptyId, exitCode: 0, signal: null })
    expect(originalNotify).not.toHaveBeenCalled()
  })
})

describe('pty-agent-bridge — terminal:* tracing (CR-TRACE-003)', () => {
  let events: TraceEvent[]
  let unregister: () => void

  beforeEach(() => {
    vi.clearAllMocks()
    lastSpawnedPty = null
    events = []
    unregister = registerTraceSink((e) => events.push(e))
  })
  afterEach(() => unregister())

  it('handlePtyCreate emits a terminal:create span, ok() contains ptyId/shell/cwd', async () => {
    const { handlePtyCreate } = await import('./pty-agent-bridge')
    const created = (await handlePtyCreate(1, { cols: 80, rows: 24 }, log, vi.fn())) as {
      result: { id: string; shell: string; cwd: string }
    }
    const start = events.find((e) => e.flow === 'terminal:create' && e.level === 'start')
    const ok = events.find((e) => e.flow === 'terminal:create' && e.level === 'ok')
    expect(start?.fields.origin).toBe('agent-pty')
    expect(ok?.fields.ptyId).toBe(created.result.id)
    expect(ok?.fields.shell).toBe(created.result.shell)
    expect(ok?.fields.cwd).toBe(created.result.cwd)
  })

  it('handlePtyAttach emits a terminal:reattach span, ok() contains wasWithinGracePeriod/replayBytes', async () => {
    const { handlePtyCreate, handlePtyAttach } = await import('./pty-agent-bridge')
    const created = (await handlePtyCreate(1, {}, log, vi.fn())) as { result: { id: string } }
    lastSpawnedPty!.emitData('buffered\n')

    await handlePtyAttach(2, { id: created.result.id, suppressReplayNotification: true }, log, vi.fn())

    const ok = events.find((e) => e.flow === 'terminal:reattach' && e.level === 'ok')
    expect(ok?.fields.ptyId).toBe(created.result.id)
    expect(ok?.fields.wasWithinGracePeriod).toBe(false)
    expect(ok?.fields.replayBytes).toBe('buffered\n'.length)
  })

  it('handlePtyAttach emits fail() for an unknown id, never a start-only span', async () => {
    const { handlePtyAttach } = await import('./pty-agent-bridge')
    await handlePtyAttach(1, { id: 'agent-pty-does-not-exist' }, log, vi.fn())
    const fail = events.find((e) => e.flow === 'terminal:reattach' && e.level === 'fail')
    expect(fail?.fields.ptyId).toBe('agent-pty-does-not-exist')
  })

  it('handlePtyResize emits a terminal:resize span with cols/rows', async () => {
    const { handlePtyCreate, handlePtyResize } = await import('./pty-agent-bridge')
    const created = (await handlePtyCreate(1, {}, log, vi.fn())) as { result: { id: string } }
    events = []
    await handlePtyResize(2, { id: created.result.id, cols: 120, rows: 40 }, log)
    const ok = events.find((e) => e.flow === 'terminal:resize' && e.level === 'ok')
    expect(ok?.fields.cols).toBe(120)
    expect(ok?.fields.rows).toBe(40)
  })

  it('handlePtyDestroy emits a terminal:destroy span with the graceful field', async () => {
    const { handlePtyCreate, handlePtyDestroy } = await import('./pty-agent-bridge')
    const created = (await handlePtyCreate(1, {}, log, vi.fn())) as { result: { id: string } }
    events = []
    await handlePtyDestroy(2, { id: created.result.id, graceful: false }, log)
    const ok = events.find((e) => e.flow === 'terminal:destroy' && e.level === 'ok')
    expect(ok?.fields.graceful).toBe(false)
  })

  it('handlePtyDestroy emits ok(alreadyDead=true) when the ptyId is not registered', async () => {
    const { handlePtyDestroy } = await import('./pty-agent-bridge')
    await handlePtyDestroy(1, { id: 'agent-pty-does-not-exist' }, log)
    const ok = events.find((e) => e.flow === 'terminal:destroy' && e.level === 'ok')
    expect(ok?.fields.alreadyDead).toBe(true)
  })

  it('handlePtySendSignal emits a terminal:destroy span for SIGKILL and SIGTERM', async () => {
    const { handlePtyCreate, handlePtySendSignal } = await import('./pty-agent-bridge')
    for (const signal of ['SIGKILL', 'SIGTERM']) {
      const created = (await handlePtyCreate(1, {}, log, vi.fn())) as { result: { id: string } }
      events = []
      await handlePtySendSignal(2, { id: created.result.id, signal }, log)
      const start = events.find((e) => e.flow === 'terminal:destroy' && e.level === 'start')
      const ok = events.find((e) => e.flow === 'terminal:destroy' && e.level === 'ok')
      expect(start?.fields.via).toBe('pty.sendSignal')
      expect(ok?.fields.signal).toBe(signal)
    }
  })

  it('handlePtySendSignal emits NO span for SIGINT/SIGHUP/SIGTSTP', async () => {
    const { handlePtyCreate, handlePtySendSignal } = await import('./pty-agent-bridge')
    const created = (await handlePtyCreate(1, {}, log, vi.fn())) as { result: { id: string } }
    events = []
    for (const signal of ['SIGINT', 'SIGHUP', 'SIGTSTP']) {
      await handlePtySendSignal(2, { id: created.result.id, signal }, log)
    }
    expect(events.filter((e) => e.flow === 'terminal:destroy')).toHaveLength(0)
  })

  it('handlePtyWrite does NOT emit any trace span regardless of call count', async () => {
    const { handlePtyCreate, handlePtyWrite } = await import('./pty-agent-bridge')
    const created = (await handlePtyCreate(1, {}, log, vi.fn())) as { result: { id: string } }
    events = []
    for (let i = 0; i < 20; i++) {await handlePtyWrite(i, { id: created.result.id, data: 'a' }, log)}
    expect(events.filter((e) => e.flow.startsWith('terminal:'))).toHaveLength(0)
  })

  it('handlePtyScrollback does NOT emit any trace span', async () => {
    const { handlePtyCreate, handlePtyScrollback } = await import('./pty-agent-bridge')
    const created = (await handlePtyCreate(1, {}, log, vi.fn())) as { result: { id: string } }
    events = []
    await handlePtyScrollback(1, { id: created.result.id }, log)
    expect(events.filter((e) => e.flow.startsWith('terminal:'))).toHaveLength(0)
  })

  it('resumes span id from params._trace.id when present', async () => {
    const { handlePtyCreate, handlePtyResize } = await import('./pty-agent-bridge')
    const created = (await handlePtyCreate(1, {}, log, vi.fn())) as { result: { id: string } }
    events = []
    await handlePtyResize(2, { id: created.result.id, cols: 10, rows: 10, _trace: { id: 'resumed-1' } }, log)
    const start = events.find((e) => e.flow === 'terminal:resize' && e.level === 'start')
    expect(start?.id).toBe('resumed-1')
  })

  it('generates a new span id when params._trace is absent', async () => {
    const { handlePtyCreate, handlePtyResize } = await import('./pty-agent-bridge')
    const created = (await handlePtyCreate(1, {}, log, vi.fn())) as { result: { id: string } }
    events = []
    await handlePtyResize(2, { id: created.result.id, cols: 10, rows: 10 }, log)
    const start = events.find((e) => e.flow === 'terminal:resize' && e.level === 'start')
    expect(start?.id).toBeTruthy()
    expect(start?.id).not.toBe('resumed-1')
  })
})

// Why: gh/glab auth-login PTYs (and any other commandDelivery:'provider'
// caller — see dev-server-pty-provider.ts) have no renderer terminal pane to
// type the command into; the agent must submit it. Previously unimplemented
// — see specs/agent/api/gaps-and-findings.md #5.
describe('pty-agent-bridge — provider-delivered startup command', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    lastSpawnedPty = null
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('writes the command to the PTY shortly after spawn when commandDelivery is provider', async () => {
    const { handlePtyCreate } = await import('./pty-agent-bridge')
    await handlePtyCreate(
      1,
      { command: 'gh auth login', commandDelivery: 'provider' },
      log,
      vi.fn()
    )
    expect(lastSpawnedPty!.write).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(50)

    expect(lastSpawnedPty!.write).toHaveBeenCalledOnce()
    const written = lastSpawnedPty!.write.mock.calls[0]![0] as string
    expect(written).toContain('gh auth login')
  })

  it('does not write a command when commandDelivery is the default (renderer)', async () => {
    const { handlePtyCreate } = await import('./pty-agent-bridge')
    await handlePtyCreate(1, { command: 'gh auth login' }, log, vi.fn())

    await vi.advanceTimersByTimeAsync(200)

    expect(lastSpawnedPty!.write).not.toHaveBeenCalled()
  })

  it('does not throw when the PTY exits before the delivery timer fires', async () => {
    const { handlePtyCreate } = await import('./pty-agent-bridge')
    const created = (await handlePtyCreate(
      1,
      { command: 'gh auth login', commandDelivery: 'provider' },
      log,
      vi.fn()
    )) as { result: { id: string } }
    lastSpawnedPty!.emitExit(0)

    await vi.advanceTimersByTimeAsync(50)

    expect(lastSpawnedPty!.write).not.toHaveBeenCalled()
    expect(created.result.id).toMatch(/^agent-pty-/)
  })

  // Why: BUG-BE-HLD-005 parity fix — this isolation previously only existed
  // on the SSH relay (pty-handler.ts); Part A had no gh/glab auth-login
  // support at all before this fix, so there was nothing to isolate.
  it('isolates GH_CONFIG_DIR per user for a provider-delivered gh command', async () => {
    const { handlePtyCreate } = await import('./pty-agent-bridge')
    await handlePtyCreate(
      1,
      { command: 'gh auth login', commandDelivery: 'provider', userId: 'user-42' },
      log,
      vi.fn()
    )

    const spawnEnv = spawnMock.mock.calls[0]?.[2]?.env as Record<string, string>
    expect(spawnEnv.GH_CONFIG_DIR).toContain('/user-42/')
  })

  it('does not isolate GH_CONFIG_DIR when userId is absent', async () => {
    const { handlePtyCreate } = await import('./pty-agent-bridge')
    await handlePtyCreate(
      1,
      { command: 'gh auth login', commandDelivery: 'provider' },
      log,
      vi.fn()
    )

    const spawnEnv = spawnMock.mock.calls[0]?.[2]?.env as Record<string, string>
    expect(spawnEnv.GH_CONFIG_DIR).toBeUndefined()
  })
})
