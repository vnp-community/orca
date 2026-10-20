/**
 * PlanRevisionPanel — CR-REQ-045
 *
 * At the Plan gate a reviewer types what should change and the AI writes the next version of the plan, without rejecting the
 * one on screen. The text lives in `usePlanRevision`, so it survives a failed send and a re-render of the panel.
 *
 * @module components/request/plan/revision/PlanRevisionPanel
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import { RevisionPromptBox } from '../../solution/RevisionPromptBox'
import { planRevisionSuggestions } from './plan-revision-suggestions'
import type { PlanRevisionApi } from './usePlanRevision'

type Props = {
  revision: PlanRevisionApi
  /** The engine that wrote the plan; picks the suggestion chips. */
  engine: string | undefined
}

export function PlanRevisionPanel({ revision, engine }: Props): React.JSX.Element {
  const { busy, draft, error, nextVersion, setDraft, submit, slow } = revision
  const busyText =
    nextVersion !== undefined
      ? translate('auto.components.request.plan.revision.busy', 'Writing version {{n}}…', {
          n: nextVersion
        })
      : translate('auto.components.request.plan.revision.busyNext', 'Writing the next version…')

  return (
    <div className="flex flex-col gap-2">
      <RevisionPromptBox
        testIdPrefix="plan-revision"
        draft={draft}
        setDraft={setDraft}
        onSubmit={() => void submit()}
        busy={busy}
        busyText={busyText}
        error={error}
        suggestions={planRevisionSuggestions(engine)}
        labels={{
          title: translate(
            'auto.components.request.plan.revision.title',
            'Request changes to this plan'
          ),
          description: translate(
            'auto.components.request.plan.revision.hint',
            'The AI revises the documents. You approve the new version afterwards.'
          ),
          fieldLabel: translate(
            'auto.components.request.plan.revision.fieldLabel',
            'What should change?'
          ),
          placeholder: translate(
            'auto.components.request.plan.revision.placeholder',
            'For example: split the migration into its own task and add a rollback step.'
          ),
          submit: translate('auto.components.request.plan.revision.submit', 'Revise plan')
        }}
      />
      {busy && slow && (
        <p role="status" className="text-xs text-muted-foreground" data-testid="plan-revision-slow">
          {translate(
            'auto.components.request.plan.revision.slow',
            'Taking longer than expected. You can keep waiting; if it fails you can try again.'
          )}
        </p>
      )}
    </div>
  )
}
