import React, { useId, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { translate } from '@/i18n/i18n'
import { validateBaseRefSyntax } from '../../../../shared/request-base-ref'
import { baseRefViolationMessage } from './base-ref-violation-message'

const T = 'auto.components.request.StartFromField.'

type Props = {
  onSubmit: (ref: string) => void
}

/** Typed branch name for a host that cannot list branches; the same syntax rules as the backend say what is wrong. */
export function StartFromManualEntry({ onSubmit }: Props): React.JSX.Element {
  const [value, setValue] = useState('')
  const [problem, setProblem] = useState<string | null>(null)
  const inputId = useId()
  const problemId = `${inputId}-problem`

  const submit = (): void => {
    const checked = validateBaseRefSyntax(value)
    if (!checked.ok) {
      setProblem(baseRefViolationMessage(checked))
      return
    }
    if (checked.value === '') {
      setProblem(baseRefViolationMessage({ ok: false, reason: 'blank' }))
      return
    }
    setProblem(null)
    onSubmit(checked.value)
  }

  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={inputId}>{translate(`${T}manualLabel`, 'Branch or tag name')}</Label>
      <div className="flex items-start gap-2">
        <Input
          id={inputId}
          value={value}
          autoComplete="off"
          spellCheck={false}
          aria-invalid={problem !== null}
          aria-describedby={problem ? problemId : undefined}
          placeholder={translate(`${T}manualPlaceholder`, 'e.g. release/1.2')}
          onChange={(e) => {
            setValue(e.target.value)
            setProblem(null)
          }}
          onKeyDown={(e) => {
            if (e.nativeEvent.isComposing) {
              return
            }
            if (e.key === 'Enter') {
              // Why: Enter must not submit the dialog form that holds this field.
              e.preventDefault()
              submit()
            } else if (e.key === 'Escape' && value !== '') {
              // Esc first clears the text; only an empty field lets the popover close.
              e.preventDefault()
              e.stopPropagation()
              setValue('')
              setProblem(null)
            }
          }}
        />
        <Button type="button" size="sm" variant="outline" onClick={submit}>
          {translate(`${T}manualUse`, 'Use this branch')}
        </Button>
      </div>
      <p id={problemId} role="alert" className="text-xs text-destructive">
        {problem}
      </p>
    </div>
  )
}
