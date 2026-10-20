/**
 *   node --test tests/stages/e2e/orca-flows/ui-live/ci-cd-kit-report.test.ts
 */
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { minutes, renderReportVi, type ReportFacts } from './ci-cd-kit-report.ts'
import { staticCheckFile } from './ci-cd-kit-git-evidence.ts'
import { mkdtempSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'

const FACTS: ReportFacts = {
  requestNumber: 1800,
  requestId: 'abc',
  finalStatus: 'completed',
  baseBranch: 'claude/x',
  baseTip: 'fd4f6ceb00000000',
  aiProposed: { type: 'Change request', size: 'M' },
  questions: [
    { prompt: 'Tiêu chí nghiệm thu?', answer: 'Theo BL-E07: pipeline mẫu fail khi dưới ngưỡng' }
  ],
  language: { isVietnamese: true, accentedPercent: 28.4, sample: 'Phương án A' },
  planWrittenWithoutGenerate: true,
  planWaitMs: 125_000,
  documentTabs: ['Proposal', 'Design', 'Tasks'],
  secretsOnScreen: [],
  runAllUsed: true,
  tasks: [{ type: 'feature', title: 'Thêm ví dụ GitHub Actions', status: 'done' }],
  worktrees: [
    {
      path: '/r-task-1',
      branch: 'task/1',
      head: 'aaaaaaaaaaaaaaaa',
      descendsFromBase: true,
      mergeBase: 'fd4f6ceb00000000',
      commitsAhead: 2,
      diffStat: ' a.yml | 3 +++',
      uncommitted: 0
    }
  ],
  staticChecks: ['ok yaml a.yml'],
  costLines: ['tổng 3.20 USD'],
  findings: [],
  timeline: ['request created: +0p05s (tại 0p05s)'],
  screenshotsDir: '/tmp/shots'
}

test('minutes formats like 2p05s', () => {
  assert.equal(minutes(125_000), '2p05s')
})

test('the report says what each of the five features did', () => {
  const text = renderReportVi(FACTS)
  assert.match(text, /Request #1800/)
  assert.match(text, /đúng tiếng Việt/)
  assert.match(text, /Plan xuất hiện mà không bấm Generate, chờ 2p05s/)
  assert.match(text, /cả 1 worktree đều là hậu duệ/)
  assert.match(text, /không thấy mẫu bí mật nào/)
})

test('a worktree that does not descend from the tip, or a secret, is called out', () => {
  const text = renderReportVi({
    ...FACTS,
    secretsOnScreen: ['credential shape in Design'],
    worktrees: [{ ...FACTS.worktrees[0], descendsFromBase: false }]
  })
  assert.match(text, /CÓ worktree không xuất phát từ nhánh đã chọn/)
  assert.match(text, /PHÁT HIỆN MẪU BÍ MẬT/)
})

test('written files are parsed, never run', () => {
  const dir = mkdtempSync(join(tmpdir(), 'cicd-static-'))
  writeFileSync(join(dir, 'ok.py'), 'x = 1\n')
  writeFileSync(join(dir, 'bad.py'), 'def (:\n')
  writeFileSync(join(dir, 'run.sh'), '#!/bin/sh\necho hi\n')
  writeFileSync(join(dir, 'bad.json'), '{')
  assert.match(staticCheckFile(join(dir, 'ok.py'), false), /^ok python/)
  assert.match(staticCheckFile(join(dir, 'bad.py'), false), /LỖI CÚ PHÁP/)
  assert.match(staticCheckFile(join(dir, 'run.sh'), false), /^ok bash -n/)
  assert.match(staticCheckFile(join(dir, 'bad.json'), false), /LỖI JSON/)
  assert.equal(staticCheckFile(join(dir, 'x.md'), false), '')
})
