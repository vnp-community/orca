import { describe, it, expect } from 'vitest'
import * as fs from 'node:fs/promises'
import * as os from 'node:os'
import * as path from 'node:path'
import {
  validateWorkspace,
  defaultScratchRoots
} from './agent-workspace-validation'

describe('agent-workspace-validation', () => {
  describe('worktree', () => {
    it('returns realPath equal to input without touching filesystem or git', async () => {
      const result = await validateWorkspace({
        kind: 'worktree',
        path: '/any/non/existent/path/worktree',
        accessMode: 'write',
        scratchRoots: []
      })
      expect(result).toEqual({ ok: true, realPath: '/any/non/existent/path/worktree' })
    })
  })

  describe('repo_root', () => {
    it('rejects when accessMode is write with REPO_ROOT_REQUIRES_READONLY before touching fs', async () => {
      const result = await validateWorkspace({
        kind: 'repo_root',
        path: '/any/path',
        accessMode: 'write',
        scratchRoots: []
      })
      expect(result.ok).toBe(false)
      if (!result.ok) {
        expect(result.code).toBe('REPO_ROOT_REQUIRES_READONLY')
      }
    })

    it('returns WORKSPACE_PATH_NOT_FOUND when directory does not exist', async () => {
      const result = await validateWorkspace({
        kind: 'repo_root',
        path: '/path/does/not/exist/at/all',
        accessMode: 'readonly',
        scratchRoots: []
      })
      expect(result.ok).toBe(false)
      if (!result.ok) {
        expect(result.code).toBe('WORKSPACE_PATH_NOT_FOUND')
      }
    })

    it('returns WORKSPACE_NOT_A_DIRECTORY when path is a regular file', async () => {
      const tempFile = path.join(os.tmpdir(), `test-file-${Date.now()}.txt`)
      await fs.writeFile(tempFile, 'hello')
      try {
        const result = await validateWorkspace({
          kind: 'repo_root',
          path: tempFile,
          accessMode: 'readonly',
          scratchRoots: []
        })
        expect(result.ok).toBe(false)
        if (!result.ok) {
          expect(result.code).toBe('WORKSPACE_NOT_A_DIRECTORY')
        }
      } finally {
        await fs.unlink(tempFile).catch(() => {})
      }
    })

    it('returns WORKSPACE_NOT_A_GIT_REPO when git rev-parse fails', async () => {
      const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), 'non-git-'))
      try {
        const result = await validateWorkspace({
          kind: 'repo_root',
          path: tempDir,
          accessMode: 'readonly',
          scratchRoots: [],
          gitToplevel: async () => null // Simulates not a git repo
        })
        expect(result.ok).toBe(false)
        if (!result.ok) {
          expect(result.code).toBe('WORKSPACE_NOT_A_GIT_REPO')
        }
      } finally {
        await fs.rm(tempDir, { recursive: true, force: true }).catch(() => {})
      }
    })

    it('accepts a valid directory inside a git repo when readonly', async () => {
      const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), 'valid-git-'))
      try {
        const result = await validateWorkspace({
          kind: 'repo_root',
          path: tempDir,
          accessMode: 'readonly',
          scratchRoots: [],
          gitToplevel: async () => tempDir
        })
        expect(result.ok).toBe(true)
        if (result.ok) {
          expect(result.realPath).toBeTruthy()
        }
      } finally {
        await fs.rm(tempDir, { recursive: true, force: true }).catch(() => {})
      }
    })
  })

  describe('scratch', () => {
    it('returns WORKSPACE_PATH_NOT_FOUND when scratch path does not exist', async () => {
      const result = await validateWorkspace({
        kind: 'scratch',
        path: '/path/does/not/exist',
        accessMode: 'write',
        scratchRoots: [os.tmpdir()]
      })
      expect(result.ok).toBe(false)
      if (!result.ok) {
        expect(result.code).toBe('WORKSPACE_PATH_NOT_FOUND')
      }
    })

    it('returns WORKSPACE_NOT_A_DIRECTORY when scratch path is a file', async () => {
      const tempFile = path.join(os.tmpdir(), `scratch-file-${Date.now()}.txt`)
      await fs.writeFile(tempFile, 'data')
      try {
        const result = await validateWorkspace({
          kind: 'scratch',
          path: tempFile,
          accessMode: 'write',
          scratchRoots: [os.tmpdir()]
        })
        expect(result.ok).toBe(false)
        if (!result.ok) {
          expect(result.code).toBe('WORKSPACE_NOT_A_DIRECTORY')
        }
      } finally {
        await fs.unlink(tempFile).catch(() => {})
      }
    })

    it('accepts path located inside an allowed scratch root', async () => {
      const rootDir = await fs.mkdtemp(path.join(os.tmpdir(), 'root-scratch-'))
      const childDir = path.join(rootDir, 'nested', 'job-1')
      await fs.mkdir(childDir, { recursive: true })
      try {
        const result = await validateWorkspace({
          kind: 'scratch',
          path: childDir,
          accessMode: 'write',
          scratchRoots: [rootDir]
        })
        expect(result.ok).toBe(true)
      } finally {
        await fs.rm(rootDir, { recursive: true, force: true }).catch(() => {})
      }
    })

    it('rejects path outside allowed scratch roots with SCRATCH_OUTSIDE_ALLOWED_ROOTS', async () => {
      const rootDir = await fs.mkdtemp(path.join(os.tmpdir(), 'allowed-root-'))
      const otherDir = await fs.mkdtemp(path.join(os.tmpdir(), 'other-dir-'))
      try {
        const result = await validateWorkspace({
          kind: 'scratch',
          path: otherDir,
          accessMode: 'write',
          scratchRoots: [rootDir]
        })
        expect(result.ok).toBe(false)
        if (!result.ok) {
          expect(result.code).toBe('SCRATCH_OUTSIDE_ALLOWED_ROOTS')
        }
      } finally {
        await fs.rm(rootDir, { recursive: true, force: true }).catch(() => {})
        await fs.rm(otherDir, { recursive: true, force: true }).catch(() => {})
      }
    })
  })

  describe('defaultScratchRoots', () => {
    it('includes os.tmpdir() and .orca-scratch under workDir', () => {
      const roots = defaultScratchRoots('/Users/test/workspace')
      expect(roots).toContain(os.tmpdir())
      expect(roots).toContain(path.join('/Users/test/workspace', '.orca-scratch'))
    })
  })
})
