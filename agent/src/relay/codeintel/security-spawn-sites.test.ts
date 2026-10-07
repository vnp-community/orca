import { describe, it, expect } from 'vitest'
import fs from 'fs'
import path from 'path'

const ALLOWED_CHILD_PROCESS_FILES = new Set([
  'codeintel-child-process-rss-sampler.ts',
  'child-process-rss-sampler.ts',
  'codeintel-git-exec.ts',
  'codeintel-index-basis-probe.ts',
  'codeintel-tool-detection.ts',
  'codeintel-reindex-runner.ts',
  'codeintel-tool-runner.ts'
])

function findCodeIntelSourceFiles(dir: string): string[] {
  const results: string[] = []
  const entries = fs.readdirSync(dir, { withFileTypes: true })
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      if (entry.name !== '__fixtures__' && entry.name !== 'node_modules') {
        results.push(...findCodeIntelSourceFiles(fullPath))
      }
    } else if (
      entry.isFile() &&
      entry.name.endsWith('.ts') &&
      !entry.name.endsWith('.test.ts') &&
      !entry.name.endsWith('.spec.ts') &&
      (entry.name.startsWith('codeintel') || dir.includes('codeintel'))
    ) {
      results.push(fullPath)
    }
  }
  return results
}

describe('security-spawn-sites (TestSpawnCallsOnlyInRunner)', () => {
  const relayDir = path.resolve(__dirname, '..')
  const files = findCodeIntelSourceFiles(relayDir)

  it('scans and finds codeintel production source files', () => {
    expect(files.length).toBeGreaterThan(10)
  })

  it('child_process is only imported in allowed runner/probe files', () => {
    const offendingFiles: string[] = []
    for (const file of files) {
      const baseName = path.basename(file)
      // spawn-recorder is test-only helper, but located in codeintel dir
      if (baseName === 'spawn-recorder.ts' || baseName === 'fake-codeintel-cli.ts') {
        continue
      }
      const content = fs.readFileSync(file, 'utf8')
      if (content.includes('child_process')) {
        if (!ALLOWED_CHILD_PROCESS_FILES.has(baseName)) {
          offendingFiles.push(baseName)
        }
      }
    }
    expect(offendingFiles).toEqual([])
  })

  it('no non-test codeintel file uses shell: true or exec(', () => {
    const shellViolations: string[] = []
    const execViolations: string[] = []

    for (const file of files) {
      const baseName = path.basename(file)
      if (baseName === 'spawn-recorder.ts' || baseName === 'fake-codeintel-cli.ts') {
        continue
      }
      const content = fs.readFileSync(file, 'utf8')
      if (/shell\s*:\s*true/.test(content)) {
        shellViolations.push(baseName)
      }
      if (
        /(?:child_process|cp)\.exec\s*\(/.test(content) ||
        /(?:child_process|cp)\.execSync\s*\(/.test(content) ||
        /import\s+{[^}]*\bexec\b[^}]*}\s+from\s+['"](?:node:)?child_process['"]/.test(content) ||
        /import\s+{[^}]*\bexecSync\b[^}]*}\s+from\s+['"](?:node:)?child_process['"]/.test(content)
      ) {
        execViolations.push(baseName)
      }
    }

    expect(shellViolations).toEqual([])
    expect(execViolations).toEqual([])
  })

  it('no production code imports spawn-recorder', () => {
    const illegalImports: string[] = []
    for (const file of files) {
      const baseName = path.basename(file)
      if (baseName === 'spawn-recorder.ts' || baseName === 'fake-codeintel-cli.ts') {
        continue
      }
      const content = fs.readFileSync(file, 'utf8')
      if (content.includes('spawn-recorder')) {
        illegalImports.push(baseName)
      }
    }
    expect(illegalImports).toEqual([])
  })
})
