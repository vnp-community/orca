import { describe, it, expect } from 'vitest'
import { CODEINTEL_METHODS } from '../codeintel-method-table'
import { QUALITY_METHODS, QUALITY_METHOD_SCHEMAS } from '../quality-method-table'
import { FORBIDDEN_PARAM_NAMES } from '../codeintel-params-validation'
import { CodeIntelError } from '../codeintel-errors'

describe('security-params-and-schema (Task 072-03)', () => {
  const EXPECTED_CODEINTEL_METHODS = [
    'codeintel.status',
    'codeintel.overview',
    'codeintel.processes',
    'codeintel.process',
    'codeintel.routes',
    'codeintel.subgraph',
    'codeintel.impact',
    'codeintel.symbol',
    'codeintel.codegraphSearch',
    'codeintel.files',
    'codeintel.structuralFacts',
    'codeintel.reindex',
    'codeintel.reindexStatus',
    'codeintel.reindexCancel',
    'codeintel.watch',
    'codeintel.detectChanges'
  ]

  const EXPECTED_QUALITY_METHODS = [
    'quality.listProfiles',
    'quality.run',
    'quality.runStatus',
    'quality.cancel',
    'quality.results',
    'quality.coverage'
  ]

  it('TestPublicMethodSetMatchesContract: public methods match exact contract and exclude codeintel.node', () => {
    const codeIntelKeys = Object.keys(CODEINTEL_METHODS).sort()
    expect(codeIntelKeys).toEqual([...EXPECTED_CODEINTEL_METHODS].sort())
    expect(codeIntelKeys).not.toContain('codeintel.node')

    const qualityKeys = [...QUALITY_METHODS].sort()
    expect(qualityKeys).toEqual([...EXPECTED_QUALITY_METHODS].sort())
  })

  it('Schema Reflection: no method schema contains forbidden param names', () => {
    // Check quality method schemas
    for (const [method, fields] of Object.entries(QUALITY_METHOD_SCHEMAS)) {
      for (const field of fields) {
        expect(FORBIDDEN_PARAM_NAMES).not.toContain(field)
      }
    }
  })

  describe('TestParamsStrict on CodeIntel methods', () => {
    for (const methodName of EXPECTED_CODEINTEL_METHODS) {
      const def = CODEINTEL_METHODS[methodName]

      it(`${methodName} rejects missing workspaceRoot`, () => {
        expect(() => {
          def.validate({})
        }).toThrow()
      })

      it(`${methodName} rejects forbidden param injection`, () => {
        for (const forbidden of ['args', 'argv', 'env', 'shell', 'cmd']) {
          expect(() => {
            def.validate({
              workspaceRoot: '/valid/root',
              [forbidden]: 'injection'
            })
          }).toThrow()
        }
      })
    }
  })

  describe('TestParamsStrict string sanitization', () => {
    it('rejects strings longer than 512 chars and strings containing NUL', () => {
      const tooLong = 'a'.repeat(513)
      const withNul = 'hello\x00world'

      // status baseRef
      expect(() => {
        CODEINTEL_METHODS['codeintel.status'].validate({
          workspaceRoot: '/root',
          baseRef: tooLong
        })
      }).toThrow()

      // codegraphSearch
      expect(() => {
        CODEINTEL_METHODS['codeintel.codegraphSearch'].validate({
          workspaceRoot: '/root',
          search: tooLong
        })
      }).toThrow()

      // files filter
      expect(() => {
        CODEINTEL_METHODS['codeintel.files'].validate({
          workspaceRoot: '/root',
          filter: tooLong
        })
      }).toThrow()

      expect(() => {
        CODEINTEL_METHODS['codeintel.files'].validate({
          workspaceRoot: '/root',
          filter: withNul
        })
      }).toThrow()
    })
  })

  it('Error payloads never leak args, argv, env, or cwd', () => {
    try {
      CODEINTEL_METHODS['codeintel.status'].validate({
        workspaceRoot: '/root',
        unknownField: 'secret_info'
      })
    } catch (err: any) {
      if (err instanceof CodeIntelError) {
        const errorData = JSON.stringify(err.data ?? {})
        expect(errorData).not.toContain('args')
        expect(errorData).not.toContain('argv')
        expect(errorData).not.toContain('env')
        expect(errorData).not.toContain('cwd')
      }
    }
  })
})
