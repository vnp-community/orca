# CR-CV-081 — Bộ chạy kiểm tra (quality runner) trên agent: profile có tên, tiến độ, huỷ, giới hạn tài nguyên, sẵn sàng môi trường

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-081 |
| **Tên** | Nhóm RPC `quality.*` trên agent (Part A): `listProfiles`, `run`, `runStatus`, `cancel`, `results` và thông báo `quality.progress`/`quality.finished`; profile kiểm tra có tên do cấu hình phía dev server định nghĩa; preflight môi trường; chạy nền, huỷ cả cây tiến trình, giới hạn đồng thời, log tạm bị cắt, cách ly tối thiểu |
| **Loại** | Feature (nền cho tín hiệu chất lượng) |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | [CR-CV-001](../agent-codeintel/CR-CV-001-codeintel-agent-foundation.md) (dispatcher mẫu, phân giải `workspaceRoot`, mã lỗi, giới hạn, tệp tạm), [CR-CV-004](../agent-codeintel/CR-CV-004-codeintel-reindex-and-index-notifications.md) (khung job nền, `codeintel-notification-sink.ts`, nhật ký job) |
| **Mở khoá** | [CR-CV-082](./CR-CV-082-quality-finding-model-and-parsers.md) (parser gắn vào bước chạy), CR-CV-083 (coverage dùng lại bộ chạy), CR-CV-084 (rule pack là profile), CR-CV-085 (cổng), CR-CV-089 (chạy lại độc lập); phía Go: `StartQualityRun` (CR-CV-085) gọi `quality.run` qua `RelayByDevServer` |
| **Tác động** | `agent/src/relay/agent-rpc-dispatch.ts` (một nhánh trong `route()`, một nhánh `extractTraceFields`), `agent/src/relay/agent-session-capabilities.ts` (thêm `quality`), `agent/src/relay/agent-session.ts` (một dòng dọn dẹp trong `stop()` như CR-CV-004), các file mới `agent/src/relay/agent-rpc-dispatch-quality.ts` và `agent/src/relay/quality-*.ts` (mục 2.1). Không sửa `runToolCommand` (bộ chạy có đường spawn riêng, mục 3) |

---

## 1. Bối cảnh và vấn đề

Đọc code/cấu hình ngày 2026-10-06. Không chạy `pnpm lint/test/build`, `go test` hay lệnh ghi.

### 1.1 Hiện không có đường để UI hỏi "chạy kiểm tra trên worktree này ngay"

- Các kiểm tra chỉ chạy ở CI sau khi có PR hoặc do người dùng chạy tay (research 11 §3.2). `agent` có `shell.exec`, `git.exec`, `agent.exec` (chạy lệnh tuỳ ý theo yêu cầu người dùng) nhưng không có khái niệm "bộ kiểm tra có tên", tiến độ, huỷ, hay thu kết quả có cấu trúc.
- Method Go gọi qua `RelayByDevServer` mặc định bị cắt ở 30 s (CR-CV-001 1.7); `lint`/`test` mất hàng chục giây đến phút, nên phải là job nền + thông báo.

### 1.2 Lệnh kiểm tra thật của repo (đã đọc), và chỗ các script gốc đã lệch cây thư mục

| Nơi đọc | Điều đọc được |
|---|---|
| `/opt/repos/orca/package.json` scripts | `lint` = `oxlint && pnpm run lint:switch-exhaustiveness && node config/scripts/check-styled-scrollbars.mjs && pnpm run check:reliability-gates && pnpm run check:max-lines-ratchet && pnpm run verify:localization-catalog && pnpm run verify:localization-coverage`; `typecheck` = ba lệnh `tsc --noEmit -p config/tsconfig.{node,tc.cli,tc.web}.json`; `test` = `node config/scripts/ensure-native-runtime.mjs --runtime=node && vitest run --config config/vitest.config.ts` |
| **Lệch cây thư mục** | Ở **gốc** repo (`ls`): `config/scripts/` chỉ có `check-max-lines-ratchet.mjs`, `check-max-lines-ratchet.test.mjs`, `rebuild-native-deps.mjs`; **không có** `config/tsconfig.node.json`, `config/vitest.config.ts`, `config/oxlint-switch-exhaustiveness.json`, `config/scripts/{check-styled-scrollbars,check-reliability-gates,verify-localization-catalog,audit-localization-coverage,ensure-native-runtime}.mjs` (mỗi tệp kiểm bằng `[ -e ]`: MISSING). Chúng nằm ở `desktop/config/` và `desktop/config/scripts/` (đã `ls`). Monorepo hiện có workspace `frontend`, `backend`, `agent`, `desktop`, `emulator`, `packages/dev-agent-transport` (`pnpm-workspace.yaml`). Hệ quả: `pnpm lint/typecheck/test` ở gốc **có thể không chạy được** như script khai báo (chưa chạy để xác nhận; theo yêu cầu không chạy). `.github/workflows/pr.yml` cũng gọi `pnpm exec vitest run --config config/vitest.config.ts` và `pnpm check:styled-scrollbars` ở gốc, cùng giả định đó |
| `desktop/package.json` | Không có script `lint`/`typecheck`; có `test` = `node config/scripts/ensure-native-runtime.mjs --runtime=node && vitest run --config config/vitest.config.ts` |
| `agent/package.json`, `backend/package.json` | `test` = `vitest run`; `agent/vitest.config.ts` include `src/**/*.test.ts`; `frontend/package.json` `test` = `vitest run --config config/vitest.config.ts`; `packages/dev-agent-transport` có `typecheck` = `tsc --noEmit` |
| `desktop/config/vitest.config.ts` | `projectRoot = resolve(process.cwd())`: **bắt buộc chạy với cwd = `desktop/`** (ghi chú trong tệp) |
| `desktop/config/scripts/ensure-native-runtime.mjs` | `ensureNodeRuntime()` nếu `node-pty` không nạp được hoặc bản vá lệch thì **chạy `pnpm rebuild node-pty`** (`runPnpm(['rebuild', …])`): **ghi vào `node_modules`**. Có nhánh chỉ-đọc `--check-only` (`CHILD_CHECK_FLAG`, dòng 14-23): thoát 1 và in `module: message` nếu không nạp được |
| `/opt/repos/orca/.oxlintrc.json` | Cấu hình oxlint ở gốc; `pr.yml:64` chạy `pnpm exec oxlint --format github` ở gốc. `node_modules/.bin/oxlint` v1.71.0, `vitest` 4.1.5, `tsc` "Version 7.0.2" có ở gốc |
| `backend-go/Makefile` | `vet`/`test`/`lint` lặp từng module trong `common`, `proto`, 19 service (`SERVICES`); `test-integration` = `go test -tags=integration` (cần Docker); `proto-lint` = `cd proto && buf lint && buf breaking --against '.git#branch=main' \|\| true` (**`\|\| true` nên không bao giờ thất bại**); `opa-test` = `opa test policy/orca-authz/ -v` |
| `backend-go/.golangci.yml` | cú pháp v1 (`disable-all`, `gosimple`), `run.timeout 5m`, bật 12 linter. Máy khảo sát: `golangci-lint v1.62.2 built with go1.22.2`; `go version go1.26.0`; `backend-go/go.work` có `go 1.26.0` và 22 mục `use` — golangci-lint v1 xây bằng Go cũ hơn module thường **từ chối chạy**; chưa chạy để xác nhận |
| `backend-go/proto/buf.yaml` | `version: v2`, lint `STANDARD`, breaking `FILE`; `buf` 1.72.0; `.github/workflows/backend-go-mcp-service.yml:36` dùng `buf breaking --path orca/mcp --against '../../.git#branch=origin/main,subdir=backend-go/proto'` |
| `backend-go` CI | 18 workflow `backend-go-*` (đếm `ls`): `go build`, `go vet`, `go test ./...`, và `-tags=integration` (testcontainers); `grep -l golangci` và `grep -l "opa "` trên toàn `.github/workflows/*.yml` đều rỗng (**CI không chạy `golangci-lint` và `opa test`**); `buf` chỉ có ở `backend-go-mcp-service.yml` (`--path orca/mcp`) |

### 1.3 Môi trường của agent không đủ cho các công cụ này

- `agent-config.ts` `buildToolPath` (dòng ~50) chỉ gồm `~/.local/bin`, `~/bin`, `/usr/local/bin`, `/usr/bin`, `/bin`, `/usr/sbin`, `/snap/bin`. Trên máy khảo sát `golangci-lint`, `buf`, `opa` nằm ở `/home/ubuntu/go/bin`, **ngoài PATH của agent**; `go` ở `/usr/bin/go` thì có.
- `toolEnv` (dòng 81-88) = `{...process.env, PATH, HOME, ANTHROPIC_API_KEY, GITHUB_TOKEN, GH_TOKEN}`: chứa **toàn bộ biến môi trường của agent cộng khoá AI và token GitHub**. Không được truyền nguyên cho tiến trình kiểm tra.
- `runToolCommand` (`agent-tool-registry.ts:72-113`): `spawn` không `detached`, timeout chỉ `child.kill('SIGTERM')` (không diệt cây), không giới hạn đầu ra. `killProcessTree` ở `agent-exec-handler.ts` dùng `taskkill /pid /T /F` trên Windows nhưng trên POSIX chỉ `child.kill('SIGKILL')` (không diệt cây) và không xuất khẩu (ghi chú "duplicated rather than imported because the relay ships to remote hosts"). `pty-daemon-client.ts:84` mới là nơi dùng `detached: true`.
- Test chạy mã của worktree: `vitest.config.ts`, mã test, `TestMain` của Go đều thực thi trong tiến trình con (research 11 §7).

### 1.4 Vấn đề cần giải

Một bề mặt hẹp `quality.*`: backend chỉ nói **tên profile + phạm vi**, agent tự biết lệnh nào, chạy ở đâu, giới hạn gì; kết quả là mã thoát + đầu ra thô cho parser (CR-CV-082); có tiến độ, huỷ thật (cây tiến trình), không làm cạn máy, không lộ secret, báo thiếu môi trường thay vì trả kết quả sai.

## 2. Giải pháp đề xuất

### 2.1 Cấu trúc file (mới, phẳng trong `agent/src/relay/`, tên theo khái niệm)

| File | Nội dung |
|---|---|
| `agent-rpc-dispatch-quality.ts` | `dispatchQualityRpc(rpc, config, log, ws, state)`: trả `null` nếu method không bắt đầu bằng `quality.`; mẫu `agent-rpc-dispatch-misc.ts` (import động, `try/catch`, `makeError`). Thêm vào `route()` ngay sau khối `dispatchCodeIntelRpc` (CR-CV-001 2.1) |
| `quality-method-table.ts` | Bảng `QUALITY_METHODS` (tên → `{validate, handle}`): `quality.listProfiles`, `run`, `runStatus`, `cancel`, `results` (và `coverage` do CR-CV-083 thêm dòng) |
| `quality-profile-schema.ts` | Kiểu `QualityCheckProfile`, `QualitySuite`, hàm kiểm hợp lệ định nghĩa (viết tay, không thêm phụ thuộc) |
| `quality-profile-catalog.ts` | Catalog **tích hợp** cho repo Orca (mục 2.3), là dữ liệu có phiên bản đi cùng bản agent |
| `quality-profile-host-overrides.ts` | Đọc tệp cấu hình **phía host** `~/.orca/quality/profiles.json` (mục 2.2) |
| `quality-run-planning.ts` | Mở suite thành `PlannedStep[]`, thay mẫu `{bin:…}`, `{files}`, chọn module Go theo phạm vi, kiểm `cwd` nằm trong repo |
| `quality-changed-files.ts` | Tính danh sách tệp đổi theo `scope` (mục 2.5) |
| `quality-environment-preflight.ts` | Preflight (mục 2.4) |
| `quality-run-manager.ts` | Máy trạng thái run, khoá theo worktree, hàng đợi, liên kết cổng nặng |
| `quality-run-step-executor.ts` | Spawn một bước, ghi stdout/stderr ra tệp, giám sát kích thước, timeout, gọi parser (CR-CV-082) |
| `quality-process-tree-kill.ts` | Diệt cả cây (mục 2.7) |
| `quality-child-env.ts` | Dựng biến môi trường tiến trình con theo danh sách cho phép (mục 2.8) |
| `quality-output-redaction.ts` | Che secret và đường dẫn tuyệt đối trong log |
| `quality-run-journal.ts` | Nhật ký run trên đĩa để biết `interrupted` sau khi agent khởi động lại (giống CR-CV-004 2.5) |
| `quality-results-store.ts` | Lưu tạm phát hiện/log theo `runId` (TTL), phân trang cho `quality.results` |
| `quality-limits.ts` | Hằng giới hạn, ghi đè qua `ORCA_QUALITY_*` |
| `agent-heavy-job-gate.ts` | Cổng "việc nặng" dùng chung với `codeintel.reindex` (mục 2.9) |

Sửa file có sẵn (nhỏ): `agent-rpc-dispatch.ts` (một khối `dispatchQualityRpc`; `extractTraceFields` chỉ ghi `workspaceRoot`, tên method, tên profile), `agent-session-capabilities.ts` (thêm `quality` khi dispatcher đã đăng ký và có ít nhất một profile `ready`), `agent-session.ts` `stop()` (không huỷ run; chỉ dừng bộ phát thông báo). Test mới ở mục 5.

### 2.2 Profile có tên: định nghĩa ở đâu và vì sao

**Quyết định:** định nghĩa lệnh thực chạy nằm ở **phía agent/dev server**, gồm hai lớp; backend chỉ lưu **chính sách** dùng tên profile.

| Lớp | Nơi | Ai sửa | Nội dung |
|---|---|---|---|
| L1 catalog tích hợp | `quality-profile-catalog.ts` trong bản agent | Người phát hành agent (PR code, có review) | Profile cho repo Orca (mục 2.3) |
| L2 ghi đè theo host | `~/.orca/quality/profiles.json` (quyền `0600`, thư mục `0700`; **ngoài mọi worktree**) | Quản trị dev server | Thêm/bớt profile cho repo khác; chỉ cho phép **thêm profile mới hoặc vô hiệu hoá profile có sẵn**, không đổi `argv` của profile tích hợp trừ khi khai báo `replace: true` kèm lý do (ghi log) |
| Chính sách ở backend | bảng `quality_profiles` (README 3.10), CR-CV-085 | Người có quyền chỉnh cổng | Tên profile **bắt buộc/tuỳ chọn**, ngưỡng, chế độ `inform\|block`; **không chứa lệnh** |

Vì sao không đặt định nghĩa trong repo (ví dụ `.orca/quality.json` trong worktree):

1. **Nội dung worktree do agent sinh hoặc do nhánh bên ngoài tạo.** Nếu lệnh nằm trong repo, agent (hoặc PR độc hại) sửa tệp rồi nhờ "kiểm tra" chạy lệnh tuỳ ý với quyền của dev server: phá vỡ nguyên tắc O11 "không nhận lệnh tuỳ ý". Hiện agent đã chạy được lệnh tuỳ ý theo yêu cầu người dùng, nhưng đó là hành động có chủ đích từng lần, còn "bấm kiểm tra" là hành động thông thường.
2. **Backend gửi lệnh** (bảng `quality_profiles` chứa argv) mở lại đường `args` tự do mà D5 cấm; người sửa bảng ở backend không nhất thiết là người tin cậy trên dev server.
3. **Catalog đi cùng phiên bản agent** nên có thể kiểm thử bằng fixture (mục 5) và đồng bộ với parser (CR-CV-082).

Repo vẫn được phép **tham số hoá** profile có sẵn mà không định nghĩa lệnh: tệp `.orca/quality.json` trong repo chỉ có thể chọn *tập con* profile áp dụng cho repo đó và đặt `timeoutMs` nhỏ hơn mặc định (không lớn hơn). Việc đọc tệp này là P1, chưa thiết kế ở đây (Q3).

Định nghĩa một profile kiểm tra (`QualityCheckProfile`):

```jsonc
{
  "id": "ts-lint",                       // [a-z0-9][a-z0-9-]{0,47}, duy nhất
  "title": "oxlint (TS/JS)",
  "kind": "lint",                        // lint|typecheck|test|proto|policy|repo-rules|coverage (CR-CV-083 thêm)
  "parser": "oxlint@json",               // khoá parser ở CR-CV-082
  "cwd": ".",                            // tương đối gốc worktree; realpath phải nằm trong worktree
  "argv": ["{bin:oxlint}", "--format", "json", "{files|.}"],   // chỉ dùng mẫu ở dưới
  "scopes": ["worktree", "changed", "commitRange"],
  "scopeStrategy": "append-files",       // append-files | full-run-filter | go-modules | none
  "fileGlobs": ["**/*.{ts,tsx,js,jsx,mjs,cjs}"],
  "timeoutMs": 600000,
  "maxOutputBytes": 33554432,
  "heavy": false,                        // true: dùng cổng việc nặng (2.9)
  "needsNativeRuntime": false,
  "network": false,                      // true: không đặt GOPROXY=off ...
  "env": { "set": { "CI": "1", "NO_COLOR": "1" }, "allowExtra": [] },
  "requires": [ { "type": "bin", "name": "oxlint" }, { "type": "file", "path": ".oxlintrc.json" } ],
  "exit": { "ok": [0], "findings": [1] }   // mã thoát không nằm trong hai tập = bước `failed`
}
```

Mẫu thay thế **chỉ** gồm: `{bin:<tên>}` (phân giải qua bộ phân giải công cụ, 2.4), `{files}`/`{files|<mặc định>}` (danh sách tệp đã lọc, đã kiểm hợp lệ), `{tmp:<tên>}` (đường dẫn trong thư mục tạm của run; ví dụ `{tmp:vitest.json}`), `{base}` (chỉ khi phạm vi có `base`, đã kiểm). Mọi token khác trong `argv` là chuỗi hằng. Không có shell (`shell:false`), không nội suy biến môi trường. `argv[0]` luôn là `{bin:…}` hoặc `node`; đường dẫn script (`config/scripts/*.mjs`) là hằng và phải nằm trong `cwd`.

Kiểm tra giá trị chèn vào argv: tên tệp bị từ chối nếu có NUL, ký tự điều khiển, bắt đầu bằng `-`, hoặc `realpath` thoát khỏi worktree (symlink); `base` khớp `^[0-9a-f]{7,64}$` hoặc tên ref theo `git check-ref-format --branch` và không bắt đầu bằng `-`.

### 2.3 Profile mặc định cho Orca (đề xuất; **chưa chạy thật**; sự tồn tại tệp đã kiểm)

Đường dẫn tương đối gốc repo. "Nặng" = dùng cổng việc nặng. Mọi `cwd` là thư mục có tệp cấu hình tồn tại (đã `ls`/`[ -e ]`).

| id | cwd | argv (sau khi thay mẫu) | Phạm vi | Nặng | Cần môi trường | Ghi chú đã kiểm |
|---|---|---|---|---|---|---|
| `ts-lint` | `.` | `node_modules/.bin/oxlint --format json [files]` | worktree, changed | không | `node_modules/.bin/oxlint`, `.oxlintrc.json` | Gốc `pr.yml:64` chạy `oxlint --format github`; `json` ở đây là chọn của CR này (CR-CV-082 2.4 nêu dự phòng `github`) |
| `ts-typecheck-desktop-node` | `desktop` | `tsc --noEmit -p config/tsconfig.node.json --pretty false` | worktree (changed = chạy đủ rồi lọc) | **có** | `tsc`, `desktop/config/tsconfig.node.json` | Script gốc `typecheck:node` trỏ `config/tsconfig.node.json` ở gốc (không có); tệp thật ở `desktop/config/` |
| `ts-typecheck-desktop-web` | `desktop` | `tsc --noEmit -p config/tsconfig.tc.web.json --pretty false` | như trên | có | như trên | idem (`typecheck:web`) |
| `ts-typecheck-desktop-cli` | `desktop` | `tsc --noEmit -p config/tsconfig.tc.cli.json --pretty false` | như trên | có | như trên | idem (`typecheck:cli`) |
| `ts-typecheck-agent` | `agent` | `tsc --noEmit -p tsconfig.json --pretty false` | như trên | có | `agent/tsconfig.json` | **Chưa chắc chạy được**: `composite: true` (đọc `agent/tsconfig.json`); root có biến thể `--composite false` cho `typecheck:tsc:*`; cần thử trước khi bật |
| `ts-typecheck-frontend` | `frontend` | `tsc --noEmit -p tsconfig.json --pretty false` | như trên | có | `frontend/tsconfig.json` | Không có trong script gốc; thêm vì `frontend` là workspace; chưa chạy |
| `ts-unit-desktop` | `desktop` | `vitest run --config config/vitest.config.ts --reporter=json --outputFile={tmp:vitest.json} [related files]` | worktree, changed (`vitest related`) | **có** | `node-pty` nạp được (`ensure-native-runtime.mjs --check-only`) | **Không** gọi `pnpm test`/`--runtime=node` (có thể `pnpm rebuild`, ghi `node_modules`). cwd phải là `desktop` (1.2) |
| `ts-unit-frontend` | `frontend` | `vitest run --config config/vitest.config.ts --reporter=json --outputFile={tmp:vitest.json}` | worktree, changed | có | `vitest` | `frontend/config/vitest.config.ts` include `src/**/*.test.{ts,tsx}` |
| `ts-unit-agent` | `agent` | `vitest run --reporter=json --outputFile={tmp:vitest.json}` | worktree, changed | có | `vitest` | CI hiện **không** chạy test của `agent/` (README v7 mục 8 điểm 16); thêm vì đây là nơi `quality-*.ts` sống |
| `ts-unit-backend` | `backend` | `vitest run --reporter=json --outputFile={tmp:vitest.json}` | worktree, changed | có | `vitest` | |
| `repo-check-max-lines` | `.` | `node config/scripts/check-max-lines-ratchet.mjs` | worktree | không | tệp tồn tại ở gốc | Đọc `git ls-files` và `config/max-lines-baseline.txt` (354 dòng) |
| `repo-check-styled-scrollbars` | `desktop` | `node config/scripts/check-styled-scrollbars.mjs` | worktree | không | `desktop/src/renderer/src` | Script quét `<cwd>/src/renderer/src` |
| `repo-check-reliability-gates` | `desktop` | `node config/scripts/check-reliability-gates.mjs` | worktree | không | `desktop/config/reliability-gates.jsonc` | |
| `go-vet` | `backend-go/<module>` | `go vet -json ./...` | worktree, changed (`go-modules`) | **có** | `go`, module cache | Một bước mỗi module. `-json` để có tên analyzer cho `ruleId`; ở chế độ này mã thoát có thể là 0 dù có phát hiện (theo tài liệu của `go vet`, chưa chạy) nên trạng thái bước suy từ parser, không từ mã thoát (CR-CV-082 2.5); `go vet ./...` dạng text là dự phòng của parser |
| `go-test` | `backend-go/<module>` | `go test -json ./...` | worktree, changed (`go-modules`) | **có** | `go`, module cache | **Không** có `-tags=integration` (cần Docker, mục 2.10) |
| `go-lint` | `backend-go/<module>` | `golangci-lint run --out-format json ./...` | worktree, changed | **có** | `golangci-lint` đủ mới so với `go.work` | Config tìm ngược lên `backend-go/.golangci.yml` (hành vi mặc định của công cụ, chưa kiểm chứng ở đây) |
| `proto-lint` | `backend-go/proto` | `buf lint --error-format json` | worktree | không | `buf` | Makefile `\|\| true` không áp dụng |
| `proto-breaking` | `backend-go/proto` | `buf breaking --against <gitCommonDir>#branch=<base>,subdir=backend-go/proto --error-format json` | `commitRange`/`changed` (cần `base`) | không | `buf`, nhánh `base` tồn tại | `<gitCommonDir>` = `git rev-parse --git-common-dir` (tuyệt đối), vì `.git` của worktree liên kết là tệp (đã đọc `.git` của `dev-process-9beda3`: `gitdir: …/.git/worktrees/…`); `buf` có chấp nhận chưa kiểm chứng |
| `opa-test` | `backend-go` | `opa test policy/orca-authz/ --format json` | worktree | không | `opa` | Makefile dùng `-v`; `--format json` là cờ có trong `opa test --help` |

Suite (gom nhiều profile thành một run, một `runId`): `fast` = `ts-lint`, `repo-check-*`, `go-vet`; `standard` = `fast` + `ts-typecheck-*`, `ts-unit-*`, `go-test`, `proto-lint`, `opa-test`; `full` = `standard` + `go-lint`, `proto-breaking`. `quality.run.profile` nhận id profile **hoặc** id suite. Tên suite/phạm vi mặc định là đề xuất, chưa có dữ liệu thời gian (mục 6).

Không đưa vào catalog (ghi rõ để không bị hiểu nhầm là bỏ sót): `lint:switch-exhaustiveness` (script gốc trỏ `src/main … config/oxlint-switch-exhaustiveness.json` không tồn tại ở gốc; bản ở `desktop/config/` chưa kiểm chứng đường dẫn bên trong), `verify:localization-*` (nặng, thuộc CR-CV-084), Playwright e2e, `go test -tags=integration`, `build:*`, `bench:*`.

### 2.4 Preflight môi trường (`quality-environment-preflight.ts`) → `CODEINTEL_ENV_NOT_READY`

Preflight **không bao giờ cài đặt hay ghi**: không `pnpm install`, không `go mod download`, không `pnpm rebuild`. Chỉ đọc và gợi ý.

| Kiểm tra | Cách làm (chỉ đọc) | Khi thất bại (`missing[].reason`) |
|---|---|---|
| Bộ phân giải công cụ `{bin:<tên>}` | Tìm theo thứ tự: `<cwd>/node_modules/.bin/<tên>`, `<repoRoot>/node_modules/.bin/<tên>`, rồi PATH mở rộng `qualityToolPath` = `toolPath` của agent + `~/go/bin` + `$(go env GOPATH)/bin` + `/usr/local/go/bin` + `~/.local/share/pnpm`. Dùng `fs.access(X_OK)`; trên Windows thêm `PATHEXT` (MVP chưa hỗ trợ win32, 2.10) | `binary_missing` (`tool`, `searched[]`) |
| `node_modules` | `<repoRoot>/node_modules/.modules.yaml` tồn tại (đã thấy ở gốc); cảnh báo (không chặn) nếu `pnpm-lock.yaml` mới hơn `.modules.yaml` | `node_modules_missing` |
| Native runtime (profile `needsNativeRuntime`) | `node <desktop>/config/scripts/ensure-native-runtime.mjs --check-only` (nhánh chỉ-đọc, 1.2); timeout 15 s; chạy bằng `process.execPath` | `native_runtime_unavailable` (`module`, `message` đã che đường dẫn); gợi ý "chạy `pnpm rebuild node-pty` thủ công" |
| Go | `go version` ≥ chỉ thị `go` trong `backend-go/go.work` (đọc tệp, parse dòng `go X.Y.Z`); `go env GOMODCACHE` tồn tại và không rỗng | `go_missing`, `go_too_old`, `go_modcache_empty` |
| `golangci-lint` | `golangci-lint --version` (in `built with goX.Y`); nếu `built with` < `go` của `go.work` thì không chạy | `tool_too_old` (`built`, `required`) |
| `buf`, `opa` | `--version` | `binary_missing` |
| Git ref cho `proto-breaking` | `git rev-parse --verify --quiet <base>^{commit}` | `base_ref_missing` |
| Dung lượng tạm | `fs.statfs(os.tmpdir())` còn ≥ 256 MiB | `tmp_space_low` |
| `qualityToolPath`/`HOME` | `HOME` được đặt (Go/pnpm cần) | `home_missing` |

Phân loại sau khi chạy (suy từ stderr/đầu ra, do parser quyết, CR-CV-082 2.5 `envFailure`): Go `cannot find module providing package`, `missing go.sum entry`, `no required module provides package`; Node `Cannot find module`, `ERR_MODULE_NOT_FOUND`, `command not found`. Khi gặp, bước kết thúc `env_not_ready` thay vì `failed`, không tạo phát hiện giả.

Kết quả preflight cache 60 s theo `(repoRoot, profileId)`; `quality.listProfiles` trả `ready` và `missing[]` **không lỗi** (backend không phụ thuộc mã lỗi qua `RelayByDevServer`, vì `error.data.code` của agent bị làm mất, README v7 mục 8 điểm 4); `quality.run` kiểm lại và trả `CODEINTEL_ENV_NOT_READY` với `data.missing[]` nếu **mọi** bước của run đều không sẵn sàng; nếu chỉ một số bước thiếu, run vẫn chạy các bước còn lại và các bước thiếu có trạng thái `env_not_ready` (không im lặng bỏ qua).

Go giữ môi trường chặt: tiến trình con đặt `GOFLAGS=-mod=readonly`, `GOTOOLCHAIN=local` (không tải toolchain mới), và nếu `network:false` thì `GOPROXY=off` (thiếu module → lỗi ngay thay vì tải về, và ghi cache). Hai biến này là lựa chọn của CR; hiệu ứng lên test cần mạng (hiếm với unit test) chưa kiểm chứng.

### 2.5 Phạm vi chạy (`scope`)

| `scope` | Ý nghĩa | Tệp cần tính |
|---|---|---|
| `worktree` | Toàn bộ cây làm việc hiện tại | không |
| `changed` | Tệp khác `mergeBase(base, HEAD)` **cộng** thay đổi chưa commit và tệp chưa theo dõi | `git diff --name-only -z --diff-filter=ACMR <base>...HEAD` + `git status --porcelain=v1 -z --untracked-files=normal` |
| `commitRange` | Chỉ các commit `base..HEAD` | `git diff --name-only -z --diff-filter=ACMR <base>...HEAD` |

`base` bắt buộc với `changed` và `commitRange` (thiếu → `CODEINTEL_INVALID_PARAMS`). Mọi lệnh git ở trên có từ Git ≤ 2.25 (dưới baseline `guides/reference/git-compatibility.md`); không dùng `--path-format`, `-z` của `worktree list`, hay `git switch`. Chiến lược theo profile (`scopeStrategy`):

- `append-files`: thêm tệp khớp `fileGlobs` vào cuối argv (tối đa 300 tệp mỗi lần gọi; nhiều hơn thì chạy `full-run-filter` và đặt cờ `scopeWidened`). Dùng cho `ts-lint`, vitest `related`.
- `full-run-filter`: công cụ không nhận danh sách tệp (tsc, buf, opa): chạy đủ, **chỉ báo** phát hiện thuộc tệp trong phạm vi; số bị lọc ghi `outsideScopeCount` (không bỏ mất: một lỗi tsc ở tệp không đổi do thay đổi API là thông tin quan trọng; CR-CV-082 có cờ `inScope` trên từng phát hiện).
- `go-modules`: chọn module có tệp đổi (`backend-go/services/<s>/**` → module đó; `backend-go/common/**` hoặc `backend-go/proto/**` → **mọi** module và đặt `scopeWidened`, vì nhiều service phụ thuộc `common`/`proto`); danh sách module đọc từ khối `use (...)` của `backend-go/go.work`, mỗi đường dẫn phải nằm trong `backend-go`.
- `none`: luôn chạy đủ.

`dirtyFingerprint` = sha256 của (danh sách `git status --porcelain=v1 -z` + mtime/size từng tệp, cắt ở 5000 mục), tính lúc bắt đầu và lúc kết thúc; khác nhau → `workTreeChangedDuringRun: true` (agent còn đang sửa) và backend phải coi kết quả là chưa chắc chắn (CR-CV-085).

### 2.6 Hợp đồng RPC

Mọi method nhận `workspaceRoot` (phân giải qua `resolveCodeIntelRepo` của CR-CV-001 2.4: tuyệt đối, realpath, là gốc worktree git; sai → `CODEINTEL_PATH_NOT_ALLOWED`). Tham số lạ → `CODEINTEL_INVALID_PARAMS` (`data.field`). **Không có** tham số `command`, `argv`, `args`, `env`, `cwd`, `timeout` từ client.

**`quality.listProfiles`** `{workspaceRoot}` →

```jsonc
{ "profiles": [
    { "id": "ts-lint", "title": "oxlint (TS/JS)", "kind": "lint", "scopes": ["worktree","changed","commitRange"],
      "heavy": false, "ready": true, "missing": [], "definitionHash": "sha256:9f…", "source": "builtin",
      "display": "oxlint --format json" },
    { "id": "go-lint", "kind": "lint", "ready": false,
      "missing": [ { "check": "golangci-lint", "reason": "tool_too_old", "built": "go1.22.2", "required": "go1.26.0",
                     "hint": "install a golangci-lint built with Go >= 1.26" } ] } ],
  "suites": [ { "id": "fast", "profiles": ["ts-lint","repo-check-max-lines","go-vet"] } ],
  "host": { "platform": "linux", "cores": 32, "loadavg1": 3.1, "freeMemBytes": 7700000000 },
  "limits": { "maxConcurrentRuns": 1, "queueMax": 4, "runTimeoutMs": 2700000 } }
```

`definitionHash` cho phép backend ghim chính sách vào đúng phiên bản định nghĩa (CR-CV-085 phát hiện trôi). `display` chỉ để người đọc, không chứa giá trị biến môi trường.

**`quality.run`** `{workspaceRoot, profile, scope, base?}` → trả ngay (< 1 s):

```jsonc
{ "runId": "qr_01JA0X8E5T", "state": "queued|running", "profile": "standard", "scope": "changed",
  "steps": [ { "id": "ts-lint", "title": "oxlint (TS/JS)" }, { "id": "go-test:services/project-service", "title": "go test" } ],
  "headCommit": "1b0c760935…", "dirtyFingerprint": "sha256:…", "queuePosition": 0 }
```

Lỗi: `CODEINTEL_PROFILE_UNKNOWN` (`data.available[]`), `CODEINTEL_ENV_NOT_READY` (`data.missing[]`), `CODEINTEL_RUN_IN_PROGRESS` (`data.runId`, `data.reason` = `worktree_busy`|`queue_full`), `CODEINTEL_INVALID_PARAMS`, `CODEINTEL_PATH_NOT_ALLOWED`, `CODEINTEL_TOOL_UNAVAILABLE (reason=quality_disabled)` khi `ORCA_QUALITY_RUN=off` hoặc nền tảng win32 (2.10).

**`quality.runStatus`** `{runId}` → `{ runId, state: "queued|running|cancelling|succeeded|failed|cancelled|interrupted", startedAt, finishedAt?, steps: [{id, status, exitCode?, durationMs?, findings: {error,warning,info}, truncated, envMissing?}], headCommit, dirtyFingerprint, workTreeChangedDuringRun, summary? }`. `runId` lạ → `CODEINTEL_RUN_NOT_FOUND` (**mã mới**, Điều chỉnh hợp đồng). Idempotent, đọc từ bộ nhớ hoặc nhật ký.

**`quality.cancel`** `{runId}` → trạng thái hiện tại; idempotent (run đã kết thúc: trả trạng thái cuối, không lỗi). Chuyển `cancelling`, diệt cây (2.7), kết thúc `cancelled`; trong bước đang chạy giữ phát hiện đã parse được (đánh dấu `partial`).

**`quality.results`** `{runId, offset, limit, view?, stepId?}` — README chỉ có `runId, offset, limit`; thêm `view` tuỳ chọn (Điều chỉnh): `findings` (mặc định, ≤ 500 mục/trang, đúng `QualityFinding` của CR-CV-082 trừ `fingerprint` có thể tính ở backend nhưng CR-CV-082 chốt tính ở agent), `steps`, `log` (văn bản đã che, tối đa 64 KiB/trang, chỉ khi `stepId`). Kết quả có phần đầu `{ totalCount, truncated, outsideScopeCount, nextOffset|null }`. Agent giữ kết quả tối đa `ORCA_QUALITY_RESULT_TTL_MS` = 1 h và 20 run/worktree; backend phải ghi vào `quality_findings` (CR-CV-082) khi nhận `quality.finished`.

**Thông báo agent → backend** (qua `codeintel-notification-sink.ts`, CR-CV-004 2.6):

```jsonc
{ "jsonrpc": "2.0", "method": "quality.progress",
  "params": { "runId": "qr_01JA0X8E5T", "stage": "step:ts-lint", "stepIndex": 1, "stepCount": 6,
              "percent": 16, "message": "ts-lint running", "at": "2026-10-06T08:00:12.100Z" } }
{ "jsonrpc": "2.0", "method": "quality.finished",
  "params": { "runId": "qr_01JA0X8E5T", "status": "succeeded",
              "summary": { "error": 3, "warning": 12, "info": 0, "stepsTotal": 6, "stepsWithFindings": 2,
                           "stepsFailed": 0, "stepsEnvNotReady": 1, "outsideScope": 4 },
              "steps": [ { "id": "ts-lint", "status": "findings", "exitCode": 1, "durationMs": 21044, "truncated": false } ],
              "headCommit": "1b0c760935…", "dirtyFingerprint": "sha256:…", "workTreeChangedDuringRun": false,
              "startedAt": "…", "finishedAt": "…" } }
```

`percent` là **`completedSteps/stepCount`** (số thật, chỉ đổi khi một bước xong) hoặc `null`; không suy diễn theo thời gian (nhất quán F10 của CR-CV-001). Trong bước dài, `message` chứa dòng đầu ra gần nhất đã che (≤ 200 ký tự, ≤ 1 thông báo/giây/run). Thông báo mất khi ws đóng (không xếp hàng vô hạn): sau khi nối lại backend gọi `quality.runStatus` cho mọi run chưa kết thúc.

Ngữ nghĩa trạng thái (chốt ở đây, vì README 3.10 chưa nói):

| Cấp | Giá trị | Nghĩa |
|---|---|---|
| Bước | `passed` | exit trong `exit.ok`, 0 phát hiện |
| | `findings` | exit trong `exit.findings` hoặc có phát hiện; công cụ chạy hết |
| | `failed` | exit lạ, crash, parser lỗi (`format_drift`), hoặc `outputTooLarge` |
| | `timeout`, `cancelled`, `skipped` (không có tệp áp dụng), `env_not_ready` | |
| Run `QualityRun.status` | `succeeded` | mọi bước chạy tới cùng (kể cả có phát hiện) |
| | `failed` | có bước `failed`, `timeout` hoặc `env_not_ready` (kết quả **không đầy đủ**; cổng phải coi là `unknown`) |
| | `cancelled` | người dùng huỷ |

Có phát hiện **không** làm run `failed`: cổng (CR-CV-085) đánh giá mức nghiêm trọng, bộ chạy chỉ thu thập.

### 2.7 Chạy nền, huỷ cây tiến trình, một run mỗi worktree

- **Một run mỗi worktree:** khoá theo `realpath(workspaceRoot)`; run thứ hai → `CODEINTEL_RUN_IN_PROGRESS` với `runId` đang chạy (backend gắn vào, không tạo mới). Khác worktree vào **hàng đợi** (`queued`, tối đa `ORCA_QUALITY_QUEUE_MAX` = 4; vượt → `RUN_IN_PROGRESS reason=queue_full`).
- **Tuần tự trong một run:** các bước chạy lần lượt (không song song) để chặn bùng tài nguyên; song song nằm trong từng công cụ và bị giới hạn (2.8).
- **Spawn:** `spawn(file, argv, { cwd, env, shell:false, detached: process.platform !== 'win32', stdio: ['ignore', fdStdout, fdStderr], windowsHide: true })`. Stdout/stderr ghi ra tệp trong `<os.tmpdir()>/orca-quality-<uid>/<runId>/` (`mkdtemp`, thư mục `0700`, tệp `0600`, cờ `wx`), không qua pipe (cùng lý do cắt cụt pipe của `gitnexus`, CR-CV-001 1.9; chưa kiểm chứng với các công cụ ở đây nhưng tệp loại bỏ rủi ro). `detached: true` đưa tiến trình con vào nhóm tiến trình riêng để diệt cả nhóm.
- **Giám sát:** mỗi 500 ms `fs.stat` tệp đầu ra; vượt `maxOutputBytes` của profile (mặc định 32 MiB, `go-test` 64 MiB vì `-json` rất dài) thì diệt cây và bước `failed (outputTooLarge)` (không cắt im lặng để parser không đọc JSON cụt). Quá `timeoutMs` → diệt cây, `timeout`, **vẫn** thử parse phần đã ghi (đánh dấu `partial`). Toàn run có `runTimeoutMs` mặc định 45 phút (giả định, chưa đo).
- **Huỷ/diệt cây (`quality-process-tree-kill.ts`):** POSIX: `process.kill(-pid, 'SIGTERM')`; chờ 5 s; còn sống thì `process.kill(-pid, 'SIGKILL')`; kiểm `process.kill(-pid, 0)` ném `ESRCH` trong ≤ 10 s, nếu không ghi `orphanSuspected` và log (không nuốt im lặng). Windows: `taskkill /pid <pid> /T /F` (mẫu đã có ở `agent-exec-handler.ts killProcessTree`, nhưng hàm đó không xuất khẩu và trên POSIX chỉ kill một tiến trình nên **không dùng lại được**; viết mới có test). Hạn chế đã biết: tiến trình con tự `setsid`/double-fork (hiếm với các công cụ này, `pnpm`/`vitest` workers dùng cùng nhóm) thoát khỏi `kill(-pid)`.
- **Khi mất kết nối ws:** run **tiếp tục** (cùng triết lý CR-CV-004); chỉ bộ phát thông báo bị gỡ khỏi `stop()`. Khi agent thoát có chủ đích: diệt mọi nhóm của run đang chạy. Nhật ký `~/.orca/quality/runs/<runId>.json` (thư mục `0700`, ghi lúc bắt đầu/kết thúc, ≥ 50 bản gần nhất, ghi `pgid` và `startedAt`); sau khi agent khởi động lại, run còn `running` đánh dấu `interrupted` (không tự chạy lại, không tự diệt tiến trình mồ côi vì không xác minh được `pgid` còn là của ta; ghi log cảnh báo kèm `pgid`).
- **Dọn:** xoá thư mục tạm của run khi hết `RESULT_TTL`; khi agent khởi động dọn `orca-quality-*` cũ hơn 2 giờ.

### 2.8 Giới hạn tài nguyên, biến môi trường, che secret

| Giới hạn | Mặc định | Cách thực thi | Mức bảo đảm |
|---|---|---|---|
| Ưu tiên CPU | `nice` 10 | `os.setPriority(pid, 10)` ngay sau spawn (đa nền tảng) | mềm |
| Song song trong công cụ | `vitest --maxWorkers=<max(1,cores/4)>`, Go `GOMAXPROCS=<max(2,cores/4)>` và `-p` qua `GOFLAGS`, `golangci-lint --concurrency` (cờ có trong `run --help`; chưa chốt giá trị) | thêm vào argv/env theo profile `limits` | mềm |
| Bộ nhớ Node | `NODE_OPTIONS=--max-old-space-size=<MB>` | env | mềm (chỉ heap V8) |
| Bộ nhớ Go | `GOMEMLIMIT=<bytes>` | env | mềm |
| Cứng (tuỳ chọn, P1) | cgroup v2 qua `systemd-run --user --scope -p MemoryMax=… -p CPUQuota=…` | bọc `argv` khi `ORCA_QUALITY_CGROUP=on` và probe thành công | `systemd-run` có ở máy khảo sát, cgroup v2 có (`/sys/fs/cgroup/cgroup.controllers`); chạy được dưới phiên người dùng của agent **chưa kiểm chứng** |
| Thời gian | timeout từng bước + run | 2.7 | cứng (diệt cây) |
| Đầu ra | `maxOutputBytes` | 2.7 | cứng |
| Đồng thời | 2.9 | | cứng |

`RLIMIT_AS` (`prlimit --as`) **không dùng**: làm hỏng V8/Go vì cấp phát không gian ảo lớn.

**Biến môi trường tiến trình con** (`quality-child-env.ts`): bắt đầu từ **rỗng**, không từ `config.toolEnv`/`process.env`. Cho phép: `PATH` (`qualityToolPath`), `HOME`, `USER`, `LOGNAME`, `LANG`, `LC_ALL`, `TZ`, `TMPDIR` (thư mục tạm của run), `SHELL`; cố định `CI=1`, `NO_COLOR=1`, `FORCE_COLOR=0`, `TERM=dumb`; Go: `GOFLAGS`, `GOTOOLCHAIN=local`, `GOMAXPROCS`, `GOMEMLIMIT`, `GOCACHE`/`GOPATH`/`GOMODCACHE` **chỉ khi** đã đặt ở môi trường agent (chuyển nguyên giá trị đường dẫn); Node: `NODE_OPTIONS` (từ profile), `PNPM_HOME`, `npm_config_cache` nếu có. Loại luôn mọi biến tên khớp `/(TOKEN|SECRET|PASSWORD|PASSWD|CREDENTIAL|API_?KEY|PRIVATE|DSN|AUTH|COOKIE|SESSION)/i` kể cả khi profile xin qua `env.allowExtra` (từ chối kèm log), và đặc biệt `ANTHROPIC_API_KEY`, `GITHUB_TOKEN`, `GH_TOKEN`, `AGENT_TOKEN`, `ORCA_*`, `SSH_AUTH_SOCK`, `AWS_*`, `GOOGLE_*`. Test bảo đảm không biến nào trong `config.toolEnv` bị khoá trên (1.3) lọt sang.

Việc không truyền secret **không** ngăn mã test đọc `~/.orca/credentials`, `~/.ssh` hay mạng với quyền của người dùng dev server (mục 2.10, 6).

**Che đầu ra** (`quality-output-redaction.ts`): trước khi lưu/gửi `message`, log hay phát hiện: (1) thay giá trị chính xác của mọi biến môi trường của agent khớp mẫu secret (độ dài ≥ 8) bằng `***`; (2) mẫu token phổ biến (`gh[pousr]_…`, `sk-…`, `AKIA…`, JWT `eyJ….….…`, `-----BEGIN … PRIVATE KEY-----`, URL `scheme://user:pass@`); (3) đường dẫn tuyệt đối của worktree → `<repo>`, `HOME` → `~`, thư mục tạm → `<tmp>` (chuẩn hoá đường dẫn cho phát hiện nằm ở CR-CV-082 2.6). Bản che là tối thiểu, không đảm bảo bắt hết secret dạng tự do; log thô không rời agent ngoài `view=log` và vẫn đi qua bộ che.

### 2.9 Giới hạn đồng thời toàn máy: cổng việc nặng chung

`agent-heavy-job-gate.ts` (mới): một semaphore trong tiến trình, `maxHeavyJobs = ORCA_HEAVY_JOBS` mặc định **1**, chia sẻ giữa `quality.run` (bước `heavy`) và `codeintel.reindex` (CR-CV-004; một dòng thay đổi ở đó, nêu ở 2.11). Lý do: `gitnexus analyze`, `go test` toàn bộ, `tsc` đều nặng CPU/RAM/đĩa và cùng chạy cạnh PTY của agent AI. Bước không `heavy` (lint, repo-check, `buf`, `opa`) không qua cổng. Hàng đợi chờ cổng tối đa `ORCA_HEAVY_QUEUE_WAIT_MS` = 10 phút rồi bước `skipped` với `reason=gate_timeout` (không âm thầm). Nhiều tiến trình agent trên cùng máy (nhiều người dùng) không chia sẻ cổng trong bộ nhớ; thêm khoá tệp `~/.orca/locks/heavy-job-<n>.lock` (`wx` + kiểm pid sống) là P1, chưa thiết kế (Q4). Backend cũng giới hạn theo dev server (CR-CV-013) nhưng không thay thế cổng ở agent vì nó không thấy `reindex` do người dùng bấm.

### 2.10 Rủi ro chạy mã không tin cậy và cách ly tối thiểu

Chạy `vitest`, `go test`, `oxlint` với cấu hình/plugin của repo thực thi mã trong worktree (do agent sinh hoặc nhánh bên ngoài). Các biện pháp, theo mức:

| Mức | Có | Không ngăn được |
|---|---|---|
| L0 (MVP, mặc định) | Chỉ profile có tên (2.2); không shell; `cwd` ràng buộc trong worktree qua `realpath`; env allowlist (2.8); nhóm tiến trình riêng và diệt cây; `nice`; timeout/đầu ra; cổng nặng; công tắc `ORCA_QUALITY_RUN=off`; quyền ghi project ở backend (O11, CR-CV-013); audit | Mã test đọc/ghi mọi thứ người dùng dev server đọc/ghi được (`~/.ssh`, `~/.orca/credentials`, `~/.gitnexus`), kết nối mạng, đào tiền: giống bề mặt `shell.exec` đã có của agent (CR-CV-001 2.9), **không** thu hẹp nó |
| L1 (tuỳ chọn, P1) | `ORCA_QUALITY_ISOLATION=bwrap`: bọc bằng `bwrap --ro-bind / / --bind <worktree> <worktree> --tmpfs <HOME>… --unshare-net --die-with-parent` (cần bind lại `GOCACHE`, kho `pnpm`, `node_modules` ghi được); `bwrap` có ở máy khảo sát (`/usr/bin/bwrap`) nhưng **chưa thử** với các công cụ này, và chặn mạng làm hỏng test cần mạng | Rò qua `/proc`, kernel exploit |
| L2 (ngoài phạm vi) | Người dùng OS riêng / container / VM tạm | |

Chính sách đề xuất (cần duyệt, README O11): **ai kích hoạt** = người có quyền ghi trên project (CR-CV-013); **nhánh không tin cậy** = nếu backend biết worktree thuộc PR từ fork/ngoài tenant thì mặc định chỉ cho profile `kind ∈ {lint, repo-rules, proto, policy}` mà không chạy `test`/`typecheck` (hai loại này thực thi mã/plug-in); cờ này thuộc CR-CV-085/013, ở đây agent chỉ cần tham số `trust: "trusted"|"untrusted"` — **chưa thêm vào hợp đồng** (Q2). Hiện chưa có nguồn dữ liệu "nhánh không tin cậy" trong series (không bịa).

Windows: `quality.*` ở MVP trả `CODEINTEL_TOOL_UNAVAILABLE (reason=unsupported_platform)` như CR-CV-001 2.7; thiết kế diệt cây đã tính `taskkill`, nhưng bộ phân giải `PATHEXT`, shim `.cmd` (xem `agent-exec-handler.ts resolveWindowsCommand`) chưa kiểm chứng.

### 2.11 Điểm chạm với CR khác (đề xuất; không sửa file của họ)

| CR | Cần |
|---|---|
| CR-CV-001 | Thêm `quality` vào `capabilities`; đưa `resolveCodeIntelRepo` ra dùng chung (đã xuất khẩu); mã lỗi mới `CODEINTEL_PROFILE_UNKNOWN`, `CODEINTEL_ENV_NOT_READY`, `CODEINTEL_RUN_IN_PROGRESS`, `CODEINTEL_RUN_CANCELLED`, `CODEINTEL_RUN_NOT_FOUND` vào `codeintel-errors.ts` (cùng ánh xạ `error.code` số như bảng 2.8 của CR-CV-001) |
| CR-CV-004 | `codeintel-reindex-job.ts` gọi `agent-heavy-job-gate.ts` trước khi spawn `analyze`; dùng chung `codeintel-notification-sink.ts` |
| CR-CV-023 | Timeout phía Go cho `quality.*` (hiện mọi method ngoài `agent.execPrompt` bị 30 s): `quality.listProfiles` có preflight tới ~10 s nên cần ≥ 30 s hoặc dùng cache; `run/status/cancel/results` nhanh; nhận thông báo `quality.progress/finished`; `error.data.code` phải tới được Go |
| CR-CV-080 | `quality.run` ghi `headCommit`/`dirtyFingerprint`; backend lấy `IndexBasis` lúc kết thúc |
| CR-CV-006 (Part B, `relay-ssh`) | **Để sau.** Đường `relay-ssh` do Go khởi tạo chạy `agent.js --stdio` (Part A, CR-CV-001 1.8) nên method mới tự có mặt; bản `desktop/src/relay/` (Part B) không có `quality.*` và `RelayDispatcher` làm rơi `error.data`. Cần CR riêng: thư mục tạm/nhật ký trên host SSH, độ trễ 50-200 ms của thông báo, `taskkill`/pgid qua SSH. Ngoài phạm vi D2 (MVP chỉ direct-websocket) |

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Lệnh định nghĩa phía agent (catalog + tệp host), không trong repo, không từ backend | 2.2; giữ D5/O11 |
| Không dùng `runToolCommand`; đường spawn riêng | Cần nhóm tiến trình, tệp đầu ra, diệt cây, giới hạn; sửa `runToolCommand` chung với `tools/call` rủi ro hơn (CR-CV-001 6) |
| Không gọi `pnpm test`/`pnpm lint` | Script gốc lệch cây (1.2) và `ensure-native-runtime` có thể `pnpm rebuild` (ghi `node_modules`); gọi thẳng công cụ trong `cwd` đúng |
| Bước chạy tuần tự, một run/worktree, cổng việc nặng chung | Dev server cạnh PTY agent AI; tránh bùng CPU/RAM |
| `percent` = bước hoàn tất/tổng, hoặc `null` | Không bịa tiến độ trong bước dài |
| Có phát hiện không làm run `failed` | Bộ chạy thu thập, cổng quyết định (CR-CV-085) |
| Env từ rỗng + allowlist, thay vì lọc bớt `toolEnv` | `toolEnv` chứa mọi `process.env` và khoá AI (1.3); danh sách cho phép an toàn hơn danh sách chặn |
| Preflight chỉ đọc, báo `ENV_NOT_READY` | Nhất quán E3: không trả kết quả sai khi thiếu `node_modules`/module Go |
| `quality.listProfiles` không lỗi khi thiếu môi trường | Mã lỗi `error.data.code` của agent bị mất qua `RelayByDevServer` |
| Windows chưa hỗ trợ | Cùng CR-CV-001; `taskkill` đã tính nhưng chưa kiểm chứng |

## 4. Tiêu chí chấp nhận

- [ ] `quality.listProfiles` trả đúng catalog Orca (mục 2.3) với `ready`/`missing[]`; không có trường chứa argv đầy đủ, biến môi trường, hay đường dẫn ngoài `workspaceRoot`; `definitionHash` đổi khi đổi `argv` của profile.
- [ ] Không có tham số `command|argv|args|env|cwd|timeout` nào được chấp nhận ở bất kỳ `quality.*` (test phản chiếu schema `validate`); tham số lạ trả `CODEINTEL_INVALID_PARAMS`.
- [ ] `quality.run` với tên lạ trả `CODEINTEL_PROFILE_UNKNOWN` và **không** spawn tiến trình nào; `workspaceRoot` không phải gốc worktree trả `CODEINTEL_PATH_NOT_ALLOWED`.
- [ ] Tên tệp có `-`, NUL, symlink thoát worktree, `base` có ký tự lạ bị từ chối; `cwd` của profile là symlink trỏ ra ngoài worktree bị từ chối.
- [ ] Run thứ hai cùng worktree trả `CODEINTEL_RUN_IN_PROGRESS` với đúng `runId`; run khác worktree khi đã đủ cổng vào hàng đợi, vượt 4 trả `queue_full`.
- [ ] Preflight thiếu `golangci-lint` (xoá khỏi PATH giả) hoặc `built with` cũ hơn `go.work` cho `ready:false`, `missing[].reason` đúng; `quality.run` chỉ có bước đó trả `CODEINTEL_ENV_NOT_READY`; run nhiều bước chạy các bước còn lại và bước thiếu có `env_not_ready`.
- [ ] Preflight và run không bao giờ gọi `pnpm install`, `pnpm rebuild`, `go mod download`, `ensure-native-runtime --runtime=…` (test quét argv của mọi `PlannedStep` và của preflight).
- [ ] Tiến trình con không nhận biến nào khớp mẫu secret; test với `process.env` chứa `ANTHROPIC_API_KEY`, `GITHUB_TOKEN`, `AGENT_TOKEN`, `FOO_SECRET` cho thấy chúng không có trong `env` của spawn.
- [ ] Đầu ra của bước vượt `maxOutputBytes` bị diệt và `failed (outputTooLarge)`, tệp tạm bị xoá khi hết TTL; thư mục tạm quyền `0700`.
- [ ] Huỷ giữa chừng một bước sinh con cháu (script Node giả `spawn` thêm 2 tiến trình): sau ≤ 15 s cả nhóm biến mất (`kill(-pid,0)` ném `ESRCH`), run `cancelled`, `quality.cancel` lặp lại không lỗi.
- [ ] Timeout bước diệt cây (không chỉ `SIGTERM` tiến trình đầu) và trả `timeout` kèm phần đã parse.
- [ ] Mất ws giữa chừng: run tiếp tục; sau nối lại `quality.runStatus` đúng; thông báo `quality.finished` không mất nếu có notifier hiện hành; khởi động lại agent đánh dấu `interrupted`.
- [ ] `quality.progress` có `percent` số hoặc `null`, ≤ 1/giây/run, `message` đã che đường dẫn tuyệt đối và secret.
- [ ] `scope=changed` với 4 tệp TS đổi chạy `ts-lint` đúng 4 tệp; với > 300 tệp chạy đủ và `scopeWidened=true`; thay đổi ở `backend-go/common` mở rộng tới mọi module Go và `scopeWidened=true`.
- [ ] `workTreeChangedDuringRun=true` khi sửa một tệp trong lúc run đang chạy.
- [ ] Bước `heavy` và `codeintel.reindex` không chạy đồng thời khi `ORCA_HEAVY_JOBS=1`.
- [ ] `ORCA_QUALITY_RUN=off` và `win32` trả `CODEINTEL_TOOL_UNAVAILABLE`.
- [ ] Không có tên tệp `helpers/utils/common/misc`; không `max-lines` disable mới; `pnpm --filter orca-agent test` (hoặc `vitest run` trong `agent/`) xanh.

## 5. Kiểm thử

Chưa chạy bất kỳ test nào ở thời điểm viết CR. Vitest trong `agent/` (`vitest.config.ts` include `src/**/*.test.ts`); CI hiện không chạy test của `agent/` (README v7 mục 8 điểm 16), nên CR-CV-070 2.7 đã thêm bước khẳng định job có chạy.

| File test (mới) | Nội dung |
|---|---|
| `agent-rpc-dispatch-quality.test.ts` | Định tuyến, tham số lạ, hình dạng JSON-RPC, `MockWs` như `agent-rpc-dispatch-misc.test.ts` |
| `quality-profile-schema.test.ts`, `quality-profile-catalog.test.ts` | Mọi profile trong catalog hợp lệ; `cwd` và tệp `requires` tĩnh tồn tại trong repo mẫu; `argv` chỉ chứa mẫu cho phép; không có `analyze|clean|install|rebuild|download` |
| `quality-run-planning.test.ts` | Mở suite; `scopeStrategy` từng loại; chọn module Go từ `go.work` giả; mở rộng khi `common`/`proto` đổi; chặn symlink thoát worktree |
| `quality-changed-files.test.ts` | Repo git thật trong thư mục tạm (đa nền tảng): commit, sửa chưa commit, tệp mới, đổi tên; `git worktree add` cho worktree liên kết; hai nhánh git baseline (theo mẫu `git-handler-worktree-git-capabilities.test.ts`) |
| `quality-environment-preflight.test.ts` | Binary thiếu/cũ, `node_modules` thiếu, `go.work` yêu cầu cao hơn, `golangci-lint` giả in `built with go1.22.2`, `tmp_space_low` (giả `statfs`), cache 60 s |
| `quality-child-env.test.ts` | Allowlist, loại mẫu secret, `allowExtra` bị từ chối khi trùng mẫu |
| `quality-output-redaction.test.ts` | Mẫu token, giá trị env, đường dẫn tuyệt đối, `user:pass@` |
| `quality-run-step-executor.test.ts` | Script Node giả làm công cụ qua PATH tạm: đầu ra lớn, timeout, exit lạ, sinh con cháu, bỏ qua `SIGTERM`; kiểm không rò tiến trình |
| `quality-process-tree-kill.test.ts` | POSIX `kill(-pid)`; Windows mock `taskkill` |
| `quality-run-manager.test.ts` | Khoá worktree, hàng đợi, cổng nặng chung với reindex, `interrupted` sau khởi động lại, mất ws, `workTreeChangedDuringRun` |
| `quality-results-store.test.ts` | Phân trang, TTL, `view=log` có che |
| `agent-heavy-job-gate.test.ts` | Giới hạn, thời gian chờ, giải phóng khi lỗi |
| `agent-session-capabilities-quality.test.ts` | `quality` có/không |

Thủ công trước khi bật (ghi số đo vào PR): chạy từng profile mục 2.3 trên bản sao Orca và ghi thời gian, RAM, exit code, kích thước đầu ra (số liệu hiện **chưa có**); thử `golangci-lint` v1.62.2 với `go.work` 1.26.0; thử `ts-typecheck-agent` (composite); thử `proto-breaking` trong worktree liên kết; thử `bwrap` (L1).

## 6. Rủi ro và điểm chưa kiểm chứng

- **Chưa chạy bất kỳ profile nào.** Tên tệp cấu hình đã kiểm tồn tại; chưa biết lệnh nào xanh trên cây hiện tại (đặc biệt `ts-typecheck-*` với `tsc 7.0.2`, `ts-typecheck-agent` composite, `go-lint` với golangci-lint v1.62.2 xây bằng go1.22.2, `proto-breaking` trong worktree liên kết).
- **Thời gian/RAM chưa đo**: `runTimeoutMs` 45 phút, `maxOutputBytes`, `ORCA_HEAVY_JOBS=1`, `nice 10`, `GOMAXPROCS = cores/4` đều là giả định. `go test ./...` trên 19 module và 3 `tsc` lớn có thể vượt hàng chục phút.
- **Script gốc lệch cây** (1.2): nếu người làm repo sửa lại để khớp (đưa `desktop/config` lên gốc, hoặc ngược lại), catalog phải đổi `cwd`/đường dẫn; `definitionHash` đổi báo hiệu cho chính sách ở backend. Việc xác nhận xem `pnpm lint/typecheck/test` ở gốc có chạy được là **chưa làm** (cấm).
- **Chạy mã không tin cậy**: L0 không chặn đọc credential/mạng (2.10). Quyết định chính sách (ai kích hoạt, nhánh không tin cậy, có cần L1/L2) thuộc người duyệt; CR này chỉ cung cấp cơ chế.
- **Tiến trình thoát khỏi nhóm** (double-fork) không bị `kill(-pid)` diệt; test chạy Docker/Playwright có thể sinh con ngoài nhóm: đã loại khỏi catalog.
- **`vitest --reporter=json --outputFile`** và `related`: cờ có trong `vitest --help` (đã đọc), hình dạng JSON chưa chạy (CR-CV-082).
- **`GOPROXY=off`/`GOFLAGS=-mod=readonly`** có thể làm test cần mạng/module chưa tải thất bại thành `env_not_ready` giả; chưa đo tỉ lệ.
- **Cache module Go và `node_modules` của worktree liên kết**: worktree mới tạo chưa `pnpm install` thì thiếu `node_modules` ở `<worktree>/node_modules` (monorepo pnpm cài ở gốc từng worktree); bộ phân giải thử cả `<repoRoot>` của worktree **chính** là rủi ro (chạy mã `node_modules` của checkout khác). Mặc định chỉ dùng `node_modules` trong chính worktree; fallback sang checkout chính là Q5.
- **Che secret chỉ ở mức tối thiểu**; log thô có thể chứa dữ liệu nhạy cảm do mã test in ra.
- **Hai agent trên cùng máy** không chia sẻ cổng nặng (2.9).
- **`error.data.code` bị mất qua `RelayByDevServer`** cho tới khi CR-CV-023 sửa; thiết kế đã giảm phụ thuộc vào mã lỗi (`listProfiles`, `runStatus`).
- **SSH**: `relay-ssh` Part B không có `quality.*` (2.11); độ trễ 50-200 ms của thông báo chưa ảnh hưởng thiết kế nhưng làm `quality.progress` thưa.
- **Git**: chỉ dùng lệnh dưới baseline 2.25; `git diff <base>...HEAD` cần `base` có trong object store; shallow clone chưa kiểm chứng.

## 7. Câu hỏi mở

- **Q1.** Có chấp nhận catalog tích hợp cho Orca cộng tệp ghi đè phía host (2.2), hay cần bảng `quality_profiles` giữ lệnh? Đề xuất: không giữ lệnh ở backend.
- **Q2.** Có thêm tham số `trust` vào `quality.run` (cho phép backend hạ cấp profile khi nhánh không tin cậy), và nguồn dữ liệu "không tin cậy" lấy từ đâu (chưa có trong series)?
- **Q3.** Có đọc `.orca/quality.json` trong repo chỉ để chọn tập con profile và hạ timeout không (P1)?
- **Q4.** Khoá tệp liên tiến trình cho cổng nặng (nhiều agent cùng máy)?
- **Q5.** Worktree liên kết thiếu `node_modules`: chỉ báo `ENV_NOT_READY` (đề xuất) hay cho dùng `node_modules` của checkout chính (tiện nhưng thực thi mã ngoài worktree)?
- **Q6.** Sửa script gốc (`package.json`) để khớp cây hiện tại có thuộc phạm vi series này không? Hiện chỉ ghi nhận.
- **Q7.** Ngưỡng mặc định `ORCA_HEAVY_JOBS`, `nice`, `maxWorkers` trên dev server nhỏ (RAM thấp)?
- **Q8.** `quality.results view=log` có cần quyền riêng (log có thể chứa dữ liệu nhạy cảm) ở backend (CR-CV-013)?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (O9-O14; mục 3.10; mục 8 điểm 1, 4, 16)
- `/opt/repos/orca/docs/research/view-code/11-additions-for-quality-control.md` §3.3 (B1), §6 (E2, E3), §7
- `/opt/repos/orca/docs/crs/v7/agent-codeintel/CR-CV-001-codeintel-agent-foundation.md` (2.1, 2.4, 2.5, 2.7, 2.8), `CR-CV-004-codeintel-reindex-and-index-notifications.md` (2.2-2.6), `CR-CV-006-codeintel-relay-ssh-part-b.md`
- `/opt/repos/orca/docs/crs/v7/quality-signals/CR-CV-080-agent-worktree-index-strategy-and-auto-refresh.md`, `CR-CV-082-quality-finding-model-and-parsers.md`
- `/opt/repos/orca/agent/src/relay/agent-config.ts` (`buildToolPath`, `loadAgentConfig` `toolEnv`), `agent-tool-registry.ts` (`runToolCommand` `:72`), `agent-exec-handler.ts` (`killProcessTree`), `agent-rpc-dispatch.ts` (`route`, `makeNotifier` `:280`, `makeError` `:408`), `agent-rpc-dispatch-misc.ts` (mẫu), `pty-daemon-client.ts` (`detached` `:84`), `agent-session.ts`, `agent-session-capabilities.ts`
- `/opt/repos/orca/package.json` (scripts), `/opt/repos/orca/desktop/package.json`, `/opt/repos/orca/agent/package.json`, `/opt/repos/orca/frontend/package.json`, `/opt/repos/orca/backend/package.json`, `/opt/repos/orca/pnpm-workspace.yaml`
- `/opt/repos/orca/config/scripts/` (3 tệp), `/opt/repos/orca/desktop/config/scripts/` (`ensure-native-runtime.mjs`, `check-styled-scrollbars.mjs`, `check-reliability-gates.mjs`, `check-max-lines-ratchet.mjs`), `/opt/repos/orca/desktop/config/{vitest.config.ts,tsconfig.node.json,tsconfig.tc.web.json,tsconfig.tc.cli.json,reliability-gates.jsonc}`, `/opt/repos/orca/.oxlintrc.json`
- `/opt/repos/orca/backend-go/Makefile`, `/opt/repos/orca/backend-go/.golangci.yml`, `/opt/repos/orca/backend-go/go.work`, `/opt/repos/orca/backend-go/proto/buf.yaml`, `/opt/repos/orca/backend-go/policy/orca-authz/`
- `/opt/repos/orca/.github/workflows/pr.yml` (`:64`), `/opt/repos/orca/.github/workflows/backend-go-mcp-service.yml` (`:26-36`), `/opt/repos/orca/.github/workflows/backend-go-project-service.yml`
- Lệnh chỉ-đọc đã chạy 2026-10-06: `oxlint --help`, `vitest --help`, `tsc --version`, `golangci-lint --version`/`run --help`, `buf --version`/`lint --help`/`breaking --help`, `opa version`/`test --help`, `go version`/`go env`, `which systemd-run prlimit bwrap`, `ls`/`[ -e ]` các tệp cấu hình
- `/opt/repos/orca/guides/reference/git-compatibility.md`, `/opt/repos/orca/AGENTS.md`
