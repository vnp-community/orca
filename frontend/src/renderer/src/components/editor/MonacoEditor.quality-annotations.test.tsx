// @vitest-environment happy-dom
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

const qualityHook = vi.hoisted(() => vi.fn())

vi.mock('@monaco-editor/react', () => ({
  default: () => null,
  loader: { config: vi.fn() }
}))
vi.mock('@/store', () => ({
  useAppStore: (selector: (state: Record<string, unknown>) => unknown) =>
    selector({
      settings: { theme: 'dark', terminalFontSize: 13, terminalFontFamily: 'monospace' },
      editorFontZoomLevel: 0,
      setPendingEditorReveal: vi.fn(),
      setEditorCursorLine: vi.fn(),
      addDiffComment: vi.fn(),
      deleteDiffComment: vi.fn(),
      updateDiffComment: vi.fn(),
      scrollToDiffCommentId: null,
      setScrollToDiffCommentId: vi.fn(),
      worktreeDiffComments: {},
      worktreesByRepo: {},
      codeIntelQualityByWorktree: {}
    })
}))
vi.mock('../diff-comments/useDiffCommentDecorator', () => ({
  useDiffCommentDecorator: vi.fn()
}))
vi.mock('./useContextualCopySetup', () => ({
  useContextualCopySetup: () => ({ setupCopy: vi.fn(), toastNode: null })
}))
vi.mock('./quality-annotations/useEditorQualityAnnotations', () => ({
  useEditorQualityAnnotations: qualityHook
}))

import { monaco } from '@/lib/monaco-setup'
import MonacoEditor from './MonacoEditor'

afterEach(() => {
  cleanup()
  qualityHook.mockReset()
})

function renderEditor(worktreeId?: string): void {
  render(
    <MonacoEditor
      fileId="file-1"
      filePath="/repo/src/a.ts"
      viewStateKey="pane:file"
      relativePath="src/a.ts"
      content="const a = 1"
      language="typescript"
      onContentChange={vi.fn()}
      onSave={vi.fn()}
      worktreeId={worktreeId}
    />
  )
}

describe('MonacoEditor quality annotations (087-11)', () => {
  it('calls the quality hook once per render with the file identity and real monaco api', () => {
    qualityHook.mockReturnValue({ notice: null, toggleVisible: false })
    renderEditor('wt')
    expect(qualityHook).toHaveBeenCalled()
    expect(qualityHook.mock.calls[0][0]).toEqual({
      editor: null,
      monacoApi: monaco,
      fileId: 'file-1',
      worktreeId: 'wt',
      relativePath: 'src/a.ts',
      readOnly: false
    })
    expect(screen.queryByTestId('quality-annotation-strip')).toBeNull()
  })

  it('floats the notice strip over the editor when the hook reports one', () => {
    qualityHook.mockReturnValue({ notice: 'content-changed', toggleVisible: false })
    renderEditor('wt')
    const strip = screen.getByTestId('quality-annotation-strip')
    expect(strip.className).toContain('absolute')
    expect(strip.textContent).not.toBe('')
  })

  it('renders no strip without a worktree', () => {
    qualityHook.mockReturnValue({ notice: 'content-changed', toggleVisible: true })
    renderEditor(undefined)
    expect(screen.queryByTestId('quality-annotation-strip')).toBeNull()
  })
})
