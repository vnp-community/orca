# TASK-011: Thêm IPC Handler `ssh:bootstrapServer`
# TASK-012: Tạo `src/cli/specs/fleet.ts`
# TASK-013: Tạo `src/cli/handlers/fleet.ts`
# TASK-014: Register fleet commands trong `dispatch.ts`

---

# TASK-011: IPC Handler `ssh:bootstrapServer`

**Source:** SOL-004  
**Phase:** 1 | **Effort:** S | **Depends on:** TASK-010

## File to modify: `src/main/ipc/ssh.ts`

```typescript
  ipcMain.handle('ssh:bootstrapServer', async (_event, args: {
    targetId: string
    fleetConfigPath?: string
    options?: {
      skipNodeInstall?: boolean
      skipGitInstall?: boolean
      skipRepoClone?: boolean
      skipSetupScript?: boolean
      nodeVersion?: string
    }
  }) => {
    try {
      const result = await orcaRuntime.bootstrapServer(args.targetId, {
        fleetConfigPath: args.fleetConfigPath,
        ...args.options,
        onProgress: (step) => {
          // Stream progress to renderer
          for (const win of BrowserWindow.getAllWindows()) {
            if (!win.isDestroyed()) {
              win.webContents.send('ssh:bootstrapProgress', {
                targetId: args.targetId,
                step,
              })
            }
          }
        },
      })
      return { ok: result.success, result }
    } catch (err) {
      return { ok: false, error: err instanceof Error ? err.message : String(err) }
    }
  })
```

## Done criteria
- [x] `ssh:bootstrapServer` IPC handler registered
- [x] Progress streamed via `ssh:bootstrapProgress` IPC event
- [x] TypeScript compile: no errors

**Status: ✅ DONE** — Handler added in `src/main/ipc/ssh.ts` at line ~928. Uses dynamic import of `fleet-bootstrap-service`. Streams step progress via `win.webContents.send('ssh:bootstrapProgress', ...)`. Also exposed as `ssh.bootstrapServer` RPC method for CLI.

---

# TASK-012: `src/cli/specs/fleet.ts`

**Source:** SOL-003  
**Phase:** 2 | **Effort:** S | **Depends on:** —

## File to create: `src/cli/specs/fleet.ts` (NEW)

```typescript
// src/cli/specs/fleet.ts
import type { CommandSpec } from './types'  // adjust import path to match project

export const FLEET_COMMAND_SPECS: CommandSpec[] = [
  {
    path: ['fleet', 'import'],
    summary: 'Import servers from a fleet config YAML file',
    usage: 'orca fleet import <config-file>',
    examples: [
      'orca fleet import deploy/dev/orca-fleet.yaml',
    ],
    allowedFlags: ['--dry-run', '--json'],
  },
  {
    path: ['fleet', 'provision'],
    summary: 'Deploy Orca relay to fleet servers',
    usage: 'orca fleet provision [--all] [--project <name>] [--server <id>] [--concurrency <n>]',
    examples: [
      'orca fleet provision --all',
      'orca fleet provision --project vnp-blc',
      'orca fleet provision --server dev-alpha',
    ],
    allowedFlags: ['--all', '--project', '--server', '--concurrency', '--dry-run', '--json'],
  },
  {
    path: ['fleet', 'status'],
    summary: 'Show health status of all fleet servers',
    usage: 'orca fleet status [--project <name>] [--team <name>] [--json]',
    allowedFlags: ['--project', '--team', '--json'],
  },
  {
    path: ['fleet', 'list'],
    summary: 'List all servers in fleet',
    usage: 'orca fleet list [--project <name>] [--json]',
    allowedFlags: ['--project', '--team', '--environment', '--json'],
  },
  {
    path: ['fleet', 'sync'],
    summary: 'Sync fleet config — add/remove servers to match config',
    usage: 'orca fleet sync <config-file>',
    allowedFlags: ['--dry-run', '--json'],
  },
  {
    path: ['fleet', 'bootstrap'],
    summary: 'Install dependencies and clone repos on a fleet server',
    usage: 'orca fleet bootstrap [--server <id>] [--all] [--config <fleet.yaml>]',
    examples: [
      'orca fleet bootstrap --server dev-alpha --config deploy/dev/orca-fleet.yaml',
      'orca fleet bootstrap --all --config deploy/dev/orca-fleet.yaml',
    ],
    allowedFlags: ['--server', '--all', '--config', '--skip-node', '--skip-git', '--json'],
  },
]
```

## Notes for AI
- Check `src/cli/specs/` for existing files to understand the `CommandSpec` type shape
- Match the exact `CommandSpec` interface used by the project

## Done criteria
- [x] `src/cli/specs/fleet.ts` created with 6 command specs
- [x] TypeScript compile: no errors

**Status: ✅ DONE** — Created `src/cli/specs/fleet.ts` using `GLOBAL_FLAGS` from `args.ts`. 6 specs: `fleet import`, `fleet provision`, `fleet status`, `fleet list`, `fleet sync`, `fleet bootstrap`. All registered in `specs/index.ts`.

---

# TASK-013: `src/cli/handlers/fleet.ts`

**Source:** SOL-003, SOL-004  
**Phase:** 2 | **Effort:** M | **Depends on:** TASK-012, TASK-003

## File to create: `src/cli/handlers/fleet.ts` (NEW)

```typescript
// src/cli/handlers/fleet.ts
// NOTE: callRuntimeIpc() connects to Orca runtime via Unix socket.
// Check how other CLI handlers call runtime IPC (e.g., src/cli/handlers/status.ts)

import type { SshTarget } from '../../../shared/ssh-types'
import type { FleetStatusReport } from '../../../shared/fleet-types'  // created in TASK-017

// p-limit for concurrent operations — check if already in package.json
// If not: npm install p-limit
import pLimit from 'p-limit'

// ── Shared utilities ──────────────────────────────────────────

const col = (s: string | undefined, w: number): string =>
  String(s ?? '—').padEnd(w).substring(0, w)

function formatUptime(seconds: number): string {
  if (seconds === 0) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function formatStatus(status: string): string {
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
  return map[status] ?? status
}

// ── fleet import ──────────────────────────────────────────────

export async function handleFleetImport(args: {
  configFile: string
  dryRun?: boolean
  json?: boolean
}): Promise<void> {
  if (args.dryRun) {
    // Parse and show what would be imported
    const { parseFleetConfig } = await import('../../main/ssh/fleet-config-parser')
    const config = await parseFleetConfig(args.configFile)
    if (args.json) {
      console.log(JSON.stringify({ dryRun: true, servers: config.servers }, null, 2))
    } else {
      console.log(`[dry-run] Would import ${config.servers.length} servers:`)
      config.servers.forEach(s => console.log(`  • ${s.label} (${s.host})`))
    }
    return
  }

  const result = await callRuntimeIpc('ssh:importFleetConfig', { filePath: args.configFile })

  if (!result.ok) {
    console.error(`Error: ${result.error}`)
    process.exit(1)
  }

  if (args.json) {
    console.log(JSON.stringify(result.result, null, 2))
    return
  }

  console.log(`✅ Imported ${result.result.serverCount} servers from ${args.configFile}`)
  for (const s of result.result.servers) {
    const icon = s.action === 'created' ? '[new]    ' : '[updated]'
    console.log(`  ${icon} ${s.fleetId}`)
  }
}

// ── fleet provision ───────────────────────────────────────────

export async function handleFleetProvision(args: {
  all?: boolean
  project?: string
  server?: string
  concurrency?: number
  dryRun?: boolean
  json?: boolean
}): Promise<void> {
  if (!args.all && !args.project && !args.server) {
    console.error('Error: specify --all, --project <name>, or --server <id>')
    process.exit(1)
  }

  const criteria: Record<string, string | undefined> = {
    project: args.project,
  }
  let targets: SshTarget[] = await callRuntimeIpc('ssh:filterTargets', criteria)

  if (args.server) {
    targets = targets.filter(t => t.fleetId === args.server || t.id === args.server)
    if (!targets.length) {
      console.error(`Error: server "${args.server}" not found`)
      process.exit(1)
    }
  }

  if (args.dryRun) {
    console.log(`[dry-run] Would provision ${targets.length} servers:`)
    targets.forEach(t => console.log(`  • ${t.label} (${t.host})`))
    return
  }

  const concurrency = args.concurrency ?? 3
  const limit = pLimit(concurrency)
  const results: Array<{ targetId: string; label: string; success: boolean; error?: string; elapsedMs: number }> = []

  console.log(`\nProvisioning ${targets.length} servers (concurrency: ${concurrency})...\n`)

  const tasks = targets.map(target =>
    limit(async () => {
      const start = Date.now()
      try {
        // Connect triggers relay deploy automatically
        await callRuntimeIpc('ssh:connect', { targetId: target.id })
        // Wait for connected status (poll up to 60s)
        await waitForStatus(target.id, 'connected', 60_000)
        const elapsed = Date.now() - start
        results.push({ targetId: target.id, label: target.label, success: true, elapsedMs: elapsed })
        console.log(`  ✅ ${target.label} (${elapsed}ms)`)
      } catch (err) {
        const elapsed = Date.now() - start
        const error = err instanceof Error ? err.message : String(err)
        results.push({ targetId: target.id, label: target.label, success: false, error, elapsedMs: elapsed })
        console.log(`  ❌ ${target.label}: ${error}`)
      }
    })
  )

  await Promise.all(tasks)

  const succeeded = results.filter(r => r.success).length
  const failed = results.filter(r => !r.success).length

  console.log(`\n${'─'.repeat(60)}`)
  console.log(`Summary: ${succeeded}/${targets.length} provisioned | ${failed} failed`)

  if (args.json) process.stdout.write(JSON.stringify(results, null, 2) + '\n')
  process.exit(failed > 0 ? 1 : 0)
}

async function waitForStatus(targetId: string, expectedStatus: string, timeoutMs: number): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const states: Record<string, { status: string }> = await callRuntimeIpc('ssh:getAllConnectionStates', {})
    const state = states[targetId]
    if (state?.status === expectedStatus) return
    if (state?.status === 'error' || state?.status === 'reconnection-failed') {
      throw new Error(`Connection failed with status: ${state.status}`)
    }
    await new Promise(resolve => setTimeout(resolve, 1000))
  }
  throw new Error(`Timeout waiting for ${expectedStatus} status`)
}

// ── fleet list ────────────────────────────────────────────────

export async function handleFleetList(args: {
  project?: string
  team?: string
  environment?: string
  json?: boolean
}): Promise<void> {
  const targets: SshTarget[] = await callRuntimeIpc('ssh:filterTargets', {
    project: args.project,
    team: args.team,
    environment: args.environment,
  })
  const states: Record<string, { status: string }> = await callRuntimeIpc('ssh:getAllConnectionStates', {})

  if (args.json) {
    console.log(JSON.stringify(targets.map(t => ({ ...t, connectionStatus: states[t.id]?.status })), null, 2))
    return
  }

  console.log('\n' + [col('ID', 20), col('LABEL', 28), col('PROJECT', 16), col('HOST', 26), col('STATUS', 16)].join('  '))
  console.log('─'.repeat(110))
  for (const t of targets) {
    const status = formatStatus(states[t.id]?.status ?? 'disconnected')
    console.log([col(t.fleetId ?? t.id, 20), col(t.label, 28), col(t.project, 16), col(t.host, 26), col(status, 16)].join('  '))
  }
  console.log(`\nTotal: ${targets.length} servers`)
}

// ── fleet bootstrap ───────────────────────────────────────────

export async function handleFleetBootstrap(args: {
  server?: string
  all?: boolean
  config?: string
  skipNode?: boolean
  skipGit?: boolean
  json?: boolean
}): Promise<void> {
  let targets: SshTarget[] = await callRuntimeIpc('ssh:listTargets', {})

  if (args.server) {
    targets = targets.filter(t => t.fleetId === args.server || t.id === args.server)
    if (!targets.length) { console.error(`Server not found: ${args.server}`); process.exit(1) }
  } else if (!args.all) {
    console.error('Error: specify --server <id> or --all'); process.exit(1)
  }

  const limit = pLimit(2)
  const tasks = targets.map(target =>
    limit(async () => {
      console.log(`\n[${target.fleetId ?? target.id}] Bootstrapping...`)
      const result = await callRuntimeIpc('ssh:bootstrapServer', {
        targetId: target.id,
        fleetConfigPath: args.config,
        options: { skipNodeInstall: args.skipNode, skipGitInstall: args.skipGit },
      })
      for (const step of result.result?.steps ?? []) {
        const icon = step.status === 'ok' ? '  ✅' : step.status === 'error' ? '  ❌' : step.status === 'skipped' ? '  ⊘ ' : '  ⊙ '
        console.log(`${icon} ${step.step}${step.message ? ': ' + step.message : ''}`)
      }
    })
  )
  await Promise.all(tasks)
}

// ── callRuntimeIpc helper (adapt to project's pattern) ────────

async function callRuntimeIpc<T>(method: string, params: unknown): Promise<T> {
  // ADAPT: Look at src/cli/handlers/ for how other handlers call runtime IPC
  // Common pattern: connect to Unix socket → send JSON-RPC → receive response
  // Example from existing handlers might use a utility like:
  //   import { callRuntime } from '../runtime-client'
  throw new Error(`callRuntimeIpc not implemented — adapt to project pattern for method: ${method}`)
}
```

## Done criteria
- [x] `handleFleetImport()` exported
- [x] `handleFleetProvision()` exported (with p-limit concurrency)
- [x] `handleFleetList()` exported
- [x] `handleFleetBootstrap()` exported
- [x] `callRuntimeIpc()` adapted to actual project runtime client

**Status: ✅ DONE** — Created `src/cli/handlers/fleet.ts` using `client.call()` RPC pattern (not raw `callRuntimeIpc`). FLEET_HANDLERS map contains: `fleet import`, `fleet sync`, `fleet list`, `fleet status`, `fleet provision`, `fleet bootstrap`. Uses `p-limit` for concurrency.

---

# TASK-014: Register fleet commands in `dispatch.ts`

**Source:** SOL-003  
**Phase:** 2 | **Effort:** XS | **Depends on:** TASK-013

## File to modify: `src/cli/dispatch.ts` (or equivalent CLI entry)

```typescript
import { FLEET_COMMAND_SPECS } from './specs/fleet'
import {
  handleFleetImport,
  handleFleetProvision,
  handleFleetList,
  handleFleetBootstrap,
} from './handlers/fleet'

// Register specs
registerSpecs(FLEET_COMMAND_SPECS)

// Register handlers
registerCommand(['fleet', 'import'], handleFleetImport)
registerCommand(['fleet', 'provision'], handleFleetProvision)
registerCommand(['fleet', 'list'], handleFleetList)
registerCommand(['fleet', 'bootstrap'], handleFleetBootstrap)
```

**Note:** Check existing `dispatch.ts` for the exact registration pattern used.

## Done criteria
- [x] `orca fleet import`, `orca fleet provision`, `orca fleet list`, `orca fleet bootstrap` all registered
- [x] TypeScript compile: no errors

**Status: ✅ DONE** — `FLEET_HANDLERS` registered in `src/cli/dispatch.ts`. `FLEET_COMMAND_SPECS` registered in `src/cli/specs/index.ts`. All 6 fleet commands available in Orca CLI.
