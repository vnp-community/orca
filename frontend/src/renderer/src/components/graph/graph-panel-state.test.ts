// @vitest-environment happy-dom
import { describe, expect, it } from 'vitest'
import {
  createGraphPanelState, filterGraphNodes, graphPanelReducer, initialView, selectNodeFromSearch, shouldOpenSearchOnKey
} from './graph-panel-state'
import { gNode } from './graph-test-fixtures'

describe('graphPanelReducer', () => {
  it('changing the lens clears focus, selection and open groups', () => {
    let s = createGraphPanelState('impact')
    s = graphPanelReducer(s, { type: 'select', id: 'a' })
    s = graphPanelReducer(s, { type: 'focus', id: 'a' })
    s = graphPanelReducer(s, { type: 'toggleGroup', id: 'g' })
    s = graphPanelReducer(s, { type: 'lens', lens: 'data' })
    expect(s).toMatchObject({ lens: 'data', selectedId: null, focusId: null })
    expect(s.openGroups.size).toBe(0)
  })

  it('switching view keeps the selection; toggling a group twice closes it', () => {
    let s = graphPanelReducer(createGraphPanelState('impact'), { type: 'select', id: 'a' })
    s = graphPanelReducer(s, { type: 'view', view: 'list' })
    expect(s.selectedId).toBe('a')
    s = graphPanelReducer(s, { type: 'toggleGroup', id: 'g' })
    s = graphPanelReducer(s, { type: 'toggleGroup', id: 'g' })
    expect(s.openGroups.has('g')).toBe(false)
  })
})

describe('initialView', () => {
  it('uses the list when truncated, narrow or in screen-reader mode', () => {
    expect(initialView({ truncated: false, narrow: false })).toBe('graph')
    expect(initialView({ truncated: true, narrow: false })).toBe('list')
    expect(initialView({ truncated: false, narrow: true })).toBe('list')
    expect(initialView({ truncated: false, narrow: false, screenReaderMode: true })).toBe('list')
  })
})

describe('filterGraphNodes', () => {
  const nodes = [gNode('1', { label: 'Đăng nhập', kind: 'rpc', group: 'auth' }), gNode('2', { label: 'Billing', kind: 'table', group: 'pay' })]
  it('matches label, kind and group case- and diacritic-insensitively', () => {
    expect(filterGraphNodes(nodes, 'dang NHAP').map((n) => n.id)).toEqual(['1'])
    expect(filterGraphNodes(nodes, 'TABLE').map((n) => n.id)).toEqual(['2'])
    expect(filterGraphNodes(nodes, 'auth').map((n) => n.id)).toEqual(['1'])
    expect(filterGraphNodes(nodes, '')).toHaveLength(2)
  })
  it('caps results', () => {
    const many = Array.from({ length: 80 }, (_, i) => gNode(`n${i}`))
    expect(filterGraphNodes(many, '')).toHaveLength(50)
  })
})

describe('selectNodeFromSearch / keys', () => {
  it('opens the containing group, selects and bumps the fit signal', () => {
    const s = selectNodeFromSearch({ ...createGraphPanelState('impact'), searchOpen: true }, gNode('x', { group: 'g' }))
    expect(s.openGroups.has('g')).toBe(true)
    expect(s).toMatchObject({ selectedId: 'x', fitViewSignal: 1, searchOpen: false })
  })

  it('ignores "/" typed into fields or with modifiers', () => {
    const input = document.createElement('input')
    expect(shouldOpenSearchOnKey({ key: '/', target: input })).toBe(false)
    expect(shouldOpenSearchOnKey({ key: '/', target: document.body })).toBe(true)
    expect(shouldOpenSearchOnKey({ key: '/', target: document.body, ctrlKey: true })).toBe(false)
    expect(shouldOpenSearchOnKey({ key: 'a', target: document.body })).toBe(false)
  })
})

import { lensDisabledReason } from './graph-panel-state'

describe('lensDisabledReason', () => {
  const ok = { backendUnsupported: false, hasPlan: true, executing: true }
  it('always enables flow and gives a reason for the others', () => {
    expect(lensDisabledReason('flow', { ...ok, backendUnsupported: true, hasPlan: false })).toBeNull()
    expect(lensDisabledReason('plan', { ...ok, hasPlan: false })).toBe('noPlan')
    expect(lensDisabledReason('execution', { ...ok, executing: false })).toBe('notExecuting')
    expect(lensDisabledReason('impact', { ...ok, impactAssessed: false })).toBe('noAssessment')
    expect(lensDisabledReason('impact', { ...ok, backendUnsupported: true })).toBe('unsupported')
    expect(lensDisabledReason('data', ok)).toBeNull()
  })
})
