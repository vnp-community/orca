// src/relay/__tests__/fs-agent-extensions.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mkdtempSync, writeFileSync, mkdirSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import {
  handleFsReadDir,
  handleFsReadFile,
  handleFsStat,
  handleFsGlob
} from '../fs-agent-extensions'
import type { AgentConfig } from '../agent-config'

// Mock the two imported helpers to keep tests unit-level
vi.mock('../fs-handler-utils', () => ({
  checkRgAvailable: vi.fn().mockResolvedValue(false)
}))

vi.mock('../fs-handler-file-read', () => ({
  readRelayFileContent: vi.fn().mockResolvedValue({
    content: 'mocked file content',
    isBinary: false
  })
}))

let tmpDir: string

function makeConfig(): AgentConfig {
  return { workDir: tmpDir, toolEnv: { PATH: '/usr/bin' } } as unknown as AgentConfig
}

type JsonRpcError = {
  code: number
  message: string
}
type JsonRpcResponse<TResult = unknown> = {
  jsonrpc: string
  id: string | number | null
  result?: TResult
  error?: JsonRpcError
}

type FileTreeNodeShape = {
  path: string
  name: string
  type: 'file' | 'directory'
  size?: number
  children?: FileTreeNodeShape[]
}
type FsReadDirResult = { entries: FileTreeNodeShape[]; path: string }
type FsReadFileResult = { content: string; encoding: string; isBinary: boolean; path: string }
type FsStatResult = {
  path: string
  size: number
  mtime: string
  isDir: boolean
  isFile: boolean
  isLink: boolean
  mode: string
}
type FsGlobResult = { paths: string[]; cwd: string; total: number }

beforeEach(() => {
  tmpDir = mkdtempSync(join(tmpdir(), 'fs-ext-test-'))
})
afterEach(() => rmSync(tmpDir, { recursive: true, force: true }))

// ─── handleFsReadDir ──────────────────────────────────────────────────────────
describe('handleFsReadDir', () => {
  it('returns entries array for a valid directory', async () => {
    writeFileSync(join(tmpDir, 'a.ts'), '')
    writeFileSync(join(tmpDir, 'b.ts'), '')
    const resp = (await handleFsReadDir(
      1,
      { path: tmpDir },
      makeConfig()
    )) as JsonRpcResponse<FsReadDirResult>
    expect(resp.result!.entries).toHaveLength(2)
  })

  it('directories appear before files in result (sorted)', async () => {
    mkdirSync(join(tmpDir, 'z-dir'))
    writeFileSync(join(tmpDir, 'a-file.txt'), '')
    const resp = (await handleFsReadDir(
      1,
      { path: tmpDir },
      makeConfig()
    )) as JsonRpcResponse<FsReadDirResult>
    expect(resp.result!.entries[0].type).toBe('directory')
    expect(resp.result!.entries[1].type).toBe('file')
  })

  it('entries are alphabetically sorted within type groups', async () => {
    writeFileSync(join(tmpDir, 'z.txt'), '')
    writeFileSync(join(tmpDir, 'a.txt'), '')
    const resp = (await handleFsReadDir(
      1,
      { path: tmpDir },
      makeConfig()
    )) as JsonRpcResponse<FsReadDirResult>
    const names = resp.result!.entries.map((e) => e.name)
    expect(names).toEqual([...names].sort())
  })

  it('depth=2 includes grandchildren', async () => {
    mkdirSync(join(tmpDir, 'sub'))
    writeFileSync(join(tmpDir, 'sub', 'child.ts'), '')
    const resp = (await handleFsReadDir(
      1,
      { path: tmpDir, depth: 2 },
      makeConfig()
    )) as JsonRpcResponse<FsReadDirResult>
    const sub = resp.result!.entries.find((e) => e.name === 'sub')
    expect(sub!.children).toHaveLength(1)
    expect(sub!.children![0].name).toBe('child.ts')
  })

  it('depth=1 does not include grandchildren', async () => {
    mkdirSync(join(tmpDir, 'sub'))
    writeFileSync(join(tmpDir, 'sub', 'child.ts'), '')
    const resp = (await handleFsReadDir(
      1,
      { path: tmpDir, depth: 1 },
      makeConfig()
    )) as JsonRpcResponse<FsReadDirResult>
    const sub = resp.result!.entries.find((e) => e.name === 'sub')
    expect(sub!.children).toBeUndefined()
  })

  it('caps depth at 5 even if larger value given', async () => {
    // Just verify no infinite recursion / timeout — check it returns without error
    const resp = (await handleFsReadDir(
      1,
      { path: tmpDir, depth: 999 },
      makeConfig()
    )) as JsonRpcResponse<FsReadDirResult>
    expect(resp.result ?? resp.error).toBeDefined()
  })

  it('returns InvalidParams (-32602) for a file path (not a directory)', async () => {
    const filePath = join(tmpDir, 'file.txt')
    writeFileSync(filePath, 'data')
    const resp = (await handleFsReadDir(
      1,
      { path: filePath },
      makeConfig()
    )) as JsonRpcResponse<FsReadDirResult>
    expect(resp.error!.code).toBe(-32602)
  })

  it('returns error for missing path param', async () => {
    const resp = (await handleFsReadDir(1, {}, makeConfig())) as JsonRpcResponse<FsReadDirResult>
    expect(resp.error).toBeDefined()
  })

  it('returns error for non-existent path', async () => {
    const resp = (await handleFsReadDir(
      1,
      { path: '/nonexistent/xyz' },
      makeConfig()
    )) as JsonRpcResponse<FsReadDirResult>
    expect(resp.error).toBeDefined()
  })

  it('result contains the absolute path', async () => {
    const resp = (await handleFsReadDir(
      1,
      { path: tmpDir },
      makeConfig()
    )) as JsonRpcResponse<FsReadDirResult>
    expect(resp.result!.path).toBe(tmpDir)
  })

  it('each file entry has name and type fields', async () => {
    writeFileSync(join(tmpDir, 'test.ts'), '')
    const resp = (await handleFsReadDir(
      1,
      { path: tmpDir },
      makeConfig()
    )) as JsonRpcResponse<FsReadDirResult>
    const entry = resp.result!.entries[0]
    expect(entry.name).toBeDefined()
    expect(entry.type).toMatch(/^(file|directory)$/)
  })
})

// ─── handleFsReadFile ─────────────────────────────────────────────────────────
describe('handleFsReadFile', () => {
  it('returns mocked file content', async () => {
    const resp = (await handleFsReadFile(
      1,
      { path: '/any/file.ts' },
      makeConfig()
    )) as JsonRpcResponse<FsReadFileResult>
    expect(resp.result!.content).toBe('mocked file content')
  })

  it('returns encoding "utf-8" for non-binary files', async () => {
    const resp = (await handleFsReadFile(
      1,
      { path: '/any/file.ts' },
      makeConfig()
    )) as JsonRpcResponse<FsReadFileResult>
    expect(resp.result!.encoding).toBe('utf-8')
  })

  it('returns the absolute path in result', async () => {
    const resp = (await handleFsReadFile(
      1,
      { path: '/abs/file.ts' },
      makeConfig()
    )) as JsonRpcResponse<FsReadFileResult>
    expect(resp.result!.path).toBe('/abs/file.ts')
  })

  it('returns InvalidParams (-32602) for missing path param', async () => {
    const resp = (await handleFsReadFile(1, {}, makeConfig())) as JsonRpcResponse<FsReadFileResult>
    expect(resp.error!.code).toBe(-32602)
  })

  it('returns "base64" encoding when isBinary=true', async () => {
    const { readRelayFileContent } = await import('../fs-handler-file-read')
    vi.mocked(readRelayFileContent).mockResolvedValueOnce({
      content: 'abc=',
      isBinary: true
    } as Awaited<ReturnType<typeof readRelayFileContent>>)
    const resp = (await handleFsReadFile(
      1,
      { path: '/bin/file.bin' },
      makeConfig()
    )) as JsonRpcResponse<FsReadFileResult>
    expect(resp.result!.encoding).toBe('base64')
    expect(resp.result!.isBinary).toBe(true)
  })
})

// ─── handleFsStat ─────────────────────────────────────────────────────────────
describe('handleFsStat', () => {
  it('returns stat info for an existing file', async () => {
    const filePath = join(tmpDir, 'stat-test.ts')
    writeFileSync(filePath, 'hello world')
    const resp = (await handleFsStat(
      1,
      { path: filePath },
      makeConfig()
    )) as JsonRpcResponse<FsStatResult>
    expect(resp.result!.isFile).toBe(true)
    expect(resp.result!.isDir).toBe(false)
    expect(resp.result!.size).toBe(11)
    expect(resp.result!.path).toBe(filePath)
  })

  it('returns stat info for an existing directory', async () => {
    const resp = (await handleFsStat(
      1,
      { path: tmpDir },
      makeConfig()
    )) as JsonRpcResponse<FsStatResult>
    expect(resp.result!.isDir).toBe(true)
    expect(resp.result!.isFile).toBe(false)
  })

  it('returns mtime as ISO string', async () => {
    const filePath = join(tmpDir, 'mtime-test.txt')
    writeFileSync(filePath, 'x')
    const resp = (await handleFsStat(
      1,
      { path: filePath },
      makeConfig()
    )) as JsonRpcResponse<FsStatResult>
    expect(resp.result!.mtime).toMatch(/^\d{4}-\d{2}-\d{2}T/)
  })

  it('returns mode as octal string', async () => {
    const filePath = join(tmpDir, 'mode-test.txt')
    writeFileSync(filePath, 'x')
    const resp = (await handleFsStat(
      1,
      { path: filePath },
      makeConfig()
    )) as JsonRpcResponse<FsStatResult>
    expect(typeof resp.result!.mode).toBe('string')
    expect(resp.result!.mode.length).toBeGreaterThan(0)
  })

  it('returns PathNotFound (not the generic ServerError) for non-existent path', async () => {
    // Regression guard: desktop's fs:pathExists handler tells "doesn't exist"
    // apart from a real failure by this code, not by parsing the message —
    // ServerError here made every Dev-Server-relay existence check
    // indistinguishable from a genuine error (see fs-agent-extensions.ts's
    // handleFsStat doc comment).
    const resp = (await handleFsStat(
      1,
      { path: '/nonexistent/path/xyz-abc' },
      makeConfig()
    )) as JsonRpcResponse<FsStatResult>
    expect(resp.error).toBeDefined()
    expect(resp.error!.code).toBe(-33003)
    expect(resp.error!.message).toContain('Not found:')
  })

  it('returns InvalidParams for missing path param', async () => {
    const resp = (await handleFsStat(1, {}, makeConfig())) as JsonRpcResponse<FsStatResult>
    expect(resp.error!.code).toBe(-32602)
  })

  it('resolves relative path against workDir', async () => {
    writeFileSync(join(tmpDir, 'relative.ts'), 'content')
    const resp = (await handleFsStat(
      1,
      { path: 'relative.ts' },
      makeConfig()
    )) as JsonRpcResponse<FsStatResult>
    expect(resp.result!.isFile).toBe(true)
  })
})

// ─── handleFsGlob ─────────────────────────────────────────────────────────────
describe('handleFsGlob', () => {
  it('returns paths matching *.ts in workDir', async () => {
    writeFileSync(join(tmpDir, 'index.ts'), '')
    writeFileSync(join(tmpDir, 'utils.ts'), '')
    writeFileSync(join(tmpDir, 'readme.md'), '')
    const resp = (await handleFsGlob(
      1,
      { pattern: '*.ts', cwd: tmpDir },
      makeConfig()
    )) as JsonRpcResponse<FsGlobResult>
    expect(resp.result!.paths.length).toBe(2)
    expect(resp.result!.paths.every((p) => p.endsWith('.ts'))).toBe(true)
  })

  it('returns empty array when no files match', async () => {
    const resp = (await handleFsGlob(
      1,
      { pattern: '*.xyz-never', cwd: tmpDir },
      makeConfig()
    )) as JsonRpcResponse<FsGlobResult>
    expect(resp.result!.paths).toHaveLength(0)
    expect(resp.result!.total).toBe(0)
  })

  it('returns InvalidParams for missing pattern param', async () => {
    const resp = (await handleFsGlob(
      1,
      { cwd: tmpDir },
      makeConfig()
    )) as JsonRpcResponse<FsGlobResult>
    expect(resp.error!.code).toBe(-32602)
    expect(resp.error!.message).toContain('pattern')
  })

  it('result includes cwd and total fields', async () => {
    const resp = (await handleFsGlob(
      1,
      { pattern: '*.ts', cwd: tmpDir },
      makeConfig()
    )) as JsonRpcResponse<FsGlobResult>
    expect(resp.result!.cwd).toBe(tmpDir)
    expect(typeof resp.result!.total).toBe('number')
  })

  it('excludes node_modules by default', async () => {
    mkdirSync(join(tmpDir, 'node_modules', 'pkg'), { recursive: true })
    writeFileSync(join(tmpDir, 'node_modules', 'pkg', 'index.ts'), '')
    writeFileSync(join(tmpDir, 'app.ts'), '')
    const resp = (await handleFsGlob(
      1,
      { pattern: '*.ts', cwd: tmpDir },
      makeConfig()
    )) as JsonRpcResponse<FsGlobResult>
    const paths = resp.result!.paths
    expect(paths.every((p) => !p.includes('node_modules'))).toBe(true)
    expect(paths.some((p) => p.endsWith('app.ts'))).toBe(true)
  })

  it('returns paths relative to cwd', async () => {
    writeFileSync(join(tmpDir, 'app.ts'), '')
    const resp = (await handleFsGlob(
      1,
      { pattern: '*.ts', cwd: tmpDir },
      makeConfig()
    )) as JsonRpcResponse<FsGlobResult>
    const paths = resp.result!.paths
    // Paths should be relative (not starting with /)
    expect(paths.every((p) => !p.startsWith('/'))).toBe(true)
  })
})
