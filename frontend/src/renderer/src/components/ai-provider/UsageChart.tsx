// UsageChart.tsx — Token usage chart for AI provider account (TASK-V5-08)
import { useAppStore } from '../../store'
import { Progress } from '../ui/progress'
import { cn } from '../../lib/utils'

export function UsageChart({ accountId }: { accountId: string }) {
  const usage   = useAppStore(s => (s as any).aiUsageByAccount?.[accountId] ?? (s as any).usageByAccount?.[accountId])
  const account = useAppStore(s =>
    ((s as any).aiAccounts ?? (s as any).accounts ?? []).find((a: any) => a.id === accountId)
  )
  if (!usage || !account) {return <span className="text-xs text-muted-foreground">—</span>}

  const quotaLimit  = account.quotaLimitDay ?? 0
  const tokensUsed  = usage.tokens ?? 0
  const pct         = quotaLimit > 0 ? Math.min(100, (tokensUsed / quotaLimit) * 100) : 0
  const isWarning   = pct >= 80
  const isExceeded  = pct >= 100

  return (
    <div className="usage-chart min-w-[100px]" data-testid={`usage-chart-${accountId}`}>
      <div className="flex justify-between text-xs mb-1">
        <span>{tokensUsed.toLocaleString()}</span>
        <span className="text-muted-foreground">
          {quotaLimit > 0 ? `/ ${quotaLimit.toLocaleString()}` : 'unlimited'}
        </span>
      </div>
      {quotaLimit > 0 && (
        <Progress
          value={pct}
          className={cn('h-1.5', isExceeded ? 'bg-red-100' : isWarning ? 'bg-yellow-100' : 'bg-gray-100')}
        />
      )}
    </div>
  )
}
