// src/main/ssh/fleet-health-monitor.ts
// Periodically polls SSH connection states and records health snapshots.
// Also emits renderer events and optional Slack/webhook alerts on error transitions.
// FIX TASK-FLEET-001: Removed direct BrowserWindow dependency — use optional onAlert callback instead.
// This makes FleetHealthMonitor usable in server mode (ORCA_MULTI_USER=1) without crashing.
import { fleetHealthStore } from './fleet-health-store'
import { collectRemoteResourceMetrics } from './fleet-remote-commands'
import { execCommand } from './ssh-relay-deploy-helpers'
import type { SshConnection } from './ssh-connection'

const DEFAULT_PING_INTERVAL_MS = 30_000 // FIX BUG-BE-HLD-010: was 60_000, doc yêu cầu 30s

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
  getSshTargets: (() => { id: string; label: string; project?: string }[] | Promise<{ id: string; label: string; project?: string }[]>) | null = null
  getConnectionState:
    | ((targetId: string) => { status: string; error?: string | null; remotePlatform?: unknown } | null)
    | null = null
  // FIX BUG-BE-HLD-010: exposes the live SshConnection so runHealthCheck() can
  // exec real CPU/RAM/disk probes. null when a target has no live connection
  // (e.g. disconnected) — health check degrades to connection-state-only.
  getSshConnection: ((targetId: string) => SshConnection | undefined) | null = null
  getWebhookUrl: (() => string | undefined) | null = null
  // FIX TASK-FLEET-001: Optional alert callback instead of BrowserWindow.
  // In Electron mode: wire this to BrowserWindow.getAllWindows().forEach(w => w.webContents.send(...))
  // In server mode: leave undefined (no renderer to notify)
  onAlert: ((alert: FleetAlert) => void) | null = null

  /** Start the periodic health check loop. No-op if already running. */
  start(intervalMs = DEFAULT_PING_INTERVAL_MS): void {
    if (this.intervalId) {return}
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
    if (!this.getSshTargets || !this.getConnectionState) {return}

    const targets = await this.getSshTargets()

    // FIX BUG-BE-HLD-010: parallelized so per-target metric probes (~1s each,
    // see collectRemoteResourceMetrics) don't multiply the cycle time linearly
    // with fleet size. Each target only touches its own map keys, so
    // concurrent execution is safe.
    await Promise.allSettled(targets.map((target) => this.checkOneTarget(target)))
  }

  private async checkOneTarget(target: { id: string; label: string; project?: string }): Promise<void> {
    const state = this.getConnectionState!(target.id)
    const status = (state?.status ?? 'disconnected') as import('../../shared/ssh-types').SshConnectionStatus

    // FIX BUG-BE-HLD-010: real metrics via SSH exec, only when connected —
    // no live connection means no channel to exec on.
    let pingLatencyMs: number | undefined
    let resourceMetrics: { cpuPercent: number | null; ramPercent: number | null; diskPercent: number | null } = {
      cpuPercent: null,
      ramPercent: null,
      diskPercent: null,
    }
    if (status === 'connected' && this.getSshConnection) {
      const conn = this.getSshConnection(target.id)
      if (conn) {
        try {
          const start = Date.now()
          await execCommand(conn, 'true', { timeoutMs: 5_000 })
          pingLatencyMs = Date.now() - start
        } catch {
          // Leave pingLatencyMs undefined — exec failed despite 'connected'
          // state (stale state); the next connection-state poll will correct it.
        }
        resourceMetrics = await collectRemoteResourceMetrics(conn)
      }
    }

    // Record snapshot in health store
    fleetHealthStore.recordConnectionState(
      {
        targetId: target.id,
        status,
        error: state?.error ?? null,
        reconnectAttempt: 0,
        remotePlatform: state?.remotePlatform as import('../../shared/ssh-types').SshRemotePlatform | undefined,
      },
      undefined,
      { pingLatencyMs, ...resourceMetrics }
    )

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

  private emitAlert(alert: FleetAlert): void {
    // FIX TASK-FLEET-001: Use injected callback instead of BrowserWindow.getAllWindows().
    // Electron mode: caller sets onAlert = (a) => BrowserWindow.getAllWindows().forEach(w => w.webContents.send('fleet:serverAlert', a))
    // Server mode: onAlert is null → only webhook fires
    if (this.onAlert) {
      this.onAlert(alert)
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
