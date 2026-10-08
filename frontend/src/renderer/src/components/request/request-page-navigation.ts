/**
 * request-page-navigation — CR-REQ-022-06
 *
 * Deep-link into a Request from the approval inbox and backlog screens.
 *
 * @module components/request/request-page-navigation
 */

import { useAppStore } from '@/store'
import type { RequestPageData } from '../../store/slices/request'

export type OpenRequestPageTarget = {
  section: RequestPageData['section']
  requestId?: string
  focus?: RequestPageData['focus']
}

export function openRequestPage(target: OpenRequestPageTarget): void {
  const store = useAppStore.getState()
  // Why: the board and other pages call this too, so make sure the Requests page is the one showing.
  store.setActiveView('requests')
  store.setRequestPageData({
    section: target.section,
    ...(target.requestId !== undefined ? { requestId: target.requestId } : {}),
    focus: target.focus
  })
}
