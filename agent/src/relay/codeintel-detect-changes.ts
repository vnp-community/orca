import { validateCodeIntelParams, assertGitRef } from './codeintel-params-validation'
import { CodeIntelError } from './codeintel-errors'
import { CodeIntelRequestContext } from './codeintel-method-table'
import { resolveCompareRange } from './codeintel-merge-base-resolution'
import { collectDiff } from './codeintel-diff-collection'
import { mapHunksToSymbols } from './codeintel-hunk-symbol-mapping'
import { findAffectedFlows, findAffectedClusters, AffectedFlowEntry, AffectedClusterEntry } from './codeintel-detect-changes-flows'
import { parseGitNexusDetectChangesHeader, evaluateCrossCheck, GitNexusRiskHint } from './gitnexus-detect-changes-header'
import { probeGitNexusIndex } from './gitnexus-index-probe'
import { resolveCodeIntelRepo } from './codeintel-repo-resolution'
import { runCodeIntelTool } from './codeintel-tool-runner'

export interface DetectChangesParams {
  workspaceRoot: string
  base?: string
  head?: string
  includeUntracked?: boolean
  withClusters?: boolean
  crossCheck?: boolean
}

export function validateDetectChanges(params: any): DetectChangesParams {
  const validated = validateCodeIntelParams(params, {
    base: { kind: 'string', required: false, maxLen: 256 },
    head: { kind: 'string', required: false, maxLen: 256 },
    includeUntracked: { kind: 'bool', required: false },
    withClusters: { kind: 'bool', required: false },
    crossCheck: { kind: 'bool', required: false }
  })

  const { workspaceRoot, base, head, includeUntracked, withClusters, crossCheck } = validated

  if (base !== undefined) {
    assertGitRef(base, 'base')
  }

  if (head !== undefined) {
    assertGitRef(head, 'head')
  }

  return {
    workspaceRoot,
    base,
    head,
    includeUntracked,
    withClusters,
    crossCheck
  }
}

export async function handleDetectChanges(
  params: DetectChangesParams,
  ctx: CodeIntelRequestContext
): Promise<any> {
  const warnings: string[] = []
  const workspaceRoot = params.workspaceRoot
  const deadline = ctx.deadline || Date.now() + 55000

  // 1. Resolve compare range
  const range = await resolveCompareRange(
    { base: params.base, head: params.head },
    workspaceRoot
  )

  if (range.unborn) {
    warnings.push('unborn_head')
    return {
      base: range.baseRef,
      baseOid: range.baseOid,
      head: 'HEAD',
      headOid: null,
      mergeBase: null,
      changedFiles: [],
      changedSymbols: [],
      affectedFlows: [],
      affectedClusters: [],
      totalCount: 0,
      unmapped: {
        filesNotIndexed: [],
        filesWithoutSymbols: [],
        filesBeyondCap: 0,
        symbolsBeyondCap: 0
      },
      index: {
        commit: null,
        stale: false,
        driftedFileCount: 0,
        mappingConfidence: 'approximate'
      },
      riskHint: null,
      truncated: false,
      warnings
    }
  }

  // Check deadline before git diff
  if (Date.now() >= deadline) {
    throw new CodeIntelError('CODEINTEL_TIMEOUT', 'Timed out before diff collection started')
  }

  // 2. Collect diff
  const diffResult = await collectDiff(range, workspaceRoot, {
    includeUntracked: params.includeUntracked
  })
  warnings.push(...diffResult.warnings)

  // 3. Resolve gitnexus repo root & probe index
  let gitnexusRepoRoot: string | undefined
  let indexInfo: { indexed: boolean; commit?: string; stale?: boolean } = { indexed: false }

  try {
    const repoRes = await resolveCodeIntelRepo(workspaceRoot, 'gitnexus')
    gitnexusRepoRoot = repoRes.effectiveRoot
    const probeRes = await probeGitNexusIndex(gitnexusRepoRoot)
    indexInfo = {
      indexed: probeRes.indexed,
      commit: probeRes.commit,
      stale: probeRes.stale
    }
  } catch {
    warnings.push('index_missing')
  }

  let isTruncated = false

  // Check deadline before symbol mapping
  if (Date.now() >= deadline) {
    isTruncated = true
    warnings.push('deadline_partial')
  }

  // 4. Map hunks to symbols
  let mappedSymbols: any[] = []
  let unmapped = {
    filesNotIndexed: [] as string[],
    filesWithoutSymbols: [] as string[],
    filesBeyondCap: 0,
    symbolsBeyondCap: 0
  }
  let driftedFileCount = 0
  let overallConfidence: 'exact' | 'approximate' = 'approximate'

  if (!isTruncated) {
    const mapResult = await mapHunksToSymbols(diffResult.changedFiles, {
      workspaceRoot,
      gitnexusRepoRoot,
      indexedCommit: indexInfo.commit,
      headOid: range.headOid,
      dirtyFiles: diffResult.dirtyFiles
    })
    mappedSymbols = mapResult.mappedSymbols
    unmapped = mapResult.unmapped
    driftedFileCount = mapResult.driftedFileCount
    overallConfidence = mapResult.overallConfidence
    warnings.push(...mapResult.warnings)
  }

  // Check deadline before flow & cluster lookup
  if (!isTruncated && Date.now() >= deadline) {
    isTruncated = true
    warnings.push('deadline_partial')
  }

  // 5. Affected flows and clusters
  let affectedFlows: AffectedFlowEntry[] = []
  let affectedClusters: AffectedClusterEntry[] = []

  if (!isTruncated && gitnexusRepoRoot) {
    const symbolIds = mappedSymbols.map(m => m.symbol.uid)
    affectedFlows = await findAffectedFlows(symbolIds, {
      repoRoot: gitnexusRepoRoot
    })

    if (params.withClusters) {
      affectedClusters = await findAffectedClusters(symbolIds, {
        repoRoot: gitnexusRepoRoot,
        withClusters: true
      })
    }
  }

  // 6. CrossCheck with gitnexus CLI if requested
  let riskHint: GitNexusRiskHint | null = null
  if (!isTruncated && params.crossCheck && gitnexusRepoRoot && Date.now() + 2000 < deadline) {
    try {
      const toolRes = await runCodeIntelTool(
        'gitnexus',
        ['detect-changes'],
        {
          config: ctx.config,
          cwd: gitnexusRepoRoot,
          signal: ctx.signal,
          timeoutMs: Math.min(10000, Math.max(1000, deadline - Date.now()))
        }
      )
      const parsedHint = parseGitNexusDetectChangesHeader(toolRes.stdout)
      const evalRes = evaluateCrossCheck(parsedHint, {
        files: diffResult.changedFiles.length,
        symbols: mappedSymbols.length
      })
      riskHint = evalRes.riskHint
      warnings.push(...evalRes.warnings)
    } catch {
      // CLI error does not fail request
    }
  }

  return {
    base: range.baseRef,
    baseOid: range.baseOid,
    head: range.headRef ?? 'HEAD',
    headOid: range.headOid,
    mergeBase: range.mergeBase,
    changedFiles: diffResult.changedFiles.map(f => ({
      path: f.path,
      oldPath: f.oldPath,
      status: f.status,
      additions: f.additions,
      deletions: f.deletions,
      untracked: f.untracked,
      binary: f.binary
    })),
    changedSymbols: mappedSymbols.map(s => ({
      symbol: s.symbol,
      status: s.status,
      confidence: s.confidence,
      hunkCount: s.hunks.length,
      containers: s.containers
    })),
    affectedFlows,
    affectedClusters,
    totalCount: mappedSymbols.length + unmapped.symbolsBeyondCap,
    unmapped,
    index: {
      commit: indexInfo.commit ?? null,
      stale: indexInfo.stale ?? false,
      driftedFileCount,
      mappingConfidence: overallConfidence
    },
    riskHint,
    truncated: isTruncated,
    warnings
  }
}
