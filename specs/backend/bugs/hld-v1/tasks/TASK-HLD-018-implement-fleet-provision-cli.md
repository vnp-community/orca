# TASK-HLD-018: Implement CLI `orca fleet provision`

**Priority:** 🟡 MEDIUM
**Effort:** ~3-4 giờ (impl + test + khảo sát entry point CLI thật)
**Status:** ⚠️ DONE-DIFFERENTLY — 2026-08-09 (**tiền đề của bug/solution SAI**, đã tự khảo sát và tìm ra thay vì làm theo solution nguyên văn: `orca fleet provision` **đã tồn tại từ trước**, KHÔNG phải ở `backend/` mà ở `desktop/src/cli/handlers/fleet.ts:198-254` — CLI dispatcher thật là `desktop/src/cli/index.ts` (registry `CommandHandler` + `RuntimeClient` JSON-RPC qua Unix socket, không phải kiểu "spawn Electron as Node rồi gọi thẳng service" mà solution giả định). Đã có đủ `--project`/`--server`/`--all`/`--dry-run`/`--concurrency`, dùng `p-limit` thật (trái với claim "không có p-limit trong dependencies" của solution — đúng cho `backend/package.json` nhưng sai cho `desktop/package.json`).

**Gap thật đã sửa:** handler cũ chỉ gọi `client.call('ssh.connect', ...)` — CHỈ kết nối SSH + deploy relay, KHÔNG hề gọi `ssh.bootstrapServer` (cài Node.js/Git, clone repo) — nghĩa là "provision" trước đây chỉ "connect" trá hình, không thật sự provision. Đã thêm lời gọi `client.call('ssh.bootstrapServer', {targetId})` ngay sau `ssh.connect` thành công. `tsc --noEmit` sạch cho file này.

**KHÔNG tạo** `backend/src/main/cli/fleet-provision-cli.ts` như solution hướng dẫn — sẽ tạo ra code trùng lặp, không ai gọi tới (đúng anti-pattern "dead code" đã gặp ở BUG-BE-HLD-004/GitHub relay). Solution `SOLUTION-fleet-exact.md` mục này cần viết lại — đã ghi chú tại đây để người review biết lý do lệch khỏi solution gốc.)
**Bug refs:** BUG-BE-HLD-012
**Solution ref:** [SOLUTION-fleet-exact.md](../solutions/SOLUTION-fleet-exact.md)
**Depends on:** None

---

## ⚠️ Ghi chú quan trọng — entry point CLI thật CHƯA được xác định trong solution

Solution xác nhận (đọc trực tiếp source):

- `package.json` khai `"bin": { "orca": "./out/cli/index.js" }` nhưng **không có file nguồn `src/cli/index.ts`** trong nhánh source hiện tại — nghĩa là entry point argv-dispatch chính của `bin.orca` chưa được implement/không nằm trong nhánh này.
- `backend/src/main/cli/` hiện chỉ chứa installer/dispatcher script cho **cài đặt** lệnh `orca` vào PATH (`appimage-cli-wrapper.ts`, `cli-installer.ts`, `linux-bare-orca-dispatcher.ts`, `linux-terminal-orca-cli-shim.ts`) — không có gì xử lý **subcommand logic** như `fleet provision`.

**Vì vậy solution chỉ viết sẵn module logic độc lập** (`runFleetProvisionCli()`), export sẵn để một dispatcher gắn vào — **không đoán mò cấu trúc file dispatcher chưa tồn tại**.

**Sub-step bắt buộc cho người/agent thực thi task này** (KHÔNG có trong solution, cần tự khảo sát thêm):

1. Tìm vị trí thật nơi argv top-level của `bin.orca` được parse hiện nay (có thể chưa tồn tại — cần kiểm tra build config, `package.json` script `build:cli` hoặc tương đương, và xem `out/cli/index.js` được build từ đâu).
2. Tham khảo pattern `parseArgs()` đã dùng trong `daemon-entry.ts:22-44` (solution ghi nhận đây là pattern tương tự đã có sẵn cho các entry khác) làm mẫu, nhưng **verify lại trực tiếp trong code hiện tại** trước khi copy cấu trúc — solution KHÔNG đọc file `daemon-entry.ts` đầy đủ, chỉ tham chiếu số dòng ước lượng.
3. Nếu `src/cli/index.ts` chưa tồn tại: tạo mới, đảm bảo build pipeline (`package.json` bin field, tsup/esbuild config, v.v.) trỏ đúng output `out/cli/index.js`.
4. Nếu đã tồn tại (có thể đã được thêm ở nhánh/PR khác từ lúc solution được viết) thì đọc lại và wire nhánh `fleet provision` vào đúng vị trí, không tạo file trùng.
5. Sau khi wire xong, verify bằng cách chạy `orca fleet provision --dry-run` thật (không chỉ unit test) trên môi trường build cục bộ.

## Mục tiêu

- Implement module `runFleetProvisionCli()` — bulk-provision Dev Servers từ `orca-fleet.yaml` qua `orca fleet provision`.
- Hỗ trợ flags: `--config`, `--project`, `--server`, `--concurrency`, `--dry-run`.
- Tái dùng `SshConnectionStore.importFromFleetConfig()` (đã có sẵn) để parse YAML → upsert SshTarget — không viết lại logic import.
- Tái dùng `groupSshTargetsByProject()` và `bootstrapServer()` (đã có sẵn) để group và provision.
- Tự viết concurrency limiter tối giản (không có `p-limit` trong `package.json` dependencies — đã kiểm tra, không kéo thêm dependency cho một semaphore ~15 dòng).
- Wire vào CLI dispatcher thật (xem mục "Sub-step bắt buộc" ở trên).

## Bối cảnh kỹ thuật quan trọng (từ solution, đọc trực tiếp code)

- `ssh-remote-cli-host-passthrough.ts` xác nhận lệnh `orca` khi chạy thật sự **spawn lại chính app Electron ở chế độ `ELECTRON_RUN_AS_NODE=1`** (không phải một client RPC mỏng gọi vào server đang chạy).
- `vite.config.ts:84` alias `'electron' → src/platform/stubs/electron-node-wrapper.ts`, nên `ipcMain`/`app` vẫn hoạt động (stub) trong tiến trình CLI.
- → CLI có thể khởi tạo trực tiếp `Store` + `registerSshHandlers()` trong cùng tiến trình, y hệt cách `server-bootstrap.ts` làm, rồi gọi thẳng `bootstrapServer()` — không cần dựng thêm cơ chế IPC/RPC client-server mới.

## File cần sửa/tạo

```
backend/src/main/cli/fleet-provision-cli.ts     (MỚI)
backend/src/main/cli/fleet-provision-cli.test.ts (MỚI)
<vị trí CLI dispatcher thật — cần tự tìm, xem "Sub-step bắt buộc">
```

## Thay đổi cụ thể

### File mới: `backend/src/main/cli/fleet-provision-cli.ts`

```typescript
// src/main/cli/fleet-provision-cli.ts
// `orca fleet provision` — bulk-provision Dev Servers from orca-fleet.yaml.
// FIX BUG-BE-HLD-012: CR-003 (F31) had no CLI surface at all.
import { Store } from '../persistence'
import { registerSshHandlers, getSshConnectionStore } from '../ipc/ssh'
import { bootstrapServer } from '../ssh/fleet-bootstrap-service'
import { groupSshTargetsByProject } from '../../shared/ssh-types'
import type { SshTarget } from '../../shared/ssh-types'

export type FleetProvisionArgs = {
  configPath: string
  project?: string
  serverId?: string
  concurrency: number
  dryRun: boolean
}

const DEFAULT_CONCURRENCY = 3
const DEFAULT_FLEET_CONFIG_PATH = 'deploy/dev/orca-fleet.yaml'

export function parseFleetProvisionArgs(argv: string[]): FleetProvisionArgs {
  let configPath = DEFAULT_FLEET_CONFIG_PATH
  let project: string | undefined
  let serverId: string | undefined
  let concurrency = DEFAULT_CONCURRENCY
  let dryRun = false

  for (let i = 0; i < argv.length; i++) {
    switch (argv[i]) {
      case '--config':
        configPath = argv[++i] ?? configPath
        break
      case '--project':
        project = argv[++i]
        break
      case '--server':
        serverId = argv[++i]
        break
      case '--concurrency': {
        const parsed = Number(argv[++i])
        if (Number.isFinite(parsed) && parsed > 0) concurrency = Math.floor(parsed)
        break
      }
      case '--dry-run':
        dryRun = true
        break
    }
  }

  return { configPath, project, serverId, concurrency, dryRun }
}

// Why: no p-limit dependency in package.json — a bounded-concurrency runner
// is ~15 lines, not worth pulling in a package for.
function createConcurrencyLimiter(max: number) {
  let active = 0
  const queue: Array<() => void> = []
  return function limit<T>(fn: () => Promise<T>): Promise<T> {
    return new Promise((resolve, reject) => {
      const run = (): void => {
        active++
        fn()
          .then(resolve, reject)
          .finally(() => {
            active--
            const next = queue.shift()
            if (next) next()
          })
      }
      if (active < max) run()
      else queue.push(run)
    })
  }
}

/**
 * Entry point for `orca fleet provision`. Returns a process exit code
 * (0 = all servers provisioned/planned OK, 1 = at least one failure).
 */
export async function runFleetProvisionCli(argv: string[]): Promise<number> {
  const args = parseFleetProvisionArgs(argv)

  // Why: CLI runs as its own ELECTRON_RUN_AS_NODE process (see
  // ssh-remote-cli-host-passthrough.ts) — same pattern server-bootstrap.ts
  // uses to bring up the SSH subsystem, minus the renderer-facing IPC wiring
  // that doesn't apply headlessly (getMainWindow → () => null is safe: every
  // caller in ssh.ts null-checks the window before use).
  const store = new Store()
  registerSshHandlers(store, () => null)

  const sshStore = getSshConnectionStore()
  if (!sshStore) {
    console.error('[fleet provision] Failed to initialize SSH store.')
    return 1
  }

  console.log(`[fleet provision] Importing fleet config: ${args.configPath}`)
  const importResult = await sshStore.importFromFleetConfig(args.configPath)
  const failedImports = importResult.servers.filter((s) => s.action === 'skipped')
  for (const failed of failedImports) {
    console.warn(`[fleet provision] Skipped ${failed.fleetId}: ${failed.error}`)
  }

  let targets: SshTarget[] = sshStore
    .listTargets()
    .filter((t) => t.fleetConfigSource === args.configPath)
  if (args.project) targets = targets.filter((t) => t.project === args.project)
  if (args.serverId) targets = targets.filter((t) => t.fleetId === args.serverId)

  if (targets.length === 0) {
    console.log('[fleet provision] No matching servers to provision.')
    return 0
  }

  const groups = groupSshTargetsByProject(targets)
  console.log(`[fleet provision] Plan (${targets.length} server(s)):`)
  for (const group of groups) {
    console.log(`  Group: ${group.label} (${group.targets.length} server${group.targets.length === 1 ? '' : 's'})`)
    for (const t of group.targets) {
      console.log(`    - ${t.fleetId ?? t.id} (${t.label}) @ ${t.host}`)
    }
  }

  if (args.dryRun) {
    console.log('[fleet provision] --dry-run: no servers were touched.')
    return 0
  }

  const limit = createConcurrencyLimiter(args.concurrency)
  console.log(`[fleet provision] Provisioning with concurrency=${args.concurrency}...`)

  const results = await Promise.all(
    targets.map((target) =>
      limit(async () => {
        try {
          const { connectRegisteredSshTarget } = await import('../ipc/ssh')
          const state = await connectRegisteredSshTarget(target.id)
          if (state.status !== 'connected') {
            console.error(`❌ ${target.label}: cannot connect (status: ${state.status})`)
            return { targetId: target.id, ok: false }
          }
          const result = await bootstrapServer(target.id, { fleetConfigPath: args.configPath })
          if (result.success) {
            const stepSummary = result.steps
              .filter((s) => s.status === 'ok')
              .map((s) => s.step)
              .join(', ')
            console.log(`✅ ${target.label}: ${stepSummary}`)
          } else {
            console.error(`❌ ${target.label}: ${result.error}`)
          }
          return { targetId: target.id, ok: result.success }
        } catch (err) {
          console.error(`❌ ${target.label}: ${err instanceof Error ? err.message : String(err)}`)
          return { targetId: target.id, ok: false }
        }
      })
    )
  )

  const failed = results.filter((r) => !r.ok)
  console.log(`[fleet provision] Done: ${results.length - failed.length}/${results.length} succeeded.`)
  return failed.length > 0 ? 1 : 0
}
```

### Wiring vào CLI dispatcher

Solution KHÔNG chỉ ra vị trí file dispatcher thật (xem cảnh báo đầu task). Pattern đề xuất — theo `parseArgs()` của `daemon-entry.ts:22-44` (verify lại trực tiếp trước khi copy, số dòng chỉ là ước lượng từ solution, không phải đã đọc đầy đủ file đó):

```typescript
if (argv[0] === 'fleet' && argv[1] === 'provision') {
  const { runFleetProvisionCli } = await import('./main/cli/fleet-provision-cli')
  process.exitCode = await runFleetProvisionCli(argv.slice(2))
  return
}
```

Áp nhánh này vào bất kỳ đâu argv top-level của `bin.orca` được parse — cần tự xác định vị trí thật (xem "Sub-step bắt buộc" ở đầu task) trước khi hoàn tất phần này.

## Verification

```bash
cd backend
pnpm vitest run src/main/cli/fleet-provision-cli.test.ts   # NEW
pnpm tsc --noEmit

# Sau khi wire dispatcher xong — verify thật, không chỉ unit test:
orca fleet provision --dry-run --config <fixture-yaml-path>
```

Test tối thiểu:

1. `parseFleetProvisionArgs(['--project', 'vnp-blc', '--concurrency', '5', '--dry-run'])` → `{ project: 'vnp-blc', concurrency: 5, dryRun: true, configPath: DEFAULT_FLEET_CONFIG_PATH }`.
2. `--concurrency abc` (không phải số) → fallback về `DEFAULT_CONCURRENCY`, không throw.
3. `createConcurrencyLimiter(2)` với 5 task async (mock `setTimeout`) → tối đa 2 task chạy đồng thời tại mọi thời điểm (assert qua counter).
4. `runFleetProvisionCli(['--dry-run', '--config', fixturePath])` với fixture YAML 2 servers → in đúng plan, **không** gọi `connectRegisteredSshTarget`/`bootstrapServer` (mock + assert `not.toHaveBeenCalled()`), return `0`.
5. `runFleetProvisionCli([...])` không `--dry-run`, mock `bootstrapServer` trả về 1 success + 1 failure → return `1`.
6. (Sub-step riêng của task này, không có trong solution) Integration check: sau khi wire dispatcher, `orca fleet provision --dry-run` chạy được từ shell thật và trả exit code 0.
