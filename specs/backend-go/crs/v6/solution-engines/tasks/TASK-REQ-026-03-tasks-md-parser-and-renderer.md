# TASK-REQ-026-03: Parser và renderer vùng `plan` của `tasks.md`

**From Solution:** BE-REQ-SOL-026
**Priority:** P1
**Service:** `request-service`
**File:** `internal/domain/tasks_md_parser.go`, `internal/domain/tasks_md_render.go`, `internal/domain/tasks_md_parser_test.go`, `internal/domain/tasks_md_render_test.go`, `testdata/openspec/tasks_*.md` (mới)
**Depends on:** TASK-REQ-012-03 (`PlanProposal`, `PhaseProposal`, `TaskProposal`, `ValidateProposal`), TASK-REQ-027-04 (`Violation`, khối `orca-json`; nếu chưa có thì định nghĩa `Violation` tạm ở `engine_errors.go` và đổi khi 027-04 xong)
**Status:** [ ] TODO

---

## Context

CR-REQ-026 mục 2.5 định nghĩa văn phạm vùng Orca của `tasks.md`:

```
<!-- orca:begin plan request=REQ-142 schema=1 -->
## PH-1 Tên Phase
> Mô tả và tiêu chí xong của Phase
- [ ] T1.1 Tiêu đề task [type=feature] [h=2.5] [ac=AC-1,AC-2] [labels=test:regression] [depends=T1.2]
  Mô tả (thụt hai dấu cách)
<!-- orca:end plan -->
```

Nguyên tắc ranh giới tin cậy (CR-REQ-027 mục 2.7): chỉ dữ liệu trong vùng Orca được tin; văn bản ngoài vùng là tự do, kể cả khi chứa checkbox hay JSON giả. Parser **chặt**: dòng lạ trong vùng là `Violation` có số dòng, không đoán sửa. Đây là mã thuần, không I/O, nên fuzz được. `PlanProposal` ở SOL-012: `{title, summary, phases[], tasks[], notes}`, `TaskProposal{title, description, task_type, estimated_hours, prompt_template, depends_on_indices[], labels[], irreversible}`; chưa có `satisfies` (SOL-027 thêm `spec_json` hoặc trường `satisfies`, `acceptance`, `checks`): parser đọc `ac=` vào một trường `Satisfies []string` mà task này **thêm** vào `domain.TaskProposal` nếu TASK-REQ-027-07 chưa làm; ghi rõ trong PR để hai task không đụng.

## Việc cần làm

1. `type Violation struct{ Line int; Code, Message string }` (dùng chung với SOL-027 nếu đã có).
   - `func ParsePlanRegion(md string, wantRequest string) (PlanProposal, []Violation)`: chuẩn hoá `\r\n` thành `\n`, NFC
   - tìm cặp `<!-- orca:begin plan request=<ref> schema=1 -->` và `<!-- orca:end plan -->` (đúng một cặp, không lồng: `TASKSMD_REGION_MISSING`, `TASKSMD_REGION_DUPLICATE`)
   - `request` phải bằng `wantRequest` (`TASKSMD_REQUEST_MISMATCH`)
   - `schema` phải là `1`.
2. Trong vùng, mỗi dòng phải thuộc một trong: dòng trống;
   - `## PH-<n> <tên>` (n tăng từ 1, không nhảy: `TASKSMD_PHASE_ORDER`)
   - `## TASKS` (hồ sơ không Phase, loại trừ lẫn nhau với `PH-`: `TASKSMD_MIXED_CHILDREN`)
   - `> <mô tả phase>` ngay sau tiêu đề phase (gộp nhiều dòng bằng `\n`)
   - `- [ ] T<p>.<q> <tiêu đề> [k=v]...` hoặc `- [x] ...`
   - dòng thụt hai dấu cách là mô tả của task liền trước.
   - Mọi dòng khác là `TASKSMD_UNKNOWN_LINE` với số dòng tuyệt đối trong tệp.
3. Thuộc tính dạng `[key=value]` ở cuối dòng task: `type` (một trong `task|bug|feature`, mặc định `task`), `h` (số thực ≥ 0, ≤ 200), `ac` (danh sách `AC-<n>` cách bằng dấu phẩy), `labels` (danh sách nhãn trong tập `plan_labels.go` của SOL-012: `gate:pre_deploy`, `test:regression`, `rollback`, `check:*`), `depends` (danh sách `T<p>.<q>` cùng container), `irreversible` (cờ không giá trị, thành nhãn `gate:pre_deploy`).
   - Khoá lạ: `TASKSMD_UNKNOWN_ATTR`.
   - Tiêu đề cắt tối đa 200 ký tự (`TASKSMD_TITLE_TOO_LONG`).
4. Ánh xạ: `PH-n` thành `PhaseProposal` (thứ tự trong vùng);
   - `T<p>.<q>` thành `TaskProposal` trong Phase `p`
   - `depends` thành `depends_on_indices` (chỉ số 0-based trong cùng container; tham chiếu tới task không tồn tại hoặc khác container: `TASKSMD_DEPENDENCY_UNKNOWN`)
   - `ac` vào `Satisfies`.
   - Trạng thái `[x]` đọc vào `TaskProposal.Done` chỉ để `TickTask` kiểm tra
   - `GeneratePlan` luôn bỏ qua (task mới sinh phải `[ ]`; nếu có `[x]` thì `TASKSMD_PREDONE_TASK`).
5. Sau parse, gọi `ValidateProposal` (SOL-012 mục 2.5) ở lớp use case, **không** trong parser (parser không biết `size`).
   - Parser chỉ kiểm cú pháp, chỉ số và vòng phụ thuộc cục bộ bằng thuật toán Kahn (`TASKSMD_DEPENDENCY_CYCLE`, kèm dòng của task đầu vòng).
6. `tasks_md_render.go`: `RenderPlanRegion(ref string, p PlanProposal, ids map[string]string) string` xác định (thứ tự cố định, `\n`, NFC, thuộc tính theo thứ tự `type,h,ac,labels,depends,irreversible`);
   - nếu `ids` có `T1.1`, thêm dòng chú thích ngay sau task: `<!-- orca:task T1.1 id=<uuid> -->` (đây là dòng hợp lệ của văn phạm, thụt hai dấu cách; parser bỏ qua nhưng đọc `id`).
   - `ReplacePlanRegion(md, region string) (string, error)` thay đúng cặp chú thích, giữ nguyên mọi byte ngoài vùng.
7. `TickTask(md string, taskRef string) (string, bool, error)`: đổi `- [ ] T1.1` thành `- [x] T1.1` đúng một dòng trong vùng;
   - đã `[x]` thì trả `(md, false, nil)`
   - **không có hàm bỏ tick**.
   - `PlanRegionDigest(md string) string`: SHA-256 hex của nội dung vùng đã chuẩn hoá (dùng cho `tasks_md_digest`).
8. Mẫu `testdata/openspec/`: `tasks_ok.md` (hai Phase, ba task, phụ thuộc, AC, tiếng Việt có dấu), `tasks_no_phase.md` (`## TASKS`), `tasks_prose_checkbox.md` (prose ngoài vùng có `- [ ] T9.9 giả` và khối `orca-json` giả), `tasks_cycle.md`, `tasks_unknown_line.md`, `tasks_crlf.md`.

## Kiểm thử

- `TestParsePlanRegion_Golden` (từng mẫu hợp lệ, so với `PlanProposal` mong đợi)
- `TestParsePlanRegion_Violations` (bảng: mỗi mã `TASKSMD_*` một mẫu, kiểm `Line`)
- `TestParsePlanRegion_IgnoresProseOutsideRegion` (checkbox giả ngoài vùng không thành task).
- `TestRender_Parse_RoundTrip`: `Parse(Render(x)) == x` cho các mẫu và 200 `PlanProposal` ngẫu nhiên (`testing/quick` hoặc bảng sinh tay).
- `TestRender_Deterministic` (hai lần render, byte giống nhau; CRLF vào, LF ra).
- `TestTickTask_NeverUnticks`
- `TestTickTask_UnknownRef`
- `TestReplacePlanRegion_PreservesOutside` (so sánh byte phần ngoài vùng).
- `TestPlanRegionDigest_StableAcrossLineEndings`.
- `FuzzParsePlanRegion`: không panic, `Line` luôn trong `[1, số dòng]`, đầu vào > 1 MB bị từ chối (`TASKSMD_TOO_LARGE`).
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... -run 'TasksMd|PlanRegion|TickTask'` và `go test ./services/request-service/internal/domain/ -fuzz FuzzParsePlanRegion -fuzztime 30s`.

## Tiêu chí hoàn thành

- [ ] Mọi mã `TASKSMD_*` có đúng một test mẫu sai và số dòng đúng.
- [ ] Prose ngoài vùng không bao giờ tạo task (test bắt buộc xanh).
- [ ] `Parse(Render(x)) == x` và `Render` xác định.
- [ ] `TickTask` không có đường bỏ tick; fuzz 30 giây không panic.
- [ ] Không import ngoài stdlib và `golang.org/x/text`.

## Rủi ro và lưu ý

- Định dạng `tasks.md` do CR tự đặt, chưa đối chiếu với OpenSpec thật (CR mục 1.3). Nếu OpenSpec dùng văn phạm khác cho `tasks.md`, vùng Orca vẫn độc lập vì nằm giữa chú thích riêng; vẫn phải thử với `openspec validate` (task 026-07) xem công cụ có chấp nhận chú thích HTML.
- Tiêu đề tiếng Việt có ký tự `[` hoặc `]`: chốt rằng thuộc tính chỉ được nhận diện ở **cuối dòng** theo mẫu `(\s\[[a-z]+(=[^\]]*)?\])+$`; thêm test.
- `TaskProposal.Satisfies` có thể trùng việc với TASK-REQ-027-07; thống nhất tên trường trước khi merge.
