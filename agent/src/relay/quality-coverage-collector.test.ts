import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import {
  collectGoCoverage,
  getCoverageReport,
  saveCoverageReport
} from './quality-coverage-collector'
import { getCatalog } from './quality-profile-catalog'

describe('quality-coverage-collector', () => {
  let tmpDir: string

  beforeEach(() => {
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'coverage-collector-test-'))
  })

  afterEach(() => {
    try {
      fs.rmSync(tmpDir, { recursive: true, force: true })
    } catch {}
  })

  it('profile coverage-go is registered and valid in catalog', () => {
    const catalog = getCatalog()
    const p = catalog.profiles.find(x => x.id === 'coverage-go')
    expect(p).toBeDefined()
    expect(p?.argv).toContain('-covermode=set')
    expect(p?.heavy).toBe(true)
  })

  it('collects coverage output and saves to store', async () => {
    const profilePath = path.join(tmpDir, 'mod1.out')
    const sampleProfile = `mode: set
github.com/example/mod1/pkg/calc.go:5.14,7.2 1 1
github.com/example/mod1/pkg/calc.go:9.14,11.2 1 0
`
    fs.writeFileSync(profilePath, sampleProfile)

    const report = await collectGoCoverage({
      workspaceRoot: tmpDir,
      runId: 'qr_test_1',
      runDir: tmpDir,
      baseCommit: null,
      headCommit: 'abcd123',
      dirty: false,
      stepOutputs: [
        { moduleName: 'mod1', profilePath, success: true }
      ],
      scope: 'worktree'
    })

    expect(report.source).toBe('measured')
    expect(report.totals.stmts).toBe(2)
    expect(report.totals.covered).toBe(1)
    expect(report.totals.pct).toBe(0.5)

    const retrieved = getCoverageReport(tmpDir, 'qr_test_1')
    expect(retrieved).toEqual(report)
  })
})
