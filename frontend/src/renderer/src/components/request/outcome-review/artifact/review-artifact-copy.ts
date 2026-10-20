/**
 * Words of the drawings of a review — FE-OR-T211..T215
 *
 * @module components/request/outcome-review/artifact/review-artifact-copy
 */

import { translate } from '@/i18n/i18n'
import type { RegenerationReason, RegenerationScope } from '../../../../hooks/reviewArtifactCalls'

const T = 'auto.components.request.outcomeReview.artifact.'

export function notDrawnText(
  kind: 'missing' | 'failed' | 'drawing' | 'disabled' | 'unreadable' | 'gone'
): string {
  switch (kind) {
    case 'missing':
      return translate(`${T}missing`, 'No diagram for this change')
    case 'failed':
      return translate(`${T}failed`, 'Drawing failed')
    case 'drawing':
      return translate(`${T}drawing`, 'Drawing…')
    case 'disabled':
      return translate(`${T}disabled`, 'Drawing is turned off, showing the table')
    case 'gone':
      return translate(`${T}gone`, 'This drawing was replaced. Reload the review.')
    default:
      return translate(`${T}unreadable`, 'The drawing could not be loaded')
  }
}

export function drawnByAgentText(): string {
  return translate(`${T}drawnByAgent`, 'Drawn by an agent from the model above. It may be wrong.')
}

export function actionText(
  id:
    | 'retry'
    | 'regenerate'
    | 'revisions'
    | 'compare'
    | 'openLarger'
    | 'zoomIn'
    | 'zoomOut'
    | 'restore'
    | 'close'
): string {
  switch (id) {
    case 'retry':
      return translate(`${T}action.retry`, 'Try again')
    case 'regenerate':
      return translate(`${T}action.regenerate`, 'Regenerate…')
    case 'revisions':
      return translate(`${T}action.revisions`, 'Revisions')
    case 'compare':
      return translate(`${T}action.compare`, 'Compare')
    case 'openLarger':
      return translate(`${T}action.openLarger`, 'Open larger')
    case 'zoomIn':
      return translate(`${T}action.zoomIn`, 'Zoom in')
    case 'zoomOut':
      return translate(`${T}action.zoomOut`, 'Zoom out')
    case 'restore':
      return translate(`${T}action.restore`, 'Use this one')
    default:
      return translate(`${T}action.close`, 'Close')
  }
}

export function originText(origin: string): string {
  switch (origin) {
    case 'auto':
      return translate(`${T}origin.auto`, 'drawn automatically')
    case 'regenerated':
      return translate(`${T}origin.regenerated`, 'regenerated')
    case 'restored':
      return translate(`${T}origin.restored`, 'restored')
    default:
      return translate(`${T}origin.unknown`, 'unknown origin')
  }
}

export function infoText(
  meta: { diagramType: string; skillVersion: string; revision: number },
  when: string,
  origin: string
): string {
  return translate(
    `${T}info`,
    '{{type}} · skill {{skill}} · revision {{revision}} · {{when}} · {{origin}}',
    {
      type: meta.diagramType || translate(`${T}typeUnknown`, 'diagram'),
      skill: meta.skillVersion || translate(`${T}skillUnknown`, 'unknown'),
      revision: meta.revision,
      when,
      origin: originText(origin)
    }
  )
}

export function scopeText(scope: RegenerationScope): string {
  switch (scope) {
    case 'artifact':
      return translate(`${T}scope.artifact`, 'Draw the diagrams again')
    case 'context':
      return translate(`${T}scope.context`, 'Read the design and the code again')
    default:
      return translate(`${T}scope.all`, 'Read again and draw again')
  }
}

export function reasonText(reason: RegenerationReason): string {
  switch (reason) {
    case 'looks_wrong':
      return translate(`${T}reason.looksWrong`, 'It looks wrong')
    case 'kg_changed':
      return translate(`${T}reason.kgChanged`, 'The knowledge graph changed')
    case 'code_changed':
      return translate(`${T}reason.codeChanged`, 'The code changed')
    case 'style_changed':
      return translate(`${T}reason.styleChanged`, 'The drawing style changed')
    case 'retry_failed':
      return translate(`${T}reason.retryFailed`, 'A drawing failed')
    default:
      return translate(`${T}reason.other`, 'Another reason')
  }
}

export function estimateText(est: {
  agentRuns: number
  diagramsToDraw: number
  costKnown: boolean
  costUsd?: number
  lastContextCost?: string
  lastDrawCost?: string
}): string[] {
  const lines = [
    translate(`${T}estimate.runs`, '{{runs}} agent run(s), {{diagrams}} diagram(s) to draw', {
      runs: est.agentRuns,
      diagrams: est.diagramsToDraw
    })
  ]
  if (est.costKnown && est.costUsd !== undefined) {
    lines.push(
      translate(`${T}estimate.cost`, 'About {{usd}} USD, from the last runs', {
        usd: est.costUsd.toFixed(2)
      })
    )
  } else {
    lines.push(translate(`${T}estimate.noCost`, 'No cost figures have been measured yet'))
  }
  return lines
}

export function compareTitleText(): string {
  return translate(`${T}compare.title`, 'Compare two revisions')
}

export type ModelChangeId =
  | 'addedNodes'
  | 'removedNodes'
  | 'changedNodes'
  | 'addedEdges'
  | 'removedEdges'
  | 'changedEdges'

export function modelChangeText(id: ModelChangeId): string {
  switch (id) {
    case 'addedNodes':
      return translate(`${T}compare.addedNodes`, 'Boxes added')
    case 'removedNodes':
      return translate(`${T}compare.removedNodes`, 'Boxes removed')
    case 'changedNodes':
      return translate(`${T}compare.changedNodes`, 'Boxes changed')
    case 'addedEdges':
      return translate(`${T}compare.addedEdges`, 'Arrows added')
    case 'removedEdges':
      return translate(`${T}compare.removedEdges`, 'Arrows removed')
    default:
      return translate(`${T}compare.changedEdges`, 'Arrows changed')
  }
}

export function noModelChangeText(): string {
  return translate(`${T}compare.none`, 'The two revisions are drawn from the same model.')
}
