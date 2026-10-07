import { assertGitRef } from './codeintel-params-validation'
import { CodeIntelError } from './codeintel-errors'
import { runGit } from './codeintel-git-exec'

export interface CompareRange {
  baseRef: string
  baseOid: string
  headOid: string | null
  mergeBase: string | null
  headRef: string | null
  unborn: boolean
}

export interface ResolveCompareRangeOptions {
  base?: string
  head?: string
}

const DEFAULT_BASE_CANDIDATES = [
  'origin/main',
  'origin/master',
  'main',
  'master'
]

async function verifyCommitRef(ref: string, workspaceRoot: string): Promise<string | null> {
  try {
    const res = await runGit(['rev-parse', '--verify', '--quiet', '--end-of-options', `${ref}^{commit}`], workspaceRoot)
    const oid = res.stdout.trim()
    return oid.length > 0 ? oid : null
  } catch {
    return null
  }
}

export async function resolveCompareRange(
  options: ResolveCompareRangeOptions,
  workspaceRoot: string
): Promise<CompareRange> {
  const { base, head } = options

  // Validate ref syntax if provided
  if (base !== undefined) {
    assertGitRef(base, 'base')
  }
  if (head !== undefined) {
    assertGitRef(head, 'head')
  }

  // 1. Resolve base ref
  let baseRef: string
  let baseOid: string

  if (base !== undefined) {
    baseRef = base
    const resolved = await verifyCommitRef(base, workspaceRoot)
    if (!resolved) {
      throw new CodeIntelError(
        'CODEINTEL_INVALID_PARAMS',
        `Base ref '${base}' does not resolve to a valid commit`,
        { reason: 'unresolved_ref', ref: base }
      )
    }
    baseOid = resolved
  } else {
    // Attempt default base resolution via symbolic-ref refs/remotes/origin/HEAD without network
    let candidateRef: string | null = null
    try {
      const symRes = await runGit(['symbolic-ref', '--quiet', 'refs/remotes/origin/HEAD'], workspaceRoot)
      const symTarget = symRes.stdout.trim()
      if (symTarget.startsWith('refs/remotes/')) {
        candidateRef = symTarget.slice('refs/remotes/'.length)
      } else if (symTarget.length > 0) {
        candidateRef = symTarget
      }
    } catch {
      // ignore
    }

    let foundOid: string | null = null
    if (candidateRef) {
      foundOid = await verifyCommitRef(candidateRef, workspaceRoot)
      if (foundOid) {
        baseRef = candidateRef
      }
    }

    if (!foundOid) {
      for (const cand of DEFAULT_BASE_CANDIDATES) {
        const oid = await verifyCommitRef(cand, workspaceRoot)
        if (oid) {
          candidateRef = cand
          foundOid = oid
          break
        }
      }
    }

    if (!candidateRef || !foundOid) {
      throw new CodeIntelError(
        'CODEINTEL_INVALID_PARAMS',
        'Base ref could not be resolved automatically',
        { reason: 'base_required' }
      )
    }

    baseRef = candidateRef
    baseOid = foundOid
  }

  // 2. Resolve head ref
  let headRef: string | null = null
  let headOid: string | null = null
  let unborn = false

  if (head !== undefined) {
    headRef = head
    const resolved = await verifyCommitRef(head, workspaceRoot)
    if (!resolved) {
      throw new CodeIntelError(
        'CODEINTEL_INVALID_PARAMS',
        `Head ref '${head}' does not resolve to a valid commit`,
        { reason: 'unresolved_ref', ref: head }
      )
    }
    headOid = resolved
  } else {
    // Check if HEAD exists and resolves to commit
    const resolved = await verifyCommitRef('HEAD', workspaceRoot)
    if (!resolved) {
      unborn = true
      return {
        baseRef,
        baseOid,
        headOid: null,
        mergeBase: null,
        headRef: null,
        unborn: true
      }
    }
    headOid = resolved
    headRef = null
  }

  // 3. Resolve merge-base
  let mergeBase: string
  try {
    const mbRes = await runGit(['merge-base', '--end-of-options', baseOid, headOid], workspaceRoot)
    mergeBase = mbRes.stdout.trim()
    if (!mergeBase) {
      throw new Error('empty merge-base')
    }
  } catch {
    throw new CodeIntelError(
      'CODEINTEL_INVALID_PARAMS',
      `No merge base found between ${baseRef} and ${headRef ?? 'HEAD'}`,
      { reason: 'no_merge_base', base: baseRef, head: headRef ?? 'HEAD' }
    )
  }

  return {
    baseRef,
    baseOid,
    headOid,
    mergeBase,
    headRef,
    unborn: false
  }
}
