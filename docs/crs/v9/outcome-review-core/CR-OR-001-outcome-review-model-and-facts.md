# CR-OR-001 — Mô hình Outcome Review và dữ kiện xác định (tầng F)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-OR-001 |
| **Tên** | Bảng `outcome_reviews` / `outcome_review_runs`, hàm dựng mô hình thuần, bộ thu thập dữ kiện xác định (độ phủ AC, tác động thực tế, tóm tắt thực thi, handover, thống kê diff), RPC `EstimateOutcomeReview`, `GenerateOutcomeReview`, `GetOutcomeReview`, `ListOutcomeReviews`, cờ `outcome_review` |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-20 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | v6 CR-REQ-027 (độ phủ AC), CR-REQ-030 (đánh giá tác động), CR-REQ-037/038 (tóm tắt thực thi); tuỳ chọn v7 CR-CV-085 (cổng chất lượng) |
| **Mở khoá** | CR-OR-002, 011, 020, 022 |
| **Tác động** | `backend-go/services/request-service` (`internal/domain/outcome_review_*.go`, `internal/usecase/`, `internal/adapter/{postgres,mysql}/`, `migrations/{postgres,mysql}/`), `backend-go/proto/orca/request/v1/outcome_review.proto`, `api-gateway` (kênh `request.review.*`), `internal/domain/engine_settings.go` (cờ) |

---

## 1. Bối cảnh và vấn đề

Đã đọc code ngày 2026-10-20:

1. Dữ liệu kết quả của một Request có sẵn nhưng **rải rác**: độ phủ `ac_id → task_id → check_id` và `uncovered_ac_ids` (`artifact.proto`, `GetRequestCoverage`); đánh giá tác động với `level`, `dimensions[]`, `digest`, ba thời điểm gồm "thực tế sau chạy" (`impact.proto`, v6 R2); tóm tắt thực thi và activity (`domain/execution_summary.go`, `request_activity.go`); Handover của engine OpenSpec (worktree task, nhánh proposal, knowledge proposal; `openspec_bundle.go`).
2. **Không có bản ghi nào gắn với commit** để người duyệt xem lại, so sánh hai lần, hay ghi quyết định.
3. `RequestStatus` không có trạng thái review; `completed` là cuối (`request_status.go`). Thêm trạng thái ngoài phạm vi (README v9 D1).
4. Mô hình artifact hiện có (`artifact_kind.go`) là 8 loại **tài liệu** có JSON Schema và trần 32–256 KiB; không phù hợp để chứa review (mô hình thay đổi theo commit, kèm blob hình).
5. **Chưa đọc**: các usecase thực thi (`StartExecution`, `EvaluateExecution`), cách Request gom nhánh của các task thành một "head" (task chạy trên worktree `<repo>-<nhánh>` riêng, handoff 2026-10-17 mục 3), `engine_settings.go`, `approval_authorization.go`, `ai_budget.go`.

Vấn đề: người duyệt phải tự ghép các nguồn trên để trả lời "yêu cầu đã giao chưa, rủi ro phát hành là gì". CR này tạo bản ghi và dữ kiện xác định; ngữ cảnh KG và diff hệ thống ở CR-OR-002.

## 2. Giải pháp đề xuất

### 2.1 Thực thể và vòng đời

`OutcomeReview` duy nhất cho `(tenant, request_id, head_key)`; `head_key` là `head_commit` hoặc, khi Request có nhiều nhánh task, digest của danh sách `<nhánh>@<commit>` đã sắp xếp (Q1). Trạng thái:

| `state` | Ý nghĩa |
|---|---|
| `draft` | Mới tạo, tầng F xong, context run đang chạy hoặc chưa chạy |
| `ready` | Có ít nhất tầng F; `warnings[]` nói thiếu gì |
| `stale` | Đầu vào hiện tại khác `inputs_digest` đã lưu (đổi AC, đổi nhánh/commit, đánh giá tác động mới) |
| `approved` | Người có quyền đã duyệt (CR-OR-022); chỉ đặt qua `DecideOutcomeReviewItem` |
| `superseded` | Có review mới hơn cho cùng Request |

`stale` là kết luận **tính khi đọc** từ dữ kiện có sẵn trong DB (bộ AC, trạng thái task, digest tác động); phát hiện commit mới cần `EstimateOutcomeReview` (một lệnh `git rev-parse` qua agent, không tốn AI). Tạo review mới cho cùng Request đặt bản cũ thành `superseded`; bản cũ vẫn đọc được.

Review **không đổi `RequestStatus`**. Có **hai pha** (README D17):

| `phase` | Khi nào tạo được | Nội dung |
|---|---|---|
| `design` | Request có Plan: `awaiting_plan_approval`, `executing`, `completed` (kể cả khi Execution chưa xong); `head_key = "design"` | Chỉ phần *Design*: AC/yêu cầu áp dụng (trạng thái "Planned", chưa có kiểm tra), các tệp mà Plan dự định chạm (đọc từ `Code:` của task, xác định, không đọc diff), và ngữ cảnh KG (CR-OR-002 chế độ `design`). Không có thống kê diff, cờ phát hành đo được, hay quyết định |
| `delivered` | Execution xong (Request `completed`, hoặc mọi task của Plan đã xong) | Thêm tầng F đầy đủ (độ phủ AC đo được, tác động thực tế, thống kê diff, cờ phát hành) và phần *After code*; quyết định và duyệt chỉ có ở pha này |

Khi Execution xong, `GenerateOutcomeReview` tạo review `delivered` mới, đặt review `design` thành `superseded` nhưng **không** bỏ dữ liệu của nó: ngữ cảnh KG và hình `design` được tái dùng (CR-OR-002 2.5, CR-OR-011). Trạng thái trước `awaiting_plan_approval` hoặc `cancelled` → `REQUEST_REVIEW_NOT_READY`; Request `executing` mà Execution chưa xong chỉ tạo được review `design` (tab hiện phần After code là "available when execution completes").

### 2.2 Tầng F: nguồn dữ kiện

| Dữ kiện | Nguồn | Quy tắc |
|---|---|---|
| Yêu cầu (mức AC) | `GetRequestCoverage` + kết quả task (`task_run_outcome.go`: `checks_run`) | `delivered`: AC có task xong và ít nhất một kiểm tra thoát 0; `partial`: có task xong nhưng thiếu kiểm tra hoặc có task lỗi; `notCovered`: nằm trong `uncovered_ac_ids`; `unknown`: không đọc được dữ liệu. Không suy ra "Covered" cho thứ không được đo |
| Rủi ro phát hành | Đánh giá tác động thời điểm "thực tế" (`level`, `top_reasons`, `index_age_seconds`) | Thiếu → `risk: unknown`, không bao giờ `low` (v6 R4) |
| Cờ phát hành | Danh sách tệp đổi | `migration` nếu chạm `migrations/`; `config` nếu chạm tệp cấu hình triển khai đã khai báo; `breaking` lấy từ kết quả `buf breaking` của CR-REQ-030 nếu có, nếu không `unknown` |
| Thực thi | `execution_summary.go`, `run_stats.go` | Số task xong/tổng, thời gian, chi phí AI, số lần thử lại, các lần dừng (drift, backlog) |
| Bàn giao (OpenSpec) | Handover của Request | Nhánh proposal, trạng thái archive, ChangeSet đề xuất, worktree chưa gộp |
| Thống kê diff | `git diff --numstat <base>...<head>` chạy qua agent trên dev server | Số tệp, dòng thêm/xoá, nhóm theo thư mục cấp một; lệnh theo baseline Git 2.25, không dùng tuỳ chọn mới |
| Cổng chất lượng (tuỳ chọn) | v7 CR-CV-085 khi bật | `quality.gate = absent` khi v7 không có; không dựng song song |

Mọi nguồn lỗi từng phần → mục thiếu kèm `warnings[]` (`partial_facts`), không thất bại cả review.

### 2.3 Mô hình và dựng

`domain/outcome_review_model.go`: hàm thuần `BuildOutcomeReview(facts Facts, ctx *Context) (OutcomeReview, error)`; sắp xếp xác định `(section, severity desc, key)`; `inputsDigest = sha256(canonical JSON)` dùng cùng cách chuẩn hoá của `canonical_json_digest.go`, **không** có thời gian. Hình dạng JSON theo README mục 3.2–3.3. Điều chỉnh hợp đồng: `subject` thêm `heads[]` (`{branch, commit, taskId?}`) và `headCommit` là `head_key` khi có nhiều nhánh (Q1).

Trần: mô hình ≤ 256 KiB; `items` ≤ 300; `title` ≤ 160, `detail` ≤ 400 ký tự; vượt → cắt có `truncated` và `warnings: ["truncated:<section>"]`. Bộ che secret chạy trên mọi chuỗi tự do trước khi lưu.

### 2.4 RPC

- `EstimateOutcomeReview(request_id)` → `{ stale, kgAvailable, contextCached, diagramsCached[], estimate: { contextRun: "skip|run", drawRun: "skip|run(n)" }, costRange? }`. **Không tốn AI**: chỉ đọc DB, một `rev-parse`, hỏi khả năng dev server (đã có hồ sơ khả năng của agent).
- `GenerateOutcomeReview(request_id, phase?, allow_code_only?, mode?)` (`phase` mặc định suy từ trạng thái: `design` nếu Execution chưa xong, `delivered` nếu xong) → tạo hoặc trả review hiện có cho cùng `inputsDigest` (idempotent). Tầng F dựng **đồng bộ**; context run (CR-OR-002) và draw run (CR-OR-010) chạy nền, tiến độ qua push `request.review.progress`.
- `GetOutcomeReview(review_id | request_id)`, `ListOutcomeReviews(request_id)` (phân trang keyset, mới nhất trước).

### 2.5 Cờ và quyền

Cờ theo project `outcome_review = off|manual` đặt cạnh cài đặt engine (cùng mẫu `engine_settings.go`, `REQUEST_DEFAULT_OUTCOME_REVIEW` cho mặc định triển khai); tắt → `REQUEST_REVIEW_DISABLED`, không gọi agent. Quyền đọc theo quyền xem Request; quyền sinh theo quyền chạy agent trên project (tên hành động OPA chốt khi đọc `approval_authorization.go`; ghi audit `request.review.generate`).

### 2.6 Cấu trúc file

Backend: `domain/outcome_review_model.go`, `outcome_review_facts.go`, `outcome_review_requirements.go` (quy tắc AC), `outcome_review_release_flags.go`, `usecase/outcome_review_generate.go`, `usecase/outcome_review_estimate.go`, `adapter/{postgres,mysql}/outcome_review.go`, `adapter/grpc/outcome_review_server.go`. Không `helpers/utils`; không `max-lines` disable.

## 3. Quyết định thiết kế

1. **Bản ghi riêng, không nhét vào `artifact_kind`**: review đổi theo commit, mang blob, có trần khác.
2. **Không đổi máy trạng thái ở MVP**; chấp nhận rằng "đã review" chưa chặn `completed`.
3. **Hàm thuần**: tầng F kiểm thử bằng fixture, cùng đầu vào cho cùng digest.
4. **Estimate không tốn AI** để người dùng thấy chi phí và cache trước khi bấm.
5. **`unknown` là trạng thái thật** ở yêu cầu và rủi ro.

## 4. Tiêu chí chấp nhận

- [ ] Hai lần `GenerateOutcomeReview` cùng đầu vào cho cùng `inputsDigest` và cùng `reviewId`; lần hai không gọi agent.
- [ ] Cùng đầu vào, hai dialect, hai tiến trình → cùng `inputsDigest`.
- [ ] AC nằm trong `uncovered_ac_ids` luôn ra `notCovered`; AC không có dữ liệu ra `unknown`, không bao giờ `delivered`.
- [ ] Thiếu đánh giá tác động → `risk: unknown`; thiếu `buf breaking` → `breaking` không được báo `false`.
- [ ] Mô hình không có đường dẫn tuyệt đối, hostname dev server, hay chuỗi khớp mẫu secret.
- [ ] Request trước `awaiting_plan_approval` hoặc `cancelled` → `REQUEST_REVIEW_NOT_READY`; cờ tắt → `REQUEST_REVIEW_DISABLED` và không có lời gọi agent.
- [ ] Request `executing` (Execution chưa xong): chỉ tạo được review `phase=design`; không có thống kê diff, cờ phát hành đo được, hay `needsDecision`.
- [ ] Execution xong → review `delivered` mới, review `design` thành `superseded`; `design.*` và hình `design` được tái dùng, không gọi lại KG nếu còn trong hạn.
- [ ] Review mới cho cùng Request đặt bản cũ `superseded`; bản cũ vẫn đọc được.
- [ ] `EstimateOutcomeReview` không tạo lượt chạy AI nào (kiểm bằng đếm lời gọi `agent.execPrompt`).

## 5. Kiểm thử

- **Unit (Go)**: bảng quy tắc AC (mọi tổ hợp thiếu/có dữ liệu), sắp xếp xác định, digest không phụ thuộc thứ tự map, cắt trần, che secret, ánh xạ cờ phát hành.
- **Repository (hai dialect)**: tạo/đọc/phân trang, `superseded`, ràng buộc duy nhất `(tenant, request, head_key)`.
- **Usecase**: idempotent, từng nguồn dữ kiện lỗi riêng (partial), cờ tắt, trạng thái Request không hợp lệ.
- **Hợp đồng**: golden JSON từ fixture Request (native và OpenSpec).

## 6. Rủi ro và điểm chưa kiểm chứng

- "Head" của một Request gồm nhiều nhánh task chưa rõ cách gom (Q1); sai điểm này làm thống kê diff sai.
- Mức "partial" của AC phụ thuộc cách task ghi `checks_run`; chưa đọc `task_run_outcome.go`.
- Nguồn `buf breaking` chỉ có khi CR-REQ-030 đã triển khai; nếu chưa, cờ `breaking` luôn `unknown`.
- Dữ liệu v6 là đề xuất ở vài chỗ; hình dạng thật có thể khác.
- Chưa đo kích thước mô hình thật; trần 256 KiB là phỏng đoán.

## 7. Câu hỏi mở

- **Q1.** "Head" của Request: nhánh tích hợp duy nhất hay danh sách nhánh task? Mặc định đề xuất: danh sách, gộp thống kê.
- **Q2.** Review có nên tạo tự động khi Request vào `completed` (P2, `auto`)?
- **Q3.** Có đặt `approved` ở đây hay để CR-OR-022 sở hữu hoàn toàn (đề xuất: 022).

## 8. Tham chiếu

- [README v9](../README.md) mục 1–3
- `backend-go/services/request-service/internal/domain/request_status.go`, `artifact_kind.go`, `execution_summary.go`, `canonical_json_digest.go`, `openspec_bundle.go`
- `backend-go/proto/orca/request/v1/artifact.proto`, `impact.proto`
- [v6 impact-risk](../../v6/impact-risk/README.md) (R2, R4, R5)
- [handoff 2026-10-17](../../../handoff/2026-10-17-request-flow-and-qa-fixes.md)
- [AGENTS.md](../../../../AGENTS.md), [STYLEGUIDE](../../../../guides/STYLEGUIDE.md)
