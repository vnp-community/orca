import { describe, expect, it } from 'vitest'
import type { C4Component, C4External, C4Relation } from '../../../../../shared/code-intel-architecture-types'
import { layoutC4Layers, C4_NODE_WIDTH, C4_COLUMN_GAP, c4LayerForComponentKind } from './c4-layer-layout'
import { c4EdgeStyle, c4StrokeWidth, C4_RELATION_DASH, C4_RELATION_KINDS } from './c4-edge-style'
import { computeC4OverlayFlags, computeC4RelationFlags, normalizeRepoPath } from './c4-overlay-model'
import { pickDefaultContainer } from './c4-container-default'

const comp = (id: string, kind: string, over: Partial<C4Component> = {}): C4Component => ({
  id,
  name: id,
  kind: kind as C4Component['kind'],
  path: `svc/${id}`,
  descriptionSource: 'none',
  symbolCount: 1,
  origin: 'derived',
  packagePaths: [`svc/${id}`],
  hidden: false,
  ...over
})
const rel = (from: string, to: string, over: Partial<C4Relation> = {}): C4Relation => ({
  from,
  to,
  kind: 'uses',
  evidence: [],
  count: 1,
  origin: 'derived',
  confidence: 1,
  violatesLayering: false,
  ...over
})
const ext: C4External = { id: 'pg', name: 'postgres', kind: 'database', origin: 'derived' }
const colOf = (x: number): number => x / (C4_NODE_WIDTH + C4_COLUMN_GAP)

describe('layoutC4Layers', () => {
  it('places components in hexagonal columns and externals in column 4', () => {
    const l = layoutC4Layers({
      components: [
        comp('srv', 'grpc-server'),
        comp('uc', 'usecase'),
        comp('dom', 'domain'),
        comp('ad', 'adapter'),
        comp('cl', 'grpc-client'),
        comp('cfg', 'config'),
        comp('wat', 'weird-future-kind')
      ],
      externals: [ext],
      relations: []
    })
    const col = Object.fromEntries(l.nodes.map((n) => [n.id, colOf(n.x)]))
    expect(col).toMatchObject({ srv: 0, uc: 1, dom: 2, ad: 3, cl: 3, pg: 4, cfg: 1, wat: 1 })
    const uc = l.nodes.find((n) => n.id === 'uc')!
    const cfg = l.nodes.find((n) => n.id === 'cfg')!
    expect(cfg.y).toBeGreaterThan(uc.y)
  })

  it('omits empty bands and the support area has no band', () => {
    const l = layoutC4Layers({ components: [comp('uc', 'usecase'), comp('o', 'other')], externals: [], relations: [] })
    expect(l.bands.map((b) => b.layer)).toEqual(['usecase'])
  })

  it('drops relations to missing nodes without throwing', () => {
    const l = layoutC4Layers({
      components: [comp('a', 'usecase'), comp('b', 'domain')],
      externals: [],
      relations: [rel('a', 'b'), rel('a', 'ghost')]
    })
    expect(l.relations).toHaveLength(1)
    expect(l.droppedRelations).toBe(1)
  })

  it('is deterministic regardless of input order and handles 150 nodes', () => {
    const items = Array.from({ length: 150 }, (_, i) => comp(`c${String(i).padStart(3, '0')}`, ['usecase', 'domain', 'adapter'][i % 3]))
    const a = layoutC4Layers({ components: items, externals: [], relations: [] })
    const b = layoutC4Layers({ components: [...items].toReversed(), externals: [], relations: [] })
    expect(a.nodes).toHaveLength(150)
    expect(b.nodes.map((n) => [n.id, n.x, n.y])).toEqual(a.nodes.map((n) => [n.id, n.x, n.y]))
  })

  it('maps kinds to layers', () => {
    expect(c4LayerForComponentKind('grpc-client')).toBe('adapter')
    expect(c4LayerForComponentKind('nope')).toBe('support')
  })
})

describe('c4EdgeStyle', () => {
  it('clamps strokeWidth to 1..5 on a log scale', () => {
    expect(c4StrokeWidth(1)).toBe(1)
    expect(c4StrokeWidth(2)).toBe(2)
    expect(c4StrokeWidth(8)).toBe(4)
    expect(c4StrokeWidth(1000)).toBe(5)
    expect(c4StrokeWidth(0)).toBe(1)
    expect(c4StrokeWidth(Number.NaN)).toBe(1)
  })
  it('gives each of the 7 kinds a distinct pattern', () => {
    expect(C4_RELATION_KINDS).toHaveLength(7)
    expect(new Set(Object.values(C4_RELATION_DASH)).size).toBe(7)
  })
  it('fades low-confidence edges and flags violations', () => {
    const s = c4EdgeStyle(rel('a', 'b', { confidence: 0.5, violatesLayering: true }))
    expect(s).toMatchObject({ lowConfidence: true, opacity: 0.5, violation: true, strokeDasharray: '3 3' })
    expect(c4EdgeStyle(rel('a', 'b', { kind: 'calls-rpc' })).strokeDasharray).toBe(C4_RELATION_DASH['calls-rpc'])
  })
})

describe('overlay flags', () => {
  const overlay = {
    changedFiles: [{ path: 'svc\\uc\\x.go', status: 'modified' as const }],
    uncoveredSymbols: [{ key: 'k1', kind: 'func', name: 'f', filePath: 'svc/dom/y.go' }],
    violations: [{ findingKey: 'f', rule: 'r', severity: 'error', file: 'svc/ad/z.go', status: 'touched' }],
    changedSymbols: [{ symbol: { key: 'S1', kind: 'func', name: 'a', filePath: 'a' }, changeKind: 'modified', tested: 'no' }]
  }
  it('matches by path prefix with Windows separators', () => {
    expect(computeC4OverlayFlags(comp('uc', 'usecase'), overlay).changed).toBe(true)
    expect(computeC4OverlayFlags(comp('ucx', 'usecase'), overlay).changed).toBe(false)
    expect(computeC4OverlayFlags(comp('dom', 'domain'), overlay)).toMatchObject({ untested: true, changed: false })
    expect(computeC4OverlayFlags(comp('ad', 'adapter'), overlay).violation).toBe(true)
  })
  it('tries container-relative paths and only sets affected with impact data', () => {
    const c = comp('uc', 'usecase', { path: 'uc', packagePaths: ['uc'] })
    const o = { ...overlay, changedFiles: [{ path: 'backend/svc/uc/a.go', status: 'added' as const }] }
    expect(computeC4OverlayFlags(c, o, null, 'backend/svc').changed).toBe(true)
    expect(computeC4OverlayFlags(c, o).affected).toBe(false)
    expect(computeC4OverlayFlags(c, o, { affectedFiles: ['uc/q.go'] }).affected).toBe(true)
  })
  it('flags edges via evidence keys', () => {
    const ev = [{ key: 'S1', kind: 'func', name: 'a', filePath: 'a' }] as unknown as C4Relation["evidence"]
    expect(computeC4RelationFlags(rel('a', 'b', { evidence: ev }), overlay).touchesChange).toBe(true)
    expect(computeC4RelationFlags(rel('a', 'b'), overlay).touchesChange).toBe(false)
  })
  it('normalizes paths', () => {
    expect(normalizeRepoPath('.\\a\\b\\')).toBe('a/b')
  })
})

describe('pickDefaultContainer', () => {
  const cs = [
    { id: 'b', name: 'beta', path: 'svc/beta', kind: 'service' as const },
    { id: 'a', name: 'alpha', path: 'svc/alpha', kind: 'service' as const }
  ]
  it('picks the container with the most changed files', () => {
    expect(pickDefaultContainer(cs, ['svc/beta/a.go', 'svc/beta/b.go', 'svc/alpha/c.go'])?.id).toBe('b')
  })
  it('breaks ties by name', () => {
    expect(pickDefaultContainer(cs, ['svc/beta/a.go', 'svc/alpha/c.go'])?.id).toBe('a')
  })
  it('falls back to the first container, or null when empty', () => {
    expect(pickDefaultContainer(cs, ['docs/x.md'])?.id).toBe('b')
    expect(pickDefaultContainer([], ['x'])).toBeNull()
  })
})
