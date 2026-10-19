import type { DevServerStatus } from '../../../shared/dev-server-types'
import { getSidebarHostHealthLabel } from '@/components/sidebar/sidebar-host-options'

/** Same wording the host selector uses for a dev server's connection state. */
export function getDevServerStatusLabel(status: DevServerStatus | undefined): string {
  switch (status) {
    case 'connected':
      return getSidebarHostHealthLabel('available')
    case 'connecting':
      return getSidebarHostHealthLabel('connecting')
    case 'error':
      return getSidebarHostHealthLabel('error')
    default:
      return getSidebarHostHealthLabel('disconnected')
  }
}
