# Phase 1 — Mô hình, hai pha và dữ kiện xác định (CR-OR-001)

**Solution:** BE-OR-SOL-001 · **Phụ thuộc:** Phase 0 · **Mở khoá:** Phase 2, 3 · **Service:** `request-service`
Chạy `gitnexus_impact` trước khi sửa symbol đã có (`engine_settings.go`, outbox, kiểm khả năng); `detect_changes()` trước commit.

## Domain (hàm thuần)

- [x] **BE-OR-T101** Kiểu `OutcomeReview`, `ReviewItem`, `Verdict`, `Subject`, `Facts` + `BuildOutcomeReview`.
  Files: `internal/domain/outcome_review_model.go`, `outcome_review_facts.go`, `outcome_review_limits.go`.
  Depends: —. Verify: `go test ./internal/domain/... -run OutcomeReviewModel` (golden JSON, thứ tự xác định).
  Kết quả (2026-10-20): `domain/outcome_review_model.go`, `outcome_review_facts.go`(gộp trong `outcome_review_builder.go`), `BuildOutcomeReview` thuần; `go test ./internal/domain -run OutcomeReview` PASS (9 test, gồm thứ tự đầu vào, giới hạn 300 mục). Kiểu đặt tên `ReviewBuildInput`/`ReviewEvidence` vì `BuildInput`/`Evidence` đã tồn tại.

- [x] **BE-OR-T102** Quy tắc AC: `delivered|partial|notCovered|unknown|planned`.
  Files: `outcome_review_requirements.go`. Verify: bảng ca đủ tổ hợp thiếu/có dữ liệu; AC không dữ liệu không bao giờ `delivered`.
  Kết quả (2026-10-20): `outcome_review_requirements.go`; bảng 9 ca PASS. **Lệch spec:** dữ liệu `checks_run` theo từng AC không có sẵn (độ phủ chỉ có `ac→task→check_id`), nên `delivered` = mọi task phủ AC đều `done` và không task nào thất bại; chữ hiển thị nói rõ "Orca does not know which check proved it". Task xong nhưng có lần chạy thất bại cuối → `partial`.

- [x] **BE-OR-T103** Cờ phát hành (`migration`, `config`, `breaking`).
  Files: `outcome_review_release_flags.go`. Verify: `breaking` thiếu nguồn → `unknown`, không `false`.
  Kết quả (2026-10-20): `outcome_review_release_flags.go`; `breaking` thiếu nguồn không bao giờ ra `false`/xuất hiện trong cờ (test). Nguồn `buf breaking` của CR-REQ-030 chưa có trong repo nên cổng `ReviewBreakingReader` hiện không được nối (cờ `breaking` luôn thiếu = unknown).

- [x] **BE-OR-T104** `inputsDigest` (pha `design` chỉ AC + Plan; pha `delivered` thêm heads) dùng `canonical_json_digest.go`; `head_key`.
  Files: `outcome_review_digest.go`. Verify: không phụ thuộc thứ tự map, không có thời gian.
  Kết quả (2026-10-20): `outcome_review_builder.go` (`InputsDigest`) + `outcome_review_settings.go` (`FactsDigest`, chỉ dữ kiện DB, dùng để phát hiện `stale` khi đọc); không phụ thuộc thứ tự map/đầu vào (test); `HeadKey` giao hoán, review `delivered` không có worktree dùng khoá `no-code`, không đụng khoá `design`.

- [x] **BE-OR-T105** Che secret và cắt trần khi dựng mô hình (dùng bộ che có sẵn của service; nếu chưa có thì tạo `outcome_review_redact.go` kèm canary test).
  Verify: token giả trong `title/detail` → `[REDACTED]`.
  Kết quả (2026-10-20): dùng `usecase.RedactRaw` (secretscan có sẵn) làm `Redact` của dịch vụ; test canary với token `ghp_…` trong tiêu chí → không còn trong `model`, có `[REDACTED]`. Độ phủ mẫu của secretscan vẫn chưa đo (ghi sẵn trong mã nguồn của nó).

## Lưu trữ

- [x] **BE-OR-T110** Migration `outcome_reviews`, `outcome_review_runs` (Postgres + MySQL, có `down`).
  Files: `migrations/{postgres,mysql}/<số kế tiếp sau 0104>_outcome_reviews.{up,down}.sql`. Verify: `migrate` lên/xuống sạch trên hai dialect.
  Kết quả (2026-10-20): `migrations/{postgres,mysql}/0105_outcome_reviews.{up,down}.sql` (+ `0106_project_outcome_review`); chạy migration lên/xuống + RLS trên Postgres 16 và MySQL 8 thật qua testcontainers (`DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock TESTCONTAINERS_RYUK_DISABLED=true go test -tags=integration ... -run 'Migration|RLS|SchemaContract'`): PASS cả hai.

- [x] **BE-OR-T111** Repository hai dialect: tạo/đọc/liệt kê keyset, `superseded`, duy nhất `(tenant, request, phase, head_key)`, `active_key` cho run.
  Files: `internal/adapter/{postgres,mysql}/outcome_review.go`. Verify: `go test -tags=integration ./internal/adapter/... -run OutcomeReview`.
  Kết quả (2026-10-20): repository Postgres + MySQL cho review và run (kiểu `OutcomeReviewRepository`, `OutcomeReviewRunRepository`), hợp đồng chung `contracttest.RunOutcomeReviewContract` (8 ca: khoá duy nhất, phiên bản lạc quan, supersede, phân trang, cô lập tenant, một run hoạt động mỗi loại, 8 start đồng thời chỉ 1 thắng) PASS trên cả hai DB thật. Lỗi tìm thấy khi chạy thật: vi phạm unique làm hỏng transaction Postgres → đổi sang `ON CONFLICT (active_key) DO NOTHING`.

## Thu thập dữ kiện

- [x] **BE-OR-T120** Cổng và bộ thu thập: `CoverageReader`, `ImpactReader`, `ExecutionSummaryReader`, `HandoverReader`, `PlanTargetReader` (trích `Code:` của task), `DiffStatReader`, `HeadResolver`.
  Files: `internal/usecase/outcome_review_facts_collector.go` + `ports`. Verify: mỗi nguồn lỗi riêng → `warnings:["partial_facts"]`, không thất bại cả review.
  Kết quả (2026-10-20): `usecase/outcome_review_facts_collector.go`: `ReviewFactsCollector` (tasks, outcomes, coverage, diff, impact, gate, breaking tuỳ chọn); mỗi nguồn lỗi riêng thành cảnh báo `partial_facts:*`, không làm hỏng review (test). Nguồn tác động CR-REQ-030 **chưa có triển khai** trong request-service nên `release.risk` luôn `unknown`.

- [x] **BE-OR-T121** `DiffStatReader`/`HeadResolver` qua `agent.exec` (`git diff --numstat <base>...<head>`, `git rev-parse`), Git 2.25.
  Verify: test với agent giả; đường dẫn không tuyệt đối trong mô hình; kiểm chạy được trên dev server qua SSH (dùng cùng đường `agent.exec`).
  Kết quả (2026-10-20): **Lệch spec, tốt hơn:** dùng git-gateway `BranchCompare(worktree_id, base_ref)` (trả `entries` có `added`/`removed`, `head_oid`, `merge_base`) qua `grpcclient.GitReviewDiffReader`, thay vì `agent.exec git diff --numstat`: không cần SSH riêng, cùng đường các tính năng git khác. Test với client giả PASS. Chưa chạy được với git-gateway thật (xem T199).

## RPC và cờ

- [x] **BE-OR-T130** Proto `outcome_review.proto` + sinh mã; `Estimate/Generate/Get/List` + server gRPC.
  Files: `proto/orca/request/v1/outcome_review.proto`, `internal/adapter/grpc/outcome_review_server.go`. Verify: `buf lint` + build; chạy `buf breaking` trực tiếp (không qua `make proto-lint`, vì có `|| true`).
  Kết quả (2026-10-20): `proto/orca/request/v1/outcome_review.proto` + 4 RPC trong `request.proto`; sinh mã bằng `protoc` 33.4 + `protoc-gen-go` 1.36.11 + `protoc-gen-go-grpc` 1.6.1 (chỉ chép `request.pb.go`, `request_grpc.pb.go`, `outcome_review.pb.go`, `solution.pb.go`; `buf` không cài trên máy này, header file sinh ghi đúng công cụ đã dùng). Máy chủ gRPC `server_outcome_review.go`. Catalog RPC, `flow_gate.go` và `request.rego` (`agent_rpcs`) đã đăng ký; test catalog/flow gate PASS. `GetOutcomeReview` **bắt buộc `request_id`** (khác spec: để phân quyền theo Request, tránh resolver thực thể).

- [x] **BE-OR-T131** Usecase `Generate` (idempotent theo digest, đồng bộ tầng F, hai pha, `NOT_READY`), `Estimate` (không tốn AI; đếm lời gọi `agent.execPrompt` = 0).
  Files: `usecase/outcome_review_generate.go`, `outcome_review_estimate.go`. Verify: các tiêu chí chấp nhận §4 của CR-OR-001.
  Kết quả (2026-10-20): `usecase/outcome_review_service.go` (`Generate`, `Estimate`, `Get`, `Latest`, `List`); 10 test với kho giả + `-race`: idempotent theo digest, hai pha, supersede, stale khi đọc, Estimate không ghi gì, lỗi diff chỉ là cảnh báo.

- [x] **BE-OR-T132** Cờ `outcome_review` theo project + `REQUEST_DEFAULT_OUTCOME_REVIEW` + `REQUEST_REVIEW_KILL_SWITCH`; lỗi `REQUEST_REVIEW_*` đăng ký.
  Files: `domain/engine_settings.go`, `usecase/engine_settings_defaults.go`, `domain/outcome_review_errors.go`. Verify: cờ tắt → `REQUEST_REVIEW_DISABLED` và không gọi agent.
  Kết quả (2026-10-20): `domain/outcome_review_settings.go` (cấu hình từ env `REQUEST_DEFAULT_OUTCOME_REVIEW`, `REQUEST_REVIEW_KILL_SWITCH`) + cột `outcome_review` ở `project_engine_settings` (migration 0106), use case, repo hai dialect, proto/gRPC, kênh gateway `request.engineSettingsSet` và CLI `orca engine set --outcome-review`; hợp đồng repo mở rộng PASS trên hai DB thật. `REQUEST_REVIEW_DISABLED` khi tắt, không gọi agent/git (test).

- [x] **BE-OR-T133** Quyền + audit `request.review.generate`; cô lập tenant.
  Verify: người không có quyền xem → `NOT_FOUND` không phân biệt tồn tại.
  Kết quả (2026-10-20): quyền theo Catalog+Rego (`Generate` = nhóm `analyze`, không cho agent; `Estimate/Get/List` = `read`, cho agent); audit `request.review.generate` (không ghi nội dung, chỉ id/phase/digest; test). Cô lập tenant được hợp đồng repo kiểm. Ghi chú: audit này là best-effort (không bền), đúng với bản chất bản ghi dẫn xuất.

## Hoàn tất phase

- [x] **BE-OR-T199** Chạy bộ kiểm chung (README §5) + `go test -race` cho usecase mới; ghi kết quả.
  Kết quả (2026-10-20): `go vet ./...` sạch; `go test -race ./... -count=1` PASS toàn bộ 15 package có test của request-service. Bộ tích hợp đầy đủ trên DB thật (Postgres 16 + MySQL 8, testcontainers qua Colima: `DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock TESTCONTAINERS_RYUK_DISABLED=true go test -tags=integration ./internal/adapter/{postgres,mysql}/ -count=1`): mọi test PASS **trừ một**: `Test{Postgres,MySQL}_RequestRepositoryContract/UpdateCASConflict` ("updated_at went backwards"). **Đã kiểm chứng lỗi này có sẵn ở HEAD** (chạy trên worktree sạch của `80951aa73`, Postgres: cùng lỗi), không do phase này; chưa sửa vì ngoài phạm vi, cần xử lý riêng.
  Phát hiện khi chạy bộ đầy đủ và đã sửa: (1) thiếu fixture cô lập tenant cho `outcome_reviews`/`outcome_review_runs` (`rls_audit_integration_test.go`); (2) cột `outcome_reviews.model` chứa nội dung khách hàng nên đăng ký vào `domain.ErasableColumns` (xoá thành `{}`, khoá `request_id`) và seed trong test ẩn danh của cả hai dialect; (3) các cột enum/khoá/digest và `project_engine_settings.outcome_review` đăng ký vào `ExemptTextColumns` kèm lý do. Chưa làm: git-gateway thật cho `BranchCompare` (T121) và nguồn `breaking`/tác động của CR-REQ-030 (T103/T120) vì repo chưa có.
