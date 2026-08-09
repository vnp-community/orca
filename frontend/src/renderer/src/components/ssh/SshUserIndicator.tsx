// TASK-FE-023: SshUserIndicator — shows SSH username and provisioning status.
import type { ProvisioningStatus } from '../../store/slices/ssh'
import { SshProvisioningProgress } from './SshProvisioningProgress'

type Props = {
  serverId: string
  linuxUsername: string
  provisioned: boolean
  provisioningStatus: ProvisioningStatus
}

export function SshUserIndicator({ linuxUsername, provisioningStatus }: Props) {
  return (
    <div className="ssh-user-indicator">
      <div className="ssh-user-indicator__header">
        <span className="ssh-user-indicator__icon">👤</span>
        <span className="ssh-user-indicator__username">{linuxUsername}</span>
        
        {provisioningStatus.phase === 'done' && (
          <span className="ssh-user-indicator__status-icon" title="Provisioned">✅</span>
        )}
      </div>

      {provisioningStatus.phase === 'checking' && (
        <div className="ssh-user-indicator__status-text">Checking...</div>
      )}

      {provisioningStatus.phase === 'provisioning' && (
        <SshProvisioningProgress 
          step={provisioningStatus.step} 
          progress={provisioningStatus.progress} 
        />
      )}

      {provisioningStatus.phase === 'error' && (
        <div className="ssh-user-indicator__error" role="alert">
          {provisioningStatus.message}
        </div>
      )}
    </div>
  )
}
