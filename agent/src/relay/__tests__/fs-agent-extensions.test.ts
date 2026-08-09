// src/relay/__tests__/fs-agent-extensions.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mkdtempSync, writeFileSync, mkdirSync, rmSync, existsSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import {
  handleFsReadDir,
  handleFsReadFile,
  handlePreflightCheck,
  handleFsStat,
  handleFsGlob,
  handleFsWriteFile,
} from '../fs-agent-extensions'
import type { AgentConfig } from '../agent-config'
import { registerTraceSink } from '../../shared/trace'
import type { TraceEvent } from '../../shared/trace'

// Mock the two imported helpers to keep tests unit-level
vi.mock('../fs-handler-utils', () => ({
  checkRgAvailable: vi.fn().mockResolvedValue(false),
}))

vi.mock('../fs-handler-file-read', () => ({
  readRelayFileContent: vi.fn().mockResolvedValue({
    content: 'mocked file content',
    isBinary: false,
  }),
}))

let tmpDir: string

function makeConfig(): AgentConfig {
  return { workDir: tmpDir, toolEnv: { PATH: '/usr/bin' } } as unknown as AgentConfig
}

beforeEach(() => { tmpDir = mkdtempSync(join(tmpdir(), 'fs-ext-test-')) })
afterEach(() => rmSync(tmpDir, { recursive: true, force: true }))

// ─── handleFsReadDir ──────────────────────────────────────────────────────────
describe('handleFsReadDir', () => {
  it('returns entries array for a valid directory', async () => {
    writeFileSync(join(tmpDir, 'a.ts'), '')
    writeFileSync(join(tmpDir, 'b.ts'), '')
    const resp = await handleFsReadDir(1, { path: tmpDir }, makeConfig()) as any
    expect(resp.result.entries).toHaveLength(2)
  })

  it('directories appear before files in result (sorted)', async () => {
    mkdirSync(join(tmpDir, 'z-dir'))
    writeFileSync(join(tmpDir, 'a-file.txt'), '')
    const resp = await handleFsReadDir(1, { path: tmpDir }, makeConfig()) as any
    expect(resp.result.entries[0].type).toBe('directory')
    expect(resp.result.entries[1].type).toBe('file')
  })

  it('entries are alphabetically sorted within type groups', async () => {
    writeFileSync(join(tmpDir, 'z.txt'), '')
    writeFileSync(join(tmpDir, 'a.txt'), '')
    const resp = await handleFsReadDir(1, { path: tmpDir }, makeConfig()) as any
    const names = resp.result.entries.map((e: any) => e.name)
    expect(names).toEqual([...names].sort())
  })

  it('depth=2 includes grandchildren', async () => {
    mkdirSync(join(tmpDir, 'sub'))
    writeFileSync(join(tmpDir, 'sub', 'child.ts'), '')
    const resp = await handleFsReadDir(1, { path: tmpDir, depth: 2 }, makeConfig()) as any
    const sub = resp.result.entries.find((e: any) => e.name === 'sub')
    expect(sub!.children).toHaveLength(1)
    expect(sub!.children[0].name).toBe('child.ts')
  })

  it('depth=1 does not include grandchildren', async () => {
    mkdirSync(join(tmpDir, 'sub'))
    writeFileSync(join(tmpDir, 'sub', 'child.ts'), '')
    const resp = await handleFsReadDir(1, { path: tmpDir, depth: 1 }, makeConfig()) as any
    const sub = resp.result.entries.find((e: any) => e.name === 'sub')
    expect(sub!.children).toBeUndefined()
  })

  it('caps depth at 5 even if larger value given', async () => {
    // Just verify no infinite recursion / timeout — check it returns without error
    const resp = await handleFsReadDir(1, { path: tmpDir, depth: 999 }, makeConfig()) as any
    expect(resp.result ?? resp.error).toBeDefined()
  })

  it('returns InvalidParams (-32602) for a file path (not a directory)', async () => {
    const filePath = join(tmpDir, 'file.txt')
    writeFileSync(filePath, 'data')
    const resp = await handleFsReadDir(1, { path: filePath }, makeConfig()) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('returns error for missing path param', async () => {
    const resp = await handleFsReadDir(1, {}, makeConfig()) as any
    expect(resp.error).toBeDefined()
  })

  it('returns error for non-existent path', async () => {
    const resp = await handleFsReadDir(1, { path: '/nonexistent/xyz' }, makeConfig()) as any
    expect(resp.error).toBeDefined()
  })

  it('result contains the absolute path', async () => {
    const resp = await handleFsReadDir(1, { path: tmpDir }, makeConfig()) as any
    expect(resp.result.path).toBe(tmpDir)
  })

  it('each file entry has name and type fields', async () => {
    writeFileSync(join(tmpDir, 'test.ts'), '')
    const resp = await handleFsReadDir(1, { path: tmpDir }, makeConfig()) as any
    const entry = resp.result.entries[0]
    expect(entry.name).toBeDefined()
    expect(entry.type).toMatch(/^(file|directory)$/)
  })
})

// ─── handleFsReadFile ─────────────────────────────────────────────────────────
describe('handleFsReadFile', () => {
  it('returns mocked file content', async () => {
    const resp = await handleFsReadFile(1, { path: '/any/file.ts' }, makeConfig()) as any
    expect(resp.result.content).toBe('mocked file content')
  })

  it('returns encoding "utf-8" for non-binary files', async () => {
    const resp = await handleFsReadFile(1, { path: '/any/file.ts' }, makeConfig()) as any
    expect(resp.result.encoding).toBe('utf-8')
  })

  it('returns the absolute path in result', async () => {
    const resp = await handleFsReadFile(1, { path: '/abs/file.ts' }, makeConfig()) as any
    expect(resp.result.path).toBe('/abs/file.ts')
  })

  it('returns InvalidParams (-32602) for missing path param', async () => {
    const resp = await handleFsReadFile(1, {}, makeConfig()) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('returns "base64" encoding when isBinary=true', async () => {
    const { readRelayFileContent } = await import('../fs-handler-file-read')
    vi.mocked(readRelayFileContent).mockResolvedValueOnce({ content: 'abc=', isBinary: true } as any)
    const resp = await handleFsReadFile(1, { path: '/bin/file.bin' }, makeConfig()) as any
    expect(resp.result.encoding).toBe('base64')
    expect(resp.result.isBinary).toBe(true)
  })
})

// ─── handlePreflightCheck ─────────────────────────────────────────────────────
describe('handlePreflightCheck', () => {
  it('returns a result object with boolean for each requested service', async () => {
    const resp = await handlePreflightCheck(1, { services: ['github-cli', 'docker'] }, makeConfig()) as any
    expect('github-cli' in resp.result).toBe(true)
    expect('docker' in resp.result).toBe(true)
    expect(typeof resp.result['github-cli']).toBe('boolean')
  })

  it('returns false for unknown service names', async () => {
    const resp = await handlePreflightCheck(1, { services: ['unknown-tool-xyz-999'] }, makeConfig()) as any
    expect(resp.result['unknown-tool-xyz-999']).toBe(false)
  })

  it('returns empty object for empty services array', async () => {
    const resp = await handlePreflightCheck(1, { services: [] }, makeConfig()) as any
    expect(Object.keys(resp.result)).toHaveLength(0)
  })

  it('handles multiple services in parallel without crash', async () => {
    const resp = await handlePreflightCheck(1,
      { services: ['github-cli', 'ripgrep', 'docker', 'claude'] },
      makeConfig()
    ) as any
    expect(Object.keys(resp.result)).toHaveLength(4)
  })
})

// ─── handlePreflightCheck — agent:preflight tracing (TASK-AG-014.2) ─────────
describe('handlePreflightCheck — agent:preflight tracing', () => {
  it('span.ok({checkedCount}) khi tất cả services khả dụng (empty list = vacuously all-ok)', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    await handlePreflightCheck(1, { services: [] }, makeConfig())
    unregister()

    const ok = events.find(e => e.flow === 'agent:preflight' && e.level === 'ok')
    expect(ok).toBeDefined()
    expect(ok?.fields.checkedCount).toBe(0)
  })

  it('span.fail("unavailable: ...") khi có service không cài đặt', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    await handlePreflightCheck(1, { services: ['not-a-real-binary-xyz'] }, makeConfig())
    unregister()

    const fail = events.find(e => e.flow === 'agent:preflight' && e.level === 'fail')
    expect(fail).toBeDefined()
    expect(fail?.fields.failedCount).toBe(1)
  })

  it('phân biệt agent:preflight với agent:fs (khác flow name)', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    await handlePreflightCheck(1, { services: ['ripgrep'] }, makeConfig())
    unregister()

    expect(events.every(e => e.flow !== 'agent:fs')).toBe(true)
    expect(events.some(e => e.flow === 'agent:preflight')).toBe(true)
  })
})

// ─── handleFsStat ─────────────────────────────────────────────────────────────
describe('handleFsStat', () => {
  it('returns stat info for an existing file', async () => {
    const filePath = join(tmpDir, 'stat-test.ts')
    writeFileSync(filePath, 'hello world')
    const resp = await handleFsStat(1, { path: filePath }, makeConfig()) as any
    expect(resp.result.isFile).toBe(true)
    expect(resp.result.isDir).toBe(false)
    expect(resp.result.size).toBe(11)
    expect(resp.result.path).toBe(filePath)
  })

  it('returns stat info for an existing directory', async () => {
    const resp = await handleFsStat(1, { path: tmpDir }, makeConfig()) as any
    expect(resp.result.isDir).toBe(true)
    expect(resp.result.isFile).toBe(false)
  })

  it('returns mtime as ISO string', async () => {
    const filePath = join(tmpDir, 'mtime-test.txt')
    writeFileSync(filePath, 'x')
    const resp = await handleFsStat(1, { path: filePath }, makeConfig()) as any
    expect(resp.result.mtime).toMatch(/^\d{4}-\d{2}-\d{2}T/)
  })

  it('returns mode as octal string', async () => {
    const filePath = join(tmpDir, 'mode-test.txt')
    writeFileSync(filePath, 'x')
    const resp = await handleFsStat(1, { path: filePath }, makeConfig()) as any
    expect(typeof resp.result.mode).toBe('string')
    expect(resp.result.mode.length).toBeGreaterThan(0)
  })

  it('returns error for non-existent path', async () => {
    const resp = await handleFsStat(1, { path: '/nonexistent/path/xyz-abc' }, makeConfig()) as any
    expect(resp.error).toBeDefined()
    expect(resp.error.code).toBeDefined()
  })

  it('returns InvalidParams for missing path param', async () => {
    const resp = await handleFsStat(1, {}, makeConfig()) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('resolves relative path against workDir', async () => {
    writeFileSync(join(tmpDir, 'relative.ts'), 'content')
    const resp = await handleFsStat(1, { path: 'relative.ts' }, makeConfig()) as any
    expect(resp.result.isFile).toBe(true)
  })
})

// ─── handleFsGlob ─────────────────────────────────────────────────────────────
describe('handleFsGlob', () => {
  it('returns paths matching *.ts in workDir', async () => {
    writeFileSync(join(tmpDir, 'index.ts'), '')
    writeFileSync(join(tmpDir, 'utils.ts'), '')
    writeFileSync(join(tmpDir, 'readme.md'), '')
    const resp = await handleFsGlob(1, { pattern: '*.ts', cwd: tmpDir }, makeConfig()) as any
    expect(resp.result.paths.length).toBe(2)
    expect(resp.result.paths.every((p: string) => p.endsWith('.ts'))).toBe(true)
  })

  it('returns empty array when no files match', async () => {
    const resp = await handleFsGlob(1, { pattern: '*.xyz-never', cwd: tmpDir }, makeConfig()) as any
    expect(resp.result.paths).toHaveLength(0)
    expect(resp.result.total).toBe(0)
  })

  it('returns InvalidParams for missing pattern param', async () => {
    const resp = await handleFsGlob(1, { cwd: tmpDir }, makeConfig()) as any
    expect(resp.error.code).toBe(-32602)
    expect(resp.error.message).toContain('pattern')
  })

  it('result includes cwd and total fields', async () => {
    const resp = await handleFsGlob(1, { pattern: '*.ts', cwd: tmpDir }, makeConfig()) as any
    expect(resp.result.cwd).toBe(tmpDir)
    expect(typeof resp.result.total).toBe('number')
  })

  it('excludes node_modules by default', async () => {
    mkdirSync(join(tmpDir, 'node_modules', 'pkg'), { recursive: true })
    writeFileSync(join(tmpDir, 'node_modules', 'pkg', 'index.ts'), '')
    writeFileSync(join(tmpDir, 'app.ts'), '')
    const resp = await handleFsGlob(1, { pattern: '*.ts', cwd: tmpDir }, makeConfig()) as any
    const paths: string[] = resp.result.paths
    expect(paths.every((p: string) => !p.includes('node_modules'))).toBe(true)
    expect(paths.some((p: string) => p.endsWith('app.ts'))).toBe(true)
  })

  it('returns paths relative to cwd', async () => {
    writeFileSync(join(tmpDir, 'app.ts'), '')
    const resp = await handleFsGlob(1, { pattern: '*.ts', cwd: tmpDir }, makeConfig()) as any
    const paths: string[] = resp.result.paths
    // Paths should be relative (not starting with /)
    expect(paths.every((p: string) => !p.startsWith('/'))).toBe(true)
  })
})

// ─── handleFsWriteFile ────────────────────────────────────────────────────────
describe('handleFsWriteFile', () => {
  it('writes file content and returns { ok: true }', async () => {
    const resp = await handleFsWriteFile(1,
      { path: join(tmpDir, 'new-file.ts'), content: 'export const x = 1' },
      makeConfig()
    ) as any
    expect(resp.result.ok).toBe(true)
    expect(existsSync(join(tmpDir, 'new-file.ts'))).toBe(true)
  })

  it('returns bytes count in result', async () => {
    const content = 'hello world'
    const resp = await handleFsWriteFile(1,
      { path: join(tmpDir, 'bytes-test.txt'), content },
      makeConfig()
    ) as any
    expect(resp.result.bytes).toBe(Buffer.byteLength(content, 'utf-8'))
  })

  it('returns absolute path in result', async () => {
    const filePath = join(tmpDir, 'abs-path.ts')
    const resp = await handleFsWriteFile(1,
      { path: filePath, content: 'x' },
      makeConfig()
    ) as any
    expect(resp.result.path).toBe(filePath)
  })

  it('creates parent directories automatically', async () => {
    const nestedPath = join(tmpDir, 'a', 'b', 'c', 'nested.ts')
    const resp = await handleFsWriteFile(1,
      { path: nestedPath, content: 'export {}' },
      makeConfig()
    ) as any
    expect(resp.result.ok).toBe(true)
    expect(existsSync(nestedPath)).toBe(true)
  })

  it('overwrites existing file', async () => {
    const filePath = join(tmpDir, 'overwrite.ts')
    writeFileSync(filePath, 'old content')
    await handleFsWriteFile(1, { path: filePath, content: 'new content' }, makeConfig())
    const { readFileSync } = await import('node:fs')
    expect(readFileSync(filePath, 'utf-8')).toBe('new content')
  })

  it('rejects path traversal outside workDir', async () => {
    const resp = await handleFsWriteFile(1,
      { path: join(tmpDir, '../../../etc/passwd'), content: 'evil' },
      makeConfig()
    ) as any
    expect(resp.error).toBeDefined()
    expect(resp.error.code).toBe(-32602)
    expect(resp.error.message).toContain('outside project root')
  })

  it('returns InvalidParams for missing path param', async () => {
    const resp = await handleFsWriteFile(1, { content: 'data' }, makeConfig()) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('returns error for content exceeding 10MB', async () => {
    const bigContent = 'x'.repeat(11 * 1024 * 1024) // 11MB
    const resp = await handleFsWriteFile(1,
      { path: join(tmpDir, 'big.txt'), content: bigContent },
      makeConfig()
    ) as any
    expect(resp.error).toBeDefined()
    expect(resp.error.message).toContain('10MB')
  })

  it('resolves relative path against workDir', async () => {
    const resp = await handleFsWriteFile(1,
      { path: 'relative-write.ts', content: 'export const x = 1' },
      makeConfig()
    ) as any
    expect(resp.result.ok).toBe(true)
    expect(existsSync(join(tmpDir, 'relative-write.ts'))).toBe(true)
  })
})
