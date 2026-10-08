/**
 * ClarificationQuestionList — FE-REQ-TASK-036-03
 *
 * @module components/request/clarification/ClarificationQuestionList
 */

import React from 'react'
import { ClarificationQuestionField } from './ClarificationQuestionField'
import type { Clarification } from '../../../../../shared/request-artifact-types'

type Props = {
  clarification: Clarification
  draft: ReadonlyMap<string, unknown>
  acceptedDefaults: ReadonlySet<string>
  onChange: (questionId: string, value: unknown) => void
  onAcceptDefault: (questionId: string, accepted: boolean) => void
  fieldErrors: Readonly<Record<string, string>>
  showErrors: boolean
  readOnly: boolean
  onSubmitShortcut: () => void
}

export function ClarificationQuestionList({
  clarification, draft, acceptedDefaults, onChange, onAcceptDefault, fieldErrors, showErrors, readOnly, onSubmitShortcut
}: Props): React.JSX.Element {
  return (
    <div className="flex flex-col gap-4">
      {[...clarification.questions].sort((a, b) => a.seq - b.seq).map((q) => (
        <ClarificationQuestionField
          key={q.id}
          question={q}
          source={clarification.source}
          value={draft.get(q.id) ?? q.answer}
          onChange={(v) => onChange(q.id, v)}
          defaultAccepted={acceptedDefaults.has(q.id)}
          onAcceptDefault={(a) => onAcceptDefault(q.id, a)}
          error={fieldErrors[q.id] ?? null}
          showErrors={showErrors}
          readOnly={readOnly}
          onSubmitShortcut={onSubmitShortcut}
        />
      ))}
    </div>
  )
}
