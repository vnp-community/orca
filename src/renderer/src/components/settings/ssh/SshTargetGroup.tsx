// SshTargetGroup — collapsible group header with connected-count badge (CR-002)
import { ChevronRight } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import type { SshTarget, SshConnectionState } from '../../../../../shared/ssh-types'

type SshTargetGroupProps = {
  /** Display label for this group. */
  label: string
  /** SSH targets belonging to this group. */
  targets: SshTarget[]
  /** Connection states keyed by target ID (from global store). */
  connectionStates: Map<string, SshConnectionState>
  /** Whether this group is currently collapsed (content hidden). */
  isCollapsed: boolean
  /** Called when the user clicks the group header to toggle collapsed state. */
  onToggleCollapse: () => void
  /** Render function for each target row. Allows the parent to inject
   *  the correct row component (SshTargetCard or SshTargetRow). */
  renderTarget: (target: SshTarget, state: SshConnectionState | undefined) => React.ReactNode
}

export function SshTargetGroup({
  label,
  targets,
  connectionStates,
  isCollapsed,
  onToggleCollapse,
  renderTarget
}: SshTargetGroupProps): React.JSX.Element {
  const connectedCount = targets.filter(
    (t) => connectionStates.get(t.id)?.status === 'connected'
  ).length

  const allConnected = connectedCount === targets.length && targets.length > 0
  const someConnected = connectedCount > 0 && !allConnected

  return (
    <Collapsible open={!isCollapsed} onOpenChange={() => onToggleCollapse()}>
      {/* ── Group header ── */}
      <CollapsibleTrigger asChild>
        <button
          type="button"
          className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left transition-colors hover:bg-muted/50"
        >
          <ChevronRight
            className={cn(
              'size-4 flex-shrink-0 text-muted-foreground transition-transform duration-150',
              !isCollapsed && 'rotate-90'
            )}
          />
          <span className="flex-1 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            {label}
          </span>
          {/* Connected/total badge */}
          <Badge
            variant="secondary"
            className={cn('ml-auto text-xs tabular-nums', {
              'bg-green-500/10 text-green-600 dark:text-green-400': allConnected,
              'bg-yellow-500/10 text-yellow-600 dark:text-yellow-400': someConnected
            })}
          >
            {connectedCount}/{targets.length}
          </Badge>
        </button>
      </CollapsibleTrigger>

      {/* ── Group items ── */}
      <CollapsibleContent>
        <div className="ml-4 mt-0.5 space-y-0.5 border-l pl-3">
          {targets.map((target) =>
            renderTarget(target, connectionStates.get(target.id))
          )}
        </div>
      </CollapsibleContent>
    </Collapsible>
  )
}
