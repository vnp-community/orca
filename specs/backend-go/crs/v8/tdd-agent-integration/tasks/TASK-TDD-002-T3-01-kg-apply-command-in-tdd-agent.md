# TASK-TDD-002-T3-01: tdd-agent `kg apply`: áp một đề xuất ChangeSet đã lưu (nếu chọn phương án a)

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.5.4
**Checklist:** [T3.1](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P1 · **Area:** repo tdd-agent (Python)
**Files:** `tdd-agentic-ai/src/tdd_agent/cli.py` (nhóm `kg`), `project.py`, `mcp/tools.py` (đọc `changeset_propose`, `_dry_run`), `store/` (`commit_graph`), `tests/`
**Depends on:** T1.6, T1.7 (chọn phương án a)
**Status:** `[ ]` TODO

---

## Context

MCP `changeset_propose` chạy thử một ChangeSet trên head, lưu bản ghi đề xuất (kind `changeset_proposal` trong `operational` audit: `proposal_id`, `changeset_hash`, `changeset`, `report`, `head`, `base_snapshot`) và **không bao giờ commit**; SAD bị bảo vệ (`SAD_READONLY`). Hiện commit duy nhất là `tdd-agent kg import <graph.json>` (nhập cả đồ thị thành snapshot mới, `commit_graph(head, graph, actor, validate)`), không có lệnh áp đề xuất đã lưu. Task này chỉ làm nếu T1.7 chọn (a); nếu (b) hoặc (c) thì đóng với ghi chú.

## Việc cần làm

1. Thêm `tdd-agent kg apply <proposal_id> --actor <id> --rationale <text> --hash <hash của đề xuất> [--yes]`.
2. Đọc đề xuất từ audit (`PROPOSAL_NOT_FOUND` nếu không có); kiểm `--hash` bằng `changeset_hash` đã lưu (ràng buộc người áp với đúng đề xuất họ đã đọc, giống `change approve --hash`).
3. Kiểm đồng bộ: head hiện tại phải bằng `head` của đề xuất (hoặc `base_snapshot` khi có), khác ⇒ `STALE_BASE` kèm cách chạy lại `changeset_propose`.
4. Kiểm quyền: `--actor` phải có trong `[[approvals.actors]]` và có vai SA hoặc SA/Dev; agent (`coding_agent`) được phép nếu cấu hình cho phép; ghi actor và rationale vào audit.
5. Chạy lại dry-run để chắc `ok` (không tin đề xuất cũ); chạm SAD ⇒ từ chối.
6. Tính trước số phần tử chuyển `needs_review` (dựa trên cùng thuật toán lan của `apply_changeset`), in `needs_review_before`/`needs_review_after`, rồi `commit_graph`; in snapshot mới.
7. Thêm cờ `--dry-run` chỉ in kết quả; thêm vào `--help` và README; xuất ra JSON với `--json`.
8. Test: áp thành công, hash sai, head đã đổi, actor không hợp lệ, đề xuất chạm SAD, đề xuất `ok:false`.

## Tiêu chí hoàn thành

- [ ] Áp được một đề xuất `ok:true` lên bản sao KG, snapshot mới đúng, số `needs_review` được in.
- [ ] Mọi nhánh từ chối có mã lỗi rõ và test.
- [ ] Không có đường nào commit mà không qua kiểm hash, head và actor.

## Kiểm thử

- pytest trong repo tdd-agent cho từng ca; thử tay trên bản sao KG của pilot (T1.6).

## Rủi ro và ghi chú

- Chỉ chạy trên bản sao KG cho tới khi người dùng đồng ý áp lên KG pilot.
- Đây là thay đổi ở repo khác của người dùng; làm trên nhánh riêng và báo trước.
