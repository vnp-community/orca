import { afterAll, beforeAll, beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { createHash } from 'node:crypto'
import { chmod, mkdtemp, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { Writable } from 'node:stream'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import type * as ReadonlyToolPolicy from './agent-readonly-tool-policy'
import { handleAgentExecPrompt } from './agent-print-mode-exec'
import { promptViaArgvEnabled, writePromptToStdin } from './agent-claude-prompt-input'

// Real spawn() against a fake `claude` executable: proves what the child sees on argv and stdin,
// which the mocked-spawn tests in agent-print-mode-exec.test.ts cannot.
vi.mock('./agent-credential-store', () => ({ readDecryptedKey: vi.fn(async () => null) }))
vi.mock('./agent-readonly-tool-policy', async (importOriginal) => {
  const actual = await importOriginal<typeof ReadonlyToolPolicy>()
  return { ...actual, detectClaudeFlags: vi.fn(async () => null) }
})

// Reports argv, the stdin it received (hash + length, text when small) and its own process-list entry.
// FAKE_CLAUDE_MODE=exit-early makes it leave without reading stdin, which is what produces EPIPE.
const FAKE_CLAUDE = `#!${process.execPath}
const { createHash } = require('node:crypto')
const { execFileSync } = require('node:child_process')
if (process.env.FAKE_CLAUDE_MODE === 'exit-early') process.exit(7)
const chunks = []
process.stdin.on('data', (c) => chunks.push(c))
process.stdin.on('end', () => {
  const stdin = Buffer.concat(chunks)
  let psArgs = ''
  try { psArgs = execFileSync('ps', ['-o', 'args=', '-p', String(process.pid)], { encoding: 'utf8' }) } catch {}
  process.stdout.write(JSON.stringify({
    argv: process.argv.slice(2),
    stdinBytes: stdin.length,
    stdinSha256: createHash('sha256').update(stdin).digest('hex'),
    stdinText: stdin.length <= 4096 ? stdin.toString('utf8') : null,
    psArgs
  }))
})
`

type FakeReport = {
  argv: string[]
  stdinBytes: number
  stdinSha256: string
  stdinText: string | null
  psArgs: string
}

const posixOnly = process.platform === 'win32' ? describe.skip : describe
const sha = (text: string): string =>
  createHash('sha256').update(Buffer.from(text, 'utf8')).digest('hex')

let fakeDir: string
let workDir: string
let config: AgentConfig
let log: AgentLogger
const savedSwitch = process.env['ORCA_AGENT_PROMPT_VIA_ARGV']

beforeAll(async () => {
  fakeDir = await mkdtemp(path.join(tmpdir(), 'orca-fake-claude-'))
  workDir = await mkdtemp(path.join(tmpdir(), 'orca-stdin-work-'))
  if (process.platform !== 'win32') {
    const bin = path.join(fakeDir, 'claude')
    await writeFile(bin, FAKE_CLAUDE)
    await chmod(bin, 0o755)
  }
  config = {
    mode: 'direct-websocket',
    orcaUrl: '',
    orcaHttpUrl: '',
    apiSecret: '',
    agentToken: '',
    agentPort: 0,
    devServerId: 'test',
    logLevel: 'info',
    workDir,
    toolPath: `${fakeDir}${path.delimiter}${process.env['PATH'] ?? ''}`,
    toolEnv: {},
    credentialDir: workDir,
    tlsRejectUnauthorized: true
  } as unknown as AgentConfig
})

afterAll(async () => {
  await rm(fakeDir, { recursive: true, force: true })
  await rm(workDir, { recursive: true, force: true })
})

beforeEach(() => {
  delete process.env['ORCA_AGENT_PROMPT_VIA_ARGV']
  log = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }
})

afterEach(() => {
  if (savedSwitch === undefined) {
    delete process.env['ORCA_AGENT_PROMPT_VIA_ARGV']
  } else {
    process.env['ORCA_AGENT_PROMPT_VIA_ARGV'] = savedSwitch
  }
})

async function run(
  prompt: string,
  extra: Record<string, unknown> = {}
): Promise<{ stdout: string; exitCode: number | null; report: FakeReport | null }> {
  const res = (await handleAgentExecPrompt(
    1,
    {
      prompt,
      worktreePath: workDir,
      workspaceKind: 'scratch',
      stepId: 's1',
      timeoutMs: 30_000,
      ...extra
    },
    config,
    log
  )) as { result?: { stdout: string; exitCode: number | null }; error?: unknown }
  expect(res.error).toBeUndefined()
  const stdout = res.result?.stdout ?? ''
  let report: FakeReport | null = null
  try {
    report = JSON.parse(stdout) as FakeReport
  } catch {
    report = null
  }
  return { stdout, exitCode: res.result?.exitCode ?? null, report }
}

posixOnly('agent.execPrompt delivers the prompt on stdin (real child process)', () => {
  it('passes a normal prompt on stdin and keeps only --print on argv', async () => {
    const { report, exitCode } = await run('fix the bug')
    expect(exitCode).toBe(0)
    expect(report?.argv).toEqual(['--print'])
    expect(report?.stdinText).toBe('fix the bug')
  })

  it('composes initFile + newline + prompt exactly as the argv path did', async () => {
    const { report } = await run('fix the bug', { initFile: '# Context\nTeam: platform\n' })
    expect(report?.stdinText).toBe('# Context\nTeam: platform\n\nfix the bug')
  })

  it('delivers a 500 KB prompt intact and never shows it in argv or the process list', async () => {
    const marker = 'PROMPT-BODY-MARKER-'
    const prompt = `${marker}${'x'.repeat(500 * 1024)}`
    const { report, exitCode } = await run(prompt)
    expect(exitCode).toBe(0)
    expect(report?.stdinBytes).toBe(Buffer.byteLength(prompt))
    expect(report?.stdinSha256).toBe(sha(prompt))
    expect(report?.argv).toEqual(['--print'])
    expect(report?.argv.join(' ')).not.toContain(marker)
    expect(report?.psArgs).toContain('--print')
    expect(report?.psArgs).not.toContain(marker)
    expect(report?.psArgs.length).toBeLessThan(1024)
  })

  it('does not let a prompt that starts with "-" be read as a flag', async () => {
    const prompt = '--dangerously-skip-permissions --output-format json do the thing'
    const { report } = await run(prompt)
    expect(report?.argv).toEqual(['--print'])
    expect(report?.stdinText).toBe(prompt)
  })

  it('keeps multi-byte UTF-8 intact even when a chunk boundary falls inside a character', async () => {
    // 4-byte emoji and 3-byte CJK repeated past several 64 KB chunks, with an odd prefix to shift alignment.
    const prompt = `a${'日本語 🚀 Việt Nam ❤️ '.repeat(20_000)}`
    const { report } = await run(prompt)
    expect(report?.stdinBytes).toBe(Buffer.byteLength(prompt))
    expect(report?.stdinSha256).toBe(sha(prompt))
  })

  it('keeps control characters, NUL, CRLF and ANSI escapes byte for byte', async () => {
    const prompt = 'a\u0000b\u0001c\u001b[31mred\u001b[0m\r\nline2\ttab\u007f end'
    const { report } = await run(prompt)
    expect(report?.stdinSha256).toBe(sha(prompt))
    expect(report?.stdinBytes).toBe(Buffer.byteLength(prompt))
  })

  it('survives the child exiting before it reads stdin (EPIPE) and reports its exit code', async () => {
    const unhandled: unknown[] = []
    const onError = (e: unknown): number => unhandled.push(e)
    process.on('uncaughtException', onError)
    process.on('unhandledRejection', onError)
    try {
      const { exitCode, stdout } = await run('y'.repeat(4 * 1024 * 1024), {
        env: { FAKE_CLAUDE_MODE: 'exit-early' }
      })
      expect(exitCode).toBe(7)
      expect(stdout).toBe('')
      await new Promise((r) => setTimeout(r, 50))
      expect(unhandled).toEqual([])
    } finally {
      process.off('uncaughtException', onError)
      process.off('unhandledRejection', onError)
    }
  })

  it('does not log the prompt, nor the rollback line, on the default path', async () => {
    await run('SECRET-PROMPT-TEXT')
    const lines = vi.mocked(log.info).mock.calls.map((c) => String(c[0]))
    expect(lines.join('\n')).not.toContain('SECRET-PROMPT-TEXT')
    expect(lines.join('\n')).not.toContain('ORCA_AGENT_PROMPT_VIA_ARGV')
  })

  it('ORCA_AGENT_PROMPT_VIA_ARGV=1 restores the argv path, logs one line and never the prompt', async () => {
    process.env['ORCA_AGENT_PROMPT_VIA_ARGV'] = '1'
    const { report, exitCode } = await run('SECRET-PROMPT-TEXT', { initFile: 'CTX' })
    expect(exitCode).toBe(0)
    expect(report?.argv).toEqual(['--print', 'CTX\nSECRET-PROMPT-TEXT'])
    expect(report?.stdinBytes).toBe(0)
    const lines = vi.mocked(log.info).mock.calls.map((c) => String(c[0]))
    const switchLines = lines.filter((l) => l.includes('ORCA_AGENT_PROMPT_VIA_ARGV'))
    expect(switchLines).toHaveLength(1)
    expect(lines.join('\n')).not.toContain('SECRET-PROMPT-TEXT')
  })

  it('turning the switch off again returns to stdin', async () => {
    process.env['ORCA_AGENT_PROMPT_VIA_ARGV'] = '0'
    const { report } = await run('fix the bug')
    expect(report?.argv).toEqual(['--print'])
    expect(report?.stdinText).toBe('fix the bug')
  })

  it('ignores the switch when it only appears in the request-supplied env', async () => {
    const { report } = await run('fix the bug', { env: { ORCA_AGENT_PROMPT_VIA_ARGV: '1' } })
    expect(report?.argv).toEqual(['--print'])
    expect(report?.stdinText).toBe('fix the bug')
  })
})

describe('promptViaArgvEnabled', () => {
  it.each([
    ['1', true],
    ['true', true],
    [' TRUE ', true],
    ['0', false],
    ['', false],
    ['no', false],
    [undefined, false]
  ])('%j -> %s', (value, expected) => {
    expect(promptViaArgvEnabled({ ORCA_AGENT_PROMPT_VIA_ARGV: value })).toBe(expected)
  })
})

describe('writePromptToStdin', () => {
  it('waits for drain and delivers every byte in order, then ends the stream', async () => {
    const received: Buffer[] = []
    let writes = 0
    const slow = new Writable({
      highWaterMark: 16,
      write(chunk: Buffer, _enc, cb) {
        writes += 1
        received.push(chunk)
        setImmediate(cb)
      }
    })
    const prompt = 'é日🚀'.repeat(60_000)
    const finished = new Promise<void>((resolve) => slow.on('finish', resolve))
    writePromptToStdin(slow, prompt)
    await finished
    expect(Buffer.concat(received).toString('utf8')).toBe(prompt)
    expect(writes).toBeGreaterThan(1)
  })

  it('does not throw or emit an unhandled error when the stream is already destroyed', async () => {
    const dead = new Writable({ write: (_c, _e, cb) => cb() })
    dead.destroy(new Error('EPIPE'))
    expect(() => writePromptToStdin(dead, 'hello')).not.toThrow()
    await new Promise((r) => setImmediate(r))
  })

  it('swallows a write error raised mid-stream', async () => {
    const failing = new Writable({
      highWaterMark: 1,
      write(_c, _e, cb) {
        cb(Object.assign(new Error('write EPIPE'), { code: 'EPIPE' }))
      }
    })
    expect(() => writePromptToStdin(failing, 'z'.repeat(200_000))).not.toThrow()
    await new Promise((r) => setTimeout(r, 20))
  })
})
