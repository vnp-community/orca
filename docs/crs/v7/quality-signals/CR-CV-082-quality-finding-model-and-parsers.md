# CR-CV-082 — Mô hình `QualityFinding`, parser đầu ra công cụ và bảng `quality_runs`/`quality_findings`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-082 |
| **Tên** | Proto `QualityFinding`/`QualityRun`/`QualityStepResult` (`orca.codeintel.v1`); quy tắc `fingerprint` ổn định (không chứa số dòng); ánh xạ severity/category; parser cho oxlint, tsc, vitest, go vet, go test, golangci-lint, buf lint/breaking, opa test và script `check-*`; giới hạn, che đường dẫn/secret, chuẩn hoá đường dẫn; phát hiện trôi định dạng; fixture vàng theo phiên bản công cụ; schema `quality_runs`/`quality_findings` (Postgres + MySQL) và quy tắc nạp kết quả |
| **Loại** | Feature (nền dữ liệu chất lượng) |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | [CR-CV-081](./CR-CV-081-quality-runner-on-agent.md) (tệp đầu ra thô, bước chạy, che secret), [CR-CV-011](../code-intel-service-foundation/CR-CV-011-code-intel-data-model-and-repositories.md) (quy ước bảng, RLS, bảo trì), [CR-CV-070](../quality-rollout/CR-CV-070-golden-fixtures-and-tool-contract-tests.md) (cơ chế fixture/`MANIFEST.json`), [CR-CV-080](./CR-CV-080-agent-worktree-index-strategy-and-auto-refresh.md) (`IndexBasis`) |
| **Mở khoá** | CR-CV-083 (coverage là một loại phát hiện), CR-CV-084 (rule pack), CR-CV-085 (cổng; `ListQualityFindings`, `WaiveFinding` dùng `fingerprint`), CR-CV-086 (gộp CI: parser SARIF/GitHub dùng lại mô hình), CR-CV-087 (chú thích trên diff), CR-CV-089, CR-CV-090 |
| **Tác động** | Proto mới `backend-go/proto/orca/codeintel/v1/codeintel_quality.proto`; agent: `agent/src/relay/quality-parser-*.ts`, `quality-finding-fingerprint.ts`, `quality-repo-path-mapping.ts`, `quality-finding-limits.ts`; fixture dưới `agent/src/relay/codeintel/__fixtures__/quality/`; backend: migration + repository + use case nạp kết quả trong `code-intel-service`. Không sửa file của CR-CV-011/070 (mục 2.9 nêu chỗ cần nối) |

---

## 1. Bối cảnh và vấn đề

Đọc/chạy chỉ-đọc ngày 2026-10-06. Mọi định dạng đầu ra dưới đây ghi rõ **đã kiểm** (đọc mã/`--help` của chính công cụ hoặc của repo) hay **suy từ tài liệu công cụ, chưa chạy thật**; CR này không chạy `lint`, `test`, `analyze`.

### 1.1 Mỗi công cụ một định dạng, một phần nằm trong repo

| Công cụ | Phiên bản máy khảo sát | Định dạng có sẵn (đã kiểm bằng `--help`/đọc mã) | Dùng ở repo |
|---|---|---|---|
| oxlint | 1.71.0 (`node_modules/.bin/oxlint`, `package.json: ^1.71.0`) | `--format`: `checkstyle`, `default`, `agent`, `github`, `gitlab`, `json`, `junit`, `sarif`, `stylish`, `unix` (`oxlint --help`) | `pr.yml:64` `pnpm exec oxlint --format github` |
| tsc | "Version 7.0.2" (`package.json: typescript ^7.0.2`) | `--pretty`, `--noEmit` (`tsc --help`); dạng đầu ra văn bản `file(line,col): error TSnnnn: …` là hành vi quen thuộc của tsc, **chưa kiểm chứng cho 7.0.2** | `typecheck*` scripts |
| vitest | 4.1.5 | `--reporter`: `default, agent, minimal, blob, verbose, dot, json, tap, tap-flat, junit, tree, hanging-process, github-actions`; `--outputFile` (`vitest --help`) | `vitest run` |
| go vet / go test | go 1.26.0 | `go vet -json` ("emit JSON output", `go help vet`); `go test -json` (hành vi `test2json`, **chưa chạy**) | `go vet ./...`, `go test ./...` ở CI |
| golangci-lint | v1.62.2 (built go1.22.2) | `--out-format`: `json, line-number, colored-line-number, tab, colored-tab, checkstyle, code-climate, html, junit-xml, junit-xml-extended, github-actions, teamcity, sarif` (`run --help`) | `make lint` (không có trong CI, CR-CV-081 1.2) |
| buf | 1.72.0 | `lint --error-format`: `text,json,msvs,junit,github-actions,gitlab-code-quality,config-ignore-yaml`; `breaking --error-format`: `text,json,msvs,junit,github-actions,gitlab-code-quality` | `backend-go-mcp-service.yml`, `make proto-lint` |
| opa | 1.19.1 | `opa test --format {pretty,json,gobench}`, `--coverage`, `--threshold` | `make opa-test` |
| `check-max-lines-ratchet.mjs` | repo | Đã đọc mã: lỗi in `::error::New max-lines bypass not allowed: <entry>` (`:128`) và `::error::Stale max-lines baseline entry (prune it): <entry>` (`:162`), kèm khung văn bản; `<entry>` có dạng `inline <đường dẫn>` hoặc dòng từ `collectMobileBumps` | `pnpm check:max-lines-ratchet` |
| `check-styled-scrollbars.mjs` | repo | Đã đọc mã: 4 dòng tiêu đề rồi mỗi dòng `<đường dẫn>:<dòng>:<cột> <văn bản>` (`formatReports`), thoát 1 | `desktop/config/scripts/` |
| `check-reliability-gates.mjs` | repo | Đã đọc mã: `Reliability gate manifest check failed with N issue(s):` rồi `- <gateId>: <thông điệp>`; thành công: `Reliability gate manifest check passed for N gate(s).` | `desktop/config/scripts/` |

### 1.2 Hậu quả

- Không có mô hình chung nên UI và cổng (CR-CV-085) không lọc/so sánh được; không có `fingerprint` ổn định nên "đã miễn trừ", "mới so với lượt trước", xu hướng đều không làm được (số dòng đổi khi agent chèn code).
- Nhiều công cụ chưa có ghim phiên bản trong repo: `golangci-lint`, `buf`, `opa` cài ngoài (`backend-go-mcp-service.yml` dùng `bufbuild/buf-setup-action@v1` không ghim), nên **trôi định dạng là rủi ro thật**; oxlint/vitest/typescript có ghim caret trong `package.json` (`^1.71.0`, `^4.1.5`, `^7.0.2`).
- Công cụ trả đường dẫn tương đối theo thư mục chạy (`cwd` của từng profile, CR-CV-081 2.3), có thể là đường dẫn tuyệt đối (go test, tsc `--listFiles` không dùng), hoặc đường dẫn module cache; cần ánh xạ về gốc repo.

## 2. Giải pháp đề xuất

### 2.1 Proto (`orca.codeintel.v1`, file `codeintel_quality.proto`, mới)

Chuỗi (không enum) cho `severity`/`category`/`status` để thêm giá trị không phá tương thích, kiểm tập giá trị ở domain (cùng quy ước `reindex_jobs.mode` ở CR-CV-011). Chỉ định nghĩa **message**; RPC của `QualityGateService` thuộc CR-CV-085 (không khai báo RPC chưa có message, quy ước v6).

```proto
message QualityFinding {
  string fingerprint = 1;            // 32 hex, quy tắc 2.2; không chứa số dòng
  string rule_id = 2;                // "<tool>/<rule>" đã chuẩn hoá, ví dụ "oxlint/eslint/no-unused-vars", "tsc/TS2322"
  string severity = 3;               // error | warning | info
  string category = 4;               // lint|typecheck|test|coverage|complexity|security|dependency|convention|architecture|ai
  string file = 5;                   // tương đối gốc repo, dấu '/', rỗng nếu không gắn tệp
  int32 line = 6;                    // 1-based; 0 = không có
  int32 end_line = 7;                // 0 = như line
  int32 column = 8;                  // 1-based; 0 = không có (thêm so với README 3.10)
  int32 end_column = 9;              // (thêm)
  string message = 10;               // đã che, ≤ 2 KiB
  string tool = 11;                  // "oxlint"|"tsc"|"vitest"|"go-vet"|"go-test"|"golangci-lint"|"buf"|"opa"|"orca-check"
  string tool_version = 12;
  string fix_hint = 13;              // chỉ khi công cụ tự cung cấp; ≤ 500 ký tự; không bao giờ tự chạy
  string step_id = 14;               // id bước của run (CR-CV-081), ví dụ "go-test:services/project-service" (thêm)
  bool in_scope = 15;                // false: thuộc tệp ngoài phạm vi changed/commitRange (CR-CV-081 2.5) (thêm)
  int32 fp_version = 16;             // phiên bản thuật toán fingerprint, hiện 1 (thêm)
}

message QualityStepResult {
  string id = 1;                     // "ts-lint", "go-test:services/project-service"
  string profile_id = 2;
  string status = 3;                 // passed|findings|failed|timeout|cancelled|skipped|env_not_ready
  string failure_kind = 4;           // "" | format_drift | output_too_large | exit_unexpected | parser_error | env (kèm env_reason)
  string env_reason = 5;
  int32 exit_code = 6;               // -1 nếu không có
  int64 duration_ms = 7;
  string tool = 8; string tool_version = 9; string definition_hash = 10;
  int32 error_count = 11; int32 warning_count = 12; int32 info_count = 13;
  int32 total_count = 14;            // trước khi cắt
  bool truncated = 15;
  int32 outside_scope_count = 16;
}

message QualityRunSummary { int32 error = 1; int32 warning = 2; int32 info = 3;
  int32 steps_total = 4; int32 steps_with_findings = 5; int32 steps_failed = 6; int32 steps_env_not_ready = 7;
  int32 outside_scope = 8; bool truncated = 9; }

message QualityRun {
  string id = 1; string worktree_id = 2;              // chuỗi tham chiếu worktree (3 dạng id, CR-CV-011 mục 1); đừng ép UUID
  string head_commit = 3; string index_commit = 4;
  string scope = 5; string base_commit = 6;
  string profile = 7;
  string status = 8;                                  // queued|running|succeeded|failed|cancelled (README 3.10)
  string source = 9;                                  // local | ci
  google.protobuf.Timestamp started_at = 10; google.protobuf.Timestamp finished_at = 11;
  QualityRunSummary summary = 12;
  repeated QualityStepResult steps = 13;
  repeated IndexBasis index_basis = 14;               // CR-CV-080
  string dirty_fingerprint = 15; bool work_tree_changed_during_run = 16; bool scope_widened = 17;
  string agent_run_id = 18;
}
```

README 3.10 viết `QualityRun.worktreeId` dưới dạng định danh; ở đây giữ đúng tên nhưng lưu chuỗi vì id worktree có ba dạng (CR-CV-011 mục 1). Trường thêm so với README được liệt kê ở "Điều chỉnh hợp đồng" của báo cáo.

### 2.2 Quy tắc `fingerprint` (phiên bản 1)

Mục tiêu: cùng một vấn đề giữ nguyên fingerprint khi code xung quanh thay đổi hoặc dòng dịch chuyển; vấn đề khác nhau thì khác; không chứa số dòng.

```
anchor      = chuẩn hoá(văn bản dòng gốc của phát hiện)          // 2.2.1; "" nếu không có dòng hoặc không đọc được tệp
normMessage = chuẩn hoá thông điệp                                // 2.2.2
key         = "v1" ␀ tool ␀ ruleId ␀ file ␀ anchor ␀ normMessage ␀ occurrence
fingerprint = hex(sha256(key))[0:32]                              // 128 bit
occurrence  = thứ tự (0-based) của phát hiện trong tập có cùng (tool, ruleId, file, anchor, normMessage), sắp theo (line, column)
```

#### 2.2.1 `anchor`

| Loại phát hiện | `anchor` |
|---|---|
| Có dòng trong tệp nguồn (oxlint, tsc, go vet, golangci-lint, buf, `check-styled-scrollbars`) | Văn bản dòng `line` đọc từ cây làm việc: bỏ khoảng trắng đầu/cuối, gộp mọi chuỗi khoảng trắng thành một dấu cách, cắt 512 ký tự; không đọc được (tệp lớn hơn 2 MiB, nhị phân, đã xoá) → `""` |
| Kiểm thử thất bại (vitest, go test, opa) | Tên đầy đủ: vitest `ancestorTitles.join(" › ") + " › " + title`; go test `<package>/<Test>/<Subtest>`; opa `<package>.<name>` (ví dụ `data.orca.authz.admin.test_x`). Không dùng số dòng hay thời lượng |
| Ở mức tệp (vitest suite hỏng khi nạp, `go.work` lỗi) | `""` (một phát hiện cho mỗi tệp) |
| `check-reliability-gates` | `<gateId>` |
| `check-max-lines-ratchet` | `<entry>` nguyên văn (đã là dạng ổn định `inline <đường dẫn>`) |

Đánh đổi (chọn có chủ ý): sửa **chính dòng** đó (đổi nội dung) làm đổi fingerprint và phát hiện sẽ thành "mới + một phát hiện biến mất". Chấp nhận vì dòng đã sửa thường là cách xử lý phát hiện; cách khác (băm cả hàm bao quanh) cần parser theo ngôn ngữ, không có sẵn ở agent (CodeGraph/GitNexus có `symbol` nhưng phụ thuộc index, CR-CV-080).

#### 2.2.2 `normMessage`

Áp theo thứ tự: (1) đổi đường dẫn tuyệt đối repo → `<repo>`, `HOME` → `~`, thư mục tạm → `<tmp>`; (2) thay mọi `<đường dẫn>.<ext>:<dòng>(:<cột>)?` và `(<dòng>,<cột>)` và `line <n>` bằng `<loc>`; (3) bỏ chuỗi thời lượng/ngày giờ (`\d+(\.\d+)?(ms|s)`, ISO-8601); (4) gộp khoảng trắng; (5) cắt 1024 ký tự đầu. **Không** bỏ tên định danh trong nháy (`'foo'`) vì nó phân biệt vấn đề (ví dụ `'x' is declared but never used` ở hai biến khác nhau trên cùng một dòng văn bản trùng nhau vẫn khác nhờ `occurrence`).

#### 2.2.3 Tính chất bắt buộc và kiểm thử (mục 5)

| Hành động | Fingerprint |
|---|---|
| Chèn/xoá dòng phía trên (dòng dịch) | **Giữ** |
| Đổi khoảng trắng/thụt đầu dòng của dòng đó | **Giữ** |
| Sửa nội dung dòng | Đổi (chấp nhận, 2.2.1) |
| Hai phát hiện giống hệt trong một tệp | Khác nhau nhờ `occurrence` |
| Đổi tên tệp | Đổi (không truy vết đổi tên ở v1; CR-CV-085 có thể dùng `git diff -M` để nối, ngoài phạm vi) |
| Nâng `toolVersion` | **Giữ** nếu `ruleId` và thông điệp chuẩn hoá không đổi (`toolVersion` **không** nằm trong khoá) |
| Đổi `ruleId` giữa phiên bản công cụ | Đổi: fixture vàng (2.8) phải bắt trước khi vào bản phát hành |
| Thêm bước chạy (cùng tệp bị hai công cụ báo) | Khác nhau nhờ `tool` trong khoá |

`fp_version` tăng khi đổi thuật toán; bảng lưu `fp_version` để migration/so sánh biết không so sánh hai phiên bản (CR-CV-085 coi phát hiện khác `fp_version` là "không so sánh được", không phải "mới"). Khoá miễn trừ `quality_waivers` (CR-CV-085) dùng `(repo_id, fingerprint, fp_version)`, khác với `finding_dismissals.finding_key` của CR-CV-037 (phát hiện cấu trúc; hai bảng không trộn).

### 2.3 Ánh xạ severity và category

`severity ∈ {error, warning, info}`. Quy tắc chung: mức do công cụ cung cấp được dùng nếu có; bảng dưới là mặc định khi công cụ không phân biệt. Đây là quyết định chính sách (Q1), không phải sự thật của công cụ.

| Công cụ | `severity` | `category` | `ruleId` |
|---|---|---|---|
| oxlint | `error`→error, `warning`→warning, `advice`→info (giá trị theo tài liệu oxlint, chưa chạy) | `lint` (cấu hình `.oxlintrc.json` đặt `categories.correctness: error`, đã đọc đầu tệp) | `oxlint/<plugin>/<rule>` từ mã dạng `plugin(rule)`; không phân tích được → `oxlint/unknown` |
| tsc | `error`→error, `warning`→warning | `typecheck` | `tsc/TS<nnnn>`; lỗi toàn cục không có tệp: `file=""` |
| vitest | test thất bại → error; suite không nạp được (import/syntax) → error | `test` | `vitest/test-failed`, `vitest/suite-failed` |
| go vet | error | `lint` | `go-vet/<analyzer>` (từ `-json`); dạng text không có tên analyzer → `go-vet/unknown` |
| go test | test thất bại → error; `panic`/timeout → error; lỗi biên dịch gói → error | `test`; lỗi biên dịch → `typecheck` | `go-test/test-failed`, `go-test/panic`, `go-test/timeout`, `go-test/build-failed` |
| golangci-lint | theo bảng linter dưới | `lint` (`typecheck` linter → `typecheck`) | `golangci/<linter>[/<rule>]` |
| buf lint | warning | `lint` | `buf/<TYPE>` (ví dụ `buf/FIELD_LOWER_SNAKE_CASE`) |
| buf breaking | error | `architecture` (phá hợp đồng giữa service, README 3.10 có `architecture`) | `buf-breaking/<TYPE>` |
| opa test | test thất bại → error; lỗi biên dịch rego → error | `test`; biên dịch → `typecheck` | `opa/test-failed`, `opa/compile-error` |
| `check-*` | error (CI chặn) trừ `stale-baseline` → warning | `convention` | `orca-check/<script>/<kind>` |

Linter của `backend-go/.golangci.yml` (12 linter đã bật): error: `typecheck`, `errcheck`, `staticcheck`, `govet`, `bodyclose`, `noctx`, `errorlint`; warning: `ineffassign`, `unused`, `gosimple`, `gocritic`; info: `gofmt`, `goimports`. Linter không có trong bảng → warning. Bảng là dữ liệu trong `quality-parser-golangci.ts`, kèm test khẳng định mọi linter trong `.golangci.yml` có dòng (chống bỏ sót khi bật linter mới).

### 2.4 Hợp đồng parser

```ts
// quality-parser-types.ts (mới)
export type QualityParserInput = {
  stepId: string
  stdoutPath: string; stderrPath: string; extraPath?: string   // tệp đầu ra từ CR-CV-081 (vitest.json, ...)
  exitCode: number | null; timedOut: boolean; cancelled: boolean
  cwd: string; repoRoot: string; platform: NodeJS.Platform
  toolVersion: string; scopeFiles: ReadonlySet<string> | null  // null = worktree
  readSourceLine(repoRelativeFile: string, line: number): Promise<string | null>
}
export type QualityParserOutput = {
  findings: RawQualityFinding[]                  // chưa fingerprint, chưa cắt
  failure: null | { kind: 'format_drift' | 'parser_error' | 'env'; envReason?: string; detail: string }  // detail không chứa nội dung thô
  stats: { totalParsed: number; testsTotal?: number; testsFailed?: number; testsSkipped?: number }
}
export type QualityParser = { key: string /* "oxlint@json" */; parse(input: QualityParserInput): Promise<QualityParserOutput> }
```

Quy tắc: parser là **hàm thuần theo đầu vào** (trừ `readSourceLine`), đọc tệp theo luồng (đầu ra `go test -json` có thể hàng chục MiB, 081 2.7), không throw (lỗi → `failure`), không in nội dung thô vào `detail`. Bộ xử lý chung (`quality-finding-pipeline.ts`) sau parser: ánh xạ đường dẫn (2.6) → che (2.6) → `fingerprint` (2.2) → `in_scope` → sắp xếp và cắt (2.5) → trạng thái bước.

**Suy trạng thái bước** (thay quy tắc dựa mã thoát đơn thuần vì `go vet -json` có thể thoát 0 có phát hiện): `cancelled`/`timeout` (từ executor) → giữ; `failure.kind=env` → `env_not_ready`; `failure.*` khác → `failed`; mã thoát nằm ngoài `exit.ok ∪ exit.findings` của profile **và** không có phát hiện → `failed (exit_unexpected)`; có phát hiện → `findings`; còn lại → `passed`.

**Chống "mất phát hiện thầm lặng"**: nếu mã thoát ∈ `exit.findings` của profile (ví dụ oxlint thoát 1, `check-*` thoát 1) mà parser trả 0 phát hiện và không `failure` → coi là `failure.kind=format_drift` (parser không hiểu đầu ra), không phải `passed`. Với `go vet -json` (thoát 0 kể cả có phát hiện) quy tắc này không áp dụng.

### 2.5 Giới hạn kích thước

| Giới hạn | Mặc định | Hành vi |
|---|---|---|
| Phát hiện mỗi bước (`QUALITY_MAX_FINDINGS_PER_STEP`) | 5 000 | Cắt, `truncated=true`, `total_count` giữ số thật |
| Phát hiện mỗi run | 20 000 | như trên; cột `findings_truncated` |
| `message` | 2 048 byte UTF-8 (cắt ở ranh giới ký tự, thêm `…`) | |
| `fix_hint` | 500 ký tự | |
| Dòng nguồn đọc cho `anchor` | 512 ký tự; tệp ≤ 2 MiB | vượt → `anchor=""` |
| Kích thước tệp đầu ra mỗi bước | theo profile (32/64 MiB, CR-CV-081 2.7) | vượt đã bị bước `failed (output_too_large)` trước khi tới parser |
| Một trang `quality.results` / `ListQualityFindings` | ≤ 500 mục và ≤ 1 MiB | gRPC của repo đang ở trần mặc định 4 MiB (README v7 mục 8 điểm 16) |

Thứ tự khi cắt (ổn định, để cùng một run luôn giữ cùng tập): (1) `error` trước `warning` trước `info`; (2) `in_scope=true` trước; (3) tệp thuộc phạm vi (nếu có `scopeFiles`) trước; (4) theo `file`, `line`, `column`, `ruleId`. Số phát hiện bị bỏ theo `severity` ghi vào `QualityStepResult` (`error_count` v.v. là số **trước** khi cắt) để cổng không "pass" nhờ cắt: CR-CV-085 phải dùng các số đếm này, không đếm lại các dòng đã lưu.

Phát hiện ngoài phạm vi (`in_scope=false`): vẫn lưu (cho `full-run-filter`, CR-CV-081 2.5) nhưng cổng mặc định chỉ tính `in_scope=true`.

### 2.6 Ánh xạ đường dẫn, che đường dẫn tuyệt đối và secret

`quality-repo-path-mapping.ts` (mới): `toRepoRelative(printedPath, cwd, repoRoot, platform) → { file: string; outside: boolean }`.

1. `abs = path.isAbsolute(p) ? p : path.resolve(cwd, p)` (dùng `path.win32` khi `platform=win32`, dù MVP chưa hỗ trợ).
2. `rel = path.relative(repoRoot, abs)`; đổi `\` → `/`.
3. Nếu `rel` bắt đầu bằng `..` hoặc là tuyệt đối khác ổ (cache module Go `~/go/pkg/mod/...`, `node_modules` ngoài repo, thư mục tạm): `file=""`, `outside=true`, tăng bộ đếm `outsideRepoCount` của bước; **không bao giờ** đưa đường dẫn tuyệt đối ra `file`.
4. Đường dẫn do công cụ in tương đối theo **thư mục chạy**: `golangci-lint`, `go vet`, `go test` ở `backend-go/services/<s>/` in `internal/...`; `buf` ở `backend-go/proto/` in `orca/mcp/v1/x.proto` (tương đối thư mục chứa `buf.yaml`); `check-styled-scrollbars` ở `desktop/` in `src/renderer/src/...`; `check-max-lines-ratchet` ở gốc in đường dẫn tương đối gốc. Mỗi profile khai báo `cwd` nên phép ánh xạ luôn có đủ thông tin; test từng công cụ kiểm tra việc nối `cwd`.
5. Không dùng `realpath` cho từng phát hiện (tốn kém, symlink); chỉ dùng cho `cwd`/`repoRoot` một lần. Phát hiện ở tệp qua symlink trỏ ra ngoài repo: `outside=true`.
6. Chữ hoa/thường: giữ nguyên như công cụ in; so sánh `scopeFiles` theo chuẩn hoá của nền tảng (không phân biệt hoa/thường trên macOS/Windows). Chưa kiểm chứng trên macOS/Windows.

Che: `message`, `fix_hint`, `anchor` (trước khi băm: băm trên văn bản **chưa che** để ổn định, nhưng **không lưu** văn bản dòng nguồn ở đâu cả) đi qua `quality-output-redaction.ts` của CR-CV-081 2.8 (giá trị env nhạy cảm, mẫu token, `user:pass@`, đường dẫn tuyệt đối → `<repo>`/`~`/`<tmp>`). Dòng nguồn dùng làm `anchor` có thể chứa secret hằng trong mã (ví dụ khoá trong test): chỉ **băm** nó (SHA-256 trong khoá fingerprint), không lưu, không đưa ra RPC; `message` của công cụ có thể trích nguyên dòng (tsc, golangci `SourceLines`): bộ che áp dụng; **`SourceLines` của golangci-lint và `source`/`snippet` của công cụ khác bị bỏ, không lưu**.

### 2.7 Parser theo công cụ

Mỗi parser có khoá `<tool>@<format>`; phiên bản công cụ tương ứng lấy từ `<tool> --version` (cache 10 phút) và đối chiếu `SUPPORTED_QUALITY_TOOL_VERSIONS` (2.8). Cột "Đã kiểm" nói nguồn của mô tả định dạng.

| Khoá | Lệnh (CR-CV-081 2.3) | Cấu trúc đầu vào | Ánh xạ | Đã kiểm |
|---|---|---|---|---|
| `oxlint@json` | `oxlint --format json` | JSON một đối tượng: `diagnostics[]` với `message`, `code`, `severity`, `filename`, `labels[].span{offset,length,line,column}`, `help`; kèm `number_of_files`, `number_of_rules` | `file=filename`, `line/column` từ nhãn chính (`labels[0]`), `end_line` = `line` + số `\n` trong đoạn nếu tính được (không thì 0), `fix_hint=help` | `--format json` có trong `--help` (đã kiểm); **hình dạng JSON suy từ tài liệu oxlint, chưa chạy**; bước chụp fixture quyết định giữ hay chuyển `oxlint@github` |
| `oxlint@github` (dự phòng) | `oxlint --format github` | Dòng `::error title=<plugin>(<rule>),file=<f>,line=<l>,endLine=<l>,col=<c>,endColumn=<c>::<msg>` (cú pháp annotation GitHub); `::warning` | cùng ánh xạ; `severity` từ `error|warning|notice` | `pr.yml:64` dùng định dạng này (đã kiểm); hình dạng dòng từ tài liệu GitHub Actions, chưa chạy |
| `tsc@text` | `tsc --noEmit -p … --pretty false` | Mỗi lỗi `path(line,col): error TS2322: message`; dòng tiếp nối thụt đầu dòng nối vào `message` (tối đa 10 dòng, 2 KiB); lỗi toàn cục `error TS5083: …` không có tệp | `rule=TS<n>`; tsc không có endLine → 0 | **Chưa kiểm chứng cho tsc 7.0.2** (bản 7 là bản viết lại; chấp nhận hình dạng bằng fixture) |
| `vitest@json` | `vitest run --reporter=json --outputFile=<tmp>` | Đối tượng kiểu Jest: `numFailedTests`, `testResults[].{name, status, message, assertionResults[].{ancestorTitles, title, fullName, status, failureMessages[], location?}}` | Mỗi `assertionResult.status === "failed"` → một phát hiện: `file=testResults.name`, `line` từ `location.line` nếu có, nếu không thì khung stack đầu tiên khớp tệp test trong `failureMessages[0]`; `message` = dòng đầu của `failureMessages[0]`; `testResults[].status==="failed"` mà không có `assertionResults` lỗi → `suite-failed` với `message` từ `testResults.message`. Bỏ qua `skipped`/`todo`, chỉ đếm | `--reporter json`, `--outputFile` có trong `--help` (đã kiểm); hình dạng JSON theo tài liệu vitest, chưa chạy |
| `govet@json` | `go vet -json ./...` | Chuỗi đối tượng JSON `{ "<pkg>": { "<analyzer>": [ { "posn": "file:line:col", "message": "…" } ] } }` (có thể nhiều đối tượng nối tiếp) lẫn dòng văn bản `# <pkg>` và lỗi biên dịch dạng text; **kênh (stdout hay stderr) chưa kiểm chứng**: parser đọc cả hai tệp | `rule=go-vet/<analyzer>`; `posn` tách bằng regex từ cuối (đường dẫn Windows có `:`); lỗi biên dịch text → `go-vet/build-failed` category `typecheck` | `-json` có trong `go help vet` (đã kiểm); cấu trúc theo tài liệu `go vet`/`unitchecker`, chưa chạy |
| `govet@text` (dự phòng) | `go vet ./...` | `# <pkg>` rồi `file.go:12:2: message`; không có tên analyzer | `rule=go-vet/unknown` | Chưa chạy |
| `gotest@json` | `go test -json ./...` | Dòng JSON `{Time, Action, Package, Test, Output, Elapsed[, FailedBuild]}`; `Action ∈ run,pause,cont,pass,fail,skip,output,start`. Gom `Output` theo `(Package, Test)`; `fail` với `Test` → phát hiện test; `fail` mức gói không có `Test` và đầu ra có `file.go:line:col:` → build lỗi (`FailedBuild` có từ Go 1.24 theo hiểu biết của người soạn, **xác nhận khi chụp fixture**); `panic:` → panic; vị trí: dòng `    file_test.go:34: …` đầu tiên trong đầu ra của test | `anchor` = `<Package>/<Test>` | Cấu trúc `test2json`, chưa chạy |
| `golangci@json` | `golangci-lint run --out-format json ./...` | Một đối tượng: `Issues[].{FromLinter, Text, Severity, Pos{Filename,Line,Column}, SourceLines, Replacement}`, `Report` | `rule=golangci/<FromLinter>`; severity theo 2.3 nếu `Severity==""`; `fix_hint` từ `Replacement` nếu có (cắt, không bao giờ áp dụng) | `--out-format json` có trong `run --help` (đã kiểm); hình dạng theo tài liệu v1, chưa chạy; golangci-lint v2 đổi cờ (`--output.json.path`) nên phiên bản ngoài `1.x` là `incompatible` |
| `buf@json` | `buf lint --error-format json` / `buf breaking … --error-format json` | Một đối tượng JSON mỗi dòng: `{path, start_line, start_column, end_line, end_column, type, message}` | `rule=buf/<type>` hoặc `buf-breaking/<type>`; `path` tương đối thư mục `buf.yaml` | `--error-format json` có trong `--help` (đã kiểm); các khoá theo tài liệu buf, chưa chạy |
| `opa@json` | `opa test … --format json` | Mảng `[ {location{file,row,col}, package, name, duration, fail?: true, error?: {…}} ]` | `fail` → `opa/test-failed`, `anchor=<package>.<name>`; `error` có `code` biên dịch → `opa/compile-error` | `--format json` có trong `--help` (đã kiểm); hình dạng theo tài liệu OPA, chưa chạy |
| `orca-check@max-lines` | `node config/scripts/check-max-lines-ratchet.mjs` | Dòng `::error::New max-lines bypass not allowed: <entry>` và `::error::Stale max-lines baseline entry (prune it): <entry>` | `<entry>` bắt đầu `inline ` → `file=<đường dẫn>`; mục khác (mobile) → `file="mobile/.oxlintrc.json"`; `kind=new-bypass` (error) hoặc `stale-baseline` (warning); bỏ các dòng khung | **Đã đọc mã** (`check-max-lines-ratchet.mjs:128,162`) |
| `orca-check@styled-scrollbars` | `node config/scripts/check-styled-scrollbars.mjs` | Bỏ 4 dòng tiêu đề; mỗi dòng còn lại `<path>:<line>:<col> <text>` | `file` nối `desktop/` (cwd), `rule=orca-check/styled-scrollbars/unstyled`, `message`=hướng dẫn cố định + `<text>` (cắt) | **Đã đọc mã** (`formatReports`, `main`) |
| `orca-check@reliability-gates` | `node config/scripts/check-reliability-gates.mjs` | Tiêu đề `Reliability gate manifest check failed with N issue(s):` rồi `- <gateId>: <msg>` hoặc `- <msg>` | `file="desktop/config/reliability-gates.jsonc"`, `anchor=<gateId>`, `rule=orca-check/reliability-gates/invalid-manifest` | **Đã đọc mã** (`:395-431`) |

CR-CV-084 sở hữu **ngữ nghĩa** quy tắc từ `AGENTS.md` (tên file mơ hồ, `metaKey` cứng, …); CR này chỉ chuyển đầu ra các script `check-*` sẵn có thành `QualityFinding`.

### 2.8 Phát hiện trôi định dạng và fixture vàng (mở rộng CR-CV-070, không sửa)

Tái dùng nguyên cơ chế của CR-CV-070 2.3-2.7 (`MANIFEST.json`, mã băm, ngân sách 20 KiB/tệp và 300 KiB/thư mục phiên bản, bộ che, chụp lại bằng PR riêng nêu phiên bản, `SUPPORTED_*` là dữ liệu). Phần thêm:

```
agent/src/relay/codeintel/__fixtures__/
  quality-mini-repo/              # mã nguồn mẫu CÓ CHỦ Ý SAI (mục dưới), không có node_modules/.gitnexus
  quality/
    oxlint/1.71.0/        MANIFEST.json  stdout.json  stdout.json.expected.json  exit.txt  stderr.txt
    tsc/7.0.2/            MANIFEST.json  stdout.txt   *.expected.json ...
    vitest/4.1.5/         MANIFEST.json  vitest.json  ...
    go-vet/go1.26.0/      go-test/go1.26.0/  (đặt theo phiên bản Go)
    golangci-lint/1.62.2/ buf/1.72.0/ opa/1.19.1/
    orca-check/<commit-của-script>/   # không có phiên bản công cụ: dùng băm nội dung script (sha256 của tệp) làm "phiên bản"
```

- `quality-mini-repo/` (mới; **không** gộp vào `mini-repo/` của CR-CV-070 để không đụng fixture đồ thị): một tệp TS có biến không dùng (oxlint) và một lỗi kiểu (tsc), một test vitest cố ý thất bại và một suite lỗi import, một gói Go có `fmt.Printf` sai định dạng (vet), một test Go thất bại + biến thể lỗi biên dịch, một lỗi `errcheck` (golangci), một `.proto` vi phạm quy tắc đặt tên (buf lint) và một commit thứ hai phá tương thích (buf breaking), một test Rego thất bại, tên tệp có khoảng trắng và Unicode, thông điệp có CRLF và rất dài, hai phát hiện giống hệt trong cùng một tệp.
- `MANIFEST.json` mở rộng khoá `markers` (ví dụ `{"format":"json"}`) và `argv` mẫu đúng bằng argv của profile (một nguồn sự thật với CR-CV-081 qua cùng hằng số, chống lệch).
- Mỗi tệp thô có `*.expected.json` là mảng `QualityFinding` đã qua toàn bộ pipeline (cắt/che/fingerprint): người duyệt kiểm tay lần đầu.
- Script chụp `agent/scripts/capture-quality-fixtures.mjs` (mới; mở rộng cấu trúc của `capture-codeintel-fixtures.mjs` ở CR-CV-070 2.4; tên theo nội dung): chạy **đúng argv của profile** trong thư mục tạm trên `quality-mini-repo/`, dừng nếu `--version` khác phiên bản đích, che đường dẫn, in `git diff --stat`. Các công cụ ở đây là linter/trình kiểm thử chạy trên **repo mẫu trong thư mục tạm**, không phải trên repo người dùng.
- Phát hiện trôi (CR-CV-070 2.6 áp dụng):

| Trạng thái | Điều kiện | Hành vi |
|---|---|---|
| `verified` | `<tool> --version` nằm trong `SUPPORTED_QUALITY_TOOL_VERSIONS` | phục vụ |
| `untested` | cùng major, minor/patch lạ | vẫn chạy; `QualityStepResult` có cờ cảnh báo; metric `orca_quality_tool_version_untested_total{tool}` |
| `incompatible` | major khác (ví dụ golangci-lint 2.x, `--out-format` đổi) | bước `env_not_ready (tool_incompatible)`, không chạy |
| `format_drift` | shape guard của parser sai (khoá bắt buộc thiếu, không phải JSON, cột sai), hoặc quy tắc "mã thoát báo phát hiện mà parser trả 0" (2.4) | bước `failed (format_drift)`; **không** đưa đầu ra thô vào `detail`; metric `orca_quality_parser_drift_total{tool,format}`; log một lần mỗi `(tool, version, format)` |

  Phiên bản đang dùng trên máy khảo sát (làm giá trị khởi đầu của `SUPPORTED_QUALITY_TOOL_VERSIONS`): oxlint `1.71.0`, tsc `7.0.2`, vitest `4.1.5`, go `1.26.0`, golangci-lint `1.62.2`, buf `1.72.0`, opa `1.19.1`. Ghim trong repo chỉ có oxlint/typescript/vitest (caret); `golangci-lint`, `buf`, `opa` cài ngoài nên cần cảnh báo `untested` hơn là chặn.
- Test hợp đồng chống lệch phiên bản ghim: một test đọc `package.json` (`oxlint`, `typescript`, `vitest`) và khẳng định khoảng caret **chứa** một phiên bản có thư mục fixture; PR nâng phiên bản mà không chụp fixture sẽ đỏ.
- Job nightly `code-intel-live-contract` (CR-CV-070 2.7) thêm bước chạy lại script chụp quality với công cụ cài sẵn trên runner, so sánh và báo cáo (không chặn).

### 2.9 Schema `quality_runs` và `quality_findings` (đồng bộ CR-CV-011)

Migration `0003_quality_runs_and_findings` (đánh số giả định: CR-CV-011 dùng `0002_code_intel_core`; người triển khai đánh lại cho đúng thứ tự). Quy ước như CR-CV-011 2.1: id do ứng dụng sinh; Postgres `UUID/TIMESTAMPTZ/JSONB/BOOLEAN`, MySQL `CHAR(36)/TIMESTAMP(6)/JSON/TINYINT(1)`; không FK (kể cả giữa hai bảng này; dọn bằng ứng dụng); `tenant_id NOT NULL`; Postgres `ENABLE` + `FORCE ROW LEVEL SECURITY` với `tenant_isolation` (`NULLIF(current_setting('app.tenant_id', true), '')::uuid`) và chính sách bảo trì hẹp như CR-CV-011 2.5; đồng hồ DB cho `*_at`. Cột ghi **(bổ sung)** là thêm so với README 3.10 (README chỉ có tên bảng).

**`quality_runs`**

| Cột | Postgres | MySQL | Ghi chú |
|---|---|---|---|
| `id` | `uuid` PK | `char(36)` PK | |
| `tenant_id` | `uuid NOT NULL` | `char(36) NOT NULL` | |
| `repo_id` | `uuid NOT NULL` | `char(36) NOT NULL` | khoá nghiệp vụ cho xu hướng/miễn trừ, sống sót khi worktree/binding bị xoá (cùng lý do `c4_overrides`, CR-CV-011 mục 1 điểm 2) |
| `repo_binding_id` | `uuid NOT NULL` | `char(36) NOT NULL` | |
| `worktree_ref` | `varchar(255) NOT NULL` | `varchar(255) NOT NULL` | chuỗi id worktree (3 dạng); proto `worktree_id` |
| `agent_run_id` | `varchar(64) NOT NULL` | `varchar(64) NOT NULL` | `runId` của agent (ULID) hoặc id ngoài với `source=ci` |
| `head_commit` | `varchar(64) NOT NULL` | idem | |
| `index_commit` | `varchar(64) NOT NULL DEFAULT ''` | idem | README `indexCommit` |
| `index_basis` | `jsonb NULL` | `json NULL` | **(bổ sung)** `IndexBasis[]`, ≤ 4 KiB (CR-CV-080 2.5) |
| `scope` | `varchar(16) NOT NULL CHECK (scope IN ('worktree','changed','commitRange'))` | idem (MySQL < 8.0.16 bỏ qua CHECK: kiểm ở domain) | |
| `base_commit` | `varchar(64) NOT NULL DEFAULT ''` | idem | **(bổ sung)** |
| `profile` | `varchar(64) NOT NULL` | idem | id profile hoặc suite |
| `status` | `varchar(16) NOT NULL CHECK (status IN ('queued','running','succeeded','failed','cancelled'))` | idem | README |
| `source` | `varchar(8) NOT NULL CHECK (source IN ('local','ci'))` | idem | README; CR-CV-086 có thể thêm cột liên kết CI, **không đặt trước ở đây** |
| `dirty_fingerprint` | `varchar(80) NOT NULL DEFAULT ''` | idem | **(bổ sung)** |
| `work_tree_changed` | `boolean NOT NULL DEFAULT false` | `tinyint(1) NOT NULL DEFAULT 0` | **(bổ sung)** `workTreeChangedDuringRun` |
| `scope_widened` | `boolean NOT NULL DEFAULT false` | idem | **(bổ sung)** |
| `error_count`, `warning_count`, `info_count` | `integer NOT NULL DEFAULT 0` | `int NOT NULL DEFAULT 0` | README `summary`; số **trước** khi cắt |
| `findings_stored` | `integer NOT NULL DEFAULT 0` | idem | **(bổ sung)** số dòng thật trong `quality_findings` |
| `findings_truncated` | `boolean NOT NULL DEFAULT false` | idem | **(bổ sung)** |
| `steps` | `jsonb NOT NULL` | `json NOT NULL` | **(bổ sung)** `QualityStepResult[]`, ≤ 64 KiB |
| `error_code` | `varchar(64) NOT NULL DEFAULT ''` | idem | **(bổ sung)** ví dụ `CODEINTEL_RUN_CANCELLED` |
| `requested_by` | `uuid NULL` | `char(36) NULL` | **(bổ sung)** NULL = hệ thống (CR-CV-080 chạy tự động); `source=ci` NULL |
| `active_key` | `uuid NULL` UNIQUE | `char(36) NULL` UNIQUE | **(bổ sung)** bằng `repo_binding_id` khi `status IN ('queued','running')`, NULL khi kết thúc: một run đang chạy mỗi worktree (đúng quy tắc agent, CR-CV-081 2.7); cùng kỹ thuật `reindex_jobs.active_key` |
| `started_at`, `finished_at` | `timestamptz NULL` | `timestamp(6) NULL` | |
| `created_at`, `updated_at` | `timestamptz NOT NULL` | idem | `updated_at` dùng phát hiện run mồ côi |
| `version` | `bigint NOT NULL DEFAULT 1` | idem | khoá lạc quan |

Duy nhất `(tenant_id, repo_binding_id, agent_run_id)` (nạp lặp an toàn). Chỉ mục `(tenant_id, repo_binding_id, created_at)`, `(tenant_id, repo_id, head_commit)`, `(status, updated_at)` (dọn run mồ côi: `queued|running` không cập nhật quá `CODEINTEL_QUALITY_RUN_STALE_AFTER` = 60 phút → `failed`, `error_code='CODEINTEL_QUALITY_RUN_ORPHANED'`, `active_key=NULL`).

**`quality_findings`**

| Cột | Postgres | MySQL | Ghi chú |
|---|---|---|---|
| `id` | `uuid` PK | `char(36)` PK | |
| `tenant_id` | `uuid NOT NULL` | `char(36) NOT NULL` | |
| `run_id` | `uuid NOT NULL` | `char(36) NOT NULL` | trỏ `quality_runs.id`, không FK |
| `repo_id` | `uuid NOT NULL` | `char(36) NOT NULL` | để tra theo `(repo_id, fingerprint)` xuyên run (xu hướng, miễn trừ) |
| `ordinal` | `integer NOT NULL` | `int NOT NULL` | thứ tự đã sắp (2.5); khoá phân trang |
| `step_id` | `varchar(96) NOT NULL` | idem | |
| `fingerprint` | `char(32) NOT NULL` | idem | |
| `fp_version` | `smallint NOT NULL DEFAULT 1` | idem | |
| `rule_id` | `varchar(128) NOT NULL` | idem | |
| `severity` | `varchar(8) NOT NULL CHECK (severity IN ('error','warning','info'))` | idem | |
| `category` | `varchar(16) NOT NULL CHECK (category IN ('lint','typecheck','test','coverage','complexity','security','dependency','convention','architecture','ai'))` | idem | |
| `file` | `varchar(1024) NOT NULL DEFAULT ''` | idem | **không đặt chỉ mục** trên cột này (MySQL utf8mb4 1024×4 byte vượt giới hạn khoá 3072 byte của InnoDB); lọc theo tệp trong một run quét tối đa 20 000 dòng |
| `line`, `end_line`, `col`, `end_col` | `integer NOT NULL DEFAULT 0` | `int NOT NULL DEFAULT 0` | |
| `message` | `varchar(2048) NOT NULL` | idem | đã che |
| `tool` | `varchar(32) NOT NULL` | idem | |
| `tool_version` | `varchar(32) NOT NULL DEFAULT ''` | idem | |
| `fix_hint` | `varchar(512) NOT NULL DEFAULT ''` | idem | |
| `in_scope` | `boolean NOT NULL DEFAULT true` | `tinyint(1) NOT NULL DEFAULT 1` | |
| `created_at` | `timestamptz NOT NULL` | idem | |

Duy nhất `(tenant_id, run_id, ordinal)`; chỉ mục `(tenant_id, repo_id, fingerprint)`. Dung lượng ước tính (**chưa đo**): ~0,5-0,8 KiB/dòng; một run đầy 20 000 dòng ≈ 10-16 MiB; hạn mức tổng theo tenant do bảo trì (dưới).

Dọn (mở rộng công việc bảo trì CR-CV-011 2.5): xoá `quality_findings` của run quá `CODEINTEL_QUALITY_FINDINGS_RETENTION` (30 ngày) theo lô 500; xoá `quality_runs` quá 180 ngày (giữ số tổng hợp; `quality_trend_points` của CR-CV-085 giữ dữ liệu xu hướng lâu hơn); dữ liệu mồ côi (run không còn binding quá 7 ngày) như `graph_snapshots`. Khác biệt dialect: Postgres `DELETE … WHERE id IN (SELECT id … LIMIT n)`, MySQL `DELETE … ORDER BY created_at LIMIT n` (CR-CV-011 2.5).

Domain (`internal/domain`): `quality_run.go`, `quality_finding.go`, `quality_finding_limits.go`, `quality_run_transitions.go` (`CanTransition` queued→running→(succeeded|failed|cancelled)); cổng repo: `QualityRunRepository` (`Create` đặt `active_key`; vi phạm → `CODEINTEL_RUN_IN_PROGRESS`; `Get`, `ListByBinding`, `ListByRepoCommit`, `Finish(CAS version)`, `MarkOrphans`), `QualityFindingRepository` (`InsertBatch`, `ListByRun(keyset ordinal, severity?, step?)`, `CountByRun`, `ListByFingerprint(repoId, fingerprint)`). Postgres dùng `withTenantTx`, MySQL kiểm tenant ở ứng dụng, tất cả `WHERE tenant_id = ?`.

### 2.10 Nạp kết quả từ agent vào bảng (use case `IngestQualityRun`)

1. `StartQualityRun` (CR-CV-085) tạo `quality_runs` `queued` (đặt `active_key`), gọi `quality.run`, lưu `agent_run_id`.
2. `quality.progress` → cập nhật `status`, `updated_at` (không ghi từng thông báo vào DB; gộp ≤ 1 lần/2 s) và đẩy `codeIntel.quality.progress`.
3. `quality.finished` hoặc phát hiện bằng `quality.runStatus` sau nối lại → phân trang `quality.results` (limit 500, tới `nextOffset=null`), `InsertBatch` từng trang (`INSERT … ON CONFLICT (tenant_id, run_id, ordinal) DO NOTHING` ở Postgres; MySQL `ON DUPLICATE KEY UPDATE id=id`), **sau đó** `Finish` bằng CAS (`status`, các đếm, `steps`, `index_basis`, `active_key=NULL`) trong cùng giao dịch với một sự kiện outbox `orca.codeintel.quality.run_finished` (README 3.10). Findings trước, trạng thái sau: người đọc không thấy run `succeeded` với phát hiện thiếu.
4. Idempotent: nhận `quality.finished` hai lần hoặc mất kết nối giữa chừng chạy lại an toàn nhờ khoá duy nhất; ghi `findings_stored` khớp tổng của các trang đã nhận, nếu khác `Σ total_count` đã cắt thì giữ `findings_truncated=true`.
5. Run mà agent báo `interrupted` hoặc không phản hồi quá `STALE_AFTER`: `failed` + `error_code`, `active_key=NULL`.
6. Gate/UI luôn dùng `error_count/warning_count/info_count` trên run (số trước cắt) cho kết luận, `quality_findings` cho danh sách.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Fingerprint dựa trên văn bản dòng chuẩn hoá, không số dòng, không `toolVersion` | Ổn định khi dòng dịch; tránh "mới" giả sau khi nâng công cụ; không cần parser ngôn ngữ |
| `occurrence` trong khoá | Hai phát hiện giống hệt vẫn phân biệt được |
| Băm dòng nguồn nhưng không lưu | Dòng có thể chứa secret hằng |
| Chuỗi thay vì enum proto cho `severity/category/status` | Thêm giá trị không phá tương thích; README dùng chuỗi |
| Trạng thái bước suy từ parser, không chỉ mã thoát | `go vet -json` thoát 0 có phát hiện; oxlint thoát 1 phải có phát hiện |
| "Mã thoát báo phát hiện mà parser trả 0 → `format_drift`" | Không để trôi định dạng biến thành "sạch" |
| Đếm trước khi cắt lưu ở `quality_runs`/`QualityStepResult` | Cổng không "pass" nhờ cắt |
| `quality_runs.repo_id` + `repo_binding_id` | Xu hướng/miễn trừ theo repo sống sót khi worktree bị xoá |
| `active_key` UNIQUE | Một run mỗi worktree, hai dialect, không cần chỉ mục từng phần |
| Không chỉ mục `file` | Giới hạn khoá MySQL; một run chứa ≤ 20 000 dòng |
| Fixture theo `quality-mini-repo/` riêng | Không đụng fixture đồ thị của CR-CV-070 |
| Parser là hàm theo luồng, không throw | Đầu ra `go test -json` lớn; lỗi thành `failure` có cấu trúc |
| Ánh xạ `check-*` ở đây, ngữ nghĩa luật ở CR-CV-084 | Tránh trùng phạm vi |

## 4. Tiêu chí chấp nhận

- [ ] `codeintel_quality.proto` qua `buf lint` (STANDARD) và `buf breaking` (FILE); `QualityFinding` có đủ trường README 3.10 (`fingerprint, ruleId, severity, category, file, line, endLine, message, tool, toolVersion, fixHint`) cộng các trường thêm ở 2.1; tên message không va chạm với package (không dùng tên chung `Finding`, v7 mục 8 điểm 11).
- [ ] Mỗi parser ở 2.7 có test trên fixture vàng của đúng phiên bản (2.8); đầu ra chuẩn hoá khớp từng byte với `*.expected.json`.
- [ ] Bảng tính chất fingerprint (2.2.3) có test: dịch dòng (chèn 5 dòng trống phía trên), đổi thụt đầu dòng, sửa nội dung dòng, hai phát hiện giống hệt, đổi tên tệp, nâng `toolVersion`; fingerprint không chứa chuỗi số dòng (test quét khoá trước khi băm).
- [ ] Cùng đầu vào chạy hai lần cho **cùng** danh sách `fingerprint` theo cùng thứ tự (xác định).
- [ ] `file` không bao giờ tuyệt đối hoặc chứa `..`: test cho từng công cụ với đường dẫn in tương đối `cwd`, tuyệt đối, ngoài repo (module cache Go), symlink thoát; phát hiện ngoài repo có `file=""` và tăng `outsideRepoCount`.
- [ ] `message`, `fix_hint` không chứa đường dẫn tuyệt đối, `HOME`, giá trị biến môi trường nhạy cảm, mẫu token (test với fixture chèn secret giả); dòng nguồn không bao giờ xuất hiện trong bản ghi lưu hay RPC; `SourceLines` của golangci-lint bị bỏ.
- [ ] Vượt giới hạn: 6 000 phát hiện trong một bước → lưu 5 000, `truncated=true`, `total_count=6000`, các số đếm `error/warning/info` đúng trước cắt; thứ tự cắt giữ error trước; `message` 5 KiB bị cắt ở ranh giới UTF-8.
- [ ] Đầu ra bị sửa (đổi tên một khoá bắt buộc, đổi dấu phân cách, JSON cụt) cho `failed (format_drift)` ở mọi parser; oxlint thoát 1 với 0 phát hiện parse được → `format_drift`; `detail` không chứa nội dung đầu ra thô.
- [ ] `golangci-lint` 2.x (giả bằng `--version` fixture) → `env_not_ready (tool_incompatible)`; phiên bản ngoài `SUPPORTED_*` nhưng cùng major → `untested` và vẫn chạy.
- [ ] Mọi linter trong `backend-go/.golangci.yml` có dòng trong bảng severity (test đọc tệp cấu hình).
- [ ] `go test -json` với 3 test thất bại, 1 subtest, 1 panic, 1 lỗi biên dịch gói cho đúng 6 phát hiện, `anchor` theo `<pkg>/<Test>`; vitest với suite lỗi import cho `vitest/suite-failed`.
- [ ] Migration `0003` up/down chạy sạch trên Postgres và MySQL; mỗi CHECK từ chối giá trị sai (Postgres; MySQL < 8.0.16 kiểm ở domain); hai `Create` run đồng thời cùng binding: một thành công, bên kia `CODEINTEL_RUN_IN_PROGRESS`; sau `Finish` tạo được run mới.
- [ ] Nạp kết quả: gửi `quality.finished` hai lần và ngắt giữa chừng giữa các trang cho **cùng** tập dòng, `run` kết thúc một lần, không trùng `ordinal`; run `succeeded` luôn có đủ phát hiện đã nhận.
- [ ] Tenant A không đọc/ghi được dòng của tenant B qua repository (cả hai dialect) và qua SQL trực tiếp trên Postgres với role không phải superuser (RLS).
- [ ] Bảo trì xoá phát hiện > 30 ngày theo lô và run > 180 ngày; run `running` không cập nhật > 60 phút thành `failed` với `active_key=NULL`.
- [ ] Không có tên tệp `helpers/utils/common/misc`; không `max-lines` disable mới.

## 5. Kiểm thử

Chưa chạy bất kỳ test nào ở thời điểm viết CR.

| Tầng | Nội dung |
|---|---|
| Agent unit (Vitest) | `quality-parser-oxlint.test.ts`, `-tsc`, `-vitest`, `-go-vet`, `-go-test`, `-golangci`, `-buf`, `-opa`, `-orca-check.test.ts`: mỗi file một bảng ca (hợp lệ, rỗng, nhiều phát hiện, đối kháng: Unicode, CRLF, khoảng trắng trong đường dẫn, thông điệp rất dài, đầu ra cụt, khoá lạ); `quality-finding-fingerprint.test.ts` (2.2.3); `quality-repo-path-mapping.test.ts` (POSIX và Windows giả); `quality-finding-pipeline.test.ts` (cắt, sắp thứ tự, che, `in_scope`); `quality-finding-limits.test.ts` |
| Fixture/hợp đồng | `TestQualityEveryToolVersionHasFixtures`, `TestQualityParserGolden`, `TestQualityFormatDrift` (mutation của fixture), `TestQualityFixtureBudget`, `TestQualityPinnedVersionsHaveFixtures` (đọc `package.json`), `TestQualityGolangciSeverityCoversConfig` |
| Go unit | domain (`CanTransition`, giới hạn, CHECK ở domain), use case `IngestQualityRun` với agent giả (phân trang, trùng, ngắt giữa) |
| Go integration (`-tags=integration`, hai dialect) | Bộ kịch bản chung `quality_repository_contract_test.go` (mẫu `repository_contract_test.go` của CR-CV-011), RLS với role `NOSUPERUSER NOBYPASSRLS`, `active_key` đồng thời, bảo trì theo lô, hợp đồng schema qua `information_schema` |
| Proto | `buf lint`, `buf breaking`; test phản chiếu: `QualityFinding` không có trường `line_text`/`source` |
| Thủ công | Chạy bước chụp fixture lần đầu trên `quality-mini-repo/` và duyệt tay `*.expected.json`; đo thời gian parse `go test -json` 50 MiB và bộ nhớ (chưa có số đo) |

## 6. Rủi ro và điểm chưa kiểm chứng

- **Hầu hết hình dạng đầu ra là suy từ tài liệu công cụ, chưa chạy** (cột "Đã kiểm" ở 2.7): oxlint JSON, tsc 7.0.2 văn bản, vitest JSON, `go vet -json` (kênh stdout/stderr), `go test -json` (`FailedBuild`), golangci-lint JSON, buf JSON, opa JSON. Bước chụp fixture là điều kiện trước khi viết parser cuối; nếu oxlint JSON khác dự kiến thì dùng `oxlint@github` (đã có chứng cứ CI).
- **tsc 7.0.2 là bản mới viết lại**: định dạng lỗi có thể khác tsc 5.x; `--pretty false` có hỗ trợ hay không đã thấy trong `--help` nhưng hình dạng dòng chưa kiểm.
- **Fingerprint sụp khi sửa dòng**: nhiều phát hiện "mới" sau khi agent sửa dòng đang bị báo; đã chấp nhận (2.2.1). Rủi ro khác: đọc dòng nguồn từ **cây làm việc đang đổi** (agent còn sửa): fingerprint có thể lệch lần chạy lại; `workTreeChangedDuringRun` (CR-CV-081) đánh dấu kết quả không chắc.
- **Mức severity là chính sách**, nhất là `warning/error` cho golangci-lint và `buf lint`; chưa có dữ liệu "bị bỏ qua/được xử lý" (E5 của research 11) để chỉnh.
- **Kích thước bảng**: `quality_findings` có thể lớn (nhiều worktree × nhiều lượt agent); mặc định 30 ngày/20 000 dòng/run chưa đo. Chưa có chiến lược phân vùng.
- **Đường dẫn `cwd` mỗi profile** là điều kiện của ánh xạ (2.6): đổi `cwd` mà không đổi parser làm sai `file`; test theo `cwd` của catalog (CR-CV-081) bắt lỗi này.
- **Thông điệp công cụ có thể chứa secret hằng trong mã**; bộ che chỉ là mức tối thiểu (CR-CV-081 2.8).
- **`go test -json` với nhiều gói song song**: đầu ra của các test xen kẽ; gom theo `(Package, Test)`; test dùng `t.Parallel()` và `TestMain` in nhiều: chưa đo độ chính xác của vị trí trích từ `Output`.
- **MySQL**: cột/CHECK/`ON DUPLICATE KEY UPDATE` chưa chạy; tên cột `file`, `line`, `col` không phải từ khoá dành riêng nhưng chưa chạy migration để xác nhận; `varchar(1024)` + chỉ mục không áp dụng.
- **SSH/Part B**: không đổi; parser chạy trong agent Part A.
- **GitLab/GitHub**: mô hình trung lập nhà cung cấp; `oxlint@github` chỉ dùng như dạng đầu ra của công cụ, không phụ thuộc GitHub; CR-CV-086 thêm `sarif`/CI.
- **Tái dùng CR-CV-070**: CR-CV-070 đặt fixture ở `agent/src/relay/codeintel/__fixtures__/` trong khi CR-CV-001 F2 đặt mã phẳng ở `agent/src/relay/`; đường dẫn cuối do CR-CV-001 chốt (CR-CV-070 2.2 đã ghi); CR này theo cùng.

## 7. Câu hỏi mở

- **Q1.** Bảng severity mặc định (2.3) có phù hợp với chính sách "chặn" của Orca không (ví dụ `gocritic` warning, `gofmt` info)? Ai duyệt?
- **Q2.** Có cần truy vết đổi tên tệp trong fingerprint (nối bằng `git diff -M`) ở v2 không?
- **Q3.** `anchor` dùng văn bản dòng; có muốn thêm tên hàm/symbol bao quanh (phụ thuộc index, CR-CV-080) để bền hơn khi dòng được sửa nhẹ không?
- **Q4.** `oxlint@json` hay `oxlint@github` làm định dạng chính (quyết định sau khi chụp fixture)?
- **Q5.** Lưu `quality_findings` trong bảng thường hay chuyển sang lưu nén theo run khi lớn (giống Q2 của CR-CV-011)?
- **Q6.** `quality_runs.steps` ≤ 64 KiB là đủ khi chạy suite `full` trên 22 module Go (mỗi module ba bước)?
- **Q7.** Có thêm `column`/`end_column` vào bảng (đã thêm) hay bỏ bớt để tiết kiệm? Mặc định giữ.
- **Q8.** Tên migration/thứ tự `0003` phụ thuộc kế hoạch CR-CV-011/085; cần chốt khi triển khai.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (O9-O14; mục 3.10; mục 8 điểm 10, 11, 16)
- `/opt/repos/orca/docs/research/view-code/11-additions-for-quality-control.md` §3.3 (B2), §6 (E4)
- `/opt/repos/orca/docs/crs/v7/quality-signals/CR-CV-081-quality-runner-on-agent.md` (2.3, 2.7, 2.8), `CR-CV-080-agent-worktree-index-strategy-and-auto-refresh.md` (2.4, 2.5)
- `/opt/repos/orca/docs/crs/v7/quality-rollout/CR-CV-070-golden-fixtures-and-tool-contract-tests.md` (2.2-2.7), `/opt/repos/orca/docs/crs/v7/code-intel-service-foundation/CR-CV-011-code-intel-data-model-and-repositories.md` (2.1, 2.4, 2.5)
- `/opt/repos/orca/config/scripts/check-max-lines-ratchet.mjs` (`:128`, `:162`), `/opt/repos/orca/desktop/config/scripts/check-styled-scrollbars.mjs` (`formatReports`, `main`), `/opt/repos/orca/desktop/config/scripts/check-reliability-gates.mjs` (`:395-431`)
- `/opt/repos/orca/.github/workflows/pr.yml` (`:64`), `/opt/repos/orca/.github/workflows/backend-go-mcp-service.yml`
- `/opt/repos/orca/backend-go/.golangci.yml`, `/opt/repos/orca/backend-go/Makefile`, `/opt/repos/orca/backend-go/proto/buf.yaml`, `/opt/repos/orca/.oxlintrc.json`, `/opt/repos/orca/package.json` (`oxlint`, `typescript`, `vitest`)
- Lệnh chỉ-đọc đã chạy 2026-10-06: `oxlint --help`, `vitest --help`, `tsc --version`/`--help`, `golangci-lint --version`/`run --help`, `buf lint --help`/`breaking --help`, `opa test --help`, `go help vet`
- `/opt/repos/orca/guides/reference/git-compatibility.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/AGENTS.md`
