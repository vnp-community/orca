import path from 'node:path'
import fs from 'node:fs'
import type { CoverBlock } from './quality-coverage-go-profile'

export interface IstanbulStatementLoc {
  start: { line: number; column: number }
  end: { line: number; column: number }
}

export interface IstanbulFileCoverage {
  path: string
  statementMap: Record<string, IstanbulStatementLoc>
  s: Record<string, number>
}

export function parseVitestCoverageFinal(
  jsonContent: string,
  workspaceRoot: string
): CoverBlock[] {
  let parsed: Record<string, IstanbulFileCoverage>
  try {
    parsed = typeof jsonContent === 'string' ? JSON.parse(jsonContent) : jsonContent
  } catch {
    return []
  }

  const blocks: CoverBlock[] = []

  for (const [absOrRelPath, fileData] of Object.entries(parsed)) {
    if (!fileData || !fileData.statementMap || !fileData.s) continue

    let relPath = absOrRelPath
    if (path.isAbsolute(absOrRelPath)) {
      if (!absOrRelPath.startsWith(workspaceRoot)) {
        // Outside workspace, skip
        continue
      }
      relPath = path.relative(workspaceRoot, absOrRelPath)
    }

    for (const [key, loc] of Object.entries(fileData.statementMap)) {
      const count = fileData.s[key] ?? 0
      blocks.push({
        file: relPath,
        startLine: loc.start.line,
        startCol: loc.start.column,
        endLine: loc.end.line,
        endCol: loc.end.column,
        numStmt: 1,
        count
      })
    }
  }

  return blocks
}

export function preflightVitestCoverage(pkgDir: string): { ready: boolean; reason?: string } {
  const providerPath = path.join(pkgDir, 'node_modules', '@vitest', 'coverage-v8')
  if (!fs.existsSync(providerPath)) {
    return {
      ready: false,
      reason: 'coverage_provider_missing'
    }
  }

  return { ready: true }
}

export const COVERAGE_TS_PROFILE = {
  id: 'coverage-ts',
  title: 'TypeScript Coverage (Vitest)',
  parser: 'coverage@vitest',
  cwd: '.',
  argv: [
    '{bin:vitest}',
    'run',
    '--coverage.enabled',
    '--coverage.provider=v8',
    '--coverage.reporter=json',
    '--coverage.reportsDirectory={tmp:coverage}'
  ],
  scopes: ['worktree', 'changed', 'commitRange'],
  scopeStrategy: 'none',
  timeoutMs: 300000,
  maxOutputBytes: 20 * 1024 * 1024,
  heavy: true,
  enabled: false, // Gated pending approval (O12)
  env: { set: {}, allowExtra: [] }
} as const
