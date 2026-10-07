import { execFile } from 'child_process'
import { promisify } from 'util'
import path from 'path'

const exec = promisify(execFile)

export function isSafeBaseRef(base: string): boolean {
  if (typeof base !== 'string' || base.length === 0) return false
  if (/^[0-9a-fA-F]{7,64}$/.test(base)) return true

  // basic check-ref-format rules without calling git:
  // no spaces, no control chars
  if (/[^\x21-\x7e]/.test(base)) return false
  // no two consecutive dots
  if (base.includes('..')) return false
  // no @{
  if (base.includes('@{')) return false
  // no multiple slashes //
  if (base.includes('//')) return false
  // no specific bad chars
  if (/[~^:?*[\\]/.test(base)) return false
  // doesn't end with . or / or .lock
  if (base.endsWith('.') || base.endsWith('/') || base.endsWith('.lock')) return false
  // doesn't start with / or -
  if (base.startsWith('/') || base.startsWith('-')) return false

  return true
}

export class QualityParamsError extends Error {
  constructor(public reason: string) {
    super(`Invalid params: ${reason}`)
  }
}

export interface ChangedFilesOpts {
  root: string
  scope: 'worktree' | 'changed' | 'commitRange'
  base?: string
}

function isValidPath(p: string, root: string): boolean {
  // no NUL or control chars
  if (/[\x00-\x1f\x7f]/.test(p)) return false
  // no starting with - (command injection risk if passed to other commands)
  if (p.startsWith('-')) return false
  // must be inside root when resolved
  const resolved = path.resolve(root, p)
  if (!resolved.startsWith(root + path.sep) && resolved !== root) return false
  return true
}

export async function changedFiles({ root, scope, base }: ChangedFilesOpts): Promise<string[] | null> {
  if (scope === 'worktree') return null

  if (scope === 'changed' || scope === 'commitRange') {
    if (!base) throw new QualityParamsError('base_required')
    if (!isSafeBaseRef(base)) throw new QualityParamsError('invalid_base')

    // check if base exists
    try {
      await exec('git', ['rev-parse', '--verify', '--quiet', base + '^{commit}'], { cwd: root })
    } catch {
      throw new QualityParamsError('unresolved_ref')
    }
  }

  const files = new Set<string>()

  // 1. diff <base>...HEAD
  if (scope === 'changed' || scope === 'commitRange') {
    try {
      const { stdout } = await exec('git', ['-c', 'core.quotePath=false', 'diff', '--name-only', '-z', '--diff-filter=ACMR', `${base}...HEAD`], { cwd: root, maxBuffer: 10 * 1024 * 1024 })
      const chunks = stdout.split('\0')
      for (const c of chunks) {
        if (c && isValidPath(c, root)) files.add(c)
      }
    } catch (e: any) {
      if (e.stderr?.includes('Not a valid object name') || e.stderr?.includes('no merge base')) {
        throw new QualityParamsError('no_merge_base')
      }
      throw e
    }
  }

  // 2. status for changed
  if (scope === 'changed') {
    const { stdout } = await exec('git', ['-c', 'core.quotePath=false', 'status', '--porcelain=v1', '-z', '--untracked-files=normal'], { cwd: root, maxBuffer: 10 * 1024 * 1024 })
    const chunks = stdout.split('\0')
    let i = 0
    while (i < chunks.length) {
      if (!chunks[i]) {
        i++
        continue
      }
      const st = chunks[i].substring(0, 2)
      const p = chunks[i].substring(3)
      // ACMR include untracked
      if (!st.includes('D')) {
        if (isValidPath(p, root)) files.add(p)
      }
      if (st[0] === 'R' || st[0] === 'C') {
        // renamed/copied have old path next
        i++ // skip old path
      }
      i++
    }
  }

  return Array.from(files)
}
