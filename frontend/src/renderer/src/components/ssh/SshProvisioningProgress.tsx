// TASK-FE-023: SshProvisioningProgress — progress bar for SSH user provisioning.
export function SshProvisioningProgress({ step, progress }: { step: string; progress: number }) {
  return (
    <div className="ssh-provisioning-progress">
      <div className="ssh-provisioning-progress__step">{step}</div>
      <div 
        className="ssh-provisioning-progress__bar"
        role="progressbar"
        aria-valuenow={progress}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        <div 
          className="ssh-provisioning-progress__fill"
          style={{ width: `${progress}%` }} 
        />
      </div>
    </div>
  )
}
