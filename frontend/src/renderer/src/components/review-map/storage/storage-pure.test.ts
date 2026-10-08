import { describe, expect, it } from 'vitest'
import {
  buildStorageViewModel,
  secretNodeId,
  serviceNodeId,
  storeNodeId
} from './storage-view-model'
import { computeStorageChangeMarks } from './storage-change-marks'
import { layoutStorageLanes } from './storage-layout'
import { sampleStorageMap } from './storage-map.fixture'

describe('buildStorageViewModel', () => {
  const vm = buildStorageViewModel(sampleStorageMap())
  const node = (id: string) => vm.nodes.find((n) => n.id === id)!

  it('puts services, stores, topics and secrets in their lanes', () => {
    expect(
      vm.nodes
        .filter((n) => n.lane === 'service')
        .map((n) => n.name)
        .sort()
    ).toEqual(['agent-gw', 'infra-fleet', 'notifier'])
    expect(
      vm.nodes
        .filter((n) => n.lane === 'store')
        .map((n) => n.name)
        .sort()
    ).toEqual(['mysql', 'postgres'])
    expect(vm.nodes.filter((n) => n.lane === 'topic')).toHaveLength(1)
    expect(vm.nodes.filter((n) => n.lane === 'secret')).toHaveLength(1)
  })

  it('shows a Vault binding as a secret carrying only key name and path', () => {
    const secret = node(secretNodeId('vault_ssh_role'))
    expect(secret).toMatchObject({ name: 'vault_ssh_role', vaultPath: 'secret/data/infra/ssh' })
    expect(secret.payload).toBeUndefined()
    const edge = vm.edges.find((e) => e.to === secret.id)!
    expect(edge).toMatchObject({ kind: 'config-key', from: serviceNodeId('infra-fleet') })
    expect(edge.via).toBeUndefined()
  })

  it('keeps every publisher/subscriber of a topic', () => {
    const topicId = 'topic:orca.infra.agent'
    expect(vm.edges.filter((e) => e.kind === 'publish' && e.to === topicId)).toHaveLength(2)
    expect(vm.edges.filter((e) => e.kind === 'subscribe' && e.from === topicId)).toHaveLength(1)
  })

  it('masks DSNs and tokens in free-form fields and flags them', () => {
    const binding = vm.edges.find(
      (e) => e.kind === 'binding' && e.from === serviceNodeId('infra-fleet')
    )!
    expect(binding.via).toBe('postgres://•••@db/app')
    expect(binding.masked).toBe(true)
    expect(node('topic:orca.infra.agent').payload).not.toContain('abc123')
    expect(node('topic:orca.infra.agent').masked).toBe(true)
  })

  it('maps unknown enums instead of throwing and keeps warnings as plain text', () => {
    const map = sampleStorageMap()
    map.bindings[0] = { ...map.bindings[0], access: 'weird' as never, confidence: 'weird' as never }
    const v = buildStorageViewModel(map)
    expect(v.edges[0]).toMatchObject({ access: 'unknown', confidence: 'unknown' })
    expect(v.warnings).toEqual([{ text: 'prod topology unknown', masked: false }])
    expect(v.redactedCount).toBe(2)
  })

  it('creates a placeholder store for a binding to an unknown store', () => {
    const map = sampleStorageMap()
    map.bindings.push({
      service: 'x',
      store: 'ghost',
      access: 'rw',
      via: '',
      evidence: [],
      confidence: 'inferred'
    })
    expect(buildStorageViewModel(map).nodes.some((n) => n.id === storeNodeId('ghost'))).toBe(true)
  })
})

describe('computeStorageChangeMarks', () => {
  const vm = buildStorageViewModel(sampleStorageMap())

  it('backend change wins over path intersection', () => {
    const map = sampleStorageMap()
    map.stores[0] = { ...map.stores[0], change: 'added' }
    const v = buildStorageViewModel(map)
    const { marks } = computeStorageChangeMarks(v, [{ path: 'deploy/pg.yml', status: 'modified' }])
    expect(marks.get(storeNodeId('pg'))).toBe('added')
  })

  it('intersects evidence with changed files and marks neighbours related', () => {
    const { marks, unknown } = computeStorageChangeMarks(vm, [
      { path: './deploy/my.yml', status: 'modified' }
    ])
    expect(marks.get(storeNodeId('my'))).toBe('modified')
    expect(marks.get(serviceNodeId('agent-gw'))).toBe('related')
    expect(marks.has(storeNodeId('pg'))).toBe(false)
    expect(unknown).toBe(false)
  })

  it('normalizes backslashes and maps deleted files to removed', () => {
    const { marks } = computeStorageChangeMarks(vm, [{ path: 'deploy\\my.yml', status: 'deleted' }])
    expect(marks.get(storeNodeId('my'))).toBe('removed')
  })

  it('marks a topic from its evidence only', () => {
    const { marks } = computeStorageChangeMarks(vm, [{ path: 'svc/infra/events.go' }])
    expect(marks.get('topic:orca.infra.agent')).toBe('modified')
  })

  it('reports unknown when no evidence exists anywhere, and nothing when nothing changed', () => {
    const bare = buildStorageViewModel(
      sampleStorageMap({
        stores: sampleStorageMap().stores.map((s) => ({ ...s, evidence: [] })),
        bindings: sampleStorageMap().bindings.map((b) => ({ ...b, evidence: [] })),
        topics: sampleStorageMap().topics.map((t) => ({ ...t, evidence: [] }))
      })
    )
    expect(computeStorageChangeMarks(bare, [{ path: 'a.go' }])).toMatchObject({ unknown: true })
    expect(computeStorageChangeMarks(bare, []).unknown).toBe(false)
    expect(computeStorageChangeMarks(vm, []).marks.size).toBe(0)
  })
})

describe('layoutStorageLanes', () => {
  const vm = buildStorageViewModel(sampleStorageMap())

  it('is deterministic with fixed lane x and no overlaps', () => {
    const a = layoutStorageLanes(vm)
    expect([...layoutStorageLanes(vm).positions]).toEqual([...a.positions])
    const xs = a.laneBounds.map((l) => l.x)
    expect(xs).toEqual([...xs].sort((p, q) => p - q))
    expect(a.laneBounds.map((l) => l.lane)).toEqual(['service', 'store', 'topic', 'secret'])
    const rects = [...a.positions.values()]
    for (let i = 0; i < rects.length; i++) {
      for (let j = i + 1; j < rects.length; j++) {
        const [p, q] = [rects[i], rects[j]]
        expect(
          p.x < q.x + q.width && q.x < p.x + p.width && p.y < q.y + q.height && q.y < p.y + p.height
        ).toBe(false)
      }
    }
  })

  it('skips empty lanes and honours the filter', () => {
    const noTopics = buildStorageViewModel(sampleStorageMap({ topics: [] }))
    expect(layoutStorageLanes(noTopics).laneBounds.map((l) => l.lane)).toEqual([
      'service',
      'store',
      'secret'
    ])
    const filtered = layoutStorageLanes({
      ...vm,
      filter: new Set([storeNodeId('my'), serviceNodeId('agent-gw')])
    })
    expect(filtered.positions.size).toBe(2)
  })

  it('orders a lane by barycenter of its service neighbours (fewer crossings)', () => {
    const map = sampleStorageMap({
      topics: [],
      bindings: [
        { service: 'a', store: 'my', access: 'rw', via: '', evidence: [], confidence: 'declared' },
        { service: 'b', store: 'pg', access: 'rw', via: '', evidence: [], confidence: 'declared' }
      ]
    })
    const l = layoutStorageLanes(buildStorageViewModel(map))
    // by name 'mysql' < 'postgres', but 'pg' is attached to service b (lower) so mysql stays first
    expect(l.positions.get(storeNodeId('my'))!.y).toBeLessThan(
      l.positions.get(storeNodeId('pg'))!.y
    )
    const swapped = sampleStorageMap({
      topics: [],
      bindings: [
        { service: 'a', store: 'pg', access: 'rw', via: '', evidence: [], confidence: 'declared' },
        { service: 'b', store: 'my', access: 'rw', via: '', evidence: [], confidence: 'declared' }
      ]
    })
    const s = layoutStorageLanes(buildStorageViewModel(swapped))
    expect(s.positions.get(storeNodeId('pg'))!.y).toBeLessThan(
      s.positions.get(storeNodeId('my'))!.y
    )
  })

  it('handles 60 nodes', () => {
    const bindings = Array.from({ length: 60 }, (_, i) => ({
      service: `s${i}`,
      store: 'pg',
      access: 'rw' as const,
      via: '',
      evidence: [],
      confidence: 'declared' as const
    }))
    expect(
      layoutStorageLanes(buildStorageViewModel(sampleStorageMap({ bindings }))).positions.size
    ).toBeGreaterThan(60)
  })
})
