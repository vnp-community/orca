/**
 * Suggestion chips of the "revise the analysis" panel (CR-REQ-041): a few starting sentences per kind of document, so a
 * reviewer who does not know what to ask for can click one and edit it. A chip only fills the text box; it never sends.
 *
 * @module components/request/solution/revision-suggestions
 */

import { translate } from '@/i18n/i18n'
import type { SolutionKind } from '../../../../../shared/request-types'

export type RevisionSuggestion = { id: string; text: string }

function solutionSuggestions(): RevisionSuggestion[] {
  return [
    {
      id: 'lowerRisk',
      text: translate(
        'auto.components.request.AnalysisRevision.suggestion.solution.lowerRisk',
        'Add a lower-risk alternative.'
      )
    },
    {
      id: 'compareCost',
      text: translate(
        'auto.components.request.AnalysisRevision.suggestion.solution.compareCost',
        'Compare the options on cost and effort.'
      )
    },
    {
      id: 'tradeOffs',
      text: translate(
        'auto.components.request.AnalysisRevision.suggestion.solution.tradeOffs',
        'Explain the trade-offs of the recommended option.'
      )
    },
    {
      id: 'rollback',
      text: translate(
        'auto.components.request.AnalysisRevision.suggestion.solution.rollback',
        'Cover the rollback plan.'
      )
    }
  ]
}

function diagnosisSuggestions(): RevisionSuggestion[] {
  return [
    {
      id: 'evidence',
      text: translate(
        'auto.components.request.AnalysisRevision.suggestion.diagnosis.evidence',
        'Show the evidence for the root cause.'
      )
    },
    {
      id: 'otherCauses',
      text: translate(
        'auto.components.request.AnalysisRevision.suggestion.diagnosis.otherCauses',
        'Consider other possible causes.'
      )
    },
    {
      id: 'reproduce',
      text: translate(
        'auto.components.request.AnalysisRevision.suggestion.diagnosis.reproduce',
        'Explain how to reproduce it.'
      )
    },
    {
      id: 'prevent',
      text: translate(
        'auto.components.request.AnalysisRevision.suggestion.diagnosis.prevent',
        'Suggest how to prevent it.'
      )
    }
  ]
}

function findingsSuggestions(): RevisionSuggestion[] {
  return [
    {
      id: 'moreAreas',
      text: translate(
        'auto.components.request.AnalysisRevision.suggestion.findings.moreAreas',
        'Cover the files and modules I did not mention.'
      )
    },
    {
      id: 'rankByRisk',
      text: translate(
        'auto.components.request.AnalysisRevision.suggestion.findings.rankByRisk',
        'Rank the findings by risk.'
      )
    },
    {
      id: 'evidence',
      text: translate(
        'auto.components.request.AnalysisRevision.suggestion.findings.evidence',
        'Add evidence for each finding.'
      )
    },
    {
      id: 'dropIrrelevant',
      text: translate(
        'auto.components.request.AnalysisRevision.suggestion.findings.dropIrrelevant',
        'Drop findings that are not relevant.'
      )
    }
  ]
}

function answerSuggestions(): RevisionSuggestion[] {
  return [
    {
      id: 'concise',
      text: translate(
        'auto.components.request.AnalysisRevision.suggestion.answer.concise',
        'Be more concise.'
      )
    },
    {
      id: 'examples',
      text: translate(
        'auto.components.request.AnalysisRevision.suggestion.answer.examples',
        'Add concrete examples.'
      )
    },
    {
      id: 'cite',
      text: translate(
        'auto.components.request.AnalysisRevision.suggestion.answer.cite',
        'Cite where in the code this comes from.'
      )
    },
    {
      id: 'uncertainty',
      text: translate(
        'auto.components.request.AnalysisRevision.suggestion.answer.uncertainty',
        'State what you are unsure about.'
      )
    }
  ]
}

/** Chips for one kind of document; a kind this client does not know gets none rather than a guess. */
export function revisionSuggestions(kind: SolutionKind): RevisionSuggestion[] {
  switch (kind) {
    case 'solution':
      return solutionSuggestions()
    case 'diagnosis':
      return diagnosisSuggestions()
    case 'findings':
      return findingsSuggestions()
    case 'answer':
      return answerSuggestions()
    default:
      return []
  }
}
