/**
 * BacklogGroupTable — CR-REQ-023-06
 *
 * Shared body of the Task and Execute backlog tables: group header rows (Plan
 * or Phase) followed by their tasks, j/k/Enter over the task rows.
 *
 * @module components/request/backlog/BacklogGroupTable
 */

import React, { useCallback, useMemo, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import { useAppStore } from '@/store'
import { useRowListKeyboardNavigation } from '@/hooks/useRowListKeyboardNavigation'
import { useRequestSummaries } from '../../../hooks/useRequestSummaries'
import { openRequestPage } from '../request-page-navigation'
import { BacklogGroupHeaderRow } from './BacklogGroupHeaderRow'
import { BacklogTaskSheet } from './BacklogTaskSheet'
import { BACKLOG_COLUMNS } from './backlog-view-columns'
import { ExecuteBacklogRow } from './ExecuteBacklogRow'
import { TaskBacklogRow } from './TaskBacklogRow'
import type { BacklogGroupData, BacklogTaskRowData } from '../../../../../shared/request-backlog-types'

export type BacklogGroupTableProps = {
  view: 'tasks' | 'execute'
  groups: BacklogGroupData[]
  hasMore: boolean
  isLoadingMore: boolean
  dimmed?: boolean
  onLoadMore: () => void
  /** The task may have changed while its sheet was open. */
  onTaskSheetClosed: () => void
}

export function BacklogGroupTable({
  view, groups, hasMore, isLoadingMore, dimmed, onLoadMore, onTaskSheetClosed
}: BacklogGroupTableProps): React.JSX.Element {
  const tasks = useAppStore((s) => s.tasks)
  const [sheetTaskId, setSheetTaskId] = useState<string | null>(null)
  const summaries = useRequestSummaries(useMemo(() => groups.map((g) => g.requestId), [groups]))

  const flat = useMemo(() => groups.flatMap((g) => g.tasks), [groups])
  const { containerProps, getRowProps, activeKey } = useRowListKeyboardNavigation({
    items: flat, getKey: (t: BacklogTaskRowData) => t.taskId, onOpen: (t) => setSheetTaskId(t.taskId), containerRole: 'grid'
  })

  const resolveTaskLabel = useCallback(
    (id: string) => tasks.find((t) => t.id === id)?.title ?? id.slice(0, 8),
    [tasks]
  )
  const closeSheet = useCallback(() => {
    setSheetTaskId(null)
    onTaskSheetClosed()
  }, [onTaskSheetClosed])

  const columns = BACKLOG_COLUMNS[view]
  const Row = view === 'tasks' ? TaskBacklogRow : ExecuteBacklogRow
  return (
    <div className={cn('flex-1 overflow-auto focus-visible:outline-none', dimmed && 'opacity-60')} data-testid={`${view}-backlog-table`} {...containerProps}>
      <Table>
        <TableHeader className="sticky top-0 z-10 bg-background">
          <TableRow>
            {columns.map((c) => (
              <TableHead key={c.id} scope="col">{translate(c.labelKey, c.fallback)}</TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {groups.map((group, index) => (
            <React.Fragment key={`${group.requestId}:${group.phaseTaskId ?? group.planTaskId ?? index}`}>
              <BacklogGroupHeaderRow
                group={group}
                view={view}
                colSpan={columns.length}
                request={summaries.byId[group.requestId]}
                onOpenPlan={(requestId) => openRequestPage({ section: 'requests', requestId, focus: 'plan' })}
              />
              {group.tasks.map((task) => (
                <Row
                  key={task.taskId}
                  task={task}
                  active={activeKey === task.taskId}
                  rowProps={getRowProps(task.taskId)}
                  resolveTaskLabel={resolveTaskLabel}
                  onOpenTask={setSheetTaskId}
                />
              ))}
            </React.Fragment>
          ))}
        </TableBody>
      </Table>
      {hasMore && (
        <div className="flex justify-center p-3">
          <Button variant="outline" size="sm" disabled={isLoadingMore} onClick={onLoadMore}>
            {translate('auto.components.request.backlog.BacklogTab.loadMore', 'Load more')}
          </Button>
        </div>
      )}
      <BacklogTaskSheet taskId={sheetTaskId} onClose={closeSheet} />
    </div>
  )
}
