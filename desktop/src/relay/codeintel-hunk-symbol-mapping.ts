import { runGit } from './codeintel-git-exec'
import { buildSymbolRef, SymbolRef } from './codeintel-symbol-ref'
import { ChangedFileEntry } from './codeintel-diff-collection'
import { DiffHunk } from './codeintel-diff-hunk-parser'
import { runCypherTemplate } from './gitnexus-cypher-runner'

export interface MappedSymbolEntry {
  symbol: SymbolRef
  status: 'modified' | 'added' | 'deleted'
  confidence: 'exact' | 'approximate'
  hunks: DiffHunk[]
  containers?: string[]
}

export interface UnmappedDetails {
  filesNotIndexed: string[]
  filesWithoutSymbols: string[]
  filesBeyondCap: number
  symbolsBeyondCap: number
}

export interface MapHunksToSymbolsResult {
  mappedSymbols: MappedSymbolEntry[]
  unmapped: UnmappedDetails
  fileConfidences: Map<string, 'exact' | 'approximate'>
  driftedFileCount: number
  overallConfidence: 'exact' | 'approximate'
  warnings: string[]
}

export interface MapHunksToSymbolsOptions {
  workspaceRoot: string
  gitnexusRepoRoot?: string
  indexedCommit?: string
  headOid: string | null
  dirtyFiles: Set<string>
  runCypher?: typeof runCypherTemplate
  execGit?: typeof runGit
}

const MAX_MAPPED_FILES = 1000
const MAX_MAPPED_SYMBOLS = 2000
const BATCH_SIZE = 100

export async function mapHunksToSymbols(
  changedFiles: ChangedFileEntry[],
  options: MapHunksToSymbolsOptions
): Promise<MapHunksToSymbolsResult> {
  const {
    workspaceRoot,
    gitnexusRepoRoot,
    indexedCommit,
    headOid,
    dirtyFiles,
    runCypher = runCypherTemplate,
    execGit = runGit
  } = options

  const warnings: string[] = []
  const fileConfidences = new Map<string, 'exact' | 'approximate'>()
  const repoRoot = gitnexusRepoRoot ?? workspaceRoot

  // 1. Determine confidence per file based on git diff between indexedCommit and headOid
  let commitReachable = false
  let indexedCommitDiffFiles = new Set<string>()

  if (indexedCommit) {
    try {
      await execGit(['cat-file', '-e', `${indexedCommit}^{commit}`], repoRoot)
      commitReachable = true
    } catch {
      warnings.push('index_commit_unreachable')
    }

    if (commitReachable && headOid) {
      if (indexedCommit !== headOid) {
        warnings.push('index_commit_differs_from_head')
        try {
          const diffRes = await execGit(
            ['-c', 'core.quotePath=false', 'diff', '--name-only', '-z', indexedCommit, headOid],
            repoRoot
          )
          for (const f of diffRes.stdout.split('\0')) {
            if (f.trim()) indexedCommitDiffFiles.add(f.trim())
          }
        } catch {
          // ignore
        }
      }
    }
  }

  for (const file of changedFiles) {
    const isDirty = dirtyFiles.has(file.path)
    const isCommitDiff = indexedCommitDiffFiles.has(file.path)

    if (!commitReachable || isDirty || isCommitDiff || !indexedCommit || !headOid) {
      fileConfidences.set(file.path, 'approximate')
    } else {
      fileConfidences.set(file.path, 'exact')
    }
  }

  // 2. Select files to map (limit 1000)
  const filesToMap = changedFiles.filter(f => f.hunks.length > 0 || f.status === 'D')
  const totalFilesToMap = filesToMap.length
  const filesBeyondCap = totalFilesToMap > MAX_MAPPED_FILES ? totalFilesToMap - MAX_MAPPED_FILES : 0
  const candidateFiles = filesToMap.slice(0, MAX_MAPPED_FILES)

  // 3. Batch query FILE_SYMBOLS_BATCH
  const fileSymbolsMap = new Map<string, SymbolRef[]>()
  const indexedFilesSet = new Set<string>()

  const batches: string[][] = []
  for (let i = 0; i < candidateFiles.length; i += BATCH_SIZE) {
    batches.push(candidateFiles.slice(i, i + BATCH_SIZE).map(f => f.path))
  }

  // Run in chunks of max 3 parallel
  for (let i = 0; i < batches.length; i += 3) {
    const chunk = batches.slice(i, i + 3)
    await Promise.all(
      chunk.map(async (paths) => {
        if (paths.length === 0) return
        try {
          const cypherRes = await runCypher('FILE_SYMBOLS_BATCH', { paths }, { repoRoot })
          for (const row of cypherRes.rows) {
            const filePath = String(row['n.filePath'] ?? '')
            const id = String(row['n.id'] ?? '')
            const label = String(row['label(n)'] ?? '')
            const name = String(row['n.name'] ?? '')
            const sLine = row['n.startLine'] !== null && row['n.startLine'] !== undefined ? Number(row['n.startLine']) : null
            const eLine = row['n.endLine'] !== null && row['n.endLine'] !== undefined ? Number(row['n.endLine']) : null

            indexedFilesSet.add(filePath)
            if (!fileSymbolsMap.has(filePath)) {
              fileSymbolsMap.set(filePath, [])
            }

            // Exclude File nodes from symbol mapping
            if (label.toLowerCase() === 'file') continue

            const sym = buildSymbolRef({
              id,
              label,
              name,
              filePath,
              startLine: sLine,
              endLine: eLine
            })
            fileSymbolsMap.get(filePath)!.push(sym)
          }
        } catch {
          // ignore batch query failure
        }
      })
    )
  }

  // 4. Map hunks to symbols
  const unmapped: UnmappedDetails = {
    filesNotIndexed: [],
    filesWithoutSymbols: [],
    filesBeyondCap,
    symbolsBeyondCap: 0
  }

  const symbolMap = new Map<string, MappedSymbolEntry>()

  for (const file of candidateFiles) {
    const conf = fileConfidences.get(file.path) ?? 'approximate'
    const symbols = fileSymbolsMap.get(file.path) ?? []

    if (!indexedFilesSet.has(file.path)) {
      unmapped.filesNotIndexed.push(file.path)
      continue
    }

    if (symbols.length === 0) {
      unmapped.filesWithoutSymbols.push(file.path)
      continue
    }

    if (file.status === 'D') {
      // Deleted file: mark all existing symbols as deleted
      for (const sym of symbols) {
        symbolMap.set(sym.key, {
          symbol: sym,
          status: 'deleted',
          confidence: 'approximate',
          hunks: []
        })
      }
      continue
    }

    // For modified/added files, match hunks
    for (const hunk of file.hunks) {
      const hStart = hunk.startLine
      const hEnd = hunk.endLine

      // Find symbols overlapping: s <= se && e >= ss
      const overlapping: SymbolRef[] = []
      for (const sym of symbols) {
        if (sym.startLine === null || sym.endLine === null) continue
        if (hStart <= sym.endLine && hEnd >= sym.startLine) {
          overlapping.push(sym)
        }
      }

      if (overlapping.length === 0) {
        // No symbol found, do not invent symbols for unindexed code
        continue
      }

      // Select innermost symbol
      // Sort priority:
      // 1. Non-value before value (prefer function/method/type over variable/const)
      // 2. Smaller line range (innermost)
      overlapping.sort((a, b) => {
        const aIsVal = a.kind === 'value' || a.kind === 'doc'
        const bIsVal = b.kind === 'value' || b.kind === 'doc'
        if (aIsVal !== bIsVal) {
          return aIsVal ? 1 : -1
        }
        const aSpan = (a.endLine ?? 0) - (a.startLine ?? 0)
        const bSpan = (b.endLine ?? 0) - (b.startLine ?? 0)
        return aSpan - bSpan
      })

      const best = overlapping[0]

      // Identify container if best is a method
      const containers: string[] = []
      for (const other of overlapping) {
        if (other.key !== best.key && (other.kind === 'type' || other.kind === 'function')) {
          containers.push(other.name)
        }
      }

      const existing = symbolMap.get(best.key)
      if (existing) {
        existing.hunks.push(hunk)
        if (containers.length > 0 && !existing.containers) {
          existing.containers = containers
        }
      } else {
        symbolMap.set(best.key, {
          symbol: best,
          status: file.status === 'A' ? 'added' : 'modified',
          confidence: conf,
          hunks: [hunk],
          containers: containers.length > 0 ? containers : undefined
        })
      }
    }
  }

  // 5. Cap symbols at 2000
  const allMapped = Array.from(symbolMap.values())
  if (allMapped.length > MAX_MAPPED_SYMBOLS) {
    // Prioritize by total hunk lines
    allMapped.sort((a, b) => {
      const aLines = a.hunks.reduce((acc, h) => acc + (h.endLine - h.startLine + 1), 0)
      const bLines = b.hunks.reduce((acc, h) => acc + (h.endLine - h.startLine + 1), 0)
      return bLines - aLines
    })
    unmapped.symbolsBeyondCap = allMapped.length - MAX_MAPPED_SYMBOLS
    warnings.push('symbols_beyond_cap')
  }

  const mappedSymbols = allMapped.slice(0, MAX_MAPPED_SYMBOLS)

  let driftedFileCount = 0
  for (const c of fileConfidences.values()) {
    if (c === 'approximate') driftedFileCount++
  }

  const overallConfidence = driftedFileCount === 0 && fileConfidences.size > 0 ? 'exact' : 'approximate'

  return {
    mappedSymbols,
    unmapped,
    fileConfidences,
    driftedFileCount,
    overallConfidence,
    warnings
  }
}
