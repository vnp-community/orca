/**
 * BacklogEngineBadge — CR-REQ-023-06
 *
 * Label for the execution engine string the backend reports. Unlike the Task
 * page's ExecutionEngineBadge it does not infer anything on the client.
 *
 * @module components/request/backlog/BacklogEngineBadge
 */

import React from 'react'
import { Badge } from '@/components/ui/badge'
import { translate } from '@/i18n/i18n'

const ENGINES: Record<string, string> = {
  workflow: 'Workflow',
  orchestration: 'Orchestration',
  direct_agent: 'Direct agent'
}

export function BacklogEngineBadge({ engine }: { engine: string | undefined }): React.JSX.Element {
  if (!engine || !ENGINES[engine]) {return <span className="text-muted-foreground">-</span>}
  return (
    <Badge variant="outline">
      {translate(`auto.components.request.backlog.BacklogEngineBadge.${engine}`, ENGINES[engine])}
    </Badge>
  )
}
