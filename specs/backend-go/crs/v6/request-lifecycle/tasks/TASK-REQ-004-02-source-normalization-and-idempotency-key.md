# TASK-REQ-004-02: Chuẩn hoá nguồn, title, body và tính khoá idempotency

**From Solution:** BE-REQ-SOL-004
**Priority:** P0
**Service:** `request-service`
**File:** `internal/domain/request_source_normalization.go`, `internal/domain/request_source_normalization_test.go`, `internal/domain/request_intake_errors.go` (mới)
**Depends on:** TASK-REQ-002-02
**Status:** [ ] TODO

---

## Context

CR-REQ-004 mục 2.2 và 2.3. Mẫu tham khảo (không sao chép): `task-service/internal/domain/task_source.go` (`NewTaskSource` chỉ trim). `SourceRef`, `SourceProvider` có từ TASK-REQ-002-02. Bảng `request_idempotency` có khoá `(tenant_id, source_provider, source_site, source_ref)` với `source_ref <> ''` (TASK-REQ-002-01).

## Việc cần làm

1. `NormalizeSourceRef(ref SourceRef) (SourceRef, error)`:
   - Jira: `site` trim, `url.Parse`, scheme và host chữ thường, bỏ `/` cuối (nếu `site` không phải URL thì chỉ trim và giữ nguyên, vì có thể là workspace id); `ref` trim và `strings.ToUpper`.
   - Linear: như Jira cho `ref`; `site` trim.
   - GitHub, GitLab: `ref` trim; tách tại `#`: phần trước chữ thường, `#<số>` giữ nguyên, số phải là số nguyên dương (`REQUEST_SOURCE_REF_INVALID`).
   - `manual`, `mcp`: `ref` phải rỗng ở nguồn; `webhook`: `ref` bắt buộc, `site` (tên nguồn) bắt buộc.
   - `ref` rỗng với provider khác `manual`, `mcp`: `REQUEST_SOURCE_REF_REQUIRED`.
2. `NormalizeTitle(s string) (string, error)`: trim, đếm `utf8.RuneCountInString`, rỗng thì `REQUEST_TITLE_REQUIRED`, trên 500 thì `REQUEST_TITLE_TOO_LONG`. `NormalizeBody(s string) (string, error)`: trên 100000 rune thì `REQUEST_BODY_TOO_LARGE`.
3. `IdempotencyKey`: `type IdempotencyKey struct { Provider SourceProvider; Site, Ref string }` và `func BuildIdempotencyKey(ref SourceRef, reporterID, clientRequestID string) (key IdempotencyKey, ok bool, err error)`: provider có nguồn thì key từ `ref`; `manual`/`mcp` thì `Site = "user:" + reporterID`, `Ref = clientRequestID`, và `ok=false` khi `clientRequestID` rỗng (không claim).
4. `request_intake_errors.go`: `REQUEST_PROJECT_REQUIRED`, `REQUEST_REPORTER_REQUIRED`, `REQUEST_SOURCE_PROVIDER_INVALID`, `REQUEST_SOURCE_REF_REQUIRED`, `REQUEST_SOURCE_REF_INVALID`, `REQUEST_TITLE_TOO_LONG`, `REQUEST_BODY_TOO_LARGE` (`InvalidArgument`); `REQUEST_SOURCE_NOT_FOUND` (`NotFound`); `REQUEST_SOURCE_FETCH_FAILED` (`Internal`).
5. Không I/O; không gọi mạng ở domain.

## Kiểm thử

Bảng test (tên): `TestNormalizeSourceRef_Jira` (`eng-1` thành `ENG-1`; `HTTPS://Acme.Atlassian.NET/` thành `https://acme.atlassian.net`; site rỗng giữ rỗng), `TestNormalizeSourceRef_GitHub` (`Owner/Repo#12` thành `owner/repo#12`; thiếu `#` hoặc `#0` hoặc `#abc` lỗi), `TestNormalizeSourceRef_GitLabSubgroups` (`Group/Sub/Repo#9`), `TestNormalizeSourceRef_Webhook`, `TestNormalizeTitle` (khoảng trắng, 500 rune đa byte, 501), `TestNormalizeBody` (100000 và 100001), `TestBuildIdempotencyKey` (jira, manual có/không `client_request_id`, mcp, webhook), `TestIdempotencyKey_TwoJiraSitesDiffer`.
Lệnh: `go test ./services/request-service/internal/domain/... -run "Normaliz|Idempotency"`.

## Tiêu chí hoàn thành

- [ ] `eng-1` và `ENG-1` cho cùng khoá; hai site Jira khác nhau cho hai khoá.
- [ ] Giới hạn title 500 và body 100000 tính theo rune.
- [ ] `manual`/`mcp` không có `client_request_id` thì `ok=false`.
- [ ] Mọi mã lỗi ở bước 4 có constructor và test ánh xạ gRPC.

## Rủi ro và lưu ý

- Chuẩn hoá `ref` GitLab nhiều cấp chưa kiểm với dữ liệu thật.
- `site` Jira có thể là workspace id (không phải URL): đừng ép `url.Parse` thất bại thành lỗi.
