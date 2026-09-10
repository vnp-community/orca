// src/relay/__tests__/shell-agent-extensions.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { EventEmitter } from 'node:events'
import { spawn } from 'node:child_process'
import type * as ChildProcess from 'node:child_process'
import { handleShellExec } from '../shell-agent-extensions'
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
  return { workDir: '/tmp', toolEnv: { PATH: '/usr/bin' } } as unknown as AgentConfig
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
