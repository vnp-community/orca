import { beforeEach, describe, expect, it, vi } from 'vitest'
import type * as ReactModule from 'react'
import type { Repo } from '../../../../shared/types'

const mocks = vi.hoisted(() => ({
  stateValues: [] as unknown[],
  stateSetters: [] as ReturnType<typeof vi.fn>[],
  stateIndex: 0,
  refValues: [] as unknown[],
  refIndex: 0,
  storeState: {
    settings: { activeRuntimeEnvironmentId: null as string | null },
    repos: [] as Repo[],
    projects: [],
    projectHostSetups: []
  },
  cloneRemote: vi.fn(),
  cloneLocal: vi.fn(),
  pickDirectory: vi.fn(),
  onCloneProgress: vi.fn(() => vi.fn()),
  callRuntimeRpc: vi.fn(),
  fetchWorktrees: vi.fn(),
  onGitRepoReady: vi.fn()
}))

vi.mock('react', async (importOriginal) => {
  const actual = await importOriginal<typeof ReactModule>()
  return {
    ...actual,
    useCallback: <T extends (...args: never[]) => unknown>(fn: T) => fn,
    useEffect: (effect: () => void | (() => void)) => {
      effect()
    },
    useRef: <T>(value: T) => {
      const index = mocks.refIndex++
      return {
        current: index in mocks.refValues ? (mocks.refValues[index] as T) : value
      }
    },
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

vi.mock('@/runtime/runtime-rpc-client', () => ({
  // Mirrors the real getActiveRuntimeTarget: 'local' only when
  // activeRuntimeEnvironmentId is falsy — see useCreateRepo's identical
  // test-file comment for why this must actually respect the input.
  getActiveRuntimeTarget: (settings: { activeRuntimeEnvironmentId?: string | null }) =>
    settings?.activeRuntimeEnvironmentId
      ? { kind: 'environment', environmentId: settings.activeRuntimeEnvironmentId }
      : { kind: 'local' },
  callRuntimeRpc: mocks.callRuntimeRpc
}))

vi.mock('sonner', () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn()
  }
}))

function makeRepo(overrides: Partial<Repo> = {}): Repo {
  return {
    id: 'repo-cloned',
    path: '/srv/orca',
    displayName: 'orca',
    badgeColor: '#999999',
    addedAt: 1,
    kind: 'git',
    ...overrides
  }
}

// Add Project registers a clone in its OWN project (SOL-FE-E2E-005): project.create carries the
// path and host, and repo.list reads the repo back. `existing` are projects the caller already has.
function ownProjectRpc(
  repoId: string,
  options: { existing?: { id: string; name: string; devServerId: string }[]; hostId: string }
): (target: unknown, method: string, params?: unknown) => Promise<unknown> {
  return async (_target, method, params) => {
    if (method === 'project.list') {
      return options.existing ?? []
    }
    if (method === 'repo.clone') {
      return { worktreePath: '/srv', defaultBranch: 'main' }
    }
    if (method === 'project.create') {
      return { id: 'own-project' }
    }
    if (method === 'repo.list') {
      return (params as { projectId: string }).projectId === 'own-project'
        ? {
            repos: [
              {
                id: repoId,
                projectId: 'own-project',
                url: '/srv',
                displayName: 'orca',
                position: 0,
                devServerId: options.hostId
              }
            ]
          }
        : { repos: [] }
    }
    throw new Error(`Unexpected runtime method ${method}`)
  }
}

describe('useAddRepoCloneFlow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.stateIndex = 0
    mocks.stateSetters = []
    mocks.refIndex = 0
    mocks.refValues = []
    mocks.stateValues = ['https://github.com/stablyai/orca.git', '/srv', false, null, null]
    mocks.storeState.repos = []
    mocks.storeState.projects = []
    mocks.storeState.projectHostSetups = []
    vi.stubGlobal('window', {
      api: {
        repos: {
          cloneRemote: mocks.cloneRemote,
          clone: mocks.cloneLocal,
          pickDirectory: mocks.pickDirectory,
          onCloneProgress: mocks.onCloneProgress
        }
      }
    })
  })

  it('clones through the selected SSH target', async () => {
    const repo = makeRepo({ connectionId: 'ssh-1' })
    mocks.cloneRemote.mockResolvedValue(repo)
    mocks.callRuntimeRpc.mockReset()
    mocks.fetchWorktrees.mockResolvedValue(true)
    const { useAddRepoCloneFlow } = await import('./useAddRepoCloneFlow')

    const result = useAddRepoCloneFlow({
      step: 'clone',
      activeRuntimeEnvironmentId: null,
      sshTargetId: 'ssh-1',
      workspaceDir: '/local/workspace',
      fetchWorktrees: mocks.fetchWorktrees,
      onGitRepoReady: mocks.onGitRepoReady
    })
    await result.handleClone()

    expect(mocks.cloneRemote).toHaveBeenCalledWith({
      connectionId: 'ssh-1',
      url: 'https://github.com/stablyai/orca.git',
      destination: '/srv'
    })
    expect(mocks.cloneLocal).not.toHaveBeenCalled()
    expect(mocks.fetchWorktrees).toHaveBeenCalledWith(repo.id, {
      requireAuthoritative: true
    })
    expect(mocks.storeState.projects).toEqual(
      expect.arrayContaining([expect.objectContaining({ sourceRepoIds: [repo.id] })])
    )
    expect(mocks.storeState.projectHostSetups).toEqual(
      expect.arrayContaining([expect.objectContaining({ repoId: repo.id, path: repo.path })])
    )
    expect(mocks.onGitRepoReady).toHaveBeenCalledWith(repo.id, 'clone_url')
  })

  it('does not prefill SSH clone destinations from the local workspace directory', async () => {
    mocks.stateValues = ['https://github.com/stablyai/orca.git', '', false, null, null]
    const { useAddRepoCloneFlow } = await import('./useAddRepoCloneFlow')

    const result = useAddRepoCloneFlow({
      step: 'clone',
      activeRuntimeEnvironmentId: null,
      sshTargetId: 'ssh-1',
      workspaceDir: '/private/tmp/orca-setup-e2e.hOWO1f',
      fetchWorktrees: mocks.fetchWorktrees,
      onGitRepoReady: mocks.onGitRepoReady
    })

    expect(result.cloneDestination).toBe('')
    expect(mocks.stateSetters[1]).not.toHaveBeenCalledWith('/private/tmp/orca-setup-e2e.hOWO1f')
  })

  it('strips Electron IPC wrappers from clone errors', async () => {
    const cloneError =
      'Clone failed: Destination already exists and is not empty: /srv/orca. Choose a different parent folder, delete the existing folder, or add the existing repository instead.'
    mocks.cloneRemote.mockRejectedValue(
      new Error(`Error invoking remote method 'repos:cloneRemote': Error: ${cloneError}`)
    )
    const { useAddRepoCloneFlow } = await import('./useAddRepoCloneFlow')

    const result = useAddRepoCloneFlow({
      step: 'clone',
      activeRuntimeEnvironmentId: null,
      sshTargetId: 'ssh-1',
      workspaceDir: '/local/workspace',
      fetchWorktrees: mocks.fetchWorktrees,
      onGitRepoReady: mocks.onGitRepoReady
    })
    await result.handleClone()

    expect(mocks.stateSetters[3]).toHaveBeenCalledWith(cloneError)
  })

  it('clones through the selected runtime environment', async () => {
    const repo = makeRepo({ id: 'runtime-repo', executionHostId: 'runtime:env-1' })
    // Why a two-step clone+add mock, not one repo.clone resolution: the Go
    // handler only relays repo.clone to git-gateway-service (disk clone, no
    // project.repos row) — useAddRepoCloneFlow now also resolves the implicit
    // default project (project.list) and calls repo.add to register it. All
    // three calls share this one mocked callRuntimeRpc, so branch on method.
    mocks.callRuntimeRpc.mockImplementation(
      ownProjectRpc(repo.id, {
        existing: [{ id: 'default-project', name: 'My Repos', devServerId: 'ds-1' }],
        hostId: 'ds-1'
      })
    )
    mocks.fetchWorktrees.mockResolvedValue(true)
    const { useAddRepoCloneFlow } = await import('./useAddRepoCloneFlow')

    const result = useAddRepoCloneFlow({
      step: 'clone',
      activeRuntimeEnvironmentId: 'env-1',
      sshTargetId: null,
      devServerId: 'ds-1',
      workspaceDir: '/local/workspace',
      fetchWorktrees: mocks.fetchWorktrees,
      onGitRepoReady: mocks.onGitRepoReady
    })
    await result.handleClone()

    // Why devServerId: 'ds-1' here, not undefined: this option is the
    // dialog's own Host selector — see useAddRepoCloneFlow.ts's doc
    // comment for the live-verified bug this regression-guards (the
    // dialog's picked dev server never reached repo.clone, which
    // requires one).
    expect(mocks.callRuntimeRpc).toHaveBeenCalledWith(
      { kind: 'environment', environmentId: 'env-1' },
      'repo.clone',
      {
        devServerId: 'ds-1',
        url: 'https://github.com/stablyai/orca.git',
        destPath: '/srv'
      },
      { timeoutMs: 10 * 60_000 }
    )
    // Own project, not the host's shared default project; it carries the clone's path and host.
    expect(mocks.callRuntimeRpc).toHaveBeenCalledWith(
      { kind: 'environment', environmentId: 'env-1' },
      'project.create',
      { name: 'orca', devServerId: 'ds-1', repoPath: '/srv', visibility: 'private' },
      { timeoutMs: 30_000 }
    )
    expect(mocks.callRuntimeRpc).not.toHaveBeenCalledWith(
      expect.anything(),
      'repo.add',
      expect.anything(),
      expect.anything()
    )
    expect(mocks.cloneLocal).not.toHaveBeenCalled()
    expect(mocks.cloneRemote).not.toHaveBeenCalled()
    expect(mocks.fetchWorktrees).toHaveBeenCalledWith(repo.id, {
      requireAuthoritative: true
    })
    expect(mocks.onGitRepoReady).toHaveBeenCalledWith(repo.id, 'clone_url')
  })

  // Live-verified regression: see useCreateRepo's identical test for the
  // full explanation — a web-mode Dev Server Host selection has no
  // activeRuntimeEnvironmentId of its own, so before this fix target
  // always resolved to 'local' (the global one was unconditionally
  // nulled out) and the clone silently fell through to
  // window.api.repos.clone (the Electron-only local IPC path) instead of
  // ever reaching repo.clone with the picked dev server.
  it('clones through a picked dev server with no explicit runtime environment', async () => {
    const repo = makeRepo({ id: 'runtime-repo', executionHostId: 'runtime:env-1' })
    mocks.storeState.settings.activeRuntimeEnvironmentId = 'session-auth'
    mocks.callRuntimeRpc.mockImplementation(ownProjectRpc(repo.id, { hostId: 'ds-1' }))
    mocks.fetchWorktrees.mockResolvedValue(true)
    const { useAddRepoCloneFlow } = await import('./useAddRepoCloneFlow')

    // No activeRuntimeEnvironmentId option — only devServerId, matching
    // exactly what AddRepoDialog.tsx passes for a devServer-kind Host
    // selection.
    const result = useAddRepoCloneFlow({
      step: 'clone',
      activeRuntimeEnvironmentId: null,
      sshTargetId: null,
      devServerId: 'ds-1',
      workspaceDir: '/local/workspace',
      fetchWorktrees: mocks.fetchWorktrees,
      onGitRepoReady: mocks.onGitRepoReady
    })
    await result.handleClone()

    expect(mocks.callRuntimeRpc).toHaveBeenCalledWith(
      { kind: 'environment', environmentId: 'session-auth' },
      'repo.clone',
      {
        devServerId: 'ds-1',
        url: 'https://github.com/stablyai/orca.git',
        destPath: '/srv'
      },
      { timeoutMs: 10 * 60_000 }
    )
    expect(mocks.cloneLocal).not.toHaveBeenCalled()
    expect(mocks.cloneRemote).not.toHaveBeenCalled()
    expect(mocks.onGitRepoReady).toHaveBeenCalledWith(repo.id, 'clone_url')
  })

  it('registers the clone in its own project even when the host already has one', async () => {
    const repo = makeRepo({ id: 'runtime-repo', executionHostId: 'runtime:env-1' })
    mocks.callRuntimeRpc.mockImplementation(
      ownProjectRpc(repo.id, {
        // A project on the same host must NOT be reused: Requests run on a project's first repo.
        existing: [{ id: 'host-project', name: 'Mac Studio', devServerId: 'ds-2' }],
        hostId: 'ds-2'
      })
    )
    mocks.fetchWorktrees.mockResolvedValue(true)
    const { useAddRepoCloneFlow } = await import('./useAddRepoCloneFlow')

    const result = useAddRepoCloneFlow({
      step: 'clone',
      activeRuntimeEnvironmentId: 'env-1',
      sshTargetId: null,
      devServerId: 'ds-2',
      workspaceDir: '/local/workspace',
      fetchWorktrees: mocks.fetchWorktrees,
      onGitRepoReady: mocks.onGitRepoReady
    })
    await result.handleClone()

    expect(mocks.callRuntimeRpc).toHaveBeenCalledWith(
      { kind: 'environment', environmentId: 'env-1' },
      'project.create',
      { name: 'orca', devServerId: 'ds-2', repoPath: '/srv', visibility: 'private' },
      { timeoutMs: 30_000 }
    )
    expect(mocks.onGitRepoReady).toHaveBeenCalledWith(repo.id, 'clone_url')
  })

  it('stops with the project name when the cloned folder is already registered', async () => {
    mocks.callRuntimeRpc.mockImplementation(async (_target: unknown, method: string) => {
      if (method === 'repo.clone') {
        return { worktreePath: '/srv', defaultBranch: 'main' }
      }
      if (method === 'project.list') {
        return [{ id: 'old-project', name: 'Old project', devServerId: 'ds-2' }]
      }
      if (method === 'repo.list') {
        return {
          repos: [{ id: 'r-old', projectId: 'old-project', url: '/srv/', devServerId: 'ds-2' }]
        }
      }
      throw new Error(`Unexpected runtime method ${method}`)
    })
    const { useAddRepoCloneFlow } = await import('./useAddRepoCloneFlow')

    const result = useAddRepoCloneFlow({
      step: 'clone',
      activeRuntimeEnvironmentId: 'env-1',
      sshTargetId: null,
      devServerId: 'ds-2',
      workspaceDir: '/local/workspace',
      fetchWorktrees: mocks.fetchWorktrees,
      onGitRepoReady: mocks.onGitRepoReady
    })
    await result.handleClone()

    expect(mocks.callRuntimeRpc).not.toHaveBeenCalledWith(
      expect.anything(),
      'project.create',
      expect.anything(),
      expect.anything()
    )
    expect(mocks.onGitRepoReady).not.toHaveBeenCalled()
    expect(mocks.stateSetters[3]).toHaveBeenCalledWith(expect.stringContaining('Old project'))
  })
})
