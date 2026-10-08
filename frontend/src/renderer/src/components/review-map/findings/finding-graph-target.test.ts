import { describe, expect, it, vi } from 'vitest'
import { openFindingInGraph, resolveFindingGraphTarget } from './finding-graph-target'
import { makeFinding } from '../../../test-support/contract-findings-fixtures'

const withSymbol = (key: string) => [{ path: 'a.go', line: 3, symbol: { key, kind: 'function', name: 'f', filePath: 'a.go' } as never }]
const ctx = (lenses: string[], keys: string[] | null = ['sym1']) => ({
  availableLensIds: new Set(lenses),
  graphSymbolKeys: keys ? new Set(keys) : null
})

describe('resolveFindingGraphTarget', () => {
  it('layer violations go to structure when the symbol is drawn, else impact, else null', () => {
    const f = makeFinding({ kind: 'layer_violation', evidence: withSymbol('sym1') })
    expect(resolveFindingGraphTarget(f, ctx(['structure', 'impact']))).toEqual({ lens: 'structure', symbolKey: 'sym1' })
    expect(resolveFindingGraphTarget(f, ctx(['impact']))).toEqual({ lens: 'impact', symbolKey: 'sym1' })
    expect(resolveFindingGraphTarget(f, ctx(['erd']))).toBeNull()
    expect(resolveFindingGraphTarget(f, ctx(['structure'], ['other']))).toBeNull()
    expect(resolveFindingGraphTarget(f, ctx(['structure'], null))).toBeNull()
  })

  it('hotspot and dead code go to structure only', () => {
    const f = makeFinding({ kind: 'hotspot', evidence: [{ path: 'a.go' }] })
    expect(resolveFindingGraphTarget(f, ctx(['structure']))).toEqual({ lens: 'structure', symbolKey: null })
    expect(resolveFindingGraphTarget(makeFinding({ kind: 'dead_code' }), ctx(['impact']))).toBeNull()
  })

  it('tenant / rls findings need params.table and the erd lens', () => {
    const f = makeFinding({ kind: 'missing_tenant_id', params: { table: 'orders' }, scope: { service: 'svc' } })
    expect(resolveFindingGraphTarget(f, ctx(['erd']))).toEqual({ lens: 'erd', table: 'orders', service: 'svc' })
    expect(resolveFindingGraphTarget({ ...f, params: {} }, ctx(['erd']))).toBeNull()
    expect(resolveFindingGraphTarget(f, ctx(['structure']))).toBeNull()
  })

  it('returns null for unknown kinds', () => {
    expect(resolveFindingGraphTarget(makeFinding({ kind: 'novel' as never }), ctx(['structure', 'erd']))).toBeNull()
  })
})

describe('openFindingInGraph', () => {
  const actions = () => ({
    setReviewLens: vi.fn(),
    selectReviewSymbol: vi.fn(),
    setErdService: vi.fn(),
    selectErdTable: vi.fn()
  })

  it('switches lens and selects the symbol', () => {
    const a = actions()
    openFindingInGraph('wt', { lens: 'structure', symbolKey: 's' }, a)
    expect(a.setReviewLens).toHaveBeenCalledWith('wt', 'structure')
    expect(a.selectReviewSymbol).toHaveBeenCalledWith('wt', 's')
    expect(a.selectErdTable).not.toHaveBeenCalled()
  })

  it('opens the erd table with its service', () => {
    const a = actions()
    openFindingInGraph('wt', { lens: 'erd', table: 'orders', service: 'svc' }, a)
    expect(a.setErdService).toHaveBeenCalledWith('wt', 'svc')
    expect(a.selectErdTable).toHaveBeenCalledWith('wt', 'orders')
  })
})
