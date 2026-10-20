---
cr_id: CR-OR-001
solution_id: BE-OR-SOL-001
product: orca
service: request-service
layer: backend
language: go
status: draft
priority: P0
effort: 5–7 ngày
depends_on: v6 CR-REQ-027 (độ phủ AC), CR-REQ-030 (tác động), CR-REQ-037/038 (tóm tắt thực thi)
created: 2026-10-20
---

# BE-OR-SOL-001 — Mô hình Outcome Review, hai pha và dữ kiện xác định

## 1. Phân tích

| Nhu cầu | Có sẵn | Thiếu |
|---|---|---|
| Độ phủ tiêu chí | `GetRequestCoverage` (`artifact.proto`): `ac_id → task_id → check_id`, `uncovered_ac_ids` | Quy tắc chuyển thành `delivered/partial/notCovered/unknown` |
| Rủi ro thực tế | `ImpactAssessmentSummary` (`impact.proto`), thời điểm "thực tế sau chạy" | Đọc theo Request, ánh xạ `unknown` |
| Thực thi | `domain/execution_summary.go`, `run_stats.go`, `request_activity.go` | Gom thành dữ kiện |
| Bàn giao OpenSpec | `openspec_bundle.go`, handover trong `orca request show` | Đọc dạng dữ liệu |
| Mục tiêu Plan chạm | `tasks.md` có `Code: modify/create \`path\`` (`codeTargetRe` trong `openspec_bundle.go`) | Trích danh sách tệp dự kiến cho pha `design` |
| Thống kê diff | không có | `git diff --numstat` qua agent trên dev server |
| Bản ghi review | không có (`artifact_kind.go` là tài liệu, không phù hợp) | Bảng, mô hình, vòng đời hai pha |

ĐỌC TRƯỚC: `usecase` thực thi (`StartExecution`, `EvaluateExecution`) để biết thế nào là "Execution xong"; `task_run_outcome.go` (`checks_run`); `engine_settings.go` và `engine_settings_defaults.go`; `approval_authorization.go`; `outbox_subjects.go`; `ai_budget.go`; `dev_server_capability.go`.

## 2. Thiết kế

### 2.1 Hai pha và vòng đời

```
design  (head_key="design")  ──Execution xong──▶  delivered (head_key=<heads>)
   └─ superseded (giữ dữ liệu, hình design tái dùng)
```

- `phase=design`: tạo được khi Request ở `awaiting_plan_approval`, `executing`, `completed`; **không** có thống kê diff, cờ phát hành đo được, quyết định.
- `phase=delivered`: khi Execution xong. Điều kiện "xong" = `status=completed` **hoặc** mọi task của Plan ở trạng thái xong (chốt sau khi đọc `EvaluateExecution`; nếu hai mốc không tách được thì chỉ dùng `completed`).
- `state`: `draft|ready|stale|approved|superseded`. `stale` tính khi đọc từ dữ kiện trong DB; phát hiện commit mới qua `Estimate` (một `git rev-parse`).
- Không đổi `RequestStatus`. Request trước `awaiting_plan_approval` hoặc `cancelled` → `REQUEST_REVIEW_NOT_READY`.

### 2.2 Domain (hàm thuần)

`internal/domain/`:
- `outcome_review_model.go`: kiểu `OutcomeReview`, `ReviewItem`, `Verdict`, `Subject`; `BuildOutcomeReview(facts Facts, ctx *Context) (OutcomeReview, error)` thuần; sắp xếp `(section, severity desc, key)`.
- `outcome_review_facts.go`: `Facts` (requirements, release, execution, handover, diffStats, planTargets).
- `outcome_review_requirements.go`: bảng quy tắc AC: `delivered` = task xong ∧ ≥1 `checks_run` thoát 0; `partial` = task xong thiếu kiểm tra hoặc có task lỗi; `notCovered` ∈ `uncovered_ac_ids`; `unknown` khi không đọc được. Pha `design`: mọi AC `planned`.
- `outcome_review_release_flags.go`: `migration` nếu chạm `migrations/`; `config` nếu chạm tệp cấu hình triển khai khai báo (`deploy/`, `*.env.example`, `docker-compose*`); `breaking` từ `buf breaking` của CR-REQ-030 nếu có, ngược lại `unknown`.
- `outcome_review_digest.go`: `inputsDigest` dùng `canonical_json_digest.go`, **không** có thời gian; ở pha `design` chỉ gồm AC và Plan.
- `outcome_review_limits.go`: trần mô hình 256 KiB, `items ≤ 300`, `title ≤ 160`, `detail ≤ 400`; cắt có `truncated`.

### 2.3 Thu thập dữ kiện (usecase)

`usecase/outcome_review_facts_collector.go` với các cổng nhỏ: `CoverageReader`, `ImpactReader`, `ExecutionSummaryReader`, `HandoverReader`, `PlanTargetReader`, `DiffStatReader` (chạy `git diff --numstat <base>...<head>` qua `agent.exec` trên dev server; Git 2.25 an toàn), `HeadResolver` (`git rev-parse`). Mỗi nguồn lỗi riêng → mục thiếu + `warnings:["partial_facts"]`, không thất bại cả review.

"Head" của Request (Q1 của CR-OR-001): danh sách `heads[] = {branch, commit, taskId}` từ các worktree task; `head_key = sha256(sorted(branch@commit))` hoặc `"design"`. ĐỌC TRƯỚC cách `task-service` lưu nhánh/worktree task.

### 2.4 Lưu trữ

Migration (số kế tiếp sau 0104; cả `postgres` và `mysql`, có `down`):
- `outcome_reviews(id, tenant_id, request_id, phase, head_key, head_commit, base_commit, inputs_digest, state, model, kg_snapshot, created_by, created_at, updated_at, version)`; duy nhất `(tenant_id, request_id, phase, head_key)`.
- `outcome_review_runs(id, tenant_id, review_id, kind, status, scope, agent_run_ref, cost_estimate, cost_actual, error_code, active_key, started_at, finished_at)`; `active_key` duy nhất `(tenant, request, kind)` khi đang chạy.
- Bảng quyết định/artifact/hồ sơ ở các solution sau.
Adapter `adapter/{postgres,mysql}/outcome_review.go` với `dbcapability`; `model` là JSON (JSONB / JSON).

### 2.5 RPC và cờ

- Proto `outcome_review.proto` (package `orca.request.v1`): `EstimateOutcomeReview`, `GenerateOutcomeReview(request_id, phase?, allow_code_only?)`, `GetOutcomeReview`, `ListOutcomeReviews` (keyset). Message tiền tố `OutcomeReview*`; không khai báo RPC chưa có message.
- `Estimate` không tốn AI: đọc DB, `rev-parse`, hồ sơ khả năng dev server; trả `{stale, kgAvailable, contextCached, diagramsCached[], estimate, lastActualCost?}`.
- `Generate` idempotent theo `inputsDigest`; tầng F dựng **đồng bộ**; context/draw run chạy nền (SOL-002, SOL-003); tiến độ qua sự kiện.
- Cờ: thêm `outcome_review ∈ {off, manual}` vào cài đặt theo project (cùng mẫu `engine_settings.go`), mặc định triển khai `REQUEST_DEFAULT_OUTCOME_REVIEW` (đề xuất `off`), công tắc `REQUEST_REVIEW_KILL_SWITCH`.
- Quyền: đọc theo quyền xem Request; sinh theo quyền chạy agent trên project (tên hành động OPA chốt sau khi đọc `approval_authorization.go`); audit `request.review.generate`.

### 2.6 Cấu trúc test

Golden JSON từ fixture Request (native bug, OpenSpec change_request, hai pha); bảng quy tắc AC đủ tổ hợp thiếu/có dữ liệu; digest không phụ thuộc thứ tự map; hai dialect; đồng thời (`-race`).

## 3. Rủi ro

- "Head" nhiều nhánh sai → thống kê diff sai (Q1). Mốc "Execution xong" chưa rõ. Nguồn `buf breaking` chỉ có khi CR-REQ-030 đã làm.
- Dữ liệu v6 là đề xuất ở vài chỗ; hình dạng thật có thể khác.
