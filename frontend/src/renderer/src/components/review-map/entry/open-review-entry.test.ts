import { beforeEach, describe, expect, it, vi } from 'vitest'

const calls: string[] = []
const state = {
  codeIntelSupportState: { state: 'enabled' as string },
  activeWorktreeId: 'other' as string | null,
  reviewUiByWorktree: {} as Record<string, { chipFilter: string | null }>,
  setReviewLens: vi.fn((_w: string, lens: string) => calls.push(`lens:${lens}`)),
  toggleReviewChipFilter: vi.fn((_w: string, chip: string) => calls.push(`chip:${chip}`))
}
const activate = vi.fn((_id: string) => {
  calls.push('activate')
  return { primaryTabId: null } as { primaryTabId: null } | false
})
const ensure = vi.fn((_id: string) => {
  calls.push('ensure')
  return 'tab1' as string | null
})

vi.mock('@/store', () => ({ useAppStore: { getState: () => state } }))
vi.mock('@/lib/worktree-activation', () => ({
  activateAndRevealWorktree: (id: string) => activate(id)
}))
vi.mock('@/lib/ensure-review-tab', () => ({ ensureReviewTab: (id: string) => ensure(id) }))

import { openReviewFromEntryPoint } from './open-review-entry'
import { takeReviewOpenSource } from '@/lib/review-open-source'

describe('openReviewFromEntryPoint', () => {
  beforeEach(() => {
    calls.length = 0
    state.codeIntelSupportState = { state: 'enabled' }
    state.activeWorktreeId = 'other'
    state.reviewUiByWorktree = {}
    vi.clearAllMocks()
  })

  it('activates before ensuring the tab, then sets lens and filter', () => {
    expect(openReviewFromEntryPoint('wt', 'cmd-k', { lens: 'erd', filter: 'untested' })).toBe(true)
    expect(calls).toEqual(['activate', 'ensure', 'lens:erd', 'chip:untested'])
  })

  it('skips activation when already active and does not retoggle an active chip', () => {
    state.activeWorktreeId = 'wt'
    state.reviewUiByWorktree = { wt: { chipFilter: 'untested' } }
    openReviewFromEntryPoint('wt', 'agent-row', { filter: 'untested' })
    expect(activate).not.toHaveBeenCalled()
    expect(state.toggleReviewChipFilter).not.toHaveBeenCalled()
  })

  it.each(['disabled', 'unknown', 'unsupported'])('does nothing when flag is %s', (s) => {
    state.codeIntelSupportState = { state: s }
    expect(openReviewFromEntryPoint('wt', 'agent-row')).toBe(false)
    expect(activate).not.toHaveBeenCalled()
    expect(ensure).not.toHaveBeenCalled()
  })

  it('returns false when ensureReviewTab refuses', () => {
    ensure.mockReturnValueOnce(null)
    expect(openReviewFromEntryPoint('wt', 'agent-row')).toBe(false)
  })

  it.each([
    ['agent-row', 'agent_row'],
    ['source-control', 'source_control'],
    ['cmd-k', 'cmd_k'],
    ['right-sidebar', 'right_sidebar'],
    ['notification', 'notification']
  ] as const)('records %s as the telemetry open source before the tab mounts', (entry, source) => {
    ensure.mockImplementationOnce((id: string) => {
      // The origin is already pending when the tab is created.
      expect(takeReviewOpenSource(id).source).toBe(source)
      return 'tab1'
    })
    openReviewFromEntryPoint('wt', entry)
  })

  it('marks afterAgentTurn from the completion and clears the origin when the tab is refused', () => {
    openReviewFromEntryPoint('wt', 'notification', { completionId: 'c1' })
    expect(takeReviewOpenSource('wt')).toEqual({ source: 'notification', afterAgentTurn: true })
    ensure.mockReturnValueOnce(null)
    openReviewFromEntryPoint('wt', 'cmd-k')
    expect(takeReviewOpenSource('wt')).toEqual({ source: 'restore', afterAgentTurn: false })
  })
})
