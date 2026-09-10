// src/relay/__tests__/fs-agent-write-extensions.test.ts
// Split out of fs-agent-extensions.test.ts to mirror the fs-agent-write-extensions.ts
// source split (max-lines ratchet) — pure test move, no behavior change.
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mkdtempSync, writeFileSync, rmSync, existsSync, readFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { handleFsWriteFile } from '../fs-agent-write-extensions'
import type { AgentConfig } from '../agent-config'

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
type FsWriteFileResult = { ok: boolean; path: string; bytes: number }

beforeEach(() => {
  tmpDir = mkdtempSync(join(tmpdir(), 'fs-ext-test-'))
})
afterEach(() => rmSync(tmpDir, { recursive: true, force: true }))

// ─── handleFsWriteFile ────────────────────────────────────────────────────────
describe('handleFsWriteFile', () => {
  it('writes file content and returns { ok: true }', async () => {
    const resp = (await handleFsWriteFile(
      1,
      { path: join(tmpDir, 'new-file.ts'), content: 'export const x = 1' },
      makeConfig()
    )) as JsonRpcResponse<FsWriteFileResult>
    expect(resp.result!.ok).toBe(true)
    expect(existsSync(join(tmpDir, 'new-file.ts'))).toBe(true)
  })

  it('returns bytes count in result', async () => {
    const content = 'hello world'
    const resp = (await handleFsWriteFile(
      1,
      { path: join(tmpDir, 'bytes-test.txt'), content },
      makeConfig()
    )) as JsonRpcResponse<FsWriteFileResult>
    expect(resp.result!.bytes).toBe(Buffer.byteLength(content, 'utf-8'))
  })

  it('returns absolute path in result', async () => {
    const filePath = join(tmpDir, 'abs-path.ts')
    const resp = (await handleFsWriteFile(
      1,
      { path: filePath, content: 'x' },
      makeConfig()
    )) as JsonRpcResponse<FsWriteFileResult>
    expect(resp.result!.path).toBe(filePath)
  })

  it('creates parent directories automatically', async () => {
    const nestedPath = join(tmpDir, 'a', 'b', 'c', 'nested.ts')
    const resp = (await handleFsWriteFile(
      1,
      { path: nestedPath, content: 'export {}' },
      makeConfig()
    )) as JsonRpcResponse<FsWriteFileResult>
    expect(resp.result!.ok).toBe(true)
    expect(existsSync(nestedPath)).toBe(true)
  })

  it('overwrites existing file', async () => {
    const filePath = join(tmpDir, 'overwrite.ts')
    writeFileSync(filePath, 'old content')
    await handleFsWriteFile(1, { path: filePath, content: 'new content' }, makeConfig())
    expect(readFileSync(filePath, 'utf-8')).toBe('new content')
  })

  it('rejects path traversal outside workDir', async () => {
    const resp = (await handleFsWriteFile(
      1,
      { path: join(tmpDir, '../../../etc/passwd'), content: 'evil' },
      makeConfig()
    )) as JsonRpcResponse<FsWriteFileResult>
    expect(resp.error).toBeDefined()
    expect(resp.error!.code).toBe(-32602)
    expect(resp.error!.message).toContain('outside project root')
  })

  it('returns InvalidParams for missing path param', async () => {
    const resp = (await handleFsWriteFile(
      1,
      { content: 'data' },
      makeConfig()
    )) as JsonRpcResponse<FsWriteFileResult>
    expect(resp.error!.code).toBe(-32602)
  })

  it('returns error for content exceeding 10MB', async () => {
    const bigContent = 'x'.repeat(11 * 1024 * 1024) // 11MB
    const resp = (await handleFsWriteFile(
      1,
      { path: join(tmpDir, 'big.txt'), content: bigContent },
      makeConfig()
    )) as JsonRpcResponse<FsWriteFileResult>
    expect(resp.error).toBeDefined()
    expect(resp.error!.message).toContain('10MB')
  })

  it('resolves relative path against workDir', async () => {
    const resp = (await handleFsWriteFile(
      1,
      { path: 'relative-write.ts', content: 'export const x = 1' },
      makeConfig()
    )) as JsonRpcResponse<FsWriteFileResult>
    expect(resp.result!.ok).toBe(true)
    expect(existsSync(join(tmpDir, 'relative-write.ts'))).toBe(true)
  })
})
