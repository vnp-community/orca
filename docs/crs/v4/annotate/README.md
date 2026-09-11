# Annotate AI Diffs (F08) — Change Requests (v4)

> **Bối cảnh:** Yêu cầu "tạo các Change Request để thực thi tính năng Annotate AI Diffs",
> dựa trên [`docs/features/F08-annotate-ai-diffs.md`](../../features/F08-annotate-ai-diffs.md)
> và [`docs/roadmap/feature-completion-matrix.md`](../../roadmap/feature-completion-matrix.md)
> (dòng F08: *"AG chỉ có shared format type, chưa có logic thực thi phía relay"*).
> Khảo sát trực tiếp code (Read/Grep + GitNexus) **không xác nhận đúng khung câu hỏi ban
> đầu** — gap thật khác hẳn những gì ma trận nêu, và nằm ở một chỗ không ngờ tới: không
> phải `agent/` thiếu gì, mà là **2 hệ thống lưu comment độc lập, không kết nối với nhau**
> đang tồn tại song song trong `frontend/`.

## Kết luận khảo sát — đính chính khung câu hỏi ban đầu

| Câu hỏi ban đầu | Trả lời (bằng chứng chi tiết trong CR-ANNOTATE-001/002 §0) |
|---|---|
| `agent/` thiếu "logic thực thi phía relay" cho diff comments? | **Không đúng.** PTY injection (`terminal.send`) đã là cơ chế **có thật, đang chạy**, dùng chung với F02 Terminal Splits — không có gap agent-side nào. Cái ma trận gọi là "shared format type" (`diff-comments-format.ts`) thực ra **đang được dùng thật** để format prompt gửi đi — không phải type chết. |
| Vậy F08 đã xong chưa? | **Chưa, nhưng vì lý do hoàn toàn khác.** Backend-go đã xây xong toàn bộ pipeline "persist + compose-with-context + send + mark-sent" cho annotation (CR-02/CR-03 series, `specs/backend-go/bugs/logic-v1/`, 12/12 task DONE) — nhưng **frontend không gọi tới nó**. Frontend có sẵn 1 hệ thống review-buffer riêng, hoạt động tốt cho việc gửi đi, **CÓ persist thật** (qua `worktree.set` → `project-service`'s `WorktreeMeta.metadata` JSONB, không mất dữ liệu) nhưng **không phải qua `annotation-service`** — 2 con đường lưu trữ tách biệt hoàn toàn, không ai đọc được dữ liệu của bên kia. |
| 2 hệ thống đó là gì? | Xem bảng "Gap thật" bên dưới. |

## Gap thật đã xác nhận

| Hệ thống | Vai trò | Trạng thái thật |
|---|---|---|
| **Review buffer đang dùng thật** — `useDiffCommentDecorator.tsx`, `DiffCommentCard.tsx`, `SourceControl.tsx`'s panel "Notes" | Click vào dòng diff → thêm comment; hiển thị badge số lượng; "Send to Agent" tới bất kỳ agent session nào đang chạy trong worktree | ✅ UX hoàn chỉnh, gửi thật qua `terminal.send` (guarded bracketed-paste + chờ agent idle + Enter). **CÓ persist thật** (`worktree.set` → `project-service`'s `WorktreeMeta.metadata` JSONB, durable) — nhưng **KHÔNG BAO GIỜ gọi `annotation-service`**, nên không query được độc lập, không có OPA author-guard khi sửa/xoá, và `annotation.sendToAgent`/`composeReviewPrompt` (CR-ANNOTATE-002) không đọc được gì từ đây |
| **`annotation-panel.tsx`** (`components/code-review/`) | Component RIÊNG, cũng render trong `DiffViewer`/`MonacoEditor`, gọi `annotation.create`/`annotation.list` | ❌ **Hỏng** — dùng shape tham số (`projectId`/`reviewId`/`lineNumber`) không khớp proto thật của backend-go (`repo_id`/`file_path`/`line`/`ref`/`side`); desktop mode **không có IPC handler nào** cho 2 method này (grep xác nhận 0 kết quả ngoài chính file này + test của nó) |
| **`annotation-service` (backend-go)** — CRUD, `side`/`end_line`/`original_code`/`sent_to_agent`/`worktree_id`, `annotation.sendToAgent` compose-with-code-context | Được thiết kế đúng để làm nguồn sự thật (source of truth) cho review comment + tự động thêm ±2 dòng code context khi gửi (BR-CR-11) | ✅ Code thật, 12/12 task DONE (`go build`/`go vet`/`go test` sạch — đã tự xác nhận), nhưng **0 caller thật từ frontend** — toàn bộ công sức này hiện đang mồ côi |
| Format prompt gửi đi thật (`shared/diff-comments-format.ts`, đang chạy) | `File: ..., Line: N, User comment: "..."` | Thiếu "Code:" (±2 dòng context, BR-CR-11) và nhãn old/new side (BR-CR-05/09) mà backend-go's `annotation.sendToAgent` đã làm sẵn nhưng không được gọi tới |

| CR | Vấn đề | Priority | Effort | Status |
|----|--------|----------|--------|--------|
| [CR-ANNOTATE-001](./CR-ANNOTATE-001-persist-diff-comments-to-annotation-service.md) | Review buffer sai tầng lưu trữ (JSONB thay vì annotation-service), không OPA-guard; `annotation-panel.tsx` gọi API sai shape, không hoạt động ở cả 2 mode | 🟡 P2 *(hạ từ P1, xem CR §1)* | Large | ✅ DONE (2026-09-11) |
| [CR-ANNOTATE-002](./CR-ANNOTATE-002-route-send-to-agent-through-composed-prompt.md) | Prompt gửi agent thiếu code context (BR-CR-11) vì không dùng `annotation.sendToAgent` đã xây sẵn ở backend-go | 🟡 P2 | Medium | ✅ DONE (2026-09-11) |

## Thứ tự thực thi

```
CR-ANNOTATE-001 (persist review buffer vào annotation-service thật,
                 sửa/thay annotation-panel.tsx)
                    │
                    ▼
CR-ANNOTATE-002 (route send-to-agent qua annotation.sendToAgent để có
                 code context — cần comment đã persist với side/
                 original_code/worktree_id trước)
```

Không tách CR riêng cho `agent/` — khảo sát xác nhận `agent/` không có gap
thật nào cho F08 (xem CR-ANNOTATE-001 §0). Đề xuất cập nhật lại
`docs/roadmap/feature-completion-matrix.md`'s dòng F08 sau khi 2 CR này
triển khai xong (không sửa trong phạm vi khảo sát này).

## Quyết định kiến trúc còn treo (không tự chốt trong bộ CR này)

CR-ANNOTATE-002's mục "Quyết định kiến trúc cần chốt trước khi code" nêu
đánh đổi giữa 2 cơ chế delivery đã tồn tại song song: backend-go's
`annotation.sendToAgent` gửi qua 1 lệnh `PtyClientFrame_Input` thô, còn
frontend's `sendNotesToActiveAgentSession` đã có sẵn logic đợi agent
idle + guarded bracketed-paste + Enter (đáng tin cậy hơn với nhiều loại
TUI agent khác nhau) — **chọn hướng nào ảnh hưởng đáng kể phạm vi
Changes Required của CR-ANNOTATE-002**, cần chốt với người phụ trách
trước khi implement.

## Impact analysis (gitnexus, chạy trước khi sửa — bắt buộc theo CLAUDE.md)

| Symbol sửa | Risk | CR |
|---|---|---|
| `DiffComment` (`frontend/src/shared/types.ts:759-780`) — thêm field lưu server-id/sync-state nếu cần | Cần chạy `impact({target: "DiffComment", direction: "upstream"})` trước khi sửa — type này được dùng ở rất nhiều nơi (`useDiffCommentDecorator`, `DiffCommentCard`, `DiffNotesSendMenu`, `SourceControl.tsx`, mobile diff review) | CR-ANNOTATE-001 |
| `formatDiffComment`/`formatDiffComments` (`frontend/src/shared/diff-comments-format.ts`) | Dùng chung cho cả diff-notes và markdown-notes send flow (`isMarkdownComment`) — sửa format phải không phá nhánh markdown | CR-ANNOTATE-002 |
| `annotation-panel.tsx` | `context()` trước khi quyết định sửa hay xoá — xác nhận đúng thật danh sách caller (`DiffViewer`/`DiffSectionBody`/`MonacoEditor`/`RichMarkdownAnnotationOverlay`/`code-review-panel.tsx`, đã grep xác nhận sơ bộ, cần xác nhận lại bằng GitNexus trước khi động vào) | CR-ANNOTATE-001 |

Symbol backend-go mới cần gọi tới (`annotation.sendToAgent` qua wscompat,
`annotation.create`/`list`/`delete` với shape đúng) đã tồn tại — chạy
`context({name: "registerAnnotationSendChannel"})`/tương đương để xác
nhận signature thật trước khi wire từ frontend, theo đúng quy tắc bắt
buộc của repo.

## Liên quan

- [F08-annotate-ai-diffs.md](../../features/F08-annotate-ai-diffs.md)
- [feature-completion-matrix.md](../../roadmap/feature-completion-matrix.md) (dòng F08 — sẽ cần đính chính sau khi 2 CR này rõ trạng thái)
- `specs/backend-go/bugs/logic-v1/BUG-CR-02-annotate-diff-partial.md` + `BUG-CR-03-gui-feedback-agent-partial.md` — nguồn gốc thiết kế `annotation-service`/`annotation.sendToAgent` mà bộ CR này tái sử dụng, không xây lại
- `specs/backend-go/bugs/logic-v1/solutions/SOL-CR-02-annotation-side-range-sent-state.md`, `SOL-CR-03-review-feedback-prompt-composition.md`
