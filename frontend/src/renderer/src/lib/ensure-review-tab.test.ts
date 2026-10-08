import { beforeEach, describe, expect, it, vi } from 'vitest'

const mockStoreState = vi.hoisted(() => ({
  activeGroupIdByWorktree: {} as Record<string, string>,
  activeWorktreeId: 'wt-1',
  activateTab: vi.fn(),
  createUnifiedTab: vi.fn(),
  createUnifiedTabInSplit: vi.fn(),
  focusGroup: vi.fn(),
  groupsByWorktree: {} as Record<string, { id: string }[]>,
  layoutByWorktree: {} as Record<string, unknown>,
  setActiveTabType: vi.fn(),
  codeIntelSupportState: { state: 'enabled' } as { state: string },
  unifiedTabsByWorktree: {} as Record<string, { id: string; groupId: string; contentType: string }[]>
}))
const selector = vi.hoisted(() => ({ value: { state: 'ready' } as { state: string } }))

vi.mock('@/store', () => ({ useAppStore: { getState: () => mockStoreState } }))
vi.mock('./code-intel-worktree-selector', () => ({
  resolveCodeIntelSelector: () => selector.value
}))

import { ensureReviewTab, getReviewTabForWorktree } from './ensure-review-tab'

beforeEach(() => {
  mockStoreState.activeGroupIdByWorktree = { 'wt-1': 'group-1' }
  mockStoreState.activeWorktreeId = 'wt-1'
  mockStoreState.groupsByWorktree = { 'wt-1': [{ id: 'group-1' }] }
  mockStoreState.layoutByWorktree = { 'wt-1': { type: 'leaf', groupId: 'group-1' } }
  mockStoreState.unifiedTabsByWorktree = { 'wt-1': [] }
  mockStoreState.codeIntelSupportState = { state: 'enabled' }
  selector.value = { state: 'ready' }
  for (const fn of [
    mockStoreState.activateTab,
    mockStoreState.createUnifiedTab,
    mockStoreState.createUnifiedTabInSplit,
    mockStoreState.focusGroup,
    mockStoreState.setActiveTabType
  ]) {
    fn.mockReset()
  }
  mockStoreState.createUnifiedTab.mockReturnValue({ id: 'review-new', groupId: 'group-1' })
})

describe('ensureReviewTab creation and reuse', () => {
  it('creates a review tab with entityId = worktreeId and surfaces it', () => {
    expect(ensureReviewTab('wt-1')).toBe('review-new')
    expect(mockStoreState.createUnifiedTab).toHaveBeenCalledWith(
      'wt-1',
      'review',
      expect.objectContaining({ entityId: 'wt-1', targetGroupId: 'group-1', activate: true })
    )
    expect(mockStoreState.setActiveTabType).toHaveBeenCalledWith('review')
    expect(mockStoreState.focusGroup).toHaveBeenCalledWith('wt-1', 'group-1')
  })

  it('reuses the existing tab instead of creating a duplicate', () => {
    mockStoreState.unifiedTabsByWorktree = {
      'wt-1': [{ id: 'review-1', groupId: 'group-1', contentType: 'review' }]
    }
    expect(ensureReviewTab('wt-1')).toBe('review-1')
    expect(mockStoreState.createUnifiedTab).not.toHaveBeenCalled()
    expect(mockStoreState.activateTab).toHaveBeenCalledWith('review-1')
    expect(mockStoreState.setActiveTabType).toHaveBeenCalledWith('review')
    expect(getReviewTabForWorktree('wt-1')?.id).toBe('review-1')
  })

  it('does not surface when surfacePane is false', () => {
    expect(ensureReviewTab('wt-1', { surfacePane: false })).toBe('review-new')
    expect(mockStoreState.createUnifiedTab).toHaveBeenCalledWith(
      'wt-1',
      'review',
      expect.objectContaining({ activate: false })
    )
    expect(mockStoreState.activateTab).not.toHaveBeenCalled()
    expect(mockStoreState.setActiveTabType).not.toHaveBeenCalled()
  })

  it('honours targetGroupId', () => {
    ensureReviewTab('wt-1', { targetGroupId: 'group-9' })
    expect(mockStoreState.createUnifiedTab).toHaveBeenCalledWith(
      'wt-1',
      'review',
      expect.objectContaining({ targetGroupId: 'group-9' })
    )
  })
})

describe('ensureReviewTab rightSplit', () => {
  it('splits right when no reusable right group exists', () => {
    mockStoreState.createUnifiedTabInSplit.mockReturnValue({ id: 'review-split', groupId: 'group-2' })
    expect(ensureReviewTab('wt-1', { placement: 'rightSplit' })).toBe('review-split')
    expect(mockStoreState.createUnifiedTabInSplit).toHaveBeenCalledWith(
      'wt-1',
      'review',
      { sourceGroupId: 'group-1', splitDirection: 'right' },
      expect.objectContaining({ entityId: 'wt-1', activate: true })
    )
    expect(mockStoreState.createUnifiedTab).not.toHaveBeenCalled()
  })

  it('reuses an existing right group', () => {
    mockStoreState.layoutByWorktree = {
      'wt-1': {
        type: 'split',
        direction: 'horizontal',
        first: { type: 'leaf', groupId: 'group-1' },
        second: { type: 'leaf', groupId: 'group-2' },
        ratio: 0.5
      }
    }
    mockStoreState.createUnifiedTab.mockReturnValue({ id: 'review-r', groupId: 'group-2' })
    expect(ensureReviewTab('wt-1', { placement: 'rightSplit' })).toBe('review-r')
    expect(mockStoreState.createUnifiedTab).toHaveBeenCalledWith(
      'wt-1',
      'review',
      expect.objectContaining({ targetGroupId: 'group-2' })
    )
    expect(mockStoreState.createUnifiedTabInSplit).not.toHaveBeenCalled()
  })
})

describe('ensureReviewTab gating', () => {
  it.each(['disabled', 'unsupported'])('returns null when support is %s', (state) => {
    mockStoreState.codeIntelSupportState = { state }
    expect(ensureReviewTab('wt-1')).toBeNull()
    expect(mockStoreState.createUnifiedTab).not.toHaveBeenCalled()
  })

  it.each(['unknown', 'enabled'])('opens when support is %s', (state) => {
    mockStoreState.codeIntelSupportState = { state }
    expect(ensureReviewTab('wt-1')).toBe('review-new')
  })

  it('returns null when the selector is unsupported', () => {
    selector.value = { state: 'unsupported' }
    expect(ensureReviewTab('wt-1')).toBeNull()
  })

  it('returns null when there is no target group', () => {
    mockStoreState.activeGroupIdByWorktree = {}
    mockStoreState.groupsByWorktree = {}
    expect(ensureReviewTab('wt-1')).toBeNull()
  })
})
