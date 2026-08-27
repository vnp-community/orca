import { describe, expect, it, vi, beforeEach } from 'vitest'
import { EventEmitter } from 'node:events'
import { spawn } from 'node:child_process'
import type * as ChildProcess from 'node:child_process'
import { tmpdir } from 'node:os'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { handleAgentExecPrompt } from './agent-print-mode-exec'

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
    if (spawnMock.mock.calls.length > 0) {return}
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
})
