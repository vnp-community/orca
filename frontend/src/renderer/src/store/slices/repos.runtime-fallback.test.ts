import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createTestStore } from './store-test-helpers'
import type { Repo } from '../../../../shared/types'
import {
  FOLDER_WORKSPACE_PATH_STATUS_RUNTIME_CAPABILITY,
  RUNTIME_CAPABILITIES
} from '../../../../shared/protocol-version'
import {
  createCompatibleRuntimeStatusResponse,
  createCompatibleRuntimeStatusResponseIfNeeded,
  type RuntimeEnvironmentCallRequest
} from '../../runtime/runtime-compatibility-test-fixture'
import { clearRuntimeCompatibilityCacheForTests } from '../../runtime/runtime-rpc-client'
import { resetDefaultProjectCacheForTests } from './repos'

const toastError = vi.hoisted(() => vi.fn())
const toastInfo = vi.hoisted(() => vi.fn())
const toastSuccess = vi.hoisted(() => vi.fn())

vi.mock('sonner', () => ({
  toast: {
    error: toastError,
    info: toastInfo,
    success: toastSuccess
  }
}))

const remoteRepo: Repo = {
  id: 'remote-repo',
  path: '/remote',
  displayName: 'Remote',
  badgeColor: '#111',
  addedAt: 2
}

const runtimeEnvironmentCall = vi.fn()
const runtimeEnvironmentTransportCall = vi.fn()

// Phase 4b: the runtime path's addRepoPath resolves an implicit default
// OrcaProject (getOrCreateDefaultProject -> project.list) before repo.add
// runs at all. Every mock below must answer 'project.list' with this fixed
// project alongside whatever repo.add/folderWorkspace.getPathStatus behavior
// the test is actually exercising.
const DEFAULT_PROJECT_ID = 'default-project'
function defaultProjectListResult() {
  return {
    id: 'rpc-project-list',
    ok: true,
    result: [
      {
        id: DEFAULT_PROJECT_ID,
        name: 'My Repos',
        defaultBranch: 'main',
        devServerId: '',
        visibility: 'private',
        createdAt: 1,
        updatedAt: 1
      }
    ],
    _meta: { runtimeId: 'runtime-remote' }
  }
}

beforeEach(() => {
  clearRuntimeCompatibilityCacheForTests()
  resetDefaultProjectCacheForTests()
  runtimeEnvironmentCall.mockReset()
  runtimeEnvironmentTransportCall.mockReset()
  toastError.mockReset()
  toastInfo.mockReset()
  toastSuccess.mockReset()
  runtimeEnvironmentTransportCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
    return createCompatibleRuntimeStatusResponseIfNeeded(args) ?? runtimeEnvironmentCall(args)
  })
  vi.stubGlobal('window', {
    api: {
      runtimeEnvironments: { call: runtimeEnvironmentTransportCall }
    }
  })
})

describe('repo slice runtime folder fallback', () => {
  it('blocks wrong-host runtime fallback', async () => {
    runtimeEnvironmentCall.mockImplementation((request: RuntimeEnvironmentCallRequest) => {
      const { method } = request
      if (method === 'project.list') {
        return defaultProjectListResult()
      }
      if (method === 'repo.add') {
        return {
          id: 'rpc-add-git',
          ok: false,
          error: { code: 'repo.invalid', message: 'Not a valid git repository' },
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      if (method === 'folderWorkspace.getPathStatus') {
        return {
          id: 'rpc-path-status',
          ok: true,
          result: { status: 'PATH_STATUS_INVALID' },
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      if (method === 'projectGroup.delete') {
        return {
          id: 'rpc-delete-status-scope',
          ok: true,
          result: { deleted: true },
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      throw new Error(`Unexpected runtime method ${method}`)
    })
    const store = createTestStore()
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'env-1' } as never,
      runtimeEnvironments: [{ id: 'env-1', name: 'Remote Mac' }] as never,
      activeDevServerId: null
    })

    await expect(
      store.getState().addRepoPath('/Users/me/GitHub/travel-hub', 'git')
    ).resolves.toBeNull()

    expect(store.getState().activeModal).not.toBe('confirm-non-git-folder')
    expect(runtimeEnvironmentCall).toHaveBeenCalledWith({
      selector: 'env-1',
      method: 'folderWorkspace.getPathStatus',
      params: { devServerId: null, path: '/Users/me/GitHub/travel-hub' },
      timeoutMs: 15_000
    })
    expect(toastError).toHaveBeenCalledWith(
      'Cannot open folder on selected runtime',
      expect.objectContaining({
        description: expect.stringContaining('Remote Mac')
      })
    )
  })

  it('treats runtime status RPC failures as host-scoped errors', async () => {
    runtimeEnvironmentCall.mockImplementation((request: RuntimeEnvironmentCallRequest) => {
      const { method } = request
      if (method === 'project.list') {
        return defaultProjectListResult()
      }
      if (method === 'repo.add') {
        return {
          id: 'rpc-add-git',
          ok: false,
          error: { code: 'repo.invalid', message: 'Not a valid git repository' },
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      if (method === 'folderWorkspace.getPathStatus') {
        throw new Error('status unavailable')
      }
      throw new Error(`Unexpected runtime method ${method}`)
    })
    const store = createTestStore()
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'env-1' } as never,
      runtimeEnvironments: [{ id: 'env-1', name: 'Remote Mac' }] as never
    })

    await expect(
      store.getState().addRepoPath('/Users/me/GitHub/travel-hub', 'git')
    ).resolves.toBeNull()

    expect(store.getState().activeModal).not.toBe('confirm-non-git-folder')
    expect(toastError).toHaveBeenCalledWith(
      'Cannot open folder on selected runtime',
      expect.objectContaining({
        description: expect.stringContaining('Remote Mac')
      })
    )
  })

  it('reports an update error when the checked runtime lacks raw path status support', async () => {
    runtimeEnvironmentTransportCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
      if (args.method === 'status.get') {
        const response = createCompatibleRuntimeStatusResponse()
        if (!response.ok) {
          throw new Error('Expected compatible runtime status fixture')
        }
        return {
          ...response,
          result: {
            ...response.result,
            capabilities: RUNTIME_CAPABILITIES.filter(
              (capability) => capability !== FOLDER_WORKSPACE_PATH_STATUS_RUNTIME_CAPABILITY
            )
          }
        }
      }
      return runtimeEnvironmentCall(args)
    })
    runtimeEnvironmentCall.mockImplementation((request: RuntimeEnvironmentCallRequest) => {
      const { method } = request
      if (method === 'project.list') {
        return defaultProjectListResult()
      }
      if (method === 'repo.add') {
        return {
          id: 'rpc-add-git',
          ok: false,
          error: { code: 'repo.invalid', message: 'Not a valid git repository' },
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      throw new Error(`Unexpected runtime method ${method}`)
    })
    const store = createTestStore()
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'env-1' } as never,
      runtimeEnvironments: [{ id: 'env-1', name: 'Remote Mac' }] as never
    })

    await expect(store.getState().addRepoPath('/srv/non-git', 'git')).resolves.toBeNull()

    expect(store.getState().activeModal).not.toBe('confirm-non-git-folder')
    expect(runtimeEnvironmentCall).not.toHaveBeenCalledWith(
      expect.objectContaining({
        method: 'folderWorkspace.getPathStatus'
      })
    )
    expect(toastError).toHaveBeenCalledWith(
      'Failed to add project',
      expect.objectContaining({
        description: 'Update Orca server to open non-Git folders on this runtime.'
      })
    )
  })

  it('keeps runtime folder fallback on the checked host', async () => {
    const folderRepo: Repo = {
      ...remoteRepo,
      id: 'runtime-folder',
      path: '/srv/non-git',
      displayName: 'non-git',
      kind: 'folder'
    }
    // Why a call counter, not a `kind` param branch: the fixed wire request
    // (channels_repo_ssh_status_workspace.go's AddRepoRequest) never carries
    // `kind` at all — repo.add only ever sees {projectId, url, displayName}.
    // addRepoPath's own `kind` argument is a purely local annotation applied
    // after the RPC response comes back. The test still needs the *first*
    // repo.add (from the initial 'git' addRepoPath call below) to reject as
    // non-git, and the *second* (from addNonGitFolder's own 'folder' retry)
    // to succeed, so count calls instead.
    let repoAddCalls = 0
    runtimeEnvironmentCall.mockImplementation((request) => {
      const { selector, method } = request as {
        selector: string
        method: string
        params?: unknown
      }
      if (method === 'project.list') {
        return defaultProjectListResult()
      }
      if (method === 'repo.add') {
        repoAddCalls += 1
        if (repoAddCalls === 1) {
          return {
            id: 'rpc-add-git',
            ok: false,
            error: { code: 'repo.invalid', message: 'Not a valid git repository' },
            _meta: { runtimeId: 'runtime-remote' }
          }
        }
        return {
          id: 'rpc-add-folder',
          ok: true,
          result: {
            id: folderRepo.id,
            projectId: DEFAULT_PROJECT_ID,
            url: folderRepo.path,
            displayName: folderRepo.displayName,
            position: 0
          },
          _meta: { runtimeId: `runtime-${selector}` }
        }
      }
      if (method === 'folderWorkspace.getPathStatus') {
        return {
          id: 'rpc-path-status',
          ok: true,
          result: { status: 'PATH_STATUS_AVAILABLE' },
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      throw new Error(`Unexpected runtime method ${method}`)
    })
    const store = createTestStore()
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'env-1' } as never,
      fetchWorktrees: vi.fn().mockResolvedValue(undefined) as never,
      activeDevServerId: null
    })

    await expect(store.getState().addRepoPath('/srv/non-git', 'git')).resolves.toBeNull()

    expect(store.getState().activeModal).toBe('confirm-non-git-folder')
    expect(store.getState().modalData).toEqual({
      folderPath: '/srv/non-git',
      runtimeEnvironmentId: 'env-1'
    })

    store.setState({ settings: { activeRuntimeEnvironmentId: 'env-2' } as never })
    await expect(
      store.getState().addNonGitFolder('/srv/non-git', { runtimeEnvironmentId: 'env-1' })
    ).resolves.toEqual({
      id: folderRepo.id,
      projectId: DEFAULT_PROJECT_ID,
      path: folderRepo.path,
      displayName: folderRepo.displayName,
      badgeColor: '',
      addedAt: expect.any(Number),
      kind: 'folder',
      executionHostId: 'runtime:env-1'
    })

    expect(runtimeEnvironmentCall).toHaveBeenCalledWith({
      selector: 'env-1',
      method: 'repo.add',
      params: { projectId: DEFAULT_PROJECT_ID, url: '/srv/non-git', displayName: 'non-git' },
      timeoutMs: 15_000
    })
    expect(runtimeEnvironmentCall).not.toHaveBeenCalledWith(
      expect.objectContaining({
        selector: 'env-2',
        method: 'repo.add',
        params: { projectId: DEFAULT_PROJECT_ID, url: '/srv/non-git', displayName: 'non-git' }
      })
    )
  })

  it('adds a local non-git folder directly without runtime metadata or a worktree', async () => {
    // Local analogue of the runtime fallback above: with no active runtime the add
    // routes through window.api.repos.add (not runtime RPC). A non-git path returns
    // Orca's own "Not a valid git repository" sentinel, so addRepoPath surfaces the
    // confirmation modal with no runtime/SSH metadata instead of throwing.
    const folderRepo: Repo = {
      id: 'local-folder',
      path: '/local/non-git',
      displayName: 'non-git',
      badgeColor: '#000',
      addedAt: 1,
      kind: 'folder'
    }
    const reposAdd = vi.fn(
      (args: { path: string; kind: string }): { repo: Repo } | { error: string } =>
        args.kind === 'folder'
          ? { repo: folderRepo }
          : { error: 'Not a valid git repository: /local/non-git' }
    )
    vi.stubGlobal('window', {
      api: {
        repos: { add: reposAdd },
        runtimeEnvironments: { call: runtimeEnvironmentTransportCall }
      }
    })
    const store = createTestStore()
    // fetchWorktrees is stubbed so the post-add activation chain (which needs
    // worktrees/onboarding APIs absent from this stub) stays out of scope.
    store.setState({ fetchWorktrees: vi.fn().mockResolvedValue(undefined) as never })

    await expect(store.getState().addRepoPath('/local/non-git', 'git')).resolves.toBeNull()

    expect(runtimeEnvironmentTransportCall).not.toHaveBeenCalled()
    expect(reposAdd).toHaveBeenNthCalledWith(1, {
      path: '/local/non-git',
      kind: 'git'
    })
    expect(store.getState().activeModal).toBe('confirm-non-git-folder')
    expect(store.getState().modalData).toEqual({ folderPath: '/local/non-git' })
    expect(store.getState().repos).toEqual([])

    // Completing the flow via "Open as Folder" adds a kind:'folder' project whose
    // workspace path IS the folder itself — proving no git worktree is created.
    const added = await store.getState().addNonGitFolder('/local/non-git')

    expect(reposAdd).toHaveBeenNthCalledWith(2, {
      path: '/local/non-git',
      kind: 'folder'
    })
    expect(added?.kind).toBe('folder')
    expect(added?.path).toBe('/local/non-git')
  })
})
