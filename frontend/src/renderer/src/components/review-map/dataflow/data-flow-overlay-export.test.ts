// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { DataFlow } from '../../../../../shared/code-intel-architecture-types'
import { buildStepRows, changedMessageSet, filterFlowsTouchingChange } from './data-flow-overlay'
import { copyMermaidSource, dataFlowExportFilename, downloadSvgFromContainer } from './data-flow-export'

const ref = (name: string) => ({ container: 'c', componentId: name, name, kind: 'component' as const })
const step = (n: number, over: Partial<DataFlow['steps'][number]> = {}): DataFlow['steps'][number] => ({
  n, from: ref('a'), to: ref('b'), kind: 'call', sync: true, confidence: 1, origin: 'declared', evidence: [], ...over
})
const sym = (key: string) => ({ key, kind: 'func', name: key, filePath: `${key}.go` }) as never
const flow = (over: Partial<DataFlow> = {}): DataFlow => ({
  id: 'f', label: 'F', trigger: { kind: 'grpc', name: 't' }, steps: [step(1, { symbol: sym('s1') }), step(2), step(3, { symbol: sym('s3'), unimplemented: true })],
  stores: [{ step: 2, store: { id: 's', kind: 'postgres', name: 'pg' }, table: 'Orders', op: 'write', confidence: 1 }],
  completeness: 'partial', gaps: [{ afterStep: 2, code: 'x', message: 'missing' }], services: [], relatedProcesses: [], ...over
})
const overlay = (over = {}) => ({
  changedSymbols: [{ symbol: sym('s1'), changeKind: 'modified', tested: 'no' }],
  uncoveredSymbols: [sym('s3')],
  touchedTables: [] as unknown[],
  affectedFlows: [] as unknown[],
  ...over
})

describe('buildStepRows', () => {
  it('flags changed by symbol key, untested, unimplemented, and inserts the gap row', () => {
    const rows = buildStepRows(flow(), overlay(), 3)
    expect(rows.map((r) => r.type)).toEqual(['step', 'step', 'gap', 'step'])
    const s = rows.filter((r) => r.type === 'step') as Extract<(typeof rows)[number], { type: 'step' }>[]
    expect(s[0].flags.changed).toBe(true)
    expect(s[1].flags.changed).toBe(false)
    expect(s[2].flags).toMatchObject({ untested: true, unimplemented: true })
    expect(rows[2]).toMatchObject({ type: 'gap', afterStep: 2, message: 'missing' })
  })

  it('flags changed via touched tables (case-insensitive, string or object)', () => {
    for (const t of ['orders', { table: 'ORDERS' }]) {
      const rows = buildStepRows(flow(), overlay({ touchedTables: [t] }), 3)
      const s2 = rows.find((r) => r.type === 'step' && r.step.n === 2)
      expect(s2 && s2.type === 'step' && s2.flags.changed).toBe(true)
    }
  })

  it('marks steps beyond the rendered part as outside the diagram', () => {
    const rows = buildStepRows(flow(), overlay(), 1).filter((r) => r.type === 'step') as { flags: { inDiagram: boolean } }[]
    expect(rows.map((r) => r.flags.inDiagram)).toEqual([true, false, false])
  })

  it('puts a gap before step 1 when afterStep is 0', () => {
    const rows = buildStepRows(flow({ gaps: [{ afterStep: 0, code: 'c', message: 'm' }] }), overlay(), 3)
    expect(rows[0].type).toBe('gap')
  })
})

describe('changedMessageSet / filterFlowsTouchingChange', () => {
  it('lists changed step numbers', () => {
    expect([...changedMessageSet(flow(), overlay({ touchedTables: ['orders'] }))].sort()).toEqual([1, 2])
  })
  it('intersects ids and reports unknown when nothing matches', () => {
    const list = [{ id: 'a' }, { id: 'b' }]
    expect(filterFlowsTouchingChange(list, overlay({ affectedFlows: [{ id: 'b' }, 'zzz'] }))).toEqual({ ids: new Set(['b']), unknown: false })
    expect(filterFlowsTouchingChange(list, overlay())).toEqual({ ids: new Set(), unknown: true })
  })
})

describe('export helpers', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('builds Windows-safe file names', () => {
    const name = dataFlowExportFilename('Create order: POST /v1/orders?x=1', 'mmd', new Date(2026, 9, 7, 8, 5))
    expect(name).toBe('dataflow-create-order-post-v1-orders-x-1-20261007-0805.mmd')
    expect(name).not.toMatch(/[:\\/*?"<>|]/)
    expect(dataFlowExportFilename('???', 'svg', new Date(2026, 0, 1, 0, 0))).toBe('dataflow-flow-20260101-0000.svg')
    expect(dataFlowExportFilename('x'.repeat(100), 'svg', new Date(2026, 0, 1)).length).toBeLessThan(70)
  })

  it('copies through window.api.ui.writeClipboardText', async () => {
    const write = vi.fn().mockResolvedValue(undefined)
    ;(window as unknown as { api: unknown }).api = { ui: { writeClipboardText: write } }
    await expect(copyMermaidSource('sequenceDiagram')).resolves.toBe(true)
    expect(write).toHaveBeenCalledWith('sequenceDiagram')
    write.mockRejectedValue(new Error('x'))
    await expect(copyMermaidSource('s')).resolves.toBe(false)
  })

  it('downloads svg content only when present and script-free', async () => {
    const blobs: Blob[] = []
    URL.createObjectURL = vi.fn((b: Blob) => { blobs.push(b); return 'blob:x' }) as never
    URL.revokeObjectURL = vi.fn()
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    const host = document.createElement('div')
    expect(downloadSvgFromContainer(host, 'a.svg')).toBe(false)
    host.innerHTML = '<svg><script>alert(1)</script></svg>'
    expect(downloadSvgFromContainer(host, 'a.svg')).toBe(false)
    host.innerHTML = '<svg onload="x()"></svg>'
    expect(downloadSvgFromContainer(host, 'a.svg')).toBe(false)
    expect(click).not.toHaveBeenCalled()
    host.innerHTML = '<svg><g><text>hi</text></g></svg>'
    expect(downloadSvgFromContainer(host, 'a.svg')).toBe(true)
    expect(click).toHaveBeenCalledTimes(1)
    const text = await blobs[0].text()
    expect(text).toContain('xmlns="http://www.w3.org/2000/svg"')
    expect(text).not.toContain('<script')
    click.mockRestore()
  })
})
