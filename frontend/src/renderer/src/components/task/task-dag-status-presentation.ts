import { Ban, Circle, CircleCheck, CircleSlash, Eye, Loader } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

export type TaskDagStatusPresentation = {
  Icon: LucideIcon
  nodeClass: string
  labelKey: string
  labelFallback: string
}

const PREFIX = 'auto.components.task.TaskDAGView.status.'

const BASE = 'rounded-lg border-2 text-foreground'

const TABLE: Record<string, TaskDagStatusPresentation> = {
  done: {
    Icon: CircleCheck,
    nodeClass: `${BASE} bg-status-success-background border-status-success-border`,
    labelKey: `${PREFIX}done`,
    labelFallback: 'Done'
  },
  in_progress: {
    Icon: Loader,
    nodeClass: `${BASE} bg-primary/10 border-primary`,
    labelKey: `${PREFIX}in_progress`,
    labelFallback: 'In progress'
  },
  blocked: {
    Icon: Ban,
    nodeClass: `${BASE} bg-destructive/10 border-destructive`,
    labelKey: `${PREFIX}blocked`,
    labelFallback: 'Blocked'
  },
  review: {
    Icon: Eye,
    nodeClass: `${BASE} bg-ai-action-accent/10 border-ai-action-accent`,
    labelKey: `${PREFIX}review`,
    labelFallback: 'In review'
  },
  open: {
    Icon: Circle,
    nodeClass: `${BASE} bg-muted border-border`,
    labelKey: `${PREFIX}open`,
    labelFallback: 'Open'
  },
  cancelled: {
    Icon: CircleSlash,
    nodeClass: `${BASE} bg-muted border-border text-muted-foreground`,
    labelKey: `${PREFIX}cancelled`,
    labelFallback: 'Cancelled'
  }
}

export function getTaskDagStatusPresentation(status: string): TaskDagStatusPresentation {
  if (status === 'todo') {return TABLE.open}
  return Object.prototype.hasOwnProperty.call(TABLE, status) ? TABLE[status] : TABLE.open
}
