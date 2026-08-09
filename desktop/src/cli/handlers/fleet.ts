// src/cli/handlers/fleet.ts
// Fleet CLI handlers — import, provision, status, list, sync, bootstrap.
// Uses client.call() via the RuntimeClient (JSON-RPC over Unix socket).
import pLimit from 'p-limit'
import type { SshTarget, SshConnectionState } from '../../shared/ssh-types'
import type { BootstrapResult } from '../../shared/fleet-types'
import type { CommandHandler } from '../dispatch'
import { RuntimeClientError } from '../runtime-client'
import { getOptionalStringFlag } from '../flags'

// ── Formatting utilities ───────────────────────────────────────

const col = (s: string | undefined, w: number): string =>
  String(s ?? '—').padEnd(w).substring(0, w)

function formatStatus(status: string | undefined): string {
  const map: Record<string, string> = {
    connected: '✅ Connected',
    connecting: '⊙  Connecting',
    disconnected: '⚪ Offline',
    'deploying-relay': '⊙  Deploying relay',
    reconnecting: '↻  Reconnecting',
    'reconnection-failed': '❌ Reconnect failed',
    'auth-failed': '🔑 Auth failed',
    error: '❌ Error',
  }
  return map[status ?? ''] ?? (status ?? 'unknown')
}

// ── Fleet handlers ─────────────────────────────────────────────

export const FLEET_HANDLERS: Record<string, CommandHandler> = {
  // ── fleet import ───────────────────────────────────────────────
  'fleet import': async ({ flags, client, cwd, json }) => {
    const rawConfigFile = flags.get('_positionals') as string | undefined
    const configFile = rawConfigFile ?? getOptionalStringFlag(flags, 'config')
    if (!configFile) {
      throw new RuntimeClientError('invalid_argument', 'Usage: orca fleet import <config-file>')
    }

    // Resolve to absolute path if relative
    const path = await import('node:path')
    const filePath = path.isAbsolute(configFile) ? configFile : path.resolve(cwd, configFile)

    const dryRun = flags.get('dry-run') === true

    if (dryRun) {
      // Parse locally for preview — requires fleet-config-parser accessible from CLI
      try {
        const { parseFleetConfig } = await import('../../shared/fleet-config-parser.js')
        const config = await parseFleetConfig(filePath)
        if (json) {
          console.log(JSON.stringify({ dryRun: true, servers: config.servers }, null, 2))
        } else {
          console.log(`[dry-run] Would import ${config.servers.length} servers from ${filePath}:`)
          config.servers.forEach((s) => console.log(`  • ${s.id ?? s.host}  ${s.label}  (${s.host})`))
        }
        return
      } catch (err) {
        throw new RuntimeClientError('runtime_error', `Failed to parse config: ${String(err)}`)
      }
    }

    const res = await client.call<{ result: { imported: number; updated: number } }>(
      'ssh.importFleetConfig',
      { filePath }
    )

    if (json) {
      console.log(JSON.stringify(res.result, null, 2))
      return
    }
    const r = res as any
    console.log(`✅ Fleet import complete: ${r.result?.imported ?? 0} new, ${r.result?.updated ?? 0} updated`)
  },

  // ── fleet sync (alias for import — idempotent) ─────────────────
  'fleet sync': async ({ flags, client, cwd, json }) => {
    const rawConfigFile = flags.get('_positionals') as string | undefined
    const configFile = rawConfigFile ?? getOptionalStringFlag(flags, 'config')
    if (!configFile) {
      throw new RuntimeClientError('invalid_argument', 'Usage: orca fleet sync <config-file>')
    }

    const path = await import('node:path')
    const filePath = path.isAbsolute(configFile) ? configFile : path.resolve(cwd, configFile)

    const res = await client.call<{ result: { imported: number; updated: number } }>(
      'ssh.importFleetConfig',
      { filePath }
    )

    if (json) {
      console.log(JSON.stringify(res.result, null, 2))
      return
    }
    const r = res as any
    console.log(`✅ Fleet synced: ${r.result?.imported ?? 0} new, ${r.result?.updated ?? 0} updated`)
  },

  // ── fleet list ────────────────────────────────────────────────
  'fleet list': async ({ flags, client, json }) => {
    const project = getOptionalStringFlag(flags, 'project')
    const team = getOptionalStringFlag(flags, 'team')
    const environment = getOptionalStringFlag(flags, 'environment')

    const [targetsRes, statesRes] = await Promise.all([
      client.call<{ targets: SshTarget[] }>('ssh.filterTargets', {
        project,
        team,
        environment: environment as 'development' | 'staging' | 'production' | undefined,
      }),
      client.call<{ states: Record<string, SshConnectionState | null> }>('ssh.getAllConnectionStates'),
    ])

    const targets = targetsRes.result.targets
    const states = statesRes.result.states

    if (json) {
      console.log(
        JSON.stringify(
          targets.map((t) => ({ ...t, connectionStatus: states[t.id]?.status })),
          null,
          2
        )
      )
      return
    }

    console.log(
      `\n${ 
        [col('ID', 20), col('LABEL', 28), col('PROJECT', 16), col('HOST', 26), col('STATUS', 18)].join(
          '  '
        )}`
    )
    console.log('─'.repeat(114))
    for (const t of targets) {
      const status = formatStatus(states[t.id]?.status)
      console.log(
        [
          col(t.fleetId ?? t.id, 20),
          col(t.label, 28),
          col(t.project, 16),
          col(t.host, 26),
          col(status, 18),
        ].join('  ')
      )
    }
    console.log(`\nTotal: ${targets.length} servers`)
  },

  // ── fleet status ──────────────────────────────────────────────
  'fleet status': async ({ flags, client, json }) => {
    const project = getOptionalStringFlag(flags, 'project')
    const team = getOptionalStringFlag(flags, 'team')

    const res = await client.call<{ report: import('../../shared/fleet-types').FleetStatusReport }>(
      'ssh.getFleetStatus',
      { project: project ?? null, team: team ?? null }
    )
    const report = res.result.report

    if (json) {
      console.log(JSON.stringify(report, null, 2))
      process.exit(report.summary.error > 0 ? 1 : 0)
      return
    }

    const formatUptime = (seconds: number): string => {
      if (!seconds) {return '—'}
      if (seconds < 60) {return `${seconds}s`}
      if (seconds < 3600) {return `${Math.floor(seconds / 60)}m`}
      return `${Math.floor(seconds / 3600)}h${Math.floor((seconds % 3600) / 60)}m`
    }

    console.log(`\nFleet Health — ${new Date(report.generatedAt).toLocaleString()}`)
    console.log('─'.repeat(95))
    console.log(
      [col('SERVER', 20), col('PROJECT', 14), col('STATUS', 22), col('UPTIME', 10), col('24H%', 6), col('RELAY', 8)].join('  ')
    )
    console.log('─'.repeat(95))

    for (const s of report.servers) {
      const status = formatStatus(s.status)
      const uptime = formatUptime(s.uptimeSeconds)
      console.log(
        [col(s.id, 20), col(s.project, 14), col(status, 22), col(uptime, 10), col(`${s.uptimePercent24h}%`, 6), col(s.relayVersion ?? 'N/A', 8)].join('  ')
      )
    }

    console.log('─'.repeat(95))
    const su = report.summary
    console.log(`Summary: ${su.connected}/${su.total} connected | ${su.error} error | Health score: ${su.healthScore}%`)
    process.exit(su.error > 0 ? 1 : 0)
  },

  // ── fleet provision ───────────────────────────────────────────
  'fleet provision': async ({ flags, client, json }) => {
    const all = flags.get('all') === true
    const project = getOptionalStringFlag(flags, 'project')
    const serverFilter = getOptionalStringFlag(flags, 'server')
    const dryRun = flags.get('dry-run') === true
    const rawConcurrency = flags.get('concurrency')
    const concurrency = typeof rawConcurrency === 'string' ? Number.parseInt(rawConcurrency, 10) : 3

    if (!all && !project && !serverFilter) {
      throw new RuntimeClientError('invalid_argument', 'Specify --all, --project <name>, or --server <id>')
    }

    const targetsRes = await client.call<{ targets: SshTarget[] }>('ssh.filterTargets', { project })
    let targets = targetsRes.result.targets

    if (serverFilter) {
      targets = targets.filter((t) => t.fleetId === serverFilter || t.id === serverFilter)
      if (!targets.length) {
        throw new RuntimeClientError('not_found', `Server "${serverFilter}" not found in fleet`)
      }
    }

    if (dryRun) {
      console.log(`[dry-run] Would provision ${targets.length} servers:`)
      targets.forEach((t) => console.log(`  • ${t.label} (${t.host})`))
      return
    }

    const limit = pLimit(concurrency)
    const results: { id: string; label: string; success: boolean; error?: string; ms: number }[] = []

    console.log(`\nProvisioning ${targets.length} servers (concurrency: ${concurrency})...\n`)

    await Promise.all(
      targets.map((target) =>
        limit(async () => {
          const start = Date.now()
          try {
            await client.call('ssh.connect', { targetId: target.id })
            // FIX BUG-BE-HLD-012: 'fleet provision' previously only connected (SSH + relay
            // deploy) — it never actually PROVISIONED the server (Node.js/Git install, repo
            // clone). ssh.bootstrapServer does that; audit found no caller anywhere wired it
            // into this command, so `orca fleet provision` silently did far less than its name
            // promised.
            const bootstrapRes = await client.call<{ result: { success: boolean; error?: string } }>(
              'ssh.bootstrapServer',
              { targetId: target.id }
            )
            if (!bootstrapRes.result.success) {
              throw new Error(bootstrapRes.result.error ?? 'bootstrap failed')
            }
            results.push({ id: target.id, label: target.label, success: true, ms: Date.now() - start })
            console.log(`  ✅ ${target.label} (${Date.now() - start}ms)`)
          } catch (err) {
            const error = err instanceof Error ? err.message : String(err)
            results.push({ id: target.id, label: target.label, success: false, error, ms: Date.now() - start })
            console.log(`  ❌ ${target.label}: ${error}`)
          }
        })
      )
    )

    const succeeded = results.filter((r) => r.success).length
    const failed = results.length - succeeded
    console.log(`\n${'─'.repeat(60)}`)
    console.log(`Summary: ${succeeded}/${targets.length} provisioned | ${failed} failed`)

    if (json) {console.log(JSON.stringify(results, null, 2))}
    if (failed > 0) {process.exit(1)}
  },

  // ── fleet bootstrap ───────────────────────────────────────────
  'fleet bootstrap': async ({ flags, client, json }) => {
    const serverFilter = getOptionalStringFlag(flags, 'server')
    const all = flags.get('all') === true
    const config = getOptionalStringFlag(flags, 'config')
    const skipNode = flags.get('skip-node') === true
    const skipGit = flags.get('skip-git') === true

    if (!all && !serverFilter) {
      throw new RuntimeClientError('invalid_argument', 'Specify --server <id> or --all')
    }

    const targetsRes = await client.call<{ targets: SshTarget[] }>('ssh.filterTargets', {})
    let targets = targetsRes.result.targets

    if (serverFilter) {
      targets = targets.filter((t) => t.fleetId === serverFilter || t.id === serverFilter)
      if (!targets.length) {
        throw new RuntimeClientError('not_found', `Server "${serverFilter}" not found in fleet`)
      }
    }

    const limit = pLimit(2) // Conservative concurrency for bootstrap

    await Promise.all(
      targets.map((target) =>
        limit(async () => {
          console.log(`\n[${target.fleetId ?? target.id}] Bootstrapping ${target.label}...`)
          const res = await client.call<{ result: BootstrapResult }>('ssh.bootstrapServer', {
            targetId: target.id,
            fleetConfigPath: config,
            skipNodeInstall: skipNode,
            skipGitInstall: skipGit,
          })

          const result = res.result.result
          for (const step of result?.steps ?? []) {
            const icon =
              step.status === 'ok'
                ? '  ✅'
                : step.status === 'error'
                  ? '  ❌'
                  : step.status === 'skipped'
                    ? '  ⊘ '
                    : '  ⊙ '
            const detail = step.message ? `: ${step.message}` : step.error ? `: ${step.error}` : ''
            console.log(`${icon} ${step.step}${detail}`)
          }

          if (result?.success === false) {
            console.log(`  ⚠️  Bootstrap failed: ${result.error ?? 'unknown error'}`)
          }

          if (json) {console.log(JSON.stringify(result, null, 2))}
        })
      )
    )
  },
}
