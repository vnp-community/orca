import { execFile } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'

export interface AddedLine {
  line: number
  text: string
}

export interface ChangedFileDiff {
  path: string
  status: 'A' | 'M' | 'R' | 'C' | 'T'
  oldPath?: string
  untracked: boolean
  binary: boolean
  addedLines: AddedLine[]
}

export interface AddedLinesResult {
  files: ChangedFileDiff[]
  warnings: string[]
  truncated: boolean
}

function execGit(args: string[], cwd: string): Promise<{ stdout: Buffer; stderr: string; code: number }> {
  return new Promise((resolve) => {
    execFile(
      'git',
      ['-c', 'core.quotePath=false', ...args],
      { cwd, maxBuffer: 30 * 1024 * 1024, encoding: 'buffer' },
      (error, stdout, stderr) => {
        resolve({
          stdout: stdout || Buffer.alloc(0),
          stderr: stderr ? stderr.toString('utf8') : '',
          code: error ? (typeof error.code === 'number' ? error.code : 1) : 0
        })
      }
    )
  })
}

export async function getAddedLines(
  root: string,
  spec: { mergeBaseOf: string | null; to: 'worktree' | 'HEAD' }
): Promise<AddedLinesResult> {
  const warnings: string[] = []
  let truncated = false

  let baseCommit = spec.mergeBaseOf
  if (spec.mergeBaseOf) {
    const mbRes = await execGit(['merge-base', 'HEAD', spec.mergeBaseOf], root)
    if (mbRes.code === 0 && mbRes.stdout.length > 0) {
      baseCommit = mbRes.stdout.toString('utf8').trim()
    } else {
      warnings.push(`merge_base_failed: ${mbRes.stderr.trim() || 'unknown'}`)
    }
  }

  const diffRef = baseCommit || 'HEAD'
  const diffArgs = spec.to === 'worktree' ? [diffRef] : [diffRef, 'HEAD']

  // 1. Raw diff with -z to get reliable file statuses and paths
  const rawRes = await execGit(['diff', '--raw', '-z', '--no-abbrev', '-M', ...diffArgs], root)
  if (rawRes.code !== 0 && rawRes.stderr) {
    warnings.push(`git_diff_raw_failed: ${rawRes.stderr.trim()}`)
  }

  const filesMap = new Map<string, ChangedFileDiff>()

  if (rawRes.code === 0 && rawRes.stdout.length > 0) {
    const rawTokens = rawRes.stdout.toString('utf8').split('\0')
    let i = 0
    while (i < rawTokens.length - 1) {
      const meta = rawTokens[i]
      if (!meta) {
        i++
        continue
      }
      const parts = meta.split(/\s+/)
      const statusToken = parts.at(-1) || 'M'
      const statusChar = statusToken[0] as 'A' | 'M' | 'R' | 'C' | 'T'
      i++
      const filePath = rawTokens[i]
      i++

      if (statusChar === 'R' || statusChar === 'C') {
        const newPath = rawTokens[i]
        i++
        filesMap.set(newPath, {
          path: newPath,
          oldPath: filePath,
          status: statusChar,
          untracked: false,
          binary: false,
          addedLines: []
        })
      } else {
        filesMap.set(filePath, {
          path: filePath,
          status: statusChar,
          untracked: false,
          binary: false,
          addedLines: []
        })
      }
    }
  }

  // 2. Unified diff with -U0 to parse added lines
  const patchRes = await execGit(['diff', '--unified=0', '-M', '--no-color', '--no-ext-diff', ...diffArgs], root)
  if (patchRes.code === 0 && patchRes.stdout.length > 0) {
    const patchText = patchRes.stdout.toString('utf8')
    parseUnifiedDiff(patchText, filesMap, warnings)
  }

  // 3. Untracked files (if inspecting worktree)
  if (spec.to === 'worktree') {
    const lsRes = await execGit(['ls-files', '--others', '--exclude-standard', '-z'], root)
    if (lsRes.code === 0 && lsRes.stdout.length > 0) {
      const untrackedTokens = lsRes.stdout.toString('utf8').split('\0').filter(Boolean)
      const maxUntracked = Math.min(untrackedTokens.length, 200)

      for (let j = 0; j < maxUntracked; j++) {
        const uPath = untrackedTokens[j]
        const fullPath = path.join(root, uPath)
        try {
          const stat = fs.statSync(fullPath)
          if (!stat.isFile() || stat.size > 2 * 1024 * 1024) continue

          const buf = fs.readFileSync(fullPath)
          if (buf.includes(0)) {
            // Binary file
            filesMap.set(uPath, {
              path: uPath,
              status: 'A',
              untracked: true,
              binary: true,
              addedLines: []
            })
            continue
          }

          const rawText = buf.toString('utf8')
          const fileLines = rawText.endsWith('\n') ? rawText.slice(0, -1).split(/\r?\n/) : rawText.split(/\r?\n/)
          const addedLines: AddedLine[] = []
          for (let lineIdx = 0; lineIdx < fileLines.length; lineIdx++) {
            let t = fileLines[lineIdx]
            if (t.length > 1000) {
              t = t.slice(0, 1000)
              truncated = true
            }
            addedLines.push({ line: lineIdx + 1, text: t })
          }

          filesMap.set(uPath, {
            path: uPath,
            status: 'A',
            untracked: true,
            binary: false,
            addedLines
          })
        } catch {}
      }
    }
  }

  const files = Array.from(filesMap.values())
  if (files.length > 5000) {
    truncated = true
    files.length = 5000
  }

  return {
    files,
    warnings,
    truncated
  }
}

function parseUnifiedDiff(
  patchText: string,
  filesMap: Map<string, ChangedFileDiff>,
  warnings: string[]
): void {
  const lines = patchText.split(/\r?\n/)
  let currentFile: ChangedFileDiff | null = null
  let currentLineNum = 0

  for (let idx = 0; idx < lines.length; idx++) {
    const line = lines[idx]

    if (line.startsWith('diff --git ')) {
      currentFile = null
      continue
    }

    if (line.startsWith('Binary files ') && line.endsWith('differ')) {
      if (currentFile) {
        currentFile.binary = true
      }
      continue
    }

    if (line.startsWith('+++ b/')) {
      const p = line.slice(6)
      currentFile = filesMap.get(p) || null
      if (!currentFile) {
        // May have been added
        currentFile = {
          path: p,
          status: 'M',
          untracked: false,
          binary: false,
          addedLines: []
        }
        filesMap.set(p, currentFile)
      }
      continue
    }

    if (line.startsWith('@@ ')) {
      // @@ -a[,b] +c[,d] @@
      const match = line.match(/@@\s+-[0-9]+(?:,[0-9]+)?\s+\+([0-9]+)(?:,([0-9]+))?\s+@@/)
      if (match) {
        currentLineNum = parseInt(match[1], 10)
      }
      continue
    }

    if (currentFile && line.startsWith('+') && !line.startsWith('+++')) {
      let text = line.slice(1)
      if (text.length > 1000) {
        text = text.slice(0, 1000)
      }
      currentFile.addedLines.push({
        line: currentLineNum,
        text
      })
      currentLineNum++
    }
  }
}
