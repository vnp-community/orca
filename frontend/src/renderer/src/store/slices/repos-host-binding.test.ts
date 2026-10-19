// SOL-FE-E2E-004: the host picked for a new project must reach repo.add on every path, and web
// must never register a repo without one (git-gateway has no local fallback for web).
import { describe, expect, it, vi } from 'vitest'
import { toast } from 'sonner'
import { createTestStore } from './store-test-helpers'
import {
  DEFAULT_PROJECT_ID,
  installReposRuntimeRoutingHarness,
  localRepo,
  remoteRepo,
  reposAdd,
  runtimeEnvironmentCall
} from './repos-runtime-routing-fixture'

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), info: vi.fn(), success: vi.fn(), warning: vi.fn() }
}))

installReposRuntimeRoutingHarness()

const rpcOk = (result: unknown): { id: string; ok: true; result: unknown; _meta: unknown } => ({
  id: 'rpc',
  ok: true,
  result,
  _meta: { runtimeId: 'runtime-remote' }
})

function projectListWith(projects: Record<string, unknown>[]): unknown {
  return rpcOk(projects)
}

function stubWebWindow(): void {
  vi.stubGlobal('window', { ...window, __ORCA_WEB_CLIENT__: true })
}

describe('addRepoPath host binding', () => {
  it('sends devServerId on repo.add through a runtime environment, in the host-bound project', async () => {
    runtimeEnvironmentCall.mockImplementation(async (args: { method: string }) => {
      if (args.method === 'project.list') {
        return projectListWith([
          { id: 'legacy', name: 'My Repos', devServerId: 'local-mac', createdAt: 1 },
          { id: 'proj-ds1', name: 'Mac Studio', devServerId: 'ds-1', createdAt: 2 }
        ])
      }
      return rpcOk({
        id: remoteRepo.id,
        projectId: 'proj-ds1',
        url: '/srv/project',
        displayName: 'project',
        devServerId: 'ds-1',
        position: 0
      })
    })
    const store = createTestStore()
    store.setState({ settings: { activeRuntimeEnvironmentId: 'env-1' } as never })

    const repo = await store.getState().addRepoPath('/srv/project', 'git', { devServerId: 'ds-1' })

    expect(repo?.devServerId).toBe('ds-1')
    expect(runtimeEnvironmentCall).toHaveBeenCalledWith({
      selector: 'env-1',
      method: 'repo.add',
      params: {
        projectId: 'proj-ds1',
        url: '/srv/project',
        displayName: 'project',
        devServerId: 'ds-1'
      },
      timeoutMs: 15_000
    })
  })

  it('creates a project bound to the host when the caller has none for it', async () => {
    runtimeEnvironmentCall.mockImplementation(async (args: { method: string }) => {
      if (args.method === 'project.list') {
        return projectListWith([{ id: 'legacy', devServerId: 'local-mac', createdAt: 1 }])
      }
      if (args.method === 'devServer.list') {
        return rpcOk([{ id: 'ds-2', name: 'Build Box' }])
      }
      if (args.method === 'project.create') {
        return rpcOk({ id: 'proj-ds2' })
      }
      return rpcOk({
        id: remoteRepo.id,
        projectId: 'proj-ds2',
        url: '/srv/project',
        displayName: 'project',
        devServerId: 'ds-2',
        position: 0
      })
    })
    const store = createTestStore()
    store.setState({ settings: { activeRuntimeEnvironmentId: 'env-1' } as never })

    await store.getState().addRepoPath('/srv/project', 'git', { devServerId: 'ds-2' })

    expect(runtimeEnvironmentCall).toHaveBeenCalledWith(
      expect.objectContaining({
        method: 'project.create',
        params: { name: 'Build Box', devServerId: 'ds-2', visibility: 'private' }
      })
    )
    expect(runtimeEnvironmentCall).toHaveBeenCalledWith(
      expect.objectContaining({
        method: 'repo.add',
        params: expect.objectContaining({ projectId: 'proj-ds2', devServerId: 'ds-2' })
      })
    )
  })

  it('sends devServerId through the local preload leg (what the web client uses)', async () => {
    stubWebWindow()
    ;(window as unknown as { api: { repos: { add: unknown } } }).api.repos.add = reposAdd
    reposAdd.mockResolvedValue({ repo: { ...localRepo, devServerId: 'ds-1' } })
    const store = createTestStore()

    await store.getState().addRepoPath('/srv/project', 'git', { devServerId: 'ds-1' })

    expect(reposAdd).toHaveBeenCalledWith({
      path: '/srv/project',
      kind: 'git',
      devServerId: 'ds-1'
    })
  })

  it('on web falls back to the globally selected dev server only when the caller gave none', async () => {
    stubWebWindow()
    reposAdd.mockResolvedValue({ repo: localRepo })
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    const store = createTestStore()
    store.setState({ activeDevServerId: 'ds-9' })

    await store.getState().addRepoPath('/srv/project')

    expect(reposAdd).toHaveBeenCalledWith({
      path: '/srv/project',
      kind: 'git',
      devServerId: 'ds-9'
    })
  })

  it('on web refuses to add without any host, before any RPC, with a clear message', async () => {
    stubWebWindow()
    const store = createTestStore()

    await expect(store.getState().addRepoPath('/srv/project')).resolves.toBeNull()

    expect(reposAdd).not.toHaveBeenCalled()
    expect(runtimeEnvironmentCall).not.toHaveBeenCalled()
    expect(toast.error).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({ description: 'Choose a dev server for this project first.' })
    )
  })

  it('leaves the desktop payload untouched when no host is involved', async () => {
    reposAdd.mockResolvedValue({ repo: localRepo })
    const store = createTestStore()

    await store.getState().addRepoPath('/local', 'git')

    expect(reposAdd).toHaveBeenCalledWith({ path: '/local', kind: 'git' })
  })
})

describe('host-scoped nested scan and import', () => {
  it('scanNestedRepos prefers the explicit host over the global selection', async () => {
    runtimeEnvironmentCall.mockResolvedValue(rpcOk([]))
    const store = createTestStore()
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'env-1' } as never,
      activeDevServerId: 'stale-host'
    })
    vi.spyOn(console, 'warn').mockImplementation(() => {})

    await store.getState().scanNestedRepos('/srv', undefined, { devServerId: 'ds-1' })

    expect(runtimeEnvironmentCall).toHaveBeenCalledWith(
      expect.objectContaining({
        method: 'projectGroup.scanNested',
        params: { devServerId: 'ds-1', rootPath: '/srv' }
      })
    )
  })
})

describe('default project per host', () => {
  it('keeps host-less behaviour for callers without a dev server (desktop runtime)', async () => {
    runtimeEnvironmentCall.mockImplementation(async (args: { method: string }) =>
      args.method === 'project.list'
        ? projectListWith([{ id: DEFAULT_PROJECT_ID, createdAt: 1, devServerId: '' }])
        : rpcOk({ id: remoteRepo.id, projectId: DEFAULT_PROJECT_ID, url: '/x', displayName: 'x' })
    )
    const store = createTestStore()
    store.setState({ settings: { activeRuntimeEnvironmentId: 'env-1' } as never })

    await store.getState().addRepoPath('/x', 'git')

    expect(runtimeEnvironmentCall).toHaveBeenCalledWith(
      expect.objectContaining({
        method: 'repo.add',
        params: { projectId: DEFAULT_PROJECT_ID, url: '/x', displayName: 'x' }
      })
    )
  })
})
