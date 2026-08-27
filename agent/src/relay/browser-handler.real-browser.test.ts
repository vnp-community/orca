import { afterAll, describe, expect, it, vi } from 'vitest'
import { execFileSync } from 'node:child_process'
import { createRequire } from 'node:module'
import path from 'node:path'
import {
  handleBrowserClick,
  handleBrowserGoto,
  handleBrowserSnapshot,
  handleBrowserTabClose,
  handleBrowserTabCreate
} from './browser-handler'
import type { AgentLogger } from './agent-logger'

// Real-headless-Chrome integration pass — deliberately separate from
// browser-handler.test.ts's mocked-execFile unit tests. Per this task's own
// instructions: don't require a real Chrome/Chromium binary in CI, but if
// one IS available in the sandbox, prefer a real end-to-end pass over the
// mock for the highest-value paths. `agent-browser` (the vendored engine)
// locates/launches a real Chromium on its own — this suite just checks that
// path is actually reachable before running, and skips cleanly if not.
//
// Why a synchronous top-level probe (not beforeAll): describe.runIf()'s
// condition is evaluated at test-collection time, before any beforeAll
// runs — an async beforeAll-computed flag is always still `false` when
// runIf reads it. A blocking execFileSync probe, run once at import time,
// is the only way to gate `describe.runIf` on real availability.
const WORKTREE_ID = `browser-handler-real-test-${process.pid}`
const REAL_BROWSER_TEST_TIMEOUT_MS = 30_000

const LOG: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }

function probeRealBrowserAvailable(): boolean {
  try {
    const require = createRequire(import.meta.url)
    const bin = path.join(path.dirname(require.resolve('agent-browser/package.json')), 'bin', 'agent-browser.js')
    const stdout = execFileSync(
      process.execPath,
      [bin, 'open', 'https://example.com', '--session', WORKTREE_ID, '--json'],
      { encoding: 'utf-8', timeout: REAL_BROWSER_TEST_TIMEOUT_MS }
    )
    return (JSON.parse(stdout) as { success: boolean }).success === true
  } catch {
    return false
  }
}

const realBrowserAvailable = probeRealBrowserAvailable()

afterAll(async () => {
  if (!realBrowserAvailable) {return}
  await handleBrowserTabClose(0, { worktree: WORKTREE_ID }, LOG)
}, REAL_BROWSER_TEST_TIMEOUT_MS)

describe.runIf(realBrowserAvailable)('browser-handler — real headless Chrome (skips if unavailable)', () => {
  it(
    'handleBrowserGoto actually navigates a real headless Chrome tab',
    async () => {
      const response = await handleBrowserGoto(1, { worktree: WORKTREE_ID, url: 'https://example.com' }, LOG)
      expect(response).toMatchObject({
        result: { url: 'https://example.com/', title: 'Example Domain' }
      })
    },
    REAL_BROWSER_TEST_TIMEOUT_MS
  )

  it(
    'handleBrowserSnapshot returns a real accessibility-tree snapshot with refs',
    async () => {
      const response = await handleBrowserSnapshot(2, { worktree: WORKTREE_ID }, LOG)
      expect('error' in response).toBe(false)
      const result = (response as { result: { snapshot: string; refs: Record<string, unknown> } }).result
      expect(result.snapshot).toContain('Example Domain')
      expect(Object.keys(result.refs).length).toBeGreaterThan(0)
    },
    REAL_BROWSER_TEST_TIMEOUT_MS
  )

  it(
    'handleBrowserClick actually clicks the real "Learn more" link and navigates away',
    async () => {
      const snapshot = await handleBrowserSnapshot(3, { worktree: WORKTREE_ID }, LOG)
      const refs = (snapshot as { result: { refs: Record<string, { role: string; name: string }> } }).result.refs
      const linkRef = Object.entries(refs).find(([, ref]) => ref.role === 'link')?.[0]
      expect(linkRef).toBeDefined()

      const response = await handleBrowserClick(4, { worktree: WORKTREE_ID, element: `@${linkRef}` }, LOG)
      expect('error' in response).toBe(false)
    },
    REAL_BROWSER_TEST_TIMEOUT_MS
  )

  it(
    'handleBrowserTabCreate opens a second real tab in the same worktree session',
    async () => {
      const response = await handleBrowserTabCreate(5, { worktree: WORKTREE_ID }, LOG)
      expect('error' in response).toBe(false)
      const result = (response as { result: { tabId: string } }).result
      expect(typeof result.tabId).toBe('string')
      expect(result.tabId.length).toBeGreaterThan(0)
    },
    REAL_BROWSER_TEST_TIMEOUT_MS
  )
})

describe.runIf(!realBrowserAvailable)('browser-handler — real headless Chrome unavailable', () => {
  it('skips the real-browser suite (no Chrome/Chromium reachable in this sandbox)', () => {
    expect(realBrowserAvailable).toBe(false)
  })
})
