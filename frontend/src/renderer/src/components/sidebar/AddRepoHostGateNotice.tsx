import { Server } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import type { AddRepoHostGate } from './add-repo-web-host-gate'

function unavailableMessage(
  gate: Extract<AddRepoHostGate, { status: 'host-unavailable' }>
): string {
  if (gate.reason === 'connecting') {
    return translate(
      'auto.components.sidebar.AddRepoHostGateNotice.reconnecting',
      '{{host}} is reconnecting. Your choice is kept, and Add Project resumes when it is back online.',
      { host: gate.label }
    )
  }
  if (gate.reason === 'removed') {
    return translate(
      'auto.components.sidebar.AddRepoHostGateNotice.removed',
      '{{host}} is no longer available. Choose another dev server.',
      { host: gate.label }
    )
  }
  return translate(
    'auto.components.sidebar.AddRepoHostGateNotice.offline',
    '{{host}} is offline. Your choice is kept; Add Project resumes when it reconnects, or choose another dev server.',
    { host: gate.label }
  )
}

/**
 * Web only: explains why Add Project is blocked until a dev server is connected and chosen.
 * Renders nothing when the gate is ready (always true on desktop).
 */
export function AddRepoHostGateNotice({
  gate
}: {
  gate: AddRepoHostGate
}): React.JSX.Element | null {
  const closeModal = useAppStore((s) => s.closeModal)
  const openSettingsPage = useAppStore((s) => s.openSettingsPage)
  const openSettingsTarget = useAppStore((s) => s.openSettingsTarget)
  if (gate.status === 'ready') {
    return null
  }
  const openDevServers = (): void => {
    closeModal()
    openSettingsTarget({ pane: 'dev-servers', repoId: null })
    openSettingsPage()
  }
  return (
    <div
      role={gate.status === 'choose-host' ? 'status' : 'alert'}
      data-testid="add-repo-host-gate"
      data-gate-status={gate.status}
      className="flex items-start gap-2 rounded-md border border-border bg-muted px-3 py-2 text-xs text-muted-foreground"
    >
      <Server className="mt-0.5 size-3.5 shrink-0" />
      <div className="min-w-0 flex-1 space-y-1.5">
        {gate.status === 'no-host' ? (
          <>
            <p className="font-medium text-foreground">
              {translate(
                'auto.components.sidebar.AddRepoHostGateNotice.noHostTitle',
                'No dev server is connected'
              )}
            </p>
            <p>
              {translate(
                'auto.components.sidebar.AddRepoHostGateNotice.noHostBody',
                'Projects live on a dev server. Connect one, then come back to add a project.'
              )}
            </p>
            <Button type="button" size="xs" variant="outline" onClick={openDevServers}>
              {translate(
                'auto.components.sidebar.AddRepoHostGateNotice.openDevServers',
                'Open Dev Servers'
              )}
            </Button>
          </>
        ) : gate.status === 'choose-host' ? (
          <p>
            {translate(
              'auto.components.sidebar.AddRepoHostGateNotice.chooseHostBody',
              'Choose the dev server this project lives on. Browse and Add unlock once you pick one.'
            )}
          </p>
        ) : (
          <p>{unavailableMessage(gate)}</p>
        )}
      </div>
    </div>
  )
}
