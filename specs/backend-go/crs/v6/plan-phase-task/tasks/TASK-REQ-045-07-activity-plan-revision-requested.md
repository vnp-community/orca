# TASK-REQ-045-07: Activity `plan.revision_requested` và sửa nhánh `analysis.revision_requested` cho khỏi bắt nhầm lượt Plan

**From Solution:** [BE-REQ-SOL-045](../solutions/BE-REQ-SOL-045-plan-documents-and-revision.md)
**Priority:** P1
**Service:** `request-service`
**File:** `internal/domain/request_activity.go` (sửa: hằng, `AllActivityKinds`, mô tả theo kind nếu có), `internal/adapter/{postgres,mysql}/request_activity.go` (sửa), `internal/adapter/contracttest/{activity_revision_contract,activity_contract}.go` (sửa), `internal/adapter/contracttest/activity_revision_sql_test.go` (sửa), test domain của danh sách kind
**Depends on:** TASK-REQ-045-02
**Status:** [ ] TODO

---

## Context

- CR-REQ-041 đã thêm nhánh UNION `analysis.revision_requested` đọc `request.analysis_runs` có `feedback` khác rỗng và không bắt đầu bằng `clarification:` (Postgres `adapter/postgres/request_activity.go:39-40`; MySQL dòng 38), tác nhân `user`, `subject_type='solution'`, **không chọn cột `feedback`** (C9).
- Nhánh đó không lọc `kind`. Lượt Plan cũng có `feedback` (cột chung), nên ngay khi Plan có phản hồi, dòng Activity sẽ ghi "Requested changes to the analysis" cho lượt Plan (G12).
- `domain/request_activity.go`: `ActivityAnalysisRevisionRequested`, `AllActivityKinds` là tập đóng cho bộ lọc.

## Việc cần làm

1. Test hợp đồng đỏ trước: Request có một lượt Solution có phản hồi và một lượt Plan có phản hồi → feed phải có đúng một `analysis.revision_requested` và đúng một `plan.revision_requested` (hiện là hai dòng `analysis.*`).
2. `request_activity.go` (domain): `const ActivityPlanRevisionRequested = "plan.revision_requested"` với chú thích "ghi việc xảy ra, không bao giờ ghi nội dung"; thêm vào `AllActivityKinds`.
3. Hai dialect: nhánh `analysis.revision_requested` thêm `AND kind <> 'plan'`; nhánh mới `plan.revision_requested`: `FROM analysis_runs WHERE tenant_id AND request_id AND kind = 'plan' AND feedback IS NOT NULL AND feedback <> '' AND feedback NOT LIKE 'clarification:%'`, cột `id:revision`, `started_at`, `'user'`, `actor_id`, `subject_type = 'plan'`, `subject_id = ''`, `summary` cố định "Requested changes to the plan", **không chọn `feedback`**. Giữ cùng thứ tự cột A..D, N với các nhánh khác (`ActivityRow`).
4. `BuildActivityEntry`/`activity_presentation` (backend nếu có hàm tóm tắt theo kind): thêm kind mới với tóm tắt cố định, mức `info`.
5. Bộ lọc `kinds` của `ListRequestActivity` chấp nhận kind mới; `ErrActivityFilterInvalid` vẫn từ chối kind lạ.
6. Chỉ mục `analysis_runs_by_request` (`0100`) đã phục vụ truy vấn; kiểm EXPLAIN nhánh mới không quét toàn bảng.

## Kiểm thử

- Hợp đồng hai dialect: ca ở việc 1; lượt Plan có phản hồi `clarification:` không sinh dòng; nội dung phản hồi không xuất hiện ở bất kỳ cột nào của dòng (chuỗi cắm sẵn không nằm trong JSON feed).
- `activity_revision_sql_test.go`: kiểm chuỗi SQL không chứa `feedback` ở danh sách cột chọn của nhánh mới.
- Domain: `AllActivityKinds` chứa kind mới; bộ lọc theo kind.
- `go test ./services/request-service/internal/... -run 'Activity' -race -count=1`; testcontainers hai dialect.

## Tiêu chí hoàn thành

- [ ] Lượt Plan có phản hồi hiện đúng một dòng `plan.revision_requested`, không còn dòng `analysis.revision_requested` cho lượt Plan.
- [ ] Dòng không chứa nội dung phản hồi (C9).
- [ ] Bộ lọc theo kind nhận kind mới; Request cũ không đổi.
- [ ] EXPLAIN dùng chỉ mục.

## Rủi ro và lưu ý

- Gateway/frontend phải chịu được kind lạ (đã có từ CR-REQ-037); frontend hiển thị ở FE-REQ-TASK-045-07.
- Chạy `gitnexus_impact` cho `AllActivityKinds` và hàm dựng UNION.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
