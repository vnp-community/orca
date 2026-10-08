/**
 * FindingsView — CR-REQ-020-02
 *
 * @module components/request/solution/FindingsView
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import type { Solution } from '../../../../../shared/request-types'
import { RequestMarkdownContent } from './RequestMarkdownContent'
import { readContentSections } from './solution-content-sections'
import { SolutionSection } from './SolutionSection'

const SPECS = [
  { name: 'question', aliases: ['question', 'researchQuestion', 'research_question'] },
  { name: 'findings', aliases: ['findings', 'results'] },
  { name: 'sources', aliases: ['sources', 'references'] },
  { name: 'conclusion', aliases: ['conclusion', 'summary'] }
]

export function FindingsView({ solution }: { solution: Solution }): React.JSX.Element {
  const { sections, fallbackText } = readContentSections(solution.content, SPECS)
  return (
    <div data-testid="findings-view" className="space-y-4">
      {fallbackText !== null ? (
        <RequestMarkdownContent content={fallbackText} />
      ) : (
        <>
          <SolutionSection
            title={translate('auto.components.request.FindingsView.question', 'Question')}
            lines={sections.question}
          />
          <SolutionSection
            title={translate('auto.components.request.FindingsView.findings', 'Findings')}
            lines={sections.findings}
            asList
          />
          <SolutionSection
            title={translate('auto.components.request.FindingsView.sources', 'Sources')}
            lines={sections.sources}
            asList
          />
          <SolutionSection
            title={translate('auto.components.request.FindingsView.conclusion', 'Conclusion')}
            lines={sections.conclusion}
          />
        </>
      )}
    </div>
  )
}
