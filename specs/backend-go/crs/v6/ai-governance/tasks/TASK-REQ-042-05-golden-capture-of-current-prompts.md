# TASK-REQ-042-05: Chụp golden của mọi prompt hiện tại (trước khi di chuyển)

**From Solution:** [BE-REQ-SOL-042](../solutions/BE-REQ-SOL-042-prompt-catalog-shared-agent-relay-and-stdin.md)
**Priority:** P1
**Service:** `request-service, task-service, git-gateway-service`
**File:** `backend-go/services/request-service/internal/usecase/testdata/prompt-golden/`, `.../adapter/grpcclient/testdata/`, `task-service/.../testdata/`, `git-gateway-service/.../testdata/` (mới) + test sinh golden; `backend-go/ci/check-prompt-golden.sh` (mới)
**Depends on:** TASK-REQ-041-02 (đổi prompt phân tích: phản hồi cũ và giới hạn `prior_artifacts`); làm trước TASK-REQ-042-06 và 07. Nếu 041-02 chưa xong thì chụp golden sau khi nó xong, không chụp hai lần.
**Status:** [ ] TODO

---

## Context

- Mọi hàm dựng prompt hiện nay sinh chuỗi từ dữ liệu miền; không có golden.
- Golden là cổng an toàn của việc chuyển sang `.tmpl`: đầu ra mới phải trùng từng byte.

## Việc cần làm

1. Lập danh sách hàm dựng và bộ fixture đại diện cho từng luồng: classify; Solution (`ai.complete` và `AgentSolution`, có/không `UseKnowledge`); chẩn đoán theo `diagnosisFocus` (bug, security, performance); Findings; Answer; có/không `Feedback`, `RetryNote`, `PriorArtifacts`, `Answers`; Plan native theo `planShapeRules`/`planTypeRules` (mỗi loại); Plan OpenSpec; Solution OpenSpec; thực thi task (có/không cha, phụ thuộc, `PromptTemplate`, `{{outputs.*}}`) kèm `ContractPromptSuffix` (các tổ hợp `requirePassingCheck`, `knowledgeHub`); tách task; sinh prompt agent; commit message; agent prompt của git.
2. Test sinh file golden (cờ `-update`), một file mỗi ca, dữ liệu fixture cố định (không thời gian, không UUID ngẫu nhiên).
3. Test so khớp mặc định (không `-update`); `ci/check-prompt-golden.sh` chạy test và thất bại nếu golden thay đổi mà không có commit sửa golden cố ý.
4. Ghi vào PR số ca và hàm đã phủ; hàm nào không thể golden (phụ thuộc IO) thì nêu lý do.

## Kiểm thử

- Chạy test hai lần liên tiếp cho cùng kết quả (xác định).
- Cố ý đổi một câu trong prompt để chứng minh CI bắt được.

## Tiêu chí hoàn thành

- [ ] Mỗi hàm dựng prompt có ít nhất một golden; luồng có nhánh có golden cho từng nhánh.
- [ ] Golden xác định, không phụ thuộc thời gian hay ngẫu nhiên.
- [ ] CI bắt thay đổi golden không chủ ý.

## Rủi ro và lưu ý

- Fixture phải tránh dữ liệu thật; không đưa bí mật vào golden.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
