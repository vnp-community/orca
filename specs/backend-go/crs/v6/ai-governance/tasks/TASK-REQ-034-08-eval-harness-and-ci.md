# TASK-REQ-034-08: Eval harness (xác định + `eval_live`), baseline và cổng CI

**From Solution:** BE-REQ-SOL-034 (mục 5, CR 2.5)
**Priority:** P1 (cổng GA cần eval đạt baseline)
**Service:** `request-service`, `backend-go/ci`
**File:** `backend-go/services/request-service/evals/golden/<step>/*.json` (mới), `.../evals/replay/<step>/*.json` (mới), `.../evals/baseline.json` (mới), `.../evals/metrics.go` (mới), `.../evals/deterministic_test.go` (mới), `.../evals/live_test.go` (mới, build tag `eval_live`), `.../evals/results/.gitkeep` (mới), `.github/workflows/backend-go-request-service.yml` (sửa), `.../evals/README.md` **không tạo** (cấu trúc mô tả ở đây)
**Depends on:** TASK-REQ-034-03 (registry, script bump), TASK-REQ-034-06 (grounding), BE-REQ-SOL-005, 007, 012 (schema đầu ra và bộ kiểm)
**Status:** [ ] TODO

---

## Context

- CR 2.5: hai tầng. (a) **Xác định, chạy mọi PR**: kiểm bộ kiểm schema, trình trích JSON, `GroundingChecker`, registry bằng đầu ra đã ghi sẵn (`replay/*.json`), không gọi LLM, không cần khoá, dưới 2 phút. (b) **Mô hình thật**, build tag `eval_live`, gọi dev server thật, trần chi phí `EVAL_MAX_COST_USD_EST`, ghi `evals/results/<ngày>.json`.
- Chưa có mẫu vàng thật nào (không có dữ liệu Request thật; CR ghi là việc phải làm sớm). Task này dựng **khung** và 30 mẫu tổng hợp mỗi bước chủ chốt ở mức tối thiểu để CI chạy; Request thật ẩn danh thêm sau (qua `secretscan`, BE-REQ-SOL-035) là việc dữ liệu.
- Chọn bộ Go (không Promptfoo) để khỏi thêm Node vào CI backend (CR 2.5, `openspec-and-ai-tooling-integration.md` mục 3.2). Không dùng LLM làm giám khảo ở v1.
- Workflow `backend-go-request-service.yml` do TASK-REQ-001-06 tạo (dựa mẫu `.github/workflows/backend-go-task-service.yml`); task này thêm bước, không viết lại.
- Chỉ số và ngưỡng (3 điểm phần trăm) là đề xuất chưa hiệu chỉnh; 30 mẫu có sai số lớn.

## Việc cần làm

1. Cấu trúc thư mục dưới `evals/`: `golden/<step>/<id>.json` với `{ "id": "...", "input": {...}, "expect": {...} }`. Theo bước:
   - `classify`: `expect{type, accept[], size}` (`accept` là các loại chấp nhận được);
   - `solution`: `expect{min_options, max_options, must_mention[]}` (`must_mention` phải có trong `affected_areas`);
   - `plan`: `expect{covers_ac: ["AC-1",...], required_labels_by_type, acyclic: true}` (CR-REQ-012/014);
   - `taskspec`: `expect{required_fields[], check_has_command: true}`.
   `replay/<step>/<id>.json` giữ `{ "raw_output": "...", "prompt_id": "...", "prompt_version": "..." }` (đầu ra đã ghi từ một lần chạy thật hoặc viết tay, **có đánh dấu `synthetic: true`**).
2. `metrics.go`: kiểu `Metrics{SchemaPass, ClassifyAccuracy, ClassifyMacroF1, ACCoverage, GroundingRatio, AvgAttempts, AvgTokens float64}` và hàm `Compare(base, cur Metrics, tolerance float64) []Regression`; `ClassifyMacroF1` tính theo 11 loại (loại không có mẫu bỏ khỏi macro). Thuần hàm, có test.
3. `deterministic_test.go` (`-run Deterministic`): với mỗi mẫu `replay`: (a) chạy **bộ trích JSON và kiểm schema thật** của SOL-005/007/012 (import hàm từ `internal/usecase`/`domain`, không sao chép logic); (b) kiểm `expect` của `golden` cùng id; (c) chạy `GroundingChecker` với `FakeRepoReader` dựng từ `golden.input.repo_fixture` (danh sách tệp giả); (d) kiểm `PromptRegistry.Render` cho `prompt_id@version` của mẫu (không rò biến). Tổng hợp `Metrics`, so với `baseline.json` ở `tolerance=0.03`; tụt quá ngưỡng ⇒ `t.Fatalf` liệt kê chỉ số. Cố ý có một "mẫu hỏng" (`replay` JSON sai schema) mong đợi **bị từ chối**: bộ kiểm bị làm hỏng thì test đỏ.
4. `baseline.json`: `{ "version": 1, "generated_at": "...", "synthetic": true, "metrics": {...}, "tolerance": 0.03 }`; lệnh cập nhật `go test ./services/request-service/evals/... -run Deterministic -update-baseline` (cờ `-update-baseline` ghi file, **chỉ chạy tay**, không bao giờ trong CI). Script bump (task 03) yêu cầu `baseline.json` đổi khi prompt đổi.
5. `live_test.go` (`//go:build eval_live`): đọc `EVAL_PROJECT_ID`, `EVAL_MAX_COST_USD_EST` (mặc định 5), gọi `AIGateway.Run` thật qua dev server; theo dõi `cost_usd_est` cộng dồn từ ledger của chính run (dùng ngân sách ảo `tenant eval`), dừng khi chạm trần (`t.Skip` phần còn lại, ghi `truncated:true`); ghi `evals/results/<YYYY-MM-DD>.json` (đã `.gitignore` trừ `.gitkeep`); in so sánh với baseline nhưng **không** làm CI đỏ (chạy tay hoặc hằng đêm, ngoài PR).
6. CI: thêm vào job `request-service` bước `go test ./services/request-service/evals/... -run Deterministic -count=1` với `timeout-minutes` cho cả job 10; đo thời gian bước, mục tiêu dưới 2 phút; thêm bước `bash backend-go/ci/check-prompt-version-bump.sh` (task 03). Job `eval-live` riêng `workflow_dispatch` + lịch hằng đêm, **tắt mặc định** (cần secret dev server; ghi "chưa kiểm chứng").
7. Dữ liệu: `golden` không chứa dữ liệu khách; test `TestGoldenHasNoSecrets` quét mọi file bằng `secretscan.Scan` (TASK-REQ-035-01) và đỏ nếu có phát hiện; mẫu ẩn danh từ Request thật cần `secretscan.Redact` rồi duyệt tay trước khi commit.
8. Quy ước: không `helpers`/`utils`; tên file theo nội dung (`metrics.go`, `deterministic_test.go`); không `max-lines` disable (tách `metrics_f1.go` nếu dài).

## Kiểm thử

- `metrics_test.go`: `Compare` đỏ khi `SchemaPass` tụt 0,04 và xanh ở 0,02; `ClassifyMacroF1` golden nhỏ (ma trận nhầm lẫn tay: 3 loại); loại không có mẫu bị bỏ.
- `deterministic_test.go`: chạy trên bộ mẫu thật của task; thêm `TestDeterministic_FailsWhenSchemaValidatorBroken`: dùng bộ kiểm giả luôn "pass" và mong đợi test của mẫu hỏng **bắt được** (kiểm khẳng định rằng mẫu hỏng bị từ chối bởi bộ kiểm thật, và rằng bộ kiểm giả làm test đỏ).
- `TestGoldenHasNoSecrets`; `TestBaselineParseable`; `TestEveryReplayHasGolden` (mỗi `replay/<step>/<id>` có `golden` cùng id và ngược lại).
- Lệnh: `cd backend-go && go test ./services/request-service/evals/... -run Deterministic -count=1`; live (tay): `EVAL_PROJECT_ID=... EVAL_MAX_COST_USD_EST=5 go test -tags eval_live ./services/request-service/evals/... -run Live -count=1 -v`.

## Tiêu chí hoàn thành

- [ ] Eval xác định chạy trong PR dưới 2 phút và thất bại khi bộ kiểm schema bị làm hỏng cố ý.
- [ ] `check-prompt-version-bump.sh` và `baseline.json` được CI kiểm khi prompt đổi.
- [ ] Mỗi bước có tối thiểu 30 mẫu (đánh dấu `synthetic` khi chưa từ Request thật).
- [ ] Không bí mật trong `golden`/`replay` (test quét).
- [ ] `eval_live` có trần chi phí, không chạy trong PR.

## Ví dụ tham khảo

Mẫu `golden/classify/c-001.json` và `replay/classify/c-001.json`:

```json
{"id": "c-001", "input": {"title": "Lỗi 500 khi lưu cấu hình", "body": "Stack trace ...", "source_provider": "manual"},
 "expect": {"type": "bug", "accept": ["bug", "hotfix"], "size": "S"}}
```

```json
{"synthetic": true, "prompt_id": "classify", "prompt_version": "1.0.0",
 "raw_output": "{\"type\":\"bug\",\"size\":\"S\",\"confidence\":0.92,\"reason\":\"...\"}"}
```

`baseline.json` mẫu: `{"version": 1, "synthetic": true, "tolerance": 0.03, "metrics": {"SchemaPass": 1.0, "ClassifyAccuracy": 0.9}}`. Cờ `-update-baseline` chỉ chạy tay, không bao giờ trong CI.

## Rủi ro và lưu ý

- Baseline dựng từ mẫu tổng hợp phản ánh ít về chất lượng thật; cổng GA (CR-REQ-025) cần `eval_live` đạt baseline trên mẫu thật, việc chưa làm.
- 30 mẫu cho sai số lớn; ngưỡng 3 điểm có thể báo giả; xem lại sau khi có số liệu ledger.
- Mẫu từ Request thật có thể chứa dữ liệu khách: phải ẩn danh, duyệt tay (BE-REQ-SOL-035).
- Chạy `eval_live` phụ thuộc dev server và khoá nhà cung cấp; không tự động hoá trong PR.
