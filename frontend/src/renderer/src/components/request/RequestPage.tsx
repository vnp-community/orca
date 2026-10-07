/**
 * RequestPage — CR-REQ-018-05
 *
 * Top-level page for request management. Contains three tabs:
 * - Requests (request list + detail pane)
 * - Approvals (pending approval queue)
 * - Backlog (request backlog)
 *
 * Shows RequestUnsupportedNotice when support === 'unsupported',
 * Skeleton when support === 'unknown'.
 *
 * @module components/request/RequestPage
 */

import React, { useCallback, useEffect } from 'react'
import { X } from 'lucide-react'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { Skeleton } from '@/components/ui/skeleton'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { useRequestFlowSupport } from '../../hooks/useRequestFlowSupport'
import { RequestUnsupportedNotice } from './RequestUnsupportedNotice'
import type { RequestPageData } from '../../store/slices/request'

function isInputElement(el: EventTarget | null): boolean {
  if (!el || !(el instanceof Element)) return false
  const tag = el.tagName.toLowerCase()
  return (
    tag === 'input' ||
    tag === 'textarea' ||
    tag === 'select' ||
    (el as HTMLElement).isContentEditable
  )
}

export default function RequestPage(): React.JSX.Element {
  // Probe runtime support; writes to store
  useRequestFlowSupport()

  const requestFlowSupport = useAppStore((s) => s.requestFlowSupport)
  const requestPage = useAppStore((s) => s.requestPage)
  const setRequestPageSection = useAppStore((s) => s.setRequestPageSection)
  const closeRequestPage = useCallback(() => {
    useAppStore.getState().setActiveView('terminal')
  }, [])

  // Escape to close (when focus is not in an input)
  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape' && !isInputElement(document.activeElement)) {
        closeRequestPage()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [closeRequestPage])

  const section = requestPage.section

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-border shrink-0">
        <h1 className="text-sm font-semibold text-foreground">
          {translate('auto.components.request.RequestPage.title', 'Requests')}
        </h1>
        <button
          onClick={closeRequestPage}
          aria-label="Close requests page"
          className="rounded p-1 text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
        >
          <X className="size-4" aria-hidden />
        </button>
      </div>

      {/* Body */}
      {requestFlowSupport === 'unknown' && (
        <div className="flex-1 p-4 flex flex-col gap-2">
          <Skeleton className="h-8 w-1/3" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-3/4" />
        </div>
      )}

      {requestFlowSupport === 'unsupported' && <RequestUnsupportedNotice />}

      {requestFlowSupport === 'supported' && (
        <Tabs
          value={section}
          onValueChange={(v) => setRequestPageSection(v as RequestPageData['section'])}
          className="flex-1 flex flex-col overflow-hidden"
        >
          <TabsList className="mx-4 mt-2 shrink-0">
            <TabsTrigger value="requests">
              {translate('auto.components.request.RequestPage.tab.requests', 'Requests')}
            </TabsTrigger>
            <TabsTrigger value="approvals">
              {translate('auto.components.request.RequestPage.tab.approvals', 'Approvals')}
            </TabsTrigger>
            <TabsTrigger value="backlog">
              {translate('auto.components.request.RequestPage.tab.backlog', 'Backlog')}
            </TabsTrigger>
          </TabsList>

          <TabsContent value="requests" className="flex-1 overflow-hidden mt-0">
            {/* Placeholder — filled by CR-019 */}
            <div data-testid="request-tab-requests" className="h-full" />
          </TabsContent>

          <TabsContent value="approvals" className="flex-1 overflow-hidden mt-0">
            {/* Placeholder — filled by CR-022 */}
            <div data-testid="request-tab-approvals" className="h-full" />
          </TabsContent>

          <TabsContent value="backlog" className="flex-1 overflow-hidden mt-0">
            {/* Placeholder — filled by CR-023 */}
            <div data-testid="request-tab-backlog" className="h-full" />
          </TabsContent>
        </Tabs>
      )}
    </div>
  )
}
