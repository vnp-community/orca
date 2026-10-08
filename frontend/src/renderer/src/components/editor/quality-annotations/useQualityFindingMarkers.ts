/**
 * useQualityFindingMarkers.ts — FE-CV-TASK-087-10
 *
 * Draws check findings on the modified side of a diff: Monaco markers under the fixed owner
 * 'orca-quality' plus gutter glyph decorations. Markers do not follow edits, so everything is
 * removed the moment the model content changes. Nothing is requested or drawn unless quality
 * support is enabled, annotations are on, and the diff side matches the run (eligibility).
 *
 * @module components/editor/quality-annotations/useQualityFindingMarkers
 */

import { useEffect, useMemo, useRef, useState } from 'react'
import type { editor } from 'monaco-editor'
import { useAppStore } from '@/store'
import { useQualitySupport } from '../../../hooks/useQualitySupport'
import { useLatestQualityRun } from '../../../hooks/useLatestQualityRun'
import { useQualityFindingsForFile } from '../../../hooks/useQualityFindingsForFile'
import { DEFAULT_QUALITY_UI } from '../../../store/slices/code-intel-quality-state-types'
import { isAnnotationEligible } from './quality-annotation-eligibility'
import type {
  QualityAnnotationSectionArea,
  QualityAnnotationSource
} from './quality-annotation-eligibility'
import { resolveQualityAnnotationNotice } from './quality-annotation-notice'
import type { QualityAnnotationNoticeKind } from './quality-annotation-notice'
import { buildQualityGlyphDecorations } from './quality-glyph-decorations'
import { buildQualityMarkers } from './quality-marker-model'
import type {
  QualityGlyph,
  QualityMarkerData,
  QualityMarkerSeverityMap
} from './quality-marker-model'

export const QUALITY_MARKER_OWNER = 'orca-quality'

/** The slice of the Monaco namespace this hook needs; callers pass the real `monaco`. */
export type QualityMonacoApi = {
  MarkerSeverity: { Error: number; Warning: number; Info: number; Hint: number }
  editor: {
    setModelMarkers: (model: editor.ITextModel, owner: string, markers: QualityMarkerData[]) => void
    MouseTargetType: { GUTTER_GLYPH_MARGIN: number }
  }
}

export type UseQualityFindingMarkersArgs = {
  editor: editor.ICodeEditor | null
  monacoApi: QualityMonacoApi
  worktreeId: string | null | undefined
  relativePath: string
  diffSource: QualityAnnotationSource | undefined
  sectionArea?: QualityAnnotationSectionArea
  compareHeadOid?: string | null
}

export type UseQualityFindingMarkersResult = {
  notice: QualityAnnotationNoticeKind | null
  /** Show the on/off switch: an eligible run exists and annotations are off or have findings. */
  toggleVisible: boolean
}

function severityMapOf(api: QualityMonacoApi): QualityMarkerSeverityMap {
  const s = api.MarkerSeverity
  return { error: s.Error, warning: s.Warning, info: s.Info, hint: s.Hint }
}

export function useQualityFindingMarkers(
  args: UseQualityFindingMarkersArgs
): UseQualityFindingMarkersResult {
  const {
    editor: codeEditor,
    monacoApi,
    worktreeId,
    relativePath,
    diffSource,
    sectionArea,
    compareHeadOid
  } = args
  const support = useQualitySupport(worktreeId)
  const annotationsOn = useAppStore((s) =>
    worktreeId
      ? (s.codeIntelQualityByWorktree[worktreeId]?.ui.annotationsOn ??
        DEFAULT_QUALITY_UI.annotationsOn)
      : false
  )
  const showWaived = useAppStore((s) =>
    worktreeId ? (s.codeIntelQualityByWorktree[worktreeId]?.ui.showWaived ?? false) : false
  )
  const currentHead = useAppStore((s) =>
    worktreeId ? s.gitStatusHeadByWorktree?.[worktreeId] : undefined
  )

  const wanted = support === 'enabled' && annotationsOn && Boolean(worktreeId)
  const run = useLatestQualityRun(worktreeId, wanted)
  const eligibility = useMemo(
    () =>
      isAnnotationEligible({
        diffSource,
        sectionArea,
        runHeadCommit: run?.headCommit,
        currentHead,
        compareHeadOid,
        hasWorktreeId: Boolean(worktreeId),
        runDirty: run?.dirty,
        workTreeChangedDuringRun: run?.workTreeChangedDuringRun
      }),
    [diffSource, sectionArea, run, currentHead, compareHeadOid, worktreeId]
  )
  const active = wanted && run !== null && eligibility.eligible
  const findings = useQualityFindingsForFile(worktreeId, relativePath, {
    enabled: active,
    runId: run?.id
  })
  const visible = useMemo(
    () => (findings ? (showWaived ? findings : findings.filter((f) => !f.waiver)) : null),
    [findings, showWaived]
  )

  const [contentChanged, setContentChanged] = useState(false)
  const [modelVersion, setModelVersion] = useState(0)
  const clearRef = useRef<(() => void) | null>(null)
  const glyphsRef = useRef<QualityGlyph[]>([])
  const runId = run?.id

  useEffect(() => {
    // Why: a new run or another file starts from a clean slate; old edits no longer matter.
    setContentChanged(false)
  }, [codeEditor, runId, relativePath])

  useEffect(() => {
    if (!codeEditor || !active) {
      return
    }
    const modelSub = codeEditor.onDidChangeModel(() => setModelVersion((v) => v + 1))
    const contentSub = codeEditor.onDidChangeModelContent(() => {
      const hadMarkers = clearRef.current !== null
      clearRef.current?.()
      if (hadMarkers) {
        setContentChanged(true)
      }
    })
    const mouseSub = codeEditor.onMouseDown((event) => {
      const position = event.target.position
      if (
        !worktreeId ||
        !position ||
        event.target.type !== monacoApi.editor.MouseTargetType.GUTTER_GLYPH_MARGIN
      ) {
        return
      }
      const glyph = glyphsRef.current.find((g) => g.line === position.lineNumber)
      if (glyph) {
        useAppStore
          .getState()
          .setQualityUi(worktreeId, { selectedFingerprint: glyph.fingerprint, source: 'quality' })
      }
    })
    return () => {
      modelSub.dispose()
      contentSub.dispose()
      mouseSub.dispose()
    }
  }, [codeEditor, active, monacoApi, worktreeId, modelVersion])

  useEffect(() => {
    const model = codeEditor?.getModel() ?? null
    if (!codeEditor || !model || !active || !visible || contentChanged) {
      return
    }
    const built = buildQualityMarkers(visible, {
      lineCount: model.getLineCount(),
      lineMaxColumn: (line) => model.getLineMaxColumn(line),
      severityMap: severityMapOf(monacoApi)
    })
    if (built.markers.length === 0) {
      return
    }
    monacoApi.editor.setModelMarkers(model, QUALITY_MARKER_OWNER, built.markers)
    codeEditor.updateOptions({ glyphMargin: true })
    const collection = codeEditor.createDecorationsCollection(
      buildQualityGlyphDecorations(built.glyphs)
    )
    glyphsRef.current = built.glyphs
    let cleared = false
    const clear = (): void => {
      if (cleared) {
        return
      }
      cleared = true
      glyphsRef.current = []
      try {
        collection.clear()
        monacoApi.editor.setModelMarkers(model, QUALITY_MARKER_OWNER, [])
      } catch {
        // Why: the editor or model may already be disposed while the diff unmounts.
      }
    }
    clearRef.current = clear
    return () => {
      clear()
      if (clearRef.current === clear) {
        clearRef.current = null
      }
    }
  }, [codeEditor, active, visible, contentChanged, monacoApi, modelVersion])

  const notice = active
    ? resolveQualityAnnotationNotice({ contentChanged, softWarning: eligibility.softWarning })
    : null
  const hasVisible = (visible?.length ?? 0) > 0
  return {
    notice: notice !== null && (contentChanged || hasVisible) ? notice : null,
    toggleVisible:
      support === 'enabled' &&
      run !== null &&
      eligibility.eligible &&
      (!annotationsOn || hasVisible)
  }
}
