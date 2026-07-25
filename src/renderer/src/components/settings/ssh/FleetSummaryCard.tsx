// FleetSummaryCard — stat card for fleet health summary (CR-005, TASK-005-C)
import { cn } from '@/lib/utils'

type CardVariant = 'default' | 'success' | 'warning' | 'destructive'

type FleetSummaryCardProps = {
  label: string
  value: number
  variant?: CardVariant
}

const VARIANT_BORDER: Record<CardVariant, string> = {
  default: 'bg-muted/50',
  success: 'bg-green-500/10 border-green-500/20',
  warning: 'bg-yellow-500/10 border-yellow-500/20',
  destructive: 'bg-destructive/10 border-destructive/20'
}

const VARIANT_VALUE: Record<CardVariant, string> = {
  default: 'text-foreground',
  success: 'text-green-600 dark:text-green-400',
  warning: 'text-yellow-600 dark:text-yellow-400',
  destructive: 'text-destructive'
}

export function FleetSummaryCard({
  label,
  value,
  variant = 'default'
}: FleetSummaryCardProps): React.JSX.Element {
  return (
    <div className={cn('space-y-1 rounded-lg border p-3', VARIANT_BORDER[variant])}>
      <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
        {label}
      </p>
      <p className={cn('text-2xl font-bold tabular-nums', VARIANT_VALUE[variant])}>
        {value}
      </p>
    </div>
  )
}
