import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  createCompatibleRuntimeStatusResponseIfNeeded,
  type RuntimeEnvironmentCallRequest
} from '../runtime/runtime-compatibility-test-fixture'
import { clearRuntimeCompatibilityCacheForTests } from '../runtime/runtime-rpc-client'
import { checkRepoIsNotAGitRepo } from './repo-git-status-check'

const runtimeEnvironmentCall = vi.fn()
const runtimeEnvironmentTransportCall = vi.fn()

beforeEach(() => {
  clearRuntimeCompatibilityCacheForTests()
  runtimeEnvironmentCall.mockReset()
  runtimeEnvironmentTransportCall.mockReset()
  runtimeEnvironmentTransportCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
    return createCompatibleRuntimeStatusResponseIfNeeded(args) ?? runtimeEnvironmentCall(args)
  })
  vi.stubGlobal('window', {
    api: {
      runtimeEnvironments: { call: runtimeEnvironmentTransportCall }
    }
  })
})

const settings = { activeRuntimeEnvironmentId: 'session-auth' }
const repo = { id: 'aiops-v3', projectId: 'default-project' }

describe('checkRepoIsNotAGitRepo', () => {
  it('returns false when worktree.detectedList succeeds', async () => {
    runtimeEnvironmentCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
      if (args.method === 'worktree.detectedList') {
        return {
          id: 'rpc-detected',
          ok: true,
          result: { repoId: repo.id, authoritative: true, source: 'git', worktrees: [] },
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      return { id: 'rpc-other', ok: true, result: {}, _meta: { runtimeId: 'runtime-remote' } }
    })

    await expect(checkRepoIsNotAGitRepo(repo, settings)).resolves.toBe(false)
  })

  it('returns true when the failure matches the not-a-git-repo signature', async () => {
    runtimeEnvironmentCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
      if (args.method === 'worktree.detectedList') {
        return {
          id: 'rpc-detected',
          ok: false,
          error: {
            code: 'WORKTREE_DETECT_FAILED',
            message:
              'WORKTREE_DETECT_FAILED: git worktree list failed: fatal: not a git repository (or any of the parent directories): .git'
          },
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      return { id: 'rpc-other', ok: true, result: {}, _meta: { runtimeId: 'runtime-remote' } }
    })

    await expect(checkRepoIsNotAGitRepo(repo, settings)).resolves.toBe(true)
  })

  it.each(['WORKTREE_REPO_NOT_FOUND', 'PROJECT_MEMBERSHIP_LOOKUP_FAILED'])(
    'returns false (fail open) for the known unrelated gap %s',
    async (code) => {
      runtimeEnvironmentCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
        if (args.method === 'worktree.detectedList') {
          return {
            id: 'rpc-detected',
            ok: false,
            error: { code, message: `${code}: something else entirely` },
            _meta: { runtimeId: 'runtime-remote' }
          }
        }
        return { id: 'rpc-other', ok: true, result: {}, _meta: { runtimeId: 'runtime-remote' } }
      })

      await expect(checkRepoIsNotAGitRepo(repo, settings)).resolves.toBe(false)
    }
  )

  it('returns false without calling the RPC when the repo has no projectId yet', async () => {
    await expect(
      checkRepoIsNotAGitRepo({ id: repo.id, projectId: undefined }, settings)
    ).resolves.toBe(false)
    expect(runtimeEnvironmentCall).not.toHaveBeenCalled()
  })

  it('returns false in local (non-environment) mode', async () => {
    await expect(checkRepoIsNotAGitRepo(repo, { activeRuntimeEnvironmentId: null })).resolves.toBe(
      false
    )
    expect(runtimeEnvironmentCall).not.toHaveBeenCalled()
  })
})
