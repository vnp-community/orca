# CR-CV-083 — Thu thập coverage và diff coverage (Go trước, TypeScript sau khi được duyệt phụ thuộc)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-083 |
| **Tên** | Thu thập độ phủ test qua profile của bộ chạy kiểm tra (CR-CV-081), parse profile Go (và vitest nếu duyệt), tính độ phủ theo tệp/hàm và **diff coverage** trên dòng đã đổi, lưu `coverage_reports` theo `(worktree, commit)`, fallback "ước lượng" từ cạnh test của GitNexus |
| **Loại** | Feature (agent + `code-intel-service`) |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-081 (profile, `quality.run`, `quality.coverage`, giới hạn tài nguyên, `CODEINTEL_ENV_NOT_READY`), CR-CV-082 (`QualityRun`, `fingerprint`, bảng `quality_runs`), CR-CV-036 (`ChangeOverlay`, `uncoveredSymbols`, mốc `mergeBase`), CR-CV-030 (đọc diff), CR-CV-011 (bảng, repository) |
| **Mở khoá** | CR-CV-085 (kiểm tra `coverage`/`diffCoverage` trong `QualityGate`), CR-CV-087 (đồng hồ diff coverage, treemap phủ test), CR-CV-089 (đối chiếu "agent báo test pass") |
| **Tác động** | `agent/src/relay` (file mới `quality-coverage-go-profile.ts`, `quality-coverage-vitest-report.ts`, `quality-diff-changed-lines.ts`; thêm profile vào cấu hình của CR-CV-081), `backend-go/services/code-intel-service` (bảng `coverage_reports`, RPC `GetCoverage`), `backend-go/proto/orca/codeintel/v1` (message `CoverageReport`, `FileCoverage`, `DiffCoverage`). **Phụ thuộc mới (chỉ phương án TS):** `@vitest/coverage-v8` — cần duyệt theo O12 |

---

## 1. Bối cảnh và vấn đề

Đã đọc code/cấu hình ngày 2026-10-06:

1. **Repo không có cấu hình coverage nào.** `grep coverage` trong `package.json` (gốc), `frontend/`, `desktop/`, `agent/`, `packages/*` chỉ thấy script `verify:localization-coverage`/`audit:localization` (không liên quan). `node_modules/@vitest/` chỉ có `expect, mocker, pretty-format, runner, snapshot, spy, utils`, **không có `coverage-v8`/`coverage-istanbul`**. `vitest` ở `4.1.5` (`node_modules/vitest/package.json`). Không có `-coverprofile` trong `.github/workflows/*` hay `backend-go/Makefile`.
2. **Test TS nằm ở nhiều gói, mỗi gói một cấu hình riêng:** `frontend/config/vitest.config.ts`, `desktop/config/vitest.config.ts`, `agent/vitest.config.ts`, cùng `backend/`, `mobile/`, `emulator/`, `tests/{server,client,e2e}/`. `frontend` và `desktop` đặt `root: process.cwd()`, alias `@`/`@renderer` → `src/renderer/src`, `testTimeout 30 s`, `hookTimeout 60 s`, `server.deps.inline` cho `renderer/src`. `agent/vitest.config.ts` chỉ có `environment: node` và `include: src/**/*.test.ts`. Không có khối `coverage:` ở file nào. Script `test` của `desktop` chạy `ensure-native-runtime.mjs` trước `vitest run` (module native), `pnpm test` ở gốc trỏ `config/vitest.config.ts` **không tồn tại ở gốc** (xem 6, lệch cấu trúc repo).
3. **Go là workspace nhiều module** (`backend-go/go.work`: `common`, `proto`, `cmd/orca-cli` và 18 service; `go 1.26.0` trong `go.work`, trong khi workflow `backend-go-task-service.yml` dùng `go-version: "1.25"`). `backend-go/Makefile` `test` lặp `for m in common proto services/*` chạy `go test ./...` trong **từng module** (không phải một lệnh gốc); `cmd/orca-cli` không nằm trong `SERVICES` nên `make test` bỏ qua. `test-integration` thêm `-tags=integration` (testcontainers, cần Docker). Vì vậy coverage Go phải thu **theo module** rồi ghép.
4. **Chỉ có tín hiệu "ước lượng":** CR-CV-036 `uncoveredSymbols` = symbol thực thi đã đổi không có cạnh `CALLS` từ tệp test (độ sâu ≤ 2); chính CR đó ghi "Không có số coverage thật (E11 chưa xác nhận)" và dễ báo nhầm với gọi gián tiếp/`vi.mock`.
5. **Cổng chất lượng (CR-CV-085) cần một con số có nguồn** (`coverage`, `diffCoverage`) và phải trả `unknown` khi thiếu dữ liệu (README 3.10), không suy diễn thành `pass`.

Vấn đề: không có đường nào hỏi "worktree này phủ bao nhiêu, phần agent vừa sửa được test chưa". CR này thêm đường đo thật (Go trước), tính diff coverage, và giữ fallback ước lượng được gắn nhãn khác hẳn.

## 2. Giải pháp đề xuất

### 2.1 Phạm vi theo giai đoạn

| Giai đoạn | Nội dung | Điều kiện |
|---|---|---|
| **A (mặc định của CR)** | Coverage **Go** của các module trong `go.work` | Không phụ thuộc mới. `go test -coverprofile` có sẵn trong toolchain |
| **B** | Coverage **TS** qua `@vitest/coverage-v8` cho `agent/`, `frontend/`, `desktop/` | **Phải duyệt phụ thuộc mới (O12)**; chưa duyệt thì chỉ có fallback ước lượng (2.7) cho TS |
| **C** | Fallback ước lượng từ đồ thị | Luôn có; nhãn `estimated` |

### 2.2 Profile chạy (mới; tên và cơ chế thuộc CR-CV-081)

Hai profile có tên (không nhận lệnh tự do, README 3.10): `coverage-go` và `coverage-ts`. Khai báo (dạng cấu hình của CR-CV-081, minh hoạ):

```yaml
coverage-go:
  kind: coverage
  language: go
  modules: from-go-work        # đọc go.work, không nhận danh sách từ client
  command: ["go", "test", "-covermode=set", "-coverprofile=<tmp>/<module>.out", "./..."]
  cwdPerModule: true
  timeoutSec: 900
  requires: ["go"]
  env: sanitized               # không chuyển biến môi trường chứa secret (O11)
coverage-ts:                    # chỉ có hiệu lực sau khi duyệt @vitest/coverage-v8
  kind: coverage
  language: ts
  packages: ["agent", "frontend", "desktop"]
  command: ["pnpm", "exec", "vitest", "run", "--config", "<pkgConfig>",
            "--coverage.enabled", "--coverage.provider=v8",
            "--coverage.reporter=json", "--coverage.reportsDirectory=<tmp>/<pkg>"]
  timeoutSec: 1500
  requires: ["pnpm", "@vitest/coverage-v8"]
```

- `-covermode=set` (không `atomic`): rẻ hơn, đủ cho "dòng này đã chạy chưa"; không dùng `-coverpkg=./...` mặc định (tăng thời gian, làm số lệch); không `-tags=integration` (cần Docker, chậm; nêu nhãn "chỉ test đơn vị" trên báo cáo). Cả hai là lựa chọn mặc định, cho phép đổi bằng profile khác (Q2).
- Module đọc từ `go.work` `use (...)` (21 mục tại thời điểm viết). Module chưa có `*_test.go` cho ra profile rỗng; coi là `0 %` có đánh dấu `noTests:true`, không bỏ khỏi mẫu số cấp module nhưng loại khỏi trung bình có trọng số khi bật `excludeNoTests` (Q4). `code-intel-service` (CR-CV-010) chưa có trong `go.work` — tự vào khi CR-CV-010 thêm.
- `go test` ghi tệp vào `<tmp>` do runner tạo (CR-CV-081 chịu trách nhiệm thư mục tạm `0700` và dọn); không ghi vào worktree (không tạo `coverage/` trong cây làm việc). Với TS, `--coverage.reportsDirectory` trỏ ra ngoài worktree vì mặc định của vitest là `./coverage` (sẽ làm bẩn `git status` của người dùng).
- Lệnh `go test` thực thi mã test của worktree (có thể do agent viết): cùng mức rủi ro với CR-CV-081 (research 11 §7); không thêm biện pháp riêng ở đây.
- Chưa kiểm chứng: thời gian chạy `go test -coverprofile ./...` toàn bộ 21 module trên dev server, và bộ test TS có chạy được dưới `--coverage` (v8 tăng thời gian/bộ nhớ). Spike S1 ở mục 6.

### 2.3 Parse profile Go (agent, mới)

File `agent/src/relay/quality-coverage-go-profile.ts` (tên theo khái niệm; không `utils`). Định dạng profile (spec của `cmd/cover`, cần đối chiếu bản Go trên dev server): dòng đầu `mode: set|count|atomic`; mỗi dòng sau `<importPath>/<file>.go:<startLine>.<startCol>,<endLine>.<endCol> <numStmt> <count>`.

- **Ánh xạ tệp → module → đường dẫn tương đối repo:** phần trước `:` là `import path` (ví dụ `github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase/list_issues.go`). Agent đọc `module` từ `go.mod` của từng module trong `go.work` (đọc tệp, không chạy lệnh) và cắt tiền tố module path, ghép với thư mục module → đường dẫn tương đối gốc repo (`backend-go/services/scm-integration-service/internal/usecase/list_issues.go`). Dòng không khớp module nào (ví dụ file sinh/thư viện ngoài) bị bỏ và đếm vào `unmappedBlocks`. Không bao giờ đưa đường dẫn tuyệt đối vào kết quả (README 3.4).
- **Loại trừ:** tệp sinh (`*.pb.go`, `*_grpc.pb.go`, thư mục `gen/` — `.golangci.yml` đã loại `gen` khỏi lint), `*_test.go`, `testutil`, `usecasetest/` khỏi mẫu số (cấu hình `exclude` trong profile; danh sách mặc định lấy từ CR-CV-036 để hai nơi không lệch).
- **Tổng hợp:** theo tệp `{stmts, coveredStmts, pct}`; theo hàm bằng `go tool cover -func=<profile>` (chỉ đọc; đầu ra `file:line:\tfunc\tpct%`; ghi nhớ định dạng phụ thuộc phiên bản Go, ghi `toolVersion`); theo module và tổng.
- Khối trùng lặp giữa nhiều gói (khi `-coverpkg`) hoặc nhiều profile: hợp nhất bằng khoá `(file, start, end)`, `count = tổng` (với `set`: OR).

### 2.4 Parse báo cáo vitest (giai đoạn B, mới)

`agent/src/relay/quality-coverage-vitest-report.ts` đọc `coverage-final.json` (định dạng istanbul: theo tệp `statementMap`, `s` đếm lần chạy, `fnMap`, `f`). Lý do chọn `json` thay `lcov`: có sẵn ranh giới câu lệnh/hàm để dùng cùng thuật toán diff 2.5 và không cần parser văn bản. Ánh xạ tệp: đường dẫn tuyệt đối trong báo cáo → tương đối gốc repo bằng cách cắt `workspaceRoot` (kiểm tra nằm trong `workspaceRoot`; ngoài thì bỏ). Chạy theo **gói** (`agent`, `frontend`, `desktop`), mỗi gói một báo cáo, ghép như Go.

**Phụ thuộc mới (O12, cần duyệt riêng):** `@vitest/coverage-v8` phải cùng phiên bản với `vitest` (`4.1.5` hiện cài; cần kiểm tra tương thích ở bước spike), thêm vào `devDependencies` của từng gói cần đo (hoặc gốc). Hệ quả: đổi `pnpm-lock.yaml`; CI `pr.yml` có bước `git diff --exit-code package.json pnpm-lock.yaml` sau install nên PR thêm phụ thuộc phải kèm lockfile. Phương án không thêm phụ thuộc: **không có** (vitest không tự đo coverage), nên TS chỉ có ước lượng nếu không duyệt. Ghi nhận ở Q1.

### 2.5 Diff coverage (mới)

`agent/src/relay/quality-diff-changed-lines.ts`: lấy **dòng đã thêm/sửa** của từng tệp nguồn theo `base` của lần chạy (mặc định merge-base với nhánh gốc, O7, giống CR-CV-036).

- Tính **ở agent** (gần worktree, tránh kéo cả hai phía nội dung tệp qua SSH): chạy `git diff --unified=0 --no-color --no-ext-diff <mergeBase>` (cờ có từ Git rất cũ; baseline 2.25 đáp ứng, không cần `GitCapabilityCache`; `-c` không dùng), đọc tiêu đề hunk `@@ -a,b +c,d @@` và thu tập số dòng mới `[c, c+d)` (`d=0` = chỉ xoá, bỏ qua). Tệp mới (untracked) chưa có trong `git diff <mergeBase>` → dùng danh sách từ `git status --porcelain` và coi toàn bộ dòng là đã thêm. Đây là lệnh git nội bộ của runner, không đi qua `git.exec`/ký tự bị chặn.
- Tên tệp không ASCII: bắt buộc dùng `-z` hoặc `core.quotePath` (xem CR-CV-030 mục 1 điểm 4); runner của agent được truyền cờ trực tiếp nên dùng được `-c core.quotePath=false` (**lưu ý AGENTS.md**: lệnh bắt đầu bằng cờ toàn cục `-c` phải giữ nguyên).
- **Quy tắc xếp loại dòng (Go, theo khối):** với mỗi dòng đã đổi `L` của tệp `F`: tìm các khối của `F` có `start ≤ L ≤ end`. Không có khối → **không thực thi được** (chú thích, import, khai báo) → loại khỏi cả tử và mẫu. Có khối: **phủ** nếu mọi khối chứa `L` có `count>0`, **chưa phủ** nếu có ít nhất một khối `count==0` (chọn bảo thủ; xem Q3). Với istanbul: dùng `statementMap` theo cùng cách.
- `diffCoverage = covered / (covered + uncovered)`; nếu mẫu số = 0 thì **`null`** (kèm `reason:"no_executable_changed_lines"`), không báo 100 %.
- Kết quả theo tệp: `changedExecutable`, `covered`, `uncoveredRanges: [[from,to],...]` (gộp dòng liền kề), để CR-CV-087 vẽ chú thích trên diff (research 11 D6) và CR-CV-036 đối chiếu `uncoveredSymbols`.
- Tệp thuộc loại bị loại (test, sinh, doc, JSON/SQL/proto/YAML không có công cụ đo) ghi vào `excludedFiles` với lý do, không im lặng.
- Gắn được với `ChangedFile.isTest/isGenerated/isDoc` của CR-CV-036 để dùng cùng quy tắc loại.

### 2.6 Lưu trữ: `coverage_reports` (mới; hai dialect, `tenant_id`, không FK chéo service)

README 3.10 đã nêu tên bảng; CR này đề xuất cột (CR-CV-011 giữ migration; cần CR-CV-082 xác nhận khoá liên kết `quality_runs.id`):

| Cột | Ghi chú |
|---|---|
| `id`, `tenant_id` | |
| `quality_run_id` | tham chiếu `quality_runs.id` (ứng dụng kiểm tra) |
| `repo_binding_id`, `worktree_id` | `worktree_id` có ba dạng chuỗi (README mục 8 điều chỉnh 10) nên khoá chính theo `repo_binding_id` |
| `head_commit`, `index_commit?` | khoá "kết luận dựa trên commit nào" (research 11 A3) |
| `base_commit` | mergeBase dùng cho diff |
| `dirty` (bool), `tree_hash?` | worktree có thay đổi chưa commit lúc đo (xem Q5) |
| `language` (`go`/`ts`), `scope_key` (module hoặc gói) | |
| `source` | `"measured"` hoặc `"estimated"` (2.7) |
| `mode` | `set`/`count`/`atomic`/`v8` |
| `total_stmts`, `covered_stmts` | tổng theo `scope_key` |
| `diff_executable`, `diff_covered` | nullable |
| `payload` (JSON), `payload_bytes`, `truncated` | chi tiết theo tệp/hàm, có giới hạn |
| `tool_versions` (JSON) | `go`, `vitest`, `@vitest/coverage-v8` |
| `created_at`, `expires_at` | |

Duy nhất `(tenant_id, repo_binding_id, head_commit, dirty, tree_hash, scope_key, source)`; chỉ mục `(tenant_id, repo_binding_id, created_at)` cho xu hướng (`quality_trend_points`, CR-CV-085/087). Postgres bật RLS theo `tenant_id`, MySQL kiểm tra ở ứng dụng (README 3.5). Upsert idempotent (consumer sự kiện at-least-once).

**Kích thước:** `payload` gồm theo tệp `{path, stmts, covered, pct, functions[{name,line,pct}]}` + `uncoveredRanges` **chỉ cho tệp đã đổi** và tệp có `pct` dưới ngưỡng hiển thị; không lưu từng dòng cho cả repo. Trần mặc định: ≤ 2 000 tệp, ≤ 1 MiB `payload`/báo cáo (gRPC nhận gói mặc định 4 MiB, README mục 8 điều chỉnh 16); vượt thì cắt theo `pct` thấp trước và đặt `truncated:true` cùng `totalCount`. Giữ 30 ngày hoặc N=20 báo cáo/worktree (cấu hình tenant; chưa đo dung lượng thực).

Truy xuất: RPC `GetCoverage` (đã có tên ở README 3.10) với `quality_run_id` hoặc `(worktree, commit)`, trả `CoverageReport{ source, headCommit, baseCommit, dirty, totals, diff: DiffCoverage, files[], truncated, totalCount, toolVersions, estimatedNote? }`. Kênh `codeIntel.quality.coverage`.

### 2.7 Fallback "ước lượng" từ GitNexus

Khi không có báo cáo đo thật (chưa duyệt TS, công cụ thiếu, chạy lỗi, hết hạn):

- Dùng `ChangeOverlay.uncoveredSymbols` và `ChangedSymbol.tested` của CR-CV-036 (cạnh `CALLS` từ tệp test, `tested:"unknown"` khi `impact` bị cắt) để sinh `CoverageReport` với `source:"estimated"`, `totals` **không có phần trăm câu lệnh**, chỉ có `changedSymbolsTested/Untested/Unknown`.
- Nhãn bắt buộc "ước lượng từ đồ thị gọi, không phải độ phủ thật" ở mọi bề mặt (gate, UI, báo cáo). `estimated` **không bao giờ** được trộn với `measured` trong một con số; với CR-CV-085 coi là tín hiệu `warn` tối đa, không thể làm `diffCoverage` đạt ngưỡng.
- Ngưỡng và `verdict` thuộc CR-CV-085; ở đây chỉ cung cấp dữ liệu và nhãn.

### 2.8 Đường chạy và sự kiện

1. Người dùng (có quyền ghi project, O11) gọi `StartQualityRun{profile:"coverage-go", scope:"changed"|"worktree"}` (CR-CV-081/082).
2. Runner chạy `go test` từng module, đẩy `quality.progress` (có `percent:null` khi không biết); khi xong, agent parse + tính diff, lưu kết quả trong bộ nhớ run, trả qua `quality.coverage {runId}`.
3. `code-intel-service` lấy kết quả (kéo theo `quality.finished`), kiểm tra `headCommit` còn là HEAD/`index` hiện tại (nếu worktree đã đổi trong lúc chạy thì đánh dấu `stale`), ghi `coverage_reports`, phát `orca.codeintel.quality.run_finished`.
4. `scope:"changed"` ở Go: chỉ chạy `go test` cho **module** có tệp đổi (suy từ diff); không thu hẹp tới từng gói vì phụ thuộc chéo gói làm số sai. Ghi `partial:true` + danh sách module đã/chưa đo; mẫu số tổng chỉ tính module đã đo.

### 2.9 Tính nhất quán đường dẫn, SSH, Windows

- Mọi tính toán chạy trên dev server (agent); chỉ kết quả nhỏ đi qua SSH/WS (đáp ứng "mọi thao tác có thể qua SSH", README mục 6).
- Dev server Windows không hỗ trợ ở series này (CR-CV-001 chặn `unsupported_platform`); parse vẫn chuẩn hoá `\` → `/` để không phụ thuộc nền.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| K1 | Go trước, TS sau duyệt | Đo được không cần phụ thuộc mới; O12 |
| K2 | Mọi phép tính ở agent, backend lưu và phục vụ | Tránh kéo nội dung hai phía qua mạng; agent đã chạy `go`/`git` sẵn |
| K3 | Diff coverage trên **dòng thực thi đã đổi**, mẫu số 0 → `null` | Tránh "100 %" giả cho thay đổi chỉ gồm chú thích/khai báo |
| K4 | Một dòng chưa phủ nếu **có khối chưa chạy** | Bảo thủ; báo nhầm thiên về "thiếu test" thay vì bỏ sót |
| K5 | `estimated` và `measured` tách bạch, không cộng gộp | Research 11 B3 "ghi rõ ước lượng"; tránh số trôi nổi |
| K6 | Không ghi vào worktree; báo cáo ra thư mục tạm | Không làm bẩn `git status`, không phụ thuộc quyền ghi |
| K7 | Không lưu từng dòng cho toàn repo | Giới hạn gRPC 4 MiB, giá lưu trữ |
| K8 | `-covermode=set`, bỏ `-tags=integration` mặc định | Chi phí và Docker; có thể thêm profile riêng |

## 4. Tiêu chí chấp nhận

- [ ] Profile `coverage-go` chỉ tồn tại dưới dạng tên; `quality.run` với lệnh/args tự do bị từ chối `CODEINTEL_PROFILE_UNKNOWN`.
- [ ] Chạy trên fixture workspace Go nhỏ (2 module, một module không có test): ra `CoverageReport` với `totals`, theo tệp, theo hàm; module không test có `noTests:true`.
- [ ] Ánh xạ import path → đường dẫn tương đối đúng cho module trong `go.work`; không có đường dẫn tuyệt đối trong kết quả; khối ngoài module đếm vào `unmappedBlocks`.
- [ ] Tệp `*.pb.go`, `gen/`, `*_test.go` không vào mẫu số.
- [ ] Diff coverage: với diff cho trước (thêm hàm được test, thêm hàm không test, sửa chú thích) cho `covered/uncovered/excluded` đúng; thay đổi chỉ chú thích → `diffCoverage:null`.
- [ ] Tệp untracked mới được tính như toàn bộ dòng thêm.
- [ ] Tên tệp có ký tự không ASCII không làm sai ánh xạ dòng (kiểm thử).
- [ ] Báo cáo ghi `headCommit`, `baseCommit`, `dirty`, `toolVersions`; chạy lại cùng commit upsert đè, không nhân đôi bản ghi.
- [ ] Payload vượt trần bị cắt theo `pct` thấp, `truncated:true`, `totalCount` đúng; gRPC không vượt 4 MiB.
- [ ] Khi thiếu dữ liệu đo: `GetCoverage` trả `source:"estimated"` kèm nhãn; không bao giờ có trường phần trăm câu lệnh trong báo cáo ước lượng.
- [ ] `scope:"changed"` chỉ chạy module có tệp đổi và đánh dấu `partial:true`.
- [ ] Worktree không có tệp mới nào (đặc biệt không có `coverage/` hay `*.out`) sau khi chạy; `git status` không đổi.
- [ ] (Giai đoạn B, sau duyệt) `coverage-ts` cho `agent/` ra báo cáo; thiếu `@vitest/coverage-v8` → `CODEINTEL_ENV_NOT_READY` với `reason:"coverage_provider_missing"`, không ra số 0.
- [ ] Cô lập tenant: tenant A không đọc được báo cáo của tenant B (kiểm thử hợp đồng với CR-CV-072).

## 5. Kiểm thử

- **Parser Go:** fixture profile thật (`mode: set`, `count`) cho nhiều module; ca có dòng trùng khối lồng nhau, khối đa dòng, tệp ngoài module; golden theo phiên bản Go (CR-CV-070 mở rộng).
- **`go tool cover -func`:** golden đầu ra theo phiên bản; ca hàm trùng tên.
- **Diff:** bảng ca đơn vị cho tiêu đề hunk (`@@ -1 +1 @@` không có số lượng, `,0`, xoá thuần, đổi tên tệp, tệp nhị phân, CRLF).
- **Tích hợp (agent):** repo git tạm có module Go nhỏ; chạy `coverage-go` thật, kiểm tra không ghi vào worktree.
- **Backend:** repository `coverage_reports` hai dialect (testcontainers như `backend-go-task-service.yml`), upsert idempotent, RLS tenant, cắt payload.
- **Hợp đồng:** nếu duyệt TS, golden `coverage-final.json` theo phiên bản vitest.
- **Không chạy** test này trong khi soạn CR; chỉ mô tả.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Chưa chạy** `go test -coverprofile` trên Orca: thời gian, RAM, và việc 21 module có test chạy được trên dev server (cần Postgres/MySQL? các test đơn vị "no Docker" theo workflow, test tích hợp bị tách bằng tag — suy từ `backend-go-task-service.yml`, chưa chạy). **Spike S1** trước khi làm.
- **Lệch cấu trúc repo ảnh hưởng profile TS:** `package.json` gốc có `"test"`/`typecheck` trỏ `config/vitest.config.ts`, `config/tsconfig.*.json` — các file này **không có ở gốc** (`config/` gốc chỉ có `scripts/`, `patches/`, `max-lines-baseline.txt`, `oxlint-react-doctor.json`); bản thật nằm ở `desktop/config/` (và `frontend/config/vitest.config.ts`). Profile TS vì thế phải theo cấu hình **từng gói**, không dùng lệnh gốc; cần xác nhận cách repo thật sự chạy test sau tách monorepo.
- **v8 coverage trong vitest với alias/`server.deps.inline`** có thể làm sai ánh xạ nguồn (source map) hoặc chậm đáng kể; chưa thử.
- **Thay đổi chưa commit:** số đo gắn `dirty`; khoá `tree_hash` cần CR-CV-081/082 xác nhận cách tính (Q5).
- **`-covermode=set` bỏ lỡ "chạy bao nhiêu lần"**; chấp nhận vì chỉ cần phủ/không phủ.
- **Heuristic "mọi khối đều phủ"** (K4) có thể làm diff coverage thấp hơn công cụ khác (Codecov dùng quy tắc riêng) → số không so sánh trực tiếp được với dịch vụ ngoài; ghi rõ định nghĩa trên UI.
- **Tệp chưa có test nào ở TS** sẽ không xuất hiện trong báo cáo vitest trừ khi cấu hình `coverage.include`/`all`; cần thiết lập `include` để tệp nguồn không bị "mất" khỏi mẫu số (chưa thử với vitest 4).
- **Fallback ước lượng** kế thừa báo nhầm của CR-CV-036 (gọi gián tiếp, `vi.mock`).
- **Dung lượng lưu** chưa đo.

## 7. Câu hỏi mở

- **Q1.** Có duyệt `@vitest/coverage-v8` (O12) không, ở gói nào trước (đề xuất `agent/` — gói nhỏ, tách biệt)?
- **Q2.** Có thêm profile `coverage-go-integration` (`-tags=integration`) không, hay chỉ đọc coverage đơn vị?
- **Q3.** Quy tắc "một dòng phủ nếu mọi khối chứa nó đều phủ" hay "nếu có khối phủ"?
- **Q4.** Module không có test: tính 0 % vào tổng hay loại khỏi tổng (nhãn `noTests`)?
- **Q5.** Khoá lưu cho worktree bẩn: `head_commit + dirty + tree_hash` hay chỉ chấp nhận đo khi sạch?
- **Q6.** Ngưỡng hiển thị `pct` thấp để đưa `uncoveredRanges` cho tệp không đổi (mặc định đề xuất 50 %)?
- **Q7.** Lưu `quality_trend_points` do CR-CV-085 hay CR này ghi điểm khi xong?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (mục 2 O9-O14, 3.10, 8)
- `/opt/repos/orca/docs/research/view-code/11-additions-for-quality-control.md` (B3, C4, E2-E4)
- `/opt/repos/orca/docs/crs/v7/code-intel-sources/CR-CV-036-change-overlay.md` (`uncoveredSymbols`, `mappingConfidence`, mergeBase)
- `/opt/repos/orca/docs/crs/v7/code-intel-sources/CR-CV-030-repo-file-access-gateway.md` (mục 1 điểm 4: `git.exec`, ký tự bị chặn, `quotePath`)
- `/opt/repos/orca/backend-go/go.work`, `/opt/repos/orca/backend-go/Makefile`, `/opt/repos/orca/backend-go/.golangci.yml`
- `/opt/repos/orca/.github/workflows/backend-go-task-service.yml`, `/opt/repos/orca/.github/workflows/pr.yml`
- `/opt/repos/orca/frontend/config/vitest.config.ts`, `/opt/repos/orca/desktop/config/vitest.config.ts`, `/opt/repos/orca/agent/vitest.config.ts`
- `/opt/repos/orca/guides/reference/git-compatibility.md`, `/opt/repos/orca/AGENTS.md`
- Cùng folder: `./README.md`; CR-CV-081, CR-CV-082 (do người soạn khác viết đồng thời, chỉ tham chiếu theo ID)
