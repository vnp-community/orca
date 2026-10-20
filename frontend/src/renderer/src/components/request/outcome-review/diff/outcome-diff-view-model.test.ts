import { describe, expect, it } from 'vitest'
import { makeItem, makeModel, makeReview, makeVerdict } from '../outcome-review.fixture'
import {
  containersOf,
  diagramsOf,
  dotsOf,
  modeAvailability,
  rowOf,
  rowsOf,
  sectionOrDefault,
  subTabsOf,
  symbolOf
} from './outcome-diff-view-model'

describe('modeAvailability', () => {
  it('before the code only Design is offered, and the others say when they will be', () => {
    const m = modeAvailability(makeReview({ phase: 'design' }), 'executing')
    expect(m.design).toEqual({ enabled: true })
    expect(m.after_code).toEqual({ enabled: false, reason: 'execution_not_finished' })
    expect(m.side_by_side).toEqual({ enabled: false, reason: 'execution_not_finished' })
    expect(m.initial).toBe('design')
  })

  it('Execution over but no after-code review yet: generate it first', () => {
    const m = modeAvailability(makeReview({ phase: 'design' }), 'completed')
    expect(m.after_code).toEqual({ enabled: false, reason: 'generate_first' })
    expect(m.side_by_side.reason).toBe('generate_first')
  })

  it('delivered: all three, After code first', () => {
    const m = modeAvailability(makeReview(), 'completed')
    expect([m.design.enabled, m.after_code.enabled, m.side_by_side.enabled]).toEqual([
      true,
      true,
      true
    ])
    expect(m.initial).toBe('after_code')
  })

  it('without the knowledge graph there is no design to show or to put beside the code', () => {
    const m = modeAvailability(
      makeReview({ model: { kg: { available: false, gaps: [], conflicts: [] } } }),
      'completed'
    )
    expect(m.design).toEqual({ enabled: false, reason: 'design_unavailable' })
    expect(m.after_code.enabled).toBe(true)
    expect(m.side_by_side).toEqual({ enabled: false, reason: 'design_unavailable' })
  })
})

describe('rows', () => {
  it('symbols are characters, never only a colour', () => {
    expect(['added', 'modified', 'removed', 'missing', 'unchanged', 'weird'].map(symbolOf)).toEqual(
      ['+', '~', '−', '∅', '·', '·']
    )
  })

  it('flags come from the data and from nothing else', () => {
    const f = (over: Parameters<typeof makeItem>[0]) => rowOf(makeItem(over)).flags
    expect(f({ severity: 'deviation', change: 'added' })).toEqual(['deviates'])
    expect(f({ severity: 'deviation', change: 'missing' })).toEqual(['missing_from_code'])
    expect(f({ severity: 'info', source: 'code' })).toEqual(['not_in_design'])
    expect(f({ severity: 'info', source: 'sad-text' })).toEqual(['sad_text_only'])
    expect(f({ severity: 'info', source: 'kg' })).toEqual([])
    expect(f({ severity: 'breaking' })).toEqual(['breaking'])
    expect(f({ severity: 'regression', origin: 'agent' })).toEqual(['regression'])
    expect(f({ severity: 'info', advisory: true, unverified: true })).toEqual([
      'advisory',
      'unverified'
    ])
    expect(f({ severity: 'mystery' })).toEqual([])
  })

  it('the dots say where a thing is, and unknown where the review cannot tell', () => {
    expect(dotsOf(makeItem({ change: 'added', expected: false }))).toEqual({
      design: 'no',
      base: 'no',
      head: 'yes'
    })
    expect(dotsOf(makeItem({ change: 'removed', expected: true }))).toEqual({
      design: 'yes',
      base: 'yes',
      head: 'no'
    })
    expect(dotsOf(makeItem({ change: 'missing', expected: true }))).toEqual({
      design: 'yes',
      base: 'no',
      head: 'no'
    })
    expect(dotsOf(makeItem({ change: 'modified' }))).toEqual({
      design: 'unknown',
      base: 'yes',
      head: 'yes'
    })
    expect(dotsOf(makeItem({ change: 'strange' }))).toEqual({
      design: 'unknown',
      base: 'unknown',
      head: 'unknown'
    })
  })

  it('marks what an agent said apart from what Orca measured', () => {
    expect(rowOf(makeItem({ origin: 'agent' })).agentReported).toBe(true)
    expect(rowOf(makeItem({ origin: 'fact' })).agentReported).toBe(false)
  })

  const items = [
    makeItem({ key: 'a', section: 'architecture', change: 'added' }),
    makeItem({ key: 'b', section: 'architecture', change: 'unchanged' }),
    makeItem({ key: 'c', section: 'flow', change: 'removed' }),
    makeItem({
      key: 'd',
      section: 'requirements',
      change: 'unchanged',
      origin: 'fact',
      before: 'planned',
      after: 'delivered'
    })
  ]
  const review = makeReview({ model: { items } })

  it('After code hides the unchanged unless asked; Design lists only the design itself', () => {
    expect(rowsOf(review, 'architecture', 'after_code', false).map((r) => r.item.key)).toEqual([
      'a'
    ])
    expect(rowsOf(review, 'architecture', 'after_code', true).map((r) => r.item.key)).toEqual([
      'a',
      'b'
    ])
    expect(rowsOf(review, 'architecture', 'design', false).map((r) => r.item.key)).toEqual(['b'])
    expect(rowsOf(review, 'flow', 'side_by_side', false).map((r) => r.item.key)).toEqual(['c'])
  })

  it('requirements always list every criterion, whatever the mode', () => {
    for (const mode of ['design', 'after_code', 'side_by_side'] as const) {
      expect(rowsOf(review, 'requirements', mode, false).map((r) => r.item.key)).toEqual(['d'])
    }
  })
})

describe('tabs', () => {
  it('Architecture and Flow show how many things are not in the design, Unknown when there is no design', () => {
    const checked = makeReview({
      model: {
        verdict: {
          ...makeModel().verdict,
          architecture: makeVerdict({ designChecked: true, unplannedEdges: 2 }),
          flow: makeVerdict({ designChecked: false, unplannedCalls: 9 })
        }
      }
    })
    const tabs = subTabsOf(checked, new Set())
    expect(tabs.map((t) => t.section)).toEqual([
      'architecture',
      'flow',
      'behavior',
      'data',
      'contracts',
      'requirements',
      'config'
    ])
    expect(tabs[0].unplanned).toBe(2)
    expect(tabs[1].unplanned).toBeNull()
    expect(tabs[2].unplanned).toBeNull()
    expect(tabs.filter((t) => t.afterCodeOnly).map((t) => t.section)).toEqual([
      'contracts',
      'config'
    ])
  })

  it('the unseen count leaves out what the viewer marked and what did not change', () => {
    const review = makeReview({
      model: {
        items: [
          makeItem({ key: 'a' }),
          makeItem({ key: 'b' }),
          makeItem({ key: 'c', change: 'unchanged' }),
          makeItem({ key: 'r', section: 'requirements', change: 'unchanged' })
        ]
      }
    })
    const tabs = subTabsOf(review, new Set(['a']))
    expect(tabs[0].unseen).toBe(1)
    expect(tabs.find((t) => t.section === 'requirements')?.unseen).toBe(1)
  })

  it('an unfamiliar section falls back to Architecture', () => {
    expect(sectionOrDefault('flow')).toBe('flow')
    expect(sectionOrDefault('security')).toBe('architecture')
  })
})

describe('diagrams and containers', () => {
  it('picks the diagrams of a section and a variant', () => {
    const review = makeReview({
      model: {
        diagrams: [
          {
            section: 'architecture',
            variant: 'design',
            type: 'architecture',
            inputDigest: '1',
            status: 'cached',
            artifactId: 'a1'
          },
          {
            section: 'architecture',
            variant: 'delta',
            type: 'architecture',
            inputDigest: '2',
            status: 'missing'
          },
          {
            section: 'flow',
            variant: 'delta',
            type: 'sequence',
            inputDigest: '3',
            status: 'cached',
            name: 'Bug status'
          }
        ]
      }
    })
    expect(diagramsOf(review, 'architecture', 'design').map((d) => d.artifactId)).toEqual(['a1'])
    expect(diagramsOf(review, 'flow', 'delta')[0].name).toBe('Bug status')
    expect(diagramsOf(review, 'data', 'delta')).toEqual([])
  })

  it('lists the containers that changed, busiest first, and ignores the unchanged', () => {
    const review = makeReview({
      model: {
        items: [
          makeItem({ key: 'a', containers: ['ms-gateway', 'ms-bug'] }),
          makeItem({ key: 'b', containers: ['ms-bug'] }),
          makeItem({ key: 'c', containers: ['ms-old'], change: 'unchanged' }),
          makeItem({ key: 'd', section: 'flow', containers: ['ms-flow'] })
        ]
      }
    })
    expect(containersOf(review)).toEqual([
      { id: 'ms-bug', items: 2 },
      { id: 'ms-gateway', items: 1 }
    ])
  })
})
