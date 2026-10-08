/**
 * ClarificationQuestionField — FE-REQ-TASK-036-03
 *
 * One input per question kind. Suggested defaults are pre-shown but never
 * submitted without an explicit "Use suggestion".
 *
 * @module components/request/clarification/ClarificationQuestionField
 */

import React, { useId } from 'react'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { ShortcutKeyCombo } from '@/components/ShortcutKeyCombo'
import { getScreenSubmitModifierLabel, isScreenSubmitShortcut } from '@/lib/screen-submit-shortcut'
import { MAX_FILE_TEXT_BYTES, MAX_TEXT_LENGTH, isTextFile, validateAnswer, type FileAnswer } from './clarification-answer-validation'
import type { ClarificationQuestion, ClarificationSource } from '../../../../../shared/request-artifact-types'

const T = 'auto.components.request.clarification.'

type Props = {
  question: ClarificationQuestion
  source: ClarificationSource
  value: unknown
  onChange: (value: unknown) => void
  defaultAccepted: boolean
  onAcceptDefault: (accepted: boolean) => void
  readOnly?: boolean
  /** Server- or client-side error text for this field. */
  error?: string | null
  /** Show required errors (after a failed submit attempt). */
  showErrors?: boolean
  onSubmitShortcut?: () => void
}

const SOURCE_FALLBACK: Record<string, string> = {
  readiness: 'Readiness check',
  solution_open_question: 'Solution open question',
  plan_assumption: 'Plan assumption',
  task_blocked: 'Task blocked'
}

export function ClarificationQuestionField({
  question, source, value, onChange, defaultAccepted, onAcceptDefault, readOnly, error, showErrors, onSubmitShortcut
}: Props): React.JSX.Element {
  const uid = useId()
  const inputId = `${uid}-input`
  const errId = `${uid}-error`
  const [fileError, setFileError] = React.useState<string | null>(null)
  const check = validateAnswer(question, value)
  const clientError = !defaultAccepted && showErrors && !check.ok ? translate(check.reasonKey, 'Invalid answer') : null
  const message = error ?? fileError ?? clientError
  const hasDefault = question.suggestedDefault !== undefined
  const untouched = value === undefined || value === ''
  const disabled = readOnly || defaultAccepted

  const shown = defaultAccepted ? question.suggestedDefault : value

  let control: React.ReactNode
  switch (question.kind) {
    case 'text':
      control = (
        <div className="flex flex-col gap-1">
          <Textarea
            id={inputId}
            value={typeof shown === 'string' ? shown : ''}
            maxLength={MAX_TEXT_LENGTH}
            disabled={disabled}
            aria-invalid={message ? true : undefined}
            aria-describedby={message ? errId : undefined}
            onChange={(e) => onChange(e.target.value)}
            onKeyDown={(e) => {
              if (isScreenSubmitShortcut(e)) {
                e.preventDefault()
                onSubmitShortcut?.()
              }
            }}
            className="min-h-20"
          />
          {onSubmitShortcut && !readOnly ? (
            <span className="flex items-center gap-1 text-[11px] text-muted-foreground">
              <ShortcutKeyCombo keys={[getScreenSubmitModifierLabel(), 'Enter']} />
              {translate(`${T}submitHint`, 'to submit')}
            </span>
          ) : null}
        </div>
      )
      break
    case 'single_choice':
      control = (
        <ToggleGroup
          id={inputId}
          type="single"
          variant="outline"
          size="sm"
          className="flex-wrap"
          value={typeof shown === 'string' ? shown : ''}
          disabled={disabled}
          onValueChange={(v) => onChange(v || undefined)}
        >
          {(question.options ?? []).map((o) => (
            <ToggleGroupItem key={o.value} value={o.value}>{o.label}</ToggleGroupItem>
          ))}
        </ToggleGroup>
      )
      break
    case 'multi_choice': {
      const selected = Array.isArray(shown) ? (shown as string[]) : []
      control = (
        <div className="flex flex-col gap-1.5" role="group" aria-labelledby={`${uid}-label`}>
          {(question.options ?? []).map((o) => (
            <Label key={o.value} className="flex items-center gap-2 text-sm font-normal">
              <Checkbox
                checked={selected.includes(o.value)}
                disabled={disabled}
                onCheckedChange={(checked) =>
                  onChange(checked ? [...selected, o.value] : selected.filter((x) => x !== o.value))
                }
              />
              {o.label}
            </Label>
          ))}
        </div>
      )
      break
    }
    case 'boolean':
      control = (
        <ToggleGroup
          id={inputId}
          type="single"
          variant="outline"
          size="sm"
          value={shown === true ? 'yes' : shown === false ? 'no' : ''}
          disabled={disabled}
          onValueChange={(v) => onChange(v === 'yes' ? true : v === 'no' ? false : undefined)}
        >
          <ToggleGroupItem value="yes">{translate(`${T}yes`, 'Yes')}</ToggleGroupItem>
          <ToggleGroupItem value="no">{translate(`${T}no`, 'No')}</ToggleGroupItem>
        </ToggleGroup>
      )
      break
    case 'file': {
      const file = shown as Partial<FileAnswer> | undefined
      control = (
        <div className="flex flex-col gap-1">
          <input
            id={inputId}
            type="file"
            disabled={disabled}
            aria-invalid={message ? true : undefined}
            aria-describedby={message ? errId : undefined}
            className="text-xs"
            onChange={async (e) => {
              const f = e.target.files?.[0]
              if (!f) {return}
              setFileError(null)
              // Why: no upload store exists; v1 accepts small text files read in the renderer.
              if (f.size > MAX_FILE_TEXT_BYTES) {
                setFileError(translate(`${T}fileTooLarge`, 'File is larger than 64 KB'))
                return
              }
              const text = await f.text()
              if (!isTextFile(f.type, text)) {
                setFileError(translate(`${T}fileBinary`, 'Only text files are supported'))
                return
              }
              onChange({ filename: f.name, mime: f.type, size: f.size, text } satisfies FileAnswer)
            }}
          />
          {file?.filename ? <span className="text-xs text-muted-foreground">{file.filename}</span> : null}
        </div>
      )
      break
    }
  }

  return (
    <div className="flex flex-col gap-1.5" data-question-id={question.id}>
      <div className="flex flex-wrap items-center gap-2">
        <Label id={`${uid}-label`} htmlFor={inputId} className="text-sm font-medium">
          {question.prompt}
          {question.required ? (
            <>
              <span aria-hidden className="ml-0.5 text-destructive">*</span>
              <span className="sr-only">{translate(`${T}required`, 'Required')}</span>
            </>
          ) : null}
        </Label>
        {source !== 'unknown' ? (
          <span className="rounded border border-border px-1.5 py-0.5 text-[10px] text-muted-foreground">
            {translate(`${T}source.${source}`, SOURCE_FALLBACK[source] ?? source)}
          </span>
        ) : null}
      </div>
      {question.reason ? <p className="text-xs text-muted-foreground">{question.reason}</p> : null}
      {control}
      {hasDefault && !readOnly && (untouched || defaultAccepted) ? (
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <span>{translate(`${T}suggested`, 'Suggested')}: {typeof question.suggestedDefault === 'string' ? question.suggestedDefault : JSON.stringify(question.suggestedDefault)}</span>
          <Button size="xs" variant={defaultAccepted ? 'secondary' : 'outline'} onClick={() => onAcceptDefault(!defaultAccepted)}>
            {defaultAccepted ? translate(`${T}editInstead`, 'Edit instead') : translate(`${T}useSuggested`, 'Use suggestion')}
          </Button>
        </div>
      ) : null}
      {message ? (
        <p id={errId} role="alert" className={cn('text-xs text-destructive')}>
          {message}
        </p>
      ) : null}
    </div>
  )
}
