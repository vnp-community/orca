/**
 * C4OverrideEditor.tsx — FE-CV-TASK-055-06
 *
 * Sheet editor for one container's c4.yaml. Local validation is syntax-only (O-12 open);
 * "Saved" appears only after the server confirms.
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'
import { translate } from '@/i18n/i18n'
import { useC4Override } from '../../../hooks/useC4Override'
import type { C4SaveFailure } from '../../../hooks/useC4Override'
import {
  lineColumnToOffset,
  validateC4OverrideDocument
} from '../../../../../shared/c4-override-document'
import type { C4OverrideIssue } from '../../../../../shared/c4-override-document'

const VALIDATE_DEBOUNCE_MS = 300
const SLOW_LABEL_MS = 1000
const SLOW_PHASE_MS = 3000

const EMPTY_TEMPLATE =
  '# c4.yaml: manual overrides for the inferred component diagram.\n# The server validates the meaning when you save.\n'

type ConflictState = { currentVersion: number | null; confirmingOverwrite: boolean }

export function C4OverrideEditor({
  open,
  worktreeId,
  environmentId,
  container,
  draft,
  onDraftChange,
  onSaved,
  onClose
}: {
  open: boolean
  worktreeId: string
  environmentId: string | null
  container: string
  /** Unsaved text kept in the store; null when there is none. */
  draft: string | null
  onDraftChange: (text: string | null) => void
  onSaved: () => void
  onClose: () => void
}): React.JSX.Element {
  const api = useC4Override({ worktreeId, environmentId, container })
  const { record } = api
  const [text, setText] = useState<string>(draft ?? '')
  const [baseline, setBaseline] = useState<string | null>(null)
  const [issues, setIssues] = useState<C4OverrideIssue[]>([])
  const [serverIssues, setServerIssues] = useState<string[]>([])
  const [warnings, setWarnings] = useState<{ code: string; message: string }[]>([])
  const [savedAt, setSavedAt] = useState<string | null>(null)
  const [failure, setFailure] = useState<C4SaveFailure | null>(null)
  const [conflict, setConflict] = useState<ConflictState | null>(null)
  const [confirmClose, setConfirmClose] = useState(false)
  const [elapsed, setElapsed] = useState(0)
  const textareaRef = useRef<HTMLTextAreaElement | null>(null)
  const adopted = useRef(false)

  // Adopt the server document once per load, unless an unsaved draft exists.
  useEffect(() => {
    if (!record || adopted.current) {
      return
    }
    adopted.current = true
    setBaseline(record.document)
    if (draft === null) {
      setText(record.document === '' ? EMPTY_TEMPLATE : record.document)
    }
  }, [record, draft])

  useEffect(() => {
    adopted.current = false
  }, [container])

  useEffect(() => {
    const t = setTimeout(() => setIssues(validateC4OverrideDocument(text).issues), VALIDATE_DEBOUNCE_MS)
    return () => clearTimeout(t)
  }, [text])

  useEffect(() => {
    if (!api.saving) {
      setElapsed(0)
      return
    }
    const a = setTimeout(() => setElapsed(1), SLOW_LABEL_MS)
    const b = setTimeout(() => setElapsed(2), SLOW_PHASE_MS)
    return () => {
      clearTimeout(a)
      clearTimeout(b)
    }
  }, [api.saving])

  const dirty = baseline !== null && text !== baseline && text !== EMPTY_TEMPLATE
  const hasErrors = issues.some((i) => i.severity === 'error')
  const canSave = !api.readOnly && !api.saving && dirty && !hasErrors && api.status === 'ready'

  const onChange = (next: string): void => {
    setText(next)
    setSavedAt(null)
    setFailure(null)
    setServerIssues([])
    onDraftChange(next)
  }

  const applyOutcome = useCallback(
    (outcome: Awaited<ReturnType<typeof api.save>>): void => {
      if (outcome.ok) {
        setBaseline(text)
        setWarnings(outcome.warnings)
        setSavedAt(new Date().toLocaleTimeString())
        setConflict(null)
        setFailure(null)
        onDraftChange(null)
        onSaved()
        return
      }
      const f = outcome.failure
      setFailure(f)
      if (f.kind === 'conflict') {
        setConflict({ currentVersion: f.currentVersion, confirmingOverwrite: false })
      } else if (f.kind === 'invalid') {
        setServerIssues([f.field ? `${f.field}: ${f.reason ?? ''}` : (f.reason ?? '')])
      }
    },
    [text, onDraftChange, onSaved]
  )

  const save = async (): Promise<void> => {
    if (!canSave) {
      return
    }
    applyOutcome(await api.save(text))
  }

  const jumpTo = (issue: C4OverrideIssue): void => {
    const el = textareaRef.current
    if (!el || issue.line <= 0) {
      return
    }
    const offset = lineColumnToOffset(text, issue.line, issue.column)
    el.focus()
    el.setSelectionRange(offset, offset)
  }

  const requestClose = (): void => {
    if (dirty && !confirmClose) {
      setConfirmClose(true)
      return
    }
    setConfirmClose(false)
    onClose()
  }

  const loadLatest = async (): Promise<void> => {
    const latest = await api.reload()
    if (latest) {
      setText(latest.document)
      setBaseline(latest.document)
      onDraftChange(null)
      setConflict(null)
      setFailure(null)
    }
  }

  const bytes = useMemo(() => new TextEncoder().encode(text).length, [text])

  return (
    <Sheet open={open} onOpenChange={(o) => (o ? undefined : requestClose())}>
      <SheetContent side="right" className="flex w-full flex-col gap-3 sm:min-w-[560px] sm:max-w-[560px]">
        <SheetHeader>
          <SheetTitle>{translate('auto.components.reviewMap.c4.editor.title', 'Edit c4.yaml')}</SheetTitle>
          <SheetDescription>
            {translate('auto.components.reviewMap.c4.editor.description', 'Manual overrides for {{container}}. The diagram is rebuilt by the server after saving.', { container })}
          </SheetDescription>
        </SheetHeader>

        {api.status === 'loading' ? (
          <p role="status" className="px-4 text-sm text-muted-foreground">
            {translate('auto.components.reviewMap.c4.editor.loading', 'Loading...')}
          </p>
        ) : null}
        {api.status === 'error' ? (
          <p role="alert" className="px-4 text-sm text-destructive">
            {translate('auto.components.reviewMap.c4.editor.loadError', 'Could not load c4.yaml.')} {api.loadError}
          </p>
        ) : null}
        {api.readOnly ? (
          <p role="status" className="px-4 text-xs text-muted-foreground">
            {translate('auto.components.reviewMap.c4.editor.readOnly', 'You do not have permission to edit c4.yaml.')}
          </p>
        ) : null}
        {record?.seedSource ? (
          <p className="px-4 text-xs text-muted-foreground">
            {translate('auto.components.reviewMap.c4.editor.seed', 'Template source: {{source}}', { source: record.seedSource })}
          </p>
        ) : null}

        <div className="flex min-h-0 flex-1 flex-col gap-2 px-4">
          <Textarea
            ref={textareaRef}
            value={text}
            readOnly={api.readOnly}
            spellCheck={false}
            aria-label={translate('auto.components.reviewMap.c4.editor.label', 'c4.yaml source')}
            className="min-h-0 flex-1 resize-none font-mono text-xs"
            onChange={(e) => onChange(e.target.value)}
            onKeyDown={(e) => {
              if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
                e.preventDefault()
                void save()
              }
            }}
          />
          <p className="text-[11px] text-muted-foreground">
            {translate('auto.components.reviewMap.c4.editor.size', '{{bytes}} / 65536 bytes', { bytes })} ·{' '}
            {translate('auto.components.reviewMap.c4.editor.semanticsNote', 'Meaning is not checked here; the server checks it when you save.')}
          </p>

          {issues.length > 0 || serverIssues.length > 0 ? (
            <ul aria-label={translate('auto.components.reviewMap.c4.editor.issues', 'Issues')} className="max-h-28 space-y-0.5 overflow-auto text-xs">
              {issues.map((i, idx) => (
                <li key={`${i.line}:${i.column}:${idx}`}>
                  <button type="button" className="text-left text-destructive hover:underline" onClick={() => jumpTo(i)}>
                    {i.line > 0 ? `${i.line}:${i.column} ` : ''}
                    {i.message}
                  </button>
                </li>
              ))}
              {serverIssues.map((s) => (
                <li key={s} className="text-destructive">
                  {s}
                </li>
              ))}
            </ul>
          ) : null}

          {warnings.length > 0 ? (
            <ul aria-label={translate('auto.components.reviewMap.c4.editor.warnings', 'Server warnings')} className="space-y-0.5 text-xs text-muted-foreground">
              {warnings.map((w, i) => (
                <li key={`${w.code}-${i}`}>{w.message}</li>
              ))}
            </ul>
          ) : null}

          {failure && failure.kind === 'offline' ? (
            <p role="alert" className="text-xs text-destructive">
              {translate('auto.components.reviewMap.c4.editor.offline', 'Not saved: the connection failed. Your draft is kept.')}
            </p>
          ) : null}
          {failure && failure.kind === 'too-large' ? (
            <p role="alert" className="text-xs text-destructive">
              {translate('auto.components.reviewMap.c4.editor.tooLarge', 'Not saved: the document is too large.')}
            </p>
          ) : null}
          {failure && failure.kind === 'other' ? (
            <p role="alert" className="text-xs text-destructive">
              {translate('auto.components.reviewMap.c4.editor.failed', 'Not saved: {{message}}', { message: failure.message })}
            </p>
          ) : null}

          {conflict ? (
            <div role="alertdialog" aria-label={translate('auto.components.reviewMap.c4.editor.conflictTitle', 'Version conflict')} className="space-y-2 rounded-md border p-2 text-xs">
              <p>{translate('auto.components.reviewMap.c4.editor.conflict', 'Someone else saved a newer c4.yaml.')}</p>
              <div className="flex flex-wrap gap-2">
                <Button size="xs" variant="outline" onClick={() => void loadLatest()}>
                  {translate('auto.components.reviewMap.c4.editor.loadLatest', 'Load latest')}
                </Button>
                <Button size="xs" variant="outline" onClick={() => void navigator.clipboard?.writeText(text)}>
                  {translate('auto.components.reviewMap.c4.editor.copyMine', 'Copy mine')}
                </Button>
                {conflict.confirmingOverwrite ? (
                  <Button
                    size="xs"
                    variant="destructive"
                    disabled={api.saving}
                    onClick={async () => applyOutcome(await api.overwrite(text))}
                  >
                    {translate('auto.components.reviewMap.c4.editor.confirmOverwrite', 'Confirm: replace the newer version')}
                  </Button>
                ) : (
                  <Button size="xs" variant="outline" onClick={() => setConflict({ ...conflict, confirmingOverwrite: true })}>
                    {translate('auto.components.reviewMap.c4.editor.overwrite', 'Overwrite with mine')}
                  </Button>
                )}
              </div>
            </div>
          ) : null}

          {confirmClose ? (
            <div role="alertdialog" aria-label={translate('auto.components.reviewMap.c4.editor.discardTitle', 'Discard changes')} className="flex items-center gap-2 rounded-md border p-2 text-xs">
              <span className="flex-1">{translate('auto.components.reviewMap.c4.editor.discard', 'Close without saving your changes?')}</span>
              <Button size="xs" variant="outline" onClick={() => setConfirmClose(false)}>
                {translate('auto.components.reviewMap.c4.editor.keepEditing', 'Keep editing')}
              </Button>
              <Button size="xs" variant="destructive" onClick={() => { setConfirmClose(false); onClose() }}>
                {translate('auto.components.reviewMap.c4.editor.discardAction', 'Discard')}
              </Button>
            </div>
          ) : null}
        </div>

        <div className="flex items-center gap-2 border-t px-4 py-3">
          <span role="status" className="flex-1 text-xs text-muted-foreground">
            {api.saving
              ? elapsed >= 2
                ? translate('auto.components.reviewMap.c4.editor.savingSlow', 'Waiting for the server...')
                : elapsed >= 1
                  ? translate('auto.components.reviewMap.c4.editor.saving', 'Saving...')
                  : ''
              : savedAt
                ? translate('auto.components.reviewMap.c4.editor.savedAt', 'Saved at {{time}}', { time: savedAt })
                : dirty
                  ? translate('auto.components.reviewMap.c4.editor.unsaved', 'Unsaved')
                  : ''}
          </span>
          <Button variant="outline" size="sm" onClick={requestClose}>
            {translate('auto.components.reviewMap.c4.editor.close', 'Close')}
          </Button>
          <Button size="sm" disabled={!canSave} onClick={() => void save()}>
            {api.saving ? <Loader2 className="size-3.5 animate-spin" aria-hidden="true" /> : null}
            {translate('auto.components.reviewMap.c4.editor.save', 'Save')}
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  )
}
