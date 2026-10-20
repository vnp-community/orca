// @vitest-environment happy-dom
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { usePlanBundleFile } from './usePlanBundleFile'

const ok = (value: unknown) => ({ ok: true as const, value })
const fail = (kind: string, message: string) => ({
  ok: false as const,
  error: { kind, code: 'runtime_error', message }
})

type Props = { id: string; path: string | null; revision?: number; key?: string }
function setup(initial: Props) {
  return renderHook((p: Props) => usePlanBundleFile(p.id, p.path, p.revision, p.key), {
    initialProps: initial
  })
}

beforeEach(() => callRequestRpc.mockReset())
afterEach(cleanup)

describe('usePlanBundleFile', () => {
  it('is idle without a path and calls nothing', async () => {
    const { result } = setup({ id: 'r1', path: null })
    await act(async () => undefined)
    expect(result.current.status).toBe('idle')
    expect(callRequestRpc).not.toHaveBeenCalled()
  })

  it('loads a file once and serves it from memory when the tab is opened again', async () => {
    callRequestRpc.mockResolvedValue(ok({ path: 'design.md', content: '# Design', omitted: false }))
    const { result, rerender } = setup({ id: 'r1', path: 'design.md' })
    expect(result.current.status).toBe('loading')
    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(result.current).toMatchObject({ status: 'ready', content: '# Design' })
    expect(callRequestRpc).toHaveBeenCalledWith('request.planBundleFile', {
      id: 'r1',
      path: 'design.md'
    })
    callRequestRpc.mockResolvedValue(
      ok({ path: 'tasks.md', content: '- [ ] 1.1 x', omitted: false })
    )
    rerender({ id: 'r1', path: 'tasks.md' })
    await waitFor(() => expect(result.current).toMatchObject({ content: '- [ ] 1.1 x' }))
    rerender({ id: 'r1', path: 'design.md' })
    expect(result.current).toMatchObject({ status: 'ready', content: '# Design' })
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
  })

  it('remembers each version of the same path on its own and sends the revision', async () => {
    callRequestRpc.mockImplementation(async (_m: string, p: { revision?: number } = {}) =>
      ok({ path: 'proposal.md', content: `v${p.revision ?? 'current'}`, omitted: false })
    )
    const { result, rerender } = setup({ id: 'r1', path: 'proposal.md', revision: 1 })
    await waitFor(() => expect(result.current).toMatchObject({ content: 'v1' }))
    rerender({ id: 'r1', path: 'proposal.md', revision: 2 })
    await waitFor(() => expect(result.current).toMatchObject({ content: 'v2' }))
    rerender({ id: 'r1', path: 'proposal.md', revision: 1 })
    expect(result.current).toMatchObject({ content: 'v1' })
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
    expect(callRequestRpc).toHaveBeenCalledWith('request.planBundleFile', {
      id: 'r1',
      path: 'proposal.md',
      revision: 2
    })
  })

  it('reads the current version again once its key changes', async () => {
    callRequestRpc.mockResolvedValueOnce(ok({ path: 'tasks.md', content: 'old', omitted: false }))
    const { result, rerender } = setup({ id: 'r1', path: 'tasks.md', key: 'commit-a' })
    await waitFor(() => expect(result.current).toMatchObject({ content: 'old' }))
    callRequestRpc.mockResolvedValueOnce(ok({ path: 'tasks.md', content: 'new', omitted: false }))
    rerender({ id: 'r1', path: 'tasks.md', key: 'commit-b' })
    await waitFor(() => expect(result.current).toMatchObject({ content: 'new' }))
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
  })

  it('does not remember an error, and Try again reads the file again', async () => {
    callRequestRpc.mockResolvedValueOnce(fail('network', 'offline'))
    const { result } = setup({ id: 'r1', path: 'design.md' })
    await waitFor(() => expect(result.current.status).toBe('error'))
    callRequestRpc.mockResolvedValueOnce(ok({ path: 'design.md', content: 'ok', omitted: false }))
    act(() => result.current.retry())
    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
  })

  it('reports a file kept only as a placeholder as omitted, flagged or not', async () => {
    callRequestRpc.mockResolvedValueOnce(ok({ path: 'a.md', content: '', omitted: true }))
    const first = setup({ id: 'r1', path: 'a.md' })
    await waitFor(() => expect(first.result.current.status).toBe('omitted'))
    callRequestRpc.mockResolvedValueOnce(
      ok({ path: 'b.md', content: '<omitted size=9000000 sha256=abc>' })
    )
    const second = setup({ id: 'r1', path: 'b.md' })
    await waitFor(() => expect(second.result.current.status).toBe('omitted'))
  })

  it('reports a pruned version, remembers it, and a missing file without remembering', async () => {
    callRequestRpc.mockResolvedValueOnce(
      fail('invalid_state', 'REQUEST_PLAN_REVISION_TEXT_PRUNED: pruned')
    )
    const { result, rerender } = setup({ id: 'r1', path: 'a.md', revision: 1 })
    await waitFor(() => expect(result.current.status).toBe('pruned'))
    callRequestRpc.mockResolvedValueOnce(
      fail('not_found', 'REQUEST_PLAN_BUNDLE_FILE_NOT_FOUND: gone')
    )
    rerender({ id: 'r1', path: 'b.md', revision: 1 })
    await waitFor(() => expect(result.current.status).toBe('notFound'))
    rerender({ id: 'r1', path: 'a.md', revision: 1 })
    expect(result.current.status).toBe('pruned')
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
  })

  it('forgets everything when the Request changes', async () => {
    callRequestRpc.mockResolvedValue(ok({ path: 'a.md', content: 'x', omitted: false }))
    const { result, rerender } = setup({ id: 'r1', path: 'a.md' })
    await waitFor(() => expect(result.current.status).toBe('ready'))
    rerender({ id: 'r2', path: 'a.md' })
    expect(result.current.status).toBe('loading')
    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
  })

  it('ignores an answer that arrives after the tab changed', async () => {
    let release: (v: unknown) => void = () => undefined
    callRequestRpc.mockImplementationOnce(() => new Promise((resolve) => (release = resolve)))
    const { result, rerender } = setup({ id: 'r1', path: 'a.md' })
    callRequestRpc.mockResolvedValueOnce(ok({ path: 'b.md', content: 'B', omitted: false }))
    rerender({ id: 'r1', path: 'b.md' })
    await waitFor(() => expect(result.current).toMatchObject({ content: 'B' }))
    await act(async () => release(ok({ path: 'a.md', content: 'A', omitted: false })))
    expect(result.current).toMatchObject({ content: 'B' })
  })
})
