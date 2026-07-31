// HealthStatusBadge.tsx — Status indicator for AI provider account health (TASK-V5-07)
import type { ReactNode } from 'react'
import { CheckCircle, Clock, XCircle, AlertTriangle, WifiOff } from 'lucide-react'
import type { AIProviderStatus } from '../../types/ai-provider-types'
import { cn } from '../../lib/utils'

const STATUS_CONFIG: Record<AIProviderStatus, { label: string; color: string; icon: ReactNode }> = {
  active:         { label: 'Active',        color: 'text-green-600',  icon: <CheckCircle   size={12} /> },
  pending:        { label: 'Pending',        color: 'text-yellow-600', icon: <Clock         size={12} /> },
  invalid:        { label: 'Invalid Key',    color: 'text-red-600',    icon: <XCircle       size={12} /> },
  quota_exceeded: { label: 'Quota Exceeded', color: 'text-orange-600', icon: <AlertTriangle size={12} /> },
  unreachable:    { label: 'Unreachable',    color: 'text-gray-500',   icon: <WifiOff       size={12} /> },
}

export function HealthStatusBadge({ status }: { status: AIProviderStatus }) {
  const cfg = STATUS_CONFIG[status]
  if (!cfg) return null
  const { label, color, icon } = cfg
  return (
    <span
      className={cn('flex items-center gap-1 text-xs font-medium', color)}
      data-testid={`status-badge-${status}`}
    >
      {icon} {label}
    </span>
  )
}
