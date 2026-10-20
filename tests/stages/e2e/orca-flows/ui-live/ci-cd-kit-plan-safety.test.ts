/**
 *   node --test tests/stages/e2e/orca-flows/ui-live/ci-cd-kit-plan-safety.test.ts
 */
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { unsafePlanText } from './ci-cd-kit-plan-safety.ts'

const REPO = '/Users/x/vnp-qa-platform'

test('a CI example that mentions curl and docker as content is fine', () => {
  assert.deepEqual(
    unsafePlanText(
      'Write ci/github-actions.yml: a step runs curl -H X-Access-Key and polls the result; image: docker.io/python:3',
      [REPO]
    ),
    []
  )
})

test('commands that start a stack, push or leave the repo are refused', () => {
  const hits = unsafePlanText(
    'Verify: docker compose up -d && pytest; then git push origin HEAD; cat ~/.ssh/id_rsa',
    [REPO]
  )
  assert.equal(hits.length, 3)
  assert.match(hits.join(' '), /docker/)
  assert.match(hits.join(' '), /git push/)
  assert.match(hits.join(' '), /path outside the repo/)
})

test('a negated instruction is not a request', () => {
  assert.deepEqual(
    unsafePlanText(
      'Do not run docker compose up and never git push. Không khởi động docker compose up.',
      [REPO]
    ),
    []
  )
})

test('negation in an earlier sentence does not excuse a later command', () => {
  assert.equal(
    unsafePlanText('Do not touch the network. Then run docker compose up.', [REPO]).length,
    1
  )
})

test('the repo and worktree paths are allowed', () => {
  assert.deepEqual(
    unsafePlanText(`Edit ${REPO}/docs/guide.md and run pytest in ${REPO}-task-abc/tests`, [REPO]),
    []
  )
})
