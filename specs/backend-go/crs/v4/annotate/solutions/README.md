# backend-go Solutions — Annotate AI Diffs (F08, v4)

**CRs:** [docs/crs/v4/annotate/](../../../../../docs/crs/v4/annotate/README.md)
**Frontend counterpart:** [specs/frontend/crs/v4/annotate/solutions/](../../../../frontend/crs/v4/annotate/solutions/README.md)
**Agent counterpart:** [specs/agent/crs/v4/annotate/solutions/](../../../../agent/crs/v4/annotate/solutions/README.md) — zero scope, see there
**TDD tham chiếu:** [`annotation-service.md`](../../../tdd/services/annotation-service.md), [`03-clean-architecture-guidelines.md`](../../../tdd/architecture/03-clean-architecture-guidelines.md)

## Đánh giá trạng thái hiện tại — trước khi thiết kế bất kỳ giải pháp nào

Audit trực tiếp mã nguồn (không tin lời task claim, đọc `.proto`/`.go` thật)
khi viết 2 solution dưới đây xác nhận: **CR-ANNOTATE-001 không cần bất kỳ
thay đổi backend-go nào** — toàn bộ CRUD + field (`side`/`end_line`/
`original_code`/`sent_to_agent`/`worktree_id`) + `MarkAnnotationsSent` mà
frontend cần đã có sẵn, đã ship thật, đã test (CR-02 series, 12/12 task
DONE, tự xác nhận lại từng field trong `annotation.proto`). Đáng chú ý:
`specs/backend-go/tdd/services/annotation-service.md`'s §3/§4 mô tả 1 thiết
kế `ProjectID`/`ReviewID` **đã cũ, không khớp code thật** — code thật dùng
`RepoID`/`WorktreeID`, không có `ReviewID`. Sự lệch pha này (đã được chính
SOL-CR-02 ghi nhận trước đây) là nguồn gốc trực tiếp của bug
`annotation-panel.tsx` (frontend) gọi sai shape — xem SOL-BE-ANNOTATE-001 §2.

**CR-ANNOTATE-002 cần 1 thay đổi nhỏ, đúng tinh thần "ít thay đổi code
nhất"**: tách 3 bước đầu (collect/context/format) của hàm
`SendReviewFeedbackToAgent` (đã có sẵn, SOL-CR-03) thành 1 hàm mới
`ComposeReviewFeedbackPrompt`, expose qua 1 channel mới
`annotation.composeReviewPrompt`. Không viết lại logic đã đúng — chỉ cắt tại
đúng ranh giới bước đã có sẵn trong chính doc comment của hàm gốc.

## Solutions

| Solution | CR | Service | Status |
|---|---|---|---|
| [SOL-BE-ANNOTATE-001](./SOL-BE-ANNOTATE-001-existing-crud-suffices.md) | CR-ANNOTATE-001 | `annotation-service` (không sửa) | 📐 Assessment — zero code change |
| [SOL-BE-ANNOTATE-002](./SOL-BE-ANNOTATE-002-extract-compose-only-channel.md) | CR-ANNOTATE-002 | `api-gateway` | 🔲 Designed — chưa implement |

## Thứ tự implement

```
SOL-BE-ANNOTATE-001 — không có task, không có gì để implement (assessment only)
        │ (chỉ cần xác nhận đúng, không chặn CR-ANNOTATE-001's phần frontend)
        ▼
SOL-BE-ANNOTATE-002 — độc lập về code với CR-ANNOTATE-001's frontend work,
                      nhưng về mặt DỮ LIỆU phụ thuộc CR-ANNOTATE-001 xong
                      trước (annotation.composeReviewPrompt đọc annotation
                      đã persist với side/original_code/worktree_id — nếu
                      comment vẫn chỉ ở Zustand local, channel này không có
                      gì để đọc, dù bản thân code backend-go build/test vẫn
                      pass)
```

## Nguyên tắc bảo mật xuyên suốt

`tenantID`/`repo_id` scoping cho `ListAnnotations`/`MarkAnnotationsSent`
trong `ComposeReviewFeedbackPrompt` giữ nguyên y hệt
`SendReviewFeedbackToAgent` đã dùng (không đổi gì) — CR này không mở thêm
bề mặt truy cập nào, chỉ tách 1 hàm nội bộ thành 2.
