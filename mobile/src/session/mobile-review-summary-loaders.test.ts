import { describe, expect, it, vi } from 'vitest'
import type { RpcResponse } from '../transport/types'
import {
  loadMobileReviewSummary,
  MOBILE_REVIEW_SUMMARY_HOST_UNSUPPORTED
} from './mobile-review-summary-loaders'

function ok(result: unknown): RpcResponse {
  return { id: 'x', ok: true, result, _meta: { runtimeId: 'r' } }
}
function fail(code: string, message: string): RpcResponse {
  return {
    id: 'x',
    ok: false,
    error: { code, message },
    _meta: { runtimeId: 'r' }
  }
}
function client(response: RpcResponse | Error) {
  const sendRequest = vi.fn(async () => {
    if (response instanceof Error) {
      throw response
    }
    return response
  })
  return { client: { sendRequest } as never, sendRequest }
}

describe('loadMobileReviewSummary', () => {
  it('sends the host method with an id: worktree selector', async () => {
    const c = client(ok({ available: true }))
    const state = await loadMobileReviewSummary(c.client, 'wt-1')
    expect(c.sendRequest).toHaveBeenCalledWith('codeIntel.reviewSummary', {
      worktree: 'id:wt-1'
    })
    expect(state.kind).toBe('ready')
  })

  it.each([
    ['flag_off', /turned off/],
    ['no_binding', /not linked/],
    ['index_missing', /No code index/],
    ['tool_unavailable', /tools are unavailable/]
  ])('maps reason %s to an unavailable message', async (reason, pattern) => {
    const state = await loadMobileReviewSummary(
      client(ok({ available: false, reason })).client,
      'wt'
    )
    expect(state.kind).toBe('unavailable')
    expect(state.kind === 'unavailable' && state.message).toMatch(pattern)
  })

  it('uses a generic message when available=false has no reason', async () => {
    const state = await loadMobileReviewSummary(client(ok({ available: false })).client, 'wt')
    expect(state).toEqual({
      kind: 'unavailable',
      message: 'Review summary is not available right now.'
    })
  })

  it.each([
    ['forbidden', 'nope'],
    ['method_not_found', 'x'],
    ['other', 'Method is not available to mobile clients']
  ])('treats %s as an unsupported host', async (code, message) => {
    const state = await loadMobileReviewSummary(client(fail(code, message)).client, 'wt')
    expect(state).toEqual({
      kind: 'unavailable',
      message: MOBILE_REVIEW_SUMMARY_HOST_UNSUPPORTED
    })
  })

  it('maps other failures, thrown errors and invalid payloads to error', async () => {
    expect(await loadMobileReviewSummary(client(fail('failed', 'boom')).client, 'wt')).toEqual({
      kind: 'error',
      message: 'boom'
    })
    expect(await loadMobileReviewSummary(client(new Error('offline')).client, 'wt')).toEqual({
      kind: 'error',
      message: 'offline'
    })
    expect((await loadMobileReviewSummary(client(ok('junk')).client, 'wt')).kind).toBe('error')
  })
})
