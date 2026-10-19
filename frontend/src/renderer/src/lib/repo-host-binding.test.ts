import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  isUnboundWebRepo,
  pickProjectBoundToDevServer,
  REPO_HOST_REQUIRED_CODE,
  RepoHostRequiredError,
  requireDevServerIdForWeb,
  requireRepoDevServerId,
  resolveProjectIdForDevServer,
  resolveRepoHostBinding
} from './repo-host-binding'

function stubWebLocation(isWeb: boolean): void {
  vi.stubGlobal('window', {
    location: { pathname: isWeb ? '/web-index.html' : '/index.html' }
  })
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('resolveRepoHostBinding', () => {
  it('maps every execution host kind, and anything unknown to local', () => {
    expect(resolveRepoHostBinding('devServer:ds-1')).toEqual({
      kind: 'devServer',
      devServerId: 'ds-1'
    })
    expect(resolveRepoHostBinding('ssh:build-box')).toEqual({
      kind: 'ssh',
      connectionId: 'build-box'
    })
    expect(resolveRepoHostBinding('runtime:env-1')).toEqual({
      kind: 'runtime',
      environmentId: 'env-1'
    })
    expect(resolveRepoHostBinding('local')).toEqual({ kind: 'local' })
    expect(resolveRepoHostBinding(null)).toEqual({ kind: 'local' })
  })
})

describe('requireRepoDevServerId', () => {
  it('returns the trimmed id and throws a coded error when it is missing', () => {
    expect(requireRepoDevServerId(' ds-1 ')).toBe('ds-1')
    for (const missing of [undefined, null, '', '   ']) {
      expect(() => requireRepoDevServerId(missing)).toThrow(RepoHostRequiredError)
    }
    try {
      requireRepoDevServerId('')
    } catch (error) {
      expect((error as RepoHostRequiredError).code).toBe(REPO_HOST_REQUIRED_CODE)
    }
  })
})

describe('requireDevServerIdForWeb', () => {
  it('forces a dev server on web but leaves desktop routing alone', () => {
    stubWebLocation(true)
    expect(requireDevServerIdForWeb('ds-1')).toBe('ds-1')
    expect(() => requireDevServerIdForWeb(null)).toThrow(RepoHostRequiredError)

    stubWebLocation(false)
    expect(requireDevServerIdForWeb(null)).toBeUndefined()
    expect(requireDevServerIdForWeb('ds-1')).toBe('ds-1')
  })
})

describe('isUnboundWebRepo', () => {
  it('flags only web repos with neither a dev server nor an SSH connection', () => {
    stubWebLocation(true)
    expect(isUnboundWebRepo({ devServerId: '' })).toBe(true)
    expect(isUnboundWebRepo({ devServerId: null, connectionId: null })).toBe(true)
    expect(isUnboundWebRepo({ devServerId: 'ds-1' })).toBe(false)
    expect(isUnboundWebRepo({ connectionId: 'ssh-1' })).toBe(false)
    expect(isUnboundWebRepo(undefined)).toBe(false)

    stubWebLocation(false)
    expect(isUnboundWebRepo({ devServerId: '' })).toBe(false)
  })
})

describe('project selection per dev server', () => {
  const projects = [
    { id: 'late', createdAt: 5, devServerId: 'ds-1' },
    { id: 'early', createdAt: 1, devServerId: 'ds-1' },
    { id: 'other', createdAt: 0, devServerId: 'ds-2' },
    { id: 'legacy', createdAt: 0, devServerId: '' }
  ]

  it('picks the earliest project bound to the host', () => {
    expect(pickProjectBoundToDevServer(projects, 'ds-1')?.id).toBe('early')
    expect(pickProjectBoundToDevServer(projects, 'ds-3')).toBeUndefined()
  })

  it('creates a host-bound project when none exists, and falls back to the earliest on failure', async () => {
    const createProject = vi.fn().mockResolvedValue({ id: 'created' })
    await expect(
      resolveProjectIdForDevServer({
        devServerId: 'ds-3',
        listProjects: async () => projects,
        createProject
      })
    ).resolves.toBe('created')
    expect(createProject).toHaveBeenCalledWith('ds-3')

    vi.spyOn(console, 'warn').mockImplementation(() => {})
    await expect(
      resolveProjectIdForDevServer({
        devServerId: 'ds-3',
        listProjects: async () => projects,
        createProject: vi.fn().mockRejectedValue(new Error('PROJECT_REPO_PATH_REQUIRED'))
      })
    ).resolves.toBe('other')
  })

  it('surfaces the create failure when the caller has no project at all', async () => {
    await expect(
      resolveProjectIdForDevServer({
        devServerId: 'ds-1',
        listProjects: async () => [],
        createProject: vi.fn().mockRejectedValue(new Error('boom'))
      })
    ).rejects.toThrow('boom')
  })

  it('reuses a bound project without creating one', async () => {
    const createProject = vi.fn()
    await expect(
      resolveProjectIdForDevServer({
        devServerId: 'ds-2',
        listProjects: async () => projects,
        createProject
      })
    ).resolves.toBe('other')
    expect(createProject).not.toHaveBeenCalled()
  })
})
