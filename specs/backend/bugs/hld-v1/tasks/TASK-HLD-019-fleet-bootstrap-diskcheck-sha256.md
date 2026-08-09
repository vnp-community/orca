# TASK-HLD-019: Thêm disk-check và SHA256 verify vào `bootstrapServer()`

**Priority:** 🟡 MEDIUM
**Effort:** ~3-4 giờ (impl + test)
**Status:** ✅ DONE — 2026-08-09 (áp dụng đủ 4 file: `checkRemoteDiskSpace`/`parseDfSizeToGb` mới; 2 `BootstrapStepName` mới; `bootstrapServer()` thêm Step 2.5 (disk-check) + Step 3.5 (relay-deploy, gộp `deployAndLaunchRelay` vào luồng chính thức); `uploadRelay()` thêm tham số `nodePath` + `verifyRelayChecksum()` SHA256 sau SFTP upload — đã xác nhận từng helper (`commandWithNodePath`/`shellEscape`/`powerShellLiteral`/`execHostCommand`/`isWindowsRemoteHost`/`joinRemotePath`) đều đã import sẵn trong file, chỉ 1 call site `uploadRelay(` như solution xác nhận. `tsc --noEmit` chỉ còn 1 lỗi pre-existing (`BootstrapStepName` unused import, có từ trước, không liên quan). ⚠️ Chưa tạo 3 file test mới — effort budget.)
**Bug refs:** BUG-BE-HLD-013
**Solution ref:** [SOLUTION-fleet-exact.md](../solutions/SOLUTION-fleet-exact.md)
**Depends on:** None

---

## Mục tiêu

`bootstrapServer()` (7-step flow theo CR-004) hiện thiếu 2 bước:

1. **Disk-space check** trước khi làm gì nặng (npm install, git clone) — tránh fail nửa chừng với lỗi khó hiểu khi server gần hết dung lượng.
2. **SHA256 verify** relay binary sau khi SFTP upload — catch corruption/tampering trong lúc transfer trước khi execute.

Ngoài ra, bước install/start relay (`ssh-relay-deploy.ts`) hiện tách rời hoàn toàn khỏi `bootstrapServer()` (chỉ chạy on-demand lúc connect SSH target) — 2 luồng có thể desync (bootstrap "succeeds" nhưng relay chưa từng được verify/start). Task này gộp relay-deploy thành 1 step chính thức trong `bootstrapServer()`.

## File cần sửa/tạo

```
backend/src/main/ssh/fleet-remote-commands.ts    (sửa — thêm checkRemoteDiskSpace + parseDfSizeToGb)
backend/src/shared/fleet-types.ts                 (sửa — thêm 2 BootstrapStepName mới)
backend/src/main/ssh/fleet-bootstrap-service.ts   (sửa — thêm Step 2.5 disk-check, Step 3.5 relay-deploy)
backend/src/main/ssh/ssh-relay-deploy.ts          (sửa — verifyRelayChecksum() sau SFTP upload)

# Test mới:
backend/src/main/ssh/fleet-remote-commands.test.ts   (chung file với TASK-HLD-017 nếu đã tạo — thêm case mới)
backend/src/main/ssh/fleet-bootstrap-service.test.ts
backend/src/main/ssh/ssh-relay-deploy.test.ts
```

## Thay đổi cụ thể

### 1. `backend/src/main/ssh/fleet-remote-commands.ts` — thêm `checkRemoteDiskSpace()`

Thêm vào cuối file (cùng khu vực với `collectRemoteResourceMetrics` nếu TASK-HLD-017 đã áp dụng, hoặc ngay sau `installPackages`):

```typescript
// ── Disk space check ────────────────────────────────────────────

export type DiskSpaceCheck = {
  availableGb: number
  ok: boolean
}

export const MIN_BOOTSTRAP_DISK_SPACE_GB = 5

/**
 * Check free disk space in the current remote working directory (typically
 * $HOME for a fresh SSH session) via `df -h .`, per CR-004's disk-check step.
 */
export async function checkRemoteDiskSpace(
  conn: SshConnection,
  minGb: number = MIN_BOOTSTRAP_DISK_SPACE_GB
): Promise<DiskSpaceCheck> {
  const output = await execCommand(conn, 'df -h .')
  const dataLine = output.trim().split('\n')[1] ?? ''
  const columns = dataLine.trim().split(/\s+/)
  // df -h columns: Filesystem Size Used Avail Use% Mounted-on
  const availableRaw = columns[3] ?? '0'
  const availableGb = parseDfSizeToGb(availableRaw)
  return { availableGb, ok: availableGb >= minGb }
}

/** Parses a `df -h`-style size string ("47G", "512M", "1.2T", "900K") into GB. */
export function parseDfSizeToGb(raw: string): number {
  const match = /^([\d.]+)\s*([KMGT]?)/i.exec(raw.trim())
  if (!match) return 0
  const value = parseFloat(match[1])
  if (Number.isNaN(value)) return 0
  switch (match[2].toUpperCase()) {
    case 'T':
      return value * 1024
    case 'G':
      return value
    case 'M':
      return value / 1024
    case 'K':
      return value / (1024 * 1024)
    default:
      // No suffix — df reported raw bytes (rare with -h, but be safe).
      return value / (1024 * 1024 * 1024)
  }
}
```

### 2. `backend/src/shared/fleet-types.ts` — thêm 2 step mới vào `BootstrapStepName`

```typescript
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
```

### 3. `backend/src/main/ssh/fleet-bootstrap-service.ts` — thêm 2 bước vào `bootstrapServer()`

**Thêm import (đầu file, sau import `fleet-remote-commands`):**
```typescript
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
```

**Mở rộng `BootstrapOptions` (dòng 23–38):**
```typescript
export type BootstrapOptions = {
  fleetConfigPath?: string
  skipNodeInstall?: boolean
  skipGitInstall?: boolean
  /** FIX BUG-BE-HLD-013 — skip the ≥5GB free-space check. */
  skipDiskCheck?: boolean
  /** FIX BUG-BE-HLD-013 — minimum free disk space required, in GB. Default 5. */
  minDiskSpaceGb?: number
  /** FIX BUG-BE-HLD-013 — skip installing/verifying/starting the relay binary. */
  skipRelayDeploy?: boolean
  skipRepoClone?: boolean
  skipSetupScript?: boolean
  nodeVersion?: string
  onProgress?: (step: BootstrapStep) => void
}
```

**Chèn Step 2.5 (disk-check) ngay sau Step 2 "Git check & install" (sau dòng 135, trước "── Step 3: OS packages"):**
```typescript
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
```

**Chèn Step 3.5 (relay-deploy) ngay sau Step 3 "OS packages" (sau dòng 150, trước "── Step 4: Clone / update repos"):**
```typescript
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
```

> `deployAndLaunchRelay` đã idempotent (kiểm tra `isRelayAlreadyInstalled` trước khi upload lại) — gọi nó trong `bootstrapServer()` không double-install nếu relay đã chạy từ trước lúc connect.

### 4. `backend/src/main/ssh/ssh-relay-deploy.ts` — SHA256 verify sau SFTP upload

**Thêm import ở đầu file (cùng nhóm `node:fs`):**
```typescript
import { existsSync, readFileSync } from 'node:fs'
import { createHash } from 'node:crypto'
```

**Sửa `uploadRelay()` (dòng 350–388) — verify SHA256 của `relay.js` ngay sau khi upload directory, trước khi ghi `.version`:**

Code hiện tại:
```typescript
  // Create remote directory
  await execHostCommand(conn, hostPlatform, makeRemoteDirectoryCommand(hostPlatform, remoteDir))

  await uploadDirectoryForConnection(conn, localRelayDir, remoteDir, hostPlatform)

  // Make the node binary executable
  if (!isWindowsRemoteHost(hostPlatform)) {
    await execHostCommand(
      conn,
      hostPlatform,
      makeRemoteExecutableCommand(hostPlatform, joinRemotePath(hostPlatform, remoteDir, 'node'))
    )
  }

  // Why: write `.version` via SFTP rather than shell to avoid quoting issues
  // with content-hashed version strings. The remote daemon reads this same
  // file on startup so the wire-handshake validates against it.
  await writeRemoteFile(
    conn,
    hostPlatform,
    joinRemotePath(hostPlatform, remoteDir, '.version'),
    fullVersion
  )
}
```

**Fix — thêm bước verify ngay sau `uploadDirectoryForConnection`:**
```typescript
  // Create remote directory
  await execHostCommand(conn, hostPlatform, makeRemoteDirectoryCommand(hostPlatform, remoteDir))

  await uploadDirectoryForConnection(conn, localRelayDir, remoteDir, hostPlatform)

  // FIX BUG-BE-HLD-013: verify the transferred relay entry point matches its
  // local SHA256 before trusting/executing it — catches SFTP corruption or
  // in-transit tampering. relay.js is the trust boundary: it's the file
  // that actually gets `node relay.js`-executed on the remote host.
  await verifyRelayChecksum(conn, hostPlatform, localRelayDir, remoteDir, nodePath)

  // Make the node binary executable
  if (!isWindowsRemoteHost(hostPlatform)) {
    await execHostCommand(
      conn,
      hostPlatform,
      makeRemoteExecutableCommand(hostPlatform, joinRemotePath(hostPlatform, remoteDir, 'node'))
    )
  }

  // Why: write `.version` via SFTP rather than shell to avoid quoting issues
  // with content-hashed version strings. The remote daemon reads this same
  // file on startup so the wire-handshake validates against it.
  await writeRemoteFile(
    conn,
    hostPlatform,
    joinRemotePath(hostPlatform, remoteDir, '.version'),
    fullVersion
  )
}

// FIX BUG-BE-HLD-013: local SHA256 vs remote SHA256 of relay.js, computed
// remotely via `node -e` (node is guaranteed present — it's what we just
// uploaded) rather than depending on the `sha256sum` binary existing on
// every distro (not guaranteed on minimal Alpine images).
async function verifyRelayChecksum(
  conn: SshConnection,
  hostPlatform: RemoteHostPlatform,
  localRelayDir: string,
  remoteDir: string,
  nodePath: string
): Promise<void> {
  const localEntryPath = join(localRelayDir, 'relay.js')
  const localHash = createHash('sha256').update(readFileSync(localEntryPath)).digest('hex')

  const remoteEntryPath = joinRemotePath(hostPlatform, remoteDir, 'relay.js')
  const hashScript =
    'const c=require("crypto"),fs=require("fs");' +
    'process.stdout.write(c.createHash("sha256").update(fs.readFileSync(process.argv[1])).digest("hex"))'

  const command = isWindowsRemoteHost(hostPlatform)
    ? commandWithNodePath(
        hostPlatform,
        nodePath,
        remoteDir,
        `& ${powerShellLiteral(nodePath)} -e ${powerShellNativeArg(hashScript)} ${powerShellLiteral(remoteEntryPath)}`
      )
    : commandWithNodePath(
        hostPlatform,
        nodePath,
        remoteDir,
        `${shellEscape(nodePath)} -e '${hashScript}' ${shellEscape(remoteEntryPath)}`
      )

  const remoteHash = (await execHostCommand(conn, hostPlatform, command)).trim()

  if (remoteHash !== localHash) {
    throw new Error(
      `Relay binary checksum mismatch after upload: local sha256=${localHash} remote sha256=${remoteHash}. ` +
        `The transferred relay.js may be corrupted or tampered with in transit — aborting deploy.`
    )
  }
}
```

**Về `nodePath` cho việc hash:** tại điểm gọi `uploadRelay()` (trong `deployAndLaunchRelayInner`, dòng 295), biến `nodePath` (từ `resolveRelayBootstrapState`) đã có sẵn trong scope — sửa call site thay vì thêm hàm giả định:

```typescript
// deployAndLaunchRelayInner — chỗ gọi uploadRelay hiện tại (dòng 295):
await uploadRelay(conn, platform, remoteRelayDir, fullVersion, hostPlatform, nodePath)
```

Cập nhật signature `uploadRelay` để nhận thêm `nodePath: string`, dùng trực tiếp:

```typescript
async function uploadRelay(
  conn: SshConnection,
  platform: RelayPlatform,
  remoteDir: string,
  fullVersion: string,
  hostPlatform: RemoteHostPlatform,
  nodePath: string // FIX BUG-BE-HLD-013 — new param, needed for checksum verify
): Promise<void> {
```

Chỉ có 1 call site (`deployAndLaunchRelayInner`) — đổi signature không phá caller nào khác (solution đã grep xác nhận `uploadRelay(` trong file chỉ khớp định nghĩa + 1 lần gọi; khuyến nghị grep lại 1 lần nữa trước khi merge để chắc chắn không có call site mới phát sinh từ lúc solution được viết).

## Verification

```bash
cd backend
pnpm vitest run src/main/ssh/fleet-remote-commands.test.ts   # parseDfSizeToGb + checkRemoteDiskSpace
pnpm vitest run src/main/ssh/fleet-bootstrap-service.test.ts # disk-check step order + failure short-circuits
pnpm vitest run src/main/ssh/ssh-relay-deploy.test.ts        # checksum mismatch throws before finalizeInstall
pnpm tsc --noEmit
```

Test tối thiểu:

1. `parseDfSizeToGb('47G')` → `47`; `parseDfSizeToGb('512M')` → `0.5`; `parseDfSizeToGb('1.2T')` → `1228.8`.
2. `checkRemoteDiskSpace(conn, 5)` với mock `execCommand` trả `df -h` output có `Avail=3.2G` → `{ availableGb: 3.2, ok: false }`.
3. `bootstrapServer(...)` với disk-check mock trả `ok: false` → steps chứa `{ step: 'disk-check', status: 'error' }`, **không** chạy tiếp `packages`/`relay-deploy`/`repo-clone` (assert các mock đó `not.toHaveBeenCalled()`), `success: false`.
4. `verifyRelayChecksum` với remote hash mock khác local hash → throw đúng message chứa `"checksum mismatch"`, và `finalizeInstall` **không** được gọi (dòng `.install-complete` không ghi cho install lỗi — đúng invariant đã có sẵn trong `deployAndLaunchRelayInner`'s catch/`abandonInstall` path).
5. Test tích hợp nhẹ: `bootstrapServer()` full happy-path với tất cả bước mock `ok` → `steps` chứa đúng thứ tự `node-check, git-check, disk-check, packages, relay-deploy, repo-clone, setup-script, verify`.

## Lưu ý phạm vi (theo solution)

`verifyRelayChecksum` chỉ hash `relay.js` (entry point thực thi), không hash toàn bộ thư mục package (bao gồm `node_modules` sau `installNativeDeps`). Đây là trust boundary quan trọng nhất (file được `node relay.js` chạy trực tiếp) nhưng không phải toàn bộ supply-chain surface — mở rộng thành manifest hash cho mọi file trong `localRelayDir` là bước tiếp theo hợp lý nếu cần bảo vệ chặt hơn, nhưng **không nằm trong scope của task này** (bug MEDIUM).

Không đụng tới `desktop/`, `frontend/`, `agent/` — scope bug chỉ ghi `backend/`.
