import { describe, expect, it, vi } from 'vitest'
import {
  deriveProjectNameFromPath,
  findRegisteredRepoRemote,
  registerRepoInOwnProject,
  type ProjectRpc,
  type RegisteredRepoView
} from './own-project-registration'
import { RepoAlreadyRegisteredError } from './registered-repo-lookup'

function repoView(over: Partial<RegisteredRepoView> = {}): RegisteredRepoView {
  return {
    id: 'r-new',
    projectId: 'p-new',
    url: '/x/foo',
    displayName: 'foo',
    position: 0,
    devServerId: 'dev-a',
    ...over
  }
}

type Handlers = Record<string, (params: unknown) => unknown>

function fakeRpc(handlers: Handlers): {
  rpc: ProjectRpc
  calls: { method: string; params: unknown }[]
} {
  const calls: { method: string; params: unknown }[] = []
  const rpc: ProjectRpc = async <T>(method: string, params?: unknown): Promise<T> => {
    calls.push({ method, params })
    const handler = handlers[method]
    if (!handler) {
      throw new Error(`unexpected ${method}`)
    }
    return (await handler(params)) as T
  }
  return { rpc, calls }
}

describe('deriveProjectNameFromPath', () => {
  it('uses the folder name for POSIX and Windows paths', () => {
    expect(deriveProjectNameFromPath('/home/me/my-app/', 'x')).toBe('my-app')
    expect(deriveProjectNameFromPath('C:\\work\\App', 'x')).toBe('App')
    expect(deriveProjectNameFromPath('/srv/repo.git', 'x')).toBe('repo')
    expect(deriveProjectNameFromPath('/', 'fallback')).toBe('fallback')
  })
})

describe('registerRepoInOwnProject', () => {
  it('creates one project with the repo path and host, then reads the repo back', async () => {
    const { rpc, calls } = fakeRpc({
      'project.list': () => [{ id: 'p-old', name: 'Demo (this Mac)' }],
      'repo.list': (p) =>
        (p as { projectId: string }).projectId === 'p-new'
          ? { repos: [repoView()] }
          : { repos: [repoView({ id: 'r-old', projectId: 'p-old', url: '/other' })] },
      'project.create': () => ({ id: 'p-new' })
    })
    const repo = await registerRepoInOwnProject(rpc, { devServerId: 'dev-a', path: '/x/foo/' })
    expect(repo.id).toBe('r-new')
    const create = calls.filter((c) => c.method === 'project.create')
    expect(create).toHaveLength(1)
    expect(create[0].params).toEqual({
      name: 'foo',
      devServerId: 'dev-a',
      repoPath: '/x/foo/',
      visibility: 'private'
    })
    expect(calls.some((c) => c.method === 'repo.add')).toBe(false)
  })

  it('uses the chosen project name', async () => {
    const { rpc, calls } = fakeRpc({
      'project.list': () => [],
      'repo.list': () => ({ repos: [repoView()] }),
      'project.create': () => ({ id: 'p-new' })
    })
    await registerRepoInOwnProject(rpc, {
      devServerId: 'dev-a',
      path: '/x/foo',
      projectName: '  Billing  '
    })
    expect(calls.find((c) => c.method === 'project.create')?.params).toMatchObject({
      name: 'Billing'
    })
  })

  it('blocks a folder that is already registered on that host and writes nothing', async () => {
    const { rpc, calls } = fakeRpc({
      'project.list': () => [{ id: 'p-old', name: 'Demo (this Mac)' }],
      'repo.list': () => ({
        repos: [repoView({ id: 'r-old', projectId: 'p-old', url: '/x/foo/' })]
      })
    })
    const err = await registerRepoInOwnProject(rpc, { devServerId: 'dev-a', path: '/x/foo' }).catch(
      (e: unknown) => e
    )
    expect(err).toBeInstanceOf(RepoAlreadyRegisteredError)
    expect((err as RepoAlreadyRegisteredError).projectName).toBe('Demo (this Mac)')
    expect((err as RepoAlreadyRegisteredError).projectId).toBe('p-old')
    expect(calls.map((c) => c.method)).not.toContain('project.create')
  })

  it('allows the same path on a different host', async () => {
    const { rpc } = fakeRpc({
      'project.list': () => [{ id: 'p-old', name: 'Other' }],
      'repo.list': (p) =>
        (p as { projectId: string }).projectId === 'p-new'
          ? { repos: [repoView({ devServerId: 'dev-b' })] }
          : { repos: [repoView({ id: 'r-old', projectId: 'p-old', devServerId: 'dev-a' })] },
      'project.create': () => ({ id: 'p-new' })
    })
    const repo = await registerRepoInOwnProject(rpc, { devServerId: 'dev-b', path: '/x/foo' })
    expect(repo.id).toBe('r-new')
  })

  it('maps the backend duplicate code to the same error', async () => {
    const { rpc } = fakeRpc({
      'project.list': () => [],
      'project.create': () => {
        throw new Error('PROJECT_REPO_ALREADY_REGISTERED: repo /x/foo already in project Old (p-1)')
      }
    })
    const err = await registerRepoInOwnProject(rpc, { devServerId: 'dev-a', path: '/x/foo' }).catch(
      (e: unknown) => e
    )
    expect(err).toBeInstanceOf(RepoAlreadyRegisteredError)
    expect((err as RepoAlreadyRegisteredError).projectName).toBe('Old')
  })

  it('adds the repo itself when an older backend created the project without one', async () => {
    const add = vi.fn(() => repoView({ id: 'r-added' }))
    const { rpc } = fakeRpc({
      'project.list': () => [],
      'repo.list': () => ({ repos: [] }),
      'project.create': () => ({ id: 'p-new' }),
      'repo.add': add
    })
    const repo = await registerRepoInOwnProject(rpc, { devServerId: 'dev-a', path: '/x/foo' })
    expect(repo.id).toBe('r-added')
    expect(add).toHaveBeenCalledWith({
      projectId: 'p-new',
      url: '/x/foo',
      displayName: 'foo',
      devServerId: 'dev-a'
    })
  })

  it('names the created project when the fallback repo.add fails', async () => {
    const { rpc } = fakeRpc({
      'project.list': () => [],
      'repo.list': () => ({ repos: [] }),
      'project.create': () => ({ id: 'p-new' }),
      'repo.add': () => {
        throw new Error('dev server offline')
      }
    })
    await expect(
      registerRepoInOwnProject(rpc, { devServerId: 'dev-a', path: '/x/foo' })
    ).rejects.toThrow(/foo.*dev server offline/)
  })

  it('passes other project.create errors through untouched', async () => {
    const { rpc } = fakeRpc({
      'project.list': () => [],
      'project.create': () => {
        throw new Error('PROJECT_REPO_PATH_NOT_FOUND: repo_path does not exist on dev server')
      }
    })
    await expect(
      registerRepoInOwnProject(rpc, { devServerId: 'dev-a', path: '/missing' })
    ).rejects.toThrow('PROJECT_REPO_PATH_NOT_FOUND')
  })
})

describe('findRegisteredRepoRemote', () => {
  it('ignores a project whose repo.list fails', async () => {
    const { rpc } = fakeRpc({
      'project.list': () => [
        { id: 'p1', name: 'Broken' },
        { id: 'p2', name: 'Good' }
      ],
      'repo.list': (p) => {
        if ((p as { projectId: string }).projectId === 'p1') {
          throw new Error('denied')
        }
        return { repos: [repoView({ projectId: 'p2' })] }
      }
    })
    const found = await findRegisteredRepoRemote(rpc, { devServerId: 'dev-a', path: '/x/foo' })
    expect(found?.projectName).toBe('Good')
  })
})
