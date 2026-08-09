// src/main/runtime/rpc/fleet-metrics-handler.ts
// Prometheus-compatible /metrics HTTP handler for fleet health monitoring.
// Optional feature: only active when fleetMetricsEnabled=true in global settings.
import type * as http from 'node:http'
import type { FleetStatusReport } from '../../../../shared/fleet-types'

/**
 * Create a Prometheus /metrics request handler.
 *
 * @param getReport - Async function that produces a FleetStatusReport on demand
 * @returns HTTP request handler function
 *
 * Usage: Wire into the RPC HTTP server's request handler before WebSocket upgrade:
 * ```typescript
 * if (req.url === '/metrics' && globalSettings?.fleetMetricsEnabled) {
 *   await metricsHandler(req, res)
 *   return
 * }
 * ```
 */
export function createFleetMetricsHandler(
  getReport: () => Promise<FleetStatusReport>
): (req: http.IncomingMessage, res: http.ServerResponse) => Promise<void> {
  return async function handleMetricsRequest(
    req: http.IncomingMessage,
    res: http.ServerResponse
  ): Promise<void> {
    if (req.url !== '/metrics' || req.method !== 'GET') return

    try {
      const report = await getReport()
      const lines: string[] = []

      // ── orca_server_connected ──────────────────────────────────
      lines.push('# HELP orca_server_connected Whether Orca relay is connected (1=yes, 0=no)')
      lines.push('# TYPE orca_server_connected gauge')
      for (const s of report.servers) {
        const labels = `server="${s.id}",project="${s.project ?? ''}",team="${s.team ?? ''}",env="${s.environment ?? ''}"`
        lines.push(`orca_server_connected{${labels}} ${s.status === 'connected' ? 1 : 0}`)
      }

      // ── orca_server_uptime_seconds ─────────────────────────────
      lines.push('')
      lines.push('# HELP orca_server_uptime_seconds Continuous uptime in seconds since last connect')
      lines.push('# TYPE orca_server_uptime_seconds gauge')
      for (const s of report.servers) {
        lines.push(`orca_server_uptime_seconds{server="${s.id}"} ${s.uptimeSeconds}`)
      }

      // ── orca_server_uptime_24h_percent ─────────────────────────
      lines.push('')
      lines.push('# HELP orca_server_uptime_24h_percent Uptime percentage over the last 24 hours (0–100)')
      lines.push('# TYPE orca_server_uptime_24h_percent gauge')
      for (const s of report.servers) {
        lines.push(`orca_server_uptime_24h_percent{server="${s.id}"} ${s.uptimePercent24h}`)
      }

      // ── orca_server_reconnect_attempts ─────────────────────────
      lines.push('')
      lines.push('# HELP orca_server_reconnect_attempts Current reconnect attempt counter (0=stable)')
      lines.push('# TYPE orca_server_reconnect_attempts gauge')
      for (const s of report.servers) {
        lines.push(`orca_server_reconnect_attempts{server="${s.id}"} ${s.reconnectAttempt}`)
      }

      // ── orca_server_cpu_percent / ram_percent / disk_percent / latency_ms ──
      // FIX BUG-BE-HLD-010: real metrics, only emitted for servers where a
      // probe actually succeeded (matches Prometheus convention of omitting
      // a series rather than emitting a fake 0/NaN for missing data).
      lines.push('')
      lines.push('# HELP orca_server_cpu_percent CPU usage percent on the remote host (0–100)')
      lines.push('# TYPE orca_server_cpu_percent gauge')
      for (const s of report.servers) {
        if (s.cpuPercent !== null) {
          lines.push(`orca_server_cpu_percent{server="${s.id}"} ${s.cpuPercent}`)
        }
      }

      lines.push('')
      lines.push('# HELP orca_server_ram_percent RAM usage percent on the remote host (0–100)')
      lines.push('# TYPE orca_server_ram_percent gauge')
      for (const s of report.servers) {
        if (s.ramPercent !== null) {
          lines.push(`orca_server_ram_percent{server="${s.id}"} ${s.ramPercent}`)
        }
      }

      lines.push('')
      lines.push('# HELP orca_server_disk_percent Disk usage percent on the remote host (0–100)')
      lines.push('# TYPE orca_server_disk_percent gauge')
      for (const s of report.servers) {
        if (s.diskPercent !== null) {
          lines.push(`orca_server_disk_percent{server="${s.id}"} ${s.diskPercent}`)
        }
      }

      lines.push('')
      lines.push('# HELP orca_server_latency_ms SSH exec round-trip latency in milliseconds')
      lines.push('# TYPE orca_server_latency_ms gauge')
      for (const s of report.servers) {
        if (s.pingLatencyMs !== null) {
          lines.push(`orca_server_latency_ms{server="${s.id}"} ${s.pingLatencyMs}`)
        }
      }

      // ── orca_fleet_* aggregates ────────────────────────────────
      lines.push('')
      lines.push('# HELP orca_fleet_health_score Overall fleet health score (0–100)')
      lines.push('# TYPE orca_fleet_health_score gauge')
      lines.push(`orca_fleet_health_score ${report.summary.healthScore}`)

      lines.push('')
      lines.push('# HELP orca_fleet_servers_total Total number of servers in fleet')
      lines.push('# TYPE orca_fleet_servers_total gauge')
      lines.push(`orca_fleet_servers_total ${report.summary.total}`)

      lines.push('# HELP orca_fleet_servers_connected Number of connected servers')
      lines.push('# TYPE orca_fleet_servers_connected gauge')
      lines.push(`orca_fleet_servers_connected ${report.summary.connected}`)

      lines.push('# HELP orca_fleet_servers_error Number of servers in error state')
      lines.push('# TYPE orca_fleet_servers_error gauge')
      lines.push(`orca_fleet_servers_error ${report.summary.error}`)

      res.writeHead(200, { 'Content-Type': 'text/plain; version=0.0.4; charset=utf-8' })
      res.end(lines.join('\n') + '\n')
    } catch (err) {
      res.writeHead(500, { 'Content-Type': 'text/plain' })
      res.end(`# Error generating metrics: ${err instanceof Error ? err.message : String(err)}\n`)
    }
  }
}
