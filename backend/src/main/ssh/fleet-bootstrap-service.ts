// src/main/ssh/fleet-bootstrap-service.ts
// Orchestrates dev-server bootstrap: Node.js, Git, packages, repos, setup script.
// Designed as a standalone service that references the SSH IPC layer — avoids
// touching the 26k-line orca-runtime.ts directly.
import {
  connectRegisteredSshTarget,
  getSshConnectionManager,
  getSshConnectionStore,
} from '../ipc/ssh'
import { resolveRemoteNodePath } from './ssh-remote-node-resolution'
import {
  installNodeJs,
  ensureGitInstalled,
  cloneOrUpdateRepo,
  installPackages,
  runRemoteScript,
  checkRemoteDiskSpace,       // FIX BUG-BE-HLD-013
  MIN_BOOTSTRAP_DISK_SPACE_GB, // FIX BUG-BE-HLD-013
} from './fleet-remote-commands'
import { deployAndLaunchRelay } from './ssh-relay-deploy' // FIX BUG-BE-HLD-013
import { parseFleetConfig } from '../../shared/fleet-config-parser'
import type { FleetConfig } from '../../shared/fleet-config-parser'

import type { BootstrapStepName, BootstrapStep, BootstrapResult } from '../../shared/fleet-types'

export type BootstrapOptions = {
  /** Path to orca-fleet.yaml to load server-specific bootstrap config */
  fleetConfigPath?: string
  /** Skip Node.js check/install (useful when already provisioned) */
  skipNodeInstall?: boolean
  /** Skip Git check/install */
  skipGitInstall?: boolean
  /** FIX BUG-BE-HLD-013 — skip the ≥5GB free-space check. */
  skipDiskCheck?: boolean
  /** FIX BUG-BE-HLD-013 — minimum free disk space required, in GB. Default 5. */
  minDiskSpaceGb?: number
  /** FIX BUG-BE-HLD-013 — skip installing/verifying/starting the relay binary. */
  skipRelayDeploy?: boolean
  /** Skip repo clone/update */
  skipRepoClone?: boolean
  /** Skip running the setupScript defined in fleet config */
  skipSetupScript?: boolean
  /** Node.js version to install if missing. Defaults to '22'. */
  nodeVersion?: string
  /** Called for every step transition — for streaming progress to UI */
  onProgress?: (step: BootstrapStep) => void
}

// ── Bootstrap Service ──────────────────────────────────────────

/**
 * Bootstrap a dev server: ensure Node.js, Git, OS packages, repos, and setup
 * script are configured. Idempotent — safe to run multiple times on the same
 * server.
 *
 * @param targetId - SshTarget ID (must already be in store)
 * @param options  - Bootstrap configuration
 */
export async function bootstrapServer(
  targetId: string,
  options: BootstrapOptions = {}
): Promise<BootstrapResult> {
  const report: BootstrapStep[] = []

  const notify = (step: BootstrapStep): void => {
    report.push(step)
    options.onProgress?.(step)
  }

  try {
    // ── Load fleet config if specified ─────────────────────────
    let fleetConfig: FleetConfig | undefined
    if (options.fleetConfigPath) {
      fleetConfig = await parseFleetConfig(options.fleetConfigPath)
    }

    // ── Resolve target ─────────────────────────────────────────
    const store = getSshConnectionStore()
    if (!store) {throw new Error('SSH store not initialized')}

    const target = store.getTarget(targetId)
    if (!target) {throw new Error(`SSH target not found: ${targetId}`)}

    // Find matching fleet config server entry
    const serverConfig = fleetConfig?.servers.find(
      (s) => s.id === target.fleetId || s.host === target.host
    )
    const globalBootstrap = fleetConfig?.bootstrap

    // ── Ensure connected ───────────────────────────────────────
    // connectRegisteredSshTarget is idempotent — returns existing state if live
    const connectionState = await connectRegisteredSshTarget(targetId)
    if (connectionState.status !== 'connected') {
      throw new Error(
        `Cannot bootstrap: target "${target.label}" is not connected (status: ${connectionState.status})`
      )
    }

    // Get the SshConnection instance for executing commands
    const manager = getSshConnectionManager()
    if (!manager) {throw new Error('SSH connection manager not initialized')}

    const conn = manager.getConnection(targetId)
    if (!conn) {
      throw new Error(
        `SSH connection instance not found for target "${target.label}". Try reconnecting.`
      )
    }

    // ── Step 1: Node.js check & install ───────────────────────
    if (!options.skipNodeInstall) {
      notify({ step: 'node-check', status: 'running' })
      try {
        await resolveRemoteNodePath(conn)
        notify({ step: 'node-check', status: 'ok', message: 'Node.js already installed' })
      } catch {
        // Not found — install
        notify({ step: 'node-install', status: 'running' })
        const version = options.nodeVersion ?? globalBootstrap?.nodeVersion ?? '22'
        try {
          await installNodeJs(conn, version)
          notify({ step: 'node-install', status: 'ok', message: `Node.js ${version} installed` })
        } catch (err) {
          notify({ step: 'node-install', status: 'error', error: String(err) })
          throw err
        }
      }
    } else {
      notify({ step: 'node-check', status: 'skipped' })
    }

    // ── Step 2: Git check & install ────────────────────────────
    if (!options.skipGitInstall) {
      notify({ step: 'git-check', status: 'running' })
      try {
        await ensureGitInstalled(conn)
        notify({ step: 'git-check', status: 'ok', message: 'Git available' })
      } catch (err) {
        notify({ step: 'git-check', status: 'error', error: String(err) })
        throw err
      }
    } else {
      notify({ step: 'git-check', status: 'skipped' })
    }

    // ── Step 2.5: Disk space check ──────────────────────────────
    // FIX BUG-BE-HLD-013: fail fast before any install/clone work — a
    // server that's nearly out of disk should never reach npm install or
    // git clone, where it fails halfway through with a confusing error.
    if (!options.skipDiskCheck) {
      notify({ step: 'disk-check', status: 'running' })
      try {
        const minGb = options.minDiskSpaceGb ?? MIN_BOOTSTRAP_DISK_SPACE_GB
        const disk = await checkRemoteDiskSpace(conn, minGb)
        if (!disk.ok) {
          const msg = `Insufficient disk space: ${disk.availableGb.toFixed(1)}GB available, need >= ${minGb}GB`
          notify({ step: 'disk-check', status: 'error', error: msg })
          throw new Error(msg)
        }
        notify({ step: 'disk-check', status: 'ok', message: `${disk.availableGb.toFixed(1)}GB available` })
      } catch (err) {
        notify({ step: 'disk-check', status: 'error', error: String(err) })
        throw err
      }
    } else {
      notify({ step: 'disk-check', status: 'skipped' })
    }

    // ── Step 3: OS packages ────────────────────────────────────
    const packages = globalBootstrap?.packages ?? []
    if (packages.length > 0) {
      notify({ step: 'packages', status: 'running', message: packages.join(', ') })
      try {
        await installPackages(conn, packages)
        notify({ step: 'packages', status: 'ok' })
      } catch (err) {
        notify({ step: 'packages', status: 'error', error: String(err) })
        throw err
      }
    } else {
      notify({ step: 'packages', status: 'skipped' })
    }

    // ── Step 3.5: Relay deploy (install + SHA256 verify + start) ──
    // FIX BUG-BE-HLD-013: was a disconnected flow (ssh-relay-deploy.ts only
    // ran on-demand at connect time), never part of bootstrap — two flows
    // that could desync (bootstrap "succeeds" but relay never verified/started).
    if (!options.skipRelayDeploy) {
      notify({ step: 'relay-deploy', status: 'running' })
      try {
        await deployAndLaunchRelay(conn, (status) => {
          notify({ step: 'relay-deploy', status: 'running', message: status })
        })
        notify({ step: 'relay-deploy', status: 'ok', message: 'Relay installed, SHA256-verified, started' })
      } catch (err) {
        notify({ step: 'relay-deploy', status: 'error', error: String(err) })
        throw err
      }
    } else {
      notify({ step: 'relay-deploy', status: 'skipped' })
    }

    // ── Step 4: Clone / update repos ──────────────────────────
    const repos = serverConfig?.bootstrap?.repos ?? []
    if (!options.skipRepoClone && repos.length > 0) {
      for (const repo of repos) {
        notify({ step: 'repo-clone', status: 'running', message: repo.path })
        try {
          const action = await cloneOrUpdateRepo(conn, {
            url: repo.url,
            path: repo.path,
            branch: repo.branch,
          })
          notify({
            step: 'repo-clone',
            status: 'ok',
            message: `${repo.path} (${action})`,
          })
        } catch (err) {
          notify({
            step: 'repo-clone',
            status: 'error',
            error: String(err),
            message: repo.path,
          })
          throw err
        }
      }
    } else {
      notify({ step: 'repo-clone', status: 'skipped' })
    }

    // ── Step 5: Setup script ───────────────────────────────────
    const setupScript = serverConfig?.bootstrap?.setupScript
    if (!options.skipSetupScript && setupScript) {
      notify({ step: 'setup-script', status: 'running' })
      try {
        await runRemoteScript(conn, setupScript)
        notify({ step: 'setup-script', status: 'ok' })
      } catch (err) {
        notify({ step: 'setup-script', status: 'error', error: String(err) })
        throw err
      }
    } else {
      notify({ step: 'setup-script', status: 'skipped' })
    }

    // ── Step 6: Final verify ───────────────────────────────────
    notify({ step: 'verify', status: 'running' })
    try {
      const nodePath = await resolveRemoteNodePath(conn)
      notify({ step: 'verify', status: 'ok', message: `Server ready. Node: ${nodePath}` })
    } catch (err) {
      notify({ step: 'verify', status: 'error', error: String(err) })
      throw err
    }

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
