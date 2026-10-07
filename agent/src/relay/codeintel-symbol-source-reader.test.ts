import { describe, it, expect, vi, beforeEach } from 'vitest'
import fs from 'fs'
import path from 'path'
import { readSymbolSource } from './codeintel-symbol-source-reader'
import * as GitExec from './codeintel-git-exec'

vi.mock('fs', () => ({
  default: {
    realpathSync: { native: vi.fn() },
    openSync: vi.fn(),
    readSync: vi.fn(),
    closeSync: vi.fn(),
    readFileSync: vi.fn()
  }
}))

vi.mock('./codeintel-git-exec', () => ({
  runGit: vi.fn()
}))

describe('codeintel-symbol-source-reader', () => {
  const workspaceRoot = '/repo'

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(fs.realpathSync.native).mockImplementation((p: string) => p)
    vi.mocked(fs.openSync).mockReturnValue(1)
    vi.mocked(fs.readSync).mockReturnValue(10)
    vi.mocked(fs.readFileSync).mockReturnValue('line1\nline2\nline3\nline4\n')
    vi.mocked(GitExec.runGit).mockResolvedValue({ exitCode: 1, stdout: '', stderr: '', durationMs: 0, stdoutBytes: 0 })
  })

  it('reads correct lines', async () => {
    const res = await readSymbolSource(workspaceRoot, 'foo.ts', 2, 3)
    expect(res.text).toBe('line2\nline3')
  })

  it('returns empty if outside workspace', async () => {
    vi.mocked(fs.realpathSync.native).mockImplementation((p: string) => {
      if (p === workspaceRoot) return workspaceRoot
      return '/outside/foo.ts'
    })
    const res = await readSymbolSource(workspaceRoot, '../outside/foo.ts', 1, 2)
    expect(res.text).toBe('')
  })

  it('returns gitignored if git check-ignore returns 0', async () => {
    vi.mocked(GitExec.runGit).mockResolvedValue({ exitCode: 0, stdout: '', stderr: '', durationMs: 0, stdoutBytes: 0 })
    const res = await readSymbolSource(workspaceRoot, 'node_modules/foo.ts', 1, 2)
    expect(res.sourceOmitted).toBe('gitignored')
  })

  it('returns binary if NUL byte found', async () => {
    vi.mocked(fs.readSync).mockImplementation((fd, buf: any) => {
      buf[0] = 0 // NUL byte
      return 10
    })
    const res = await readSymbolSource(workspaceRoot, 'foo.bin', 1, 2)
    expect(res.sourceOmitted).toBe('binary')
  })
})
