/**
 * Tests for scrollback-save tracing (TASK-BE-003.3/003.4).
 *
 * Covers `migrateWorkspaceSessionTerminalScrollbackSnapshots()` in
 * `src/main/terminal-scrollback-snapshots.ts` — instrumented with
 * `Tracers.terminalDestroy` (flow `terminal:destroy`), guarded so no span is
 * created when there are no pending scrollback buffers to write (this
 * function runs on every `Store.setWorkspaceSession()`/`load()` call).
 *
 * @module main/__tests__/terminal-scrollback-snapshots.test
 */

import { afterEach, describe, expect, it, vi } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

// Why: terminal-scrollback-snapshots.ts imports `app` from 'electron' for its
// legacy-root fallback (unused here since every test passes an explicit
// `snapshotRoot`), but the module import itself still needs `electron` to
// resolve under Vitest's plain Node environment — mirrors persistence.test.ts.
vi.mock('electron', () => ({
  app: {
    getPath: () => '/unused-legacy-snapshot-root'
  }
}))

import { registerTraceSink, type TraceEvent } from '../../shared/trace'
import { migrateWorkspaceSessionTerminalScrollbackSnapshots } from '../terminal-scrollback-snapshots'
import type { TerminalLayoutSnapshot, WorkspaceSessionState } from '../../shared/types'

function makeSession(
  terminalLayoutsByTabId: Record<string, TerminalLayoutSnapshot>
): WorkspaceSessionState {
  return {
    activeRepoId: null,
    activeWorktreeId: null,
    activeTabId: null,
    tabsByWorktree: {},
    terminalLayoutsByTabId
  }
}

function makeLayout(buffersByLeafId?: Record<string, string>): TerminalLayoutSnapshot {
  return {
    root: null,
    activeLeafId: null,
    expandedLeafId: null,
    ...(buffersByLeafId ? { buffersByLeafId } : {})
  }
}

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

describe('migrateWorkspaceSessionTerminalScrollbackSnapshots tracing (CR-TRACE-003)', () => {
  let tmpDirs: string[] = []

  afterEach(() => {
    for (const dir of tmpDirs) {
      rmSync(dir, { recursive: true, force: true })
    }
    tmpDirs = []
  })

  function makeSnapshotRoot(): string {
    const dir = mkdtempSync(join(tmpdir(), 'orca-scrollback-tracing-'))
    tmpDirs.push(dir)
    return dir
  }

  it('skips span entirely when no buffersByLeafId is pending', () => {
    const session = makeSession({
      'tab-1': makeLayout(),
      'tab-2': makeLayout({})
    })
    const { events, stop } = captureTraceEvents()

    const result = migrateWorkspaceSessionTerminalScrollbackSnapshots(session)
    stop()

    expect(result).toEqual({ session, changed: false })
    expect(events).toEqual([])
  })

  it('skips span entirely when terminalLayoutsByTabId is absent', () => {
    const session = makeSession({})
    const { events, stop } = captureTraceEvents()

    const result = migrateWorkspaceSessionTerminalScrollbackSnapshots(session)
    stop()

    expect(result).toEqual({ session, changed: false })
    expect(events).toEqual([])
  })

  it('emits a terminalDestroy span with step write-snapshot-sync per leaf, ok() with aggregated bytesWritten', () => {
    const snapshotRoot = makeSnapshotRoot()
    const session = makeSession({
      'tab-1': makeLayout({ 'leaf-1': 'hello', 'leaf-2': 'world!' })
    })
    const { events, stop } = captureTraceEvents()

    const result = migrateWorkspaceSessionTerminalScrollbackSnapshots(session, { snapshotRoot })
    stop()

    expect(result.changed).toBe(true)
    const destroyEvents = events.filter((e) => e.flow === 'terminal:destroy')
    expect(destroyEvents[0]?.level).toBe('start')
    const steps = destroyEvents.filter((e) => e.level === 'step')
    expect(steps).toHaveLength(2)
    expect(steps.map((e) => e.fields.leafId).sort()).toEqual(['leaf-1', 'leaf-2'])
    const ok = destroyEvents.find((e) => e.level === 'ok')
    expect(ok?.fields).toMatchObject({ bytesWritten: 'hello'.length + 'world!'.length })
  })

  it('calls span.fail() when the write loop throws unexpectedly, then rethrows', () => {
    const snapshotRoot = makeSnapshotRoot()
    // Why: writeTerminalScrollbackSnapshotSync itself never throws (it catches
    // fs errors internally and returns null) — simulate an unexpected failure
    // elsewhere in the per-leaf loop via a buffer whose .length getter throws,
    // to exercise the span.fail()/rethrow path around the whole write loop.
    const throwingBuffer = new Proxy(
      {},
      {
        get(_target, prop) {
          if (prop === 'length') {
            throw new Error('boom')
          }
          return undefined
        }
      }
    ) as unknown as string
    const session = makeSession({
      'tab-1': makeLayout({ 'leaf-1': throwingBuffer })
    })
    const { events, stop } = captureTraceEvents()

    expect(() =>
      migrateWorkspaceSessionTerminalScrollbackSnapshots(session, { snapshotRoot })
    ).toThrow('boom')
    stop()

    const failEvent = events.find((e) => e.flow === 'terminal:destroy' && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.err).toContain('boom')
  })
})
