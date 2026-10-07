import * as fs from 'node:fs/promises'
import * as path from 'node:path'
import { runGit } from './codeintel-git-exec'
import { CompareRange } from './codeintel-merge-base-resolution'
import {
  parseRawZ,
  parseNumstatZ,
  parseUnifiedZero,
  matchHunksToFiles,
  DiffHunk,
  RawDiffEntry,
  NumstatEntry
} from './codeintel-diff-hunk-parser'

export interface ChangedFileEntry {
  path: string
  oldPath?: string
  status: string
  additions: number
  deletions: number
  hunks: DiffHunk[]
  untracked?: boolean
  binary?: boolean
  unsafePath?: boolean
}

export interface CollectDiffOptions {
  includeUntracked?: boolean
  maxBuffer?: number
}

export interface CollectDiffResult {
  changedFiles: ChangedFileEntry[]
  dirtyFiles: Set<string>
  warnings: string[]
  totalFilesCount: number
  hasMoreFiles: boolean
}

const MAX_CHANGED_FILES = 5000
const MAX_UNTRACKED_FILES = 200
const MAX_FILE_SIZE_BYTES = 2 * 1024 * 1024 // 2 MiB
const BINARY_PROBE_BYTES = 8192 // 8 KiB

async function isBinaryFile(filePath: string): Promise<boolean> {
  try {
    const handle = await fs.open(filePath, 'r')
    try {
      const buf = Buffer.alloc(BINARY_PROBE_BYTES)
      const { bytesRead } = await handle.read(buf, 0, BINARY_PROBE_BYTES, 0)
      for (let i = 0; i < bytesRead; i++) {
        if (buf[i] === 0) return true
      }
      return false
    } finally {
      await handle.close()
    }
  } catch {
    return false
  }
}

async function countLinesInFile(filePath: string): Promise<number> {
  try {
    const content = await fs.readFile(filePath, 'utf8')
    if (content.length === 0) return 0
    let lines = 0
    for (let i = 0; i < content.length; i++) {
      if (content.charCodeAt(i) === 10) lines++
    }
    if (content.charCodeAt(content.length - 1) !== 10) lines++
    return lines
  } catch {
    return 0
  }
}

export async function collectDiff(
  range: CompareRange,
  workspaceRoot: string,
  options: CollectDiffOptions = {}
): Promise<CollectDiffResult> {
  const warnings: string[] = []
  const dirtyFiles = new Set<string>()

  if (range.unborn || !range.mergeBase) {
    return {
      changedFiles: [],
      dirtyFiles,
      warnings,
      totalFilesCount: 0,
      hasMoreFiles: false
    }
  }

  const isWorkingTree = range.headRef === null && range.headOid !== null

  // 1. If comparing against working tree, collect dirty files and optionally untracked
  if (isWorkingTree) {
    try {
      const unstaged = await runGit(['-c', 'core.quotePath=false', 'diff', '--name-only', '-z', 'HEAD'], workspaceRoot)
      for (const p of unstaged.stdout.split('\0')) {
        if (p.trim()) dirtyFiles.add(p.trim())
      }
    } catch {
      // ignore
    }

    try {
      const staged = await runGit(['-c', 'core.quotePath=false', 'diff', '--name-only', '--cached', '-z', 'HEAD'], workspaceRoot)
      for (const p of staged.stdout.split('\0')) {
        if (p.trim()) dirtyFiles.add(p.trim())
      }
    } catch {
      // ignore
    }
  }

  // 2. Prepare diff arguments
  const baseArgs = ['-c', 'core.quotePath=false', 'diff', '--no-color', '--no-ext-diff', '--no-textconv', '-M']
  const targetRevision = isWorkingTree ? [] : [range.headOid!]

  // Run diff --raw -z
  const rawRes = await runGit(
    [...baseArgs, '--raw', '-z', '--no-abbrev', range.mergeBase, ...targetRevision],
    workspaceRoot
  )
  const rawEntries = parseRawZ(rawRes.stdout)

  // Run diff --numstat -z
  const numstatRes = await runGit(
    [...baseArgs, '--numstat', '-z', range.mergeBase, ...targetRevision],
    workspaceRoot
  )
  const numstatEntries = parseNumstatZ(numstatRes.stdout)
  const numstatMap = new Map<string, NumstatEntry>()
  for (const n of numstatEntries) {
    numstatMap.set(n.path, n)
  }

  // Run diff -U0 (unified=0)
  let hunksMap = new Map<string, DiffHunk[]>()
  try {
    const diffU0Res = await runGit(
      [...baseArgs, '-U0', range.mergeBase, ...targetRevision],
      workspaceRoot,
      { maxBuffer: options.maxBuffer }
    )
    if (diffU0Res.stderr.includes('rename detection was skipped')) {
      warnings.push('rename_detection_skipped')
    }
    const blocks = parseUnifiedZero(diffU0Res.stdout)
    const matchRes = matchHunksToFiles(rawEntries, blocks)
    hunksMap = matchRes.fileHunks
    for (const w of matchRes.warnings) {
      warnings.push(w)
    }
  } catch (err: any) {
    const isMaxBuffer =
      err?.code === 'ERR_CHILD_PROCESS_STDIO_MAXBUFFER' ||
      err?.message?.includes('maxBuffer')
    if (isMaxBuffer) {
      warnings.push('hunks_unavailable_diff_too_large')
      for (const r of rawEntries) {
        hunksMap.set(r.path, [])
      }
    } else {
      throw err
    }
  }

  // Combine git-tracked changes
  const combined: ChangedFileEntry[] = []

  for (const raw of rawEntries) {
    const num = numstatMap.get(raw.path)
    const hunks = hunksMap.get(raw.path) ?? []
    const isBinary = num?.binary ?? false

    combined.push({
      path: raw.path,
      oldPath: raw.oldPath,
      status: raw.status,
      additions: num?.additions ?? 0,
      deletions: num?.deletions ?? 0,
      hunks,
      binary: isBinary ? true : undefined,
      unsafePath: raw.unsafePath
    })
  }

  // 3. Collect untracked files if working tree and includeUntracked is not false
  if (isWorkingTree && options.includeUntracked !== false) {
    try {
      const untrackedRes = await runGit(
        ['-c', 'core.quotePath=false', 'ls-files', '--others', '--exclude-standard', '-z'],
        workspaceRoot
      )
      const untrackedList = untrackedRes.stdout.split('\0').filter(p => p.length > 0)
      if (untrackedList.length > MAX_UNTRACKED_FILES) {
        warnings.push('untracked_files_capped')
      }
      const filesToProcess = untrackedList.slice(0, MAX_UNTRACKED_FILES)

      for (const relPath of filesToProcess) {
        const fullPath = path.join(workspaceRoot, relPath)
        let stat: import('node:fs').Stats | undefined
        try {
          stat = await fs.stat(fullPath)
        } catch {
          continue
        }

        if (!stat.isFile()) continue

        dirtyFiles.add(relPath)

        if (stat.size > MAX_FILE_SIZE_BYTES) {
          // File too large -> treat as binary / no hunks
          combined.push({
            path: relPath,
            status: 'A',
            additions: 0,
            deletions: 0,
            hunks: [],
            untracked: true,
            binary: true
          })
          continue
        }

        const isBin = await isBinaryFile(fullPath)
        if (isBin) {
          combined.push({
            path: relPath,
            status: 'A',
            additions: 0,
            deletions: 0,
            hunks: [],
            untracked: true,
            binary: true
          })
        } else {
          const lines = await countLinesInFile(fullPath)
          const hunk: DiffHunk = {
            oldStart: 0,
            oldLines: 0,
            newStart: 1,
            newLines: lines,
            startLine: 1,
            endLine: lines === 0 ? 1 : lines,
            pureDeletion: false,
            header: `@@ -0,0 +1,${lines} @@`
          }

          combined.push({
            path: relPath,
            status: 'A',
            additions: lines,
            deletions: 0,
            hunks: [hunk],
            untracked: true
          })
        }
      }
    } catch {
      // ignore untracked error
    }
  }

  const totalFilesCount = combined.length
  const hasMoreFiles = totalFilesCount > MAX_CHANGED_FILES
  const changedFiles = combined.slice(0, MAX_CHANGED_FILES)

  return {
    changedFiles,
    dirtyFiles,
    warnings,
    totalFilesCount,
    hasMoreFiles
  }
}
