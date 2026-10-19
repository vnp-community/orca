// node --test tests/stages/e2e/orca-flows/ui-live/live-agent-session-judge.test.ts   (no stack, no Claude)
import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  judgeBackendOutput,
  judgeActivityPanel,
  judgeDevServer,
  judgeDevServerFacts,
  judgeHistoryReplay,
  judgeRunningIndicator,
  judgeRunTrace,
  judgeStatusFollows,
  judgeUiShowsOutput,
  judgeWorktreeTerminal,
  judgeWorktreeView,
  linesAddedByRun
} from './live-agent-session-judge.ts'

const times = { startedAt: 0, leftRunningAt: 60_000 }
const online = (l: string): boolean => /^(connected|macOS)$/.test(l)
const codes = (v: { code: string }[]): string[] => v.map((x) => x.code)

test('no output frame at all is a backend gap', () => {
  assert.deepEqual(codes(judgeBackendOutput(times, [])), ['AGENT-OUTPUT-NOT-BROADCAST'])
})

test('a single frame at the end of a long run is not a stream', () => {
  assert.deepEqual(codes(judgeBackendOutput(times, [59_000])), ['AGENT-OUTPUT-NOT-STREAMED'])
})

test('a frame well before the end counts as live; a short run cannot tell', () => {
  assert.deepEqual(judgeBackendOutput(times, [10_000, 59_000]), [])
  assert.deepEqual(judgeBackendOutput({ startedAt: 0, leftRunningAt: 8_000 }, [7_500]), [])
})

test('UI must show what the backend broadcast, and live when the backend streamed', () => {
  assert.deepEqual(
    codes(judgeUiShowsOutput(times, 'added negate to calc', { seenAt: null }, true)),
    ['UI-AGENT-OUTPUT-NOT-SHOWN']
  )
  assert.deepEqual(codes(judgeUiShowsOutput(times, 'x'.repeat(5), { seenAt: 61_000 }, true)), [
    'UI-AGENT-OUTPUT-NOT-LIVE'
  ])
  assert.deepEqual(judgeUiShowsOutput(times, 'x'.repeat(5), { seenAt: 61_000 }, false), [])
  assert.deepEqual(judgeUiShowsOutput(times, '', { seenAt: null }, true), [])
})

test('history replay is judged only when there was something to replay', () => {
  assert.deepEqual(judgeHistoryReplay('', false), [])
  assert.deepEqual(codes(judgeHistoryReplay('phrase', false)), ['UI-NO-SESSION-HISTORY'])
  assert.deepEqual(judgeHistoryReplay('phrase', true), [])
})

test('the run trace needs the changed file and a test run in text the run added', () => {
  const before = 'Plan\nAdd negate to src/calc.py and run python3 -m unittest'
  assert.deepEqual(codes(judgeRunTrace(linesAddedByRun(before, before))), ['UI-NO-RUN-TRACE'])
  const after = `${before}\nStatus: done\nChanged src/calc.py and tests/test_calc.py\nRan 3 tests OK`
  assert.deepEqual(judgeRunTrace(linesAddedByRun(before, after)), [])
})

test('the worktree view needs the button and the changed file', () => {
  assert.deepEqual(codes(judgeWorktreeView(false, null)), ['UI-NO-WORKTREE-VIEW'])
  assert.deepEqual(codes(judgeWorktreeView(true, 'nothing here')), ['UI-NO-WORKTREE-VIEW'])
  assert.deepEqual(judgeWorktreeView(true, 'M src/calc.py'), [])
})

test('dev server: offline, never busy, identical busy and idle states, not idle afterwards', () => {
  const idle = { ui: 'connected', busy: 'false', api: '{"busy":false}' }
  const busy = { ui: 'connected', busy: 'true', api: '{"busy":true}' }
  assert.deepEqual(judgeDevServer(online, [busy, busy], idle), [])
  const flat = { ui: 'connected', busy: '', api: '{}' }
  assert.deepEqual(codes(judgeDevServer(online, [flat], flat)), [
    'DEVSERVER-NEVER-SHOWED-BUSY',
    'DEVSERVER-NO-BUSY-STATE',
    'DEVSERVER-NOT-IDLE-AFTER-RUN'
  ])
  assert.deepEqual(codes(judgeDevServer(online, [{ ...busy, ui: 'disconnected' }], idle)), [
    'DEVSERVER-OFFLINE-IN-UI'
  ])
  assert.deepEqual(codes(judgeDevServer(online, [busy], { ...busy })), [
    'DEVSERVER-NO-BUSY-STATE',
    'DEVSERVER-NOT-IDLE-AFTER-RUN'
  ])
})

test('dev server facts: platform and health', () => {
  assert.deepEqual(judgeDevServerFacts('macOS · arm64', '{"healthStatus":"healthy"}'), [])
  assert.deepEqual(codes(judgeDevServerFacts('', '{"healthStatus":"pending"}')), [
    'DEVSERVER-NO-PLATFORM',
    'DEVSERVER-NOT-HEALTHY'
  ])
})

test('activity panel: present, running with Live, finished with lines', () => {
  const run = { state: 'running', live: true, lines: 3 }
  assert.deepEqual(judgeActivityPanel([run], { state: 'finished', live: false, lines: 9 }), [])
  assert.deepEqual(codes(judgeActivityPanel([{ state: 'absent', live: false, lines: 0 }], null)), [
    'UI-NO-AGENT-ACTIVITY-PANEL'
  ])
  assert.deepEqual(codes(judgeActivityPanel([{ state: 'idle', live: false, lines: 0 }], null)), [
    'UI-ACTIVITY-PANEL-STATE'
  ])
  assert.deepEqual(codes(judgeActivityPanel([run], { state: 'idle', live: false, lines: 0 })), [
    'UI-ACTIVITY-PANEL-STATE'
  ])
})

test('the sheet status must reach the service status', () => {
  assert.deepEqual(
    judgeStatusFollows(
      ['in_progress', 'review', 'done'],
      ['Status: in_progress', 'Status: review']
    ),
    []
  )
  assert.deepEqual(codes(judgeStatusFollows(['in_progress', 'review'], ['Status: in_progress'])), [
    'UI-TASK-STATUS-STALE'
  ])
  assert.deepEqual(judgeStatusFollows(['in_progress'], ['Status: in_progress']), [])
})

test('the running indicator is judged only when the task really ran', () => {
  assert.deepEqual(judgeRunningIndicator(false, false), [])
  assert.deepEqual(judgeRunningIndicator(true, true), [])
  assert.deepEqual(codes(judgeRunningIndicator(true, false)), ['UI-NO-RUNNING-INDICATOR'])
})

test('the worktree terminal must not report a pty spawn failure', () => {
  assert.deepEqual(judgeWorktreeTerminal('Terminal 1'), [])
  assert.deepEqual(
    codes(judgeWorktreeTerminal('rpc error ... INFRA_AGENT_SPAWN_PTY_FAILED: failed')),
    ['WORKTREE-TERMINAL-FAILED']
  )
})
