import { describe, expect, it, vi, beforeEach } from 'vitest'
import { EventEmitter } from 'node:events'
import { spawn } from 'node:child_process'
import type * as ChildProcess from 'node:child_process'
import { tmpdir } from 'node:os'
import { createWireState, decodeFrame } from 'orca-dev-agent-transport'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { handleAgentExecPrompt, handleAgentExecPromptStream } from './agent-print-mode-exec'

const readDecryptedKeyMock = vi.hoisted(() => vi.fn(async (): Promise<string | null> => null))
vi.mock('./agent-credential-store', () => ({ readDecryptedKey: readDecryptedKeyMock }))

vi.mock('node:child_process', async (importOriginal) => {
  const actual = await importOriginal<typeof ChildProcess>()
  return { ...actual, spawn: vi.fn() }
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

// handleAgentExecPrompt awaits buildAgentEnv() before calling spawn() (unlike
// the older agent-exec-handler.ts, which spawns synchronously) — so spawn()
// isn't called until at least one microtask tick after the handler starts.
// Flush ticks until it's actually been invoked before emitting on the fake
// child, or the emitted events fire before any listener is attached.
async function waitForSpawn(): Promise<void> {
  for (let i = 0; i < 50; i++) {
    if (spawnMock.mock.calls.length > 0) {
      return
    }
    await Promise.resolve()
  }
  throw new Error('spawn() was never called')
}

const MOCK_CONFIG: AgentConfig = {
  mode: 'direct-websocket',
  orcaUrl: '',
  agentToken: '',
  agentPort: 6799,
  devServerId: 'test-server',
  logLevel: 'info',
  workDir: tmpdir(),
  toolPath: '/usr/local/bin:/usr/bin:/bin',
  toolEnv: { PATH: '/usr/local/bin:/usr/bin:/bin' },
  credentialDir: tmpdir(),
  tlsRejectUnauthorized: true
}

const MOCK_LOG: AgentLogger = {
  info: vi.fn(),
  warn: vi.fn(),
  error: vi.fn(),
  debug: vi.fn()
}

// Why: StepExecutors.ts/ProfileAwareAgentSpawner.ts previously sent this exact
// shape to agent.exec (which only accepts {binary,args,cwd,stdin,env,timeoutMs})
// and always failed with InvalidParams. See specs/agent/api/gaps-and-findings.md.
describe('handleAgentExecPrompt', () => {
  beforeEach(() => {
    spawnMock.mockReset()
    readDecryptedKeyMock.mockClear()
  })

  it('rejects a missing prompt', async () => {
    const result = (await handleAgentExecPrompt(
      1,
      { worktreePath: '/repo' },
      MOCK_CONFIG,
      MOCK_LOG
    )) as { error?: { message: string } }

    expect(result.error?.message).toContain('missing required field(s): prompt')
    expect(spawnMock).not.toHaveBeenCalled()
  })

  it('rejects a missing worktreePath', async () => {
    const result = (await handleAgentExecPrompt(
      1,
      { prompt: 'do the thing' },
      MOCK_CONFIG,
      MOCK_LOG
    )) as { error?: { message: string } }

    expect(result.error?.message).toContain('missing required field(s): worktreePath')
  })

  it('rejects an unsupported (non-claude) model with UNSUPPORTED_MODEL_FOR_ONE_SHOT_EXEC', async () => {
    const result = (await handleAgentExecPrompt(
      1,
      { prompt: 'do the thing', worktreePath: '/repo', model: 'gpt-4o' },
      MOCK_CONFIG,
      MOCK_LOG
    )) as { error?: { message: string } }

    expect(result.error?.message).toContain('UNSUPPORTED_MODEL_FOR_ONE_SHOT_EXEC')
    expect(spawnMock).not.toHaveBeenCalled()
  })

  it('invokes claude in --print mode with the prompt as an argv element', async () => {
    const child = createFakeChild()
    spawnMock.mockReturnValue(child as never)

    const pending = handleAgentExecPrompt(
      1,
      { prompt: 'fix the bug', worktreePath: '/repo' },
      MOCK_CONFIG,
      MOCK_LOG
    )
    await waitForSpawn()
    child.stdout.emit('data', Buffer.from('done'))
    child.emit('close', 0)

    const result = (await pending) as { result?: { stdout: string; exitCode: number } }
    expect(result.result).toMatchObject({ stdout: 'done', exitCode: 0, timedOut: false })
    expect(spawnMock).toHaveBeenCalledWith(
      'claude',
      ['--print', 'fix the bug'],
      expect.objectContaining({ cwd: '/repo', stdio: ['ignore', 'pipe', 'pipe'] })
    )
  })

  it("appends YOLO_TUI_AGENT_ARGS.claude when trustPreset is 'full'", async () => {
    const child = createFakeChild()
    spawnMock.mockReturnValue(child as never)

    const pending = handleAgentExecPrompt(
      1,
      { prompt: 'fix the bug', worktreePath: '/repo', trustPreset: 'full' },
      MOCK_CONFIG,
      MOCK_LOG
    )
    await waitForSpawn()
    child.emit('close', 0)
    await pending

    expect(spawnMock).toHaveBeenCalledWith(
      'claude',
      ['--print', 'fix the bug', '--dangerously-skip-permissions'],
      expect.anything()
    )
  })

  it("does not append the skip-permissions flag for trustPreset 'default'/'standard'/'none'", async () => {
    const child = createFakeChild()
    spawnMock.mockReturnValue(child as never)

    const pending = handleAgentExecPrompt(
      1,
      { prompt: 'fix the bug', worktreePath: '/repo', trustPreset: 'default' },
      MOCK_CONFIG,
      MOCK_LOG
    )
    await waitForSpawn()
    child.emit('close', 0)
    await pending

    expect(spawnMock).toHaveBeenCalledWith('claude', ['--print', 'fix the bug'], expect.anything())
  })

  it('merges caller-provided env on top of the base env', async () => {
    const child = createFakeChild()
    spawnMock.mockReturnValue(child as never)

    const pending = handleAgentExecPrompt(
      1,
      { prompt: 'fix the bug', worktreePath: '/repo', env: { ORCA_PROJECT_ID: 'proj-1' } },
      MOCK_CONFIG,
      MOCK_LOG
    )
    await waitForSpawn()
    child.emit('close', 0)
    await pending

    const spawnEnv = spawnMock.mock.calls[0]?.[2]?.env as Record<string, string>
    expect(spawnEnv.ORCA_PROJECT_ID).toBe('proj-1')
  })

  it('throws PermissionDenied when accountId is set but no credential/resolvedApiKey exists', async () => {
    readDecryptedKeyMock.mockResolvedValueOnce(null)
    const result = (await handleAgentExecPrompt(
      1,
      { prompt: 'fix the bug', worktreePath: '/repo', accountId: 'acct-1' },
      MOCK_CONFIG,
      MOCK_LOG
    )) as { error?: { code: number; message: string } }

    expect(result.error?.message).toContain('no credential found for accountId=acct-1')
    expect(spawnMock).not.toHaveBeenCalled()
  })

  it('surfaces a non-zero exit code without throwing', async () => {
    const child = createFakeChild()
    spawnMock.mockReturnValue(child as never)

    const pending = handleAgentExecPrompt(
      1,
      { prompt: 'fix the bug', worktreePath: '/repo' },
      MOCK_CONFIG,
      MOCK_LOG
    )
    await waitForSpawn()
    child.stderr.emit('data', Buffer.from('boom'))
    child.emit('close', 1)

    const result = (await pending) as { result?: { exitCode: number; stderr: string } }
    expect(result.result).toMatchObject({ exitCode: 1, stderr: 'boom' })
  })

  it('includes stepId in the result when provided', async () => {
    const child = createFakeChild()
    spawnMock.mockReturnValue(child as never)

    const pending = handleAgentExecPrompt(
      1,
      { prompt: 'fix the bug', worktreePath: '/repo', stepId: 'step-7' },
      MOCK_CONFIG,
      MOCK_LOG
    )
    await waitForSpawn()
    child.emit('close', 0)

    const result = (await pending) as { result?: { stepId?: string } }
    expect(result.result?.stepId).toBe('step-7')
  })

  it('emits agent.execPrompt.output notifications for each stdout/stderr chunk', async () => {
    const child = createFakeChild()
    spawnMock.mockReturnValue(child as never)
    const notify = vi.fn()

    const pending = handleAgentExecPrompt(
      1,
      { prompt: 'hi', worktreePath: '/repo', stepId: 'step-42' },
      MOCK_CONFIG,
      MOCK_LOG,
      notify
    )
    await waitForSpawn()
    child.stdout.emit('data', Buffer.from('chunk-1'))
    child.stderr.emit('data', Buffer.from('warn-1'))
    child.emit('close', 0)
    await pending

    expect(notify).toHaveBeenCalledWith('agent.execPrompt.output', {
      stepId: 'step-42',
      stream: 'stdout',
      data: 'chunk-1'
    })
    expect(notify).toHaveBeenCalledWith('agent.execPrompt.output', {
      stepId: 'step-42',
      stream: 'stderr',
      data: 'warn-1'
    })
  })

  it('still resolves with the unchanged response shape when notify is omitted', async () => {
    const child = createFakeChild()
    spawnMock.mockReturnValue(child as never)

    const pending = handleAgentExecPrompt(
      2,
      { prompt: 'hi', worktreePath: '/repo' },
      MOCK_CONFIG,
      MOCK_LOG
      // notify omitted — backward-compat, existing callers don't pass it
    )
    await waitForSpawn()
    child.stdout.emit('data', Buffer.from('done'))
    child.emit('close', 0)
    const result = (await pending) as { result?: { stdout: string } }
    expect(result.result?.stdout).toBe('done')
  })

  it('prefers explicit params.taskId/projectId over stepId when building env', async () => {
    const child = createFakeChild()
    spawnMock.mockReturnValue(child as never)

    const pending = handleAgentExecPrompt(
      1,
      {
        prompt: 'hi',
        worktreePath: '/repo',
        stepId: 'req-1',
        taskId: 'task-42',
        projectId: 'proj-7'
      },
      MOCK_CONFIG,
      MOCK_LOG
    )
    await waitForSpawn()
    child.emit('close', 0)
    await pending

    const spawnEnv = spawnMock.mock.calls[0]?.[2]?.env as Record<string, string>
    expect(spawnEnv.ORCA_TASK_ID).toBe('task-42')
    expect(spawnEnv.ORCA_PROJECT_ID).toBe('proj-7')
  })

  it('falls back to stepId as taskId when params.taskId is absent (backward-compat)', async () => {
    const child = createFakeChild()
    spawnMock.mockReturnValue(child as never)

    const pending = handleAgentExecPrompt(
      1,
      { prompt: 'hi', worktreePath: '/repo', stepId: 'req-1' },
      MOCK_CONFIG,
      MOCK_LOG
    )
    await waitForSpawn()
    child.emit('close', 0)
    await pending

    const spawnEnv = spawnMock.mock.calls[0]?.[2]?.env as Record<string, string>
    expect(spawnEnv.ORCA_TASK_ID).toBe('req-1')
  })
})

// ─── handleAgentExecPromptStream (CR-TG-006) ───────────────────────────────
// Mirrors agent-ephemeral-vm-handler.test.ts's describe('handleVmProvision', ...)
// pattern — the one real streaming-handler test precedent in this codebase —
// using a local MockWs + orca-dev-agent-transport's createWireState/decodeFrame
// to capture and decode every frame the handler sends.
class MockWs {
  readyState = 1
  sent: Buffer[] = []
  send = vi.fn((frame: Buffer) => {
    this.sent.push(frame)
  })
}

function decodeSentFrames(ws: MockWs): unknown[] {
  const receiver = createWireState()
  return ws.sent.map((frame) => {
    const decoded = decodeFrame(receiver, frame)!
    return JSON.parse(decoded.payload.toString('utf8'))
  })
}

describe('handleAgentExecPromptStream', () => {
  beforeEach(() => {
    spawnMock.mockReset()
    readDecryptedKeyMock.mockClear()
  })

  it('sends one stream.chunk frame per stdout/stderr data event, then one stream.end, all sharing the request id', async () => {
    const child = createFakeChild()
    spawnMock.mockReturnValue(child as never)
    const ws = new MockWs()
    const wireState = createWireState()

    const pending = handleAgentExecPromptStream(
      ws as unknown as never,
      wireState,
      'req-1',
      { prompt: 'fix the bug', worktreePath: '/repo' },
      MOCK_CONFIG,
      MOCK_LOG
    )
    await waitForSpawn()
    child.stdout.emit('data', Buffer.from('line 1'))
    child.stdout.emit('data', Buffer.from('line 2'))
    child.stderr.emit('data', Buffer.from('warn 1'))
    child.emit('close', 0)
    await pending

    const frames = decodeSentFrames(ws)
    expect(frames).toHaveLength(4)
    expect(frames[0]).toMatchObject({
      id: 'req-1',
      result: { type: 'stream.chunk', line: 'line 1' }
    })
    expect(frames[1]).toMatchObject({
      id: 'req-1',
      result: { type: 'stream.chunk', line: 'line 2' }
    })
    expect(frames[2]).toMatchObject({
      id: 'req-1',
      result: { type: 'stream.chunk', line: 'warn 1', source: 'stderr' }
    })
    expect(frames[3]).toMatchObject({ id: 'req-1', result: { type: 'stream.end', exitCode: 0 } })
  })

  it('forwards raw data events verbatim without line-splitting (Open Question 1 resolution)', async () => {
    const child = createFakeChild()
    spawnMock.mockReturnValue(child as never)
    const ws = new MockWs()
    const wireState = createWireState()

    const pending = handleAgentExecPromptStream(
      ws as unknown as never,
      wireState,
      'req-2',
      { prompt: 'fix the bug', worktreePath: '/repo' },
      MOCK_CONFIG,
      MOCK_LOG
    )
    await waitForSpawn()
    child.stdout.emit('data', Buffer.from('partial line without newline\nsecond'))
    child.emit('close', 0)
    await pending

    const frames = decodeSentFrames(ws) as { result?: { line?: string } }[]
    expect(frames[0]?.result?.line).toBe('partial line without newline\nsecond')
  })

  it('sends a single error frame (no stream.chunk/stream.end) for a missing prompt', async () => {
    const ws = new MockWs()
    const wireState = createWireState()

    await handleAgentExecPromptStream(
      ws as unknown as never,
      wireState,
      'req-3',
      { worktreePath: '/repo' },
      MOCK_CONFIG,
      MOCK_LOG
    )

    const frames = decodeSentFrames(ws) as { error?: { message: string } }[]
    expect(frames).toHaveLength(1)
    expect(frames[0]?.error?.message).toContain('missing required field(s): prompt')
    expect(spawnMock).not.toHaveBeenCalled()
  })

  it('sends a single error frame for an unsupported model', async () => {
    const ws = new MockWs()
    const wireState = createWireState()

    await handleAgentExecPromptStream(
      ws as unknown as never,
      wireState,
      'req-4',
      { prompt: 'do the thing', worktreePath: '/repo', model: 'gpt-4o' },
      MOCK_CONFIG,
      MOCK_LOG
    )

    const frames = decodeSentFrames(ws) as { error?: { message: string } }[]
    expect(frames).toHaveLength(1)
    expect(frames[0]?.error?.message).toContain('UNSUPPORTED_MODEL_FOR_ONE_SHOT_EXEC')
    expect(spawnMock).not.toHaveBeenCalled()
  })

  it('kills the child and sends stream.end with exitCode -1 on timeout (Open Question 2 resolution)', async () => {
    vi.useFakeTimers()
    try {
      const child = createFakeChild()
      spawnMock.mockReturnValue(child as never)
      const ws = new MockWs()
      const wireState = createWireState()

      const pending = handleAgentExecPromptStream(
        ws as unknown as never,
        wireState,
        'req-5',
        { prompt: 'fix the bug', worktreePath: '/repo', timeoutMs: 1_000 },
        MOCK_CONFIG,
        MOCK_LOG
      )
      // Flush the buildAgentEnv() microtask chain before spawn() is invoked.
      for (let i = 0; i < 50 && spawnMock.mock.calls.length === 0; i++) {
        await Promise.resolve()
      }
      await vi.advanceTimersByTimeAsync(1_000)
      await pending

      expect(child.kill).toHaveBeenCalledWith('SIGKILL')
      const frames = decodeSentFrames(ws)
      expect(frames).toHaveLength(1)
      expect(frames[0]).toMatchObject({ id: 'req-5', result: { type: 'stream.end', exitCode: -1 } })
    } finally {
      vi.useRealTimers()
    }
  })
})
