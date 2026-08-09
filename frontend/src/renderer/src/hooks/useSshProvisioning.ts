// TASK-FE-024: useSshProvisioning
import { useEffect } from 'react'
import { useAppStore } from '../store'

type SshProvisioningEvent = {
  serverId: string
  step: string
  progress: number         // 0–100
  linuxUsername?: string   // populated khi progress=100
}

export function useSshProvisioning(serverId: string): void {
  const updateProvisioningStatus = useAppStore(s => s.updateProvisioningStatus)

  useEffect(() => {
    // Handler cho WS events từ runtime
    function handleEvent(event: SshProvisioningEvent) {
      if (event.serverId !== serverId) {return}

      if (event.progress < 100) {
        updateProvisioningStatus(serverId, {
          phase: 'provisioning',
          step: event.step,
          progress: event.progress
        })
      } else {
        updateProvisioningStatus(serverId, {
          phase: 'done',
          linuxUsername: event.linuxUsername!
        })
      }
    }

    // Subscribe: Desktop mode qua window.api.ssh.onProvisionProgress
    // Web mode: sẽ được thiết lập qua sync-runtime-graph
    // NOTE: actual event subscription cần adapt theo platform
    // @ts-ignore
    const off = window.api?.ssh?.onProvisionProgress?.(handleEvent)
    return () => { off?.() }
  }, [serverId, updateProvisioningStatus])
}
