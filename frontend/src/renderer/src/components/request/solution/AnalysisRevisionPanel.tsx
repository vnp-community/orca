/**
 * AnalysisRevisionPanel — CR-REQ-041
 *
 * At the Analysis gate a reviewer types what should change and the AI writes the next version, without rejecting the
 * one on screen. The text lives in `useAnalysisRevision`, so it survives a failed send and a re-render of the panel.
 *
 * @module components/request/solution/AnalysisRevisionPanel
 */

import React, { useId, useRef } from 'react'
import { Loader2, Wand2 } from 'lucide-react'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { getShortcutPlatform } from '../../../lib/shortcut-platform'
import type { SolutionKind } from '../../../../../shared/request-types'
import { revisionSuggestions } from './revision-suggestions'
import { FEEDBACK_MAX_LENGTH } from './solution-view-model'
import type { AnalysisRevision } from './useAnalysisRevision'

/** Cmd+Enter on macOS, Ctrl+Enter elsewhere; any other modifier combination is left to the textarea. */
function isSubmitShortcut(event: React.KeyboardEvent<HTMLTextAreaElement>, mac: boolean): boolean {
  if (event.key !== 'Enter' || event.nativeEvent.isComposing || event.shiftKey || event.altKey) {
    return false
  }
  return mac ? event.metaKey && !event.ctrlKey : event.ctrlKey && !event.metaKey
}

function appendSentence(draft: string, sentence: string): string {
  const base = draft.trimEnd()
  return base === '' ? sentence : `${base} ${sentence}`
}

export type AnalysisRevisionPanelProps = {
  revision: AnalysisRevision
  kind: SolutionKind
}

export function AnalysisRevisionPanel({
  revision,
  kind
}: AnalysisRevisionPanelProps): React.JSX.Element {
  const { busy, draft, error, nextVersion, setDraft, submit } = revision
  const id = useId()
  const titleId = `${id}-title`
  const fieldId = `${id}-field`
  const countId = `${id}-count`
  const errorId = `${id}-error`
  const fieldRef = useRef<HTMLTextAreaElement>(null)
  const mac = getShortcutPlatform() === 'darwin'
  const length = [...draft].length
  const overLimit = length > FEEDBACK_MAX_LENGTH
  const suggestions = revisionSuggestions(kind)
  const shortcut = mac ? '⌘Enter' : 'Ctrl+Enter'
  const busyText =
    nextVersion !== undefined
      ? translate('auto.components.request.AnalysisRevision.busy', 'Analyzing version {{n}}…', {
          n: nextVersion
        })
      : translate(
          'auto.components.request.AnalysisRevision.busyNext',
          'Analyzing the next version…'
        )

  return (
    <section
      data-testid="analysis-revision-panel"
      aria-labelledby={titleId}
      aria-busy={busy}
      className="flex flex-col gap-2 rounded-md border border-border bg-card p-3"
    >
      <h3 id={titleId} className="text-sm font-medium">
        {translate(
          'auto.components.request.AnalysisRevision.title',
          'Request changes to this analysis'
        )}
      </h3>
      <p className="text-xs text-muted-foreground">
        {translate(
          'auto.components.request.AnalysisRevision.hint',
          'Describe what should change. The AI writes a new version and this one stays in the history.'
        )}
      </p>
      <label htmlFor={fieldId} className="text-xs font-medium">
        {translate('auto.components.request.AnalysisRevision.fieldLabel', 'What should change?')}
      </label>
      <Textarea
        ref={fieldRef}
        id={fieldId}
        data-testid="analysis-revision-input"
        rows={3}
        value={draft}
        disabled={busy}
        aria-invalid={error !== null || overLimit}
        aria-describedby={error ? `${countId} ${errorId}` : countId}
        aria-keyshortcuts={mac ? 'Meta+Enter' : 'Control+Enter'}
        placeholder={translate(
          'auto.components.request.AnalysisRevision.placeholder',
          'For example: compare the options on cost and add a rollback plan.'
        )}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (isSubmitShortcut(e, mac)) {
            e.preventDefault()
            void submit()
          }
        }}
      />
      {suggestions.length > 0 && (
        <div
          role="group"
          aria-label={translate(
            'auto.components.request.AnalysisRevision.suggestions',
            'Suggestions'
          )}
          className="flex flex-wrap gap-1"
        >
          {suggestions.map((s) => (
            <Button
              key={s.id}
              type="button"
              size="xs"
              variant="outline"
              disabled={busy}
              data-testid={`analysis-revision-chip-${s.id}`}
              onClick={() => {
                setDraft(appendSentence(draft, s.text))
                fieldRef.current?.focus()
              }}
            >
              {s.text}
            </Button>
          ))}
        </div>
      )}
      {error && (
        <p
          id={errorId}
          role="alert"
          data-testid="analysis-revision-error"
          className="text-xs text-destructive"
        >
          {error.message}
        </p>
      )}
      <div className="flex flex-wrap items-center gap-2">
        <span
          id={countId}
          data-testid="analysis-revision-count"
          className={cn(
            'text-xs tabular-nums',
            overLimit ? 'text-destructive' : 'text-muted-foreground'
          )}
        >
          {translate('auto.components.request.AnalysisRevision.counter', '{{count}} / {{max}}', {
            count: length,
            max: FEEDBACK_MAX_LENGTH
          })}
        </span>
        <div
          role="status"
          aria-live="polite"
          data-testid="analysis-revision-status"
          className="text-xs text-muted-foreground"
        >
          {busy && busyText}
        </div>
        <span className="ml-auto text-xs text-muted-foreground">
          {translate(
            'auto.components.request.AnalysisRevision.shortcutHint',
            'or press {{shortcut}}',
            { shortcut }
          )}
        </span>
        <Button
          type="button"
          disabled={busy}
          data-testid="analysis-revision-submit"
          onClick={() => void submit()}
        >
          {busy ? (
            <Loader2 className="size-4 animate-spin" aria-hidden />
          ) : (
            <Wand2 className="size-4" aria-hidden />
          )}
          {translate('auto.components.request.AnalysisRevision.submit', 'Revise analysis')}
        </Button>
      </div>
    </section>
  )
}
