/**
 * Pure checks of the rules that guard the user's repo during a real-AI run; no stack, no git, no browser.
 *   node --test tests/stages/e2e/orca-flows/ui-live/live-main-checkout-rules.test.ts
 */
import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  checkoutViolations,
  growthSince,
  restoredProblems,
  type CheckoutState
} from './live-main-checkout-rules.ts'

const REPO = '/work/agentic/deepseek-harness'

function fp(head: string, files: Record<string, string> = {}): CheckoutState['main'] {
  return {
    path: REPO,
    head,
    refs: '',
    worktrees: '',
    config: 'cfg',
    refList: [],
    worktreeList: [],
    files
  }
}

function base(): CheckoutState {
  return {
    repo: REPO,
    parent: '/work/agentic',
    main: fp('aaa', { 'docs/NOTES.md': 'h:1' }),
    refs: {
      'refs/heads/master': 'aaa',
      'refs/heads/test-01': 'aaa',
      'refs/remotes/origin/master': 'aaa'
    },
    worktrees: { '/work/agentic/deepseek-harness-test-01': fp('aaa') },
    neighbours: ['codex', 'deepseek-harness', 'deepseek-harness-test-01']
  }
}

/** The state after Orca ran a task: a task branch and its worktree next to the repo. */
function grown(): CheckoutState {
  const s = base()
  s.refs['refs/heads/task/abc'] = 'bbb'
  s.worktrees['/work/agentic/deepseek-harness-task-abc'] = fp('bbb', { 'scripts/new.mjs': 'h:2' })
  s.neighbours = [...s.neighbours, 'deepseek-harness-task-abc'].sort()
  return s
}

test('an unchanged repo has no violations', () => {
  assert.deepEqual(checkoutViolations(base(), base()), [])
})

test('a task branch and a `<repo>-*` worktree next to the repo are the allowed growth', () => {
  assert.deepEqual(checkoutViolations(base(), grown()), [])
  assert.deepEqual(growthSince(base(), grown()), {
    branches: ['refs/heads/task/abc'],
    worktrees: ['/work/agentic/deepseek-harness-task-abc']
  })
})

test('after cleanup, leftover growth is reported; a restored repo is clean', () => {
  const problems = restoredProblems(base(), grown())
  assert.ok(problems.some((p) => p.includes('branch refs/heads/task/abc was not removed')))
  assert.ok(problems.some((p) => p.includes('deepseek-harness-task-abc was not removed')))
  assert.deepEqual(restoredProblems(base(), base()), [])
})

test('every way of touching the main checkout is a violation', () => {
  const moved = base()
  moved.main = fp('ccc', { 'docs/NOTES.md': 'h:1' })
  assert.match(checkoutViolations(base(), moved).join('|'), /moved HEAD/)

  const edited = base()
  edited.main = fp('aaa', { 'docs/NOTES.md': 'h:9' })
  assert.match(checkoutViolations(base(), edited).join('|'), /1 file\(s\) of the main checkout/)

  const created = base()
  created.main = fp('aaa', { 'docs/NOTES.md': 'h:1', 'stray.txt': 'h:3' })
  assert.match(checkoutViolations(base(), created).join('|'), /1 file\(s\) of the main checkout/)

  const config = base()
  config.main = { ...fp('aaa', { 'docs/NOTES.md': 'h:1' }), config: 'other' }
  assert.match(checkoutViolations(base(), config).join('|'), /git config changed/)
})

test('a push, a tag, a moved or deleted ref are violations; a new local branch is not', () => {
  const pushed = base()
  pushed.refs['refs/remotes/origin/task/abc'] = 'bbb'
  assert.match(checkoutViolations(base(), pushed).join('|'), /new ref outside refs\/heads\//)

  const tagged = base()
  tagged.refs['refs/tags/v1'] = 'aaa'
  assert.match(checkoutViolations(base(), tagged).join('|'), /new ref outside refs\/heads\//)

  const reset = base()
  reset.refs['refs/heads/master'] = 'zzz'
  assert.match(checkoutViolations(base(), reset).join('|'), /ref refs\/heads\/master moved/)

  const deleted = base()
  delete deleted.refs['refs/heads/test-01']
  assert.match(
    checkoutViolations(base(), deleted).join('|'),
    /ref refs\/heads\/test-01 was deleted/
  )
})

test('the user worktree must keep its HEAD and files; new worktrees must sit next to the repo as `<repo>-*`', () => {
  const touched = base()
  touched.worktrees['/work/agentic/deepseek-harness-test-01'] = fp('aaa', { 'x.ts': 'h:1' })
  assert.match(checkoutViolations(base(), touched).join('|'), /worktree .* of the user changed/)

  const gone = base()
  delete gone.worktrees['/work/agentic/deepseek-harness-test-01']
  assert.match(checkoutViolations(base(), gone).join('|'), /of the user disappeared/)

  const elsewhere = base()
  elsewhere.worktrees['/tmp/somewhere'] = fp('bbb')
  assert.match(checkoutViolations(base(), elsewhere).join('|'), /outside the allowed place/)

  const wrongName = base()
  wrongName.worktrees['/work/agentic/other-task'] = fp('bbb')
  wrongName.neighbours = [...wrongName.neighbours, 'other-task'].sort()
  assert.match(checkoutViolations(base(), wrongName).join('|'), /outside the allowed place/)
})

test('a stray directory next to the repo, or a missing one, is a violation', () => {
  const stray = base()
  stray.neighbours = [...stray.neighbours, 'leftover'].sort()
  assert.match(checkoutViolations(base(), stray).join('|'), /entries next to the repo changed/)

  const lost = base()
  lost.neighbours = lost.neighbours.filter((n) => n !== 'codex')
  assert.match(checkoutViolations(base(), lost).join('|'), /entries next to the repo changed/)
})
