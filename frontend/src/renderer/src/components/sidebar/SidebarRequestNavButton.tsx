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
  if (requestFlowSupport !== 'supported') return null

  const isActive = activeView === 'requests'
  const label = translate(
    'auto.components.request.SidebarRequestNavButton.label',
    'Requests'
  )

  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      aria-pressed={isActive}
      onClick={() => setActiveView('requests')}
      className={cn(
        'relative flex items-center justify-center size-8 rounded transition-colors',
        isActive
          ? 'bg-accent text-accent-foreground'
          : 'text-muted-foreground hover:text-foreground hover:bg-accent/60'
      )}
    >
      <Inbox className="size-4" aria-hidden />
      {pendingApprovalCount > 0 && (
        <span
          aria-label={translate(
            'auto.components.request.SidebarRequestNavButton.badge',
            `${pendingApprovalCount} pending approvals`
          )}
          className="absolute -top-1 -right-1 flex items-center justify-center min-w-[16px] h-4 px-0.5 rounded-full bg-destructive text-destructive-foreground text-[9px] font-bold leading-none"
        >
          {formatBadgeCount(pendingApprovalCount)}
        </span>
      )}
    </button>
  )
}
