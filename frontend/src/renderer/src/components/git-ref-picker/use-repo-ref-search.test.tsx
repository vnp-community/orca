// @vitest-environment happy-dom
import { act, cleanup, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const getDefault = vi.fn()
const searchRefs = vi.fn()
vi.mock('@/runtime/runtime-repo-client', () => ({
  getRuntimeRepoBaseRefDefault: (...args: unknown[]) => getDefault(...args),
  searchRuntimeRepoBaseRefs: (...args: unknown[]) => searchRefs(...args)
}))

const storeState: { repos: unknown[]; settings: unknown } = { repos: [], settings: null }
vi.mock('@/store', () => ({
  useAppStore: (selector: (state: typeof storeState) => unknown) => selector(storeState)
}))

import {
  REPO_REF_SEARCH_DEBOUNCE_MS,
  REPO_REF_SEARCH_RESULT_LIMIT,
  isRepoRefHostOffline,
  useRepoRefSearch
} from './use-repo-ref-search'

type Deferred<T> = {
  promise: Promise<T>
  resolve: (value: T) => void
  reject: (e: unknown) => void
}
function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

async function flush(): Promise<void> {
  await act(async () => {
    await Promise.resolve()
  })
}

async function advance(ms: number): Promise<void> {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })
}

beforeEach(() => {
  vi.useFakeTimers()
  getDefault.mockReset().mockResolvedValue({ defaultBaseRef: 'origin/main', remoteCount: 1 })
  searchRefs.mockReset().mockResolvedValue([])
  storeState.repos = []
  storeState.settings = null
  vi.spyOn(console, 'error').mockImplementation(() => {})
})

afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('useRepoRefSearch default lookup', () => {
  it('starts loading, then exposes the default and the remote count', async () => {
    getDefault.mockResolvedValue({ defaultBaseRef: 'origin/main', remoteCount: 2 })
    const { result } = renderHook(() => useRepoRefSearch('repo-1'))
    expect(result.current.defaultStatus).toBe('loading')
    expect(result.current.defaultBaseRef).toBeNull()
    await flush()
    expect(result.current.defaultStatus).toBe('ready')
    expect(result.current.defaultBaseRef).toBe('origin/main')
    expect(result.current.remoteCount).toBe(2)
    expect(getDefault).toHaveBeenCalledWith({ activeRuntimeEnvironmentId: null }, 'repo-1')
  })

  it('never invents a default: a repo without one stays null', async () => {
    getDefault.mockResolvedValue({ defaultBaseRef: null, remoteCount: 0 })
    const { result } = renderHook(() => useRepoRefSearch('repo-1'))
    await flush()
    expect(result.current.defaultStatus).toBe('ready')
    expect(result.current.defaultBaseRef).toBeNull()
  })

  it('reports a failed lookup as failed with no default and no remote hint, and logs it', async () => {
    getDefault.mockRejectedValue(new Error('boom'))
    const { result } = renderHook(() => useRepoRefSearch('repo-1'))
    await flush()
    expect(result.current.defaultStatus).toBe('failed')
    expect(result.current.defaultBaseRef).toBeNull()
    expect(result.current.remoteCount).toBe(0)
    expect(console.error).toHaveBeenCalled()
  })

  it('routes through the runtime environment that owns the repo', async () => {
    storeState.settings = { activeRuntimeEnvironmentId: 'env-7' }
    renderHook(() => useRepoRefSearch('repo-1'))
    await flush()
    expect(getDefault).toHaveBeenCalledWith({ activeRuntimeEnvironmentId: 'env-7' }, 'repo-1')
  })

  it("drops the previous repo's default at once and ignores its late answer", async () => {
    const slow = deferred<{ defaultBaseRef: string | null; remoteCount: number }>()
    getDefault.mockResolvedValueOnce({ defaultBaseRef: 'origin/main', remoteCount: 3 })
    const { result, rerender } = renderHook(({ repoId }) => useRepoRefSearch(repoId), {
      initialProps: { repoId: 'repo-a' }
    })
    await flush()
    expect(result.current.defaultBaseRef).toBe('origin/main')

    getDefault.mockReturnValueOnce(slow.promise)
    rerender({ repoId: 'repo-b' })
    expect(result.current.defaultBaseRef).toBeNull()
    expect(result.current.remoteCount).toBe(0)
    expect(result.current.defaultStatus).toBe('loading')

    slow.resolve({ defaultBaseRef: 'origin/develop', remoteCount: 1 })
    await flush()
    expect(result.current.defaultBaseRef).toBe('origin/develop')
  })

  it('ignores a late default of the repo it has already left', async () => {
    const late = deferred<{ defaultBaseRef: string | null; remoteCount: number }>()
    getDefault.mockReturnValueOnce(late.promise)
    const { result, rerender } = renderHook(({ repoId }) => useRepoRefSearch(repoId), {
      initialProps: { repoId: 'repo-a' }
    })
    getDefault.mockResolvedValueOnce({ defaultBaseRef: 'origin/develop', remoteCount: 1 })
    rerender({ repoId: 'repo-b' })
    await flush()
    late.resolve({ defaultBaseRef: 'origin/stale', remoteCount: 9 })
    await flush()
    expect(result.current.defaultBaseRef).toBe('origin/develop')
    expect(result.current.remoteCount).toBe(1)
  })
})

describe('useRepoRefSearch search', () => {
  it('stays idle and sends nothing for a query shorter than two characters', async () => {
    const { result } = renderHook(() => useRepoRefSearch('repo-1'))
    act(() => result.current.setQuery('m'))
    await advance(1000)
    expect(searchRefs).not.toHaveBeenCalled()
    expect(result.current.status).toBe('idle')
    act(() => result.current.setQuery('  m  '))
    await advance(1000)
    expect(searchRefs).not.toHaveBeenCalled()
  })

  it('debounces: one call, with the last text trimmed, after the quiet period', async () => {
    searchRefs.mockResolvedValue(['origin/main'])
    const { result } = renderHook(() => useRepoRefSearch('repo-1'))
    act(() => result.current.setQuery('ma'))
    await advance(REPO_REF_SEARCH_DEBOUNCE_MS - 50)
    act(() => result.current.setQuery('  mai  '))
    await advance(REPO_REF_SEARCH_DEBOUNCE_MS - 50)
    expect(searchRefs).not.toHaveBeenCalled()
    expect(result.current.status).toBe('searching')

    await advance(60)
    expect(searchRefs).toHaveBeenCalledTimes(1)
    expect(searchRefs).toHaveBeenCalledWith(
      { activeRuntimeEnvironmentId: null },
      'repo-1',
      'mai',
      REPO_REF_SEARCH_RESULT_LIMIT
    )
    expect(result.current.status).toBe('results')
    expect(result.current.results).toEqual(['origin/main'])
  })

  it('reports empty when the host finds nothing', async () => {
    const { result } = renderHook(() => useRepoRefSearch('repo-1'))
    act(() => result.current.setQuery('zz'))
    await advance(REPO_REF_SEARCH_DEBOUNCE_MS)
    expect(result.current.status).toBe('empty')
    expect(result.current.results).toEqual([])
  })

  it('never sends a query past the byte limit and shows it as no match', async () => {
    const { result } = renderHook(() => useRepoRefSearch('repo-1'))
    act(() => result.current.setQuery('a'.repeat(2049)))
    await advance(1000)
    expect(searchRefs).not.toHaveBeenCalled()
    expect(result.current.status).toBe('empty')
  })

  it('keeps only the answer to the latest query when an older one arrives late', async () => {
    const first = deferred<string[]>()
    const second = deferred<string[]>()
    searchRefs.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    const { result } = renderHook(() => useRepoRefSearch('repo-1'))

    act(() => result.current.setQuery('ma'))
    await advance(REPO_REF_SEARCH_DEBOUNCE_MS)
    act(() => result.current.setQuery('mai'))
    await advance(REPO_REF_SEARCH_DEBOUNCE_MS)
    expect(searchRefs).toHaveBeenCalledTimes(2)

    second.resolve(['origin/main'])
    await flush()
    first.resolve(['origin/master', 'origin/mainline-old'])
    await flush()
    expect(result.current.results).toEqual(['origin/main'])
    expect(result.current.status).toBe('results')
  })

  it('does not call the host when the query is edited inside the quiet period', async () => {
    const { result } = renderHook(() => useRepoRefSearch('repo-1'))
    act(() => result.current.setQuery('ma'))
    await advance(100)
    act(() => result.current.setQuery(''))
    await advance(1000)
    expect(searchRefs).not.toHaveBeenCalled()
    expect(result.current.status).toBe('idle')
  })

  it('reports a failed search as error, logs it, and clears the old results', async () => {
    searchRefs.mockResolvedValueOnce(['origin/main'])
    const { result } = renderHook(() => useRepoRefSearch('repo-1'))
    act(() => result.current.setQuery('ma'))
    await advance(REPO_REF_SEARCH_DEBOUNCE_MS)
    expect(result.current.results).toEqual(['origin/main'])

    searchRefs.mockRejectedValueOnce(new Error('git failed'))
    act(() => result.current.setQuery('mas'))
    await advance(REPO_REF_SEARCH_DEBOUNCE_MS)
    expect(result.current.status).toBe('error')
    expect(result.current.results).toEqual([])
    expect(console.error).toHaveBeenCalled()
  })

  it.each(['runtime_unavailable', 'remote_runtime_unavailable', 'connection_refused', 'timeout'])(
    'reports a %s failure as offline',
    async (code) => {
      searchRefs.mockRejectedValueOnce(Object.assign(new Error('x'), { code }))
      const { result } = renderHook(() => useRepoRefSearch('repo-1'))
      act(() => result.current.setQuery('ma'))
      await advance(REPO_REF_SEARCH_DEBOUNCE_MS)
      expect(result.current.status).toBe('offline')
    }
  )

  it('reports any failure as offline while the browser itself is offline', async () => {
    vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false)
    searchRefs.mockRejectedValueOnce(new Error('fetch failed'))
    const { result } = renderHook(() => useRepoRefSearch('repo-1'))
    act(() => result.current.setQuery('ma'))
    await advance(REPO_REF_SEARCH_DEBOUNCE_MS)
    expect(result.current.status).toBe('offline')
  })

  it('searches again on retry, for the default and for the query', async () => {
    searchRefs.mockRejectedValueOnce(new Error('git failed'))
    const { result } = renderHook(() => useRepoRefSearch('repo-1'))
    act(() => result.current.setQuery('ma'))
    await advance(REPO_REF_SEARCH_DEBOUNCE_MS)
    expect(result.current.status).toBe('error')

    searchRefs.mockResolvedValueOnce(['origin/main'])
    act(() => result.current.retry())
    expect(result.current.status).toBe('searching')
    await advance(REPO_REF_SEARCH_DEBOUNCE_MS)
    expect(result.current.status).toBe('results')
    expect(getDefault).toHaveBeenCalledTimes(2)
  })

  it('forgets the query and the results when the repo changes', async () => {
    searchRefs.mockResolvedValue(['origin/main'])
    const { result, rerender } = renderHook(({ repoId }) => useRepoRefSearch(repoId), {
      initialProps: { repoId: 'repo-a' }
    })
    act(() => result.current.setQuery('ma'))
    await advance(REPO_REF_SEARCH_DEBOUNCE_MS)
    expect(result.current.results).toEqual(['origin/main'])

    rerender({ repoId: 'repo-b' })
    expect(result.current.query).toBe('')
    expect(result.current.results).toEqual([])
    expect(result.current.status).toBe('idle')
  })

  it("never shows a repo's results for the same text typed in another repo before the answer arrives", async () => {
    const slow = deferred<string[]>()
    searchRefs.mockResolvedValueOnce(['repo-a/main']).mockReturnValueOnce(slow.promise)
    const { result, rerender } = renderHook(({ repoId }) => useRepoRefSearch(repoId), {
      initialProps: { repoId: 'repo-a' }
    })
    act(() => result.current.setQuery('ma'))
    await advance(REPO_REF_SEARCH_DEBOUNCE_MS)
    rerender({ repoId: 'repo-b' })
    act(() => result.current.setQuery('ma'))
    expect(result.current.results).toEqual([])
    expect(result.current.status).toBe('searching')
    await advance(REPO_REF_SEARCH_DEBOUNCE_MS)
    expect(result.current.results).toEqual([])
    slow.resolve(['repo-b/main'])
    await flush()
    expect(result.current.results).toEqual(['repo-b/main'])
  })
})

describe('isRepoRefHostOffline', () => {
  it('is false for an error the host answered with', () => {
    expect(isRepoRefHostOffline(new Error('bad ref'))).toBe(false)
    expect(isRepoRefHostOffline(Object.assign(new Error('x'), { code: 'runtime_error' }))).toBe(
      false
    )
    expect(isRepoRefHostOffline(null)).toBe(false)
    expect(isRepoRefHostOffline(undefined)).toBe(false)
  })
})
