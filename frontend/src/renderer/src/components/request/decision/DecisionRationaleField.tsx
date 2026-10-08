/**
 * DecisionRationaleField — FE-REQ-TASK-036-04
 *
 * @module components/request/decision/DecisionRationaleField
 */

import React, { useId, useState } from 'react'
import { translate } from '@/i18n/i18n'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { ShortcutKeyCombo } from '@/components/ShortcutKeyCombo'
import { getScreenSubmitModifierLabel, isScreenSubmitShortcut } from '@/lib/screen-submit-shortcut'
import { DECISION_RATIONALE_MIN_LENGTH, isRationaleValid } from './decision-rules'

const T = 'auto.components.request.decision.'

type Props = {
  value: string
  onChange: (value: string) => void
  required: boolean
  disabled?: boolean
  onSubmit?: () => void
}

export function DecisionRationaleField({ value, onChange, required, disabled, onSubmit }: Props): React.JSX.Element {
  const id = useId()
  const [touched, setTouched] = useState(false)
  const valid = isRationaleValid(value, required)
  const invalid = touched && !valid
  return (
    <div className="flex flex-col gap-1">
      <Label htmlFor={id} className="text-xs font-medium">
        {translate(`${T}rationaleLabel`, 'Reason for choosing')}
        {required ? <span aria-hidden className="ml-0.5 text-destructive">*</span> : null}
      </Label>
      <Textarea
        id={id}
        value={value}
        rows={3}
        disabled={disabled}
        aria-invalid={invalid ? true : undefined}
        aria-describedby={`${id}-hint`}
        onChange={(e) => onChange(e.target.value)}
        onBlur={() => setTouched(true)}
        onKeyDown={(e) => {
          if (onSubmit && isScreenSubmitShortcut(e)) {
            e.preventDefault()
            if (valid) {onSubmit()}
          }
        }}
      />
      <div id={`${id}-hint`} className="flex items-center justify-between text-[11px] text-muted-foreground">
        <span className={invalid ? 'text-destructive' : undefined}>
          {required
            ? translate(`${T}rationaleRequired`, 'A reason is required when choosing something other than the recommendation')
            : translate(`${T}rationaleOptional`, 'Optional')}
        </span>
        <span className="flex items-center gap-2">
          <span>{value.trim().length}/{DECISION_RATIONALE_MIN_LENGTH}</span>
          {onSubmit ? <ShortcutKeyCombo keys={[getScreenSubmitModifierLabel(), 'Enter']} /> : null}
        </span>
      </div>
    </div>
  )
}
