/**
 * AnalysisRevisionPanel — CR-REQ-041
 *
 * At the Analysis gate a reviewer types what should change and the AI writes the next version, without rejecting the
 * one on screen. The text lives in `useAnalysisRevision`, so it survives a failed send and a re-render of the panel.
 *
 * @module components/request/solution/AnalysisRevisionPanel
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import type { SolutionKind } from '../../../../../shared/request-types'
import { RevisionPromptBox } from './RevisionPromptBox'
import { revisionSuggestions } from './revision-suggestions'
import type { AnalysisRevision } from './useAnalysisRevision'

export type AnalysisRevisionPanelProps = {
  revision: AnalysisRevision
  kind: SolutionKind
}

export function AnalysisRevisionPanel({
  revision,
  kind
}: AnalysisRevisionPanelProps): React.JSX.Element {
  const { busy, draft, error, nextVersion, setDraft, submit } = revision
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
    <RevisionPromptBox
      testIdPrefix="analysis-revision"
      draft={draft}
      setDraft={setDraft}
      onSubmit={() => void submit()}
      busy={busy}
      busyText={busyText}
      error={error}
      suggestions={revisionSuggestions(kind)}
      labels={{
        title: translate(
          'auto.components.request.AnalysisRevision.title',
          'Request changes to this analysis'
        ),
        description: translate(
          'auto.components.request.AnalysisRevision.hint',
          'Describe what should change. The AI writes a new version and this one stays in the history.'
        ),
        fieldLabel: translate(
          'auto.components.request.AnalysisRevision.fieldLabel',
          'What should change?'
        ),
        placeholder: translate(
          'auto.components.request.AnalysisRevision.placeholder',
          'For example: compare the options on cost and add a rollback plan.'
        ),
        submit: translate('auto.components.request.AnalysisRevision.submit', 'Revise analysis')
      }}
    />
  )
}
