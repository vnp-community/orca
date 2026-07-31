// WorkspaceSkeletonLoader.tsx — Loading skeleton while project initializes
export function WorkspaceSkeletonLoader() {
  return (
    <div
      className="workspace-skeleton p-4 space-y-3 h-full animate-pulse"
      data-testid="workspace-skeleton"
    >
      <div className="h-8 w-64 bg-muted rounded" />
      <div className="flex gap-3 h-[calc(100%-3rem)]">
        <div className="h-full w-1/4 bg-muted rounded" />
        <div className="h-full flex-1 bg-muted rounded" />
        <div className="h-full w-1/3 bg-muted rounded" />
      </div>
    </div>
  )
}
