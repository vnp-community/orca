/**
 * Suggestion chips of the "revise the plan" panel (CR-REQ-045): a few starting sentences per engine. A chip only fills the
 * text box; it never sends.
 *
 * @module components/request/plan/revision/plan-revision-suggestions
 */

import { translate } from '@/i18n/i18n'
import type { RevisionSuggestion } from '../../solution/revision-suggestions'

function openspecSuggestions(): RevisionSuggestion[] {
  return [
    {
      id: 'tests',
      text: translate(
        'auto.components.request.plan.revision.suggestion.openspec.tests',
        'Add a task for tests.'
      )
    },
    {
      id: 'splitBiggest',
      text: translate(
        'auto.components.request.plan.revision.suggestion.openspec.splitBiggest',
        'Split the biggest task.'
      )
    },
    {
      id: 'tightenDesign',
      text: translate(
        'auto.components.request.plan.revision.suggestion.openspec.tightenDesign',
        'Tighten the design.'
      )
    },
    {
      id: 'edgeScenario',
      text: translate(
        'auto.components.request.plan.revision.suggestion.openspec.edgeScenario',
        'Add a scenario for the edge case.'
      )
    }
  ]
}

function nativeSuggestions(): RevisionSuggestion[] {
  return [
    {
      id: 'splitLargest',
      text: translate(
        'auto.components.request.plan.revision.suggestion.native.splitLargest',
        'Split the largest task.'
      )
    },
    {
      id: 'rollback',
      text: translate(
        'auto.components.request.plan.revision.suggestion.native.rollback',
        'Add a rollback task.'
      )
    },
    {
      id: 'riskyFirst',
      text: translate(
        'auto.components.request.plan.revision.suggestion.native.riskyFirst',
        'Reorder to ship the risky part first.'
      )
    },
    {
      id: 'mergeTiny',
      text: translate(
        'auto.components.request.plan.revision.suggestion.native.mergeTiny',
        'Merge tiny tasks.'
      )
    }
  ]
}

/** Chips for the engine that wrote the plan; an engine the client does not know is a native-style plan. */
export function planRevisionSuggestions(engine: string | undefined): RevisionSuggestion[] {
  return engine === 'openspec' ? openspecSuggestions() : nativeSuggestions()
}
