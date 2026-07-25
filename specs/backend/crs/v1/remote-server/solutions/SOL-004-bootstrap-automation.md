# SOL-004: Dev Server Bootstrap Automation — Backend Solution

**CR:** [CR-004](../../../../../../../docs/crs/v1/remote-server/CR-004-dev-server-bootstrap.md)  
**Backend TDD refs:** `05-ssh-relay.md`, `07-runtime-service.md`, `09-ipc-handlers.md`  
**Effort:** Medium (2–3 ngày)  
**Phase:** 1

---

## 1. Phân tích backend hiện tại

Từ `TDD-05 (SSH Relay)` và code thực:

```typescript
// src/main/ssh/ssh-remote-node-resolution.ts — ĐÃ CÓ
// - Detect Node.js path trên remote
// - Thử: node, nodejs, ~/.nvm, ~/.fnm, ...
// - Nếu không tìm thấy → throw với hướng dẫn install

// src/main/ssh/ssh-remote-node-install-guidance.ts — ĐÃ CÓ
// - Format hướng dẫn install Node.js cho user

// ssh-relay-deploy.ts — ĐÃ CÓ
// - SFTP upload orca-relay binary
// - Start relay process
```

**Gap:**
1. `resolveRemoteNodePath()` chỉ detect, không install
2. Không có `cloneRepo()`, `installGit()` trên remote
3. `orca fleet bootstrap` command chưa tồn tại
4. `orca-fleet.yaml` chưa có `bootstrap` section

---

## 2. Giải pháp backend

### 2.1 SSH Remote Commands module mới

```typescript
// src/main/ssh/ssh-remote-commands.ts — NEW FILE
// Execute remote shell commands qua SSH connection

import type { SshConnection } from './ssh-connection'

export interface RemoteCommandResult {
  stdout: string
  stderr: string
  exitCode: number
}

export async function execRemoteCommand(
  connection: SshConnection,
  command: string,
  options?: { timeout?: number; sudo?: boolean }
): Promise<RemoteCommandResult> {
  const cmd = options?.sudo ? `sudo sh -c '${command}'` : command
  const result = await connection.exec(cmd, { timeout: options?.timeout ?? 30_000 })
  return result
}

// ── Node.js auto-install ─────────────────────────────────────

export async function installNodeJs(
  connection: SshConnection,
  version: string = '22'
): Promise<void> {
  const platform = await detectRemotePlatform(connection)

  let installCmd: string
  switch (platform.distro) {
    case 'ubuntu':
    case 'debian':
      installCmd = [
        `curl -fsSL https://deb.nodesource.com/setup_${version}.x | sudo -E bash -`,
        `sudo apt-get install -y nodejs`,
      ].join(' && ')
      break
    case 'centos':
    case 'rhel':
    case 'fedora':
      installCmd = [
        `curl -fsSL https://rpm.nodesource.com/setup_${version}.x | sudo bash -`,
        `sudo yum install -y nodejs`,
      ].join(' && ')
      break
    case 'alpine':
      installCmd = `sudo apk add --no-cache nodejs npm`
      break
    default:
      // Fallback: nvm
      installCmd = [
        `curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash`,
        `export NVM_DIR="$HOME/.nvm"`,
        `[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"`,
        `nvm install ${version}`,
        `nvm use ${version}`,
      ].join(' && ')
  }

  const result = await execRemoteCommand(connection, installCmd, { timeout: 120_000 })
  if (result.exitCode !== 0) {
    throw new Error(`Node.js install failed: ${result.stderr}`)
  }
}

// ── Git install ──────────────────────────────────────────────

export async function ensureGitInstalled(connection: SshConnection): Promise<void> {
  const check = await execRemoteCommand(connection, 'git --version')
  if (check.exitCode === 0) return  // Already installed

  const platform = await detectRemotePlatform(connection)
  let installCmd: string
  switch (platform.distro) {
    case 'ubuntu': case 'debian':
      installCmd = 'sudo apt-get update && sudo apt-get install -y git'
      break
    case 'centos': case 'rhel':
      installCmd = 'sudo yum install -y git'
      break
    default:
      throw new Error(`Cannot auto-install Git on ${platform.distro}. Please install manually.`)
  }

  const result = await execRemoteCommand(connection, installCmd, { timeout: 60_000 })
  if (result.exitCode !== 0) {
    throw new Error(`Git install failed: ${result.stderr}`)
  }
}

// ── Repo cloning ────────────────────────────────────────────

export async function cloneOrUpdateRepo(
  connection: SshConnection,
  args: {
    url: string
    path: string
    branch?: string
  }
): Promise<'cloned' | 'updated'> {
  // Check nếu path đã tồn tại và là git repo
  const checkResult = await execRemoteCommand(
    connection,
    `test -d "${args.path}/.git" && echo exists || echo missing`
  )

  if (checkResult.stdout.trim() === 'exists') {
    // Update existing
    const branch = args.branch ?? 'HEAD'
    await execRemoteCommand(
      connection,
      `git -C "${args.path}" fetch --all && git -C "${args.path}" checkout ${branch}`,
      { timeout: 60_000 }
    )
    return 'updated'
  } else {
    // Clone new
    await execRemoteCommand(
      connection,
      `mkdir -p "${args.path}" && git clone "${args.url}" "${args.path}"` +
      (args.branch ? ` --branch ${args.branch}` : ''),
      { timeout: 120_000 }
    )
    return 'cloned'
  }
}

// ── Package installation ────────────────────────────────────

export async function installPackages(
  connection: SshConnection,
  packages: string[]
): Promise<void> {
  if (!packages.length) return
  const platform = await detectRemotePlatform(connection)
  let cmd: string
  switch (platform.distro) {
    case 'ubuntu': case 'debian':
      cmd = `sudo apt-get update && sudo apt-get install -y ${packages.join(' ')}`
      break
    case 'centos': case 'rhel':
      cmd = `sudo yum install -y ${packages.join(' ')}`
      break
    case 'alpine':
      cmd = `sudo apk add --no-cache ${packages.join(' ')}`
      break
    default:
      throw new Error(`Package install not supported on ${platform.distro}`)
  }
  const result = await execRemoteCommand(connection, cmd, { timeout: 120_000 })
  if (result.exitCode !== 0) {
    throw new Error(`Package install failed: ${result.stderr}`)
  }
}

// ── Remote setup script ─────────────────────────────────────

export async function runRemoteScript(
  connection: SshConnection,
  script: string,
  cwd?: string
): Promise<void> {
  const cmd = cwd ? `cd "${cwd}" && ${script}` : script
  const result = await execRemoteCommand(connection, cmd, { timeout: 300_000 })  // 5min
  if (result.exitCode !== 0) {
    throw new Error(`Setup script failed:\n${result.stderr}`)
  }
}

// ── Platform detection ──────────────────────────────────────

type RemotePlatform = {
  distro: 'ubuntu' | 'debian' | 'centos' | 'rhel' | 'fedora' | 'alpine' | 'unknown'
  arch: string
}

async function detectRemotePlatform(connection: SshConnection): Promise<RemotePlatform> {
  const result = await execRemoteCommand(
    connection,
    `cat /etc/os-release 2>/dev/null | grep ^ID= | cut -d= -f2 | tr -d '"'`
  )
  const distroRaw = result.stdout.trim().toLowerCase()
  const distroMap: Record<string, RemotePlatform['distro']> = {
    ubuntu: 'ubuntu', debian: 'debian', centos: 'centos',
    rhel: 'rhel', fedora: 'fedora', alpine: 'alpine',
  }
  return {
    distro: distroMap[distroRaw] ?? 'unknown',
    arch: 'x64',
  }
}
```

### 2.2 `OrcaRuntimeService.bootstrapServer()` mới

```typescript
// src/main/runtime/orca-runtime.ts — ADD METHOD

async bootstrapServer(args: {
  targetId: string
  fleetConfig?: FleetConfig
  options?: {
    skipNodeInstall?: boolean
    skipGitInstall?: boolean
    skipRepoClone?: boolean
    skipSetupScript?: boolean
    nodeVersion?: string
    onProgress?: (step: BootstrapStep) => void
  }
}): Promise<BootstrapResult> {
  const target = sshConnectionStore.getTarget(args.targetId)
  if (!target) throw new Error(`SSH target not found: ${args.targetId}`)

  // Lấy server config từ fleet config (nếu có)
  const serverConfig = args.fleetConfig?.servers.find(
    s => s.id === target.fleetId || s.host === target.host
  )
  const globalBootstrap = args.fleetConfig?.bootstrap

  const report: BootstrapStep[] = []
  const notify = (step: BootstrapStep) => {
    report.push(step)
    args.options?.onProgress?.(step)
  }

  // Ensure SSH connected
  const connection = await this.getOrCreateSshConnection(args.targetId)

  // ── Step 1: Node.js ─────────────────────────────────────
  if (!args.options?.skipNodeInstall) {
    notify({ step: 'node-check', status: 'running' })
    try {
      await resolveRemoteNodePath(connection)
      notify({ step: 'node-check', status: 'ok', message: 'Node.js already installed' })
    } catch {
      // Not found → install
      notify({ step: 'node-install', status: 'running' })
      const version = args.options?.nodeVersion ?? globalBootstrap?.nodeVersion ?? '22'
      await installNodeJs(connection, version)
      notify({ step: 'node-install', status: 'ok', message: `Node.js ${version} installed` })
    }
  }

  // ── Step 2: Git ─────────────────────────────────────────
  if (!args.options?.skipGitInstall) {
    notify({ step: 'git-check', status: 'running' })
    await ensureGitInstalled(connection)
    notify({ step: 'git-check', status: 'ok' })
  }

  // ── Step 3: Packages ────────────────────────────────────
  const packages = globalBootstrap?.packages ?? []
  if (packages.length > 0) {
    notify({ step: 'packages', status: 'running', message: packages.join(', ') })
    await installPackages(connection, packages)
    notify({ step: 'packages', status: 'ok' })
  }

  // ── Step 4: Clone/update repos ──────────────────────────
  if (!args.options?.skipRepoClone && serverConfig?.bootstrap?.repos?.length) {
    for (const repo of serverConfig.bootstrap.repos) {
      notify({ step: 'repo-clone', status: 'running', message: repo.path })
      const action = await cloneOrUpdateRepo(connection, repo)
      notify({ step: 'repo-clone', status: 'ok', message: `${repo.path} (${action})` })
    }
  }

  // ── Step 5: Setup script ────────────────────────────────
  if (!args.options?.skipSetupScript && serverConfig?.bootstrap?.setupScript) {
    notify({ step: 'setup-script', status: 'running' })
    await runRemoteScript(connection, serverConfig.bootstrap.setupScript)
    notify({ step: 'setup-script', status: 'ok' })
  }

  // ── Step 6: Verify relay requirements ───────────────────
  notify({ step: 'verify', status: 'running' })
  await resolveRemoteNodePath(connection)  // verify Node.js works
  notify({ step: 'verify', status: 'ok', message: 'Server ready for Orca relay' })

  return { targetId: args.targetId, steps: report, success: true }
}

type BootstrapStep = {
  step: 'node-check' | 'node-install' | 'git-check' | 'packages' | 'repo-clone' | 'setup-script' | 'verify'
  status: 'running' | 'ok' | 'error' | 'skipped'
  message?: string
  error?: string
}

type BootstrapResult = {
  targetId: string
  steps: BootstrapStep[]
  success: boolean
  error?: string
}
```

### 2.3 IPC Handler mới

```typescript
// src/main/ipc/ssh.ts — ADD HANDLER

ipcMain.handle('ssh:bootstrapServer', async (_event, args: {
  targetId: string
  fleetConfigPath?: string
}) => {
  let fleetConfig: FleetConfig | undefined
  if (args.fleetConfigPath) {
    fleetConfig = await parseFleetConfig(args.fleetConfigPath)
  }

  const result = await orcaRuntime.bootstrapServer({
    targetId: args.targetId,
    fleetConfig,
    options: {
      onProgress: (step) => {
        // Notify renderer về progress
        BrowserWindow.getAllWindows().forEach(win => {
          win.webContents.send('ssh:bootstrapProgress', {
            targetId: args.targetId,
            step,
          })
        })
      }
    }
  })
  return { ok: result.success, result }
})
```

### 2.4 CLI Handler: `fleet bootstrap`

```typescript
// src/cli/handlers/fleet.ts — ADD

async function handleFleetBootstrap(args: {
  serverId?: string
  all?: boolean
  configFile?: string
  concurrency?: number
}): Promise<void> {
  const config = args.configFile
    ? await parseFleetConfig(args.configFile)
    : undefined

  // Resolve target list
  let targetIds: string[] = []
  if (args.all) {
    const targets: SshTarget[] = await callRuntimeIpc('ssh:listTargets', {})
    targetIds = targets.map(t => t.id)
  } else if (args.serverId) {
    const targets: SshTarget[] = await callRuntimeIpc('ssh:listTargets', {})
    const found = targets.find(t => t.fleetId === args.serverId || t.id === args.serverId)
    if (!found) throw new Error(`Server not found: ${args.serverId}`)
    targetIds = [found.id]
  }

  const limit = pLimit(args.concurrency ?? 2)

  const tasks = targetIds.map(targetId =>
    limit(async () => {
      console.log(`\n[${targetId}] Starting bootstrap...`)
      const result = await callRuntimeIpc('ssh:bootstrapServer', {
        targetId,
        fleetConfigPath: args.configFile,
      })
      for (const step of result.result.steps) {
        const icon = step.status === 'ok' ? '  ✅' : step.status === 'error' ? '  ❌' : '  ⊙ '
        console.log(`${icon} [${targetId}] ${step.step}${step.message ? `: ${step.message}` : ''}`)
      }
    })
  )

  await Promise.all(tasks)
}
```

---

## 3. Files cần thay đổi

| File | Action | Chi tiết |
|------|--------|---------|
| `src/main/ssh/ssh-remote-commands.ts` | **NEW** | Remote exec, Node.js/Git install, repo clone |
| `src/main/runtime/orca-runtime.ts` | MODIFY | `bootstrapServer()` method |
| `src/main/ipc/ssh.ts` | MODIFY | `ssh:bootstrapServer` IPC handler |
| `src/cli/specs/fleet.ts` | MODIFY | Thêm `fleet bootstrap` spec |
| `src/cli/handlers/fleet.ts` | MODIFY | `handleFleetBootstrap()` |
| `src/shared/fleet-types.ts` | **NEW** | `BootstrapStep`, `BootstrapResult` types |

---

## 4. Rollback Strategy

```
Nếu bất kỳ step nào fail → report lỗi rõ ràng, không rollback (idempotent by design):
- Node.js install fail → log error, không touch Git/repos
- Git clone fail → log error, không xóa đã clone
- Setup script fail → log stderr, không uninstall packages

Chạy lại an toàn vì:
- Node.js: đã install → skip
- Git clone: đã clone → git fetch (update only)
- Packages: apt-get idempotent
```

---

## 5. Implementation Status

> **✅ IMPLEMENTED — Phase 1 Complete**  
> Ngày: 2026-07-22

### Đã triển khai

| File | Status | Deviation từ spec |
|------|--------|-------------------|
| [`src/main/ssh/fleet-remote-commands.ts`](../../../../../src/main/ssh/fleet-remote-commands.ts) | ✅ Done | **NEW** (tên khác `ssh-remote-commands.ts` — đã tồn tại). `detectRemotePlatform()` cache WeakMap, distro-aware install |
| [`src/main/ssh/fleet-bootstrap-service.ts`](../../../../../src/main/ssh/fleet-bootstrap-service.ts) | ✅ Done | **NEW** Standalone service (không trong `orca-runtime.ts`) — tránh modify file 26k lines |
| [`src/main/ipc/ssh.ts`](../../../../../src/main/ipc/ssh.ts) | ✅ Done | `ssh:bootstrapServer` IPC handler + `ssh:bootstrapProgress` streaming event |
| [`src/cli/specs/fleet.ts`](../../../../../src/cli/specs/fleet.ts) | ✅ Done | `fleet bootstrap` spec đã included |
| [`src/cli/handlers/fleet.ts`](../../../../../src/cli/handlers/fleet.ts) | ✅ Done | `fleet bootstrap` handler với concurrency 2 |
| [`src/shared/fleet-types.ts`](../../../../../src/shared/fleet-types.ts) | ✅ Done | **NEW** — `FleetServerStatus`, `FleetStatusReport` (bootstrap types nằm trong `fleet-bootstrap-service.ts`) |
| [`src/main/runtime/rpc/methods/ssh.ts`](../../../../../src/main/runtime/rpc/methods/ssh.ts) | ✅ Done | `ssh.bootstrapServer` RPC method cho CLI |

### Bootstrap Steps đã implement

```
1. node-check    → resolveRemoteNodePath()
2. node-install  → installNodeJs() (Ubuntu/Debian/CentOS/Alpine/nvm fallback)
3. git-check     → ensureGitInstalled()
4. packages      → installPackages() (OS packages từ fleet config)
5. repo-clone    → cloneOrUpdateRepo() (idempotent)
6. setup-script  → runRemoteScript()
7. verify        → resolveRemoteNodePath() final check
```

### Deviation từ design gốc

> `bootstrapServer()` là standalone service (`fleet-bootstrap-service.ts`) thay vì method trong `OrcaRuntimeService`. Vẫn access được đầy đủ qua `getSshConnectionManager()` / `getSshConnectionStore()` exports.

### Notes

- **TASK-009** (fleet-remote-commands): ✅ Done  
- **TASK-010** (bootstrapServer method): ✅ Done  
- **TASK-011** (ssh:bootstrapServer IPC): ✅ Done
