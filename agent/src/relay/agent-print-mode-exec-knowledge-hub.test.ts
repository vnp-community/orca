import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { EventEmitter } from 'node:events'
import { spawn } from 'node:child_process'
import type * as ChildProcess from 'node:child_process'
import type * as ReadonlyToolPolicy from './agent-readonly-tool-policy'
import { existsSync } from 'node:fs'
import { mkdir, mkdtemp, readFile, readdir, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { handleAgentExecPrompt } from './agent-print-mode-exec'

vi.mock('./agent-credential-store', () => ({ readDecryptedKey: vi.fn(async () => null) }))
const probed = vi.hoisted(() => ({ value: null as Record<string, boolean> | null }))
vi.mock('./agent-readonly-tool-policy', async (importOriginal) => {
  const actual = await importOriginal<typeof ReadonlyToolPolicy>()
  return { ...actual, detectClaudeFlags: vi.fn(async () => probed.value) }
})
vi.mock('node:child_process', async (importOriginal) => {
  const actual = await importOriginal<typeof ChildProcess>()
  return { ...actual, spawn: vi.fn() }
})
const spawnMock = vi.mocked(spawn)
const ALL_FLAGS = {
  tools: true,
  permissionMode: true,
  disallowedTools: true,
  mcpConfig: true,
  allowedTools: true,
  streamJson: false
}

type FakeChild = EventEmitter & {
  stdout: EventEmitter
  stderr: EventEmitter
  kill: ReturnType<typeof vi.fn>
}
function fakeChild(): FakeChild {
  return Object.assign(new EventEmitter(), {
    stdout: new EventEmitter(),
    stderr: new EventEmitter(),
    kill: vi.fn()
  })
}
async function waitForSpawn(): Promise<void> {
  for (let i = 0; i < 100 && spawnMock.mock.calls.length === 0; i++) {
    await new Promise((r) => setImmediate(r))
  }
}

const LOG: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }
let tmp: string
let config: AgentConfig
let root: string
let scratchDir: string

beforeEach(async () => {
  spawnMock.mockReset()
  probed.value = ALL_FLAGS
  tmp = await mkdtemp(path.join(tmpdir(), 'orca-exec-kh-'))
  root = path.join(tmp, 'repo')
  scratchDir = path.join(tmp, 'scratch')
  await mkdir(scratchDir, { recursive: true })
  await mkdir(path.join(root, '.tdd-agent'), { recursive: true })
  await writeFile(path.join(root, '.tdd-agent', 'config.toml'), '')
  config = {
    mode: 'direct-websocket',
    orcaUrl: '',
    orcaHttpUrl: '',
    apiSecret: '',
    agentToken: '',
    agentPort: 6799,
    devServerId: 'test-server',
    logLevel: 'info',
    workDir: path.join(tmp, 'work'),
    toolPath: '/usr/bin:/bin',
    toolEnv: { PATH: '/usr/bin:/bin' },
    credentialDir: tmp,
    tlsRejectUnauthorized: true
  }
})
afterEach(async () => {
  await rm(tmp, { recursive: true, force: true })
})

async function run(params: Record<string, unknown>, onSpawn?: (c: FakeChild) => Promise<void>) {
  const child = fakeChild()
  spawnMock.mockReturnValue(child as never)
  const pending = handleAgentExecPrompt(
    1,
    {
      prompt: 'p',
      worktreePath: scratchDir,
      workspaceKind: 'scratch',
      accessMode: 'readonly',
      ...params
    },
    config,
    LOG
  )
  await waitForSpawn()
  if (spawnMock.mock.calls.length > 0) {
    await onSpawn?.(child)
    child.emit('close', 0)
  }
  return (await pending) as {
    result?: Record<string, unknown>
    error?: { data?: { reason?: string } }
  }
}

describe('handleAgentExecPrompt with a knowledge root', () => {
  it('adds the MCP flags after the read-only flags and removes the config after the run', async () => {
    let configPath = ''
    const out = await run({ knowledgeRoot: root }, async () => {
      const args = spawnMock.mock.calls[0]?.[1] as string[]
      configPath = args[args.indexOf('--mcp-config') + 1]
      expect(existsSync(configPath)).toBe(true)
      expect(JSON.parse(await readFile(configPath, 'utf8')).mcpServers['tdd-agent'].command).toBe(
        'tdd-agent'
      )
    })
    const args = spawnMock.mock.calls[0]?.[1] as string[]
    expect(args.slice(0, 2)).toEqual(['--print', 'p'])
    expect(args.indexOf('--tools')).toBeLessThan(args.indexOf('--mcp-config'))
    expect(args).toContain('--strict-mcp-config')
    expect(args[args.indexOf('--allowedTools') + 1]).not.toContain('changeset_propose')
    expect(args[args.indexOf('--disallowedTools') + 1]).toContain(
      'mcp__tdd-agent__changeset_propose'
    )
    expect(out.result).toBeDefined()
    expect(existsSync(configPath)).toBe(false)
  })

  it('allows changeset_propose only when proposeChangesets is set', async () => {
    await run({ knowledgeRoot: root, proposeChangesets: true })
    const args = spawnMock.mock.calls[0]?.[1] as string[]
    expect(args[args.indexOf('--allowedTools') + 1]).toContain('mcp__tdd-agent__changeset_propose')
    expect(args[args.indexOf('--disallowedTools') + 1]).not.toContain('changeset_propose')
  })

  it('sends no MCP flag when the caller sent no knowledgeRoot', async () => {
    await run({})
    const args = spawnMock.mock.calls[0]?.[1] as string[]
    expect(args).not.toContain('--mcp-config')
    expect(args).not.toContain('--allowedTools')
  })

  it('removes the config also when the process times out', async () => {
    const child = fakeChild()
    spawnMock.mockReturnValue(child as never)
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      const pending = handleAgentExecPrompt(
        1,
        {
          prompt: 'p',
          worktreePath: scratchDir,
          workspaceKind: 'scratch',
          accessMode: 'readonly',
          knowledgeRoot: root,
          timeoutMs: 1000
        },
        config,
        LOG
      )
      for (let i = 0; i < 100 && spawnMock.mock.calls.length === 0; i++) {
        await vi.advanceTimersByTimeAsync(1)
      }
      await vi.advanceTimersByTimeAsync(130_000)
      const out = (await pending) as { result: { timedOut: boolean } }
      expect(out.result.timedOut).toBe(true)
    } finally {
      vi.useRealTimers()
    }
    expect(await readdir(path.join(config.workDir, 'tmp'))).toEqual([])
  })

  it('removes the config when spawn throws', async () => {
    spawnMock.mockImplementation(() => {
      throw new Error('spawn blew up')
    })
    await expect(
      handleAgentExecPrompt(
        1,
        {
          prompt: 'p',
          worktreePath: scratchDir,
          workspaceKind: 'scratch',
          accessMode: 'readonly',
          knowledgeRoot: root
        },
        config,
        LOG
      )
    ).rejects.toThrow('spawn blew up')
    expect(await readdir(path.join(config.workDir, 'tmp'))).toEqual([])
  })

  it('refuses a root without .tdd-agent and never spawns', async () => {
    const out = await run({ knowledgeRoot: tmp })
    expect(out.error?.data?.reason).toBe('KNOWLEDGE_CONFIG_MISSING')
    expect(spawnMock).not.toHaveBeenCalled()
  })

  it('fails closed when claude has no --mcp-config', async () => {
    probed.value = { ...ALL_FLAGS, mcpConfig: false }
    const out = await run({ knowledgeRoot: root })
    expect(out.error?.data?.reason).toBe('KNOWLEDGE_HUB_UNSUPPORTED')
    expect(spawnMock).not.toHaveBeenCalled()
  })
})
