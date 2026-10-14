# TASK-TDD-002-T1-07: Quyết định kênh commit cho ghi ngược KG

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.5.4, 6 (H5)
**Checklist:** [T1.7](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P1 · **Area:** quyết định
**Files:** `evidence/T1.7-decision.md` (mới); nếu chọn (a): repo tdd-agent
**Depends on:** T1.6
**Status:** `[ ]` TODO

---

## Context

MCP `changeset_propose` chỉ chạy thử và lưu đề xuất; commit duy nhất hiện có là `tdd-agent kg import <graph.json>` (nhập cả đồ thị thành snapshot mới), không có lệnh áp một đề xuất đã lưu. Ba phương án: (a) thêm `tdd-agent kg apply`; (b) Orca áp bằng script; (c) chỉ đề xuất, người áp tay.

## Việc cần làm

1. Trình bày ba phương án với số liệu T1.6 (nợ duyệt) và chi phí: (a) thay đổi repo tdd-agent, an toàn nhất vì áp đúng ChangeSet đã chạy thử, kiểm base snapshot và actor; (b) Orca phải chạy Python bên ngoài, vi phạm nguyên tắc "Orca không đụng KG"; (c) không tự động.
2. Khuyến nghị (a) với hợp đồng: `kg apply <proposal_id> --actor <id> --rationale <text> --hash <hash của đề xuất>`; từ chối khi head đã đổi (`STALE_BASE`), actor không có trong `[[approvals.actors]]`, hoặc đề xuất chạm SAD; in `needs_review` trước/sau.
3. Hỏi người dùng chọn; ghi quyết định, người quyết, ngày vào `evidence/T1.7-decision.md`.
4. Nếu chọn (a): mở task trong repo tdd-agent (`src/tdd_agent/cli.py`, `project.py`, test) và nối vào T3.1.

## Tiêu chí hoàn thành

- [ ] Có quyết định bằng văn bản của người dùng.
- [ ] Nếu (a): có task tương ứng ở repo tdd-agent.

## Kiểm thử

- Không có test.

## Rủi ro và ghi chú

- Cho tới khi có quyết định, ghi ngược KG-only dừng ở "đã đề xuất" (T3.2).
