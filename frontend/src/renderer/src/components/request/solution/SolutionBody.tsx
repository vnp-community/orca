/**
 * SolutionBody — CR-REQ-020-02
 *
 * Routes a Solution to the view for its kind.
 *
 * @module components/request/solution/SolutionBody
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import type { RequestType, Solution } from '../../../../../shared/request-types'
import { AnswerView } from './AnswerView'
import { DiagnosisView } from './DiagnosisView'
import { FindingsView } from './FindingsView'
import { SolutionOptionCompare } from './SolutionOptionCompare'

export type SolutionBodyProps = {
  solution: Solution
  requestType: RequestType
  selectedId: string | null
  onSelect: (optionId: string) => void
  readOnly: boolean
  onRegenerate?: () => void
  regenerateDisabled?: boolean
}

export function SolutionBody(props: SolutionBodyProps): React.JSX.Element {
  const { solution } = props
  switch (solution.kind) {
    case 'solution':
      return (
        <SolutionOptionCompare
          solution={solution}
          requestType={props.requestType}
          selectedId={props.selectedId}
          onSelect={props.onSelect}
          readOnly={props.readOnly}
          onRegenerate={props.onRegenerate}
          regenerateDisabled={props.regenerateDisabled}
        />
      )
    case 'diagnosis':
      return <DiagnosisView solution={solution} />
    case 'findings':
      return <FindingsView solution={solution} />
    case 'answer':
      return <AnswerView solution={solution} />
    default:
      return (
        <p className="text-sm text-muted-foreground">
          {translate('auto.components.request.SolutionPanel.noData', 'No data')}
        </p>
      )
  }
}
