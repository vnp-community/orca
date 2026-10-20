# TASK-REQ-045-03: Làm sạch bí mật của bộ tài liệu khi lưu và khi đọc; kích thước thật của file bị lược

**From Solution:** [BE-REQ-SOL-045](../solutions/BE-REQ-SOL-045-plan-documents-and-revision.md)
**Priority:** P1
**Service:** `request-service`
**File:** `migrations/{postgres,mysql}/0103_openspec_change_redaction.{up,down}.sql` (mới: cột `redacted_secrets`), `internal/domain/openspec_change.go` (sửa: `RedactedSecrets`, `ParseOmittedMarker`), `internal/usecase/{plan_bundle_redaction,run_proposal_generation,get_plan_bundle_file,get_plan_proposal,tasks_md_sync}.go` (mới/sửa), `internal/adapter/{postgres,mysql}/openspec_change_repository.go` (sửa), `internal/adapter/contracttest/{openspec_change_contract,expected_columns}.go` (sửa)
**Depends on:** Không
**Status:** [x] DONE (2026-10-20)

---

## Context

- `commitAndRead` lưu `bundle[rel] = string(b)` thô (`run_proposal_generation.go:353`); native và Solution đã làm sạch (`RedactDocument` trong `EncodePlanProposal`, `plan_document.go:96-103`; `RedactSecrets`, `analysis_secret_redaction.go:12`). `common/secretscan` quét cửa sổ 1 MiB; mỗi file đọc tối đa 256 KiB nên một lần quét phủ hết file.
- `request.planBundleFile` trả thẳng `change.Bundle[rel]` (`get_plan_bundle_file.go`), nên bí mật nằm trong DB được trả cho giao diện và CLI.
- `addChange` đặt `Size: int64(len(content))` (`get_plan_proposal.go:133`): với file bị lược `content` là dấu, nên `Size` sai.
- `TasksMdSyncer.restoreDrift` (`tasks_md_sync.go:298`) lấy vùng Plan từ `change.Bundle["tasks.md"]` để khôi phục khi tasks.md thật bị sửa tay: nếu bản lưu đã bị che, khôi phục ghi `[REDACTED]` vào file thật.
- Nhánh git `request/<n>-proposal` trên dev server vẫn chứa văn bản thô của agent; làm sạch chỉ bảo vệ DB, giao diện, CLI.

## Việc cần làm

1. `plan_bundle_redaction.go`: `redactBundle(files []bundleFile, budget int) (map[string]string, int)`; mỗi file: `RedactSecrets(string(b))` cộng dồn số thay; file vượt ngân sách còn lại vẫn thành `omittedMarker(b)` tính trên byte thô (kích thước thật). `commitAndRead` dùng nó và trả thêm số bí mật; `finish` đặt `change.RedactedSecrets`.
2. `domain.OpenSpecChange.RedactedSecrets int`; cột `openspec_changes.redacted_secrets INT NOT NULL DEFAULT 0` (Postgres `ALTER TABLE ... ADD COLUMN`; MySQL tương đương) trong migration `0103_openspec_change_redaction` (có `.down.sql` cả hai dialect, không backfill). Repository đọc/ghi cột; `expected_columns*.go` cập nhật.
3. `planChecksFor` thêm `SECRETS_REDACTED` (severity `info`, `Message`: "N secrets were masked in the stored copy; the file on the proposal branch is unchanged") khi `RedactedSecrets > 0`. Không đưa nội dung hay vị trí bí mật vào thông điệp.
4. `GetPlanBundleFile.Execute`: chạy `RedactSecrets` lên `Content` trước khi trả (bỏ qua file `omitted`); che luôn dòng lưu thô trước CR này.
5. `domain.ParseOmittedMarker(s) (size int64, sha256 string, ok bool)`; `addChange` dùng nó để đặt `Size` thật cho file bị lược (`omitted` giữ nguyên). Test với dấu hợp lệ, sai khuôn, số tràn.
6. `restoreDrift`: khi `change.RedactedSecrets > 0`, mốc khi `SyncDigest == ""` là `change.TasksMDDigest` (digest bản thô lúc tác giả) và **không** khôi phục từ `stored`: giữ vùng hiện có, log warn một lần (không kèm nội dung) và vẫn tăng `TasksDrift`. Khi `RedactedSecrets == 0` hành vi cũ nguyên vẹn.
7. `BundleDigest` không đổi cách tính nhưng giờ dùng bản đã làm sạch; ghi chú trong comment rằng Approval cũ (trước CR) có digest tính trên bản thô vẫn khớp vì dòng cũ không bị ghi lại.

## Kiểm thử

- `TestRedactBundle_*`: mỗi loại bí mật của `secretscan` trong `design.md`, trong vùng Plan của `tasks.md`, trong `specs/x/spec.md`; file 256 KiB; nhiều bí mật; bí mật ở đuôi file; file bị lược dấu đúng kích thước thật.
- `TestFinish_StoresRedactedBundle_AndCount`; `TestGetPlanBundleFile_RedactsLegacyRow` (dòng lưu thô); `TestPlanChecks_SecretsRedacted`.
- `TestRestoreDrift_RedactedBundle_NeverWritesMask` (khẳng định file thật không bao giờ chứa `[REDACTED]`), `TestRestoreDrift_NoRedaction_UnchangedBehaviour`.
- Hợp đồng hai dialect: cột `redacted_secrets` ghi/đọc, mặc định 0 cho dòng cũ.
- `go test ./services/request-service/internal/... -run 'Redact|Bundle|RestoreDrift|PlanChecks' -race -count=1`.

## Tiêu chí hoàn thành

- [x] Bộ tài liệu có bí mật cắm sẵn được che khi lưu; `redacted_secrets` đúng; cảnh báo `SECRETS_REDACTED` hiện. (`TestFinish_StoresRedactedBundle_AndCount`, `TestRedactBundle_MasksEveryKindInEveryDocument`, `TestPlanChecks_SecretsRedacted`)
- [x] Dòng cũ lưu thô được che khi đọc qua `GetPlanBundleFile`; không cần backfill. (`TestGetPlanBundleFile_RedactsLegacyRow`)
- [x] `Size` của file bị lược trong danh sách là kích thước thật. (`TestGetPlanProposal_OmittedFileListsItsRealSize`)
- [x] `TasksMdSyncer` không ghi dấu che vào `tasks.md` thật; hành vi cũ khi không có bí mật không đổi. (`TestRestoreDrift_*` (4 test))
- [x] Digest Approval của Request đang chờ không bị đổi bởi migration. (`TestPostgres_Migration_OpenSpecChangeRedaction`, `TestMySQL_Migration_OpenSpecChangeRedaction`: migration chỉ thêm cột, không ghi lại dòng nào; `BundleDigest` không đổi cách tính)

## Rủi ro và lưu ý

- `secretscan` có thể dương tính giả trên văn bản kỹ thuật (UUID, hash): chấp nhận, vì thay `[REDACTED]` chỉ làm mất chữ trong bản xem, không hỏng file trên nhánh; đo số dương tính giả trên bộ tài liệu pilot (chưa đo).
- Số migration `0103` (và `0102` của task 02, `0104` của task 04; `0101` là của BE-REQ-SOL-044) có thể bị CR khác giữ; kiểm lại `ls migrations/postgres | tail`.
- Chạy `gitnexus_impact` cho `commitAndRead`, `GetPlanBundleFile`, `restoreDrift`.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.

## Ghi chú triển khai

**Số migration.** `0102_openspec_change_redaction` (không phải `0103` như đề bài): theo chỉ dẫn của orchestrator, 0102 và các số sau thuộc CR-045 và task 02 chưa làm; **task 02 (cần sửa CHECK/ENUM `origin` của `plan_auto_jobs`) phải dùng `0104`**, không phải `0102`, vì golang-migrate bỏ qua số nhỏ hơn bản đã áp dụng. Task 04 dùng `0103`.

**Blast radius (grep).** `commitAndRead` chỉ có một người gọi (`RunProposalGeneration.finish`); `GetPlanBundleFile.Execute` được gọi bởi `server_plan.go`; `restoreDrift` chỉ trong `TasksMdSyncer.syncTasks`; `OpenSpecChange` có thêm trường `RedactedSecrets` (mọi nơi dựng literal đều dùng tên trường nên không gãy); cột mới đi qua hai adapter `openspec_change_repository.go` (cột chọn, quét, upsert).

**Tệp đã đổi.** Mới: `migrations/{postgres,mysql}/0102_openspec_change_redaction.{up,down}.sql`, `internal/usecase/plan_bundle_redaction.go` (`redactBundle`, `isOmittedMarker`), `internal/adapter/contracttest/expected_columns_plan_documents.go` (bảng `openspec_changes` chưa nằm trong kiểm tra cột nào; thêm đủ 25 cột), `internal/domain/openspec_change_omitted_test.go`, test `plan_bundle_redaction_test.go`, `tasks_md_sync_redaction_test.go`, `{postgres,mysql}/plan_documents_migration_integration_test.go`. Sửa: `domain/openspec_change.go` (`RedactedSecrets`, `ParseOmittedMarker`), `usecase/{run_proposal_generation,get_plan_bundle_file,get_plan_proposal,tasks_md_sync}.go`, `adapter/{postgres,mysql}/openspec_change_repository.go`, `contracttest/{openspec_change_contract,expected_columns}.go`. `get_plan_proposal.go` là file của agent CR-044: chỉ sửa `addChange`, `planChecksFor` và import.

**Hành vi.**
- `redactBundle` che từng file bằng `RedactSecrets` (mỗi file đọc tối đa 256 KiB nên một lần quét phủ hết), rồi tính ngân sách trên văn bản đã che: file vượt phần còn lại thành `omittedMarker` của byte thô (kích thước và hash thật) và không tính bí mật của nó. Nhánh `proposal` trên dev server giữ văn bản thô.
- `GetPlanBundleFile` che lại khi đọc (dòng cũ lưu thô), bỏ qua file `omitted`.
- `addChange` dùng `ParseOmittedMarker` đặt `Size` thật; dấu sai khuôn rơi về `len(content)` và vẫn `Omitted`.
- `SECRETS_REDACTED` (info) khi `RedactedSecrets > 0`, thông điệp đúng chữ của đề bài, không chứa nội dung hay vị trí.
- `restoreDrift` trả `driftNone|driftRestored|driftKept`. Khi `RedactedSecrets > 0` không bao giờ ghi lại `stored`: đã đồng bộ thì so với `SyncDigest` (digest vùng Orca đã ghi); chưa đồng bộ thì so digest **cả file** `md` với `TasksMDDigest` (digest cả file thô lúc tác giả; đề bài ghi "digest bản thô" mà `TasksMDDigest` là của cả file chứ không phải của vùng nên phải so cả file). Lệch thì giữ vùng hiện có, `TasksDrift()` + log warn một lần (sau đó `recordInSync` đặt `SyncDigest` mới). Hệ quả đã biết: trước lần đồng bộ đầu, sửa tay phần văn xuôi ngoài vùng Plan cũng bị đếm là drift khi bản lưu đã che (chỉ ghi số đo và log, không đổi file). Khi `RedactedSecrets == 0` hành vi cũ nguyên vẹn (các test cũ vẫn xanh).

**Lệnh đã chạy.**
- `go test ./internal/usecase/ -run 'Redact|Bundle|RestoreDrift|PlanChecks|Finish_|GetPlanProposal_Omitted' -race -count=1` xanh; `go test ./... -race -count=1` toàn module xanh.
- Integration (colima): `go test -tags=integration -p 1 ./internal/adapter/postgres/ ./internal/adapter/mysql/ -run 'OpenSpec|PlanRevision|SchemaContract|Security|Erasable' -count=1` xanh trên Postgres 16 và MySQL 8: hợp đồng `redacted_secrets` ghi/đọc/thay đổi, mặc định 0 cho dòng cũ, migration 0102 xuôi, ngược, xuôi lại với một dòng có sẵn.

**Chưa kiểm chứng.** Đo dương tính giả của `secretscan` trên bộ tài liệu pilot (đề bài ghi "chưa đo"); chạy thật với agent thật trên stack.
