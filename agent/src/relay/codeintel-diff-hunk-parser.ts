export interface RawDiffEntry {
  oldMode: string
  newMode: string
  oldSha: string
  newSha: string
  status: string
  score?: number
  path: string
  oldPath?: string
  unsafePath?: boolean
}

export interface NumstatEntry {
  additions: number
  deletions: number
  binary: boolean
  path: string
  oldPath?: string
}

export interface DiffHunk {
  oldStart: number
  oldLines: number
  newStart: number
  newLines: number
  startLine: number
  endLine: number
  pureDeletion: boolean
  header: string
}

export interface FileHunkBlock {
  oldPath?: string
  newPath?: string
  hunks: DiffHunk[]
}

export interface MatchHunksResult {
  fileHunks: Map<string, DiffHunk[]>
  warnings: string[]
}

function hasControlChars(str: string): boolean {
  // ASCII 0-31 except tab/newline or DEL 127
  for (let i = 0; i < str.length; i++) {
    const code = str.charCodeAt(i)
    if (code < 32 || code === 127) {
      return true
    }
  }
  return false
}

export function parseRawZ(rawOutput: string): RawDiffEntry[] {
  if (!rawOutput) return []
  const tokens = rawOutput.split('\0')
  const entries: RawDiffEntry[] = []

  let i = 0
  while (i < tokens.length) {
    const meta = tokens[i].trim()
    if (!meta) {
      i++
      continue
    }

    if (!meta.startsWith(':')) {
      i++
      continue
    }

    // Format: :oldMode newMode oldSha newSha statusScore
    const parts = meta.slice(1).trim().split(/\s+/)
    if (parts.length < 5) {
      i++
      continue
    }

    const [oldMode, newMode, oldSha, newSha, statusField] = parts
    const status = statusField[0]
    const score = statusField.length > 1 ? parseInt(statusField.slice(1), 10) : undefined

    let path = ''
    let oldPath: string | undefined

    if (status === 'R' || status === 'C') {
      oldPath = tokens[i + 1] ?? ''
      path = tokens[i + 2] ?? ''
      i += 3
    } else {
      path = tokens[i + 1] ?? ''
      i += 2
    }

    const unsafePath = hasControlChars(path) || (oldPath !== undefined && hasControlChars(oldPath))

    entries.push({
      oldMode,
      newMode,
      oldSha,
      newSha,
      status,
      score: Number.isNaN(score) ? undefined : score,
      path,
      oldPath,
      unsafePath: unsafePath ? true : undefined
    })
  }

  return entries
}

export function parseNumstatZ(numstatOutput: string): NumstatEntry[] {
  if (!numstatOutput) return []
  const tokens = numstatOutput.split('\0')
  const entries: NumstatEntry[] = []

  let i = 0
  while (i < tokens.length) {
    const token = tokens[i]
    if (!token) {
      i++
      continue
    }

    // Typically: "<additions>\t<deletions>\t<path>" or "<additions>\t<deletions>\t" followed by paths
    const tabParts = token.split('\t')
    if (tabParts.length >= 2) {
      const addStr = tabParts[0].trim()
      const delStr = tabParts[1].trim()
      const isBinary = addStr === '-' && delStr === '-'
      const additions = isBinary ? 0 : parseInt(addStr, 10) || 0
      const deletions = isBinary ? 0 : parseInt(delStr, 10) || 0

      let path = ''
      let oldPath: string | undefined

      if (tabParts.length >= 3 && tabParts[2].length > 0) {
        path = tabParts[2]
        i++
      } else {
        // Renamed files in -z mode: next token is oldPath, subsequent token is newPath
        oldPath = tokens[i + 1] ?? ''
        path = tokens[i + 2] ?? ''
        i += 3
      }

      entries.push({
        additions,
        deletions,
        binary: isBinary,
        path,
        oldPath: oldPath || undefined
      })
    } else {
      i++
    }
  }

  return entries
}

const HUNK_REGEX = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@/

export function parseUnifiedZero(diffText: string): FileHunkBlock[] {
  if (!diffText) return []

  const blocks: FileHunkBlock[] = []
  const lines = diffText.split('\n')

  let currentBlock: FileHunkBlock | null = null

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]

    if (line.startsWith('diff --git ')) {
      if (currentBlock) {
        blocks.push(currentBlock)
      }
      currentBlock = { hunks: [] }
      continue
    }

    if (line.startsWith('--- ')) {
      if (currentBlock) {
        const rawOld = line.slice(4).trim()
        currentBlock.oldPath = rawOld.startsWith('a/') ? rawOld.slice(2) : rawOld
      }
      continue
    }

    if (line.startsWith('+++ ')) {
      if (currentBlock) {
        const rawNew = line.slice(4).trim()
        currentBlock.newPath = rawNew.startsWith('b/') ? rawNew.slice(2) : rawNew
      }
      continue
    }

    if (line.startsWith('@@ ')) {
      if (!currentBlock) {
        currentBlock = { hunks: [] }
      }
      const match = line.match(HUNK_REGEX)
      if (match) {
        const oldStart = parseInt(match[1], 10)
        const oldLines = match[2] !== undefined ? parseInt(match[2], 10) : 1
        const newStart = parseInt(match[3], 10)
        const newLines = match[4] !== undefined ? parseInt(match[4], 10) : 1
        const pureDeletion = newLines === 0

        const startLine = newStart === 0 ? 1 : newStart
        const endLine = pureDeletion ? startLine : startLine + newLines - 1

        currentBlock.hunks.push({
          oldStart,
          oldLines,
          newStart,
          newLines,
          startLine,
          endLine,
          pureDeletion,
          header: line
        })
      }
    }
  }

  if (currentBlock) {
    blocks.push(currentBlock)
  }

  return blocks
}

export function matchHunksToFiles(
  rawEntries: RawDiffEntry[],
  hunkBlocks: FileHunkBlock[]
): MatchHunksResult {
  const fileHunks = new Map<string, DiffHunk[]>()
  const warnings: string[] = []

  // Check order mismatch or count mismatch
  // A raw entry may not have a hunk block if it was binary or pure mode change
  // But for text changes, the order of hunkBlocks matches rawEntries
  let blockIndex = 0
  let orderMismatch = false

  for (const raw of rawEntries) {
    fileHunks.set(raw.path, [])
  }

  for (const block of hunkBlocks) {
    const blockPath = block.newPath !== '/dev/null' ? block.newPath : block.oldPath
    if (!blockPath) continue

    // Find in rawEntries starting from blockIndex
    let foundIndex = -1
    for (let i = blockIndex; i < rawEntries.length; i++) {
      if (rawEntries[i].path === blockPath || rawEntries[i].oldPath === blockPath) {
        foundIndex = i
        break
      }
    }

    if (foundIndex === -1) {
      // Check anywhere
      foundIndex = rawEntries.findIndex(r => r.path === blockPath || r.oldPath === blockPath)
      orderMismatch = true
    } else if (foundIndex !== blockIndex) {
      // Skipped some files (maybe binary or mode change), which is normal in git diff
      blockIndex = foundIndex
    }

    if (foundIndex !== -1) {
      const targetPath = rawEntries[foundIndex].path
      fileHunks.set(targetPath, block.hunks)
      blockIndex = foundIndex + 1
    } else {
      orderMismatch = true
    }
  }

  if (orderMismatch) {
    warnings.push('hunk_file_order_mismatch')
  }

  return { fileHunks, warnings }
}
