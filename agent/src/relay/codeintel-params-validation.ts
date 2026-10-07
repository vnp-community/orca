import path from 'path'
import { CodeIntelError } from './codeintel-errors'

export type ParamKind = 'string' | 'int' | 'bool' | 'stringList' | 'enum' | 'object'

export interface ParamSpec {
  kind: ParamKind
  required?: boolean
  min?: number
  max?: number
  default?: any
  enum?: string[]
  maxLen?: number
}

export type CodeIntelSchema = Record<string, ParamSpec>

export const FORBIDDEN_PARAM_NAMES = [
  'args', 'argv', 'command', 'cmd', 'cwd', 'env', 'repo', 'cypher', 'shell', 'timeout', 'tool'
]

export function assertSchemaHasNoForbiddenParams(schema: CodeIntelSchema): void {
  for (const key of Object.keys(schema)) {
    if (FORBIDDEN_PARAM_NAMES.includes(key)) {
      throw new Error(`Schema contains forbidden parameter name: ${key}`)
    }
  }
}

export function assertSafeClientString(v: string, field: string): void {
  if (typeof v !== 'string') {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${field}' must be a string`, { field })
  }
  if (v.length < 1 || v.length > 512) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${field}' length must be 1..512`, { field })
  }
  // No NUL or control characters (U+0000 - U+001F, U+007F)
  if (/[\x00-\x1F\x7F]/.test(v)) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${field}' contains invalid control characters`, { field })
  }
  
  const normalized = v.normalize('NFKC')
  if (normalized.startsWith('-') || normalized.startsWith('\uFF0D') || normalized.startsWith('\u2212')) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${field}' cannot start with '-'`, { field })
  }
}

export function assertRelativeRepoPath(v: string, field: string): void {
  if (typeof v !== 'string') {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${field}' must be a string`, { field })
  }
  if (path.isAbsolute(v) || v.startsWith('/') || v.startsWith('\\')) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${field}' must be a relative path`, { field })
  }
  if (v.includes('..')) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${field}' cannot contain '..'`, { field })
  }
  if (v.includes('\\')) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${field}' cannot contain backslashes`, { field })
  }
}

export function assertGitRef(v: string, field: string): void {
  if (typeof v !== 'string') {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${field}' must be a string`, { field })
  }
  if (v.length < 1 || v.length > 256) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${field}' length must be 1..256`, { field })
  }
  if (v.startsWith('-')) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${field}' cannot start with '-'`, { field })
  }
  if (!/^[A-Za-z0-9._/@^~{}+-]+$/.test(v)) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${field}' contains invalid characters for git ref`, { field })
  }
}

export function validateCodeIntelParams(params: unknown, spec: CodeIntelSchema): Record<string, any> {
  if (!params || typeof params !== 'object') {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Params must be an object')
  }

  const p = params as Record<string, unknown>
  const result: Record<string, any> = {}
  
  // workspaceRoot is always required and validated here
  if (!p.workspaceRoot) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', "Param 'workspaceRoot' is required", { field: 'workspaceRoot' })
  }
  if (typeof p.workspaceRoot !== 'string') {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', "Param 'workspaceRoot' must be a string", { field: 'workspaceRoot' })
  }
  if (p.workspaceRoot.length > 4096 || p.workspaceRoot.includes('\x00')) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', "Param 'workspaceRoot' is invalid", { field: 'workspaceRoot' })
  }
  if (!path.isAbsolute(p.workspaceRoot)) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', "Param 'workspaceRoot' must be an absolute path", { field: 'workspaceRoot' })
  }
  result.workspaceRoot = p.workspaceRoot

  if ('_trace' in p) {
    result._trace = p._trace
  }

  for (const key of Object.keys(p)) {
    if (key === 'workspaceRoot' || key === '_trace') continue
    if (!spec[key]) {
      throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Unknown parameter: ${key}`, { field: key })
    }
  }

  for (const [key, fieldSpec] of Object.entries(spec)) {
    let val = p[key]
    
    if (val === undefined) {
      if (fieldSpec.required) {
        throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${key}' is required`, { field: key })
      }
      if (fieldSpec.default !== undefined) {
        val = fieldSpec.default
      } else {
        continue
      }
    }

    switch (fieldSpec.kind) {
      case 'string':
        if (typeof val !== 'string') throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${key}' must be a string`, { field: key })
        if (fieldSpec.maxLen && val.length > fieldSpec.maxLen) {
           throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${key}' exceeds max length`, { field: key })
        }
        break
      case 'int':
        if (typeof val !== 'number' || !Number.isInteger(val)) throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${key}' must be an integer`, { field: key })
        if (fieldSpec.min !== undefined && val < fieldSpec.min) throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${key}' < min`, { field: key })
        if (fieldSpec.max !== undefined && val > fieldSpec.max) throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${key}' > max`, { field: key })
        break
      case 'bool':
        if (typeof val !== 'boolean') throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${key}' must be a boolean`, { field: key })
        break
      case 'stringList':
        if (!Array.isArray(val) || !val.every(x => typeof x === 'string')) throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${key}' must be a string list`, { field: key })
        break
      case 'enum':
        if (typeof val !== 'string' || (fieldSpec.enum && !fieldSpec.enum.includes(val))) throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${key}' must be one of enum values`, { field: key })
        break
      case 'object':
        if (!val || typeof val !== 'object') throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Param '${key}' must be an object`, { field: key })
        break
    }
    
    result[key] = val
  }

  return result
}
