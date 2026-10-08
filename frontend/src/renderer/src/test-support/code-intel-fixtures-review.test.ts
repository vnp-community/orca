import { describe, expect, it } from 'vitest'
import { createCodeIntelBridge } from '../../../shared/code-intel-bridge'
import { parseIndexStatus } from '../../../shared/code-intel-index-status-parser'
import { CODE_INTEL_RPC_METHODS as M } from '../../../shared/code-intel-rpc-methods'
import { classifyCodeIntelError } from '../runtime/code-intel-client'
import { normalizeChangeOverlay } from '../components/review-map/review-shell-data'
import { parseDataFlow } from '../hooks/useDataFlow'
import { createFakeCodeIntelBackend } from './code-intel-fake-backend'
import * as F from './code-intel-fixtures'

const SEL = { projectId: 'proj', worktreeId: 'wt' }

describe('review-frontend code-intel fixtures (one source, contract §4 types)', () => {
  it('has one IndexStatus per overall state and each survives parseIndexStatus unchanged', () => {
    const entries = Object.entries(F.INDEX_STATUS_FIXTURES)
    expect(entries).toHaveLength(9)
    for (const [name, status] of entries) {
      const parsed = parseIndexStatus(status)
      expect(parsed.overall).toBe(name.toUpperCase())
    }
  })

  it('normalizes every ChangeOverlay variant, keeping emptyReason and truncation', () => {
    const small = normalizeChangeOverlay(F.CHANGE_OVERLAY_SMALL)
    const medium = normalizeChangeOverlay(F.CHANGE_OVERLAY_MEDIUM)
    expect(small.changedFiles.length).toBeGreaterThan(0)
    expect(medium.changedFiles.length).toBeGreaterThan(small.changedFiles.length)
    expect(normalizeChangeOverlay(F.CHANGE_OVERLAY_EMPTY_UNBORN).emptyReason).toBe('unborn-head')
    expect(JSON.stringify(normalizeChangeOverlay(F.CHANGE_OVERLAY_TRUNCATED))).toContain('true')
  })

  it('keeps graph/detail fixtures in their documented shapes', () => {
    // "No edges": the contract ImpactGraph is level-based; lenses derive edges from `via`.
    expect('edges' in F.IMPACT_GRAPH_NO_EDGES).toBe(false)
    expect(F.IMPACT_GRAPH_NO_EDGES.levels[0].symbols[0].via).toBe('calls')
    expect(F.SYMBOL_DETAIL_SOURCE_OMITTED.sourceOmitted).toBeTruthy()
    expect(F.MODULE_GRAPH_PAGE_1_TOKEN).toBe('page-2')
    expect(F.CONTAINER_REFS.length).toBeGreaterThan(0)
    expect(F.READING_STEPS.length).toBeGreaterThan(0)
    expect(F.COMPONENT_GROUPS.length).toBeGreaterThan(0)
    expect(F.REVIEW_STATE_INITIAL.version).toBe(0)
  })

  it('parses the partial DataFlow with its SequenceModel and gaps', () => {
    const parsed = parseDataFlow({ flow: F.DATA_FLOW_PARTIAL, sequence: F.SEQUENCE_MODEL })
    expect(parsed.flow.completeness).toBe('partial')
    expect(parsed.flow.gaps.length).toBeGreaterThan(0)
    expect(parsed.sequence?.participants.length).toBeGreaterThan(0)
  })

  it.each([
    ['timeout', 'timeout', { inProgress: true, retryAfterMs: 5000 }],
    ['versionConflict', 'conflict', { currentVersion: 3 }],
    ['ambiguousSymbol', 'ambiguous', null],
    ['reindexCooldown', 'rate-limited', { retryAfterSeconds: 300 }]
  ] as const)('error wire %s classifies as %s with its data', (wire, kind, data) => {
    const err = classifyCodeIntelError(F.ERROR_WIRES[wire])
    expect(err.kind).toBe(kind)
    if (data) {
      expect(err.data).toMatchObject(data)
    }
  })

  it('reproduces the same error wires through the shared fake backend failNext', async () => {
    const backend = createFakeCodeIntelBackend()
    const bridge = createCodeIntelBridge(backend.bridgeDeps)
    backend.failNext(M.STATUS, F.ERROR_WIRES.timeout.error.message)
    const res = await bridge.call({ environmentId: 'env-1', method: M.STATUS, params: SEL })
    const err = classifyCodeIntelError(res)
    expect(err.kind).toBe('timeout')
    expect(err.data).toMatchObject({ inProgress: true })
    // failNext is one-shot.
    expect((await bridge.call({ environmentId: 'env-1', method: M.STATUS, params: SEL })).ok).toBe(
      true
    )
  })

  it('contains no absolute paths or secrets', () => {
    const text = JSON.stringify(F)
    expect(text).not.toMatch(/"\/(home|Users|root|tmp)\//)
    expect(text).not.toMatch(/[A-Za-z]:\\\\/)
    expect(text).not.toMatch(/(api[_-]?key|password|secret|BEGIN [A-Z ]*PRIVATE KEY)/i)
  })
})
