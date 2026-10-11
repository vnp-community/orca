// SOL-FE-E2E-005: Add Project puts each repo in its OWN project and refuses a folder that is
// already registered on that host.
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { toast } from 'sonner'
import { createTestStore } from './store-test-helpers'
import {
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

const rpcFail = (
  message: string
): { id: string; ok: false; error: { code: string; message: string }; _meta: unknown } => ({
  id: 'rpc',
  ok: false,
  error: { code: 'internal', message },
  _meta: { runtimeId: 'runtime-remote' }
})

function repoView(over: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: remoteRepo.id,
    projectId: 'proj-own',
    url: '/srv/project',
    displayName: 'project',
    devServerId: 'ds-1',
    position: 0,
    ...over
  }
}

function stubWebWindow(): void {
  vi.stubGlobal('window', { ...window, __ORCA_WEB_CLIENT__: true })
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('addRepoPath with ownProject', () => {
  it('creates a new project holding the repo through a runtime environment', async () => {
    runtimeEnvironmentCall.mockImplementation(
      async (args: { method: string; params?: unknown }) => {
        if (args.method === 'project.list') {
          return rpcOk([
            { id: 'proj-demo', name: 'Demo (this Mac)', devServerId: 'ds-1', createdAt: 1 }
          ])
        }
        if (args.method === 'project.create') {
          return rpcOk({ id: 'proj-own' })
        }
        if (args.method === 'repo.list') {
          const projectId = (args.params as { projectId: string }).projectId
          return rpcOk({ repos: projectId === 'proj-own' ? [repoView()] : [] })
        }
        throw new Error(`unexpected ${args.method}`)
      }
    )
    const store = createTestStore()
    store.setState({ settings: { activeRuntimeEnvironmentId: 'env-1' } as never })

    const repo = await store
      .getState()
      .addRepoPath('/srv/project', 'git', { devServerId: 'ds-1', ownProject: { name: 'Billing' } })

    expect(repo?.projectId).toBe('proj-own')
    expect(runtimeEnvironmentCall).toHaveBeenCalledWith({
      selector: 'env-1',
      method: 'project.create',
      params: {
        name: 'Billing',
        devServerId: 'ds-1',
        repoPath: '/srv/project',
        visibility: 'private'
      },
      timeoutMs: 30_000
    })
    expect(runtimeEnvironmentCall).not.toHaveBeenCalledWith(
      expect.objectContaining({ method: 'repo.add' })
    )
  })

  it('passes projectName to the local preload leg (what the web client uses)', async () => {
    stubWebWindow()
    ;(window as unknown as { api: { repos: { add: unknown } } }).api.repos.add = reposAdd
    reposAdd.mockResolvedValue({ repo: { ...localRepo, devServerId: 'ds-1' } })
    const store = createTestStore()

    await store
      .getState()
      .addRepoPath('/srv/project', 'git', { devServerId: 'ds-1', ownProject: { name: 'Billing' } })

    expect(reposAdd).toHaveBeenCalledWith({
      path: '/srv/project',
      kind: 'git',
      devServerId: 'ds-1',
      projectName: 'Billing'
    })
  })

  it('does not send projectName without ownProject (onboarding and other callers)', async () => {
    stubWebWindow()
    reposAdd.mockResolvedValue({ repo: { ...localRepo, devServerId: 'ds-1' } })
    const store = createTestStore()

    await store.getState().addRepoPath('/srv/project', 'git', { devServerId: 'ds-1' })

    expect(reposAdd).toHaveBeenCalledWith({
      path: '/srv/project',
      kind: 'git',
      devServerId: 'ds-1'
    })
  })

  it('refuses a folder registered in another project and points at that project', async () => {
    runtimeEnvironmentCall.mockImplementation(async (args: { method: string }) => {
      if (args.method === 'project.list') {
        return rpcOk([
          { id: 'proj-demo', name: 'Demo (this Mac)', devServerId: 'ds-1', createdAt: 1 }
        ])
      }
      if (args.method === 'repo.list') {
        return rpcOk({
          repos: [repoView({ id: 'repo-old', projectId: 'proj-demo', url: '/srv/project/' })]
        })
      }
      throw new Error(`unexpected ${args.method}`)
    })
    const store = createTestStore()
    store.setState({ settings: { activeRuntimeEnvironmentId: 'env-1' } as never })

    const repo = await store
      .getState()
      .addRepoPath('/srv/project', 'git', { devServerId: 'ds-1', ownProject: { name: 'Billing' } })

    expect(repo).toBeNull()
    expect(runtimeEnvironmentCall).not.toHaveBeenCalledWith(
      expect.objectContaining({ method: 'project.create' })
    )
    expect(toast.error).toHaveBeenCalledWith(
      'Project already added',
      expect.objectContaining({
        description: expect.stringContaining('Demo (this Mac)'),
        action: expect.objectContaining({ label: 'Open project' })
      })
    )
  })

  it('maps the backend duplicate code to the same notice', async () => {
    runtimeEnvironmentCall.mockImplementation(async (args: { method: string }) => {
      if (args.method === 'project.list') {
        return rpcOk([])
      }
      if (args.method === 'project.create') {
        return rpcFail(
          'PROJECT_REPO_ALREADY_REGISTERED: /srv/project already in project Old one (proj-old)'
        )
      }
      throw new Error(`unexpected ${args.method}`)
    })
    const store = createTestStore()
    store.setState({ settings: { activeRuntimeEnvironmentId: 'env-1' } as never })

    const repo = await store
      .getState()
      .addRepoPath('/srv/project', 'git', { devServerId: 'ds-1', ownProject: { name: 'Billing' } })

    expect(repo).toBeNull()
    expect(toast.error).toHaveBeenCalledWith(
      'Project already added',
      expect.objectContaining({ description: expect.stringContaining('Old one') })
    )
  })
})
