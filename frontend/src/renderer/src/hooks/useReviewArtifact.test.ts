// @vitest-environment happy-dom
import { renderHook, waitFor, cleanup } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import {
  ARTIFACT_CACHE_SIZE,
  resetReviewArtifactCache,
  useReviewArtifact
} from './useReviewArtifact'

const answer = (id: string, extra: Record<string, unknown> = {}) => ({
  ok: true,
  value: {
    artifact: {
      artifactId: id,
      section: 'flow',
      variant: 'delta',
      blobDigest: `sha-${id}`,
      viewBox: '0 0 800 400',
      revision: 1
    },
    mediaType: 'text/html',
    content: `<svg>${id}</svg>`,
    ...extra
  }
})

beforeEach(() => {
  callRequestRpc.mockReset()
  resetReviewArtifactCache()
})
afterEach(cleanup)

describe('useReviewArtifact', () => {
  it('reads the drawing when asked and only then', async () => {
    callRequestRpc.mockResolvedValue(answer('a1'))
    const { result, rerender } = renderHook(
      ({ id }: { id: string | undefined }) => useReviewArtifact('r1', id, true),
      { initialProps: { id: undefined as string | undefined } }
    )
    expect(result.current.status).toBe('idle')
    expect(callRequestRpc).not.toHaveBeenCalled()
    rerender({ id: 'a1' })
    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(result.current.html).toBe('<svg>a1</svg>')
    expect(result.current.meta?.viewBox).toBe('0 0 800 400')
    expect(callRequestRpc).toHaveBeenCalledWith('request.review.artifact', {
      id: 'r1',
      artifactId: 'a1'
    })
  })

  it('does not read while disabled', () => {
    renderHook(() => useReviewArtifact('r1', 'a1', false))
    expect(callRequestRpc).not.toHaveBeenCalled()
  })

  it('shows a drawing it has already read at once and only asks whether it changed', async () => {
    callRequestRpc.mockResolvedValue(answer('a1'))
    const first = renderHook(() => useReviewArtifact('r1', 'a1'))
    await waitFor(() => expect(first.result.current.status).toBe('ready'))
    first.unmount()

    callRequestRpc.mockResolvedValue({
      ok: true,
      value: { artifact: { artifactId: 'a1', blobDigest: 'sha-a1' }, notModified: true }
    })
    const second = renderHook(() => useReviewArtifact('r1', 'a1'))
    expect(second.result.current.status).toBe('ready')
    expect(second.result.current.html).toBe('<svg>a1</svg>')
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalledTimes(2))
    expect(callRequestRpc.mock.calls[1][1]).toEqual({
      id: 'r1',
      artifactId: 'a1',
      ifDigest: 'sha-a1'
    })
    expect(second.result.current.html).toBe('<svg>a1</svg>')
  })

  it('keeps what it showed when a refresh fails, and says not found when the drawing is gone', async () => {
    callRequestRpc.mockResolvedValue(answer('a1'))
    const first = renderHook(() => useReviewArtifact('r1', 'a1'))
    await waitFor(() => expect(first.result.current.status).toBe('ready'))
    first.unmount()
    callRequestRpc.mockResolvedValue({ ok: false, error: { kind: 'network', message: 'x' } })
    const again = renderHook(() => useReviewArtifact('r1', 'a1'))
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalledTimes(2))
    expect(again.result.current.status).toBe('ready')

    callRequestRpc.mockResolvedValue({
      ok: false,
      error: { kind: 'not_found', backendCode: 'REQUEST_REVIEW_ARTIFACT_NOT_FOUND', message: 'x' }
    })
    const gone = renderHook(() => useReviewArtifact('r1', 'zz'))
    await waitFor(() => expect(gone.result.current.status).toBe('not_found'))
  })

  it('keeps at most eight drawings', async () => {
    callRequestRpc.mockImplementation(async (_m: string, p: { artifactId: string }) =>
      answer(p.artifactId)
    )
    for (let i = 0; i < ARTIFACT_CACHE_SIZE + 2; i++) {
      const h = renderHook(() => useReviewArtifact('r1', `a${i}`))
      await waitFor(() => expect(h.result.current.status).toBe('ready'))
      h.unmount()
    }
    callRequestRpc.mockClear()
    const oldest = renderHook(() => useReviewArtifact('r1', 'a0'))
    expect(oldest.result.current.status).toBe('loading')
    const newest = renderHook(() => useReviewArtifact('r1', `a${ARTIFACT_CACHE_SIZE + 1}`))
    expect(newest.result.current.status).toBe('ready')
  })

  it('an answer without the drawing is an error, not an empty frame', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: { artifact: { artifactId: 'a9' } } })
    const { result } = renderHook(() => useReviewArtifact('r1', 'a9'))
    await waitFor(() => expect(result.current.status).toBe('error'))
    expect(result.current.html).toBeNull()
  })
})
