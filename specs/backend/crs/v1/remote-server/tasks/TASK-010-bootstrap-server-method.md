# TASK-010: Thêm `bootstrapServer()` vào `OrcaRuntimeService`

**Source:** SOL-004  
**Phase:** 1 | **Effort:** M (1.5–3 giờ)  
**Depends on:** TASK-009

---

## Objective

Thêm method `bootstrapServer()` vào `OrcaRuntimeService` (hoặc tương đương trong `src/main/runtime/orca-runtime.ts`). Method này orchestrate toàn bộ bootstrap flow:
1. Check/install Node.js
2. Check/install Git
3. Install OS packages (từ fleet config)
4. Clone/update repos (từ fleet config)
5. Run setup script (từ fleet config)
6. Verify relay requirements

---

## File to modify

**`src/main/runtime/orca-runtime.ts`**

---

## Step 1: Understand existing SSH connection access

Đọc cách `orca-runtime.ts` hiện tại access SSH connections. Tìm:
- Có `sshManager` hoặc `sshConnectionManager` reference?
- Method `getConnection(targetId)` hoặc tương tự?
- Cách resolve `SshConnection` instance từ `targetId`

---

## Implementation

### Step A: Add imports

```typescript
import {
  execRemoteCommand,
  detectRemotePlatform,
  installNodeJs,
  ensureGitInstalled,
  cloneOrUpdateRepo,
  installPackages,
  runRemoteScript,
} from '../ssh/ssh-remote-commands'
import { parseFleetConfig } from '../ssh/fleet-config-parser'
import type { FleetConfig } from '../ssh/fleet-config-parser'
```

### Step B: Add types (top-level or in shared types file)

```typescript
// Can also go in src/shared/fleet-types.ts (NEW file)
export type BootstrapStepName =
  | 'node-check'
  | 'node-install'
  | 'git-check'
  | 'packages'
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

export type BootstrapOptions = {
  fleetConfigPath?: string
  skipNodeInstall?: boolean
  skipGitInstall?: boolean
  skipRepoClone?: boolean
  skipSetupScript?: boolean
  nodeVersion?: string
  onProgress?: (step: BootstrapStep) => void
}
```

### Step C: Add method to OrcaRuntimeService class

```typescript
  /**
   * Bootstrap a dev server: install Node.js, Git, clone repos, run setup script.
   * Idempotent: safe to run multiple times.
   */
  async bootstrapServer(targetId: string, options: BootstrapOptions = {}): Promise<BootstrapResult> {
    const report: BootstrapStep[] = []

    const notify = (step: BootstrapStep): void => {
      report.push(step)
      options.onProgress?.(step)
    }

    try {
      // Load fleet config if provided
      let fleetConfig: FleetConfig | undefined
      if (options.fleetConfigPath) {
        fleetConfig = await parseFleetConfig(options.fleetConfigPath)
      }

      // Find target and its fleet config entry
      const target = sshConnectionStore.getTarget(targetId)
      if (!target) throw new Error(`SSH target not found: ${targetId}`)

      const serverConfig = fleetConfig?.servers.find(
        s => s.id === target.fleetId || s.host === target.host
      )
      const globalBootstrap = fleetConfig?.bootstrap

      // Get SSH connection (must already be connected or connectable)
      // ADAPT: use the actual method to get/create SshConnection
      const connection = await this.getOrCreateSshConnection(targetId)

      // ── Step 1: Node.js check & install ─────────────────────
      if (!options.skipNodeInstall) {
        notify({ step: 'node-check', status: 'running' })
        try {
          // Use existing resolveRemoteNodePath if available
          await resolveRemoteNodePath(connection)
          notify({ step: 'node-check', status: 'ok', message: 'Node.js already installed' })
        } catch {
          notify({ step: 'node-install', status: 'running' })
          const version = options.nodeVersion ?? globalBootstrap?.nodeVersion ?? '22'
          try {
            await installNodeJs(connection, version)
            notify({ step: 'node-install', status: 'ok', message: `Node.js ${version} installed` })
          } catch (err) {
            notify({ step: 'node-install', status: 'error', error: String(err) })
            throw err
          }
        }
      } else {
        notify({ step: 'node-check', status: 'skipped' })
      }

      // ── Step 2: Git check & install ──────────────────────────
      if (!options.skipGitInstall) {
        notify({ step: 'git-check', status: 'running' })
        try {
          await ensureGitInstalled(connection)
          notify({ step: 'git-check', status: 'ok' })
        } catch (err) {
          notify({ step: 'git-check', status: 'error', error: String(err) })
          throw err
        }
      } else {
        notify({ step: 'git-check', status: 'skipped' })
      }

      // ── Step 3: OS packages ──────────────────────────────────
      const packages = globalBootstrap?.packages ?? []
      if (packages.length > 0) {
        notify({ step: 'packages', status: 'running', message: packages.join(', ') })
        try {
          await installPackages(connection, packages)
          notify({ step: 'packages', status: 'ok' })
        } catch (err) {
          notify({ step: 'packages', status: 'error', error: String(err) })
          throw err
        }
      } else {
        notify({ step: 'packages', status: 'skipped' })
      }

      // ── Step 4: Clone/update repos ───────────────────────────
      if (!options.skipRepoClone && serverConfig?.bootstrap?.repos?.length) {
        for (const repo of serverConfig.bootstrap.repos) {
          notify({ step: 'repo-clone', status: 'running', message: repo.path })
          try {
            const action = await cloneOrUpdateRepo(connection, repo)
            notify({ step: 'repo-clone', status: 'ok', message: `${repo.path} (${action})` })
          } catch (err) {
            notify({ step: 'repo-clone', status: 'error', error: String(err), message: repo.path })
            throw err
          }
        }
      } else {
        notify({ step: 'repo-clone', status: 'skipped' })
      }

      // ── Step 5: Setup script ─────────────────────────────────
      if (!options.skipSetupScript && serverConfig?.bootstrap?.setupScript) {
        notify({ step: 'setup-script', status: 'running' })
        try {
          await runRemoteScript(connection, serverConfig.bootstrap.setupScript)
          notify({ step: 'setup-script', status: 'ok' })
        } catch (err) {
          notify({ step: 'setup-script', status: 'error', error: String(err) })
          throw err
        }
      } else {
        notify({ step: 'setup-script', status: 'skipped' })
      }

      // ── Step 6: Final verify ─────────────────────────────────
      notify({ step: 'verify', status: 'running' })
      await resolveRemoteNodePath(connection)
      notify({ step: 'verify', status: 'ok', message: 'Server ready for Orca relay' })

      return { targetId, steps: report, success: true }

    } catch (err) {
      return {
        targetId,
        steps: report,
        success: false,
        error: err instanceof Error ? err.message : String(err),
      }
    }
  }
```

---

## Notes for AI

1. `this.getOrCreateSshConnection(targetId)` — implement using existing SSH connection management. Look for how the runtime gets SshConnection instances in other methods.
2. `resolveRemoteNodePath(connection)` — import from `src/main/ssh/ssh-remote-node-resolution.ts` (already exists).
3. If `sshConnectionStore.getTarget(id)` doesn't exist, use `sshConnectionStore.listTargets().find(t => t.id === id)`.

---

## Verification

```bash
npx tsc --noEmit 2>&1 | grep orca-runtime | head -20
```

---

## Done criteria

- [x] `bootstrapServer(targetId, options)` method exists in standalone service
- [x] Returns `BootstrapResult` with steps array
- [x] All 6 steps implemented (node, git, packages, repos, script, verify)
- [x] `onProgress` callback called for each step transition
- [x] Method is idempotent (skip already-done steps)
- [x] TypeScript compile: no errors

**Status: ✅ DONE** — Created `src/main/ssh/fleet-bootstrap-service.ts`. Implemented as standalone service (not in orca-runtime.ts) using `getSshConnectionManager()` + `getSshConnectionStore()` exports from `ipc/ssh.ts`. `BootstrapStepName`, `BootstrapStep`, `BootstrapResult`, `BootstrapOptions` types exported.
