// src/shared/fleet-types.ts
// Shared types for fleet status reporting — used by backend (main) and frontend (renderer/CLI).
import type { SshConnectionStatus } from './ssh-types'

export type FleetServerStatus = {
  id: string               // fleetId or targetId
  label: string
  host: string
  project?: string
  team?: string
  environment?: string
  status: SshConnectionStatus
  error: string | null
  uptimeSeconds: number
  uptimePercent24h: number
  relayVersion: string | null
  lastSeenAt: number | null
  reconnectAttempt: number
  // FIX BUG-BE-HLD-010: real resource metrics, surfaced from HealthRecord.
  cpuPercent: number | null
  ramPercent: number | null
  diskPercent: number | null
  pingLatencyMs: number | null
}

export type FleetStatusReport = {
  generatedAt: number
  servers: FleetServerStatus[]
  summary: {
    total: number
    connected: number
    disconnected: number
    error: number
    healthScore: number    // 0–100 (connected / total × 100)
  }
}

// ── Bootstrap Types ────────────────────────────────────────────

export type BootstrapStepName =
  | 'node-check'
  | 'node-install'
  | 'git-check'
  | 'disk-check'    // FIX BUG-BE-HLD-013
  | 'packages'
  | 'relay-deploy'  // FIX BUG-BE-HLD-013 — install + SHA256 verify + start, gộp từ ssh-relay-deploy.ts
  | 'repo-clone'
  | 'setup-script'
  | 'verify'

export type BootstrapStep = {
  step: BootstrapStepName
  status: 'running' | 'ok' | 'error' | 'skipped'
  message?: string
  error?: string
}

export type BootstrapResult = {
  targetId: string
  steps: BootstrapStep[]
  success: boolean
  error?: string
}
