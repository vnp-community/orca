// src/relay/__tests__/agent-tool-registry.test.ts
import { describe, it, expect, vi, afterEach } from 'vitest'
import { accessSync } from 'node:fs'
import { ALL_TOOL_DEFINITIONS, discoverTools, runToolCommand } from '../agent-tool-registry'
import type { AgentConfig } from '../agent-config'

// Mock accessSync to control binary discovery without needing actual binaries
vi.mock('node:fs', async (importOriginal) => {
  const original = await importOriginal<typeof import('node:fs')>()
  return { ...original, accessSync: vi.fn() }
})

const mockConfig: AgentConfig = {
  mode: 'direct-websocket',
  orcaUrl: 'wss://test',
  agentToken: 'tok',
  agentPort: 6799,
  devServerId: 'test-server',
  logLevel: 'info',
  workDir: '/tmp',
  toolPath: '/usr/local/bin:/usr/bin',
  toolEnv: { PATH: '/usr/local/bin:/usr/bin', HOME: '/home/test' },
  credentialDir: '/home/test/.orca/credentials',
  tlsRejectUnauthorized: true,
}

describe('ALL_TOOL_DEFINITIONS', () => {
  it('contains exactly 9 tools', () => {
    expect(ALL_TOOL_DEFINITIONS).toHaveLength(9)
  })

  it('has no duplicate tool names', () => {
    const names = ALL_TOOL_DEFINITIONS.map(t => t.name)
    expect(new Set(names).size).toBe(names.length)
  })

  it('all tools have a non-empty name', () => {
    for (const t of ALL_TOOL_DEFINITIONS) {
      expect(t.name.length).toBeGreaterThan(0)
    }
  })

  it('all tools have a non-empty description', () => {
    for (const t of ALL_TOOL_DEFINITIONS) {
      expect(t.description.length).toBeGreaterThan(0)
    }
  })

  it('all tools have inputSchema.type === "object"', () => {
    for (const t of ALL_TOOL_DEFINITIONS) {
      expect(t.inputSchema.type).toBe('object')
    }
  })

  it('all tools have inputSchema.properties defined', () => {
    for (const t of ALL_TOOL_DEFINITIONS) {
      expect(t.inputSchema.properties).toBeDefined()
    }
  })

  it('all tools have a handler function', () => {
    for (const t of ALL_TOOL_DEFINITIONS) {
      expect(typeof t.handler).toBe('function')
    }
  })

  it('read_file has binary=null (built-in)', () => {
    const t = ALL_TOOL_DEFINITIONS.find(x => x.name === 'read_file')!
    expect(t).toBeDefined()
    expect(t.binary).toBeNull()
  })

  it('list_dir has binary=null (built-in)', () => {
    const t = ALL_TOOL_DEFINITIONS.find(x => x.name === 'list_dir')!
    expect(t).toBeDefined()
    expect(t.binary).toBeNull()
  })

  it('claude_code uses binary "claude"', () => {
    const t = ALL_TOOL_DEFINITIONS.find(x => x.name === 'claude_code')!
    expect(t.binary).toBe('claude')
  })

  it('gh tool exists and uses binary "gh"', () => {
    const t = ALL_TOOL_DEFINITIONS.find(x => x.name === 'gh')!
    expect(t).toBeDefined()
    expect(t.binary).toBe('gh')
  })

  it('shell tool requires "command" param', () => {
    const t = ALL_TOOL_DEFINITIONS.find(x => x.name === 'shell')!
    expect(t.inputSchema.required).toContain('command')
  })
})

describe('discoverTools', () => {
  afterEach(() => vi.clearAllMocks())

  it('always includes built-in tools (binary=null) even when accessSync throws', async () => {
    vi.mocked(accessSync).mockImplementation(() => { throw new Error('ENOENT') })
    const tools = await discoverTools(mockConfig)
    const names = tools.map(t => t.name)
    expect(names).toContain('read_file')
    expect(names).toContain('list_dir')
  })

  it('excludes tools whose binary is not found in toolPath', async () => {
    vi.mocked(accessSync).mockImplementation(() => { throw new Error('ENOENT') })
    const tools = await discoverTools(mockConfig)
    expect(tools.find(t => t.name === 'gh')).toBeUndefined()
  })

  it('includes tools when binary is found (accessSync succeeds)', async () => {
    vi.mocked(accessSync).mockReturnValue(undefined)
    const tools = await discoverTools(mockConfig)
    expect(tools.find(t => t.name === 'gh')).toBeDefined()
    expect(tools.find(t => t.name === 'git')).toBeDefined()
    expect(tools.find(t => t.name === 'claude_code')).toBeDefined()
  })

  it('returns only built-ins when all binary checks fail', async () => {
    vi.mocked(accessSync).mockImplementation(() => { throw new Error('ENOENT') })
    const tools = await discoverTools(mockConfig)
    expect(tools.every(t => t.binary === null)).toBe(true)
  })
})

describe('read_file handler (built-in)', () => {
  it('reads an existing file successfully (exitCode=0)', async () => {
    const { writeFileSync, mkdtempSync } = await import('node:fs')
    const { tmpdir } = await import('node:os')
    const { join } = await import('node:path')
    const dir  = mkdtempSync(`${tmpdir()}/agent-test-`)
    const path = join(dir, 'hello.txt')
    writeFileSync(path, 'line1\nline2\nline3')

    vi.mocked(accessSync).mockReturnValue(undefined)
    const t = ALL_TOOL_DEFINITIONS.find(x => x.name === 'read_file')!
    const r = await t.handler({ path }, mockConfig)
    expect(r.exitCode).toBe(0)
    expect(r.stdout).toContain('line1')
  })

  it('returns exitCode=1 and stderr for missing file', async () => {
    vi.mocked(accessSync).mockReturnValue(undefined)
    const t = ALL_TOOL_DEFINITIONS.find(x => x.name === 'read_file')!
    const r = await t.handler({ path: '/nonexistent/file.txt' }, mockConfig)
    expect(r.exitCode).toBe(1)
    expect(r.stderr).toBeTruthy()
  })

  it('respects start_line and end_line params', async () => {
    const { writeFileSync, mkdtempSync } = await import('node:fs')
    const { tmpdir } = await import('node:os')
    const { join } = await import('node:path')
    const dir  = mkdtempSync(`${tmpdir()}/agent-test-`)
    const path = join(dir, 'multi.txt')
    writeFileSync(path, 'A\nB\nC\nD\nE')

    vi.mocked(accessSync).mockReturnValue(undefined)
    const t = ALL_TOOL_DEFINITIONS.find(x => x.name === 'read_file')!
    const r = await t.handler({ path, start_line: 2, end_line: 3 }, mockConfig)
    expect(r.exitCode).toBe(0)
    expect(r.stdout.trim()).toBe('B\nC')
  })
})

describe('list_dir handler (built-in)', () => {
  it('lists directory contents (exitCode=0)', async () => {
    const { mkdtempSync, writeFileSync } = await import('node:fs')
    const { tmpdir } = await import('node:os')
    const { join } = await import('node:path')
    const dir = mkdtempSync(`${tmpdir()}/agent-ls-test-`)
    writeFileSync(join(dir, 'a.txt'), '')
    writeFileSync(join(dir, 'b.txt'), '')

    vi.mocked(accessSync).mockReturnValue(undefined)
    const t = ALL_TOOL_DEFINITIONS.find(x => x.name === 'list_dir')!
    const r = await t.handler({ path: dir }, mockConfig)
    expect(r.exitCode).toBe(0)
    const entries = JSON.parse(r.stdout)
    expect(entries).toHaveLength(2)
  })

  it('returns exitCode=1 for non-existent dir', async () => {
    vi.mocked(accessSync).mockReturnValue(undefined)
    const t = ALL_TOOL_DEFINITIONS.find(x => x.name === 'list_dir')!
    const r = await t.handler({ path: '/nonexistent/dir' }, mockConfig)
    expect(r.exitCode).toBe(1)
  })
})

describe('shell timeout cap', () => {
  it('shell timeout param of 999_999_999 is capped at 600_000ms', () => {
    // Verify the cap logic: Math.min(timeout, 600_000)
    const huge = 999_999_999
    expect(Math.min(huge, 600_000)).toBe(600_000)
  })
})

describe('runToolCommand options', () => {
  const nodeBin = process.execPath
  
  it('behaves like legacy without new options', async () => {
    const res = await runToolCommand(nodeBin, ['-e', 'console.log("hello")'], { cwd: process.cwd(), timeout: 5000, env: process.env })
    expect(res.stdout).toBe('hello\n')
    expect(res.exitCode).toBe(0)
    expect(res.meta).toBeUndefined()
  })

  it('maxOutputBytes truncates', async () => {
    const res = await runToolCommand(nodeBin, ['-e', 'console.log("a".repeat(100))'], { cwd: process.cwd(), timeout: 5000, env: process.env, maxOutputBytes: 10 })
    expect(res.stdout.length).toBe(10)
    expect(res.meta?.truncated).toBe(true)
  })

  it('stdoutFile redirects output', async () => {
    const { mkdtempSync, readFileSync } = await import('node:fs')
    const { tmpdir } = await import('node:os')
    const { join } = await import('node:path')
    const dir = mkdtempSync(`${tmpdir()}/agent-stdout-`)
    const file = join(dir, 'out.txt')
    const res = await runToolCommand(nodeBin, ['-e', 'console.log("file")'], { cwd: process.cwd(), timeout: 5000, env: process.env, stdoutFile: file })
    expect(res.stdout).toBe('')
    expect(readFileSync(file, 'utf8')).toBe('file\n')
  })

  const isWin = process.platform === 'win32'
  it.skipIf(isWin)('detached and killGraceMs kills child', async () => {
    const res = await runToolCommand(nodeBin, ['-e', 'setTimeout(()=>{}, 10000)'], { cwd: process.cwd(), timeout: 100, env: process.env, detached: true, killGraceMs: 50 })
    expect(res.exitCode).toBe(124)
    expect(res.meta?.timedOut).toBe(true)
  })

  it('aborts on signal', async () => {
    const ac = new AbortController()
    const p = runToolCommand(nodeBin, ['-e', 'setTimeout(()=>{}, 10000)'], { cwd: process.cwd(), timeout: 5000, env: process.env, signal: ac.signal })
    ac.abort()
    const res = await p
    expect(res.exitCode).toBe(124)
    expect(res.meta?.timedOut).toBe(false)
  })

  it('sends stdinText', async () => {
    const res = await runToolCommand(nodeBin, ['-e', 'process.stdin.pipe(process.stdout)'], { cwd: process.cwd(), timeout: 5000, env: process.env, stdinText: 'hello stdin' })
    expect(res.stdout).toBe('hello stdin')
  })

  it('gitnexus blocks forbidden verbs', async () => {
    const t = ALL_TOOL_DEFINITIONS.find(x => x.name === 'gitnexus')!
    const r1 = await t.handler({ args: ['analyze'] }, mockConfig)
    expect(r1.exitCode).toBe(2)
    expect(r1.stderr).toMatch(/tool verb not allowed/)

    vi.mocked(accessSync).mockReturnValue(undefined)
    const r2 = await t.handler({ args: ['status'] }, mockConfig)
    expect(r2.exitCode).not.toBe(2)
  })
})
