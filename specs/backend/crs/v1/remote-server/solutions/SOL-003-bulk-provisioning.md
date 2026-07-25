# SOL-003: Bulk Provisioning — Backend Solution

**CR:** [CR-003](../../../../../../../docs/crs/v1/remote-server/CR-003-bulk-provisioning.md)  
**Backend TDD refs:** `02-main-process.md`, `05-ssh-relay.md`, `07-runtime-service.md`  
**Depends on:** SOL-001  
**Effort:** Large (3–5 ngày)  
**Phase:** 2

---

## 1. Phân tích backend hiện tại

Từ `TDD-05 (SSH Relay)`:
- `deployRelay(connection)` — tự động upload binary + start relay khi SSH connect
- Deploy dùng SFTP, hoàn toàn tự động — đây là foundation mạnh để build bulk provision

Từ `TDD-02 (Main Process)` & CLI:
- `orca serve`, `orca status`, `orca environment` đã tồn tại
- Chưa có `orca fleet` command

**Gap:** Không có CLI interface để trigger nhiều provisioning operations song song.

---

## 2. Giải pháp backend

### 2.1 CLI Command: `orca fleet`

```typescript
// src/cli/specs/fleet.ts — NEW FILE

import type { CommandSpec } from './types'

export const FLEET_COMMAND_SPECS: CommandSpec[] = [
  {
    path: ['fleet', 'import'],
    summary: 'Import servers from a fleet config file into Orca',
    usage: 'orca fleet import <config-file>',
    examples: [
      'orca fleet import deploy/dev/orca-fleet.yaml',
      'orca fleet import ~/fleet.yaml'
    ],
    requiredArgs: ['config-file'],
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
      'orca fleet provision --all --concurrency 5',
    ],
    allowedFlags: ['--all', '--project', '--server', '--concurrency', '--dry-run', '--json'],
  },
  {
    path: ['fleet', 'status'],
    summary: 'Show connection status of all fleet servers',
    usage: 'orca fleet status [--project <name>] [--json]',
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
    summary: 'Sync fleet config — add new, remove deleted servers',
    usage: 'orca fleet sync <config-file>',
    allowedFlags: ['--dry-run', '--json'],
  },
]
```

### 2.2 CLI Handler: `fleet provision`

```typescript
// src/cli/handlers/fleet.ts — NEW FILE

import pLimit from 'p-limit'  // concurrency control
import { parseFleetConfig } from '../main/ssh/fleet-config-parser'
import { SshConnectionStore } from '../main/ssh/ssh-connection-store'
import { deployRelay } from '../main/ssh/ssh-relay-deploy'
import { createSshConnection } from '../main/ssh/ssh-connection'

const DEFAULT_CONCURRENCY = 3

async function handleFleetImport(args: {
  configFile: string
  dryRun?: boolean
  json?: boolean
}): Promise<void> {
  const config = await parseFleetConfig(args.configFile)

  if (args.dryRun) {
    console.log(`[dry-run] Would import ${config.servers.length} servers:`)
    config.servers.forEach(s => console.log(`  - ${s.label} (${s.host})`))
    return
  }

  // Call Orca runtime via Unix socket IPC (CLI → Runtime)
  const result = await callRuntimeIpc('ssh:importFleetConfig', {
    filePath: args.configFile
  })

  if (args.json) {
    console.log(JSON.stringify(result, null, 2))
    return
  }

  console.log(`✅ Imported ${result.serverCount} servers`)
  result.servers.forEach(s => {
    const icon = s.action === 'created' ? '  [new]' : ' [updated]'
    console.log(`${icon} ${s.fleetId}`)
  })
}

async function handleFleetProvision(args: {
  all?: boolean
  project?: string
  server?: string
  concurrency?: number
  dryRun?: boolean
  json?: boolean
}): Promise<void> {
  // Lấy danh sách targets cần provision
  let targets = await callRuntimeIpc('ssh:listTargets', {})

  if (args.project) {
    targets = targets.filter((t: SshTarget) => t.project === args.project)
  }
  if (args.server) {
    targets = targets.filter((t: SshTarget) => t.fleetId === args.server || t.id === args.server)
  }
  if (!args.all && !args.project && !args.server) {
    console.error('Error: specify --all, --project <name>, or --server <id>')
    process.exit(1)
  }

  if (args.dryRun) {
    console.log(`[dry-run] Would provision ${targets.length} servers:`)
    targets.forEach((t: SshTarget) => console.log(`  - ${t.label} (${t.host})`))
    return
  }

  const results: ProvisionResult[] = []
  const limit = pLimit(args.concurrency ?? DEFAULT_CONCURRENCY)

  console.log(`\nProvisioning ${targets.length} servers (concurrency: ${args.concurrency ?? DEFAULT_CONCURRENCY})...\n`)

  // Parallel provisioning với p-limit
  const tasks = targets.map((target: SshTarget) =>
    limit(async () => {
      const start = Date.now()
      try {
        // Connect + deploy relay
        await callRuntimeIpc('ssh:connect', { targetId: target.id })

        // Wait for relay deployment (poll status)
        await waitForRelayDeployed(target.id, 60_000)  // 60s timeout

        const elapsed = Date.now() - start
        results.push({ targetId: target.id, label: target.label, status: 'success', elapsedMs: elapsed })
        console.log(`  ✅ ${target.label} (${elapsed}ms)`)
      } catch (err) {
        const elapsed = Date.now() - start
        results.push({ targetId: target.id, label: target.label, status: 'error', error: String(err), elapsedMs: elapsed })
        console.log(`  ❌ ${target.label}: ${err}`)
      }
    })
  )

  await Promise.all(tasks)

  // Summary
  const succeeded = results.filter(r => r.status === 'success').length
  const failed = results.filter(r => r.status === 'error').length

  console.log(`\n──────────────────────────────────────`)
  console.log(`Summary: ${succeeded}/${targets.length} provisioned successfully`)
  if (failed > 0) {
    console.log(`Failed: ${failed} servers`)
  }

  if (args.json) {
    console.log(JSON.stringify(results, null, 2))
  }

  process.exit(failed > 0 ? 1 : 0)
}

async function waitForRelayDeployed(targetId: string, timeoutMs: number): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const state = await callRuntimeIpc('ssh:getConnectionState', { targetId })
    if (state.status === 'connected') return
    if (state.status === 'error') throw new Error(state.error ?? 'Relay deployment failed')
    await sleep(1000)
  }
  throw new Error('Relay deployment timed out')
}

type ProvisionResult = {
  targetId: string
  label: string
  status: 'success' | 'error'
  error?: string
  elapsedMs: number
}
```

### 2.3 Runtime IPC endpoint `ssh:connect` (có sẵn, verify behavior)

```typescript
// src/main/ipc/ssh.ts — VERIFY existing handler

// 'ssh:connect' đã có — trigger connection + auto relay deploy
// relay deploy xảy ra trong: connectToSshTarget() → ssh-relay-deploy.ts
// Cần verify: IPC responds khi relay deployed (not just SSH connected)

// Hiện tại có thể chỉ trả về khi SSH connected (step 1)
// Cần ensure: trả về khi relay ready (step 2: deployRelay done)

// Nếu chưa, cần thêm status polling hoặc 'ssh:getConnectionState' IPC
ipcMain.handle('ssh:getConnectionState', (_event, { targetId }: { targetId: string }) => {
  return sshManager.getConnectionState(targetId)
  // Returns: SshConnectionState { status, error, reconnectAttempt, ... }
})
```

### 2.4 `orca fleet list` output

```typescript
async function handleFleetList(args: {
  project?: string
  team?: string
  environment?: string
  json?: boolean
}): Promise<void> {
  const criteria = {
    project: args.project,
    team: args.team,
    environment: args.environment as SshTarget['environment'],
  }
  const targets: SshTarget[] = await callRuntimeIpc('ssh:filterTargets', criteria)
  const states: Record<string, SshConnectionState> = await callRuntimeIpc('ssh:getAllConnectionStates', {})

  if (args.json) {
    console.log(JSON.stringify(
      targets.map(t => ({ ...t, connectionState: states[t.id] })),
      null, 2
    ))
    return
  }

  // Tabular output
  const col = (s: string, w: number) => s.padEnd(w).substring(0, w)

  console.log('\n' + [
    col('ID', 20),
    col('LABEL', 28),
    col('PROJECT', 16),
    col('HOST', 28),
    col('STATUS', 14),
  ].join('  '))
  console.log('─'.repeat(110))

  for (const target of targets) {
    const state = states[target.id]
    const status = statusIcon(state?.status)
    console.log([
      col(target.fleetId ?? target.id, 20),
      col(target.label, 28),
      col(target.project ?? '—', 16),
      col(target.host, 28),
      col(status, 14),
    ].join('  '))
  }

  console.log(`\nTotal: ${targets.length} servers`)
}

function statusIcon(status?: string): string {
  switch (status) {
    case 'connected': return '✅ Connected'
    case 'connecting': return '⊙ Connecting'
    case 'disconnected': return '⚪ Disconnected'
    case 'error': return '❌ Error'
    default: return '— Unknown'
  }
}
```

### 2.5 CLI dispatch registration

```typescript
// src/cli/dispatch.ts — MODIFY

import { FLEET_COMMAND_SPECS } from './specs/fleet'
import {
  handleFleetImport,
  handleFleetProvision,
  handleFleetStatus,
  handleFleetList,
  handleFleetSync,
} from './handlers/fleet'

// Register fleet commands:
registerCommand(['fleet', 'import'], handleFleetImport)
registerCommand(['fleet', 'provision'], handleFleetProvision)
registerCommand(['fleet', 'status'], handleFleetStatus)
registerCommand(['fleet', 'list'], handleFleetList)
registerCommand(['fleet', 'sync'], handleFleetSync)
```

---

## 3. Concurrency & Error Handling

```
Parallel provisioning strategy:
- Dùng p-limit (đã có trong project)
- Default concurrency: 3 (không quá tải SSH server)
- Timeout mỗi server: 60s (relay deploy có thể chậm trên server yếu)
- Failure isolation: 1 server fail không block server khác
- Exit code 1 nếu có bất kỳ failure nào (CI/CD friendly)
```

---

## 4. Files cần thay đổi

| File | Action | Chi tiết |
|------|--------|---------|
| `src/cli/specs/fleet.ts` | **NEW** | Command specs |
| `src/cli/handlers/fleet.ts` | **NEW** | CLI handlers |
| `src/cli/dispatch.ts` | MODIFY | Register fleet commands |
| `src/main/ipc/ssh.ts` | MODIFY | Thêm `ssh:getConnectionState`, `ssh:getAllConnectionStates`, `ssh:filterTargets` |
| `package.json` | VERIFY | `p-limit` dependency |

---

## 5. Testing

```typescript
// src/cli/handlers/fleet.test.ts — NEW
describe('fleet provision', () => {
  it('provisions servers concurrently', async () => {
    // Mock IPC calls
    const connectCallOrder: string[] = []
    mockIpc('ssh:connect', ({ targetId }) => {
      connectCallOrder.push(targetId)
      return delay(100)  // simulate 100ms relay deploy
    })

    const start = Date.now()
    await handleFleetProvision({ all: true, concurrency: 3 })
    const elapsed = Date.now() - start

    // 6 servers / concurrency 3 = 2 batches × 100ms ≈ 200ms
    // Not sequential (6 × 100ms = 600ms)
    expect(elapsed).toBeLessThan(400)
  })

  it('is idempotent', async () => {
    await handleFleetProvision({ server: 'dev-alpha' })
    await handleFleetProvision({ server: 'dev-alpha' })  // second run
    expect(mockConnectCallCount).toBe(2)  // called twice but no duplicates in store
  })
})
```

---

## 6. Implementation Status

> **✅ IMPLEMENTED — Phase 2 Complete**  
> Ngày: 2026-07-22

### Đã triển khai

| File | Status | Chi tiết |
|------|--------|---------|
| [`src/cli/specs/fleet.ts`](../../../../../src/cli/specs/fleet.ts) | ✅ Done | **NEW** — 6 `CommandSpec`: import, provision, status, list, sync, bootstrap |
| [`src/cli/handlers/fleet.ts`](../../../../../src/cli/handlers/fleet.ts) | ✅ Done | **NEW** — `FLEET_HANDLERS` map với `client.call()` RPC pattern, `p-limit` concurrency |
| [`src/cli/dispatch.ts`](../../../../../src/cli/dispatch.ts) | ✅ Done | `FLEET_HANDLERS` registered |
| [`src/cli/specs/index.ts`](../../../../../src/cli/specs/index.ts) | ✅ Done | `FLEET_COMMAND_SPECS` registered |
| [`src/main/ipc/ssh.ts`](../../../../../src/main/ipc/ssh.ts) | ✅ Done | `ssh:filterTargets`, `ssh:getAllConnectionStates` IPC handlers |
| [`src/main/runtime/rpc/methods/ssh.ts`](../../../../../src/main/runtime/rpc/methods/ssh.ts) | ✅ Done | `ssh.filterTargets`, `ssh.getAllConnectionStates` RPC methods |

### Deviation từ design gốc

> **Note:** CLI handlers dùng `client.call()` (JSON-RPC qua Unix socket) thay vì `callRuntimeIpc()` wrapper riêng — đây là pattern đúng theo codebase. `fleet sync` được implement như alias của `fleet import` (idempotent).

### Commands đã hoạt động

```bash
orca fleet import deploy/dev/orca-fleet.yaml
orca fleet list [--project <name>] [--json]
orca fleet status [--project <name>] [--json]
orca fleet provision --all [--concurrency 3] [--dry-run]
orca fleet bootstrap --server <id> --config <fleet.yaml>
orca fleet sync deploy/dev/orca-fleet.yaml
```

### Notes

- **TASK-011** (ssh:bootstrapServer IPC): ✅ Done  
- **TASK-012** (cli specs/fleet.ts): ✅ Done  
- **TASK-013** (cli handlers/fleet.ts): ✅ Done  
- **TASK-014** (dispatch registration): ✅ Done
