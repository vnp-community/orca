/**
 * TaskBacklogTable — CR-REQ-023-06
 *
 * Tasks whose Plan/Phase is not approved yet, grouped by Plan.
 *
 * @module components/request/backlog/TaskBacklogTable
 */

import React from 'react'
import { BacklogGroupTable, type BacklogGroupTableProps } from './BacklogGroupTable'

export function TaskBacklogTable(props: Omit<BacklogGroupTableProps, 'view'>): React.JSX.Element {
  return <BacklogGroupTable view="tasks" {...props} />
}
