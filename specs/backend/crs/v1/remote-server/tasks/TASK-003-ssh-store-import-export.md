# TASK-003: Thêm `importFromFleetConfig()` và `exportToFleetConfig()` vào `SshConnectionStore`

**Source:** SOL-001  
**Phase:** 1 | **Effort:** S (30–90 min)  
**Depends on:** TASK-002

---

## Objective

Thêm 3 methods vào class `SshConnectionStore` trong `src/main/ssh/ssh-connection-store.ts`:
1. `importFromFleetConfig(filePath)` — parse YAML + upsert targets
2. `exportToFleetConfig()` — serialize current targets to FleetConfig
3. `findTargetByFleetId(fleetId)` — private lookup helper

---

## File to modify

**`src/main/ssh/ssh-connection-store.ts`**

---

## Implementation

### Step 1: Add import at top of file

```typescript
import {
  parseFleetConfig,
  fleetServerToSshTarget,
  sshTargetsToFleetConfig,
  type FleetConfig,
} from './fleet-config-parser'
```

### Step 2: Add these methods to the `SshConnectionStore` class

```typescript
  // ── Fleet Config Import / Export (NEW) ─────────────────────

  /**
   * Import servers from a fleet config YAML file.
   * - Creates new SshTargets for servers not yet in store
   * - Updates metadata (project/team/etc.) for existing targets (matched by fleetId)
   * - Does NOT overwrite connection settings (identityFile, port) if user customized them
   * - Idempotent: safe to run multiple times
   */
  async importFromFleetConfig(filePath: string): Promise<FleetImportResult> {
    const config = await parseFleetConfig(filePath)
    const results: FleetImportResult['servers'] = []

    for (const server of config.servers) {
      const newTarget = fleetServerToSshTarget(server, config.defaults, filePath)
      const existing = this.findTargetByFleetId(server.id)

      if (existing) {
        // Update fleet metadata only — preserve connection settings
        this.updateTarget(existing.id, {
          project: newTarget.project,
          team: newTarget.team,
          environment: newTarget.environment,
          tags: newTarget.tags,
          repos: newTarget.repos,
          fleetConfigSource: filePath,
        })
        results.push({ fleetId: server.id, action: 'updated', targetId: existing.id })
      } else {
        // Create new target
        const created = this.addTarget(newTarget)
        results.push({ fleetId: server.id, action: 'created', targetId: created.id })
      }
    }

    return {
      configPath: filePath,
      serverCount: config.servers.length,
      servers: results,
    }
  }

  /**
   * Export fleet-aware targets to FleetConfig format.
   * Includes targets that have fleetId or project metadata.
   */
  exportToFleetConfig(): FleetConfig {
    const targets = this.listTargets()
    return sshTargetsToFleetConfig(targets)
  }

  /**
   * Find target by fleet config stable ID.
   * @private
   */
  private findTargetByFleetId(fleetId: string): SshTarget | undefined {
    return this.listAllTargets().find(t => t.fleetId === fleetId)
  }
```

### Step 3: Add `FleetImportResult` type (at top-level of file, not inside class)

```typescript
export type FleetImportResult = {
  configPath: string
  serverCount: number
  servers: Array<{
    fleetId: string
    action: 'created' | 'updated' | 'skipped'
    targetId: string
    error?: string
  }>
}
```

---

## Notes for AI

- `this.listAllTargets()` — use the method that returns ALL targets including runtime-owned ones (for fleet ID lookup). If only `listTargets()` exists (non-runtime), use that.
- `this.addTarget()` / `this.updateTarget()` — use the existing store methods. Check their signatures first.
- The `updateTarget()` call should do a **partial update** (merge), not full replace.

---

## Verification

```bash
npx tsc --noEmit 2>&1 | grep ssh-connection-store | head -20
```

---

## Done criteria

- [x] `importFromFleetConfig(filePath: string): Promise<FleetImportResult>` exists
- [x] `exportToFleetConfig(): FleetConfig` exists
- [x] `FleetImportResult` type exported
- [x] Import is idempotent (second call updates, doesn't duplicate)
- [x] TypeScript compile: no errors

**Status: ✅ DONE** — `ssh-connection-store.ts` updated. `importFromFleetConfig` upserts by `fleetId`, `exportToFleetConfig` serializes fleet-aware targets. `FleetImportResult` type exported.
