# SOL-001: Fleet Inventory Config — Backend Solution

**CR:** [CR-001](../../../../../../../docs/crs/v1/remote-server/CR-001-fleet-inventory-config.md)  
**Backend TDD refs:** `06-persistence.md`, `09-ipc-handlers.md`, `07-runtime-service.md`  
**Effort:** Medium (2–3 ngày)  
**Phase:** 1 (Critical)

---

## 1. Phân tích backend hiện tại

Từ `TDD-06 (Persistence)`, `SshTarget` được persist trong `PersistedState.sshTargets[]` (JSON store) với schema:

```typescript
// src/shared/ssh-types.ts (hiện tại)
type SshTarget = {
  id: string
  label: string
  host: string
  port: number
  username: string
  identityFile?: string
  jumpHost?: string
  proxyCommand?: string
  relayGracePeriodSeconds?: number
  // THIẾU: project, team, environment, repos
}
```

Từ `TDD-09 (IPC Handlers)`, SSH IPC handlers hiện có:
- `ssh.listTargets`
- `ssh.addTarget`
- `ssh.removeTarget`
- `ssh.importFromSshConfig`

**Chưa có:** `ssh.importFleetConfig`, fleet config parser.

---

## 2. Giải pháp backend

### 2.1 Extend `SshTarget` type

```typescript
// src/shared/ssh-types.ts — MODIFY

export type SshTarget = {
  // ── Existing fields (không đổi) ──────────────────────────
  id: string
  label: string
  host: string
  port: number
  username: string
  identityFile?: string
  jumpHost?: string
  proxyCommand?: string
  relayGracePeriodSeconds?: number

  // ── NEW: Fleet metadata ───────────────────────────────────
  /** Project identifier. e.g. "vnp-blc", "vnp-ai-ops" */
  project?: string
  /** Team identifier. e.g. "backend", "frontend" */
  team?: string
  /** Deployment environment */
  environment?: 'development' | 'staging' | 'production'
  /** Free-form tags for flexible grouping */
  tags?: string[]
  /** Repos available on this server */
  repos?: Array<{
    path: string   // /srv/projects/vnp-blc
    name: string   // vnp-blc
  }>
  /** Source: was this target imported from fleet config? */
  fleetConfigSource?: string  // path to orca-fleet.yaml
  /** Stable ID from fleet config (để detect re-import) */
  fleetId?: string
}
```

### 2.2 Fleet Config Schema và Parser

```typescript
// src/main/ssh/fleet-config-parser.ts — NEW FILE

import * as z from 'zod'
import * as yaml from 'js-yaml'
import * as fs from 'node:fs/promises'

// Zod schema cho orca-fleet.yaml
const FleetServerSchema = z.object({
  id: z.string(),
  label: z.string(),
  host: z.string(),
  port: z.number().optional().default(22),
  username: z.string().optional(),
  identityFile: z.string().optional(),
  jumpHost: z.string().optional(),
  proxyCommand: z.string().optional(),
  relayGracePeriodSeconds: z.number().optional(),
  project: z.string().optional(),
  team: z.string().optional(),
  environment: z.enum(['development', 'staging', 'production']).optional(),
  tags: z.array(z.string()).optional(),
  repos: z.array(z.object({
    path: z.string(),
    name: z.string(),
    url: z.string().optional(),
    branch: z.string().optional(),
  })).optional(),
  portForwards: z.array(z.object({
    remotePort: z.number(),
    localPort: z.number(),
    label: z.string().optional(),
  })).optional(),
  bootstrap: z.object({
    repos: z.array(z.object({
      url: z.string(),
      path: z.string(),
      branch: z.string().optional(),
    })).optional(),
    setupScript: z.string().optional(),
  }).optional(),
})

const FleetConfigSchema = z.object({
  version: z.literal('1'),
  defaults: FleetServerSchema.partial().optional(),
  bootstrap: z.object({
    nodeVersion: z.string().optional(),
    gitVersion: z.string().optional(),
    packages: z.array(z.string()).optional(),
  }).optional(),
  access: z.object({
    sso: z.object({
      provider: z.string(),
      clientId: z.string(),
      allowedOrg: z.string().optional(),
    }).optional(),
    policies: z.array(z.object({
      team: z.string().optional(),
      role: z.string().optional(),
      allowedServers: z.union([z.literal('*'), z.array(z.string())]),
      agentTrust: z.enum(['minimal', 'standard', 'full']).optional(),
    })).optional(),
  }).optional(),
  servers: z.array(FleetServerSchema),
})

export type FleetConfig = z.infer<typeof FleetConfigSchema>
export type FleetServer = z.infer<typeof FleetServerSchema>

export async function parseFleetConfig(filePath: string): Promise<FleetConfig> {
  const content = await fs.readFile(filePath, 'utf-8')
  const raw = yaml.load(content)
  return FleetConfigSchema.parse(raw)
}

export function fleetServerToSshTarget(
  server: FleetServer,
  defaults: Partial<FleetServer> | undefined,
  fleetConfigPath: string
): SshTarget {
  // Merge defaults với server-specific values
  const merged = { ...defaults, ...server }
  return {
    id: `fleet-${server.id}`,   // prefix để tránh collision
    fleetId: server.id,
    fleetConfigSource: fleetConfigPath,
    label: merged.label,
    host: merged.host,
    port: merged.port ?? 22,
    username: merged.username ?? 'dev',
    identityFile: merged.identityFile,
    jumpHost: merged.jumpHost,
    proxyCommand: merged.proxyCommand,
    relayGracePeriodSeconds: merged.relayGracePeriodSeconds ?? 86400,
    project: merged.project,
    team: merged.team,
    environment: merged.environment,
    tags: merged.tags,
    repos: merged.repos,
  }
}
```

### 2.3 Import method trong `SshConnectionStore`

```typescript
// src/main/ssh/ssh-connection-store.ts — ADD METHOD

import { parseFleetConfig, fleetServerToSshTarget } from './fleet-config-parser'

class SshConnectionStore {
  // ... existing methods ...

  async importFromFleetConfig(filePath: string): Promise<FleetImportResult> {
    const config = await parseFleetConfig(filePath)
    const results: FleetImportResult['servers'] = []

    for (const server of config.servers) {
      const target = fleetServerToSshTarget(server, config.defaults, filePath)
      const existing = this.findTargetByFleetId(server.id)

      if (existing) {
        // Re-import: update metadata nhưng không overwrite connection settings
        this.updateTarget(existing.id, {
          project: target.project,
          team: target.team,
          environment: target.environment,
          tags: target.tags,
          repos: target.repos,
          // KHÔNG overwrite: identityFile, port nếu user đã customize
        })
        results.push({ fleetId: server.id, action: 'updated', targetId: existing.id })
      } else {
        // New target
        const created = this.addTarget(target)
        results.push({ fleetId: server.id, action: 'created', targetId: created.id })
      }
    }

    return {
      configPath: filePath,
      serverCount: config.servers.length,
      servers: results,
    }
  }

  async exportToFleetConfig(): Promise<FleetConfig> {
    const targets = this.listTargets()
    const fleetTargets = targets.filter(t => t.fleetId || t.project)
    return {
      version: '1',
      servers: fleetTargets.map(t => ({
        id: t.fleetId ?? t.id,
        label: t.label,
        host: t.host,
        port: t.port,
        username: t.username,
        project: t.project,
        team: t.team,
        environment: t.environment,
        repos: t.repos,
      })),
    }
  }

  private findTargetByFleetId(fleetId: string): SshTarget | undefined {
    return this.listAllTargets().find(t => t.fleetId === fleetId)
  }
}

type FleetImportResult = {
  configPath: string
  serverCount: number
  servers: Array<{
    fleetId: string
    action: 'created' | 'updated' | 'skipped'
    targetId: string
  }>
}
```

### 2.4 IPC Handler mới

```typescript
// src/main/ipc/ssh.ts — ADD HANDLERS

// Handler: ssh.importFleetConfig
ipcMain.handle('ssh:importFleetConfig', async (_event, { filePath }: { filePath: string }) => {
  try {
    const result = await sshConnectionStore.importFromFleetConfig(filePath)
    // Trigger runtime graph sync sau khi import
    scheduleRuntimeGraphSync()
    return { ok: true, result }
  } catch (err) {
    return { ok: false, error: String(err) }
  }
})

// Handler: ssh.exportFleetConfig
ipcMain.handle('ssh:exportFleetConfig', async () => {
  const config = await sshConnectionStore.exportToFleetConfig()
  return { ok: true, config }
})

// Handler: ssh.watchFleetConfig (auto re-import khi file thay đổi)
ipcMain.handle('ssh:watchFleetConfig', async (_event, { filePath }: { filePath: string }) => {
  const watcher = fs.watch(filePath, async () => {
    try {
      await sshConnectionStore.importFromFleetConfig(filePath)
      scheduleRuntimeGraphSync()
      // Notify renderer: fleet config changed
      BrowserWindow.getAllWindows().forEach(win => {
        win.webContents.send('ssh:fleetConfigChanged', { filePath })
      })
    } catch (err) {
      // Log parse error but don't crash
      logger.error('Fleet config watch error', { err, filePath })
    }
  })
  // Cleanup khi window đóng
  return { ok: true }
})
```

---

## 3. Persistence

Orca dùng JSON store (`store.json`) — KHÔNG phải raw SQL migrations.  
Từ `TDD-06 (Persistence)`:

```typescript
// SshTarget mới (backward compatible):
// - Các field cũ: vẫn đọc được
// - Field mới (project, team, ...): undefined nếu chưa có → safe

// Không cần migration script vì JSON store tự handle optional fields
// PersistedState type cần update để TypeScript check ok:
// Vẫn backward compatible vì tất cả new fields đều optional
```

---

## 4. Files cần thay đổi

| File | Action | Chi tiết |
|------|--------|---------|
| `src/shared/ssh-types.ts` | MODIFY | Thêm 5 optional fields |
| `src/main/ssh/fleet-config-parser.ts` | **NEW** | Zod schema + YAML parser |
| `src/main/ssh/ssh-connection-store.ts` | MODIFY | `importFromFleetConfig()`, `exportToFleetConfig()` |
| `src/main/ipc/ssh.ts` | MODIFY | Thêm 3 IPC handlers |
| `package.json` | MODIFY | Verify `js-yaml` dependency (đã có?) |

---

## 5. Testing

```typescript
// src/main/ssh/fleet-config-parser.test.ts — NEW
describe('parseFleetConfig', () => {
  it('parses valid fleet config', async () => {
    const config = await parseFleetConfig('fixtures/orca-fleet.yaml')
    expect(config.version).toBe('1')
    expect(config.servers).toHaveLength(3)
    expect(config.servers[0].project).toBe('vnp-blc')
  })

  it('applies defaults to servers', () => {
    const server = fleetServerToSshTarget(
      { id: 'test', label: 'Test', host: 'test.example.com' },
      { username: 'dev', port: 22, identityFile: '~/.ssh/key' },
      '/path/to/fleet.yaml'
    )
    expect(server.username).toBe('dev')
    expect(server.port).toBe(22)
  })

  it('importFromFleetConfig is idempotent', async () => {
    await store.importFromFleetConfig('fixture.yaml')
    await store.importFromFleetConfig('fixture.yaml')  // second run
    const targets = store.listTargets()
    expect(targets.filter(t => t.fleetId)).toHaveLength(3)  // no duplicates
  })
})
```

---

## 6. Acceptance Criteria Map

| AC (từ CR-001) | Backend implementation |
|---------------|----------------------|
| File `orca-fleet.yaml` parseable bởi Orca | `fleet-config-parser.ts` với Zod validation |
| `importFromFleetConfig()` tạo SshTarget trong store | `ssh-connection-store.ts` method |
| Group/filter theo project, team | CR-002 solution (depends on schema) |
| Re-import không overwrite manual settings | Merge logic giữ identityFile/port nếu đã exist |
| Export fleet ra YAML | `exportToFleetConfig()` method |

---

## 7. Implementation Status

> **✅ IMPLEMENTED — Phase 1 Complete**  
> Ngày: 2026-07-22

### Đã triển khai

| File | Status | Chi tiết |
|------|--------|---------|
| [`src/shared/ssh-types.ts`](../../../../../src/shared/ssh-types.ts) | ✅ Done | `SshTarget` extended: `fleetId`, `project`, `team`, `environment`, `tags`, `repos[]`, `bootstrap{}` |
| [`src/main/ssh/fleet-config-parser.ts`](../../../../../src/main/ssh/fleet-config-parser.ts) | ✅ Done | **NEW** — `parseFleetConfig()` với Zod schema, YAML parsing, validation |
| [`src/main/ssh/ssh-connection-store.ts`](../../../../../src/main/ssh/ssh-connection-store.ts) | ✅ Done | `importFromFleetConfig()`, `exportToFleetConfig()`, `SshTargetFilterCriteria` type |
| [`src/main/ipc/ssh.ts`](../../../../../src/main/ipc/ssh.ts) | ✅ Done | `ssh:importFleetConfig`, `ssh:exportFleetConfig`, `ssh:watchFleetConfig` handlers |
| [`src/main/runtime/rpc/methods/ssh.ts`](../../../../../src/main/runtime/rpc/methods/ssh.ts) | ✅ Done | `ssh.importFleetConfig` RPC method cho CLI |

### Acceptance Criteria — Kết quả

| AC | Trạng thái |
|----|-----------|
| `orca-fleet.yaml` parseable | ✅ `fleet-config-parser.ts` với Zod validation |
| `importFromFleetConfig()` tạo SshTarget | ✅ Implemented in `ssh-connection-store.ts` |
| Group/filter theo project, team | ✅ Implemented in SOL-002 (CR-002) |
| Re-import không overwrite manual settings | ✅ Merge logic: giữ `identityFile`/`port` nếu đã exist |
| Export fleet ra YAML | ✅ `exportToFleetConfig()` method |

### Notes

- **TASK-001** (SshTarget extension): ✅ Done
- **TASK-002** (fleet-config-parser): ✅ Done  
- **TASK-003** (ssh-store import/export): ✅ Done
- **TASK-004** (ssh IPC fleet handlers): ✅ Done
