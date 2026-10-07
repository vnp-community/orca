# AG-CV-SOL-082: Parser đầu ra công cụ, `fingerprint` v1, pipeline chuẩn hoá và phát hiện trôi định dạng

> ✅ **Đã triển khai.** Ngày triển khai 2026-10-07. Đã hoàn thành toàn bộ các task 082-01 đến 082-09, kiểm thử tự động xác nhận qua vitest, đạt 100% tiêu chí chấp nhận.

**CR:** [CR-CV-082](../../../../../../docs/crs/v7/quality-signals/CR-CV-082-quality-finding-model-and-parsers.md) (phần agent: 2.2 đến 2.8). Phần proto/bảng/nạp (2.1, 2.9, 2.10) thuộc `BE-CV-SOL-082-quality-run-storage-and-ingest`.
**Khu vực:** `agent/src/relay/`. **Feature:** `quality-signals`. Task AG-CV-TASK-082-01 đến 09.
**TDD/Spec:** [TDD-AG-01](../../../../tdd/v5/01-architecture.md), [TDD-AG-06](../../../../tdd/v5/06-tool-handlers.md), [api/gaps-and-findings.md](../../../../api/gaps-and-findings.md).

## 1. Hợp đồng áp dụng

| Mục ([`CONTRACT-codeintel-agent-rpc.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md)) | Áp vào |
|---|---|
| §5.5 `QualityFinding` (camelCase, `fingerprint` 32 hex, `fpVersion`, `ruleId`, `severity`, `category`, vị trí 1-based, `message ≤ 2 KiB` đã che, `tool`, `toolVersion`, `fixHint`, `stepId`, `inScope`); `QualityStepResult`; ngân sách 5 000/bước, 20 000/run, cắt `error` trước; fingerprint tính ở agent: `sha256("v1"␀tool␀ruleId␀file␀anchor␀normMessage␀occurrence)[0:32]`, `anchor` chỉ băm, không ra RPC | tasks 02-04 |
| §5.3 trạng thái bước/run; `failureKind ∈ ""|format_drift|output_too_large|exit_unexpected|parser_error|env` | task 04, 09 |
| PQ-26 không gian tên `ruleId` (`<tool>/<rule>`, regex `^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`) | tasks 05-08 |
| PQ-06 (`QualityFinding` tách khỏi `Finding`), §10 CI không chạy test agent | tasks 01, 09 |
| §9.4 secret không vào kết quả; `SourceLines`/`snippet` của công cụ bị bỏ | tasks 02, 04, 07 |
| Dải phiên bản (CR-070 cơ chế `MANIFEST.json`, `SUPPORTED_*`, `verified/untested/incompatible/format_drift`) | task 09 |
| PQ-21, §8.3 | trích |

## 2. Lệch giữa CR và hợp đồng

| # | CR-CV-082 | Hợp đồng | Solution theo |
|---|---|---|---|
| 1 | Tên proto `snake_case` (`fix_hint`, `in_scope`) | §5.5 JSON `camelCase` | camelCase ở dây agent |
| 2 | Fingerprint dựng từ `anchor` là dòng nguồn | Hợp đồng giống; nhưng **CR-091** cần fingerprint bí mật không dẫn xuất từ giá trị, CR-084 dùng công thức riêng `sha256(ruleId+file+matchedText)` | Hợp đồng là công thức chung. Pipeline nhận `anchorOverride?: string \| null` trên `RawQualityFinding` (mới; hợp đồng chưa nêu): `null`/`""` = không anchor (bí mật, lỗ hổng gói), chuỗi = dùng nguyên; CR-084/091 dùng cơ chế này; câu hỏi mở 1 |
| 3 | `parser` chạy trong executor (CR-081) | — | Registry khoá `<tool>@<format>`; executor gọi qua `StepParser` (AG-CV-SOL-081-A) |
| 4 | Tên file `quality-parser-<tool>.ts` | CR-084/091 dùng `quality-parse-*` | Thống nhất `quality-parser-<tool>.ts`; solution 091 đổi theo |
| 5 | `CODEINTEL_ENV_NOT_READY (tool_incompatible)` khi major lạ | §3.2 có `reason: tool_incompatible` | Theo |
| 6 | `quality_runs.steps` chứa `QualityStepResult` | §5.5 `view=steps` có `profileId`, `failureKind`, `envReason`, `definitionHash`, `outsideScopeCount` | Theo hợp đồng; `skipReason` (đề xuất ở SOL-081-A) |
| 7 | `oxlint@json` hay `@github` chưa chốt (Q4) | — | Task 01 chốt bằng fixture; mặc định `@json`, `@github` là dự phòng |

## 3. Phụ thuộc chéo khu vực

| Đối ứng | Quan hệ |
|---|---|
| `AG-CV-SOL-081-quality-runner-core` / `-profile-catalog-and-preflight` | Cung cấp `PlannedStep`, tệp đầu ra, `cwd` mỗi profile, `redactor`, `argv` mẫu dùng chung với `MANIFEST.json` (một nguồn sự thật) |
| `AG-CV-SOL-070-golden-fixtures-and-parsers` | Cơ chế `MANIFEST.json`, ngân sách 20 KiB/tệp, 300 KiB/thư mục phiên bản, script chụp; solution này mở rộng, không tạo cơ chế song song |
| `AG-CV-SOL-083/084/091` | Dùng `quality-finding-pipeline.ts`, `anchorOverride`, parser `orca-check` |
| `BE-CV-SOL-082-quality-run-storage-and-ingest` | Nhận `QualityFinding`, `QualityStepResult`; dùng số đếm **trước cắt** cho cổng |
| `BE-CV-SOL-085-…` | Dùng `fpVersion` (khác phiên bản = "không so sánh được") |
| Thứ tự: `081 → 082-AG → 083, 084, 091` |

## 4. Re-verify (đã đọc, 2026-10-06)

Đã đọc `config/scripts/check-max-lines-ratchet.mjs` (dòng 120-130, 158-165: `::error::New max-lines bypass not allowed: <entry>`, `::error::Stale max-lines baseline entry (prune it): <entry>`), `desktop/config/scripts/check-styled-scrollbars.mjs`, `check-reliability-gates.mjs` (`main`), `agent/package.json` (`yaml`, `jsonc-parser`, `vitest ^4.1.5`, `oxlint ^1.71.0`, `typescript ^7.0.2`), `.oxlintrc.json`, `agent/src/shared/git-cquoted-path.ts` (tên, qua CR-037). Phiên bản công cụ máy khảo sát lấy từ CR (oxlint 1.71.0, tsc 7.0.2, vitest 4.1.5, go 1.26.0, golangci-lint 1.62.2, buf 1.72.0, opa 1.19.1); `golangci-lint`, `buf`, `opa`, `go` có trên máy (`which`).

| Định dạng | Trạng thái |
|---|---|
| `orca-check` (3 script) | **Đã đọc mã** |
| oxlint JSON/github, tsc 7.0.2 text, vitest JSON, `go vet -json`, `go test -json`, golangci JSON, buf JSON, opa JSON | **Suy từ tài liệu, chưa chạy**: task 01 chụp trước khi viết parser |

Correction relative to CR: CR-082 2.7 ghi `FailedBuild` của `go test -json` "từ Go 1.24" là hiểu biết của người soạn — xác nhận khi chụp fixture (task 01). `go vet -json` đi stdout hay stderr chưa biết: parser đọc cả hai tệp.

## 5. Giải pháp

### 5.1 Cây file (mới)

```
quality-parser-types.ts                   task 02  QualityParserInput/Output, RawQualityFinding, QualityParser
quality-repo-path-mapping.ts              task 02
quality-finding-fingerprint.ts            task 03
quality-finding-pipeline.ts, quality-finding-limits.ts   task 04
quality-parser-oxlint.ts, -tsc.ts, -vitest.ts            task 05
quality-parser-go-vet.ts, -go-test.ts                    task 06
quality-parser-golangci.ts (+ -severity.ts), -buf.ts, -opa.ts   task 07
quality-parser-orca-check.ts                              task 08
quality-tool-version-support.ts, quality-parser-registry.ts      task 09
__fixtures__/quality-mini-repo/ ; __fixtures__/quality/<tool>/<version>/   task 01
agent/scripts/capture-quality-fixtures.mjs                task 01
```

### 5.2 `RawQualityFinding` và pipeline

```ts
export type RawQualityFinding = {
  tool: string; ruleId: string; severity: 'error'|'warning'|'info'
  category: 'lint'|'typecheck'|'test'|'coverage'|'complexity'|'security'|'dependency'|'convention'|'architecture'|'ai'
  printedPath: string | null; line: number; endLine: number; column: number; endColumn: number
  message: string; fixHint?: string
  anchorOverride?: string | null     // (mới) null/"": không anchor; undefined: đọc dòng nguồn
  testAnchor?: string                // go/vitest/opa: tên đầy đủ của test
}
```
Pipeline (task 04), thứ tự: ánh xạ đường dẫn → che `message`/`fixHint` → `inScope` (theo `scopeFiles`, so khớp không phân biệt hoa/thường trên macOS/Windows) → `anchor` (`anchorOverride ?? testAnchor ?? chuẩn hoá(readSourceLine)`) → `occurrence` (nhóm theo `(tool, ruleId, file, anchor, normMessage)` sắp `(line, column)`) → `fingerprint` v1 → sắp xếp và cắt (`error` trước `warning` trước `info`; `inScope` trước; theo `file,line,column,ruleId`) → số đếm **trước cắt**. Suy trạng thái bước: `cancelled|timeout` giữ; `failure.kind=env` → `env_not_ready`; `failure.*` khác → `failed`; mã thoát ngoài `exit.ok ∪ exit.findings` mà không có phát hiện → `failed (exit_unexpected)`; có phát hiện → `findings`; còn lại `passed`. **Chống mất thầm lặng**: mã thoát ∈ `exit.findings` nhưng parser trả 0 phát hiện và không `failure` → `format_drift` (không áp cho `go vet -json`).

### 5.3 Fingerprint v1 (task 03), đúng hợp đồng

`key = "v1" + "\0" + tool + "\0" + ruleId + "\0" + file + "\0" + anchor + "\0" + normMessage + "\0" + occurrence`; `fingerprint = hex(sha256(key)).slice(0, 32)`; `fpVersion = 1`. `anchor` = dòng nguồn: `normalize('NFC')`, trim, gộp khoảng trắng, cắt 512 ký tự; không đọc được (tệp > 2 MiB, nhị phân, đã xoá, symlink ra ngoài) → `""`. `normMessage`: (1) đường dẫn tuyệt đối repo → `<repo>`, `HOME` → `~`, tmp → `<tmp>`; (2) `…:<dòng>(:<cột>)?`, `(<dòng>,<cột>)`, `line <n>` → `<loc>`; (3) thời lượng/ISO-8601 bỏ; (4) gộp khoảng trắng; (5) cắt 1 024. Không chứa số dòng, không `toolVersion`. Dòng nguồn chỉ **băm**, không lưu, không RPC.

### 5.4 Bảng severity/category và ruleId (task 05-08)

| Công cụ | severity | category | `ruleId` |
|---|---|---|---|
| oxlint | `error→error`, `warning→warning`, `advice→info` | lint | `oxlint/<plugin>/<rule>` từ `plugin(rule)`; lạ `oxlint/unknown` |
| tsc | error/warning | typecheck | `tsc/TS<nnnn>` |
| vitest | failed → error | test | `vitest/test-failed`, `vitest/suite-failed` |
| go vet | error | lint (build lỗi → typecheck) | `go-vet/<analyzer>`, `go-vet/unknown`, `go-vet/build-failed` |
| go test | error | test (build lỗi → typecheck) | `go-test/test-failed|panic|timeout|build-failed` |
| golangci-lint | theo bảng linter (12 linter của `.golangci.yml`; linter lạ → warning) | lint (`typecheck` → typecheck) | `golangci/<linter>[/<rule>]` |
| buf lint / breaking | warning / error | lint / architecture | `buf/<TYPE>`, `buf-breaking/<TYPE>` |
| opa test | error | test (biên dịch → typecheck) | `opa/test-failed`, `opa/compile-error` |
| `check-*` | error (`stale-baseline` warning) | convention | `orca-check/<script>/<kind>` |

## 6. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| 1 | Parser là hàm theo luồng, không ném | `go test -json` hàng chục MiB; lỗi thành `failure` |
| 2 | Đếm trước cắt đi vào kết quả | Cổng không "pass" nhờ cắt |
| 3 | `anchorOverride` | CR-084/091 cần anchor khác dòng nguồn |
| 4 | Không `toolVersion` trong khoá | Nâng công cụ không tạo "mới" giả |
| 5 | Mã thoát báo phát hiện mà parser 0 → `format_drift` | Không để trôi định dạng thành "sạch" |
| 6 | `SourceLines`, `snippet` bị bỏ | Có thể chứa secret hằng |
| 7 | Fixture dựa trên `quality-mini-repo/` riêng | Không đụng fixture đồ thị của CR-070 |

## 7. Tiêu chí chấp nhận

- [x] Mỗi parser ở 5.4 có test trên fixture vàng đúng phiên bản; đầu ra khớp từng byte với `*.expected.json`.
- [x] Fingerprint: chèn 5 dòng phía trên giữ; đổi thụt đầu dòng giữ; sửa nội dung dòng đổi; hai phát hiện giống hệt khác nhau nhờ `occurrence`; đổi tên tệp đổi; nâng `toolVersion` giữ; khoá không chứa số dòng; cùng đầu vào hai lần → cùng danh sách và thứ tự.
- [x] `file` không bao giờ tuyệt đối hoặc chứa `..`; ngoài repo → `file:""` + `outsideRepoCount`.
- [x] `message`/`fixHint` không chứa đường dẫn tuyệt đối, `HOME`, giá trị env nhạy cảm, token; dòng nguồn không bao giờ ra khỏi tiến trình.
- [x] 6 000 phát hiện/bước → lưu 5 000, `truncated`, `totalCount=6000`, đếm đúng trước cắt; `message` 5 KiB cắt ở ranh giới UTF-8 ≤ 2 048 byte.
- [x] Đầu ra bị sửa (đổi khoá bắt buộc, JSON cụt) → `failed (format_drift)` ở mọi parser; oxlint thoát 1 với 0 phát hiện → `format_drift`; `detail` không chứa đầu ra thô.
- [x] golangci-lint 2.x → `env_not_ready (tool_incompatible)`; cùng major khác patch → `untested` vẫn chạy.
- [x] Mọi linter trong `backend-go/.golangci.yml` có dòng trong bảng severity (test đọc tệp).
- [x] `go test -json` 3 test fail, 1 subtest, 1 panic, 1 lỗi biên dịch → đúng 6 phát hiện.

## 8. Kiểm thử

Test theo task (`quality-parser-*.test.ts`, `quality-finding-fingerprint.test.ts`, `quality-repo-path-mapping.test.ts`, `quality-finding-pipeline.test.ts`, `quality-finding-limits.test.ts`, `quality-tool-version-support.test.ts`, `quality-fixture-contract.test.ts`). Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-parser-types.test.ts src/relay/quality-finding-fingerprint.test.ts src/relay/quality-finding-pipeline.test.ts src/relay/quality-parser-oxlint.test.ts src/relay/quality-parser-tsc.test.ts src/relay/quality-parser-vitest.test.ts src/relay/quality-parser-go-vet.test.ts src/relay/quality-parser-go-test.test.ts src/relay/quality-parser-golangci.test.ts src/relay/quality-parser-buf.test.ts src/relay/quality-parser-opa.test.ts src/relay/quality-parser-orca-check.test.ts src/relay/quality-fixture-contract.test.ts`. Hợp đồng fixture chạy trong job CI `code-intel-contract` (CR-070).

## 9. Rủi ro và chưa kiểm chứng

- Hầu hết định dạng chưa chạy (mục 4): task 01 là điều kiện trước. tsc 7.0.2 là bản viết lại, dòng lỗi có thể khác.
- Fingerprint sụp khi sửa chính dòng (chấp nhận, CR 2.2.1); đọc dòng nguồn từ cây đang đổi có thể lệch (`workTreeChangedDuringRun`).
- Mức severity là chính sách (Q1 CR), chưa có dữ liệu bỏ qua.
- `golangci-lint` v1.62.2 (go1.22.2) không chạy với `go.work` 1.26: chụp fixture trên module mẫu `go 1.22`.
- Windows/macOS: so khớp hoa/thường, `\` chưa kiểm chứng.
- Đầu ra `go test` song song xen kẽ: gom theo `(Package, Test)`, vị trí trích có thể sai.

## 10. Câu hỏi mở

1. Thêm `anchorOverride` (khái niệm nội bộ) có cần ghi vào hợp đồng? Mặc định: không (không ra dây), chỉ ghi chú.
2. Bảng severity mặc định có phù hợp chính sách chặn của Orca?
3. `oxlint@json` hay `@github`: quyết định theo fixture task 01.
