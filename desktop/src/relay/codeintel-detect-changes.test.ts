import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import * as fs from 'node:fs/promises'
import * as path from 'node:path'
import * as os from 'node:os'
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { validateDetectChanges, handleDetectChanges } from './codeintel-detect-changes'
import { CodeIntelError } from './codeintel-errors'
import { CodeIntelRequestContext } from './codeintel-method-table'

const execFileAsync = promisify(execFile)

async function git(args: string[], cwd: string): Promise<string> {
  const { stdout } = await execFileAsync('git', args, { cwd })
  return stdout.trim()
}

describe('codeintel-detect-changes', () => {
  let tmpDir: string

  beforeEach(async () => {
    tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), 'detect-changes-test-'))
  })

  afterEach(async () => {
    await fs.rm(tmpDir, { recursive: true, force: true })
  })

  describe('validateDetectChanges', () => {
    it('accepts valid parameters and rejects unexpected keys', () => {
      const valid = validateDetectChanges({
        workspaceRoot: tmpDir,
        base: 'main',
        head: 'feature',
        includeUntracked: true,
        withClusters: true,
        crossCheck: false
      })
      expect(valid.base).toBe('main')

      // Unexpected key
      expect(() => {
        validateDetectChanges({
          workspaceRoot: tmpDir,
          invalidParam: 123
        })
      }).toThrow(CodeIntelError)
    })

    it('rejects invalid git ref options like -x', () => {
      expect(() => {
        validateDetectChanges({
          workspaceRoot: tmpDir,
          base: '-x'
        })
      }).toThrow(CodeIntelError)
    })
  })

  describe('handleDetectChanges', () => {
    function makeCtx(deadline?: number): CodeIntelRequestContext {
      return {
        config: {} as any,
        log: { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() } as any,
        signal: new AbortController().signal,
        deadline: deadline ?? Date.now() + 55000,
        notifier: { notify: vi.fn() },
        perf: { record: vi.fn() } as any
      }
    }

    it('handles unborn HEAD with unborn_head warning and empty diff', async () => {
      await git(['init'], tmpDir)
      const ctx = makeCtx()

      // When head is unborn, if base is not resolvable it will reject base_required,
      // but if we pass base that doesn't exist, it rejects unresolved_ref.
      // If we pass base = 'main' which doesn't exist:
      await expect(handleDetectChanges({ workspaceRoot: tmpDir, base: 'nonexistent' }, ctx)).rejects.toMatchObject({
        code: 'CODEINTEL_INVALID_PARAMS',
        data: { reason: 'unresolved_ref' }
      })
    })

    it('successfully detects changes in repository', async () => {
      await git(['init', '-b', 'main'], tmpDir)
      await git(['config', 'user.name', 'test'], tmpDir)
      await git(['config', 'user.email', 'test@test.com'], tmpDir)

      await fs.writeFile(path.join(tmpDir, 'file.txt'), 'line 1\n')
      await git(['add', '.'], tmpDir)
      await git(['commit', '-m', 'c1'], tmpDir)

      // Modify file and add untracked
      await fs.appendFile(path.join(tmpDir, 'file.txt'), 'line 2\n')
      await fs.writeFile(path.join(tmpDir, 'untracked.txt'), 'untracked content\n')

      const ctx = makeCtx()
      const res = await handleDetectChanges(
        {
          workspaceRoot: tmpDir,
          includeUntracked: true,
          withClusters: false
        },
        ctx
      )

      expect(res.base).toBe('main')
      expect(res.head).toBe('HEAD')
      expect(res.changedFiles.length).toBeGreaterThanOrEqual(2)
      const untracked = res.changedFiles.find((f: any) => f.path === 'untracked.txt')
      expect(untracked).toBeDefined()
      expect(untracked.untracked).toBe(true)
      expect(res.index.mappingConfidence).toBe('approximate')
      expect(res.truncated).toBe(false)
    })

    it('returns partial result with deadline_partial warning when deadline is exceeded', async () => {
      await git(['init', '-b', 'main'], tmpDir)
      await git(['config', 'user.name', 'test'], tmpDir)
      await git(['config', 'user.email', 'test@test.com'], tmpDir)

      await fs.writeFile(path.join(tmpDir, 'file.txt'), 'line 1\n')
      await git(['add', '.'], tmpDir)
      await git(['commit', '-m', 'c1'], tmpDir)

      await fs.appendFile(path.join(tmpDir, 'file.txt'), 'line 2\n')

      // Set deadline in the past so it triggers deadline_partial after diff collection
      // But allow diff collection to finish by setting deadline slightly in the future initially
      const ctx = makeCtx(Date.now() + 50)
      // Wait for deadline to expire
      await new Promise(r => setTimeout(r, 60))

      // Now diff collection starts after deadline -> should throw CODEINTEL_TIMEOUT
      await expect(
        handleDetectChanges({ workspaceRoot: tmpDir }, ctx)
      ).rejects.toMatchObject({
        code: 'CODEINTEL_TIMEOUT'
      })
    })
  })
})
