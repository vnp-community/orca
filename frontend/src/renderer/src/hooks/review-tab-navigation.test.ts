import { beforeEach, describe, expect, it, vi } from 'vitest'

// FE-CV-TASK-050-18: per-module cases for the Review tab type in tab navigation helpers.
const h = vi.hoisted(() => ({ store: {} as Record<string, unknown> }))
vi.mock('../store', () => ({ useAppStore: { getState: () => h.store } }))
vi.mock('@/store', () => ({ useAppStore: { getState: () => h.store } }))
vi.mock('@/lib/worktree-runtime-owner', () => ({ getRuntimeEnvironmentIdForWorktree: () => null }))
vi.mock('@/runtime/web-runtime-session', () => ({
  activateWebRuntimeSessionTab: vi.fn(),
  isWebRuntimeSessionActive: () => false
}))

import { activateCyclableTab } from './ipc-tab-switch'
import { resolveZoomTarget } from './resolve-zoom-target'
import { resolveModalReturnFocusAction } from './modal-return-focus-action'
import { activateTabNumberShortcut } from '../lib/tab-number-shortcuts'
import {
  getActiveEntityIdForTabType,
  getNextTabAcrossAllTypes
} from '../components/terminal/tab-type-cycle'

function actions() {
  return {
    setActiveTab: vi.fn(),
    setActiveFile: vi.fn(),
    setActiveBrowserTab: vi.fn(),
    activateTab: vi.fn(),
    setActiveTabType: vi.fn(),
    focusGroup: vi.fn()
  }
}

beforeEach(() => {
  h.store = {}
})

describe('review tab navigation', () => {
  it('cycling onto a review tab activates its unified id and the review type', () => {
    const store = actions()
    activateCyclableTab(store as never, { type: 'review', id: 'review-1', tabId: 'review-1' })
    expect(store.setActiveTab).toHaveBeenCalledWith('review-1')
    expect(store.activateTab).toHaveBeenCalledWith('review-1')
    expect(store.setActiveTabType).toHaveBeenCalledWith('review')
    expect(store.setActiveFile).not.toHaveBeenCalled()
  })

  it('cycles across types from a review tab using the unified tab id', () => {
    const tabs = [
      { type: 'terminal' as const, id: 't1', tabId: 'u-t1' },
      { type: 'review' as const, id: 'review-1', tabId: 'review-1' },
      { type: 'editor' as const, id: 'f1', tabId: 'u-f1' }
    ]
    expect(getActiveEntityIdForTabType('review', 'review-1', 'f1', null)).toBe('review-1')
    const next = getNextTabAcrossAllTypes({
      tabs,
      activeTabType: 'review',
      activeTabId: 'review-1',
      activeFileId: null,
      activeBrowserTabId: null,
      direction: 1
    })
    expect(next?.id).toBe('f1')
  })

  it('number shortcuts open the review tab without touching the active file', () => {
    const store = actions()
    const review = {
      id: 'review-1',
      entityId: 'wt-1',
      groupId: 'g1',
      worktreeId: 'wt-1',
      contentType: 'review',
      label: 'Review',
      customLabel: null,
      color: null,
      sortOrder: 0,
      createdAt: 0
    }
    h.store = {
      ...store,
      activeView: 'terminal',
      activeWorktreeId: 'wt-1',
      activeGroupIdByWorktree: { 'wt-1': 'g1' },
      groupsByWorktree: {
        'wt-1': [{ id: 'g1', worktreeId: 'wt-1', activeTabId: null, tabOrder: ['review-1'] }]
      },
      unifiedTabsByWorktree: { 'wt-1': [review] },
      repos: [],
      settings: { activeRuntimeEnvironmentId: null },
      worktreesByRepo: { r: [{ id: 'wt-1', repoId: 'r' }] }
    }
    expect(activateTabNumberShortcut(0)).toBe(true)
    expect(store.focusGroup).toHaveBeenCalledWith('wt-1', 'g1')
    expect(store.activateTab).toHaveBeenCalledWith('review-1')
    expect(store.setActiveTabType).toHaveBeenCalledWith('review')
    expect(store.setActiveFile).not.toHaveBeenCalled()
  })

  it('zoom in a review tab targets the UI, not the editor', () => {
    expect(
      resolveZoomTarget({ activeView: 'terminal', activeTabType: 'review', activeElement: null })
    ).toBe('ui')
  })

  it('returns focus to the review surface after a modal closes', () => {
    const surface = {
      tabType: 'review' as const,
      worktreeId: 'wt-1',
      browserPageId: null,
      browserTarget: 'page' as never,
      terminalTabId: null,
      terminalLeafId: null
    }
    expect(resolveModalReturnFocusAction(surface)).toEqual({ kind: 'surface' })
    expect(resolveModalReturnFocusAction({ ...surface, worktreeId: null })).toEqual({
      kind: 'none'
    })
  })
})
