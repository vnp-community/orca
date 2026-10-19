import { beforeEach, describe, expect, it, vi } from 'vitest'
import type * as ReactModule from 'react'
import type { Repo } from '../../../../shared/types'

const mocks = vi.hoisted(() => ({
  stateValues: [] as unknown[],
  stateSetters: [] as ReturnType<typeof vi.fn>[],
  stateIndex: 0,
  addRepoPath: vi.fn(),
  closeModal: vi.fn(),
  fetchWorktrees: vi.fn(),
  getNestedRepoRuntimeKind: vi.fn(),
  scanNestedRepos: vi.fn(),
  setActiveNestedScanId: vi.fn(),
  setNestedScanInProgress: vi.fn(),
  showNestedRepoReview: vi.fn(),
  onGitRepoReady: vi.fn(),
  setAddProjectBusyLabel: vi.fn(),
  markOnboardingProjectAdded: vi.fn(),
  track: vi.fn()
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

vi.mock('@/lib/onboarding-project-checklist', () => ({
  markOnboardingProjectAdded: mocks.markOnboardingProjectAdded
}))

// Why this mock is needed now: useAddRepoServerPathFlow's folder-add branch
// reads useAppStore.getState().settings to pass along to
// markOnboardingProjectAdded — without a stub, the real store module
// initializes for real and this test hangs waiting on it.
vi.mock('@/store', () => ({
  useAppStore: {
    getState: () => ({ settings: {} })
  }
}))

vi.mock('@/lib/telemetry', () => ({
  track: mocks.track
}))

function makeRepo(overrides: Partial<Repo> = {}): Repo {
  return {
    id: 'server-folder',
    path: '/server/docs',
    displayName: 'docs',
    badgeColor: '#999999',
    addedAt: 1,
    kind: 'folder',
    ...overrides
  }
}

describe('useAddRepoServerPathFlow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.stateIndex = 0
    mocks.stateSetters = []
    mocks.stateValues = ['/server/docs', false]
  })

  it('marks onboarding folder progress before closing server folder adds', async () => {
    const repo = makeRepo()
    mocks.addRepoPath.mockResolvedValue(repo)
    const { useAddRepoServerPathFlow } = await import('./useAddRepoServerPathFlow')

    const result = useAddRepoServerPathFlow({
      addRepoPath: mocks.addRepoPath,
      closeModal: mocks.closeModal,
      fetchWorktrees: mocks.fetchWorktrees,
      getNestedRepoRuntimeKind: mocks.getNestedRepoRuntimeKind,
      scanNestedRepos: mocks.scanNestedRepos,
      setActiveNestedScanId: mocks.setActiveNestedScanId,
      setNestedScanInProgress: mocks.setNestedScanInProgress,
      showNestedRepoReview: mocks.showNestedRepoReview,
      onGitRepoReady: mocks.onGitRepoReady,
      setAddProjectBusyLabel: mocks.setAddProjectBusyLabel
    })
    await result.handleAddServerPath('folder')

    expect(mocks.addRepoPath).toHaveBeenCalledWith('/server/docs', 'folder')
    expect(mocks.scanNestedRepos).not.toHaveBeenCalled()
    expect(mocks.fetchWorktrees).not.toHaveBeenCalled()
    expect(mocks.onGitRepoReady).not.toHaveBeenCalled()
    // Second arg is whatever settings useAppStore.getState() has at call
    // time — not this test's concern, only that the checklist item fires.
    expect(mocks.markOnboardingProjectAdded.mock.calls[0]?.[0]).toBe('addedFolder')
    expect(mocks.closeModal).toHaveBeenCalled()
  })

  it('registers the project on the dialog host and scans that host for nested repos', async () => {
    const repo = makeRepo({ kind: 'git' })
    mocks.addRepoPath.mockResolvedValue(repo)
    mocks.scanNestedRepos.mockResolvedValue({ selectedPathKind: 'git_repo', repos: [] })
    mocks.getNestedRepoRuntimeKind.mockReturnValue('runtime')
    const { useAddRepoServerPathFlow } = await import('./useAddRepoServerPathFlow')

    const result = useAddRepoServerPathFlow({
      devServerId: 'ds-1',
      addRepoPath: mocks.addRepoPath,
      closeModal: mocks.closeModal,
      fetchWorktrees: mocks.fetchWorktrees,
      getNestedRepoRuntimeKind: mocks.getNestedRepoRuntimeKind,
      scanNestedRepos: mocks.scanNestedRepos,
      setActiveNestedScanId: mocks.setActiveNestedScanId,
      setNestedScanInProgress: mocks.setNestedScanInProgress,
      showNestedRepoReview: mocks.showNestedRepoReview,
      onGitRepoReady: mocks.onGitRepoReady,
      setAddProjectBusyLabel: mocks.setAddProjectBusyLabel
    })
    await result.handleAddServerPath('git')

    // Why: the host picked in the dialog must reach repo.add, not only the folder browser.
    expect(mocks.scanNestedRepos).toHaveBeenCalledWith(
      '/server/docs',
      undefined,
      expect.objectContaining({ devServerId: 'ds-1' })
    )
    // Why ownProject: Add Project creates a new project named after the folder (SOL-FE-E2E-005).
    expect(mocks.addRepoPath).toHaveBeenCalledWith('/server/docs', 'git', {
      devServerId: 'ds-1',
      ownProject: { name: 'docs' }
    })
  })

  it('sends the project name the user typed', async () => {
    mocks.stateValues = ['/server/docs', false, '  Billing  ']
    mocks.addRepoPath.mockResolvedValue(makeRepo({ kind: 'folder' }))
    const { useAddRepoServerPathFlow } = await import('./useAddRepoServerPathFlow')

    const result = useAddRepoServerPathFlow({
      devServerId: 'ds-1',
      addRepoPath: mocks.addRepoPath,
      closeModal: mocks.closeModal,
      fetchWorktrees: mocks.fetchWorktrees,
      getNestedRepoRuntimeKind: mocks.getNestedRepoRuntimeKind,
      scanNestedRepos: mocks.scanNestedRepos,
      setActiveNestedScanId: mocks.setActiveNestedScanId,
      setNestedScanInProgress: mocks.setNestedScanInProgress,
      showNestedRepoReview: mocks.showNestedRepoReview,
      onGitRepoReady: mocks.onGitRepoReady,
      setAddProjectBusyLabel: mocks.setAddProjectBusyLabel
    })
    await result.handleAddServerPath('folder')

    expect(mocks.addRepoPath).toHaveBeenCalledWith('/server/docs', 'folder', {
      devServerId: 'ds-1',
      ownProject: { name: 'Billing' }
    })
  })

  it('blocks an already registered folder: no scan, no add, and exposes the match', async () => {
    const match = { repoId: 'r-old', projectId: 'p-old', projectName: 'Old project' }
    const findRegisteredRepo = vi.fn().mockResolvedValue(match)
    const { useAddRepoServerPathFlow } = await import('./useAddRepoServerPathFlow')

    const result = useAddRepoServerPathFlow({
      devServerId: 'ds-1',
      findRegisteredRepo,
      addRepoPath: mocks.addRepoPath,
      closeModal: mocks.closeModal,
      fetchWorktrees: mocks.fetchWorktrees,
      getNestedRepoRuntimeKind: mocks.getNestedRepoRuntimeKind,
      scanNestedRepos: mocks.scanNestedRepos,
      setActiveNestedScanId: mocks.setActiveNestedScanId,
      setNestedScanInProgress: mocks.setNestedScanInProgress,
      showNestedRepoReview: mocks.showNestedRepoReview,
      onGitRepoReady: mocks.onGitRepoReady,
      setAddProjectBusyLabel: mocks.setAddProjectBusyLabel
    })
    await result.handleAddServerPath('git')

    expect(findRegisteredRepo).toHaveBeenCalledWith({ devServerId: 'ds-1', path: '/server/docs' })
    expect(mocks.scanNestedRepos).not.toHaveBeenCalled()
    expect(mocks.addRepoPath).not.toHaveBeenCalled()
    expect(mocks.stateSetters[3]).toHaveBeenCalledWith(match)
  })

  it('opens the project that already holds the folder', async () => {
    const match = { repoId: 'r-old', projectId: 'p-old', projectName: 'Old project' }
    mocks.stateValues = ['/server/docs', false, null, match]
    const onOpenRegisteredRepo = vi.fn()
    const { useAddRepoServerPathFlow } = await import('./useAddRepoServerPathFlow')

    const result = useAddRepoServerPathFlow({
      devServerId: 'ds-1',
      onOpenRegisteredRepo,
      addRepoPath: mocks.addRepoPath,
      closeModal: mocks.closeModal,
      fetchWorktrees: mocks.fetchWorktrees,
      getNestedRepoRuntimeKind: mocks.getNestedRepoRuntimeKind,
      scanNestedRepos: mocks.scanNestedRepos,
      setActiveNestedScanId: mocks.setActiveNestedScanId,
      setNestedScanInProgress: mocks.setNestedScanInProgress,
      showNestedRepoReview: mocks.showNestedRepoReview,
      onGitRepoReady: mocks.onGitRepoReady,
      setAddProjectBusyLabel: mocks.setAddProjectBusyLabel
    })
    result.openDuplicate()

    expect(result.duplicate).toEqual(match)
    expect(onOpenRegisteredRepo).toHaveBeenCalledWith(match)
  })
})
