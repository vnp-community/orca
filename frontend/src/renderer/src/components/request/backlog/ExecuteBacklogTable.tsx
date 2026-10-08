/**
 * ExecuteBacklogTable — CR-REQ-023-06
 *
 * Approved tasks that have not started or have failed, grouped by Phase.
 *
 * @module components/request/backlog/ExecuteBacklogTable
 */

import React from 'react'
import { BacklogGroupTable, type BacklogGroupTableProps } from './BacklogGroupTable'

export function ExecuteBacklogTable(props: Omit<BacklogGroupTableProps, 'view'>): React.JSX.Element {
  return <BacklogGroupTable view="execute" {...props} />
}
