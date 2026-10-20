/**
 * RevisionPromptBox — CR-REQ-041, CR-REQ-045
 *
 * The box where a reviewer types what should change in the document under review (analysis or plan): text area, counter in
 * characters, suggestion chips, send button and the Cmd/Ctrl+Enter shortcut. It owns no state beyond focus: the text and the
 * sending live in the hook of each gate, so they survive a failed send. The titles and the test ids come from the caller.
 *
 * @module components/request/solution/RevisionPromptBox
 */

import React, { useId, useRef } from 'react'
import { Loader2, Wand2 } from 'lucide-react'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { getShortcutPlatform } from '../../../lib/shortcut-platform'
import type { RevisionSuggestion } from './revision-suggestions'
import { FEEDBACK_MAX_LENGTH } from './solution-view-model'

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

export type RevisionPromptLabels = {
  title: string
  description: string
  fieldLabel: string
  placeholder: string
  submit: string
}

export type RevisionPromptBoxProps = {
  draft: string
  setDraft: (text: string) => void
  onSubmit: () => void
  busy: boolean
  /** What the status line says while busy. */
  busyText: string
  error: { message: string } | null
  suggestions: readonly RevisionSuggestion[]
  labels: RevisionPromptLabels
  /** `analysis-revision` gives `analysis-revision-panel`, `-input`, `-chip-<id>`, `-error`, `-count`, `-status`, `-submit`. */
  testIdPrefix: string
}

export function RevisionPromptBox({
  draft,
  setDraft,
  onSubmit,
  busy,
  busyText,
  error,
  suggestions,
  labels,
  testIdPrefix
}: RevisionPromptBoxProps): React.JSX.Element {
  const id = useId()
  const titleId = `${id}-title`
  const fieldId = `${id}-field`
  const countId = `${id}-count`
  const errorId = `${id}-error`
  const fieldRef = useRef<HTMLTextAreaElement>(null)
  const mac = getShortcutPlatform() === 'darwin'
  const length = [...draft].length
  const overLimit = length > FEEDBACK_MAX_LENGTH
  const shortcut = mac ? '⌘Enter' : 'Ctrl+Enter'

  return (
    <section
      data-testid={`${testIdPrefix}-panel`}
      aria-labelledby={titleId}
      aria-busy={busy}
      className="flex flex-col gap-2 rounded-md border border-border bg-card p-3"
    >
      <h3 id={titleId} className="text-sm font-medium">
        {labels.title}
      </h3>
      <p className="text-xs text-muted-foreground">{labels.description}</p>
      <label htmlFor={fieldId} className="text-xs font-medium">
        {labels.fieldLabel}
      </label>
      <Textarea
        ref={fieldRef}
        id={fieldId}
        data-testid={`${testIdPrefix}-input`}
        rows={3}
        value={draft}
        disabled={busy}
        aria-invalid={error !== null || overLimit}
        aria-describedby={error ? `${countId} ${errorId}` : countId}
        aria-keyshortcuts={mac ? 'Meta+Enter' : 'Control+Enter'}
        placeholder={labels.placeholder}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (isSubmitShortcut(e, mac)) {
            e.preventDefault()
            onSubmit()
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
              data-testid={`${testIdPrefix}-chip-${s.id}`}
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
          data-testid={`${testIdPrefix}-error`}
          className="text-xs text-destructive"
        >
          {error.message}
        </p>
      )}
      <div className="flex flex-wrap items-center gap-2">
        <span
          id={countId}
          data-testid={`${testIdPrefix}-count`}
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
          data-testid={`${testIdPrefix}-status`}
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
          data-testid={`${testIdPrefix}-submit`}
          onClick={onSubmit}
        >
          {busy ? (
            <Loader2 className="size-4 animate-spin" aria-hidden />
          ) : (
            <Wand2 className="size-4" aria-hidden />
          )}
          {labels.submit}
        </Button>
      </div>
    </section>
  )
}
