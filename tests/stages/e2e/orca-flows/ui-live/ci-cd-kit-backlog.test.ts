/**
 * The BL-E07 text and the Clarification answers built from it (no stack needed):
 *   node --test tests/stages/e2e/orca-flows/ui-live/ci-cd-kit-backlog.test.ts
 */
import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import { test } from 'node:test'
import {
  BACKLOG_FILE,
  CICDKIT_REPO,
  EXECUTION_LIMITS,
  parseBacklog,
  pickOption,
  POINTER_ANSWER,
  readBacklog,
  requestBody,
  textAnswer
} from './ci-cd-kit-backlog.ts'

const SAMPLE = `---
id: BL-E07
title: "Bộ tích hợp CI/CD: API key AK/SK"
priority: P1
milestone: M2
size: S
---

# BL-E07 — x

## Bối cảnh — vì sao cần

CI gọi TMS: chạy plan và nhận pass/fail.

## Hiện trạng (bằng chứng)

- Có \`GET result\`, execute có Idempotency-Key.

## Phạm vi công việc

- [ ] Ví dụ GitHub Actions/GitLab CI/Jenkins gọi execute và poll result.
- [ ] Sửa fixture test; tài liệu quản trị key.

## Tiêu chí nghiệm thu

- Pipeline mẫu fail khi pass-rate < ngưỡng.
- Secret chỉ hiển thị cho chủ khoá.

## Phụ thuộc

- Cần xong trước: BL-B07

## Nguồn (đường dẫn)

- \`backend/specs/crs/v4/tasks/TASKS-P3a.md\`
`

test('the front matter and the sections are read', () => {
  const item = parseBacklog(SAMPLE)
  assert.equal(item.id, 'BL-E07')
  assert.equal(item.priority, 'P1')
  assert.equal(item.size, 'S')
  assert.ok(Object.keys(item.sections).includes('Phạm vi công việc'))
})

test('the Request body says where it comes from, keeps goal, scope and criteria, and the limits', () => {
  const body = requestBody(parseBacklog(SAMPLE))
  assert.match(body, /^Nguồn: backlog BL-E07/)
  assert.match(body, /## Mục tiêu\nCI gọi TMS/)
  assert.match(body, /## Phạm vi công việc\n- \[ \] Ví dụ GitHub Actions/)
  assert.match(body, /## Tiêu chí nghiệm thu\n- Pipeline mẫu fail/)
  for (const limit of EXECUTION_LIMITS) {
    assert.ok(body.includes(limit))
  }
  assert.ok(!body.includes('Cần xong trước'), 'the dependency section is not part of the body')
})

test('an acceptance question is answered with the acceptance criteria', () => {
  const answer = textAnswer(parseBacklog(SAMPLE), 'What are the acceptance criteria?')
  assert.match(answer, /^Theo BL-E07:/)
  assert.match(answer, /Pipeline mẫu fail khi pass-rate/)
  assert.match(
    textAnswer(parseBacklog(SAMPLE), 'Tiêu chí nghiệm thu là gì?'),
    /Secret chỉ hiển thị/
  )
})

test('a question about the value is answered from the goal, not with the pointer', () => {
  assert.match(textAnswer(parseBacklog(SAMPLE), 'Giá trị mang lại'), /CI gọi TMS/)
})

test('a priority question gets the facts, an unrelated one the pointer', () => {
  const item = parseBacklog(SAMPLE)
  assert.match(textAnswer(item, 'What is the priority and deadline?'), /ưu tiên P1, mốc M2, cỡ S/)
  assert.equal(textAnswer(item, 'Which colour is the logo?'), POINTER_ANSWER)
})

test('a question that names a topic of the text is answered with its lines', () => {
  const answer = textAnswer(
    parseBacklog(SAMPLE),
    'Should the pipeline example poll result of execute?'
  )
  assert.match(answer, /poll result/)
})

test('a choice is the option closest to the text, the first when none is', () => {
  const item = parseBacklog(SAMPLE)
  assert.equal(pickOption(item, ['Jenkins only', 'GitHub Actions and GitLab CI']), 1)
  assert.equal(pickOption(item, ['alpha', 'beta']), 0)
  assert.equal(pickOption(item, []), -1)
})

test('the real item of the user repo reads (skipped when the repo is not on this machine)', (t) => {
  if (!existsSync(`${CICDKIT_REPO}/${BACKLOG_FILE}`)) {
    t.skip('vnp-qa-platform is not on this machine')
    return
  }
  const item = readBacklog()
  assert.equal(item.id, 'BL-E07')
  const body = requestBody(item)
  assert.ok(body.length < 6_000, `the body stays short (${body.length} characters)`)
  assert.match(body, /Tiêu chí nghiệm thu/)
})
