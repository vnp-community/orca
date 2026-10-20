/**
 * usePlanVersions — CR-REQ-045
 *
 * The versions of the plan and which one the reader looks at. The current one is followed as it moves on; an older one is
 * kept until the reader leaves it, even when a newer version appears meanwhile.
 *
 * @module components/request/plan/revision/usePlanVersions
 */

import { useCallback, useMemo, useState } from 'react'
import { usePlanRevisions, type PlanRevisionsApi } from '../../../../hooks/usePlanRevisions'
import type { PlanRevision } from '../../../../../../shared/request-plan-revision'

type Chosen = { requestId: string; revision: number; currentWhenChosen: number }

export type PlanVersionsApi = {
  list: PlanRevisionsApi
  /** The older version the reader chose; null while the current one is on screen. */
  shown: PlanRevision | null
  viewingOlder: boolean
  /** A newer current version appeared after the reader chose an older one. */
  newerAvailable: boolean
  select: (revision: number) => void
  backToCurrent: () => void
}

export function usePlanVersions(request: Parameters<typeof usePlanRevisions>[0]): PlanVersionsApi {
  const list = usePlanRevisions(request)
  const [chosen, setChosen] = useState<Chosen | null>(null)
  const { current, revisions } = list

  const mine = chosen?.requestId === request.id ? chosen : null
  const shown = useMemo(
    () =>
      mine && current && mine.revision !== current.revision
        ? (revisions.find((r) => r.revision === mine.revision) ?? null)
        : null,
    [mine, current, revisions]
  )

  const select = useCallback(
    (revision: number): void => {
      if (!current || revision === current.revision) {
        setChosen(null)
        return
      }
      setChosen({ requestId: request.id, revision, currentWhenChosen: current.revision })
    },
    [current, request.id]
  )
  const backToCurrent = useCallback(() => setChosen(null), [])

  return {
    list,
    shown,
    viewingOlder: shown !== null,
    newerAvailable:
      shown !== null && !!mine && !!current && current.revision > mine.currentWhenChosen,
    select,
    backToCurrent
  }
}
