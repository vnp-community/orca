/**
 * DiagnosisView — CR-REQ-020-02
 *
 * @module components/request/solution/DiagnosisView
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import type { Solution } from '../../../../../shared/request-types'
import { RequestMarkdownContent } from './RequestMarkdownContent'
import { readContentSections } from './solution-content-sections'
import { SolutionSection } from './SolutionSection'

const SPECS = [
  { name: 'rootCause', aliases: ['rootCause', 'root_cause', 'cause'] },
  { name: 'impact', aliases: ['impact', 'affectedScope', 'affected_scope', 'scope'] },
  { name: 'baseline', aliases: ['baseline', 'measuredBaseline', 'measured_baseline'] }
]

export function DiagnosisView({ solution }: { solution: Solution }): React.JSX.Element {
  const { sections, fallbackText } = readContentSections(solution.content, SPECS)
  return (
    <div data-testid="diagnosis-view" className="space-y-4">
      {fallbackText !== null ? (
        <RequestMarkdownContent content={fallbackText} />
      ) : (
        <>
          <SolutionSection
            title={translate('auto.components.request.DiagnosisView.rootCause', 'Root cause')}
            lines={sections.rootCause}
          />
          <SolutionSection
            title={translate('auto.components.request.DiagnosisView.impact', 'Affected scope')}
            lines={sections.impact}
          />
          <SolutionSection
            title={translate('auto.components.request.DiagnosisView.baseline', 'Measured baseline')}
            lines={sections.baseline}
          />
        </>
      )}
    </div>
  )
}
