/** Writes the Vietnamese report of the ci-cd-kit run next to the screenshots (and to ORCA_E2E_REPORT_DIR when set) and prints it. */
import { copyFileSync, mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import type { TestInfo } from '@playwright/test'
import { renderReportVi, type ReportFacts, type WorktreeFact } from './ci-cd-kit-report'

export function writeReportFile(
  testInfo: TestInfo,
  facts: ReportFacts,
  worktrees: WorktreeFact[]
): void {
  const dir = process.env.ORCA_E2E_REPORT_DIR || testInfo.outputDir
  mkdirSync(dir, { recursive: true })
  const report = renderReportVi(facts)
  writeFileSync(join(dir, 'ci-cd-kit-report.vi.md'), report)
  if (dir !== testInfo.outputDir) {
    copyFileSync(
      join(dir, 'ci-cd-kit-report.vi.md'),
      join(testInfo.outputDir, 'ci-cd-kit-report.vi.md')
    )
  }
  console.log(`[ci-cd-kit] report written to ${join(dir, 'ci-cd-kit-report.vi.md')}\n${report}`)
  testInfo.annotations.push({
    type: 'new-worktrees',
    description: worktrees.map((w) => `${w.path} [${w.branch}]`).join(' | ')
  })
}
