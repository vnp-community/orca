import { describe, it, expect } from 'vitest'
import { parseCypherOutput, assertNoFreeTextColumns } from './gitnexus-cypher-markdown-parser'
import { CypherTemplate } from './gitnexus-cypher-templates'

describe('gitnexus-cypher-markdown-parser', () => {
  const tpl: CypherTemplate = {
    id: 'TEST',
    text: '',
    slots: {},
    columns: [
      { header: 'id', type: 'string' },
      { header: 'count', type: 'int' },
      { header: 'score', type: 'float' },
      { header: 'arr', type: 'jsonArray' }
    ]
  }

  const freeTpl: CypherTemplate = {
    id: 'FREE',
    text: '',
    slots: {},
    columns: [
      { header: 'id', type: 'string' },
      { header: 'label', type: 'string' }
    ],
    freeTextColumn: true
  }

  it('parses valid markdown table', () => {
    const stdout = `
| id | count | score | arr |
|---|---|---|---|
| node1 | 42 | 3.14 | ["'val1'"] |
| node2 | null | | [] |
(2 rows)
    `.trim()

    const res = parseCypherOutput(stdout, tpl)
    expect(res.rowCount).toBe(2)
    expect(res.skippedRows).toBe(0)
    expect(res.rows).toEqual([
      { id: 'node1', count: 42, score: 3.14, arr: ['val1'] },
      { id: 'node2', count: null, score: null, arr: [] }
    ])
  })

  it('handles empty []', () => {
    const res = parseCypherOutput('[]', tpl)
    expect(res.rows).toEqual([])
  })

  it('handles {"error"}', () => {
    expect(() => parseCypherOutput('{"error": "some cypher error"}', tpl)).toThrow(/GitNexus error: some cypher error/)
  })

  it('handles unexpected columns', () => {
    const stdout = `
| wrong | columns |
|---|---|
| a | b |
    `.trim()
    expect(() => parseCypherOutput(stdout, tpl)).toThrow(/unexpected_columns/)
  })

  it('gathers extra columns into freeTextColumn', () => {
    const stdout = `
| id | label |
|---|---|
| n1 | Section:CLAUDE.md:L23:GitNexus | Code Intelligence |
(1 rows)
    `.trim()
    const res = parseCypherOutput(stdout, freeTpl)
    expect(res.rows).toEqual([
      { id: 'n1', label: 'Section:CLAUDE.md:L23:GitNexus | Code Intelligence' }
    ])
  })

  it('skips rows with wrong column count and warns', () => {
    const stdout = `
| id | count | score | arr |
|---|---|---|---|
| n1 | 1 | 2 | [] |
| n2 | 1 | 2 |
| n3 | 1 | 2 | [] |
(3 rows)
    `.trim()
    const res = parseCypherOutput(stdout, tpl)
    expect(res.skippedRows).toBe(1)
    expect(res.warnings).toContain('gitnexus: skipped 1 rows with unparsable columns')
    expect(res.warnings).toContain('row_count_mismatch')
  })

  describe('assertNoFreeTextColumns', () => {
    it('throws if forbidden column name is used', () => {
      const badTpl: CypherTemplate = {
        id: 'BAD',
        text: '',
        slots: {},
        columns: [{ header: 'content', type: 'string' }]
      }
      expect(() => assertNoFreeTextColumns(badTpl)).toThrow(/forbidden free text column/)
    })

    it('passes if no forbidden column name', () => {
      expect(() => assertNoFreeTextColumns(tpl)).not.toThrow()
    })
  })
})
