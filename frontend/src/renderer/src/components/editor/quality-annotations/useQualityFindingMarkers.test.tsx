// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook } from '@testing-library/react'
import type { QualityCall } from '../../../store/slices/code-intel-quality-slice-context'

const call = vi.fn<QualityCall>()

vi.mock('@/store', async () => {
  const { createCodeIntelQualityTestStore } =
    await import('../../../test-support/code-intel-quality-test-store')
  return { useAppStore: createCodeIntelQualityTestStore((...args) => call(...args)) }
})

import { useAppStore } from '@/store'
import { ENABLED_QUALITY_SUPPORT } from '../../../test-support/code-intel-quality-test-store'
import { QUALITY_MARKER_OWNER, useQualityFindingMarkers } from './useQualityFindingMarkers'
import type { QualityMonacoApi } from './useQualityFindingMarkers'

const ok = (result: unknown) => Promise.resolve({ ok: true as const, result })
const RUN = {
  id: 'run-1',
  headCommit: 'abc',
  status: 'succeeded',
  source: 'local',
  finishedAt: '2026-10-07T00:00:00Z'
}
const FINDING = {
  fingerprint: 'fp-1',
  ruleId: 'r1',
  severity: 'error',
  category: 'lint',
  file: 'src/a.ts',
  line: 2,
  endLine: 2,
  column: 1,
  endColumn: 4,
  message: 'boom',
  tool: 'oxlint',
  toolVersion: '1',
  stepId: 's',
  inScope: true
}

async function flush(): Promise<void> {
  await act(async () => {
    for (let i = 0; i < 8; i++) {
      await Promise.resolve()
    }
  })
}

function makeEditor() {
  const contentListeners: (() => void)[] = []
  const mouseListeners: ((e: unknown) => void)[] = []
  const collection = { clear: vi.fn() }
  const model = { getLineCount: () => 5, getLineMaxColumn: () => 20 }
  const editor = {
    getModel: () => model,
    onDidChangeModel: vi.fn(() => ({ dispose: vi.fn() })),
    onDidChangeModelContent: vi.fn((cb: () => void) => {
      contentListeners.push(cb)
      return { dispose: vi.fn(() => contentListeners.splice(contentListeners.indexOf(cb), 1)) }
    }),
    onMouseDown: vi.fn((cb: (e: unknown) => void) => {
      mouseListeners.push(cb)
      return { dispose: vi.fn() }
    }),
    updateOptions: vi.fn(),
    createDecorationsCollection: vi.fn(() => collection)
  }
  return { editor, model, collection, contentListeners, mouseListeners }
}

const setModelMarkers = vi.fn()
const monacoApi: QualityMonacoApi = {
  MarkerSeverity: { Error: 8, Warning: 4, Info: 2, Hint: 1 },
  editor: { setModelMarkers, MouseTargetType: { GUTTER_GLYPH_MARGIN: 2 } }
}

function reply(findings: unknown[] = [FINDING]) {
  call.mockImplementation((_w, method) => {
    if (method === 'codeIntel.quality.runs') {
      return ok({ runs: [RUN] })
    }
    return ok({ findings, totalCount: findings.length })
  })
}

type Fake = ReturnType<typeof makeEditor>
function render(
  fake: Fake | null,
  patch: Partial<Parameters<typeof useQualityFindingMarkers>[0]> = {}
) {
  return renderHook(
    (props: Partial<Parameters<typeof useQualityFindingMarkers>[0]>) =>
      useQualityFindingMarkers({
        editor: fake?.editor as never,
        monacoApi,
        worktreeId: 'wt',
        relativePath: 'src/a.ts',
        diffSource: 'unstaged',
        ...props
      }),
    { initialProps: patch }
  )
}

beforeEach(() => {
  call.mockReset()
  setModelMarkers.mockReset()
  useAppStore.setState({
    codeIntelSupportState: ENABLED_QUALITY_SUPPORT,
    codeIntelQualityByWorktree: {},
    gitStatusHeadByWorktree: { wt: 'abc' }
  } as never)
})
afterEach(() => cleanup())

describe('useQualityFindingMarkers', () => {
  it('sets markers under the fixed owner and glyph decorations', async () => {
    reply()
    const fake = makeEditor()
    render(fake)
    await flush()
    expect(setModelMarkers).toHaveBeenCalledTimes(1)
    const [model, owner, markers] = setModelMarkers.mock.calls[0]
    expect(model).toBe(fake.model)
    expect(owner).toBe(QUALITY_MARKER_OWNER)
    expect(owner).toBe('orca-quality')
    expect(markers).toHaveLength(1)
    expect(fake.editor.updateOptions).toHaveBeenCalledWith({ glyphMargin: true })
    expect(fake.editor.createDecorationsCollection).toHaveBeenCalledTimes(1)
    const decorations = (fake.editor.createDecorationsCollection.mock.calls[0] as unknown[])[0] as {
      options: { glyphMarginClassName: string }
    }[]
    expect(decorations[0].options.glyphMarginClassName).toBe('orca-quality-glyph-error')
  })

  it('clears at once when the content changes and shows a notice', async () => {
    reply()
    const fake = makeEditor()
    const { result } = render(fake)
    await flush()
    expect(result.current.notice).toBeNull()
    act(() => fake.contentListeners.forEach((cb) => cb()))
    expect(fake.collection.clear).toHaveBeenCalled()
    expect(setModelMarkers).toHaveBeenLastCalledWith(fake.model, 'orca-quality', [])
    await flush()
    expect(result.current.notice).toBe('content-changed')
  })

  it('cleans up on unmount without touching other owners', async () => {
    reply()
    const fake = makeEditor()
    const { unmount } = render(fake)
    await flush()
    unmount()
    expect(fake.collection.clear).toHaveBeenCalled()
    for (const c of setModelMarkers.mock.calls) {
      expect(c[1]).toBe('orca-quality')
    }
    expect(setModelMarkers).toHaveBeenLastCalledWith(fake.model, 'orca-quality', [])
    expect(fake.contentListeners).toHaveLength(0)
  })

  it('replaces markers when the findings data changes and clears when annotations are turned off', async () => {
    reply()
    const fake = makeEditor()
    render(fake)
    await flush()
    act(() => {
      useAppStore.setState({
        codeIntelQualityByWorktree: {
          wt: {
            ...useAppStore.getState().codeIntelQualityByWorktree.wt,
            findingsByFile: {
              'src/a.ts': {
                data: [FINDING, { ...FINDING, fingerprint: 'fp-2', line: 3, endLine: 3 }],
                fetchedAt: 1,
                stale: false
              }
            }
          }
        }
      } as never)
    })
    expect(setModelMarkers.mock.calls.filter((c) => (c[2] as unknown[]).length === 2)).toHaveLength(
      1
    )
    act(() => useAppStore.getState().setQualityUi('wt', { annotationsOn: false }))
    expect(setModelMarkers).toHaveBeenLastCalledWith(fake.model, 'orca-quality', [])
  })

  it('glyph click selects the finding in the quality dock source', async () => {
    reply()
    const fake = makeEditor()
    render(fake)
    await flush()
    act(() => fake.mouseListeners[0]({ target: { type: 2, position: { lineNumber: 2 } } }))
    const ui = useAppStore.getState().codeIntelQualityByWorktree.wt.ui
    expect(ui.selectedFingerprint).toBe('fp-1')
    expect(ui.source).toBe('quality')
    act(() => fake.mouseListeners[0]({ target: { type: 3, position: { lineNumber: 2 } } }))
    expect(useAppStore.getState().codeIntelQualityByWorktree.wt.ui.selectedFingerprint).toBe('fp-1')
  })

  it('hides waived findings unless showWaived is on', async () => {
    reply([{ ...FINDING, waiver: { by: 'u', reason: 'r', expiresAt: '2026-11-01T00:00:00Z' } }])
    const fake = makeEditor()
    render(fake)
    await flush()
    expect(setModelMarkers).not.toHaveBeenCalled()
    act(() => useAppStore.getState().setQualityUi('wt', { showWaived: true }))
    expect(setModelMarkers).toHaveBeenCalledTimes(1)
  })

  it('makes no call and draws nothing when not eligible, disabled or without a worktree', async () => {
    reply()
    const fake = makeEditor()
    render(fake, { diffSource: 'staged' })
    render(fake, { worktreeId: undefined })
    useAppStore.setState({ codeIntelSupportState: { state: 'disabled' } } as never)
    render(fake)
    await flush()
    expect(call.mock.calls.filter((c) => c[1] === 'codeIntel.quality.findings')).toHaveLength(0)
    expect(setModelMarkers).not.toHaveBeenCalled()
  })

  it('does not request findings before a finished run exists', async () => {
    call.mockImplementation(() => ok({ runs: [] }))
    render(makeEditor())
    await flush()
    expect(call.mock.calls.map((c) => c[1])).toEqual(['codeIntel.quality.runs'])
  })

  it('reports a soft warning for a dirty run but still draws', async () => {
    call.mockImplementation((_w, method) =>
      method === 'codeIntel.quality.runs'
        ? ok({ runs: [{ ...RUN, dirty: true }] })
        : ok({ findings: [FINDING] })
    )
    const { result } = render(makeEditor())
    await flush()
    expect(setModelMarkers).toHaveBeenCalledTimes(1)
    expect(result.current.notice).toBe('dirty')
  })
})
