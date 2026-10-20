// @vitest-environment happy-dom
import { act, cleanup, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const listBranches = vi.fn()
vi.mock('@/runtime/runtime-repo-client', () => ({
  listRuntimeRepoBranches: (...args: unknown[]) => listBranches(...args)
}))

const storeState: { repos: unknown[]; settings: unknown } = { repos: [], settings: null }
vi.mock('@/store', () => ({
  useAppStore: (selector: (state: typeof storeState) => unknown) => selector(storeState)
}))

import { useRepoBranchList } from './use-repo-branch-list'

type Deferred<T> = { promise: Promise<T>; resolve: (value: T) => void }
function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((res) => {
    resolve = res
  })
  return { promise, resolve }
}

async function flush(): Promise<void> {
  await act(async () => {
    await Promise.resolve()
  })
}

const ok = (names: string[], truncated = false) => ({
  status: 'ok',
  branches: names.map((name) => ({
    name,
    remote: name.includes('/') && name.startsWith('origin')
  })),
  truncated
})

beforeEach(() => {
  listBranches.mockReset().mockResolvedValue(ok(['main', 'release/1.x']))
  storeState.repos = []
  storeState.settings = null
  vi.spyOn(console, 'error').mockImplementation(() => {})
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('useRepoBranchList', () => {
  it('loads when it mounts: loading first, then the branches and the truncation flag', async () => {
    listBranches.mockResolvedValue(ok(['main', 'origin/hotfix'], true))
    const { result } = renderHook(() => useRepoBranchList('repo-1'))
    expect(result.current.status).toBe('loading')
    await flush()
    expect(result.current).toMatchObject({ status: 'ready', truncated: true })
    expect(result.current.status === 'ready' && result.current.branches.map((b) => b.name)).toEqual(
      ['main', 'origin/hotfix']
    )
    expect(listBranches).toHaveBeenCalledTimes(1)
    expect(listBranches).toHaveBeenCalledWith({ activeRuntimeEnvironmentId: null }, 'repo-1')
  })

  it('asks the runtime that owns the repo', async () => {
    storeState.settings = { activeRuntimeEnvironmentId: 'env-7' }
    renderHook(() => useRepoBranchList('repo-1'))
    await flush()
    expect(listBranches).toHaveBeenCalledWith({ activeRuntimeEnvironmentId: 'env-7' }, 'repo-1')
  })

  it('falls back to search (unsupported) when the gateway has no such channel', async () => {
    listBranches.mockResolvedValue({ status: 'unsupported' })
    const { result } = renderHook(() => useRepoBranchList('repo-1'))
    await flush()
    expect(result.current.status).toBe('unsupported')
    expect(console.error).not.toHaveBeenCalled()
  })

  it('reports a failure, and retry loads again', async () => {
    listBranches.mockRejectedValueOnce(new Error('git failed'))
    const { result } = renderHook(() => useRepoBranchList('repo-1'))
    await flush()
    expect(result.current).toMatchObject({ status: 'error', offline: false })
    expect(console.error).toHaveBeenCalled()

    act(() => result.current.retry())
    expect(result.current.status).toBe('loading')
    await flush()
    expect(result.current.status).toBe('ready')
    expect(listBranches).toHaveBeenCalledTimes(2)
  })

  it('marks a transport failure as offline', async () => {
    listBranches.mockRejectedValueOnce(
      Object.assign(new Error('x'), { code: 'runtime_unavailable' })
    )
    const { result } = renderHook(() => useRepoBranchList('repo-1'))
    await flush()
    expect(result.current).toMatchObject({ status: 'error', offline: true })
  })

  it('never shows the previous repo list after a switch, even when the old answer arrives late', async () => {
    const first = deferred<unknown>()
    listBranches.mockReturnValueOnce(first.promise).mockResolvedValueOnce(ok(['develop']))
    const { result, rerender } = renderHook(({ id }) => useRepoBranchList(id), {
      initialProps: { id: 'repo-1' }
    })
    rerender({ id: 'repo-2' })
    expect(result.current.status).toBe('loading')
    await flush()
    first.resolve(ok(['main']))
    await flush()
    expect(result.current.status === 'ready' && result.current.branches.map((b) => b.name)).toEqual(
      ['develop']
    )
    // The first repo's own answer never came back through the second one's key.
    rerender({ id: 'repo-1' })
    expect(result.current.status).toBe('loading')
  })
})
