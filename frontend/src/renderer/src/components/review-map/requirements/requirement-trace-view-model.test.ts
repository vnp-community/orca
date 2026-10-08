import { describe, expect, it } from 'vitest'
import { buildRequirementTraceViewModel, parseRequirementTrace } from './requirement-trace-view-model'
import { evidenceWire, requirementWire, traceFixture, traceWire } from './requirement-trace.fixture'

const build = (over: Record<string, unknown>, showInferred = true) =>
  buildRequirementTraceViewModel(traceFixture(over), { showInferred })

const req = (key: string, state: string, evidence: unknown[] = []) => requirementWire({ key, state, evidence })

describe('buildRequirementTraceViewModel', () => {
  it('orders groups with "no evidence" first', () => {
    const vm = build({
      requirements: [
        req('a', 'has_evidence', [evidenceWire()]),
        req('b', 'unknown'),
        req('c', 'manual_pending'),
        req('d', 'partial', [evidenceWire({ ref: 'x' })]),
        req('e', 'no_evidence')
      ]
    })
    expect(vm.groups.map((g) => g.key)).toEqual(['noEvidence', 'partial', 'manualPending', 'hasEvidence', 'unknown'])
    expect(vm.counts).toEqual({ noEvidence: 1, partial: 1, manualPending: 1, hasEvidence: 1, unknown: 1 })
  })

  it('keeps unknown distinct from no_evidence (different label and icon)', () => {
    const vm = build({ requirements: [req('a', 'unknown'), req('b', 'no_evidence')] })
    const rows = vm.groups.flatMap((g) => g.rows)
    const unknown = rows.find((r) => r.key === 'a')!
    const none = rows.find((r) => r.key === 'b')!
    expect(unknown.stateLabelKey).not.toBe(none.stateLabelKey)
    expect(unknown.stateIcon).not.toBe(none.stateIcon)
    expect(unknown.stateLabelKey).toMatch(/state\.unknown$/)
  })

  it('treats inferred-only evidence as no evidence and lists it under suggestions without counting it', () => {
    const vm = build({
      requirements: [req('a', 'has_evidence', [evidenceWire({ confidence: 'inferred', ref: 'guess.ts' })])]
    })
    expect(vm.counts.hasEvidence).toBe(0)
    expect(vm.counts.noEvidence).toBe(1)
    const row = vm.groups[0].rows[0]
    expect(row.state).toBe('no_evidence')
    expect(row.evidence).toEqual([])
    expect(row.suggestions.map((s) => s.ref)).toEqual(['guess.ts'])
    expect(row.suggestions[0].inferred).toBe(true)
  })

  it('hides suggestions when showInferred is off', () => {
    const vm = build({ requirements: [req('a', 'no_evidence', [evidenceWire({ confidence: 'inferred' })])] }, false)
    expect(vm.groups[0].rows[0].suggestions).toEqual([])
  })

  it('keeps a partial requirement partial when it has confirmed evidence', () => {
    const vm = build({
      requirements: [req('a', 'partial', [evidenceWire(), evidenceWire({ confidence: 'inferred', ref: 'g' })])]
    })
    expect(vm.groups[0].key).toBe('partial')
    expect(vm.groups[0].rows[0].evidence).toHaveLength(1)
  })

  it('skips retired requirements', () => {
    const vm = build({ requirements: [requirementWire({ retired: true })] })
    expect(vm.isEmpty).toBe(true)
  })

  it('allows linking only when no confirmed link exists', () => {
    expect(build({ linkConfidence: 'none' }).canLinkTask).toBe(true)
    expect(build({ linkConfidence: 'inferred' }).canLinkTask).toBe(true)
    expect(build({ linkConfidence: 'explicit' }).canLinkTask).toBe(false)
  })

  it('maps known warnings to keys and unknown ones to the generic key', () => {
    const vm = build({ warnings: ['no_structured_criteria', 'weird_code'] })
    expect(vm.warnings.map((w) => w.labelKey)).toEqual([
      expect.stringMatching(/warning\.no_structured_criteria$/),
      expect.stringMatching(/warning\.generic$/)
    ])
  })

  it('truncates long requirement text for display and keeps markup as text', () => {
    const vm = build({ requirements: [requirementWire({ text: `<script>alert(1)</script>${'x'.repeat(400)}`, state: 'no_evidence', evidence: [] })] })
    const text = vm.groups[0].rows[0].text
    expect(text.length).toBeLessThanOrEqual(300)
    expect(text.startsWith('<script>')).toBe(true) // untouched: the view renders text nodes only
  })

  it('returns an empty, linkable view model without a trace', () => {
    expect(buildRequirementTraceViewModel(null, { showInferred: false })).toMatchObject({ isEmpty: true, canLinkTask: true })
  })
})

describe('parseRequirementTrace', () => {
  it.each([null, undefined, 3, 'x', []])('never throws on %j', (raw) => {
    const trace = parseRequirementTrace(raw)
    expect(trace.requirements).toEqual([])
    expect(trace.linkConfidence).toBe('none')
  })

  it('coerces unknown enums to the weakest/unknown value', () => {
    const trace = parseRequirementTrace(
      traceWire({
        linkConfidence: 'certain',
        requirements: [requirementWire({ state: 'satisfied', evidence: [evidenceWire({ confidence: 'sure', kind: 'zzz' })] })]
      })
    )
    expect(trace.linkConfidence).toBe('none')
    expect(trace.requirements[0].state).toBe('unknown')
    expect(trace.requirements[0].evidence[0]).toMatchObject({ confidence: 'inferred', kind: 'unknown' })
  })
})
