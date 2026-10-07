# AG-CV-SOL-081-B: Catalog profile có tên, ghi đè theo host, lập kế hoạch run, preflight môi trường, `quality.listProfiles`

> ✅ **Đã triển khai.** Ngày triển khai 2026-10-07. Đã hoàn thành toàn bộ các task 081-10 đến 081-18, kiểm thử tự động xác nhận qua vitest, đạt 100% tiêu chí chấp nhận.

**CR:** [CR-CV-081](../../../../../../docs/crs/v7/quality-signals/CR-CV-081-quality-runner-on-agent.md) mục 2.2 đến 2.5, 2.8 (`qualityToolPath`), 2.10. Nhóm B; nhóm A (lõi chạy): [AG-CV-SOL-081-quality-runner-core](./AG-CV-SOL-081-quality-runner-core.md). Task AG-CV-TASK-081-10 đến 18.
**Khu vực:** `agent/src/relay/`. **Feature:** `quality-signals`.
**TDD/Spec:** [TDD-AG-01](../../../../tdd/v5/01-architecture.md), [TDD-AG-05](../../../../tdd/v5/05-tool-registry.md) (`resolveToolBinary`), [TDD-AG-04 mục 7](../../../../tdd/v5/04-handshake-session.md), [api/agent-rpc-catalog-runtime.md `preflight.*`](../../../../api/agent-rpc-catalog-runtime.md).

## 1. Hợp đồng áp dụng

| Mục ([`CONTRACT-codeintel-agent-rpc.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md)) | Áp vào |
|---|---|
| §5.1 `quality.listProfiles` (trường, `missing[].reason` enum, `definitionHash`, `display`, không lỗi khi thiếu môi trường, preflight chỉ đọc cache 60 s, lọc cờ tenant là việc backend) | tasks 10, 12, 16, 17 |
| §5.2 `quality.run` (tham số, `scope`, `append-files` ≤ 300, `full-run-filter`, `go-modules` từ `go.work`, `common/**`/`proto/**` đổi → mọi module + `scopeWidened`, `ENV_NOT_READY` chỉ khi **mọi** bước thiếu) | tasks 14, 15, 18 |
| §2.4 `qualityToolPath = toolPath + ~/go/bin + $(go env GOPATH)/bin + /usr/local/go/bin + ~/.local/share/pnpm` | task 16 |
| §9.5 Profile là dữ liệu của người phát hành agent (L1) hoặc quản trị dev server (L2 `~/.orca/quality/profiles.json`, `0600`, thư mục `0700`, chỉ thêm hoặc vô hiệu hoá; `replace:true` kèm lý do); `.orca/quality.json` P1; backend không giữ lệnh; mẫu thay `argv` chỉ `{bin:<tên>}`, `{files}`, `{files\|<mặc định>}`, `{tmp:<tên>}`, `{base}`; tên tệp bị từ chối nếu NUL/điều khiển/`-`/`realpath` thoát worktree | tasks 10, 13, 15 |
| §9.1 whitelist lệnh, `TestSpawnNeverUsesShell`, §9.6 tải | tasks 12, 15 |
| §2.1 `workspaceRoot` (tuyệt đối, realpath, là gốc worktree git, `ORCA_CODEINTEL_ALLOWED_ROOTS`) | task 14 |
| PQ-01(4) (`security-*`/`dependency-diff` do agent liệt kê, backend ẩn) | task 12 (catalog có mặt, nhóm AG-CV-SOL-091) |
| PQ-26 (`ruleId`), PQ-21, PQ-27, §8.3 | trích |
| `guides/reference/git-compatibility.md`, AGENTS.md (Git Binary Compatibility, SSH, cross-platform, max-lines) | tasks 14, 15 |

## 2. Lệch giữa CR và hợp đồng

| # | CR-CV-081 | Hợp đồng | Solution theo |
|---|---|---|---|
| 1 | Catalog tên/suite là "đề xuất" | §5.1 liệt kê catalog mặc định (đề xuất) và nói "tên cuối do solution này chốt" | **Solution chốt** danh sách ở mục 5.3 (giữ đúng tên hợp đồng, thêm `coverage-go`, `coverage-ts`, `repo-rules`, `repo-rules-scripts`, `security-*`, `dependency-diff` do các solution 083/084/091 điền định nghĩa) |
| 2 | `missing[]` kèm `searched[]` | §5.1: không có | Bỏ `searched[]` (đường dẫn ngoài `workspaceRoot` không được lộ) |
| 3 | Mẫu thay `argv` gồm `{bin}`, `{files}`, `{tmp}`, `{base}` | §9.5 giống | `proto-breaking` cần thêm **đường dẫn `.git` chung** (`--against <gitCommonDir>#branch=<base>,subdir=backend-go/proto`): token `{gitCommonDir}` **không có trong hợp đồng**. Solution thêm token tính bởi agent (không nhận từ client), **cần sửa hợp đồng §9.5** (câu hỏi mở 1) |
| 4 | Một `argv` mỗi profile, `vitest ... [related files]` | — | Vitest `related` là subcommand khác; thêm `scopeArgv` (mới) cho phép argv riêng theo `scope`; xem 5.2 |
| 5 | CR-081 2.4 `ready` theo từng profile | §5.1 `ready` theo profile trong `listProfiles` | Theo hợp đồng; preflight gắn `workspaceRoot` |
| 6 | `go.work` "22 mục `use`" (CR) | Đã đọc `backend-go/go.work`: 21 mục (`common`, `proto`, `cmd/orca-cli`, 18 service) | Số thực 21; parser đọc động, không hằng số |
| 7 | `quality.run` có `trust` (CR Q2) | §5.2: chưa thêm | Không có `trust` |
| 8 | Preflight native: `ensure-native-runtime.mjs --check-only` | README v7 điểm 22 giống | Theo (đã đọc script: nhánh `--check-only` chạy trong tiến trình, thoát 1 nếu không nạp được `node-pty`) |
| 9 | Script gốc `check-*` ở `config/scripts/` | README v7 điểm 20: nằm ở `desktop/config/scripts/`; riêng `check-max-lines-ratchet.mjs` có ở **cả hai** | Catalog dùng đường dẫn đã `ls` (mục 5.3); `check-max-lines-ratchet` chạy ở gốc `.`; profile chạy với `cwd` đúng |

## 3. Phụ thuộc chéo khu vực

| Đối ứng | Quan hệ |
|---|---|
| `AG-CV-SOL-081-quality-runner-core` | Cung cấp `PlannedStep`, run manager, RPC; solution này cấp kế hoạch và `listProfiles` |
| `AG-CV-SOL-001` | `codeintel-repo-resolution.ts`: task 14 cần hàm `resolveWorktreeRoot` (bước 1-4: hình dạng → `realpath` → allowed roots → `git rev-parse --show-toplevel` trùng). Nếu SOL-001 chưa xuất, task 14 trích hàm đó ra (thoả thuận với chủ SOL-001), không nhân đôi logic |
| `AG-CV-SOL-080` (task 04 `readHostSnapshot`) | Task 17 dùng lại khối `host` |
| `AG-CV-SOL-082` (`quality-parser-registry`, khoá `parserKey`) | Catalog tham chiếu khoá `oxlint@json`... |
| `AG-CV-SOL-083/084/091` | Điền profile `coverage-*`, `repo-rules*`, `security-*`, `dependency-diff` qua API đăng ký ở task 12 |
| `BE-CV-SOL-085-quality-gate-evaluator-and-profiles`, `BE-CV-SOL-082-…` | Gọi `listProfiles`, ghim chính sách vào `definitionHash`, ẩn `security-*` theo cờ tenant (PQ-01); backend **không** giữ lệnh |
| `BE-CV-SOL-023-…` | Timeout Go `listProfiles` 45 s (agent 40 s) |
| `AG-CV-SOL-072-security-tests-agent` | Test phản chiếu catalog: không có `pnpm install|rebuild`, `go mod download`, `--runtime=`, `--init`, `--prune` |
| Thứ tự: `001 → 081-A → 081-B → 082 …` (đợt 7) |

## 4. Re-verify (đã đọc, 2026-10-06)

Đã đọc/`ls`: `/opt/repos/orca/package.json` scripts (qua CR), `agent/src/relay/agent-config.ts` (`buildToolPath`), `agent-tool-registry.ts` (`resolveToolBinary` dòng ~60), `desktop/config/scripts/ensure-native-runtime.mjs` (dòng 1-60: cờ `--check-only`, `projectDir = resolve(dirname, '../..')`), `config/scripts/check-max-lines-ratchet.mjs` (dòng 120-260: **`--init` và `--prune` GHI baseline**, mặc định chỉ đọc), `desktop/config/scripts/check-styled-scrollbars.mjs`, `check-reliability-gates.mjs` (`main(root = process.cwd())`), `backend-go/go.work`, `node_modules/.bin/{oxlint,tsc,vitest}` ở gốc (có), `which go golangci-lint buf opa` (có: `/usr/bin/go`, `~/go/bin/{golangci-lint,buf,opa}`; **không có** `govulncheck`, `osv-scanner`, `gitleaks`), `agent/tsconfig.json` (`composite:true`).

| Điểm | Hiện trạng | Hệ quả |
|---|---|---|
| Tồn tại cấu hình | `desktop/config/{tsconfig.node.json,tsconfig.tc.web.json,tsconfig.tc.cli.json,vitest.config.ts,reliability-gates.jsonc}`, `desktop/config/scripts/*.mjs`, gốc `config/scripts/check-max-lines-ratchet.mjs`, `config/max-lines-baseline.txt` đều có | `requires:[{type:"file"}]` kiểm đúng các đường dẫn này |
| `check-max-lines-ratchet.mjs --init|--prune` | Ghi `config/max-lines-baseline.txt` | argv của profile **chỉ** `node config/scripts/check-max-lines-ratchet.mjs`, không tham số; test cấm `--init`/`--prune` |
| `ensure-native-runtime` | `--runtime=node` có thể `pnpm rebuild`; `--check-only` chỉ đọc | Preflight chỉ dùng `--check-only` |
| Công cụ trên máy khảo sát | `golangci-lint` ở `~/go/bin` (ngoài `buildToolPath`) | `qualityToolPath` |
| `agent/tsconfig.json` `composite:true` | `tsc --noEmit -p tsconfig.json` chưa chắc chạy | Task 11 thử; mặc định profile `ts-typecheck-agent` thêm `--composite false` nếu cần (chưa kiểm chứng) |
| Test | `agent/vitest.config.ts` include `src/**/*.test.ts`; test dùng repo git thật trong thư mục tạm là mẫu có sẵn (`git-handler-worktree-git-capabilities.test.ts` dùng mock; repo thật là đề xuất) | |

Correction relative to CR: CR-081 1.2 viết `config/scripts/` ở gốc chỉ có 3 tệp — đúng (đã `ls`); CR nói `pr.yml` trỏ `config/vitest.config.ts` — theo hợp đồng §10, chưa chạy lại.

## 5. Giải pháp

### 5.1 Cây file (mới)

```
quality-profile-schema.ts / .test.ts          task 10   kiểu QualityCheckProfile/Suite, validate, definitionHash
quality-profile-catalog-evidence.test.ts      task 11   (+ fixtures help/--version) bằng chứng tồn tại tệp và cờ
quality-profile-catalog.ts / .test.ts         task 12   catalog tích hợp + API đăng ký (cho 083/084/091)
quality-profile-host-overrides.ts / .test.ts  task 13
quality-workspace-root.ts, quality-changed-files.ts, quality-dirty-fingerprint.ts / .test.ts   task 14
quality-run-planning.ts / .test.ts            task 15
quality-tool-resolution.ts, quality-environment-preflight.ts / .test.ts   task 16
quality-list-profiles.ts / .test.ts           task 17
quality-run-start.ts / .test.ts               task 18   nối `quality.run`
```

### 5.2 Schema profile (task 10)

```ts
export type QualityCheckProfile = {
  id: string                                   // ^[a-z0-9][a-z0-9-]{0,47}$
  title: string; kind: 'lint'|'typecheck'|'test'|'proto'|'policy'|'repo-rules'|'coverage'|'security'|'dependency'
  parser: string | null                        // khoá SOL-082
  cwd: string                                  // tương đối gốc worktree
  argv: readonly string[]                      // argv[0] là "{bin:x}" hoặc "node"; mẫu hợp lệ duy nhất ở 5.2
  scopeArgv?: Partial<Record<'changed'|'commitRange', readonly string[]>>   // (mới)
  scopes: readonly ('worktree'|'changed'|'commitRange')[]
  scopeStrategy: 'append-files'|'full-run-filter'|'go-modules'|'none'
  fileGlobs?: readonly string[]
  timeoutMs: number; maxOutputBytes: number; heavy: boolean
  needsNativeRuntime?: boolean; network?: boolean
  env: { set: Record<string,string>; allowExtra: readonly string[] }
  requires: readonly ({ type: 'bin'; name: string } | { type: 'file'; path: string })[]
  exit: { ok: readonly number[]; findings: readonly number[] }
  perModule?: boolean                          // Go: một bước mỗi module
}
```
Mẫu thay trong argv: `{bin:<tên>}` (phân giải, task 16), `{files}` (nguyên một phần tử, mở thành nhiều), `{files|<mặc định>}` (mặc định dùng khi `scope=worktree`), `{tmp:<tên>}` (trong thư mục run), `{base}`, và `{gitCommonDir}` (mới; đường dẫn tuyệt đối `git rev-parse --git-common-dir` đã `realpath`). Mọi token `{...}` khác là lỗi nạp. `definitionHash = "sha256:" + hex(sha256(canonicalJSON({schemaVersion, id, kind, parser, cwd, argv, scopeArgv, scopes, scopeStrategy, fileGlobs, env.set, exit, requires, perModule})))` (khoá sắp xếp, không `title`/`display`).

### 5.3 Catalog tích hợp cho Orca (task 12; giữ đúng tên hợp đồng §5.1)

`cwd` tương đối gốc repo; **chưa chạy thật**; sự tồn tại tệp đã `ls`.

| id | cwd | argv (sau thay mẫu) | scopes / strategy | heavy |
|---|---|---|---|---|
| `ts-lint` | `.` | `oxlint --format json {files\|.}` | all / `append-files` | không |
| `ts-typecheck-desktop-node` / `-web` / `-cli` | `desktop` | `tsc --noEmit -p config/tsconfig.node.json\|tsconfig.tc.web.json\|tsconfig.tc.cli.json --pretty false` | `full-run-filter` | có |
| `ts-typecheck-agent` | `agent` | `tsc --noEmit -p tsconfig.json --pretty false` (composite chưa chắc) | `full-run-filter` | có |
| `ts-typecheck-frontend` | `frontend` | `tsc --noEmit -p tsconfig.json --pretty false` | `full-run-filter` | có |
| `ts-unit-desktop` | `desktop` | `vitest run --config config/vitest.config.ts --reporter=json --outputFile={tmp:vitest.json}`; `scopeArgv.changed = vitest related --run … {files}` | `append-files` | có (`needsNativeRuntime`) |
| `ts-unit-frontend` | `frontend` | như trên, `--config config/vitest.config.ts` | `append-files` | có |
| `ts-unit-agent`, `ts-unit-backend` | `agent`, `backend` | `vitest run --reporter=json --outputFile={tmp:vitest.json}` | `append-files` | có |
| `repo-check-max-lines` | `.` | `node config/scripts/check-max-lines-ratchet.mjs` | `none` | không |
| `repo-check-styled-scrollbars` | `desktop` | `node config/scripts/check-styled-scrollbars.mjs` | `none` | không |
| `repo-check-reliability-gates` | `desktop` | `node config/scripts/check-reliability-gates.mjs` | `none` | không |
| `go-vet` / `go-test` / `go-lint` | `backend-go/<module>` | `go vet -json ./...` / `go test -json ./...` / `golangci-lint run --out-format json ./...` | `go-modules`, `perModule` | có |
| `proto-lint` / `proto-breaking` | `backend-go/proto` | `buf lint --error-format json` / `buf breaking --against {gitCommonDir}#branch={base},subdir=backend-go/proto --error-format json` | `none` / (cần `base`) | không |
| `opa-test` | `backend-go` | `opa test policy/orca-authz/ --format json` | `full-run-filter` | không |
| `coverage-go`, `coverage-ts` | — | điền ở AG-CV-SOL-083 | | có |
| `repo-rules`, `repo-rules-scripts` | — | điền ở AG-CV-SOL-084 | | không |
| `security-go-vuln`, `security-deps-osv`, `security-secrets-diff`, `dependency-diff` | — | điền ở AG-CV-SOL-091 | | |

Suite: `fast` = `ts-lint`, `repo-check-max-lines`, `go-vet`; `standard` = `fast` + `ts-typecheck-*`, `ts-unit-*`, `go-test`, `proto-lint`, `opa-test`, `repo-check-*`; `full` = `standard` + `go-lint`, `proto-breaking`. Không đưa vào: `lint:switch-exhaustiveness`, `verify:localization-*`, Playwright e2e, `go test -tags=integration`, `build:*`, `bench:*`. Cấm tuyệt đối trong mọi argv (test): `install`, `rebuild`, `mod download`, `--runtime=`, `--init`, `--prune`, `analyze`, `clean`.

### 5.4 Kế hoạch run (task 15) và preflight (task 16)

`planRun(input: { root; profileOrSuite; scope; base; catalog; env; now }) → RunPlan` mở suite thành `PlannedStep[]` (id `go-test:services/project-service`), thay mẫu, kiểm `cwd` (realpath nằm trong worktree), kiểm tệp chèn (NUL/điều khiển/`-`/symlink thoát), `append-files` ≤ 300 tệp (hơn → `full-run-filter` + `scopeWidened`), `go-modules` đọc khối `use (...)` (và dạng `use ./x`) của `backend-go/go.work`, mỗi đường dẫn nằm trong `backend-go`; `common/**` hoặc `proto/**` đổi → mọi module + `scopeWidened`. Preflight (chỉ đọc, cache 60 s theo `(repoRoot, profileId)`): bin resolver (`<cwd>/node_modules/.bin`, `<repoRoot>/node_modules/.bin` **trong chính worktree**, rồi `qualityToolPath`), `node_modules/.modules.yaml`, native `--check-only` (timeout 15 s, `process.execPath`), Go (`go version` ≥ chỉ thị `go` của `go.work`; GOMODCACHE không rỗng), `golangci-lint --version` (`built with goX` so `go.work`; major 2 → `tool_incompatible`), `buf`/`opa`, `git rev-parse --verify --quiet <base>^{commit}`, `fs.statfs(tmpdir) ≥ 256 MiB`, `HOME`.

## 6. Quyết định thiết kế

| # | Quyết định | Lý do | Bỏ |
|---|---|---|---|
| 1 | Catalog là mã TS (không YAML/JSON) | Agent đóng gói thành một `agent.js` bằng esbuild (đã đọc `build.mjs`); không có loader cho tệp dữ liệu | Tệp `.yaml` đi kèm |
| 2 | L1 trong mã, L2 ở `~/.orca/quality/profiles.json` chỉ thêm/vô hiệu | §9.5 | Lệnh trong repo hoặc backend |
| 3 | `scopeArgv` thay vì nhồi mọi biến thể vào một argv | `vitest related` ≠ `vitest run` | Một argv điều kiện |
| 4 | Token `{gitCommonDir}` do agent tính | `.git` của worktree liên kết là tệp (CR-081 2.3) | Nhận đường dẫn từ client |
| 5 | Bin chỉ trong worktree | Không chạy mã `node_modules` của checkout khác (CR Q5) | Fallback sang checkout chính |
| 6 | Preflight không bao giờ cài/ghi | O12/O11 | `pnpm install` |
| 7 | `ready` theo repo, `capability` theo máy | handshake không có `workspaceRoot` | — |

## 7. Tiêu chí chấp nhận

- [x] `listProfiles` trả đúng catalog 5.3 với `ready`/`missing[]` (reason thuộc enum §5.1), không `searched[]`, không đường dẫn ngoài `workspaceRoot`, không argv/env; **không lỗi** khi thiếu môi trường; `definitionHash` đổi khi đổi `argv`.
- [x] Test quét mọi argv của catalog: không có token cấm (5.3), `argv[0]` là `{bin:}` hoặc `node`, chỉ mẫu hợp lệ.
- [x] `quality.run` tên lạ → `PROFILE_UNKNOWN` không spawn; `ENV_NOT_READY` chỉ khi mọi bước thiếu; thiếu một phần → bước `env_not_ready`.
- [x] Tên tệp bắt đầu `-`, chứa NUL, symlink thoát worktree, `base` lạ bị từ chối; `cwd` symlink ra ngoài bị từ chối.
- [x] `scope=changed`: 4 tệp TS → `ts-lint` đúng 4 tệp; > 300 tệp → chạy đủ + `scopeWidened`; đổi `backend-go/common/**` → mọi module + `scopeWidened`.
- [x] Preflight: thiếu `golangci-lint`, `built with` cũ hơn `go.work`, thiếu `node_modules`, `tmp_space_low`; native chỉ dùng `--check-only`.
- [x] L2 không đổi được `argv` nếu không `replace:true` + lý do; tệp quyền `0600`.

## 8. Kiểm thử

Mỗi task có file test riêng (mục "Kiểm thử" của task). Lệnh tổng: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-profile-schema.test.ts src/relay/quality-profile-catalog.test.ts src/relay/quality-profile-host-overrides.test.ts src/relay/quality-changed-files.test.ts src/relay/quality-run-planning.test.ts src/relay/quality-environment-preflight.test.ts src/relay/quality-list-profiles.test.ts src/relay/quality-run-start.test.ts`. Repo git thật trong thư mục tạm cho `quality-changed-files` (kể cả `git worktree add`) và baseline Git 2.25.

## 9. Rủi ro và chưa kiểm chứng

- Chưa chạy profile nào: đặc biệt `ts-typecheck-*` với tsc 7.0.2, `ts-typecheck-agent` (composite), `go-lint` (golangci-lint v1.62.2 build go1.22.2 vs `go.work` 1.26 → `tool_too_old`), `proto-breaking` trong worktree liên kết, `vitest related`.
- `GOPROXY=off`/`-mod=readonly` có thể biến test cần mạng thành `env_not_ready` giả (chưa đo).
- Đổi cấu trúc repo (đưa `desktop/config` lên gốc hoặc ngược lại) buộc đổi `cwd`/đường dẫn catalog; `definitionHash` đổi báo hiệu.
- Windows: `PATHEXT`, shim `.cmd` chưa làm; MVP `unsupported_platform`.
- Git: `rev-parse --git-common-dir` trả đường dẫn tương đối ở một số bản; luôn `path.resolve(cwd, out)`; không dùng `--path-format` (Git 2.31).

## 10. Câu hỏi mở

1. Thêm `{gitCommonDir}` và `scopeArgv` vào hợp đồng §9.5/§5.1?
2. Có ghi `ts-typecheck-agent` ở trạng thái mặc định tắt (`enabled:false`) tới khi chạy thử được không?
3. `repo-check-max-lines` cwd `.` hay `desktop`? Có hai bản baseline (README v7 điểm 20): mặc định `.`.
