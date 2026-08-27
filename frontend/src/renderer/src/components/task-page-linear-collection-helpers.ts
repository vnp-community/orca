import type { LinearCollectionResult, LinearCustomViewModel } from '../../../shared/types'
import type { LinearDisplayProperty } from '@/components/task-page-localized-options'

// Why: split out of TaskPage.tsx (TASK-BIGFILE-241) — pure Linear browse-state
// constants/helpers with no React state of their own. Kept separate so both
// TaskPage.tsx (board/section rendering, which stays behind due to the
// linearTeamSelection circular dependency) and the new browse-state hooks can
// import them without TaskPage.tsx importing back from a hook file.

export const LINEAR_CUSTOM_VIEW_MODELS = [
  'issue',
  'project'
] satisfies readonly LinearCustomViewModel[]

export function mergeLinearCollectionResults<T>(
  results: LinearCollectionResult<T>[]
): LinearCollectionResult<T> {
  const errors = results.flatMap((result) => result.errors ?? [])
  return {
    items: results.flatMap((result) => result.items),
    ...(errors.length > 0 ? { errors } : {}),
    ...(results.some((result) => result.hasMore) ? { hasMore: true } : {})
  }
}

export const DEFAULT_LINEAR_DISPLAY_PROPERTIES: LinearDisplayProperty[] = [
  'state',
  'priority',
  'assignee',
  'team',
  'labels',
  'updated'
]
