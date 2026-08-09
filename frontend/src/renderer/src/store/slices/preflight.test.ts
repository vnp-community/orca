import { describe, expect, it, vi } from 'vitest'
import { create } from 'zustand'
import type { PreflightStatus } from '../../../../preload/api-types'
import type { Repo, Worktree } from '../../../../shared/types'
import type { AppState } from '../types'
import { createPreflightSlice } from './preflight'

const preflightCheck = vi.fn()
const callRuntimeRpc = vi.fn()
const platformGet = vi.fn(() => ({ platform: 'linux' }))

vi.mock('@/runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: (...args: unknown[]) => callRuntimeRpc(...args),
  getActiveRuntimeTarget: (
    settings?: { activeRuntimeEnvironmentId?: string | null } | null
  ): { kind: 'local' } | { kind: 'environment'; environmentId: string } => {
    const environmentId = settings?.activeRuntimeEnvironmentId?.trim()
    return environmentId ? { kind: 'environment', environmentId } : { kind: 'local' }
  }
}))

// TASK-FE-014.1: mock spans expose id/step/ok/fail so tests can assert both
// the RPC `traceId` forwarding AND the span lifecycle calls (start/step/ok/fail fields).
const { preflightSpans, uiRemoteIntegrationPreflightFlowStart } = vi.hoisted(() => {
  const preflightSpans: { id: string; step: ReturnType<typeof vi.fn>; ok: ReturnType<typeof vi.fn>; fail: ReturnType<typeof vi.fn> }[] = []
  const uiRemoteIntegrationPreflightFlowStart = vi.fn(() => {
    const span = {
      id: `preflight-span-${preflightSpans.length}`,
      step: vi.fn(),
      ok: vi.fn(),
      fail: vi.fn()
    }
    preflightSpans.push(span)
    return span
  })
  return { preflightSpans, uiRemoteIntegrationPreflightFlowStart }
})

vi.mock('../../../../shared/trace/tracers', () => ({
  Tracers: {
    uiRemoteIntegrationPreflightFlow: { start: uiRemoteIntegrationPreflightFlowStart }
  }
}))

globalThis.window = {
  api: {
    preflight: {
      check: preflightCheck,
      detectAgents: vi.fn().mockResolvedValue([]),
      refreshAgents: vi.fn().mockResolvedValue({
        agents: [],
        addedPathSegments: [],
        shellHydrationOk: false,
        pathSource: 'sync_seed_only',
        pathFailureReason: 'spawn_error'
      }),
      detectRemoteAgents: vi.fn().mockResolvedValue([])
    },
    platform: {
      get: platformGet
    }
  } as unknown as Window['api']
} as Window & typeof globalThis

function createTestStore() {
  return create<AppState>()(
    (...a) =>
      ({
        ...createPreflightSlice(...a)
      }) as AppState
  )
}

function resetPreflightMocks(): void {
  preflightCheck.mockReset()
  callRuntimeRpc.mockReset()
  platformGet.mockReset().mockReturnValue({ platform: 'linux' })
  preflightSpans.length = 0
  uiRemoteIntegrationPreflightFlowStart.mockClear()
}

function makeStatus(glabInstalled: boolean): PreflightStatus {
  return {
    git: { installed: true },
    gh: { installed: true, authenticated: true },
    glab: { installed: glabInstalled, authenticated: glabInstalled }
  }
}

function makeRepo(overrides: Partial<Repo> & { id: string; path: string }): Repo {
  return {
    displayName: 'Repo',
    badgeColor: '#000000',
    addedAt: 0,
    ...overrides
  }
}

function makeWorktree(
  overrides: Partial<Worktree> & { id: string; repoId: string; path: string }
): Worktree {
  return {
    head: 'abc123',
    branch: 'refs/heads/main',
    isBare: false,
    isMainWorktree: false,
    displayName: 'main',
    comment: '',
    linkedIssue: null,
    linkedPR: null,
    linkedLinearIssue: null,
    linkedGitLabMR: null,
    linkedGitLabIssue: null,
    isArchived: false,
    isUnread: false,
    isPinned: false,
    sortOrder: 0,
    lastActivityAt: 0,
    ...overrides
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

describe('createPreflightSlice', () => {
  it('dedupes concurrent non-forced checks', async () => {
    resetPreflightMocks()
    const pending = deferred<PreflightStatus>()
    preflightCheck.mockReturnValueOnce(pending.promise)
    const store = createTestStore()

    const first = store.getState().refreshPreflightStatus()
    const second = store.getState().refreshPreflightStatus()

    expect(preflightCheck).toHaveBeenCalledTimes(1)
    pending.resolve(makeStatus(true))
    await Promise.all([first, second])

    expect(store.getState().preflightStatus?.glab?.installed).toBe(true)
    expect(store.getState().preflightStatusChecked).toBe(true)
    expect(store.getState().preflightStatusLoading).toBe(false)
  })

  it('lets forced checks bypass non-forced dedupe and win stale races', async () => {
    resetPreflightMocks()
    const stale = deferred<PreflightStatus>()
    const fresh = deferred<PreflightStatus>()
    preflightCheck.mockReturnValueOnce(stale.promise).mockReturnValueOnce(fresh.promise)
    const store = createTestStore()

    const normal = store.getState().refreshPreflightStatus()
    const forced = store.getState().refreshPreflightStatus({ force: true })

    expect(preflightCheck).toHaveBeenNthCalledWith(1, undefined)
    expect(preflightCheck).toHaveBeenNthCalledWith(2, { force: true })

    fresh.resolve(makeStatus(true))
    await forced
    stale.resolve(makeStatus(false))
    await normal

    expect(store.getState().preflightStatus?.glab?.installed).toBe(true)
  })

  it('dedupes lazy checks onto an in-flight forced refresh', async () => {
    resetPreflightMocks()
    const fresh = deferred<PreflightStatus>()
    preflightCheck.mockReturnValueOnce(fresh.promise)
    const store = createTestStore()

    const forced = store.getState().refreshPreflightStatus({ force: true })
    const lazy = store.getState().refreshPreflightStatus()

    expect(preflightCheck).toHaveBeenCalledTimes(1)
    fresh.resolve(makeStatus(true))
    await Promise.all([forced, lazy])

    expect(store.getState().preflightStatus?.glab?.installed).toBe(true)
  })

  it('checks integrations inside the active WSL worktree distro', async () => {
    resetPreflightMocks()
    preflightCheck.mockResolvedValueOnce(makeStatus(true))
    const store = createTestStore()
    store.setState({
      repos: [
        makeRepo({
          id: 'repo-1',
          path: 'C:\\repo'
        })
      ],
      worktreesByRepo: {
        'repo-1': [
          makeWorktree({
            id: 'wt-1',
            repoId: 'repo-1',
            path: '\\\\wsl.localhost\\Ubuntu\\home\\alice\\repo'
          })
        ]
      },
      activeRepoId: 'repo-1',
      activeWorktreeId: 'wt-1'
    } as Partial<AppState>)

    await store.getState().refreshPreflightStatus()

    expect(preflightCheck).toHaveBeenCalledWith({ wslDistro: 'Ubuntu' })
  })

  it('checks integrations through the resolved project runtime on Windows', async () => {
    resetPreflightMocks()
    platformGet.mockReturnValue({ platform: 'win32' })
    preflightCheck.mockResolvedValueOnce(makeStatus(true))
    const store = createTestStore()
    store.setState({
      repos: [
        makeRepo({
          id: 'repo-1',
          path: 'C:\\repo'
        })
      ],
      worktreesByRepo: {
        'repo-1': [
          makeWorktree({
            id: 'wt-1',
            repoId: 'repo-1',
            path: '\\\\wsl.localhost\\Ubuntu\\home\\alice\\repo'
          })
        ]
      },
      activeRepoId: 'repo-1',
      activeWorktreeId: 'wt-1'
    } as Partial<AppState>)

    await store.getState().refreshPreflightStatus()

    expect(preflightCheck).toHaveBeenCalledWith({
      projectRuntime: {
        status: 'resolved',
        runtime: {
          kind: 'wsl',
          hostPlatform: 'wsl',
          projectId: 'repo-1',
          distro: 'Ubuntu',
          reason: 'project-override',
          cacheKey: 'repo-1:wsl:Ubuntu'
        }
      },
      wslDistro: 'Ubuntu'
    })
  })

  it('passes repair-required project runtime context through Windows preflight errors', async () => {
    resetPreflightMocks()
    platformGet.mockReturnValue({ platform: 'win32' })
    preflightCheck.mockRejectedValueOnce(
      new Error('Project runtime requires repair before preflight: wsl-distro-required')
    )
    const store = createTestStore()
    store.setState({
      settings: {
        localWindowsRuntimeDefault: { kind: 'wsl', distro: null }
      },
      repos: [makeRepo({ id: 'repo-1', path: 'C:\\repo' })],
      worktreesByRepo: {},
      activeRepoId: 'repo-1',
      activeWorktreeId: null
    } as Partial<AppState>)

    await store.getState().refreshPreflightStatus()

    expect(preflightCheck).toHaveBeenCalledWith({
      projectRuntime: {
        status: 'repair-required',
        repair: {
          projectId: 'repo-1',
          preferredRuntime: { kind: 'wsl', distro: null },
          reason: 'wsl-distro-required',
          source: 'global-default',
          cacheKey: 'repo-1:repair:wsl-distro-required:default'
        }
      }
    })
    expect(store.getState().preflightStatusError).toBe(
      'Project runtime requires repair before preflight: wsl-distro-required'
    )
  })

  it('keeps preflight request dedupe scoped by WSL distro context', async () => {
    resetPreflightMocks()
    const ubuntu = deferred<PreflightStatus>()
    const debian = deferred<PreflightStatus>()
    preflightCheck.mockReturnValueOnce(ubuntu.promise).mockReturnValueOnce(debian.promise)
    const store = createTestStore()
    store.setState({
      repos: [makeRepo({ id: 'repo-1', path: '\\\\wsl.localhost\\Ubuntu\\home\\alice\\repo' })],
      worktreesByRepo: {},
      activeRepoId: 'repo-1',
      activeWorktreeId: null
    } as Partial<AppState>)

    const first = store.getState().refreshPreflightStatus()
    store.setState({
      repos: [makeRepo({ id: 'repo-1', path: '\\\\wsl.localhost\\Debian\\home\\alice\\repo' })]
    } as Partial<AppState>)
    const second = store.getState().refreshPreflightStatus()

    expect(preflightCheck).toHaveBeenNthCalledWith(1, { wslDistro: 'Ubuntu' })
    expect(preflightCheck).toHaveBeenNthCalledWith(2, { wslDistro: 'Debian' })
    ubuntu.resolve(makeStatus(false))
    debian.resolve(makeStatus(true))
    await Promise.all([first, second])
    expect(store.getState().preflightStatus?.glab?.installed).toBe(true)
  })

  it('checks integrations through the active runtime environment', async () => {
    resetPreflightMocks()
    const firstRuntime = deferred<PreflightStatus>()
    const secondRuntime = deferred<PreflightStatus>()
    callRuntimeRpc
      .mockReturnValueOnce(firstRuntime.promise)
      .mockReturnValueOnce(secondRuntime.promise)
    const store = createTestStore()
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'runtime-1' }
    } as Partial<AppState>)

    const first = store.getState().refreshPreflightStatus()
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'runtime-2' }
    } as Partial<AppState>)
    const second = store.getState().refreshPreflightStatus()

    expect(preflightCheck).not.toHaveBeenCalled()
    expect(callRuntimeRpc).toHaveBeenNthCalledWith(
      1,
      { kind: 'environment', environmentId: 'runtime-1' },
      'preflight.check',
      { traceId: preflightSpans[0].id }
    )
    expect(callRuntimeRpc).toHaveBeenNthCalledWith(
      2,
      { kind: 'environment', environmentId: 'runtime-2' },
      'preflight.check',
      { traceId: preflightSpans[1].id }
    )
    firstRuntime.resolve(makeStatus(false))
    secondRuntime.resolve(makeStatus(true))
    await Promise.all([first, second])
    expect(store.getState().preflightStatusContextKey).toBe('runtime:runtime-2#0')
    expect(store.getState().preflightStatus?.glab?.installed).toBe(true)
  })

  it('clears checked status immediately when refreshing a different local context', async () => {
    resetPreflightMocks()
    const host = deferred<PreflightStatus>()
    const wsl = deferred<PreflightStatus>()
    preflightCheck.mockReturnValueOnce(host.promise).mockReturnValueOnce(wsl.promise)
    const store = createTestStore()

    const first = store.getState().refreshPreflightStatus()
    host.resolve(makeStatus(true))
    await first
    expect(store.getState().preflightStatusChecked).toBe(true)

    store.setState({
      repos: [makeRepo({ id: 'repo-1', path: '\\\\wsl.localhost\\Ubuntu\\home\\alice\\repo' })],
      activeRepoId: 'repo-1',
      activeWorktreeId: null
    } as Partial<AppState>)
    const second = store.getState().refreshPreflightStatus()

    expect(store.getState().preflightStatus).toBeNull()
    expect(store.getState().preflightStatusChecked).toBe(false)
    expect(store.getState().preflightStatusLoading).toBe(true)

    wsl.resolve(makeStatus(false))
    await second
    expect(store.getState().preflightStatus?.glab?.installed).toBe(false)
  })

  // --- TASK-FE-014.1: ui:remoteIntegration.preflight tracer coverage ---

  it('starts a ui:remoteIntegration.preflight span with { force, mode } before calling window.api.preflight.check', async () => {
    resetPreflightMocks()
    preflightCheck.mockResolvedValueOnce(makeStatus(true))
    const store = createTestStore()

    await store.getState().refreshPreflightStatus()

    expect(uiRemoteIntegrationPreflightFlowStart).toHaveBeenCalledWith({ force: false, mode: 'local' })
    expect(uiRemoteIntegrationPreflightFlowStart.mock.invocationCallOrder[0]).toBeLessThan(
      preflightCheck.mock.invocationCallOrder[0]
    )
  })

  it('starts a span with { force: true, mode } before calling callRuntimeRpc when force refreshing a runtime environment', async () => {
    resetPreflightMocks()
    callRuntimeRpc.mockResolvedValueOnce(makeStatus(true))
    const store = createTestStore()
    store.setState({ settings: { activeRuntimeEnvironmentId: 'runtime-1' } } as Partial<AppState>)

    await store.getState().refreshPreflightStatus({ force: true })

    expect(uiRemoteIntegrationPreflightFlowStart).toHaveBeenCalledWith({
      force: true,
      mode: 'environment'
    })
    expect(uiRemoteIntegrationPreflightFlowStart.mock.invocationCallOrder[0]).toBeLessThan(
      callRuntimeRpc.mock.invocationCallOrder[0]
    )
  })

  it('emits a relayDelegate step before delegating to callRuntimeRpc', async () => {
    resetPreflightMocks()
    callRuntimeRpc.mockResolvedValueOnce(makeStatus(true))
    const store = createTestStore()
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'runtime-1' },
      activeDevServerId: 'dev-server-1'
    } as Partial<AppState>)

    await store.getState().refreshPreflightStatus()

    const span = preflightSpans[0]
    expect(span.step).toHaveBeenCalledWith('relayDelegate', { devServerId: 'dev-server-1' })
    expect(span.step.mock.invocationCallOrder[0]).toBeLessThan(callRuntimeRpc.mock.invocationCallOrder[0])
  })

  it('forwards traceId: span.id in callRuntimeRpc params only when runtimeTarget.kind is environment', async () => {
    resetPreflightMocks()
    callRuntimeRpc.mockResolvedValueOnce(makeStatus(true))
    const store = createTestStore()
    store.setState({ settings: { activeRuntimeEnvironmentId: 'runtime-1' } } as Partial<AppState>)

    await store.getState().refreshPreflightStatus()

    expect(callRuntimeRpc).toHaveBeenCalledWith(
      { kind: 'environment', environmentId: 'runtime-1' },
      'preflight.check',
      expect.objectContaining({ traceId: preflightSpans[0].id })
    )
  })

  it('does NOT include traceId in window.api.preflight.check args when runtimeTarget.kind is local', async () => {
    resetPreflightMocks()
    preflightCheck.mockResolvedValueOnce(makeStatus(true))
    const store = createTestStore()

    await store.getState().refreshPreflightStatus()

    expect(preflightCheck).toHaveBeenCalledWith(undefined)
  })

  it('does NOT start a new span for a request coalesced by the non-forced dedupe guard', async () => {
    resetPreflightMocks()
    const pending = deferred<PreflightStatus>()
    preflightCheck.mockReturnValueOnce(pending.promise)
    const store = createTestStore()

    const first = store.getState().refreshPreflightStatus()
    const second = store.getState().refreshPreflightStatus()

    expect(uiRemoteIntegrationPreflightFlowStart).toHaveBeenCalledTimes(1)
    pending.resolve(makeStatus(true))
    await Promise.all([first, second])
    expect(uiRemoteIntegrationPreflightFlowStart).toHaveBeenCalledTimes(1)
  })

  it('does NOT start a new span for a lazy request coalesced onto an in-flight forced refresh', async () => {
    resetPreflightMocks()
    const fresh = deferred<PreflightStatus>()
    preflightCheck.mockReturnValueOnce(fresh.promise)
    const store = createTestStore()

    const forced = store.getState().refreshPreflightStatus({ force: true })
    const lazy = store.getState().refreshPreflightStatus()

    expect(uiRemoteIntegrationPreflightFlowStart).toHaveBeenCalledTimes(1)
    fresh.resolve(makeStatus(true))
    await Promise.all([forced, lazy])
    expect(uiRemoteIntegrationPreflightFlowStart).toHaveBeenCalledTimes(1)
  })

  it('marks the span ok with ghAuthenticated/glabAuthenticated on success', async () => {
    resetPreflightMocks()
    preflightCheck.mockResolvedValueOnce(makeStatus(true))
    const store = createTestStore()

    await store.getState().refreshPreflightStatus()

    expect(preflightSpans[0].ok).toHaveBeenCalledWith({ ghAuthenticated: true, glabAuthenticated: true })
  })

  it('marks the span ok({ stale: true }) and does not overwrite state for a superseded response', async () => {
    resetPreflightMocks()
    const stale = deferred<PreflightStatus>()
    const fresh = deferred<PreflightStatus>()
    preflightCheck.mockReturnValueOnce(stale.promise).mockReturnValueOnce(fresh.promise)
    const store = createTestStore()

    const normal = store.getState().refreshPreflightStatus()
    const forced = store.getState().refreshPreflightStatus({ force: true })

    fresh.resolve(makeStatus(true))
    await forced
    stale.resolve(makeStatus(false))
    await normal

    expect(preflightSpans[0].ok).toHaveBeenCalledWith({ stale: true })
    expect(store.getState().preflightStatus?.glab?.installed).toBe(true)
  })

  it('marks the span fail(error, { force, mode }) when the request rejects', async () => {
    resetPreflightMocks()
    const error = new Error('boom')
    preflightCheck.mockRejectedValueOnce(error)
    const store = createTestStore()

    await store.getState().refreshPreflightStatus()

    expect(preflightSpans[0].fail).toHaveBeenCalledWith(error, { force: false, mode: 'local' })
  })
})
