import { describe, expect, it } from 'vitest'
import type { Tab, TabGroup } from '../../../../shared/types'
import { getGroupVisibleTabOrder } from './group-tab-order'
import { reconcileTabOrder } from './reconcile-order'
import { getActiveEntityIdForTabType } from '../terminal/tab-type-cycle'
import { resolveZoomTarget } from '../../hooks/resolve-zoom-target'
import { resolveModalReturnFocusAction } from '../../hooks/modal-return-focus-action'

// FE-CV-TASK-050-17/18: the review tab participates in tab order, cycling, zoom and focus return.

function tab(id: string, contentType: Tab['contentType'], entityId = id): Tab {
  return {
    id,
    entityId,
    groupId: 'g1',
    worktreeId: 'wt',
    contentType,
    label: id,
    customLabel: null,
    color: null,
    sortOrder: 0,
    createdAt: 0
  }
}

describe('group-tab-order with a review tab', () => {
  it('keeps the review tab in strip order without needing a backing entity', () => {
    const group: TabGroup = {
      id: 'g1',
      worktreeId: 'wt',
      activeTabId: 'r1',
      tabOrder: ['t1', 'r1', 'e1']
    }
    const tabs = [tab('t1', 'terminal'), tab('r1', 'review', 'wt'), tab('e1', 'editor', 'file-1')]
    expect(
      getGroupVisibleTabOrder(group, tabs, new Set(['t1']), new Set(['file-1']), new Set())
    ).toEqual([
      { type: 'terminal', id: 't1', tabId: 't1' },
      { type: 'review', id: 'r1', tabId: 'r1' },
      { type: 'editor', id: 'file-1', tabId: 'e1' }
    ])
  })
})

describe('reconcileTabOrder with review ids', () => {
  it('appends unseen review tabs after existing items and honours stored order', () => {
    expect(reconcileTabOrder(['r1', 't1'], ['t1'], [], [], [], ['r1'])).toEqual(['r1', 't1'])
    expect(reconcileTabOrder(undefined, ['t1'], ['e1'], [], [], ['r1'])).toEqual(['t1', 'e1', 'r1'])
    expect(reconcileTabOrder(['gone', 't1'], ['t1'], [], [], [], [])).toEqual(['t1'])
  })
})

describe('tab type cycle helpers', () => {
  it('review is identified by the unified tab id', () => {
    expect(getActiveEntityIdForTabType('review', 'unified-1', 'file-1', 'browser-1')).toBe('unified-1')
  })
})

describe('zoom target', () => {
  it('review zoom belongs to the app UI, not the terminal or editor', () => {
    expect(
      resolveZoomTarget({ activeView: 'terminal', activeTabType: 'review', activeElement: null })
    ).toBe('ui')
    // even if an xterm textarea was the last focused element
    expect(
      resolveZoomTarget({
        activeView: 'terminal',
        activeTabType: 'review',
        activeElement: { classList: { contains: () => true } }
      })
    ).toBe('ui')
  })
})

describe('modal return focus', () => {
  it('returns focus to the surface for a review tab', () => {
    expect(
      resolveModalReturnFocusAction({
        tabType: 'review',
        worktreeId: 'wt',
        browserPageId: null,
        browserTarget: 'webview',
        terminalTabId: null,
        terminalLeafId: null
      })
    ).toEqual({ kind: 'surface' })
  })
})
