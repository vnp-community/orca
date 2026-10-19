/**
 * SidebarRequestNavButton — CR-REQ-018-05
 *
 * Sidebar button for the Requests page. Hides when requestFlowSupport !== 'supported'.
 * Shows pending approval count badge (capped at 99).
 *
 * @module components/sidebar/SidebarRequestNavButton
 */

import React from 'react'
import { Inbox } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'

function formatBadgeCount(n: number): string {
  return n > 99 ? '99+' : String(n)
}

export function SidebarRequestNavButton(): React.JSX.Element | null {
  const requestFlowSupport = useAppStore((s) => s.requestFlowSupport)
  const pendingApprovalCount = useAppStore((s) => s.pendingApprovalCount)
  const activeView = useAppStore((s) => s.activeView)
  const setActiveView = useAppStore((s) => s.setActiveView)

  // Only visible when runtime supports the request-service
  if (requestFlowSupport !== 'supported') {
    return null
  }

  const isActive = activeView === 'requests'
  const label = translate('auto.components.request.SidebarRequestNavButton.label', 'Requests')
  const badgeLabel = translate(
    'auto.components.request.SidebarRequestNavButton.badge',
    '{{count}} pending approvals',
    // Why: the catalog string carries the count; without it screen readers lost the number.
    { count: pendingApprovalCount }
  )
  const accessibleName = pendingApprovalCount > 0 ? `${label}, ${badgeLabel}` : label

  return (
    <button
      type="button"
      title={accessibleName}
      aria-label={accessibleName}
      aria-current={isActive ? 'page' : undefined}
      data-testid="sidebar-requests"
      onClick={() => setActiveView('requests')}
      // Why: same row anatomy as Automations/Agents so the entry reads as a labelled nav item.
      className={cn(
        'flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[13px] font-medium tracking-tight transition-colors',
        isActive
          ? 'bg-worktree-sidebar-accent text-worktree-sidebar-accent-foreground'
          : 'text-worktree-sidebar-foreground/60 hover:bg-worktree-sidebar-foreground/8'
      )}
    >
      <Inbox
        className={cn('size-4 shrink-0', !isActive && 'text-worktree-sidebar-foreground/30')}
        strokeWidth={isActive ? 2.25 : 1.75}
        aria-hidden
      />
      <span className="min-w-0 flex-1 truncate">{label}</span>
      {pendingApprovalCount > 0 && (
        <span
          data-testid="sidebar-requests-badge"
          className="shrink-0 rounded-full bg-primary px-1.5 py-px text-[10px] font-semibold text-primary-foreground"
        >
          {formatBadgeCount(pendingApprovalCount)}
        </span>
      )}
    </button>
  )
}
