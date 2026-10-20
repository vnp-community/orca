# TASK-REQ-045-03: Làm sạch bí mật của bộ tài liệu khi lưu và khi đọc; kích thước thật của file bị lược

**From Solution:** [BE-REQ-SOL-045](../solutions/BE-REQ-SOL-045-plan-documents-and-revision.md)
**Priority:** P1
**Service:** `request-service`
**File:** `migrations/{postgres,mysql}/0103_openspec_change_redaction.{up,down}.sql` (mới: cột `redacted_secrets`), `internal/domain/openspec_change.go` (sửa: `RedactedSecrets`, `ParseOmittedMarker`), `internal/usecase/{plan_bundle_redaction,run_proposal_generation,get_plan_bundle_file,get_plan_proposal,tasks_md_sync}.go` (mới/sửa), `internal/adapter/{postgres,mysql}/openspec_change_repository.go` (sửa), `internal/adapter/contracttest/{openspec_change_contract,expected_columns}.go` (sửa)
**Depends on:** Không
**Status:** [ ] TODO

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

- [ ] Bộ tài liệu có bí mật cắm sẵn được che khi lưu; `redacted_secrets` đúng; cảnh báo `SECRETS_REDACTED` hiện.
- [ ] Dòng cũ lưu thô được che khi đọc qua `GetPlanBundleFile`; không cần backfill.
- [ ] `Size` của file bị lược trong danh sách là kích thước thật.
- [ ] `TasksMdSyncer` không ghi dấu che vào `tasks.md` thật; hành vi cũ khi không có bí mật không đổi.
- [ ] Digest Approval của Request đang chờ không bị đổi bởi migration.

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
