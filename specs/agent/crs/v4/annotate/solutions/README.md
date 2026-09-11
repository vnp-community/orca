# agent Solutions — Annotate AI Diffs (F08, v4)

**CRs:** [docs/crs/v4/annotate/](../../../../../docs/crs/v4/annotate/README.md)
**Frontend counterpart:** [specs/frontend/crs/v4/annotate/solutions/](../../../../frontend/crs/v4/annotate/solutions/README.md)
**Backend-go counterpart:** [specs/backend-go/crs/v4/annotate/solutions/](../../../../backend-go/crs/v4/annotate/solutions/README.md)
**TDD tham chiếu:** [`00-index.md`](../../../tdd/v5/00-index.md) Addendum A.12 (Feature → Dev Server Component Mapping)

## Đánh giá trạng thái hiện tại — trước khi thiết kế bất kỳ giải pháp nào

Cả 2 CR trong nhóm này (CR-ANNOTATE-001: đổi nơi lưu comment; CR-ANNOTATE-002:
đổi nơi compose prompt) đều **không đụng tới cách prompt được tiêm vào PTY**
— cả hai đều giữ nguyên `active-agent-note-send.ts`'s cơ chế guarded-paste
đã có (xem CR-ANNOTATE-002 §0's ràng buộc tường minh). PTY handler ở `agent/`
(`pty-handler.ts`) không quan tâm nội dung byte nó ghi — dùng chung y hệt với
F02 Terminal Splits. Grep xác nhận `agent/src/` không có bất kỳ khái niệm
nào về "annotation"/"diff comment"/"review feedback". `specs/agent/tdd/v5/00-index.md`'s
bảng "Feature → Dev Server Component Mapping" cũng không có dòng F08 — khớp
với kết luận không có gap.

## Solutions

| CR | Solution | Status | Note |
|----|----------|--------|------|
| CR-ANNOTATE-001 | [SOL-AG-ANNOTATE-001](./SOL-AG-ANNOTATE-001-zero-agent-scope.md) | 📐 Assessment | **Zero code change** — persistence-layer change, không chạm delivery |
| CR-ANNOTATE-002 | [SOL-AG-ANNOTATE-001](./SOL-AG-ANNOTATE-001-zero-agent-scope.md) (cùng 1 solution) | 📐 Assessment | **Zero code change** — chỉ đổi nguồn compose prompt, không đổi cách gửi |

Dùng chung 1 solution cho cả 2 CR (không tách riêng như mobile-companion đã
làm) vì lý do "không có gap" giống hệt nhau cho cả hai — tách ra sẽ lặp lại
đúng 1 bằng chứng 2 lần, không thêm giá trị. (Mirrors the precedent in
`specs/agent/crs/v4/automation/solutions/README.md`, which also used one
combined solution for 8 CRs sharing the same "agent RPC surface already
suffices" conclusion.)

## Why this has no tasks

Assessment-only, per its own "Not in scope" section: *"this is a
confirmation, not a solution to implement."* Creating a task here would
invent work the solution itself says isn't there — mirrors
`specs/agent/crs/v4/task-graph/tasks/README.md`'s "Why SOL-AG-TG-001 has no
tasks" precedent.
