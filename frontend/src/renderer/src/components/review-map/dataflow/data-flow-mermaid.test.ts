import { describe, expect, it } from 'vitest'
import type { SequenceModel } from '../../../../../shared/code-intel-architecture-types'
import { buildSequenceDiagram, escapeMermaidLabel } from './data-flow-mermaid'

const msg = (n: number, over: Partial<SequenceModel['messages'][number]> = {}): SequenceModel['messages'][number] => ({
  n, from: 'a', to: 'b', label: `m${n}`, kind: 'rpc', sync: true, dashedReturn: false, confidence: 1, ...over
})
const seq = (messages: SequenceModel['messages'], extra: string[] = []): SequenceModel => ({
  participants: [{ id: 'a', label: 'UI', kind: 'ui' }, { id: 'b', label: 'Order', kind: 'component' },
    ...extra.map((id) => ({ id, label: id, kind: 'component' as const }))],
  messages
})

describe('buildSequenceDiagram', () => {
  it('prints participants in order of appearance with generated ids', () => {
    const r = buildSequenceDiagram(seq([msg(1, { from: 'b', to: 'a' })]))
    expect(r.ok && r.source.split('\n').slice(0, 4)).toEqual([
      'sequenceDiagram', '  autonumber', '  participant p1 as Order', '  participant p2 as UI'
    ])
  })

  it('maps sync/dashedReturn to arrow styles', () => {
    const r = buildSequenceDiagram(seq([
      msg(1), msg(2, { dashedReturn: true }), msg(3, { sync: false }), msg(4, { sync: false, dashedReturn: true })
    ]))
    const body = r.ok ? r.source : ''
    expect(body).toContain('p1->>p2: m1')
    expect(body).toContain('p1-->>p2: m2')
    expect(body).toContain('p1-)p2: m3')
    expect(body).toContain('p1--)p2: m4')
  })

  it('prefixes changed messages and emits notes', () => {
    const r = buildSequenceDiagram(seq([msg(1, { note: 'check' }), msg(2)]), { changedMessages: new Set([2]), changedPrefix: '[chg] ' })
    const src = r.ok ? r.source : ''
    expect(src).toContain('Note over p1,p2: check')
    expect(src).toContain(': [chg] m2')
    expect(src).not.toContain(': [chg] m1')
  })

  it('truncates to 60 messages and reports rendered/total', () => {
    const r = buildSequenceDiagram(seq(Array.from({ length: 80 }, (_, i) => msg(i + 1))))
    expect(r).toMatchObject({ ok: true, renderedMessages: 60, totalMessages: 80 })
  })

  it('refuses > 14 participants, empty input and over-long source', () => {
    const ids = Array.from({ length: 15 }, (_, i) => `x${i}`)
    const many = seq(ids.slice(1).map((id, i) => msg(i + 1, { from: ids[i], to: id })), ids)
    expect(buildSequenceDiagram(many)).toEqual({ ok: false, reason: 'too-many-participants' })
    expect(buildSequenceDiagram(seq([]))).toEqual({ ok: false, reason: 'empty' })
    expect(buildSequenceDiagram(seq([msg(1)]), { maxChars: 20 })).toEqual({ ok: false, reason: 'too-long' })
  })

  it('is deterministic', () => {
    const s = seq([msg(1), msg(2)])
    expect(buildSequenceDiagram(s)).toEqual(buildSequenceDiagram(s))
  })

  it('cannot be made to inject structure through labels, notes or participant names', () => {
    const evil = 'x\nparticipant evil\n%%{init:{"securityLevel":"loose"}}%%\n---\nclick a call alert()<script>;'
    const s = seq([msg(1, { label: evil, note: evil })])
    s.participants[0].label = evil
    const r = buildSequenceDiagram(s)
    const lines = (r.ok ? r.source : '').split('\n')
    // 2 header + 2 participants + 1 message + 1 note, nothing more.
    expect(lines).toHaveLength(6)
    for (const l of lines) {
      expect(l).not.toContain('%%')
      expect(l).not.toContain('<')
      expect(l).not.toMatch(/^\s*(participant evil|click|---)/)
    }
    expect(lines.filter((l) => l.startsWith('  participant '))).toHaveLength(2)
  })
})

describe('escapeMermaidLabel', () => {
  it('escapes in a single pass so entities are not double-escaped', () => {
    expect(escapeMermaidLabel('a#b;c')).toBe('a#35;b#59;c')
  })
  it('turns newlines to spaces and strips control characters', () => {
    expect(escapeMermaidLabel('a\r\nb\u0000c d')).toBe('a bcd')
  })
  it('handles quotes, angle brackets, percent and backticks', () => {
    expect(escapeMermaidLabel('"<>%`')).toBe('#quot;#lt;#gt;#37;#96;')
  })
  it('cuts at 48 characters before escaping and does not split entities', () => {
    expect(escapeMermaidLabel(';'.repeat(100))).toBe('#59;'.repeat(48))
  })
  it('survives non-string input', () => {
    expect(escapeMermaidLabel(undefined as unknown as string)).toBe('')
  })
})
