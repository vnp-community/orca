// src/main/ssh/fleet-health-monitor.ts
// Periodically polls SSH connection states and records health snapshots.
// Also emits renderer events and optional Slack/webhook alerts on error transitions.
import { BrowserWindow } from 'electron'
import { fleetHealthStore } from './fleet-health-store'

const DEFAULT_PING_INTERVAL_MS = 60_000 // 1 minute

export type FleetAlert = {
  targetId: string
  label: string
  project?: string
  status: string
  error: string | null
}

export class FleetHealthMonitor {
  private intervalId: NodeJS.Timeout | null = null
  private lastAlertedStatus: Map<string, string> = new Map()

  // Dependency injection via properties — set externally after construction
  getSshTargets: (() => Array<{ id: string; label: string; project?: string }> | Promise<Array<{ id: string; label: string; project?: string }>>) | null = null
  getConnectionState:
    | ((targetId: string) => { status: string; error?: string | null; remotePlatform?: unknown } | null)
    | null = null
  getWebhookUrl: (() => string | undefined) | null = null

  /** Start the periodic health check loop. No-op if already running. */
  start(intervalMs = DEFAULT_PING_INTERVAL_MS): void {
    if (this.intervalId) return
    this.intervalId = setInterval(() => {
      this.runHealthCheck().catch((err) => {
        console.error('[fleet-monitor] Health check error:', err)
      })
    }, intervalMs)
  }

  /** Stop the periodic health check loop. */
  stop(): void {
    if (this.intervalId) {
      clearInterval(this.intervalId)
      this.intervalId = null
    }
  }

  /** Run a single health check cycle — poll all targets and record states. */
  async runHealthCheck(): Promise<void> {
    if (!this.getSshTargets || !this.getConnectionState) return

    const targets = await this.getSshTargets()
    for (const target of targets) {
      const state = this.getConnectionState(target.id)
      const status = (state?.status ?? 'disconnected') as import('../../shared/ssh-types').SshConnectionStatus

      // Record snapshot in health store
      fleetHealthStore.recordConnectionState({
        targetId: target.id,
        status,
        error: state?.error ?? null,
        reconnectAttempt: 0,
        remotePlatform: state?.remotePlatform as import('../../shared/ssh-types').SshRemotePlatform | undefined,
      })

      // Alert on error-state transitions (not repeated spam)
      const isErrorState = status === 'error' || status === 'reconnection-failed' || status === 'auth-failed'
      const prevStatus = this.lastAlertedStatus.get(target.id)

      if (isErrorState && prevStatus !== status) {
        this.lastAlertedStatus.set(target.id, status)
        this.emitAlert({
          targetId: target.id,
          label: target.label,
          project: target.project,
          status,
          error: state?.error ?? null,
        })
      } else if (!isErrorState) {
        this.lastAlertedStatus.delete(target.id)
      }
    }
  }

  private emitAlert(alert: FleetAlert): void {
    // Notify all renderer windows
    for (const win of BrowserWindow.getAllWindows()) {
      if (!win.isDestroyed()) {
        win.webContents.send('fleet:serverAlert', alert)
      }
    }
    // Send webhook if configured
    const webhookUrl = this.getWebhookUrl?.()
    if (webhookUrl) {
      this.sendWebhookAlert(webhookUrl, alert).catch((err) => {
        console.error('[fleet-monitor] Webhook error:', err)
      })
    }
  }

  private async sendWebhookAlert(url: string, alert: FleetAlert): Promise<void> {
    const payload = {
      text: `⚠️ Orca Fleet Alert: *${alert.label}* (${alert.project ?? 'no project'}) → \`${alert.status}\``,
      attachments: [
        {
          color: 'danger',
          fields: [
            { title: 'Server', value: alert.label, short: true },
            { title: 'Status', value: alert.status, short: true },
            { title: 'Error', value: alert.error ?? 'No details', short: false },
          ],
        },
      ],
    }
    await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
  }
}

// Process-lifetime singleton
export const fleetHealthMonitor = new FleetHealthMonitor()
