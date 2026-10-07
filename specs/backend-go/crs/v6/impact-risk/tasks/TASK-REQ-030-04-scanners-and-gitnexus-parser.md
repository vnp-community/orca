# TASK-REQ-030-04: Bộ quét `migrationscan`, `contractscan`, `gitnexusparse`, `AreaResolver` và fixture từ mẫu thật

**From Solution:** [BE-REQ-SOL-030](../solutions/BE-REQ-SOL-030-impact-assessment-and-risk-scoring.md) mục 2.D
**Priority:** P1
**Service/Area:** `request-service` (mới) / adapter thuần Go (không gọi mạng), domain `AreaResolver`
**File:** `internal/adapter/migrationscan/{scan.go,sql_statements.go,scan_test.go}` (mới), `internal/adapter/contractscan/{scan.go,rules.go,scan_test.go}` (mới), `internal/adapter/gitnexusparse/{impact.go,detect_changes.go,status.go,parse_test.go}` (mới), `internal/domain/area_resolver.go` (mới), `internal/adapter/*/testdata/` (mới), và `internal/domain/area_resolver_test.go`
**Depends on:** TASK-REQ-030-02 (kiểu `Signal`, `Finding`), TASK-REQ-001-01 (module)
**Status:** [x] DONE

---

## Context

Đã đọc ngày 2026-10-06:
- GitNexus CLI (`node .gitnexus/run.cjs`): `impact [target]` với `-d/--direction`, `--depth`, `--summary-only`, `-l/--limit`, `-f/--file`, `--kind`; `detect-changes --scope compare --base-ref <ref>`; `status`. **Không có `--json`**; định dạng văn bản chưa kiểm chứng. Vì vậy parser **phải** được viết từ mẫu thật: bước đầu tiên của task là chạy từng lệnh trên máy dev và lưu đầu ra vào `testdata/`.
- `.gitnexus/gitnexus.json` là JSON (có `lastCommit`, `indexedAt`, `branch`, `stats`); `GitNexusStatus` đọc tệp này bằng `fs.readFile` (task 05), không cần parse văn bản của `status` cho tuổi index.
- Bộ quét thuần Go: nhận nội dung (từ `fs.readFile`/`git diff` do task 05 lấy) và trả `[]domain.Signal` + `[]domain.Finding`; **không** gọi relay (để test không cần dev server).
- Migration của repo: `migrations/{postgres,mysql}/NNNN_name.{up,down}.sql` (`task-service`, `infra-fleet-service`, ...). Bốn quy tắc CR 2.3 (Dữ liệu): có migration +20; thiếu `.down.sql` +50; chỉ một dialect +50; thao tác phá huỷ (`DROP`, `ALTER ... TYPE`, `TRUNCATE`, `DELETE`/`UPDATE` không `WHERE`) +60; `NOT NULL` không `DEFAULT` +30; backfill.
- Hợp đồng (CR 2.3): kênh WS thêm, xoá, đổi tên (`api-gateway/internal/adapter/wscompat/channels_*.go`), `ToolSpec` thiếu (`parity_test.go` và `tools/excluded_channels.yaml`), payload outbox đổi (`*payload*.go`); vùng nhạy cảm: `auth-service`, `tenant-service`, `credential-broker-service`, `common/tenant`, `policy/**/*.rego`, `SecretRedactor`; ingress mới (webhook, route công khai).
- `max-lines` disable mới: AGENTS.md cấm; `config/scripts/check-max-lines-ratchet.mjs` và `config/max-lines-baseline.txt` là nguồn.
- Nhận diện tên kênh: các tệp `channels_*.go` đăng ký kênh bằng chuỗi (ví dụ `"accounts.list"`); `contractscan` dùng regex trên diff, đánh `severity` thấp hơn cho kết quả suy đoán (CR 2.5).

## Việc cần làm

1. **Thu mẫu thật** (không bỏ qua): trên máy dev chạy `node .gitnexus/run.cjs impact <symbol> --direction upstream --depth 3 --summary-only` cho ≥ 3 symbol (một nhỏ như `buildExecutePrompt`, một lớn như `ResolveConnection`, một không tồn tại), `impact ... ` không có `--summary-only` cho một symbol, `detect-changes --scope compare --base-ref HEAD~3`, và `status`; lưu nguyên văn vào `internal/adapter/gitnexusparse/testdata/*.txt` kèm tệp `README.txt` ghi lệnh, ngày, phiên bản `gitnexus --version`. Nếu đầu ra có mã màu ANSI, lưu cả hai dạng.
2. `gitnexusparse.ParseImpact(out string) (ImpactSummary, error)` với `ImpactSummary{Symbol string; Risk string; DirectCallers, IndirectCallers, ExecutionFlows, Modules int; Truncated bool}`:
   - `ParseDetectChanges(out string) (ChangeSummary, error)` với `ChangeSummary{ChangedSymbols []string; AffectedFlows int; Services []string}`
   - `ParseStatus` chỉ đọc cờ "stale" và hai commit nếu có. Lỗi parse (định dạng lạ) trả `ErrUnrecognizedFormat` kèm 200 ký tự đầu, để use case ghi `impact_tool_runs.status=error` (CR 2.5). Loại ANSI trước khi parse. Hằng `SupportedGitNexusVersion` (nếu `--version` có) ghi ở `tool_version`.
3. `ImpactSummary` → `[]domain.Signal` cho chiều Phạm vi ảnh hưởng: `BLAST_DIRECT` (<5: 10; 5 đến 20: 35; 21 đến 50: 60; >50: 85, lấy **mức cao nhất** trong các symbol), `BLAST_FLOWS` (+5 mỗi luồng), `BLAST_SERVICES` (≥ 3 service +20; số service từ `Modules`/đường dẫn). Bảng điểm lấy từ `domain.DefaultRulePolicyV1()` (task 02), không hard-code lại ở đây.
4. `migrationscan.Scan(files []MigrationFile) ScanResult` với `MigrationFile{Path string; Content string; Added bool}`:
   - nhận diện: cặp `.up.sql`/`.down.sql` (thiếu `.down.sql` thì `MIGRATION_NO_DOWN`)
   - hai dialect (`/postgres/` và `/mysql/` cùng tên cơ sở; chỉ một thì `MIGRATION_ONE_DIALECT`)
   - câu lệnh phá huỷ bằng bộ tách câu lệnh đơn giản (bỏ comment `--` và `/* */`, tách theo `;` ngoài chuỗi): `DROP TABLE|COLUMN|INDEX`, `ALTER TABLE ... ALTER COLUMN ... TYPE`, `TRUNCATE`, `DELETE FROM x` không `WHERE`, `UPDATE x SET ...` không `WHERE` (`MIGRATION_DESTRUCTIVE`)
   - `ADD COLUMN ... NOT NULL` không `DEFAULT` (`MIGRATION_NOT_NULL_NO_DEFAULT`)
   - câu `UPDATE`/`INSERT ... SELECT` lớn (`MIGRATION_BACKFILL`). `ScanResult{Signals []Signal; Findings []Finding; Irreversible bool}` với `Irreversible = thiếu down || destructive` (đầu vào cho luật cứng). Chỉ `Added` (tệp thêm trong diff) mới tính (migration cũ không phạt).
5. `migrationscan.ScanPaths(paths []string) ScanResult` (dùng ở thời điểm `plan`, chỉ theo tên tệp trong `scope.create`): suy ra thiếu `.down.sql`, một dialect; **không** đọc nội dung; `basis=path`.
6. `contractscan.Scan(diff DiffInput) ScanResult` với `DiffInput{ChangedFiles []string; FileDiffs map[string]string; ExcludedChannels []string}`: (a) kênh WS: trong diff của `wscompat/channels_*.go` tìm dòng thêm/xoá chứa chuỗi tên kênh dạng `"<tên>.<tên>"` trong lời gọi đăng ký (regex bảo thủ):
   - xoá hoặc đổi tên thì `CHANNEL_REMOVED` (+60), thêm thì `CHANNEL_ADDED` (+10, và kiểm tên có trong `ExcludedChannels` hoặc `ToolSpec`, thiếu thì `TOOLSPEC_MISSING` +30)
   - (b) payload outbox: tệp khớp `*payload*.go` đổi tên trường `json:"..."` thì `OUTBOX_PAYLOAD_CHANGED` (+40)
   - (c) vùng nhạy cảm: đường dẫn khớp danh sách (`auth-service`, `tenant-service`, `credential-broker-service`, `common/tenant`, `policy/**/*.rego`, `**/secret_redactor.go`) thì `SENSITIVE_AREA` (+70, trần 100, đặt `TouchesAuthTenantCredOPA`)
   - (d) ingress mới: thêm route HTTP công khai (`mux.Handle`, `HandleFunc` trong `api-gateway`) thì `NEW_INGRESS` (+40)
   - (e) `max-lines`: diff thêm dòng chứa `eslint-disable max-lines` hoặc `oxlint-disable max-lines` hoặc dòng mới trong `config/max-lines-baseline.txt` thì `MAX_LINES_DISABLE_ADDED`
   - (f) thay đổi `proto/orca/**/*.proto`: xoá hoặc đổi số trường `message` (`PROTO_FIELD_REMOVED`) là heuristic **bổ sung** cho `buf breaking`, `severity=low`. Mỗi finding có `Path` và `Message` không chứa nội dung dòng nhạy cảm (chỉ tên kênh/tên trường).
7. `contractscan.ScanPaths(paths []string) ScanResult`: chỉ khớp đường dẫn (vùng nhạy cảm, tệp proto, `wscompat`), cho `basis=path`.
8. `domain.AreaResolver`: `Resolve(area AffectedArea) ([]string, bool)` với `AffectedArea{Name, Kind string}` (`service|module|api|schema|ui|infra`): `service` thì `backend-go/services/<tên>`:
   - `module` thì `backend-go/<tên>`
   - `api` thì `backend-go/proto/orca/<tên>`
   - `schema` thì `backend-go/services/<tên>/migrations`
   - `ui` thì `frontend/src/renderer/src/<tên>` hoặc theo tên cấu hình
   - `infra` thì `backend-go/deploy`. Tên không khớp `^[a-z0-9][a-z0-9._/-]{0,63}$` hoặc chứa `..` thì không ánh xạ được (`ok=false`, `AREA_UNRESOLVED`). Danh sách service hợp lệ lấy từ danh mục (CR-REQ-031) khi có, nếu chưa thì kiểm `go.work` đã nạp (đọc `go.work` bằng `ReadFile` ở task 05)
   - bảng ánh xạ là dữ liệu, không `switch` dài.
9. Fixture cho scanner: `testdata/migrations/` (cặp đúng, thiếu down, một dialect, `DROP COLUMN`, `UPDATE` không `WHERE`, `NOT NULL` không default, comment chứa `DROP` không phải câu lệnh), `testdata/diffs/` (diff mẫu thật rút gọn từ lịch sử repo: thêm kênh, xoá kênh, thêm `max-lines` disable).

## Kiểm thử

- `TestParseImpact_RealSamples` (mỗi mẫu thật một ca), `_ANSIStripped`, `_UnknownSymbol`, `_UnrecognizedFormat`, `_TruncatedFlag`; tương tự `ParseDetectChanges`, `ParseStatus`. Test **phải** dùng mẫu thật ở bước 1, không mẫu bịa.
- `TestImpactSignals_MaxAcrossSymbols`, `TestImpactSignals_UsesRulePolicyTable` (đổi bảng thì điểm đổi).
- `TestMigrationScan_Table` (mọi mẫu dương và âm ở bước 9), `_OnlyAddedFilesCount`, `_CommentIsNotStatement`, `_IrreversibleFlag`, `TestMigrationScanPaths_NoContentRead`.
- `TestContractScan_ChannelAddedRemovedRenamed`, `_ToolSpecMissing`, `_OutboxPayloadChanged`, `_SensitiveAreaSetsFlag`, `_NewIngress`, `_MaxLinesDisable`, `_FindingsNeverContainSourceLines`, `TestContractScanPaths_OnlyPathMatches`.
- `TestAreaResolver_Table` (service, module, api, schema, tên lạ, `..`, Unicode).
- Fuzz ngắn: `FuzzMigrationScan` và `FuzzParseImpact` không panic (chạy 30 giây cục bộ).
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/adapter/migrationscan/... ./services/request-service/internal/adapter/contractscan/... ./services/request-service/internal/adapter/gitnexusparse/... ./services/request-service/internal/domain/... -run "Scan|Parse|AreaResolver"`.

## Tiêu chí hoàn thành

- [x] `MigrationScanner` phát hiện: thiếu down, một dialect, `DROP COLUMN`, `UPDATE` không `WHERE`, `NOT NULL` không default (mẫu dương và âm).
- [x] `GitNexusOutputParser` có golden từ mẫu thật; lỗi parse trả `ErrUnrecognizedFormat`, không panic.
- [x] `ContractScanner` không đưa nội dung dòng nguồn vào `Finding`.
- [x] Bộ quét không gọi mạng và không đọc đĩa (nhận nội dung qua tham số).
- [x] Mẫu GitNexus ghi rõ ngày và phiên bản công cụ.
- [x] Không file nào tên `helpers`/`utils`/`common`/`misc`; không `max-lines` disable.

## Rủi ro và lưu ý

- Định dạng văn bản GitNexus dễ gãy khi nâng phiên bản; test mẫu thật giúp phát hiện nhưng không chặn việc nâng cấp. Phương án thay: dùng MCP của GitNexus (CR-REQ-031).
- Regex tên kênh WS là heuristic; kênh đăng ký động (vòng lặp, bảng) bị bỏ sót, kênh bị đổi tên mà không đổi chuỗi (đổi hàm xử lý) cũng không thấy.
- Bộ tách câu lệnh SQL đơn giản có thể sai với `DO $$ ... $$` (Postgres); gặp khối `$$` thì coi cả khối là một câu lệnh và đánh `MIGRATION_OPAQUE_BLOCK` (mức `warn`).
- `UPDATE` không `WHERE` có thể hợp lệ trong backfill nhỏ; vẫn đánh phá huỷ (bảo thủ), người duyệt `RiskAcceptance` được.
- Mẫu diff từ lịch sử repo có thể chứa nội dung nhạy cảm; rà soát trước khi commit fixture.
