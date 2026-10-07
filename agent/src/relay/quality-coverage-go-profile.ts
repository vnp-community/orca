import fs from 'node:fs'
import path from 'node:path'

export interface CoverBlock {
  file: string
  startLine: number
  startCol: number
  endLine: number
  endCol: number
  numStmt: number
  count: number
}

export interface GoWorkModule {
  dir: string // relative to repoRoot
  modulePath: string
}

export interface ParseGoCoverProfileResult {
  blocks: CoverBlock[]
  unmappedBlocks: number
  mode: 'set' | 'count' | 'atomic'
}

export const COVERAGE_EXCLUDE_DEFAULTS = [
  '*.pb.go',
  '*_grpc.pb.go',
  '/gen/',
  '*_test.go',
  'testutil',
  'usecasetest/'
]

export function isFileExcluded(filePath: string, excludePatterns: string[] = COVERAGE_EXCLUDE_DEFAULTS): boolean {
  for (const pat of excludePatterns) {
    if (pat.startsWith('*') && pat.endsWith('.go')) {
      const suffix = pat.slice(1)
      if (filePath.endsWith(suffix)) return true
    } else if (filePath.includes(pat.replace(/^\/|\/$/g, ''))) {
      return true
    }
  }
  return false
}

export function readGoWorkModules(repoRoot: string): GoWorkModule[] {
  const goWorkPath = path.join(repoRoot, 'backend-go', 'go.work')
  if (!fs.existsSync(goWorkPath)) {
    return []
  }

  const content = fs.readFileSync(goWorkPath, 'utf8')
  const modules: GoWorkModule[] = []

  const useBlockMatch = content.match(/use\s*\(([\s\S]*?)\)/)
  const useDirs: string[] = []
  if (useBlockMatch) {
    const lines = useBlockMatch[1].split('\n')
    for (const line of lines) {
      const trimmed = line.trim()
      if (trimmed && !trimmed.startsWith('//')) {
        useDirs.push(trimmed)
      }
    }
  } else {
    const inlineMatches = content.matchAll(/use\s+([^\s]+)/g)
    for (const m of inlineMatches) {
      useDirs.push(m[1])
    }
  }

  for (const dirRel of useDirs) {
    const fullDir = path.resolve(path.join(repoRoot, 'backend-go'), dirRel)
    const goModPath = path.join(fullDir, 'go.mod')
    if (fs.existsSync(goModPath)) {
      const modContent = fs.readFileSync(goModPath, 'utf8')
      const modMatch = modContent.match(/^module\s+([^\s\r\n]+)/m)
      if (modMatch) {
        const relFromRepo = path.relative(repoRoot, fullDir)
        modules.push({
          dir: relFromRepo,
          modulePath: modMatch[1]
        })
      }
    }
  }

  return modules
}

export function parseGoCoverProfile(
  content: string,
  ctx: { modules: GoWorkModule[]; exclude?: string[] }
): ParseGoCoverProfileResult {
  const lines = content.split(/\r?\n/)
  let mode: 'set' | 'count' | 'atomic' = 'set'
  const blocks: CoverBlock[] = []
  let unmappedBlocks = 0
  const excludeRules = ctx.exclude ?? COVERAGE_EXCLUDE_DEFAULTS

  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed) continue
    if (trimmed.startsWith('mode:')) {
      const parts = trimmed.split(':')
      const parsedMode = parts[1]?.trim()
      if (parsedMode === 'count' || parsedMode === 'atomic' || parsedMode === 'set') {
        mode = parsedMode
      }
      continue
    }

    // Format: <importPath>/<file>.go:<sl>.<sc>,<el>.<ec> <numStmt> <count>
    const spaceIdx1 = trimmed.lastIndexOf(' ')
    if (spaceIdx1 === -1) continue
    const countStr = trimmed.slice(spaceIdx1 + 1)
    const rest1 = trimmed.slice(0, spaceIdx1)

    const spaceIdx2 = rest1.lastIndexOf(' ')
    if (spaceIdx2 === -1) continue
    const numStmtStr = rest1.slice(spaceIdx2 + 1)
    const locStr = rest1.slice(0, spaceIdx2)

    const colonIdx = locStr.indexOf(':')
    if (colonIdx === -1) continue
    const importFilePath = locStr.slice(0, colonIdx)
    const rangesStr = locStr.slice(colonIdx + 1)

    const commaIdx = rangesStr.indexOf(',')
    if (commaIdx === -1) continue
    const startParts = rangesStr.slice(0, commaIdx).split('.')
    const endParts = rangesStr.slice(commaIdx + 1).split('.')

    const startLine = parseInt(startParts[0], 10)
    const startCol = parseInt(startParts[1] || '0', 10)
    const endLine = parseInt(endParts[0], 10)
    const endCol = parseInt(endParts[1] || '0', 10)
    const numStmt = parseInt(numStmtStr, 10) || 1
    const count = parseInt(countStr, 10) || 0

    // Map importFilePath to repo relative file path
    let repoRelativeFile: string | null = null
    if (ctx.modules.length === 0) {
      repoRelativeFile = importFilePath
    } else {
      for (const mod of ctx.modules) {
        if (importFilePath.startsWith(mod.modulePath + '/')) {
          const subPath = importFilePath.slice(mod.modulePath.length + 1)
          repoRelativeFile = path.join(mod.dir, subPath)
          break
        } else if (importFilePath === mod.modulePath) {
          repoRelativeFile = mod.dir
          break
        }
      }
    }

    if (!repoRelativeFile) {
      unmappedBlocks++
      continue
    }

    if (isFileExcluded(repoRelativeFile, excludeRules)) {
      continue
    }

    blocks.push({
      file: repoRelativeFile,
      startLine,
      startCol,
      endLine,
      endCol,
      numStmt,
      count
    })
  }

  return {
    blocks,
    unmappedBlocks,
    mode
  }
}

export interface GoCoverFuncEntry {
  file: string
  line: number
  name: string
  pct: number
}

export function parseGoCoverFunc(text: string): GoCoverFuncEntry[] {
  const lines = text.split(/\r?\n/)
  const result: GoCoverFuncEntry[] = []

  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('total:')) continue

    // Format: path/file.go:line:\tfuncName\t100.0%
    const parts = trimmed.split(/\t+/)
    if (parts.length < 3) continue

    const locPart = parts[0]
    const funcName = parts[1]
    const pctStr = parts[2].replace('%', '')

    const colonMatch = locPart.match(/^(.*?):(\d+):?$/)
    if (!colonMatch) continue

    const file = colonMatch[1]
    const lineNum = parseInt(colonMatch[2], 10)
    const pct = parseFloat(pctStr) / 100

    result.push({
      file,
      line: lineNum,
      name: funcName,
      pct: isNaN(pct) ? 0 : pct
    })
  }

  return result
}

export interface FileCoverageRollup {
  file: string
  stmts: number
  coveredStmts: number
  pct: number
}

export function rollupFileCoverage(blocks: CoverBlock[]): Record<string, FileCoverageRollup> {
  const rollups: Record<string, { stmts: number; coveredStmts: number }> = {}

  for (const b of blocks) {
    if (!rollups[b.file]) {
      rollups[b.file] = { stmts: 0, coveredStmts: 0 }
    }
    rollups[b.file].stmts += b.numStmt
    if (b.count > 0) {
      rollups[b.file].coveredStmts += b.numStmt
    }
  }

  const result: Record<string, FileCoverageRollup> = {}
  for (const [f, data] of Object.entries(rollups)) {
    result[f] = {
      file: f,
      stmts: data.stmts,
      coveredStmts: data.coveredStmts,
      pct: data.stmts > 0 ? data.coveredStmts / data.stmts : 0
    }
  }

  return result
}
