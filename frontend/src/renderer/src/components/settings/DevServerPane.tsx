// ─── DevServerPane ─────────────────────────────────────────────────────────────
// Settings pane for Dev Server management — visible in both Web mode and Desktop
// mode. In Web mode this is the primary entry point to connect the relay that
// routes preflight.check, gh auth login, and workspace operations to the actual
// developer machine (CR-GH-001 / CR-GH-003).
import { Server } from 'lucide-react'
import { DevServerList } from '../dev-server/DevServerList'

export function DevServerPane() {
  return (
    <div className="dev-server-pane">
      <div className="dev-server-pane__header">
        <div className="dev-server-pane__title-row">
          <Server className="dev-server-pane__icon" aria-hidden="true" />
          <h2 className="dev-server-pane__title">Dev Servers</h2>
        </div>
        <p className="dev-server-pane__description">
          Connect remote developer machines so Orca agents run on your actual dev environment.
          GitHub, GitLab and Git integrations will be checked against the connected dev server.
        </p>
      </div>
      <DevServerList />
    </div>
  )
}
