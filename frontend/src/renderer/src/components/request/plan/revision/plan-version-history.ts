/**
 * Version history of a Request's plan (CR-REQ-045): the one-line note under a version, the badge, and how a version's size and
 * cost read. Pure; the analysis list shares the note length (solution-version-history.ts), not its types.
 *
 * @module components/request/plan/revision/plan-version-history
 */

import { translate } from '@/i18n/i18n'
import { versionNote } from '../../solution/solution-version-history'
import type {
  PlanRevision,
  PlanRevisionStats,
  PlanRevisionStatus
} from '../../../../../../shared/request-plan-revision'

/** The note under a version: what was asked for, cut to one line; the first version has no request, a later one may have none. */
export function planVersionNote(
  revision: Pick<PlanRevision, 'revision' | 'revisionFeedback'>
): string {
  const note = versionNote(revision.revisionFeedback)
  if (note !== null) {
    return note
  }
  return revision.revision <= 1
    ? translate('auto.components.request.plan.revision.firstProposal', 'First proposal')
    : translate('auto.components.request.plan.revision.revised', 'Revised')
}

export function planVersionBadge(revision: Pick<PlanRevision, 'status'>): PlanRevisionStatus {
  return revision.status
}

/** "3 files changed"; none for a version that kept no file list (a native plan, or a Request older than the history). */
export function filesChangedLabel(
  revision: Pick<PlanRevision, 'filesChanged' | 'fileCount'>
): string | null {
  if (revision.fileCount <= 0) {
    return null
  }
  return translate(
    'auto.components.request.plan.revision.filesChanged',
    '{{count}} files changed',
    { count: revision.filesChanged }
  )
}

function formatDuration(ms: number): string {
  const seconds = Math.round(ms / 1000)
  if (seconds < 60) {
    return `${seconds}s`
  }
  const minutes = Math.floor(seconds / 60)
  return `${minutes}m ${seconds % 60}s`
}

/** "1m 30s, $1.25"; a cost the run did not report is left out rather than shown as free. */
export function versionStatsLabel(stats: PlanRevisionStats | undefined): string | null {
  if (!stats) {
    return null
  }
  const parts: string[] = []
  if (stats.durationMs !== undefined) {
    parts.push(formatDuration(stats.durationMs))
  }
  if (stats.costUsd !== undefined) {
    parts.push(`$${stats.costUsd.toFixed(2)}`)
  }
  return parts.length > 0 ? parts.join(', ') : null
}
