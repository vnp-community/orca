import { createHash } from 'crypto'
import { SECRET_ENV_NAME_PATTERN } from './quality-child-env'

export interface QualityCheckProfile {
  id: string
  kind?: string
  title: string
  parser: string
  argv: string[]
  cwd?: string
  env?: { set?: Record<string, string>; copy?: string[] }
  timeoutMs?: number
  maxOutputBytes?: number
  heavy?: boolean
  fileExtensions?: string[]
  enabled?: boolean
  network?: boolean
  scopes?: ('worktree' | 'changed' | 'commitRange')[]
  scopeStrategy?: 'append-files' | 'full-run-filter' | 'go-modules' | 'none'
  requires?: { file?: string; nodeModules?: boolean }[]
}

export interface QualitySuite {
  id: string
  title: string
  profiles: string[]
}

export type ArgvToken =
  | { type: 'literal'; value: string }
  | { type: 'bin'; name: string }
  | { type: 'files'; sep?: string }
  | { type: 'tmp'; path?: string }
  | { type: 'base' }
  | { type: 'gitCommonDir' }

export function parseArgvTemplate(token: string): ArgvToken[] {
  const res: ArgvToken[] = []
  const rx = /\{(bin:[^}]+|files(?:\|[^}]+)?|tmp:[^}]+|tmp|base|gitCommonDir)\}/g
  let last = 0
  let m: RegExpExecArray | null

  while ((m = rx.exec(token)) !== null) {
    if (m.index > last) {
      res.push({ type: 'literal', value: token.substring(last, m.index) })
    }
    const inner = m[1]
    if (inner.startsWith('bin:')) res.push({ type: 'bin', name: inner.substring(4) })
    else if (inner.startsWith('files')) {
      const sep = inner.length > 5 ? inner.substring(6) : undefined
      res.push({ type: 'files', sep })
    }
    else if (inner.startsWith('tmp:')) res.push({ type: 'tmp', path: inner.substring(4) })
    else if (inner === 'tmp') res.push({ type: 'tmp' })
    else if (inner === 'base') res.push({ type: 'base' })
    else if (inner === 'gitCommonDir') res.push({ type: 'gitCommonDir' })

    last = m.index + m[0].length
  }
  if (last < token.length) {
    res.push({ type: 'literal', value: token.substring(last) })
  }
  return res.length > 0 ? res : [{ type: 'literal', value: token }]
}

export function validateProfile(raw: unknown): { ok: true; value: QualityCheckProfile } | { ok: false; errors: string[] } {
  const errors: string[] = []
  if (!raw || typeof raw !== 'object') return { ok: false, errors: ['Not an object'] }

  const obj = raw as Record<string, any>

  if (typeof obj.id !== 'string' || !/^[a-z0-9][a-z0-9-]{0,47}$/.test(obj.id)) {
    errors.push('id must match ^[a-z0-9][a-z0-9-]{0,47}$')
  }
  if (typeof obj.title !== 'string') errors.push('title missing or invalid')
  if (typeof obj.parser !== 'string') errors.push('parser missing or invalid')

  if (obj.kind === 'repo-rules' && Array.isArray(obj.argv) && obj.argv.length === 0) {
    // Valid for in-process repo-rules
  } else if (!Array.isArray(obj.argv) || obj.argv.length === 0 || !obj.argv.every(x => typeof x === 'string')) {
    errors.push('argv must be a non-empty string array')
  } else {
    const a0 = obj.argv[0]
    if (a0 !== 'node' && !/^\{bin:[^}]+\}$/.test(a0)) {
      errors.push('argv[0] must be node or {bin:name}')
    }

    for (const arg of obj.argv) {
      const tokens = parseArgvTemplate(arg)
      if (tokens.some(t => t.type === 'files') && tokens.length > 1) {
        errors.push('{files} must be the entire argument')
      }
      // Check invalid braces
      const leftover = tokens.filter(t => t.type === 'literal').map(t => (t as any).value).join('')
      if (/\{[^}]+\}/.test(leftover)) {
        errors.push(`Invalid token in argv: ${arg}`)
      }
    }
  }

  if (obj.cwd !== undefined) {
    if (typeof obj.cwd !== 'string' || path.isAbsolute(obj.cwd) || obj.cwd.includes('..') || obj.cwd.includes('\\')) {
      errors.push('cwd must be relative, forward-slash only, no ..')
    }
  }

  if (obj.env !== undefined) {
    if (typeof obj.env !== 'object') {
      errors.push('env must be an object')
    } else {
      if (obj.env.set) {
        for (const k of Object.keys(obj.env.set)) {
          console.log('PATTERN:', SECRET_ENV_NAME_PATTERN, 'k:', k, 'test:', SECRET_ENV_NAME_PATTERN.test(k)); if (SECRET_ENV_NAME_PATTERN.test(k)) {
            errors.push(`env.set cannot contain secret-matching key ${k}`)
          }
        }
      }
    }
  }

  if (obj.timeoutMs !== undefined) {
    if (typeof obj.timeoutMs !== 'number' || obj.timeoutMs < 1 || obj.timeoutMs > 2700000) {
      errors.push('timeoutMs must be 1..2700000')
    }
  }
  if (obj.maxOutputBytes !== undefined) {
    if (typeof obj.maxOutputBytes !== 'number' || obj.maxOutputBytes < 0 || obj.maxOutputBytes > 67108864) {
      errors.push('maxOutputBytes must be <= 64 MiB')
    }
  }

  if (errors.length > 0) return { ok: false, errors }
  return { ok: true, value: obj as QualityCheckProfile }
}

function stableStringify(obj: any): string {
  if (obj === null || typeof obj !== 'object') return JSON.stringify(obj)
  if (Array.isArray(obj)) return `[${obj.map(stableStringify).join(',')}]`
  const keys = Object.keys(obj).sort()
  return `{${keys.map(k => `${JSON.stringify(k)}:${stableStringify(obj[k])}`).join(',')}}`
}

export function definitionHashOf(profile: QualityCheckProfile): string {
  const clone = { ...profile } as any
  delete clone.title
  const str = stableStringify(clone)
  return 'sha256:' + createHash('sha256').update(str).digest('hex')
}

export function displayOf(profile: QualityCheckProfile): string {
  const args = profile.argv.map(arg => {
    return arg.replace(/\{tmp(?::([^}]+))?\}/g, '<tmp>$1')
  }).join(' ')
  let d = `${profile.id}: ${args}`
  if (profile.cwd) d += ` (cwd: ${profile.cwd})`
  return d
}

import path from 'path'
