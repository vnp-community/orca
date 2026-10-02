import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'

export function McpListSkeleton({ rows = 3 }: { rows?: number }): React.JSX.Element {
  return (
    <div
      className="space-y-2"
      aria-busy="true"
      aria-label={translate('auto.mcp.common.loading', 'Loading…')}
    >
      {Array.from({ length: rows }, (_, i) => (
        <Skeleton key={i} className="h-9 w-full" />
      ))}
    </div>
  )
}

export function McpInlineAlert({
  message,
  onRetry
}: {
  message: string
  onRetry?: () => void
}): React.JSX.Element {
  return (
    <div
      role="alert"
      className="flex items-center justify-between gap-3 rounded-md border border-border px-3 py-2 text-sm"
    >
      <span className="text-destructive">{message}</span>
      {onRetry ? (
        <Button variant="ghost" size="sm" onClick={onRetry}>
          {translate('auto.mcp.pane.retry', 'Retry')}
        </Button>
      ) : null}
    </div>
  )
}

export function McpMutedNote({ children }: { children: React.ReactNode }): React.JSX.Element {
  return <p className="text-sm text-muted-foreground">{children}</p>
}
