/**
 * Opens Settings > Servers (dev servers) — shared by graph/impact/readiness
 * "connect dev server" actions. Reuses the navigation the sidebar already uses.
 *
 * @module components/request/use-open-dev-server-settings
 */

import { useCallback } from 'react'
import { useAppStore } from '@/store'

export function useOpenDevServerSettings(): () => void {
  const openSettingsPage = useAppStore((s) => s.openSettingsPage)
  const openSettingsTarget = useAppStore((s) => s.openSettingsTarget)
  return useCallback(() => {
    openSettingsTarget({ pane: 'servers', repoId: null })
    openSettingsPage()
  }, [openSettingsPage, openSettingsTarget])
}
