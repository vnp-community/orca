/**
 * The second browser tab of the monitoring spec: Settings > Dev Servers, which keeps polling `devServer.list` by itself, read
 * together with the gateway's own `devServer.list` row so a mismatch between service and screen is visible too.
 */
import type { BrowserContext, Page } from '@playwright/test'
import { ConsoleAudit } from './live-console-audit'
import type { LiveFlow } from './live-flow-harness'
import type { DevServerSample } from './live-agent-session-judge'
import { openDevServersPane, readDevServerCard } from './live-agent-session-ui'

export type DevServerReading = DevServerSample & { platform: string }

export class DevServerTab {
  private page: Page | null = null
  private audit: ConsoleAudit | null = null

  constructor(
    private readonly f: LiveFlow,
    private readonly context: BrowserContext
  ) {}

  get isOpen(): boolean {
    return this.page !== null
  }

  async open(): Promise<void> {
    this.page = await this.context.newPage()
    this.page.on('dialog', (d) => void d.accept().catch(() => {}))
    this.audit = new ConsoleAudit(this.page, () => 'dev-server monitor tab')
    await openDevServersPane(this.page)
  }

  async sample(): Promise<DevServerReading> {
    const card = await readDevServerCard(this.page!, this.f.cfg.devServerName)
    const rows = await this.f.rpc.call<Record<string, unknown>[]>('devServer.list', {})
    const row = (rows ?? []).find((s) => s.name === this.f.cfg.devServerName)
    // Why: only fields that describe the server's state; ids and timestamps would make every sample differ.
    const api = JSON.stringify({
      status: row?.status,
      healthStatus: row?.healthStatus,
      busy: row?.busy,
      activeExecutions: row?.activeExecutions,
      lastError: row?.lastError ?? null
    })
    return { ui: card.badge, busy: card.busy, api, platform: card.platform }
  }

  /** Both tabs share the SPA's stored last view: leave Settings first or the main page comes back on Settings after a reload. */
  async close(): Promise<void> {
    if (!this.page) {
      return
    }
    await this.page
      .getByRole('button', { name: 'Back to app' })
      .click({ timeout: 5_000 })
      .catch(() => {})
    if (this.audit) {
      this.f.audit.entries.push(...this.audit.entries)
    }
    await this.page.close().catch(() => {})
    this.page = null
  }
}
