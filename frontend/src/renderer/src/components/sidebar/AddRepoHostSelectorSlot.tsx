import { useState } from 'react'
import { AddRepoHostSelector } from './AddRepoHostSelector'
import type { useAddRepoHostSelection } from './use-add-repo-host-selection'
import { AddRemoteHostDialog, type AddRemoteHostMode } from './AddRemoteHostDialog'
import { isWebClientLocation } from '@/lib/web-client-location'
import { useAppStore } from '@/store'

export function AddRepoHostSelectorSlot({
  hostSelection
}: {
  hostSelection: ReturnType<typeof useAddRepoHostSelection>
}) {
  const [addRemoteHostMode, setAddRemoteHostMode] = useState<AddRemoteHostMode | null>(null)
  const isWeb = isWebClientLocation()
  const openSettingsPage = useAppStore((s) => s.openSettingsPage)
  const openSettingsTarget = useAppStore((s) => s.openSettingsTarget)

  // In web mode: "Add remote host" opens Settings > Dev Servers pane
  // instead of showing the SSH/Orca-server sub-menu — web clients connect
  // via Dev Servers (daemon agents), not SSH or paired Orca sessions.
  const handleAddRemoteInWeb = () => {
    openSettingsTarget({ pane: 'servers', repoId: null })
    openSettingsPage()
  }

  return (
    <>
      <AddRepoHostSelector
        hosts={hostSelection.hostOptions}
        selectedHostId={hostSelection.selectedHostId}
        open={hostSelection.hostSelectorOpen}
        onOpenChange={hostSelection.setHostSelectorOpen}
        onSelectHost={(hostId) => void hostSelection.handleSelectAddProjectHost(hostId)}
        onConnectHost={(hostId) => void hostSelection.handleConnectAddProjectHost(hostId)}
        // Web mode: no SSH host option — agents connect via Dev Servers
        onAddSshHost={isWeb ? undefined : () => setAddRemoteHostMode('ssh')}
        onAddRemoteServer={isWeb ? handleAddRemoteInWeb : () => setAddRemoteHostMode('server')}
      />
      {!isWeb && (
        <AddRemoteHostDialog mode={addRemoteHostMode} onOpenChange={setAddRemoteHostMode} />
      )}
    </>
  )
}
