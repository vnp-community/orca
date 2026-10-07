import { describe, it, expect } from 'vitest'
import {
  assertSafeClientString,
  assertRelativeRepoPath,
  assertGitRef,
  validateCodeIntelParams,
  assertSchemaHasNoForbiddenParams,
  CodeIntelSchema
} from './codeintel-params-validation'
import { CodeIntelError } from './codeintel-errors'

describe('codeintel-params-validation', () => {
  describe('assertSafeClientString', () => {
    it('allows valid strings', () => {
      expect(() => assertSafeClientString('valid', 'field')).not.toThrow()
    })

    it('rejects empty strings', () => {
      expect(() => assertSafeClientString('', 'field')).toThrow(CodeIntelError)
    })

    it('rejects long strings', () => {
      expect(() => assertSafeClientString('a'.repeat(513), 'field')).toThrow(CodeIntelError)
    })

    it('rejects strings starting with hyphens', () => {
      expect(() => assertSafeClientString('-x', 'field')).toThrow(CodeIntelError)
      expect(() => assertSafeClientString('\uFF0Dx', 'field')).toThrow(CodeIntelError)
      expect(() => assertSafeClientString('\u2212x', 'field')).toThrow(CodeIntelError)
    })

    it('rejects control characters and NUL', () => {
      expect(() => assertSafeClientString('a\0b', 'field')).toThrow(CodeIntelError)
      expect(() => assertSafeClientString('a\nb', 'field')).toThrow(CodeIntelError)
    })
  })

  describe('assertRelativeRepoPath', () => {
    it('allows valid relative paths', () => {
      expect(() => assertRelativeRepoPath('src/main.ts', 'field')).not.toThrow()
      expect(() => assertRelativeRepoPath('package.json', 'field')).not.toThrow()
    })

    it('rejects absolute paths', () => {
      expect(() => assertRelativeRepoPath('/etc/passwd', 'field')).toThrow(CodeIntelError)
      expect(() => assertRelativeRepoPath('C:\\Windows', 'field')).toThrow(CodeIntelError)
    })

    it('rejects paths with parent traversal', () => {
      expect(() => assertRelativeRepoPath('../etc', 'field')).toThrow(CodeIntelError)
      expect(() => assertRelativeRepoPath('src/../../etc', 'field')).toThrow(CodeIntelError)
    })

    it('rejects paths with backslashes', () => {
      expect(() => assertRelativeRepoPath('src\\main.ts', 'field')).toThrow(CodeIntelError)
    })
  })

  describe('assertGitRef', () => {
    it('allows valid git refs', () => {
      expect(() => assertGitRef('main', 'field')).not.toThrow()
      expect(() => assertGitRef('feature/foo', 'field')).not.toThrow()
      expect(() => assertGitRef('v1.0.0', 'field')).not.toThrow()
    })

    it('rejects refs starting with hyphen', () => {
      expect(() => assertGitRef('-main', 'field')).toThrow(CodeIntelError)
    })

    it('rejects refs with invalid characters', () => {
      expect(() => assertGitRef('a b', 'field')).toThrow(CodeIntelError)
    })
  })

  describe('validateCodeIntelParams', () => {
    const spec: CodeIntelSchema = {
      name: { kind: 'string', required: true },
      count: { kind: 'int', default: 1 },
    }

    it('requires workspaceRoot', () => {
      expect(() => validateCodeIntelParams({}, spec)).toThrowError(/workspaceRoot.*required/)
    })

    it('rejects unknown params', () => {
      expect(() => validateCodeIntelParams({ workspaceRoot: '/foo', unknown: 1 }, spec))
        .toThrowError(/Unknown parameter: unknown/)
    })

    it('allows _trace', () => {
      const result = validateCodeIntelParams({ workspaceRoot: '/foo', name: 'x', _trace: true }, spec)
      expect(result._trace).toBe(true)
    })

    it('validates required fields', () => {
      expect(() => validateCodeIntelParams({ workspaceRoot: '/foo' }, spec))
        .toThrowError(/Param 'name' is required/)
    })
    
    it('uses default values', () => {
      const result = validateCodeIntelParams({ workspaceRoot: '/foo', name: 'x' }, spec)
      expect(result.count).toBe(1)
    })
  })

  describe('assertSchemaHasNoForbiddenParams', () => {
    it('throws if forbidden param is in schema', () => {
      const spec: CodeIntelSchema = {
        cwd: { kind: 'string' }
      }
      expect(() => assertSchemaHasNoForbiddenParams(spec)).toThrowError(/cwd/)
    })

    it('passes if no forbidden params', () => {
      const spec: CodeIntelSchema = {
        name: { kind: 'string' }
      }
      expect(() => assertSchemaHasNoForbiddenParams(spec)).not.toThrow()
    })
  })
})
