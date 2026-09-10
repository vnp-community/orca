import { beforeEach, describe, expect, it, vi } from 'vitest'
import type * as ReactModule from 'react'
import type { Repo } from '../../../../shared/types'

const mocks = vi.hoisted(() => ({
  stateValues: [] as unknown[],
  stateSetters: [] as ReturnType<typeof vi.fn>[],
  stateIndex: 0,
  storeState: {
    settings: { activeRuntimeEnvironmentId: null as string | null },
    repos: [] as Repo[],
    projects: [],
    projectHostSetups: [],
    worktreesByRepo: {} as Record<string, unknown[]>
  },
  createRepo: vi.fn(),
  createRemoteRepo: vi.fn(),
  callRuntimeRpc: vi.fn(),
  fetchWorktrees: vi.fn(),
  onGitRepoReady: vi.fn(),
  activateAndRevealWorktree: vi.fn(),
  markOnboardingProjectAdded: vi.fn()
}))

vi.mock('react', async (importOriginal) => {
  const actual = await importOriginal<typeof ReactModule>()
  return {
    ...actual,
    useCallback: <T extends (...args: never[]) => unknown>(fn: T) => fn,
    useRef: <T>(value: T) => ({ current: value }),
    useState: <T>(initial: T | (() => T)) => {
      const index = mocks.stateIndex++
      const value =
        index in mocks.stateValues
          ? mocks.stateValues[index]
          : typeof initial === 'function'
            ? (initial as () => T)()
            : initial
      const setter = vi.fn()
      mocks.stateSetters[index] = setter
      return [value as T, setter]
    }
  }
})

vi.mock('@/hooks/useMountedRef', () => ({
  useMountedRef: () => ({ current: true })
}))

vi.mock('@/lib/worktree-activation', () => ({
  activateAndRevealWorktree: mocks.activateAndRevealWorktree
}))

vi.mock('@/lib/onboarding-project-checklist', () => ({
  markOnboardingProjectAdded: mocks.markOnboardingProjectAdded
}))

vi.mock('@/store', () => {
  const useAppStore = Object.assign(
    (selector: (state: typeof mocks.storeState) => unknown) => selector(mocks.storeState),
    {
      getState: () => mocks.storeState,
      setState: (next: Partial<typeof mocks.storeState>) => {
        Object.assign(mocks.storeState, next)
      }
    }
  )
  return { useAppStore }
})

vi.mock('sonner', () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
    info: vi.fn()
  }
}))

vi.mock('@/runtime/runtime-rpc-client', () => ({
  // Mirrors the real getActiveRuntimeTarget: 'local' only when
  // activeRuntimeEnvironmentId is falsy — needed so the devServerId
  // regression test below (which relies on this resolving to
  // 'environment' from the store's real, non-nulled settings) is
  // actually exercising useCreateRepo's own logic, not a mock that
  // always answers the same way regardless of input.
  getActiveRuntimeTarget: (settings: { activeRuntimeEnvironmentId?: string | null }) =>
    settings?.activeRuntimeEnvironmentId
      ? { kind: 'environment', environmentId: settings.activeRuntimeEnvironmentId }
      : { kind: 'local' },
  callRuntimeRpc: mocks.callRuntimeRpc
}))

const STATE_NAME = 0
const STATE_PARENT_PATH = 1
const STATE_ERROR_MESSAGE = 2
const STATE_IS_CREATING = 3

function makeRepo(overrides: Partial<Repo> = {}): Repo {
  return {
    id: 'repo-created',
    path: '/projects/created',
    displayName: 'created',
    badgeColor: '#999999',
    addedAt: 1,
    kind: 'git',
    ...overrides
  }
}

describe('useCreateRepo default-checkout handoff', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.stateIndex = 0
    mocks.stateSetters = []
    mocks.stateValues = []
    mocks.stateValues[STATE_NAME] = 'created'
    mocks.stateValues[STATE_PARENT_PATH] = '/projects'
    mocks.stateValues[STATE_ERROR_MESSAGE] = null
    mocks.stateValues[STATE_IS_CREATING] = false
    mocks.storeState.repos = []
    mocks.storeState.projects = []
    mocks.storeState.projectHostSetups = []
    mocks.storeState.worktreesByRepo = {}
    mocks.createRepo.mockReset()
    mocks.createRemoteRepo.mockReset()
    mocks.storeState.settings.activeRuntimeEnvironmentId = null
    vi.stubGlobal('window', {
      api: {
        repos: {
          create: mocks.createRepo,
          createRemote: mocks.createRemoteRepo,
          pickDirectory: vi.fn()
        }
      }
    })
  })

  it('requests an authoritative worktree refresh before handoff', async () => {
    const repo = makeRepo()
    mocks.createRepo.mockResolvedValue({ repo })
    mocks.fetchWorktrees.mockResolvedValue(true)
    const { useCreateRepo } = await import('./useCreateRepo')

    const result = useCreateRepo(mocks.fetchWorktrees, vi.fn(), mocks.onGitRepoReady)
    await result.handleCreate()

    expect(mocks.createRepo).toHaveBeenCalledWith({
      parentPath: '/projects',
      name: 'created',
      kind: 'git'
    })
    expect(mocks.fetchWorktrees).toHaveBeenCalledWith(repo.id, {
      requireAuthoritative: true
    })
    expect(mocks.storeState.projects).toEqual(
      expect.arrayContaining([expect.objectContaining({ sourceRepoIds: [repo.id] })])
    )
    expect(mocks.storeState.projectHostSetups).toEqual(
      expect.arrayContaining([expect.objectContaining({ repoId: repo.id, path: repo.path })])
    )
    expect(mocks.onGitRepoReady).toHaveBeenCalledWith(repo.id)
  })

  it('returns the selected parent directory after the local picker applies it', async () => {
    const pickedDir = '/Users/alice/custom-projects'
    vi.mocked(window.api.repos.pickDirectory).mockResolvedValue(pickedDir)
    const { useCreateRepo } = await import('./useCreateRepo')

    const result = useCreateRepo(mocks.fetchWorktrees, vi.fn(), mocks.onGitRepoReady)
    await expect(result.handlePickParent()).resolves.toBe(pickedDir)

    expect(mocks.stateSetters[STATE_PARENT_PATH]).toHaveBeenCalledWith(pickedDir)
  })

  it('does not return a parent path when the runtime target blocks the local picker', async () => {
    const { useCreateRepo } = await import('./useCreateRepo')

    const result = useCreateRepo(mocks.fetchWorktrees, vi.fn(), mocks.onGitRepoReady, {
      runtimeEnvironmentId: 'env-1'
    })
    await expect(result.handlePickParent()).resolves.toBeNull()

    expect(window.api.repos.pickDirectory).not.toHaveBeenCalled()
    expect(mocks.stateSetters[STATE_PARENT_PATH]).not.toHaveBeenCalled()
  })

  it('continues to completion when refresh is not authoritative after create', async () => {
    const repo = makeRepo()
    mocks.createRepo.mockResolvedValue({ repo })
    mocks.fetchWorktrees.mockResolvedValue(false)
    const { useCreateRepo } = await import('./useCreateRepo')

    const result = useCreateRepo(mocks.fetchWorktrees, vi.fn(), mocks.onGitRepoReady)
    await result.handleCreate()

    expect(mocks.fetchWorktrees).toHaveBeenCalledWith(repo.id, {
      requireAuthoritative: true
    })
    expect(mocks.onGitRepoReady).toHaveBeenCalledWith(repo.id)
    expect(mocks.stateSetters[STATE_ERROR_MESSAGE]).not.toHaveBeenCalledWith(
      'Could not refresh project worktrees. Try again.'
    )
  })

  it('opens an existing folder project returned by create dedupe', async () => {
    const repo = makeRepo({ kind: 'folder' })
    const worktree = { id: `${repo.id}::/projects/created` }
    const closeModal = vi.fn()
    mocks.createRepo.mockResolvedValue({ repo })
    mocks.fetchWorktrees.mockImplementation(async (repoId: string) => {
      mocks.storeState.worktreesByRepo = { [repoId]: [worktree] }
      return true
    })
    const { useCreateRepo } = await import('./useCreateRepo')

    const result = useCreateRepo(mocks.fetchWorktrees, closeModal, mocks.onGitRepoReady)
    await result.handleCreate()

    expect(mocks.createRepo).toHaveBeenCalledWith({
      parentPath: '/projects',
      name: 'created',
      kind: 'git'
    })
    expect(mocks.fetchWorktrees).toHaveBeenCalledWith(repo.id)
    expect(mocks.activateAndRevealWorktree).toHaveBeenCalledWith(worktree.id, {
      sidebarRevealBehavior: 'auto'
    })
    // Second arg is whatever settings useAppStore.getState() has at call
    // time — not this test's concern, only that the checklist item fires.
    expect(mocks.markOnboardingProjectAdded.mock.calls[0]?.[0]).toBe('addedFolder')
    expect(closeModal).toHaveBeenCalled()
    expect(mocks.onGitRepoReady).not.toHaveBeenCalled()
  })

  it('creates projects through the SSH host when an SSH target is selected', async () => {
    const repo = makeRepo({ connectionId: 'ssh-1', path: '/srv/created' })
    mocks.createRemoteRepo.mockResolvedValue({ repo })
    mocks.fetchWorktrees.mockResolvedValue(true)
    const { useCreateRepo } = await import('./useCreateRepo')

    const result = useCreateRepo(mocks.fetchWorktrees, vi.fn(), mocks.onGitRepoReady, {
      sshTargetId: 'ssh-1'
    })
    await result.handleCreate()

    expect(mocks.createRemoteRepo).toHaveBeenCalledWith({
      connectionId: 'ssh-1',
      parentPath: '/projects',
      name: 'created',
      kind: 'git'
    })
    expect(mocks.createRepo).not.toHaveBeenCalled()
    expect(mocks.fetchWorktrees).toHaveBeenCalledWith(repo.id, {
      requireAuthoritative: true
    })
    expect(mocks.onGitRepoReady).toHaveBeenCalledWith(repo.id)
  })

  it('creates projects through the selected runtime environment', async () => {
    const repo = makeRepo({
      id: 'default-project-repo-id',
      executionHostId: 'runtime:env-1',
      path: '/projects/created'
    })
    // Why a two-step create+add mock, not one repo.create resolution: the Go
    // handler only relays repo.create to git-gateway-service (bare repo on
    // disk, no project.repos row) — useCreateRepo now also resolves the
    // implicit default project (project.list) and calls repo.add to register
    // it. All three calls share this one mocked callRuntimeRpc, so branch on
    // method.
    mocks.callRuntimeRpc.mockImplementation(async (_target: unknown, method: string) => {
      if (method === 'project.list') {
        return [
          {
            id: 'default-project',
            name: 'My Repos',
            defaultBranch: 'main',
            devServerId: '',
            visibility: 'private',
            createdAt: 1,
            updatedAt: 1
          }
        ]
      }
      if (method === 'repo.create') {
        return { path: '/projects/created', defaultBranch: 'main' }
      }
      if (method === 'repo.add') {
        return {
          id: repo.id,
          projectId: 'default-project',
          url: '/projects/created',
          displayName: 'created',
          position: 0
        }
      }
      throw new Error(`Unexpected runtime method ${method}`)
    })
    mocks.fetchWorktrees.mockResolvedValue(true)
    const { useCreateRepo } = await import('./useCreateRepo')

    const result = useCreateRepo(mocks.fetchWorktrees, vi.fn(), mocks.onGitRepoReady, {
      hostId: 'runtime:env-1',
      runtimeEnvironmentId: 'env-1',
      devServerId: 'ds-1'
    })
    await result.handleCreate()

    // Why devServerId: 'ds-1' here, not undefined: this option is the
    // dialog's own Host selector — see useCreateRepo.ts's doc comment for
    // the live-verified bug this regression-guards (the dialog's picked
    // dev server never reached repo.create, which requires one).
    expect(mocks.callRuntimeRpc).toHaveBeenCalledWith(
      { kind: 'environment', environmentId: 'env-1' },
      'repo.create',
      {
        devServerId: 'ds-1',
        destPath: '/projects/created',
        defaultBranch: ''
      },
      { timeoutMs: 60_000 }
    )
    expect(mocks.callRuntimeRpc).toHaveBeenCalledWith(
      { kind: 'environment', environmentId: 'env-1' },
      'repo.add',
      { projectId: 'default-project', url: '/projects/created', displayName: 'created' },
      { timeoutMs: 15_000 }
    )
    expect(mocks.createRepo).not.toHaveBeenCalled()
    expect(mocks.createRemoteRepo).not.toHaveBeenCalled()
    expect(mocks.fetchWorktrees).toHaveBeenCalledWith(repo.id, {
      requireAuthoritative: true
    })
    expect(mocks.onGitRepoReady).toHaveBeenCalledWith(repo.id)
  })

  // Live-verified regression: a web-mode Dev Server Host selection (the
  // dialog's own dropdown) parses to kind: 'devServer', so
  // options.runtimeEnvironmentId (only set for kind: 'runtime') stays
  // empty — unlike the "selected runtime environment" test above, which
  // exercises the OTHER, already-working case. Before this fix, target
  // always resolved to 'local' here (activeRuntimeEnvironmentId was
  // unconditionally nulled out), so the whole create silently fell
  // through to window.api.repos.create (the Electron-only local IPC
  // path) instead of ever reaching repo.create with the picked dev
  // server. Confirmed live: window.api.repos.create's web implementation
  // still relays repo.create, but with the wrong param shape entirely,
  // producing GITGATEWAY_MISSING_DEV_SERVER_ID.
  it('creates projects through a picked dev server with no explicit runtime environment', async () => {
    const repo = makeRepo({
      id: 'default-project-repo-id',
      executionHostId: 'runtime:env-1',
      path: '/projects/created'
    })
    mocks.storeState.settings.activeRuntimeEnvironmentId = 'session-auth'
    mocks.callRuntimeRpc.mockImplementation(async (_target: unknown, method: string) => {
      if (method === 'project.list') {
        return [
          {
            id: 'default-project',
            name: 'My Repos',
            defaultBranch: 'main',
            devServerId: '',
            visibility: 'private',
            createdAt: 1,
            updatedAt: 1
          }
        ]
      }
      if (method === 'repo.create') {
        return { path: '/projects/created', defaultBranch: 'main' }
      }
      if (method === 'repo.add') {
        return {
          id: repo.id,
          projectId: 'default-project',
          url: '/projects/created',
          displayName: 'created',
          position: 0
        }
      }
      throw new Error(`Unexpected runtime method ${method}`)
    })
    mocks.fetchWorktrees.mockResolvedValue(true)
    const { useCreateRepo } = await import('./useCreateRepo')

    // No hostId/runtimeEnvironmentId — only devServerId, matching exactly
    // what AddRepoDialog.tsx passes for a devServer-kind Host selection.
    const result = useCreateRepo(mocks.fetchWorktrees, vi.fn(), mocks.onGitRepoReady, {
      devServerId: 'ds-1'
    })
    await result.handleCreate()

    expect(mocks.callRuntimeRpc).toHaveBeenCalledWith(
      { kind: 'environment', environmentId: 'session-auth' },
      'repo.create',
      {
        devServerId: 'ds-1',
        destPath: '/projects/created',
        defaultBranch: ''
      },
      { timeoutMs: 60_000 }
    )
    expect(mocks.createRepo).not.toHaveBeenCalled()
    expect(mocks.onGitRepoReady).toHaveBeenCalledWith(repo.id)
  })
})
