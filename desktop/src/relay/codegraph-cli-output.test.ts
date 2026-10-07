import { describe, it, expect } from 'vitest'
import {
  parseCodeGraphStatus,
  parseCodeGraphQuery,
  parseCodeGraphFiles,
  parseCodeGraphAffected,
} from './codegraph-cli-output'
import { CodeIntelError } from './codeintel-errors'

describe('codegraph-cli-output', () => {
  it('parses valid files and truncates if needed', () => {
    const stdout = JSON.stringify([
      { path: 'a.ts', language: 'typescript', nodeCount: 10, size: 100 },
      { path: 'b.ts', language: 'typescript', nodeCount: 20, size: 200 }
    ])
    
    const res1 = parseCodeGraphFiles(stdout)
    expect(res1.files.length).toBe(2)
    expect(res1.truncated).toBe(false)
    
    const res2 = parseCodeGraphFiles(stdout, 1)
    expect(res2.files.length).toBe(1)
    expect(res2.files[0].path).toBe('a.ts')
    expect(res2.truncated).toBe(true)
  })
  
  it('parses affected tests correctly', () => {
    const stdout = JSON.stringify({
      changedFiles: ['a.ts'],
      affectedTests: [{ name: 'test_a', file: 'a.test.ts' }]
    })
    const res = parseCodeGraphAffected(stdout)
    expect(res.changedFiles).toEqual(['a.ts'])
    expect(res.affectedTests.length).toBe(1)
  })

  it('throws INDEX_MISSING on specific ansi output', () => {
    const stdout = '\\x1b[31m✗ CodeGraph not initialized in /tmp\\x1b[0m'
    let error: any
    try {
      parseCodeGraphStatus(stdout)
    } catch (e) {
      error = e
    }
    expect(error).toBeInstanceOf(CodeIntelError)
    expect(error.code).toBe('CODEINTEL_INDEX_MISSING')
  })

  it('throws SYMBOL_NOT_FOUND on symbol not found', () => {
    const stdout = '\\x1b[33mℹ Symbol "X" not found\\x1b[0m'
    let error: any
    try {
      parseCodeGraphQuery(stdout)
    } catch (e) {
      error = e
    }
    expect(error).toBeInstanceOf(CodeIntelError)
    expect(error.code).toBe('CODEINTEL_SYMBOL_NOT_FOUND')
  })

  it('throws unknown_shape when files is not an array', () => {
    const stdout = JSON.stringify({ path: 'a.ts' })
    let error: any
    try {
      parseCodeGraphFiles(stdout)
    } catch (e) {
      error = e
    }
    expect(error).toBeInstanceOf(CodeIntelError)
    expect(error.code).toBe('CODEINTEL_TOOL_FAILED')
    expect(error.data?.reason).toBe('unknown_shape')
  })
})
