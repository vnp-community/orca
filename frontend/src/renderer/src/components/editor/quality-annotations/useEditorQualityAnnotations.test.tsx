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
import { useEditorQualityAnnotations } from './useEditorQualityAnnotations'
import type { UseEditorQualityAnnotationsArgs } from './useEditorQualityAnnotations'
import type { QualityMonacoApi } from './useQualityFindingMarkers'

const ok = (result: unknown) => Promise.resolve({ ok: true as const, result })
const RUN = { id: 'run-1', headCommit: 'abc', status: 'succeeded', source: 'local' }
const FINDING = {
  fingerprint: 'fp-1',
  ruleId: 'r1',
  severity: 'warning',
  category: 'lint',
  file: 'src/a.ts',
  line: 2,
  endLine: 2,
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
  const collection = { clear: vi.fn() }
  const model = { getLineCount: () => 5, getLineMaxColumn: () => 20 }
  const editor = {
    getModel: () => model,
    onDidChangeModel: vi.fn(() => ({ dispose: vi.fn() })),
    onDidChangeModelContent: vi.fn((cb: () => void) => {
      contentListeners.push(cb)
      return { dispose: vi.fn() }
    }),
    onMouseDown: vi.fn(() => ({ dispose: vi.fn() })),
    updateOptions: vi.fn(),
    createDecorationsCollection: vi.fn(() => collection)
  }
  return { editor, model, collection, contentListeners }
}

const setModelMarkers = vi.fn()
const monacoApi: QualityMonacoApi = {
  MarkerSeverity: { Error: 8, Warning: 4, Info: 2, Hint: 1 },
  editor: { setModelMarkers, MouseTargetType: { GUTTER_GLYPH_MARGIN: 2 } }
}

function render(
  fake: ReturnType<typeof makeEditor>,
  patch: Partial<UseEditorQualityAnnotationsArgs> = {}
) {
  return renderHook(() =>
    useEditorQualityAnnotations({
      editor: fake.editor as never,
      monacoApi,
      fileId: 'f1',
      worktreeId: 'wt',
      relativePath: 'src/a.ts',
      ...patch
    })
  )
}

const findingCalls = () => call.mock.calls.filter((c) => c[1] === 'codeIntel.quality.findings')

beforeEach(() => {
  call.mockReset()
  setModelMarkers.mockReset()
  call.mockImplementation((_w, method) =>
    method === 'codeIntel.quality.runs' ? ok({ runs: [RUN] }) : ok({ findings: [FINDING] })
  )
  useAppStore.setState({
    codeIntelSupportState: ENABLED_QUALITY_SUPPORT,
    codeIntelQualityByWorktree: {},
    gitStatusHeadByWorktree: { wt: 'abc' },
    openFiles: [{ id: 'f1', isDirty: false }],
    editorDrafts: {}
  } as never)
})
afterEach(() => cleanup())

describe('useEditorQualityAnnotations', () => {
  it('draws worktree-side markers in a clean plain editor', async () => {
    const fake = makeEditor()
    render(fake)
    await flush()
    expect(setModelMarkers).toHaveBeenCalledTimes(1)
    expect(setModelMarkers.mock.calls[0][1]).toBe('orca-quality')
    expect(fake.editor.updateOptions).toHaveBeenCalledWith({ glyphMargin: true })
  })

  it('skips a tab that opened with an unsaved draft or dirty buffer', async () => {
    useAppStore.setState({ editorDrafts: { f1: 'draft' } } as never)
    render(makeEditor())
    useAppStore.setState({ editorDrafts: {}, openFiles: [{ id: 'f1', isDirty: true }] } as never)
    render(makeEditor())
    await flush()
    expect(findingCalls()).toHaveLength(0)
    expect(setModelMarkers).not.toHaveBeenCalled()
  })

  it('skips read-only tabs and files outside a worktree', async () => {
    render(makeEditor(), { readOnly: true })
    render(makeEditor(), { worktreeId: undefined })
    await flush()
    expect(findingCalls()).toHaveLength(0)
    expect(setModelMarkers).not.toHaveBeenCalled()
  })

  it('keeps the mount-time decision when the buffer turns dirty, clearing on edit with a notice', async () => {
    const fake = makeEditor()
    const { result } = render(fake)
    await flush()
    act(() => {
      useAppStore.setState({ openFiles: [{ id: 'f1', isDirty: true }] } as never)
      fake.contentListeners.forEach((cb) => cb())
    })
    await flush()
    expect(setModelMarkers).toHaveBeenLastCalledWith(fake.model, 'orca-quality', [])
    expect(result.current.notice).toBe('content-changed')
  })

  it('clears when annotations are switched off', async () => {
    const fake = makeEditor()
    render(fake)
    await flush()
    act(() => useAppStore.getState().setQualityUi('wt', { annotationsOn: false }))
    expect(fake.collection.clear).toHaveBeenCalled()
    expect(setModelMarkers).toHaveBeenLastCalledWith(fake.model, 'orca-quality', [])
  })
})
