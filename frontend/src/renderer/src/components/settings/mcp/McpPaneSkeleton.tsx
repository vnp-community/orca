import { Skeleton } from '@/components/ui/skeleton'

export function McpPaneSkeleton(): React.JSX.Element {
  return (
    <div className="space-y-3" aria-busy="true">
      <Skeleton className="h-9 w-64" />
      <Skeleton className="h-24 w-full" />
      <Skeleton className="h-24 w-full" />
    </div>
  )
}
