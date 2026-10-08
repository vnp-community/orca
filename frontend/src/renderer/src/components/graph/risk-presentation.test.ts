import { describe, expect, it } from 'vitest'
import { riskPresentation } from './risk-presentation'
import type { GraphRisk } from '../../../../shared/graph-types'

const LEVELS: GraphRisk[] = ['low', 'medium', 'high', 'critical', 'unknown']

describe('riskPresentation', () => {
  it('gives every level a label key, icon and distinct shape cues', () => {
    const icons = new Set(LEVELS.map((l) => riskPresentation(l).Icon))
    expect(icons.size).toBe(5)
    for (const l of LEVELS) {
      expect(riskPresentation(l).labelKey).toBe(`auto.components.graph.RiskLevel.${l}`)
    }
    expect(riskPresentation('critical').doubleRing).toBe(true)
    expect(riskPresentation('high').borderWidthClass).toBe('border-2')
    expect(riskPresentation('unknown').dashed).toBe(true)
  })

  it('never styles unknown like low and never uses risk tokens for it', () => {
    const unknown = riskPresentation('unknown')
    expect(unknown.textClass).not.toBe(riskPresentation('low').textClass)
    expect(unknown.textClass).toBe('text-muted-foreground')
    expect(JSON.stringify([unknown.textClass, unknown.bgClass, unknown.borderClass])).not.toContain('risk-')
  })

  it('falls back to unknown for out-of-domain values', () => {
    expect(riskPresentation('bogus' as GraphRisk).level).toBe('unknown')
    expect(riskPresentation('toString' as GraphRisk).level).toBe('unknown')
  })

  it('contains no hex colors', () => {
    for (const l of LEVELS) {
      const p = riskPresentation(l)
      expect(`${p.textClass} ${p.bgClass} ${p.borderClass}`).not.toMatch(/#[0-9a-fA-F]{3,6}/)
    }
  })
})
