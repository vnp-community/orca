import { describe, expect, it, vi } from 'vitest'
import {
  handleMockReviewSummaryRequest,
  MOCK_REVIEW_SUMMARY_SCENARIOS
} from '../../scripts/mock-server-review-summary-data'
import type { RpcResponse } from '../transport/types'
import {
  loadMobileReviewSummary,
  MOBILE_REVIEW_SUMMARY_HOST_UNSUPPORTED
} from './mobile-review-summary-loaders'
import { buildMobileReviewSummaryChip } from './mobile-review-summary-chip'

const success = (id: string, result: unknown) =>
  ({ id, ok: true, result, _meta: { runtimeId: 'mock' } }) as never
const failure = (id: string, code: string, message: string) =>
  ({ id, ok: false, error: { code, message }, _meta: { runtimeId: 'mock' } }) as never

// Runs the mobile mock-server handler through the real loader so the device
// scenarios stay in sync with the parser the screen uses.
async function loadScenario(scenario: string) {
  let response: RpcResponse | null = null
  const handled = handleMockReviewSummaryRequest(
    { id: '1', method: 'codeIntel.reviewSummary', params: { worktree: 'id:w' } },
    (r) => {
      response = r as RpcResponse
    },
    success,
    failure,
    scenario
  )
  expect(handled).toBe(true)
  const client = { sendRequest: vi.fn(async () => response!) }
  return loadMobileReviewSummary(client as never, 'w')
}

describe('mobile mock-server codeIntel.reviewSummary', () => {
  it('ignores other methods', () => {
    const respond = vi.fn()
    expect(
      handleMockReviewSummaryRequest({ id: '1', method: 'git.status' }, respond, success, failure)
    ).toBe(false)
    expect(respond).not.toHaveBeenCalled()
  })

  it('every scenario maps to a screen state the loader understands', async () => {
    const kinds: Record<string, string> = {}
    for (const scenario of MOCK_REVIEW_SUMMARY_SCENARIOS) {
      kinds[scenario] = (await loadScenario(scenario)).kind
    }
    expect(kinds).toEqual({
      ready: 'ready',
      stale: 'ready',
      truncated: 'ready',
      empty: 'ready',
      flag_off: 'unavailable',
      no_binding: 'unavailable',
      index_missing: 'unavailable',
      tool_unavailable: 'unavailable',
      unsupported: 'unavailable',
      error: 'error'
    })
  })

  it('ready data parses fully and drives the branch-card chip', async () => {
    const state = await loadScenario('truncated')
    expect(state.kind === 'ready' && state.summary.findings?.items).toHaveLength(3)
    expect(state.kind === 'ready' && state.summary.truncated).toBe(true)
    expect(buildMobileReviewSummaryChip(state)).toMatchObject({ tone: 'danger' })
    expect(await loadScenario('unsupported')).toEqual({
      kind: 'unavailable',
      message: MOBILE_REVIEW_SUMMARY_HOST_UNSUPPORTED
    })
    expect(buildMobileReviewSummaryChip(await loadScenario('flag_off'))).toBeNull()
  })

  it('falls back to the ready scenario for an unknown env value', async () => {
    expect((await loadScenario('bogus')).kind).toBe('ready')
  })
})
