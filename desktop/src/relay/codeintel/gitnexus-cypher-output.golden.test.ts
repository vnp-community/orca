import { describe, it, expect } from 'vitest'
import fs from 'fs'
import path from 'path'
import { parseCypherOutput } from '../gitnexus-cypher-markdown-parser'
import { CypherTemplate } from '../gitnexus-cypher-templates'
import { CodeIntelError } from '../codeintel-errors'

describe('gitnexus-cypher-output golden tests', () => {
  const fixtureDir = path.join(__dirname, '__fixtures__', 'gitnexus', '1.6.9')

  const baseTpl: CypherTemplate = {
    id: 'BASE',
    text: '',
    slots: {},
    columns: [
      { header: 'id', type: 'string' },
      { header: 'count', type: 'int' },
      { header: 'score', type: 'float' },
      { header: 'arr', type: 'jsonArray' }
    ]
  }

  const freeTextTpl: CypherTemplate = {
    id: 'FREETEXT',
    text: '',
    slots: {},
    columns: [
      { header: 'id', type: 'string' },
      { header: 'label', type: 'string' }
    ],
    freeTextColumn: true
  }

  const arrTpl: CypherTemplate = {
    id: 'ARR',
    text: '',
    slots: {},
    columns: [
      { header: 'id', type: 'string' },
      { header: 'arr', type: 'jsonArray' }
    ]
  }

  function getTemplateForFixture(name: string): CypherTemplate {
    if (name.includes('pipe-newline')) return freeTextTpl
    if (name.includes('communities-array')) return arrTpl
    return baseTpl
  }

  function sortObjectKeys(obj: any): any {
    if (obj === null || typeof obj !== 'object') return obj
    if (Array.isArray(obj)) return obj.map(sortObjectKeys)
    const sorted: Record<string, any> = {}
    for (const key of Object.keys(obj).sort()) {
      sorted[key] = sortObjectKeys(obj[key])
    }
    return sorted
  }

  it('TestParserGolden: matches expected golden files byte for byte', () => {
    const files = fs.readdirSync(fixtureDir).filter(f => f.startsWith('cypher-') && f.endsWith('.json') && !f.endsWith('.expected.json'))

    for (const file of files) {
      const baseName = file.replace('.json', '')
      const expectedFile = `${baseName}.expected.json`
      const fixtureContent = JSON.parse(fs.readFileSync(path.join(fixtureDir, file), 'utf8'))
      const expectedContent = JSON.parse(fs.readFileSync(path.join(fixtureDir, expectedFile), 'utf8'))
      const tpl = getTemplateForFixture(file)

      if (expectedContent.error) {
        let thrownError: any = null
        try {
          parseCypherOutput(fixtureContent.stdout, tpl)
        } catch (e) {
          thrownError = e
        }
        expect(thrownError).toBeInstanceOf(CodeIntelError)
        expect(thrownError.code).toBe(expectedContent.error.code)
        if (expectedContent.error.reason) {
          expect(thrownError.data?.reason).toBe(expectedContent.error.reason)
        }
      } else {
        const actual = parseCypherOutput(fixtureContent.stdout, tpl)
        const sortedActual = sortObjectKeys(actual)
        const sortedExpected = sortObjectKeys(expectedContent)
        expect(JSON.stringify(sortedActual, null, 2)).toBe(JSON.stringify(sortedExpected, null, 2))
      }
    }
  })

  it('handles CRLF line endings identically to LF', () => {
    const fixtureContent = JSON.parse(fs.readFileSync(path.join(fixtureDir, 'cypher-rows.json'), 'utf8'))
    const crlfStdout = fixtureContent.stdout.replace(/\n/g, '\r\n')
    const res = parseCypherOutput(crlfStdout, baseTpl)
    expect(res.rowCount).toBe(2)
    expect(res.rows.length).toBe(2)
  })

  it('throws on format drift with file context and without dumping raw stdout', () => {
    const invalidHeaderStdout = '| id | count | bad_col | arr |\n|---|---|---|---|\n| n1 | 1 | 2.0 | [] |\n(1 rows)'
    let error: any = null
    try {
      parseCypherOutput(invalidHeaderStdout, baseTpl)
    } catch (e) {
      error = e
    }
    expect(error).toBeInstanceOf(CodeIntelError)
    expect(error.code).toBe('CODEINTEL_TOOL_FAILED')
    expect(error.message).toContain('unexpected_columns')
    expect(error.message).not.toContain(invalidHeaderStdout)
  })

  it.each([
    ['cypher-empty.json', 0],
    ['cypher-empty-markdown.json', 0],
    ['cypher-rows.json', 2]
  ])('row counts match for %s', (fixtureFile, expectedCount) => {
    const fixtureContent = JSON.parse(fs.readFileSync(path.join(fixtureDir, fixtureFile), 'utf8'))
    const res = parseCypherOutput(fixtureContent.stdout, baseTpl)
    expect(res.rows.length).toBe(expectedCount)
  })
})
