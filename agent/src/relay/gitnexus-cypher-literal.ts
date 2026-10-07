import { CodeIntelError } from './codeintel-errors'

export const GITNEXUS_EDGE_KINDS = ['CALLS', 'IMPLEMENTS', 'INHERITS', 'IMPORTS', 'EXPORTS', 'DEFINES', 'REFERENCES']

export function cypherInt(n: any, min: number, max: number): string {
  if (typeof n !== 'number' || !Number.isInteger(n) || n < min || n > max) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Integer out of range [${min}, ${max}]: ${n}`)
  }
  return String(n)
}

export function cypherString(s: any): string {
  if (typeof s !== 'string') {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Expected a string')
  }
  if (s.length < 1 || s.length > 512) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `String length out of bounds: ${s.length}`)
  }
  if (/[\x00-\x1F\x7F]/.test(s)) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'String contains control characters')
  }

  const escaped = s.replace(/\\/g, '\\\\').replace(/'/g, "\\'")
  return `'${escaped}'`
}

export function cypherStringList(arr: any): string {
  if (!Array.isArray(arr) || arr.length === 0 || arr.length > 300) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Expected a string array of length 1..300')
  }
  const elements = arr.map(item => cypherString(item))
  return `[${elements.join(', ')}]`
}

export function cypherKindList(arr: any, allowed: string[] = GITNEXUS_EDGE_KINDS): string {
  if (!Array.isArray(arr) || arr.length === 0 || arr.length > 300) {
    throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Expected an array of kinds')
  }
  const elements = arr.map(item => {
    if (typeof item !== 'string' || !allowed.includes(item)) {
      throw new CodeIntelError('CODEINTEL_INVALID_PARAMS', `Invalid kind: ${item}`)
    }
    return cypherString(item)
  })
  return `[${elements.join(', ')}]`
}
