import type { StepStatus } from '@shared/workflow-types'
import { CheckCircle2, Loader2, Clock, XCircle, SkipForward } from 'lucide-react'
import { cn } from '../../lib/utils'
import type { ReactNode } from 'react'

const STEP_STATUS: Record<StepStatus, { icon: ReactNode; className: string; label: string }> = {
  pending:   { icon: <Clock      size={14} />, className: 'text-gray-400',  label: 'Pending'   },
  running:   { icon: <Loader2    size={14} className="animate-spin" />, className: 'text-blue-500',  label: 'Running'   },
  completed: { icon: <CheckCircle2 size={14} />, className: 'text-green-500', label: 'Completed' },
  failed:    { icon: <XCircle    size={14} />, className: 'text-red-500',   label: 'Failed'    },
  skipped:   { icon: <SkipForward size={14} />, className: 'text-gray-400', label: 'Skipped'   },
}

export function StepStatusBadge({ status }: { status: StepStatus }) {
  const { icon, className, label } = STEP_STATUS[status]
  return (
    <span className={cn('flex items-center gap-1 text-xs font-medium', className)}>
      {icon} {label}
    </span>
  )
}
