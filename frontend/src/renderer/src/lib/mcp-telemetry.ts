import type { McpRisk } from '../../../shared/mcp-types'
import type { EventProps } from '../../../shared/telemetry-events'
import { track } from './telemetry'

// Components call these wrappers, never track() directly, so payloads stay coarse by construction.
// Web build: telemetryTrack is a no-op, so these are inert there.

export type McpTelemetryTab = EventProps<'mcp_settings_opened'>['tab']
export type McpCountBucket = '1' | '2' | '3+'
export type McpLifetimeBucket = '<=7d' | '<=30d' | '<=90d' | '>90d'
export type McpLatencyBucket = '<10s' | '<60s' | '<10m' | '>=10m'

export const bucketScopeCount = (n: number): McpCountBucket => (n <= 1 ? '1' : n === 2 ? '2' : '3+')

export function bucketLifetimeDays(days: number): McpLifetimeBucket {
  return days <= 7 ? '<=7d' : days <= 30 ? '<=30d' : days <= 90 ? '<=90d' : '>90d'
}

export function bucketLatencyMs(ms: number): McpLatencyBucket {
  return ms < 10_000 ? '<10s' : ms < 60_000 ? '<60s' : ms < 600_000 ? '<10m' : '>=10m'
}

export function trackMcpConsentDecided(a: {
  decision: 'approve' | 'deny'
  scopeCount: number
  newClient: boolean
  viaDcr: boolean
  narrowed: boolean
}): void {
  track('mcp_consent_decided', {
    decision: a.decision,
    scope_count: bucketScopeCount(a.scopeCount),
    new_client: a.newClient,
    via_dcr: a.viaDcr,
    narrowed: a.narrowed
  })
}

export function trackMcpTokenCreated(a: { lifetimeDays: number; scopeCount: number }): void {
  track('mcp_token_created', {
    lifetime_bucket: bucketLifetimeDays(a.lifetimeDays),
    scope_count: bucketScopeCount(a.scopeCount)
  })
}

export function trackMcpTokenRevoked(): void {
  track('mcp_token_revoked', {})
}

export function trackMcpApprovalDecided(a: {
  decision: 'approve' | 'deny'
  risk: McpRisk
  via: 'dialog' | 'inbox' | 'deeplink'
  latencyMs: number
}): void {
  track('mcp_approval_decided', {
    decision: a.decision,
    risk: a.risk,
    via: a.via,
    latency_bucket: bucketLatencyMs(a.latencyMs)
  })
}

export function trackMcpSettingsOpened(a: { tab: McpTelemetryTab; role: 'admin' | 'user' }): void {
  track('mcp_settings_opened', { tab: a.tab, role: a.role })
}

export function trackMcpKillSwitchToggled(a: {
  scope: 'tenant' | 'client' | 'grant' | 'session'
  active: boolean
}): void {
  track('mcp_killswitch_toggled', { scope: a.scope, active: a.active })
}
