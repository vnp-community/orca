import { describe, it, expect, vi } from 'vitest'
import fs from 'fs'
import path from 'path'
import os from 'os'
import {
  validateCodeIntelParams,
  assertRelativeRepoPath
} from '../codeintel-params-validation'
import { resolveCodeIntelRepo, invalidateRepoBindings } from '../codeintel-repo-resolution'
import { CodeIntelError } from '../codeintel-errors'

interface PathVector {
  input: string
  field: string
  expect: 'deny' | 'allow'
  code: string
  note: string
}

describe('security-workspace-root-and-paths (Task 072-05)', () => {
  const vectorsPath = path.resolve(__dirname, '__fixtures__/path-attack-vectors.json')
  const vectors: PathVector[] = JSON.parse(fs.readFileSync(vectorsPath, 'utf8'))

  describe('Path attack vectors validation', () => {
    for (const v of vectors) {
      it(`evaluates ${v.field}: "${v.input}" -> ${v.expect} (${v.note})`, () => {
        if (v.field === 'workspaceRoot') {
          if (v.expect === 'deny') {
            expect(() => {
              validateCodeIntelParams({ workspaceRoot: v.input }, {})
            }).toThrow(CodeIntelError)
            try {
              validateCodeIntelParams({ workspaceRoot: v.input }, {})
            } catch (err: any) {
              expect(err.code).toBe(v.code)
            }
          } else {
            expect(() => {
              validateCodeIntelParams({ workspaceRoot: v.input }, {})
            }).not.toThrow()
          }
        } else if (v.field === 'filePath') {
          if (v.expect === 'deny') {
            expect(() => {
              assertRelativeRepoPath(v.input, 'filePath')
            }).toThrow(CodeIntelError)
            try {
              assertRelativeRepoPath(v.input, 'filePath')
            } catch (err: any) {
              expect(err.code).toBe(v.code)
            }
          } else {
            expect(() => {
              assertRelativeRepoPath(v.input, 'filePath')
            }).not.toThrow()
          }
        }
      })
    }
  })

  it('rejects relative workspaceRoot before inspecting filesystem realpath', async () => {
    const realpathSpy = vi.spyOn(fs, 'realpathSync')
    expect(() => {
      validateCodeIntelParams({ workspaceRoot: '../../etc' }, {})
    }).toThrow(CodeIntelError)
    expect(realpathSpy).not.toHaveBeenCalled()
    realpathSpy.mockRestore()
  })

  describe('Symlink and ALLOWED_ROOTS boundary enforcement', () => {
    const tmpBase = fs.mkdtempSync(path.join(os.tmpdir(), 'orca-security-root-'))
    const allowedDir = path.join(tmpBase, 'allowed')
    const outsideDir = path.join(tmpBase, 'outside')
    fs.mkdirSync(allowedDir, { recursive: true })
    fs.mkdirSync(outsideDir, { recursive: true })

    const fakeConfig: any = { mode: 'direct-websocket', orcaUrl: '' }

    it('enforces directory boundary and rejects prefix collisions', async () => {
      const originalEnv = process.env.ORCA_CODEINTEL_ALLOWED_ROOTS
      process.env.ORCA_CODEINTEL_ALLOWED_ROOTS = allowedDir

      try {
        // Path outside allowed root is rejected
        await expect(resolveCodeIntelRepo(outsideDir, fakeConfig)).rejects.toThrow(CodeIntelError)

        // Sibling dir starting with same prefix but not a subfolder (e.g., allowed-old)
        const prefixCollisionDir = path.join(tmpBase, 'allowed-collision')
        fs.mkdirSync(prefixCollisionDir, { recursive: true })
        await expect(resolveCodeIntelRepo(prefixCollisionDir, fakeConfig)).rejects.toThrow(CodeIntelError)
      } finally {
        if (originalEnv !== undefined) {
          process.env.ORCA_CODEINTEL_ALLOWED_ROOTS = originalEnv
        } else {
          delete process.env.ORCA_CODEINTEL_ALLOWED_ROOTS
        }
        invalidateRepoBindings()
        fs.rmSync(tmpBase, { recursive: true, force: true })
      }
    })
  })
})
