import { describe, it, expect } from 'vitest'
import fs from 'fs'
import path from 'path'
import {
  parseCodeGraphStatus,
  parseCodeGraphQuery,
  parseCodeGraphCallers,
  parseCodeGraphCallees,
  parseCodeGraphFiles
} from '../codegraph-cli-output'
import { classifyCodeGraphStdout } from '../codeintel-tool-output-classification'
import { CodeIntelError } from '../codeintel-errors'

describe('codegraph-output golden tests', () => {
  const fixtureDir = path.join(__dirname, '__fixtures__', 'codegraph', '1.4.1')

  it('parses status.json correctly', () => {
    const raw = fs.readFileSync(path.join(fixtureDir, 'status.json'), 'utf8')
    const res = parseCodeGraphStatus(raw)
    expect(res.status).toBe('ready')
    expect(res.project).toBe('/tmp/repo')
  })

  it('parses query-results.json correctly', () => {
    const raw = fs.readFileSync(path.join(fixtureDir, 'query-results.json'), 'utf8')
    const res = parseCodeGraphQuery(raw)
    expect(res.length).toBe(1)
    expect(res[0].node.name).toBe('GetUser')
  })

  it('handles query-empty.json as empty results without format_drift', () => {
    const raw = fs.readFileSync(path.join(fixtureDir, 'query-empty.json'), 'utf8')
    const res = parseCodeGraphQuery(raw)
    expect(res).toEqual([])
  })

  it('handles ANSI "not found" output by throwing CODEINTEL_SYMBOL_NOT_FOUND instead of syntax error', () => {
    const raw = fs.readFileSync(path.join(fixtureDir, 'symbol-not-found.txt'), 'utf8')
    let err: any = null
    try {
      classifyCodeGraphStdout(raw)
    } catch (e) {
      err = e
    }
    expect(err).toBeInstanceOf(CodeIntelError)
    expect(err.code).toBe('CODEINTEL_SYMBOL_NOT_FOUND')
    expect(err.message).toContain('UnknownSymbol')
  })

  it('parses callers.json and callees.json correctly', () => {
    const callersRaw = fs.readFileSync(path.join(fixtureDir, 'callers.json'), 'utf8')
    const callers = parseCodeGraphCallers(callersRaw)
    expect(callers.length).toBe(1)
    expect(callers[0].caller).toBe('HandleUser')

    const calleesRaw = fs.readFileSync(path.join(fixtureDir, 'callees.json'), 'utf8')
    const callees = parseCodeGraphCallees(calleesRaw)
    expect(callees.length).toBe(1)
    expect(callees[0].callee).toBe('GetUser')
  })

  it('parses files.json with limit and truncation flag', () => {
    const raw = fs.readFileSync(path.join(fixtureDir, 'files.json'), 'utf8')
    const res = parseCodeGraphFiles(raw, 3)
    expect(res.files.length).toBe(3)
    expect(res.truncated).toBe(true)
  })

  it('parses sqlite-schema.json and checks extractionVersion and schema_versions max', () => {
    const raw = fs.readFileSync(path.join(fixtureDir, 'sqlite-schema.json'), 'utf8')
    const schema = JSON.parse(raw)
    expect(schema.extractionVersion).toBe(24)
    expect(Math.max(...schema.schema_versions)).toBe(8)
  })

  it('throws on truly malformed JSON output', () => {
    const broken = '{"some": broken json'
    expect(() => classifyCodeGraphStdout(broken)).toThrowError(CodeIntelError)
  })
})
